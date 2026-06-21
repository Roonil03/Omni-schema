package codec

import (
	"fmt"
	"strings"

	"omni-schema/internal/uir"
)

// GenerateGraphQL takes a UIR Node graph (typically mapped from a payload)
// and synthesizes it into a valid GraphQL Type string.
func GenerateGraphQL(n *uir.Node) ([]byte, error) {
	var builder strings.Builder
	builder.WriteString("type Root {\n")
	writeGraphQLType(&builder, n, 2)
	builder.WriteString("}\n")
	return []byte(builder.String()), nil
}

func writeGraphQLType(builder *strings.Builder, n *uir.Node, indent int) {
	padding := strings.Repeat(" ", indent)
	for _, child := range n.Children {
		switch child.Type {
		case uir.TypeMap:
			builder.WriteString(fmt.Sprintf("%s%s: {\n", padding, child.Key))
			writeGraphQLType(builder, child, indent+2)
			builder.WriteString(fmt.Sprintf("%s}\n", padding))
		case uir.TypeArray:
			builder.WriteString(fmt.Sprintf("%s%s: [", padding, child.Key))
			if len(child.Children) > 0 {
				builder.WriteString(resolveScalarType(child.Children[0].Type))
			} else {
				builder.WriteString("String")
			}
			builder.WriteString("]\n")
		default:
			builder.WriteString(fmt.Sprintf("%s%s: %s\n", padding, child.Key, resolveScalarType(child.Type)))
		}
	}
}

func resolveScalarType(t uir.UIRType) string {
	switch t {
	case uir.TypeString:
		return "String!"
	case uir.TypeFloat64:
		return "Float!"
	case uir.TypeBoolean:
		return "Boolean!"
	case uir.TypeInt32, uir.TypeInt64:
		return "Int!"
	default:
		return "String"
	}
}
