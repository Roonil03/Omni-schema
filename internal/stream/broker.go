package stream

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"omni-schema/internal/ast"
	"omni-schema/internal/codec"
	"omni-schema/internal/network"
	"omni-schema/internal/registry"
	"omni-schema/internal/uir"
)

type Broker struct {
	mu            sync.RWMutex
	subscriptions map[*Subscription]bool
	eventSeq      atomic.Uint64
	order         string
}

var (
	ErrBuffering  = errors.New("batch buffering")
	DefaultBroker = NewBroker()
)

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
		sub.Queue = make(chan Frame, 100)
	}
	b.subscriptions[sub] = true
	log.Printf("Added subscription %s schema=%s:%s format=%s delivery=%s replay=%s", sub.ID, sub.SchemaName, sub.SchemaVersion, sub.TargetFormat, DeliveryMode, ReplayMode)
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
		if !sub.Remember(evt.ID) {
			continue
		}
		if !schemaStillValid(sub) {
			b.enqueue(sub, Frame{Opcode: network.OpText, Payload: errorJSON(sub.ID, "schema version missing or deleted")})
			sub.Close()
			continue
		}
		srcFmt := evt.SourceFormat
		if sub.SourceFormat != "" {
			srcFmt = sub.SourceFormat
		}
		decOpts := codec.Options{Schema: sub.SourceSchema, TypeName: sub.SourceType}
		node, err := codec.DecodePayload(srcFmt, evt.Payload, decOpts)
		if err != nil {
			log.Printf("decode event for sub %s: %v", sub.ID, err)
			continue
		}
		rf, ok := sub.matchesEvent(evt.Type)
		if !ok {
			continue
		}
		frame, err := b.buildFrame(sub, evt, node, rf)
		if errors.Is(err, ErrBuffering) {
			continue
		}
		if err != nil {
			log.Printf("build payload for sub %s: %v", sub.ID, err)
			continue
		}
		b.enqueue(sub, frame)
	}
}

func errorJSON(id, msg string) []byte {
	b, _ := json.Marshal(map[string]any{"id": id, "type": "error", "payload": map[string]string{"message": msg}})
	return b
}

func (s *Subscription) appendBatch(n *uir.Node) []*uir.Node {
	s.batchMu.Lock()
	defer s.batchMu.Unlock()
	s.batchNodes = append(s.batchNodes, n)
	if len(s.batchNodes) >= s.BatchSize {
		out := s.batchNodes
		s.batchNodes = nil
		return out
	}
	return nil
}

func (b *Broker) enqueue(sub *Subscription, frame Frame) {
	select {
	case sub.Queue <- frame:
	default:
		select {
		case <-sub.Queue:
			log.Printf("Backpressure: dropped oldest event for sub %s (%s)", sub.ID, DeliveryMode)
		default:
		}
		select {
		case sub.Queue <- frame:
		default:
			log.Printf("Backpressure: dropped event for sub %s", sub.ID)
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

func (b *Broker) buildFrame(sub *Subscription, evt Event, eventRoot *uir.Node, rf RootField) (Frame, error) {
	finalNode := eventRoot
	var err error
	if sub.SchemaVersion != "unknown" && sub.SchemaVersion != "" {
		finalNode, err = b.projectEventToNode(sub, rf, eventRoot)
		if err != nil {
			return Frame{}, err
		}
	}

	if codec.IsContainerFormat(sub.TargetFormat) && sub.BatchSize > 1 {
		if flushed := sub.appendBatch(finalNode); flushed != nil {
			arr := uir.NewNode(uir.TypeArray, "Root", nil)
			for _, n := range flushed {
				arr.AddChild(n)
			}
			finalNode = arr
		} else {
			return Frame{}, ErrBuffering
		}
	}

	opts := codec.Options{Schema: sub.TargetSchema, TypeName: sub.TargetType}
	if opts.Schema == nil && sub.SchemaVersion != "" && sub.SchemaVersion != "unknown" {
		if meta, ok := registry.Default.GetVersion(sub.SchemaName, sub.SchemaVersion); ok {
			opts.Schema = meta.Root
		}
	}

	encoded, err := codec.EncodePayload(sub.TargetFormat, finalNode, opts)
	if err != nil {
		return Frame{}, err
	}

	if isTextFormat(sub.TargetFormat) {
		if sub.TargetFormat == "graphql" {
			payload, err := wrapGraphQLTransport(sub, evt, encoded, rf)
			return Frame{Opcode: network.OpText, Payload: payload}, err
		}
		payload, err := wrapJSONTransport(sub, evt, encoded)
		return Frame{Opcode: network.OpText, Payload: payload}, err
	}

	body := encodeBinaryEnvelope(sub, evt, encoded)
	return Frame{Opcode: network.OpBinary, Payload: body}, nil
}

func encodeBinaryEnvelope(sub *Subscription, evt Event, body []byte) []byte {
	hdr, _ := json.Marshal(map[string]string{
		"eventId":       evt.ID,
		"cursor":        evt.Cursor,
		"eventType":     evt.Type,
		"format":        sub.TargetFormat,
		"schema":        sub.SchemaName,
		"schemaVersion": sub.SchemaVersion,
	})
	out := make([]byte, 0, 7+len(hdr)+len(body))
	out = append(out, 'O', 'M', 'N', 'I', 1)
	var ln [2]byte
	binary.BigEndian.PutUint16(ln[:], uint16(len(hdr)))
	out = append(out, ln[:]...)
	out = append(out, hdr...)
	out = append(out, body...)
	return out
}

func DecodeBinaryEnvelope(frame []byte) (hdr map[string]string, body []byte, err error) {
	if len(frame) < 7 || string(frame[:4]) != "OMNI" {
		return nil, nil, fmt.Errorf("not an omni binary envelope")
	}
	n := int(binary.BigEndian.Uint16(frame[5:7]))
	if 7+n > len(frame) {
		return nil, nil, fmt.Errorf("truncated binary header")
	}
	hdr = map[string]string{}
	if err := json.Unmarshal(frame[7:7+n], &hdr); err != nil {
		return nil, nil, err
	}
	return hdr, frame[7+n:], nil
}

func wrapGraphQLTransport(sub *Subscription, evt Event, encoded []byte, rf RootField) ([]byte, error) {
	var gqlPayload map[string]any
	_ = json.Unmarshal(encoded, &gqlPayload)
	dataMap, _ := gqlPayload["data"].(map[string]any)
	key := rf.Alias
	if key == "" {
		key = rf.Name
	}
	inner := any(dataMap)
	if dataMap != nil {
		if v, ok := dataMap[evt.Type]; ok {
			inner = v
		}
	}
	result := map[string]any{"data": map[string]any{key: inner}}
	envelope := map[string]any{
		"id":      sub.ID,
		"type":    "next",
		"payload": result,
		"extensions": map[string]any{
			"eventId": evt.ID, "cursor": evt.Cursor, "schemaVersion": sub.SchemaVersion,
		},
	}
	return json.Marshal(envelope)
}

func wrapJSONTransport(sub *Subscription, evt Event, encoded []byte) ([]byte, error) {
	envelope := map[string]any{
		"id":   sub.ID,
		"type": "next",
		"payload": map[string]any{
			"data": map[string]any{evt.Type: json.RawMessage(encoded)},
		},
		"extensions": map[string]any{"eventId": evt.ID, "cursor": evt.Cursor},
	}
	return json.Marshal(envelope)
}

func (b *Broker) projectEventToNode(sub *Subscription, rf RootField, eventRoot *uir.Node) (*uir.Node, error) {
	schemaMeta, found := registry.Default.GetVersion(sub.SchemaName, sub.SchemaVersion)
	if !found {
		return nil, fmt.Errorf("schema %s version %s not found", sub.SchemaName, sub.SchemaVersion)
	}
	var schemaTarget *uir.Node
	var err error
	if sub.TargetFormat == "graphql" {
		schemaTarget, err = ReturnTypeForEvent(schemaMeta.Root, rf.Name)
		if err != nil {
			return nil, err
		}
	} else {
		name := sub.TargetType
		if name == "" {
			name = rf.Name
		}
		schemaTarget, err = uir.ResolvePayloadType(schemaMeta.Root, name)
		if err != nil {
			return nil, err
		}
	}
	opts := uir.DefaultProjectOptions()
	opts.UnknownFields = uir.UnknownFieldIgnore
	opts.EmitNullForMissing = true
	opts.SchemaRoot = schemaMeta.Root
	key := uir.PlanCacheKey(sub.SchemaName, rf.Name, sub.SchemaName, schemaTarget.Key, sub.SchemaVersion, sub.SchemaVersion)
	plan := uir.GetOrCompilePlan(key, eventRoot, schemaTarget, opts)
	projected, err := uir.ApplyPlan(eventRoot, plan)
	if err != nil {
		return nil, err
	}
	sels := rf.Selections
	if len(sels) == 0 {
		sels = sub.RequestedFields
	}
	if len(sels) > 0 {
		projected = filterBySelection(projected, sels, sub.Fragments)
	}
	return projected, nil
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
