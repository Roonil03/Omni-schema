package uir

import "testing"

func TestTypeExprNestedLists(t *testing.T) {
	expr := ParseGraphQLTypeExpr("[[User!]!]!")
	if expr.String() != "[[User!]!]!" {
		t.Fatalf("got %s", expr.String())
	}
	if expr.NamedLeaf() != "User" {
		t.Fatalf("leaf %s", expr.NamedLeaf())
	}
	if !expr.IsNonNull() {
		t.Fatal("outer should be non-null")
	}
}

func TestFloatToIntStrict(t *testing.T) {
	n := NewNode(TypeFloat64, "x", 12.9)
	_, _, err := coerceValue(n, TypeInt32, ProjectOptions{Lossiness: LossyStrict})
	if err == nil {
		t.Fatal("expected precision loss error")
	}
}

func TestUint64ToInt64Overflow(t *testing.T) {
	n := NewNode(TypeUInt64, "x", uint64(^uint64(0)))
	_, _, err := coerceValue(n, TypeInt64, ProjectOptions{Lossiness: LossyStrict})
	if err == nil {
		t.Fatal("expected overflow")
	}
}

func TestMissingRequired(t *testing.T) {
	schema := NewNode(TypeMap, "User", nil)
	id := NewNode(TypeString, "id", nil)
	id.SetAnnotation("nonNull", "true")
	schema.AddChild(id)
	data := NewNode(TypeMap, "User", nil)
	_, err := Project(data, schema, ProjectOptions{Lossiness: LossyStrict})
	if err == nil {
		t.Fatal("expected missing required")
	}
}

func TestDefaultValue(t *testing.T) {
	schema := NewNode(TypeMap, "User", nil)
	role := NewNode(TypeString, "role", nil)
	role.SetAnnotation("default", "guest")
	schema.AddChild(role)
	data := NewNode(TypeMap, "User", nil)
	out, err := Project(data, schema, ProjectOptions{Lossiness: LossySafe})
	if err != nil {
		t.Fatal(err)
	}
	if out.ChildByKey("role").Value != "guest" {
		t.Fatalf("default not applied: %#v", out.ChildByKey("role"))
	}
}
