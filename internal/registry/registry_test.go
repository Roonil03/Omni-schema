package registry

import (
	"os"
	"path/filepath"
	"testing"

	"omni-schema/internal/lexer"
	"omni-schema/internal/lower"
)

func TestLoadFailsOnUnknownFormat(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "reg.json")
	os.WriteFile(p, []byte(`{"format_version":2,"active":{"x":"v"},"schemas":[{"name":"x","version":"v","format":"unknown","raw_content":"e30="}]}`), 0644)
	r := NewRegistry()
	if err := r.LoadFromFile(p); err == nil {
		t.Fatal("expected reconstruction failure")
	}
}

func TestRecursiveDiff(t *testing.T) {
	l := &lexer.GraphQLLexer{}
	a, _ := l.Parse(`type User { id: ID! profile: Profile } type Profile { bio: String }`)
	b, _ := l.Parse(`type User { id: ID! profile: Profile } type Profile { bio: String email: String }`)
	d := Diff(lower.LowerGraphQL(a), lower.LowerGraphQL(b))
	if len(d["added"]) == 0 {
		t.Fatalf("expected nested add, got %v", d)
	}
}
