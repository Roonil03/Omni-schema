package stream

import (
	"sync"
	"sync/atomic"
	"time"

	"omni-schema/internal/ast"
	"omni-schema/internal/network"
)

// DeliveryMode is at-most-once / best-effort. The bounded queue uses DropOldest.
const DeliveryMode = "at-most-once/best-effort"

type SchemaLifecycle string

const (
	SchemaBound       SchemaLifecycle = "bound"
	SchemaMissing     SchemaLifecycle = "missing"
	SchemaDeprecated  SchemaLifecycle = "deprecated"
	SchemaIncompatible SchemaLifecycle = "incompatible"
)

type Subscription struct {
	ID              string
	Conn            *network.Conn
	SchemaName      string
	SchemaVersion   string
	RequestedFields []ast.GraphQLSelection
	Fragments       map[string]*ast.GraphQLFragmentDefinition
	ResponseKey     string
	EventType       string
	Queue           chan []byte
	Closed          chan struct{}
	closeOnce       sync.Once
	TargetFormat    string
	SourceFormat    string
	SchemaVersionID string
	CorrelationID   string
	BinaryMeta      map[string]string
	BatchSize       int
	FlushInterval   time.Duration
	batchMu         sync.Mutex
	batch           [][]byte
	batchNodes      []*batchItem
	OpName          string
	Lifecycle       SchemaLifecycle

	seq atomic.Uint64
}

type batchItem struct {
	eventType string
	payload   []byte
}

func NewSubscription() *Subscription {
	return &Subscription{
		Queue:         make(chan []byte, 100),
		Closed:        make(chan struct{}),
		Fragments:     map[string]*ast.GraphQLFragmentDefinition{},
		BinaryMeta:    map[string]string{},
		BatchSize:     1,
		FlushInterval: 0,
		Lifecycle:     SchemaBound,
	}
}

func (s *Subscription) Close() {
	if s == nil {
		return
	}
	s.closeOnce.Do(func() {
		close(s.Closed)
	})
}

func (s *Subscription) NextEventID() string {
	n := s.seq.Add(1)
	return s.ID + ":" + itoa(n)
}

func itoa(n uint64) string {
	if n == 0 {
		return "0"
	}
	var a [20]byte
	i := len(a)
	for n > 0 {
		i--
		a[i] = byte('0' + n%10)
		n /= 10
	}
	return string(a[i:])
}
