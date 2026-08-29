package stream

import (
	"testing"

	"omni-schema/internal/codec"
	"omni-schema/internal/lexer"
	"omni-schema/internal/lower"
	"omni-schema/internal/network"
)

func TestSelectSubscriptionRejectsQuery(t *testing.T) {
	l := &lexer.GraphQLLexer{}
	doc, err := l.Parse(`query Q { a { id } }`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := SelectSubscription(doc, ""); err == nil {
		t.Fatal("expected rejection")
	}
}

func TestMalformedOperationRejected(t *testing.T) {
	l := &lexer.GraphQLLexer{}
	if _, err := l.Parse(`subscription {`); err == nil {
		t.Fatal("expected parse error")
	}
}

func TestBinaryEnvelopeNotBase64(t *testing.T) {
	sub := NewSubscription()
	sub.TargetFormat = "msgpack"
	sub.SchemaName = "s"
	body := []byte{0x80}
	frame := encodeBinaryEnvelope(sub, Event{ID: "1", Type: "e", Cursor: "1"}, body)
	if frame[0] != 'O' {
		t.Fatal("expected OMNI magic")
	}
	hdr, got, err := DecodeBinaryEnvelope(frame)
	if err != nil {
		t.Fatal(err)
	}
	if hdr["format"] != "msgpack" || string(got) != string(body) {
		t.Fatalf("%v %v", hdr, got)
	}
}

func TestStreamingMatrix(t *testing.T) {
	srcJSON := []byte(`{"name":"Ada","id":42}`)
	node, err := codec.DecodePayload("json", srcJSON, codec.Options{})
	if err != nil {
		t.Fatal(err)
	}
	for _, tgt := range codec.AdvertisedFormats {
		sub := NewSubscription()
		sub.ID = "1"
		sub.TargetFormat = tgt
		if codec.IsContainerFormat(tgt) {
			sub.BatchSize = 1
		}
		b := NewBroker()
		frame, err := b.buildFrame(sub, Event{ID: "e1", Type: "ev", SourceFormat: "json"}, node, RootField{Name: "ev", Alias: "ev"})
		if err != nil {
			t.Fatalf("%s: %v", tgt, err)
		}
		if codec.IsBinaryFormat(tgt) && tgt != "odata" {
			if frame.Opcode != network.OpBinary {
				t.Fatalf("%s expected OpBinary got %v", tgt, frame.Opcode)
			}
		}
	}
}

func TestEventDedup(t *testing.T) {
	s := NewSubscription()
	if !s.Remember("a") || s.Remember("a") {
		t.Fatal("dedup")
	}
}

func TestGraphQLFieldValidation(t *testing.T) {
	l := &lexer.GraphQLLexer{}
	doc, err := l.Parse(`
type Transaction { id: ID! status: String! }
type Subscription { transactionUpdated: Transaction }
`)
	if err != nil {
		t.Fatal(err)
	}
	root := lower.LowerGraphQL(doc)
	opDoc, err := l.Parse(`subscription { transactionUpdated { nope } }`)
	if err != nil {
		t.Fatal(err)
	}
	op, _ := SelectSubscription(opDoc, "")
	rf, err := RootFieldsFromOp(op, root)
	if err != nil {
		t.Fatal(err)
	}
	ret, err := ReturnTypeForEvent(root, rf[0].Name)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateSelections(ret, rf[0].Selections, nil); err == nil {
		t.Fatal("expected unknown field")
	}
}
