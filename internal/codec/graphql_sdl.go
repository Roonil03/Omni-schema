package codec

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	"omni-schema/internal/uir"
)

// GenerateGraphQLSDL takes a UIR Node graph and synthesizes valid, flat GraphQL SDL.
// Every nested object becomes a separate named type definition. Type names are derived
// from schema annotations when available, or synthesized deterministically from the
// parent-child path when processing untyped data (e.g. JSON inference).
func GenerateGraphQLSDL(n *uir.Node) ([]byte, error) {
	var builder strings.Builder

	// Collect all distinct type definitions into a flat map.
	typeDefs := make(map[string]*uir.Node)
	collectTypes(n, typeDefs, "")

	// Sort type names for deterministic output.
	var names []string
	for name := range typeDefs {
		names = append(names, name)
	}
	sort.Strings(names)

	// Emit root-level types first (Root, or schema doc roots), then the rest.
	var rootNames, otherNames []string
	for _, name := range names {
		if name == "Root" || strings.HasSuffix(name, "_root") {
			rootNames = append(rootNames, name)
		} else {
			otherNames = append(otherNames, name)
		}
	}
	ordered := append(rootNames, otherNames...)

	for _, name := range ordered {
		writeGraphQLTypeDefinition(&builder, name, typeDefs[name], typeDefs)
	}

	return []byte(builder.String()), nil
}

// collectTypes performs a depth-first walk of the UIR graph and registers every
// TypeMap node as a named GraphQL type. For nodes that come from parsed schemas
// (annotated with kind=type or kind=message), the node's own Key is used as the
// type name. For nodes inferred from JSON data, a deterministic name is synthesised
// as ParentTypeName + "_" + capitalize(fieldKey) to avoid collisions.
func collectTypes(n *uir.Node, typeDefs map[string]*uir.Node, parentTypeName string) {
	if n.Type != uir.TypeMap {
		return
	}

	// Determine the type name for this node.
	typeName := resolveTypeName(n, parentTypeName)

	// Document-root wrappers (graphql_root, proto_root) are not emitted as types
	// themselves; only their children become types.
	if n.Key == "graphql_root" || n.Key == "proto_root" {
		for _, child := range n.Children {
			collectTypes(child, typeDefs, "")
		}
		return
	}

	// Register this node as a type.
	typeDefs[typeName] = n

	// Recurse into children that are themselves TypeMap (nested objects).
	for _, child := range n.Children {
		if child.Type == uir.TypeMap && len(child.Children) > 0 {
			collectTypes(child, typeDefs, typeName)
		}
	}
}

// resolveTypeName determines the GraphQL type name for a UIR node.
func resolveTypeName(n *uir.Node, parentTypeName string) string {
	kind := n.TypeAnnotations["kind"]

	// Schema-derived nodes already have a proper type name.
	if kind == "type" || kind == "message" {
		return capitalize(n.Key)
	}

	// The root node from JSON parsing.
	if n.Key == "Root" && parentTypeName == "" {
		return "Root"
	}

	// Nested objects inferred from JSON: synthesise a deterministic name.
	if parentTypeName != "" {
		return parentTypeName + "_" + capitalize(n.Key)
	}

	return capitalize(n.Key)
}

func writeGraphQLTypeDefinition(builder *strings.Builder, name string, n *uir.Node, allTypes map[string]*uir.Node) {
	builder.WriteString(fmt.Sprintf("type %s {\n", name))

	// Sort fields for determinism.
	fields := make([]*uir.Node, len(n.Children))
	copy(fields, n.Children)
	sort.Slice(fields, func(i, j int) bool {
		return fields[i].Key < fields[j].Key
	})

	for _, field := range fields {
		writeGraphQLField(builder, field, name, allTypes)
	}
	builder.WriteString("}\n\n")
}

func writeGraphQLField(builder *strings.Builder, child *uir.Node, parentTypeName string, allTypes map[string]*uir.Node) {
	var typeStr string

	switch child.Type {
	case uir.TypeArray:
		elemTypeStr := resolveFieldTypeString(child.ElementType, child, parentTypeName, allTypes)
		typeStr = fmt.Sprintf("[%s]", elemTypeStr)
	case uir.TypeMap:
		// Reference the named type that was registered for this nested object.
		typeStr = resolveNestedTypeName(child, parentTypeName, allTypes)
	default:
		typeStr = resolveScalarTypeString(child.Type, child)
	}

	// Determine nullability.
	nonNull := "!"
	if child.TypeAnnotations["nonNull"] == "false" {
		nonNull = ""
	}
	if !strings.HasSuffix(typeStr, "!") {
		typeStr += nonNull
	}

	builder.WriteString(fmt.Sprintf("  %s: %s\n", child.Key, typeStr))
}

// resolveNestedTypeName finds the registered type name for a nested TypeMap field.
func resolveNestedTypeName(child *uir.Node, parentTypeName string, allTypes map[string]*uir.Node) string {
	// First try schema-derived name (kind=type or kind=message).
	kind := child.TypeAnnotations["kind"]
	if kind == "type" || kind == "message" {
		return capitalize(child.Key)
	}

	// Otherwise use the synthesised parent_Child name.
	candidate := parentTypeName + "_" + capitalize(child.Key)
	if _, ok := allTypes[candidate]; ok {
		return candidate
	}

	// Fallback: capitalised key.
	return capitalize(child.Key)
}

// resolveFieldTypeString resolves the GraphQL type string for an element type
// (used inside arrays).
func resolveFieldTypeString(t uir.UIRType, n *uir.Node, parentTypeName string, allTypes map[string]*uir.Node) string {
	if t == uir.TypeMap {
		return resolveNestedTypeName(n, parentTypeName, allTypes)
	}
	return resolveScalarTypeString(t, n)
}

// resolveScalarTypeString maps a UIR scalar type to its GraphQL equivalent.
func resolveScalarTypeString(t uir.UIRType, n *uir.Node) string {
	// Prefer explicit gql_type annotation if set during schema lowering.
	if n != nil {
		if gqlType, ok := n.TypeAnnotations["gql_type"]; ok {
			return gqlType
		}
	}

	switch t {
	case uir.TypeString:
		return "String"
	case uir.TypeFloat64:
		return "Float"
	case uir.TypeBoolean:
		return "Boolean"
	case uir.TypeInt32, uir.TypeInt64:
		return "Int"
	default:
		return "String"
	}
}

func capitalize(s string) string {
	if len(s) == 0 {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// Format ensures the bytes are well formatted.
func Format(b []byte) []byte {
	return bytes.TrimSpace(b)
}
