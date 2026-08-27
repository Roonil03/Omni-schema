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
	TargetFormat    string
	SourceFormat    string
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
	log.Printf("Added subscription %s bound to schema %s:%s (format: %s)", sub.ID, sub.SchemaName, sub.SchemaVersion, sub.TargetFormat)
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

// buildSubscriptionPayload constructs the response for a single subscriber, 
// projecting the event data through the UIR and encoding to TargetFormat.
func (b *Broker) buildSubscriptionPayload(sub *Subscription, eventType string, eventRoot *uir.Node) ([]byte, error) {
	var finalNode *uir.Node
	var err error

	if sub.SchemaVersion != "unknown" {
		finalNode, err = b.projectEventToNode(sub, eventType, eventRoot)
		if err != nil {
			log.Printf("Projection failed for sub %s: %v", sub.ID, err)
			return nil, err
		}
	} else {
		finalNode = eventRoot
	}

	// Use generic encoder for the target format
	encoder, err := codec.GetEncoder(sub.TargetFormat)
	if err != nil {
		return nil, fmt.Errorf("failed to get encoder for target format %s: %v", sub.TargetFormat, err)
	}

	encodedBytes, err := encoder.Encode(finalNode)
	if err != nil {
		return nil, fmt.Errorf("encoding failed for %s: %v", sub.TargetFormat, err)
	}

	// If the target is graphql, wrap it in a subscription envelope
	if sub.TargetFormat == "graphql" || sub.TargetFormat == "json" {
		// GraphQL over WebSocket format requires an envelope
		// Note: The GraphQLResult encoder returns {"data": {...}}
		var resultData json.RawMessage
		if sub.TargetFormat == "graphql" {
			// Strip the {"data": ...} to fit in the graphql-ws envelope.
			// Actually, GenerateGraphQLResult returns exactly {"data": {...}}, 
			// so we can just embed it in the payload.
			resultData = json.RawMessage(encodedBytes)
		} else {
			resultData = json.RawMessage(encodedBytes)
		}
		
		envelope := map[string]any{
			"id":   sub.ID,
			"type": "next",
			"payload": resultData,
		}
		
		if sub.TargetFormat == "json" {
			envelope["payload"] = map[string]any{"data": map[string]any{eventType: json.RawMessage(encodedBytes)}}
		} else {
			// For graphql, encodedBytes is already a JSON object {"data": ...}, we just need to wrap it inside the "payload" key
			// Wait, the payload field in graphql-ws *is* the GraphQL response, i.e., {"data": {...}, "errors": [...]}.
			// So `encodedBytes` is perfectly formatted to be the `payload` value.
			var gqlPayload map[string]any
			json.Unmarshal(encodedBytes, &gqlPayload)
			
			// We have to wrap it back in the top-level event type key
			dataMap, ok := gqlPayload["data"].(map[string]any)
			if ok {
				newPayload := map[string]any{
					"data": map[string]any{
						eventType: dataMap,
					},
				}
				envelope["payload"] = newPayload
			} else {
				envelope["payload"] = gqlPayload
			}
		}

		return json.Marshal(envelope)
	}

	return encodedBytes, nil
}

// projectEventToNode retrieves the subscriber's bound schema from the registry, projects the
// event UIR against the schema UIR, and returns the projected UIR Node.
func (b *Broker) projectEventToNode(sub *Subscription, eventType string, eventRoot *uir.Node) (*uir.Node, error) {
	schemaMeta, found := registry.Default.GetVersion(sub.SchemaName, sub.SchemaVersion)
	if !found {
		return nil, fmt.Errorf("schema %s version %s not found in registry", sub.SchemaName, sub.SchemaVersion)
	}

	var schemaTarget *uir.Node
	if schemaMeta.Root != nil {
		for _, child := range schemaMeta.Root.Children {
			if strings.EqualFold(child.Key, eventType) {
				schemaTarget = child
				break
			}
		}
	}

	if schemaTarget == nil {
		return nil, fmt.Errorf("schema %s has no matching type definition for event '%s'", sub.SchemaName, eventType)
	}

	opts := uir.ProjectOptions{
		UnknownFields:      uir.UnknownFieldIgnore,
		EmitNullForMissing: true,
	}
	projected, err := uir.Project(eventRoot, schemaTarget, opts)
	if err != nil {
		return nil, fmt.Errorf("schema validation failed: %w", err)
	}

	if len(sub.RequestedFields) > 0 {
		projected = filterBySelection(projected, sub.RequestedFields)
	}

	return projected, nil
}
