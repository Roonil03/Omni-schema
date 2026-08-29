package stream

import (
	"sync"
	"sync/atomic"
	"time"

	"omni-schema/internal/ast"
	"omni-schema/internal/network"
	"omni-schema/internal/uir"
)

type SchemaLifecycle string

const (
	SchemaBound        SchemaLifecycle = "bound"
	SchemaMissing      SchemaLifecycle = "missing"
	SchemaDeprecated   SchemaLifecycle = "deprecated"
	SchemaIncompatible SchemaLifecycle = "incompatible"
)

type Subscription struct {
	ID              string
	Conn            *network.Conn
	SchemaName      string
	SchemaVersion   string
	RequestedFields []ast.GraphQLSelection
	Fragments       map[string]*ast.GraphQLFragmentDefinition
	RootFields      []RootField
	ResponseKey     string
	EventType       string
	Queue           chan Frame
	Closed          chan struct{}
	closeOnce       sync.Once
	TargetFormat    string
	SourceFormat    string
	SourceSchema    *uir.Node
	TargetSchema    *uir.Node
	SourceType      string
	TargetType      string
	CorrelationID   string
	BatchSize       int
	FlushInterval   time.Duration
	batchMu         sync.Mutex
	batchNodes      []*uir.Node
	OpName          string
	Lifecycle       SchemaLifecycle
	Seen            map[string]struct{}
	seenMu          sync.Mutex
	Tenant          string
	seq             atomic.Uint64
}

func NewSubscription() *Subscription {
	return &Subscription{
		Queue:         make(chan Frame, 100),
		Closed:        make(chan struct{}),
		Fragments:     map[string]*ast.GraphQLFragmentDefinition{},
		Seen:          map[string]struct{}{},
		BatchSize:     1,
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

func (s *Subscription) Remember(id string) bool {
	if id == "" {
		return true
	}
	s.seenMu.Lock()
	defer s.seenMu.Unlock()
	if _, ok := s.Seen[id]; ok {
		return false
	}
	if len(s.Seen) > 4096 {
		s.Seen = map[string]struct{}{}
	}
	s.Seen[id] = struct{}{}
	return true
}

func (s *Subscription) matchesEvent(eventType string) (RootField, bool) {
	if len(s.RootFields) == 0 {
		if s.EventType == "" || s.EventType == eventType {
			return RootField{Name: eventType, Alias: s.ResponseKey}, true
		}
		return RootField{}, false
	}
	for _, rf := range s.RootFields {
		if rf.Name == eventType {
			return rf, true
		}
	}
	return RootField{}, false
}
