package lower

import (
	"omni-schema/internal/ast"
	"omni-schema/internal/uir"
)

// LowerGraphQL syntax-directs the translation of the GraphQL AST down to the UIR.
func LowerGraphQL(doc *ast.GraphQLDocument) *uir.Node {
	root := uir.NewNode(uir.TypeMap, "graphql_root", nil)
	for _, def := range doc.Definitions {
		switch d := def.(type) {
		case *ast.GraphQLTypeDefinition:
			typeNode := uir.NewNode(uir.TypeMap, d.Name, nil)
			typeNode.SetAnnotation("kind", "type")
			
			for _, field := range d.Fields {
				fieldNode := uir.NewNode(mapGraphQLType(field.Type, field.IsList), field.Name, nil)
				if field.IsList {
					fieldNode.ElementType = mapGraphQLType(field.Type, false)
				}
				if field.NonNull {
					fieldNode.SetAnnotation("nonNull", "true")
				}
				fieldNode.SetAnnotation("gql_type", field.Type)
				typeNode.AddChild(fieldNode)
			}
			root.AddChild(typeNode)

		case *ast.GraphQLOperation:
			opNode := uir.NewNode(uir.TypeMap, d.Name, nil)
			opNode.SetAnnotation("operation", d.OperationType)
			root.AddChild(opNode)
		}
	}
	return root
}

func mapGraphQLType(gqlType string, isList bool) uir.UIRType {
	if isList {
		return uir.TypeArray
	}
	switch gqlType {
	case "String", "ID":
		return uir.TypeString
	case "Int":
		return uir.TypeInt32
	case "Float":
		return uir.TypeFloat64
	case "Boolean":
		return uir.TypeBoolean
	default:
		// For object types, map to TypeMap
		return uir.TypeMap
	}
}
