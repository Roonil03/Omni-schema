package stream

import (
	"sync"
	"testing"

	"omni-schema/internal/ast"
	"omni-schema/internal/lexer"
)

func TestSubscriptionCloseIdempotent(t *testing.T) {
	s := NewSubscription()
	s.ID = "1"
	b := NewBroker()
	b.AddSubscription(s)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			b.RemoveSubscription(s)
			s.Close()
		}()
	}
	wg.Wait()
}

func TestSelectOperationAmbiguous(t *testing.T) {
	l := &lexer.GraphQLLexer{}
	doc, err := l.Parse(`subscription A { a { id } } subscription B { b { id } }`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = SelectOperation(doc, "")
	if err == nil {
		t.Fatal("expected ambiguity error")
	}
	op, err := SelectOperation(doc, "B")
	if err != nil || op.Name != "B" {
		t.Fatalf("%v %#v", err, op)
	}
	_ = ast.GraphQLOperation{}
}
