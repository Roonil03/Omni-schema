package stream

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"

	"omni-schema/internal/ast"
	"omni-schema/internal/codec"
	"omni-schema/internal/network"
	"omni-schema/internal/registry"
	"omni-schema/internal/uir"
)

// Broker manages active subscriptions and event routing. When a registered schema
// is bound to a subscription, events are projected through the UIR against the
// subscriber's schema version before being serialised to the target format.
type Broker struct {
	mu            sync.RWMutex
	subscriptions map[*Subscription]bool
}

type Subscription struct {
	ID              string
	Conn            *network.Conn
	SchemaName      string
	SchemaVersion   string
	RequestedFields []ast.GraphQLSelection
	Queue           chan []byte
	Closed          chan struct{}
}

// DefaultBroker is the global event broker.
var DefaultBroker = NewBroker()

func NewBroker() *Broker {
	return &Broker{
		subscriptions: make(map[*Subscription]bool),
	}
}

func (b *Broker) AddSubscription(sub *Subscription) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.subscriptions[sub] = true
	log.Printf("Added subscription %s bound to schema %s:%s", sub.ID, sub.SchemaName, sub.SchemaVersion)
}

func (b *Broker) RemoveSubscription(sub *Subscription) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, ok := b.subscriptions[sub]; ok {
		delete(b.subscriptions, sub)
		close(sub.Closed)
		log.Printf("Removed subscription %s", sub.ID)
	}
}

// Publish decodes an incoming event using the registry and routes it to all active subscribers.
// The full pipeline is:
//	event bytes → codec.GetDecoder(sourceFormat) → event UIR graph
//	  → registry.GetVersion(sub.SchemaName, sub.SchemaVersion) → schema UIR
//	  → uir.Project(eventUIR, schemaUIR) → projected UIR
//	  → codec.GenerateJSON(projected) → GraphQL result JSON
//	  → wrap in subscription envelope → WebSocket text frame
func (b *Broker) Publish(sourceFormat string, eventType string, rawData []byte) {
	decoder, err := codec.GetDecoder(sourceFormat)
	if err != nil {
		log.Printf("Unsupported source format %s: %v", sourceFormat, err)
		return
	}

	eventRoot, err := decoder.Decode(rawData)
	if err != nil {
		log.Printf("Failed to decode event payload: %v", err)
		return
	}

	b.mu.RLock()
	defer b.mu.RUnlock()

	for sub := range b.subscriptions {
		payloadBytes, err := b.buildSubscriptionPayload(sub, eventType, eventRoot)
		if err != nil {
			log.Printf("Failed to build payload for sub %s: %v", sub.ID, err)
			continue
		}

		select {
		case sub.Queue <- payloadBytes:
		default:
			log.Printf("Backpressure: dropped event for sub %s (queue full)", sub.ID)
		}
	}
}

// buildSubscriptionPayload constructs the GraphQL-over-WebSocket response for a
// single subscriber, projecting the event data through the UIR if a schema is bound.
func (b *Broker) buildSubscriptionPayload(sub *Subscription, eventType string, eventRoot *uir.Node) ([]byte, error) {
	var resultData any
	var errorsList []map[string]any

	if sub.SchemaVersion != "unknown" {
		// Schema-aware path: convert event data to UIR, project against schema, generate JSON data payload.
		projectedJSONBytes, err := b.projectEvent(sub, eventType, eventRoot)
		if err != nil {
			// Projection failed. Fail closed and emit a GraphQL error.
			log.Printf("Projection failed for sub %s: %v", sub.ID, err)
			errorsList = append(errorsList, map[string]any{"message": err.Error()})
		} else {
			resultData = map[string]any{
				eventType: json.RawMessage(projectedJSONBytes),
			}
		}
	} else {
		// No schema bound — raw JSON passthrough.
		// Since we have an eventRoot UIR node instead of a raw map, we can just serialize it directly.
		rawJSON, _ := codec.GenerateJSON(eventRoot)
		resultData = map[string]any{eventType: json.RawMessage(rawJSON)}
	}

	payload := map[string]any{
		"id":   sub.ID,
		"type": "next",
		"payload": map[string]any{
			"data": resultData,
		},
	}
	
	if len(errorsList) > 0 {
		payload["payload"].(map[string]any)["errors"] = errorsList
	}

	return json.Marshal(payload)
}

// projectEvent retrieves the subscriber's bound schema from the registry, projects the
// event UIR against the schema UIR, and returns the projected result as a JSON byte array.
func (b *Broker) projectEvent(sub *Subscription, eventType string, eventRoot *uir.Node) ([]byte, error) {
	// Step 1: Retrieve the subscriber's bound schema version from the registry.
	schemaMeta, found := registry.Default.GetVersion(sub.SchemaName, sub.SchemaVersion)
	if !found {
		return nil, fmt.Errorf("schema %s version %s not found in registry", sub.SchemaName, sub.SchemaVersion)
	}

	// Step 3: Project the event UIR against the schema UIR.
	// Map the event type to the corresponding type in the schema.
	var schemaTarget *uir.Node
	if schemaMeta.Root != nil {
		for _, child := range schemaMeta.Root.Children {
			// simple heuristic: match event type string to type name ignoring case
			if strings.EqualFold(child.Key, eventType) {
				schemaTarget = child
				break
			}
		}
	}

	if schemaTarget == nil {
		return nil, fmt.Errorf("schema %s has no matching type definition for event '%s'", sub.SchemaName, eventType)
	}

	projected, err := uir.Project(eventRoot, schemaTarget)
	if err != nil {
		return nil, fmt.Errorf("schema validation failed: %w", err)
	}

	// Step 3.5: If the client requested specific fields, filter the projected result.
	if len(sub.RequestedFields) > 0 {
		projected = filterBySelection(projected, sub.RequestedFields)
	}

	// Step 4: Generate actual JSON data from the projected UIR (Not Schema SDL).
	jsonBytes, err := codec.GenerateJSON(projected)
	if err != nil {
		return nil, fmt.Errorf("JSON payload generation failed: %w", err)
	}

	return jsonBytes, nil
}
