package stream

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"omni-schema/internal/codec"
	"omni-schema/internal/lexer"
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

// Subscription represents an active client stream.
type Subscription struct {
	ID            string
	Conn          *network.Conn
	SchemaName    string
	SchemaVersion string
	Closed        chan struct{}
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

// Publish routes an event to all active subscribers. The full pipeline is:
//
//	event (map[string]any)
//	  → lexer.MapToUIR → event UIR graph
//	  → registry.GetVersion(sub.SchemaName, sub.SchemaVersion) → schema UIR
//	  → uir.Project(eventUIR, schemaUIR) → projected UIR
//	  → codec.GenerateGraphQL(projected) → GraphQL result string
//	  → wrap in subscription envelope
//	  → WebSocket text frame
//
// If the subscriber has no schema bound (version == "unknown"), the event is sent
// as raw JSON passthrough for backwards compatibility.
func (b *Broker) Publish(eventType string, data any) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	for sub := range b.subscriptions {
		payloadBytes, err := b.buildSubscriptionPayload(sub, eventType, data)
		if err != nil {
			log.Printf("Failed to build payload for sub %s: %v", sub.ID, err)
			continue
		}

		if err := sub.Conn.WriteMessage(network.OpText, payloadBytes); err != nil {
			log.Printf("Failed to write to sub %s: %v", sub.ID, err)
		}
	}
}

// buildSubscriptionPayload constructs the GraphQL-over-WebSocket response for a
// single subscriber, projecting the event data through the UIR if a schema is bound.
func (b *Broker) buildSubscriptionPayload(sub *Subscription, eventType string, data any) ([]byte, error) {
	var resultData any

	if sub.SchemaVersion != "unknown" {
		// Schema-aware path: convert event data to UIR, project against schema, generate GraphQL.
		projected, err := b.projectEvent(sub, eventType, data)
		if err != nil {
			// Fall back to raw passthrough on projection failure.
			log.Printf("Projection failed for sub %s, falling back to raw: %v", sub.ID, err)
			resultData = map[string]any{eventType: data}
		} else {
			resultData = map[string]any{
				eventType: projected,
			}
		}
	} else {
		// No schema bound — raw JSON passthrough.
		resultData = map[string]any{eventType: data}
	}

	payload := map[string]any{
		"id":   sub.ID,
		"type": "next",
		"payload": map[string]any{
			"data":             resultData,
			"__schema_version": sub.SchemaVersion,
			"__timestamp":      time.Now().Unix(),
		},
	}

	return json.Marshal(payload)
}

// projectEvent converts the raw event data into a UIR graph, retrieves the subscriber's
// bound schema from the registry, projects the event UIR against the schema UIR, and
// returns the projected result as a map[string]any for JSON serialisation.
func (b *Broker) projectEvent(sub *Subscription, eventType string, data any) (any, error) {
	// Step 1: Convert the event data to a UIR graph.
	dataMap, ok := data.(map[string]any)
	if !ok {
		// If the data is not a map, wrap it so we have a consistent structure.
		dataMap = map[string]any{"value": data}
	}

	eventRoot := uir.NewNode(uir.TypeMap, "event", nil)
	lexer.MapToUIR(eventRoot, dataMap)

	// Step 2: Retrieve the subscriber's bound schema version from the registry.
	schemaMeta, found := registry.Default.GetVersion(sub.SchemaName, sub.SchemaVersion)
	if !found {
		return nil, fmt.Errorf("schema %s version %s not found in registry", sub.SchemaName, sub.SchemaVersion)
	}

	// Step 3: Project the event UIR against the schema UIR.
	// We need to find the appropriate type in the schema to project against.
	// Use the first child type of the schema root as the projection target.
	var schemaTarget *uir.Node
	if schemaMeta.Root != nil && len(schemaMeta.Root.Children) > 0 {
		schemaTarget = schemaMeta.Root.Children[0]
	} else {
		schemaTarget = schemaMeta.Root
	}

	if schemaTarget == nil {
		return nil, fmt.Errorf("schema %s has no type definitions", sub.SchemaName)
	}

	projected := uir.Project(eventRoot, schemaTarget)

	// Step 4: Generate GraphQL SDL from the projected UIR.
	graphqlBytes, err := codec.GenerateGraphQL(projected)
	if err != nil {
		return nil, fmt.Errorf("GraphQL generation failed: %w", err)
	}

	// Return the projected GraphQL as a structured response.
	return map[string]any{
		"__projected_schema": string(graphqlBytes),
		"__source_fields":    countFields(projected),
	}, nil
}

// countFields counts the number of fields in a UIR node tree (for diagnostics).
func countFields(n *uir.Node) int {
	count := 0
	for _, child := range n.Children {
		count++
		count += countFields(child)
	}
	return count
}
