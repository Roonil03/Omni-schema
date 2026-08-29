package lexer

import "testing"

func TestCapnProtoStructFields(t *testing.T) {
	l := &CapnProtoLexer{}
	f, err := l.Parse(`struct Person { name @0 :Text; age @1 :Int32; }`)
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Structs) != 1 || f.Structs[0].Name != "Person" {
		t.Fatalf("%+v", f.Structs)
	}
	if len(f.Structs[0].Fields) != 2 {
		t.Fatalf("fields %+v", f.Structs[0].Fields)
	}
}
