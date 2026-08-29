package stream

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"omni-schema/internal/ast"
	"omni-schema/internal/codec"
	"omni-schema/internal/registry"
	"omni-schema/internal/uir"
)

type Broker struct {
	mu            sync.RWMutex
	subscriptions map[*Subscription]bool
	eventSeq      atomic.Uint64
	order         string // per-subscription
}

var DefaultBroker = NewBroker()

func NewBroker() *Broker {
	return &Broker{
		subscriptions: make(map[*Subscription]bool),
		order:         "per-subscription",
	}
}

func (b *Broker) AddSubscription(sub *Subscription) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if sub.Closed == nil {
		sub.Closed = make(chan struct{})
	}
	if sub.Queue == nil {
		sub.Queue = make(chan []byte, 100)
	}
	b.subscriptions[sub] = true
	log.Printf("Added subscription %s bound to schema %s:%s (format: %s) delivery=%s", sub.ID, sub.SchemaName, sub.SchemaVersion, sub.TargetFormat, DeliveryMode)
}

func (b *Broker) RemoveSubscription(sub *Subscription) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, ok := b.subscriptions[sub]; ok {
		delete(b.subscriptions, sub)
	}
	sub.Close()
}

func (b *Broker) Publish(sourceFormat string, eventType string, rawData []byte) {
	b.PublishEvent(Event{
		ID:           fmt.Sprintf("evt-%d", b.eventSeq.Add(1)),
		Type:         eventType,
		SourceFormat: sourceFormat,
		Payload:      rawData,
		Time:         time.Now(),
	})
}

type Event struct {
	ID           string
	Type         string
	SourceFormat string
	Payload      []byte
	Time         time.Time
	Cursor       string
}

func (b *Broker) PublishEvent(evt Event) {
	if evt.ID == "" {
		evt.ID = fmt.Sprintf("evt-%d", b.eventSeq.Add(1))
	}
	if evt.Cursor == "" {
		evt.Cursor = evt.ID
	}
	node, err := codec.DecodePayload(evt.SourceFormat, evt.Payload, codec.Options{})
	if err != nil {
		log.Printf("Failed to decode event payload: %v", err)
		return
	}

	b.mu.RLock()
	subs := make([]*Subscription, 0, len(b.subscriptions))
	for sub := range b.subscriptions {
		subs = append(subs, sub)
	}
	b.mu.RUnlock()

	for _, sub := range subs {
		select {
		case <-sub.Closed:
			continue
		default:
		}
		if !schemaStillValid(sub) {
			continue
		}
		payloadBytes, err := b.buildSubscriptionPayload(sub, evt, node)
		if err != nil {
			log.Printf("Failed to build payload for sub %s: %v", sub.ID, err)
			continue
		}
		if shouldBatch(sub.TargetFormat) && sub.BatchSize > 1 {
			if flushed := sub.enqueueBatch(payloadBytes); flushed != nil {
				b.enqueue(sub, flushed)
			}
			continue
		}
		b.enqueue(sub, payloadBytes)
	}
}

func shouldBatch(format string) bool {
	switch format {
	case "parquet", "hdf5", "avro":
		return true
	default:
		return false
	}
}

func (s *Subscription) enqueueBatch(item []byte) []byte {
	s.batchMu.Lock()
	defer s.batchMu.Unlock()
	s.batch = append(s.batch, item)
	if len(s.batch) >= s.BatchSize {
		out, _ := json.Marshal(map[string]any{
			"type":  "batch",
			"count": len(s.batch),
			"items": s.batch,
		})
		s.batch = nil
		return out
	}
	return nil
}

func (b *Broker) enqueue(sub *Subscription, payloadBytes []byte) {
	select {
	case sub.Queue <- payloadBytes:
	default:
		select {
		case <-sub.Queue:
			log.Printf("Backpressure: dropped oldest event for sub %s (at-most-once)", sub.ID)
		default:
		}
		select {
		case sub.Queue <- payloadBytes:
		default:
			log.Printf("Backpressure: dropped event for sub %s (queue still full)", sub.ID)
		}
	}
}

func schemaStillValid(sub *Subscription) bool {
	if sub.SchemaVersion == "" || sub.SchemaVersion == "unknown" {
		return true
	}
	meta, ok := registry.Default.GetVersion(sub.SchemaName, sub.SchemaVersion)
	if !ok || meta == nil {
		sub.Lifecycle = SchemaMissing
		return false
	}
	if meta.Deprecated {
		sub.Lifecycle = SchemaDeprecated
	}
	return true
}

func (b *Broker) buildSubscriptionPayload(sub *Subscription, evt Event, eventRoot *uir.Node) ([]byte, error) {
	var finalNode *uir.Node
	var err error

	if sub.SchemaVersion != "unknown" && sub.SchemaVersion != "" {
		finalNode, err = b.projectEventToNode(sub, evt.Type, eventRoot)
		if err != nil {
			return nil, err
		}
	} else {
		finalNode = eventRoot
	}

	opts := codec.Options{}
	if sub.SchemaVersion != "" && sub.SchemaVersion != "unknown" {
		if meta, ok := registry.Default.GetVersion(sub.SchemaName, sub.SchemaVersion); ok {
			opts.Schema = meta.Root
		}
	}

	encodedBytes, err := codec.EncodePayload(sub.TargetFormat, finalNode, opts)
	if err != nil {
		return nil, fmt.Errorf("encoding failed for %s: %v", sub.TargetFormat, err)
	}

	switch sub.TargetFormat {
	case "graphql":
		return wrapGraphQLTransport(sub, evt, encodedBytes)
	case "json":
		return wrapJSONTransport(sub, evt, encodedBytes)
	default:
		return wrapBinaryTransport(sub, evt, encodedBytes)
	}
}

func wrapGraphQLTransport(sub *Subscription, evt Event, encoded []byte) ([]byte, error) {
	var gqlPayload map[string]any
	if err := json.Unmarshal(encoded, &gqlPayload); err != nil {
		gqlPayload = map[string]any{"data": json.RawMessage(encoded)}
	}
	dataMap, _ := gqlPayload["data"].(map[string]any)
	key := sub.ResponseKey
	if key == "" {
		key = evt.Type
	}
	inner := any(dataMap)
	if dataMap != nil {
		if v, ok := dataMap[evt.Type]; ok {
			inner = v
		}
	}
	result := map[string]any{
		"data": map[string]any{
			key: inner,
		},
	}
	envelope := map[string]any{
		"id":      sub.ID,
		"type":    "next",
		"payload": result,
		"extensions": map[string]any{
			"eventId":       evt.ID,
			"cursor":        evt.Cursor,
			"schemaVersion": sub.SchemaVersion,
			"correlationId": sub.CorrelationID,
		},
	}
	return json.Marshal(envelope)
}

func wrapJSONTransport(sub *Subscription, evt Event, encoded []byte) ([]byte, error) {
	envelope := map[string]any{
		"id":   sub.ID,
		"type": "next",
		"payload": map[string]any{
			"data": map[string]any{
				evt.Type: json.RawMessage(encoded),
			},
		},
		"extensions": map[string]any{
			"eventId": evt.ID,
			"cursor":  evt.Cursor,
		},
	}
	return json.Marshal(envelope)
}

func wrapBinaryTransport(sub *Subscription, evt Event, encoded []byte) ([]byte, error) {
	meta := map[string]string{
		"format":        sub.TargetFormat,
		"schema":        sub.SchemaName,
		"schemaVersion": sub.SchemaVersion,
		"eventId":       evt.ID,
		"eventType":     evt.Type,
		"encoding":      "base64",
	}
	for k, v := range sub.BinaryMeta {
		meta[k] = v
	}
	envelope := map[string]any{
		"id":       sub.ID,
		"type":     "next",
		"format":   sub.TargetFormat,
		"metadata": meta,
		"payload":  base64.StdEncoding.EncodeToString(encoded),
	}
	return json.Marshal(envelope)
}

func (b *Broker) projectEventToNode(sub *Subscription, eventType string, eventRoot *uir.Node) (*uir.Node, error) {
	schemaMeta, found := registry.Default.GetVersion(sub.SchemaName, sub.SchemaVersion)
	if !found {
		return nil, fmt.Errorf("schema %s version %s not found in registry", sub.SchemaName, sub.SchemaVersion)
	}

	schemaTarget := resolveEventType(schemaMeta.Root, eventType)
	if schemaTarget == nil {
		return nil, fmt.Errorf("schema %s has no matching type definition for event '%s'", sub.SchemaName, eventType)
	}

	opts := uir.DefaultProjectOptions()
	opts.UnknownFields = uir.UnknownFieldIgnore
	opts.EmitNullForMissing = true
	opts.SchemaRoot = schemaMeta.Root
	projected, err := uir.Project(eventRoot, schemaTarget, opts)
	if err != nil {
		return nil, fmt.Errorf("schema validation failed: %w", err)
	}

	if len(sub.RequestedFields) > 0 {
		projected = filterBySelection(projected, sub.RequestedFields, sub.Fragments)
	}
	return projected, nil
}

func resolveEventType(root *uir.Node, eventType string) *uir.Node {
	if root == nil {
		return nil
	}
	if direct := root.FindNamedType(eventType); direct != nil && direct.Annotation("kind") != "service" {
		if direct.Type == uir.TypeMap || direct.Type == uir.TypeInterface || direct.Type == uir.TypeUnion {
			return direct
		}
	}
	var subscription *uir.Node
	for _, child := range root.Children {
		if strings.EqualFold(child.Key, "Subscription") || child.Annotation("kind") == "schema" && child.Annotation("subscription") != "" {
			if strings.EqualFold(child.Key, "Subscription") {
				subscription = child
				break
			}
		}
	}
	if subscription == nil {
		for _, child := range root.Children {
			if strings.EqualFold(child.Key, "Subscription") {
				subscription = child
			}
		}
	}
	if subscription != nil {
		if field := subscription.ChildByKey(eventType); field != nil {
			named := field.Annotation("gql_type")
			if named != "" {
				if t := root.FindNamedType(named); t != nil {
					return t
				}
			}
			return field
		}
	}
	for _, child := range root.Children {
		if strings.EqualFold(child.Key, eventType) {
			return child
		}
	}
	return nil
}

func ResolveType(root *uir.Node, typeName string) *uir.Node {
	if root == nil {
		return nil
	}
	if typeName == "" {
		return nil
	}
	return root.FindNamedType(typeName)
}

func SelectOperation(doc *ast.GraphQLDocument, operationName string) (*ast.GraphQLOperation, error) {
	var ops []*ast.GraphQLOperation
	for _, d := range doc.Definitions {
		if op, ok := d.(*ast.GraphQLOperation); ok {
			ops = append(ops, op)
		}
	}
	if len(ops) == 0 {
		return nil, fmt.Errorf("no operation in document")
	}
	if operationName != "" {
		for _, op := range ops {
			if op.Name == operationName {
				return op, nil
			}
		}
		return nil, fmt.Errorf("operation %q not found", operationName)
	}
	if len(ops) > 1 {
		return nil, fmt.Errorf("multiple operations present; provide operationName")
	}
	return ops[0], nil
}
