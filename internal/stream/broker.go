package stream

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	"omni-schema/internal/network"
)

// Broker manages active subscriptions and event routing.
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

// Publish payload to all active subscribers.
func (b *Broker) Publish(eventType string, data any) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	// In a complete schema-aware projection, we would transform the `data` UIR
	// against each subscriber's bound schema version. Here we do a basic projection.
	
	for sub := range b.subscriptions {
		// Construct the GraphQL subscription payload
		payload := map[string]any{
			"id":   sub.ID,
			"type": "next",
			"payload": map[string]any{
				"data": map[string]any{
					eventType: data,
				},
				"__schema_version": sub.SchemaVersion, // To prove version binding
				"__timestamp":      time.Now().Unix(),
			},
		}

		payloadBytes, err := json.Marshal(payload)
		if err == nil {
			err = sub.Conn.WriteMessage(network.OpText, payloadBytes)
			if err != nil {
				log.Printf("Failed to write to sub %s: %v", sub.ID, err)
				// A real broker would enqueue this for removal
			}
		}
	}
}
