package codec

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	"omni-schema/internal/uir"
)

// GenerateGraphQL takes a UIR Node graph and synthesizes valid GraphQL SDL.
func GenerateGraphQL(n *uir.Node) ([]byte, error) {
	var builder strings.Builder
	
	// Collect all distinct types to ensure flat definitions
	typeDefs := make(map[string]*uir.Node)
	collectTypes(n, typeDefs)

	// Sort type names for determinism
	var names []string
	for name := range typeDefs {
		names = append(names, name)
	}
	sort.Strings(names)

	// Always ensure Root is at the top if it exists
	for _, name := range names {
		if name == "Root" || name == "graphql_root" || name == "proto_root" {
			writeGraphQLTypeDefinition(&builder, name, typeDefs[name])
		}
	}
	for _, name := range names {
		if name != "Root" && name != "graphql_root" && name != "proto_root" {
			writeGraphQLTypeDefinition(&builder, name, typeDefs[name])
		}
	}

	return []byte(builder.String()), nil
}

func collectTypes(n *uir.Node, typeDefs map[string]*uir.Node) {
	if n.Type == uir.TypeMap {
		// Use node key as type name, or a default
		name := n.Key
		if name == "" {
			name = "AnonymousType"
		}
		
		// Capitalize first letter for GraphQL type conventions if needed
		if len(name) > 0 {
			name = strings.ToUpper(name[:1]) + name[1:]
		}
		
		// If it's a root wrapper (e.g. from protobuf or graphql doc), we might just want to process its children as types
		// But in our UIR, a TypeMap is an object type.
		if n.TypeAnnotations["kind"] == "message" || n.TypeAnnotations["kind"] == "type" || n.Key == "graphql_root" || n.Key == "proto_root" {
			if n.Key == "graphql_root" || n.Key == "proto_root" {
				// The children of root are the actual types
				for _, child := range n.Children {
					collectTypes(child, typeDefs)
				}
				return // Don't add root itself as a GraphQL type if it's just a file wrapper
			}
			typeDefs[name] = n
		} else {
			// If it's a nested TypeMap, add it as well
			typeDefs[name] = n
		}
	}

	for _, child := range n.Children {
		collectTypes(child, typeDefs)
	}
}

func writeGraphQLTypeDefinition(builder *strings.Builder, name string, n *uir.Node) {
	builder.WriteString(fmt.Sprintf("type %s {\n", name))
	
	// Sort fields for determinism
	var fields []*uir.Node
	fields = append(fields, n.Children...)
	sort.Slice(fields, func(i, j int) bool {
		return fields[i].Key < fields[j].Key
	})

	for _, field := range fields {
		writeGraphQLField(builder, field, 2)
	}
	builder.WriteString("}\n\n")
}

func writeGraphQLField(builder *strings.Builder, child *uir.Node, indent int) {
	padding := strings.Repeat(" ", indent)
	
	var typeStr string
	switch child.Type {
	case uir.TypeArray:
		elemType := resolveScalarType(child.ElementType, child)
		typeStr = fmt.Sprintf("[%s]", elemType)
	case uir.TypeMap:
		typeName := child.Key
		if typeName != "" {
			typeStr = strings.ToUpper(typeName[:1]) + typeName[1:]
		} else {
			typeStr = "UnknownObject"
		}
	default:
		typeStr = resolveScalarType(child.Type, child)
	}

	nonNull := "!"
	if child.TypeAnnotations["nonNull"] == "true" {
		nonNull = "!"
	} else if child.TypeAnnotations["nonNull"] == "false" {
		nonNull = ""
	}

	// For simplified determinism in this prototype, default to non-null unless specifically marked.
	// We'll leave the ! on typeStr.
	if !strings.HasSuffix(typeStr, "!") {
		typeStr += nonNull
	}

	builder.WriteString(fmt.Sprintf("%s%s: %s\n", padding, child.Key, typeStr))
}

func resolveScalarType(t uir.UIRType, n *uir.Node) string {
	// If it has a specific gql_type or proto_type, we can use that mapping
	if gqlType, ok := n.TypeAnnotations["gql_type"]; ok {
		return gqlType
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
	case uir.TypeMap:
		if n != nil && n.Key != "" {
			return strings.ToUpper(n.Key[:1]) + n.Key[1:]
		}
		return "Object"
	default:
		return "String"
	}
}

// Format ensures the bytes are well formatted.
func Format(b []byte) []byte {
	return bytes.TrimSpace(b)
}
