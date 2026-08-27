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
			lowerFields(typeNode, d.Fields)
			root.AddChild(typeNode)

		case *ast.GraphQLInterfaceDefinition:
			typeNode := uir.NewNode(uir.TypeMap, d.Name, nil)
			typeNode.SetAnnotation("kind", "interface")
			lowerFields(typeNode, d.Fields)
			root.AddChild(typeNode)

		case *ast.GraphQLInputDefinition:
			typeNode := uir.NewNode(uir.TypeMap, d.Name, nil)
			typeNode.SetAnnotation("kind", "input")
			lowerFields(typeNode, d.Fields)
			root.AddChild(typeNode)

		case *ast.GraphQLEnumDefinition:
			typeNode := uir.NewNode(uir.TypeString, d.Name, nil)
			typeNode.SetAnnotation("kind", "enum")
			for _, val := range d.Values {
				typeNode.AddChild(uir.NewNode(uir.TypeString, val, nil))
			}
			root.AddChild(typeNode)

		case *ast.GraphQLUnionDefinition:
			typeNode := uir.NewNode(uir.TypeMap, d.Name, nil)
			typeNode.SetAnnotation("kind", "union")
			for _, t := range d.Types {
				typeNode.AddChild(uir.NewNode(uir.TypeMap, t, nil))
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

func lowerFields(parent *uir.Node, fields []*ast.GraphQLFieldDefinition) {
	for _, field := range fields {
		fieldNode := lowerTypeRef(field.Name, field.Type)
		parent.AddChild(fieldNode)
	}
}

func lowerTypeRef(name string, t *ast.GraphQLTypeRef) *uir.Node {
	if t.IsList {
		node := uir.NewNode(uir.TypeArray, name, nil)
		if t.IsNonNull {
			node.SetAnnotation("nonNull", "true")
		}
		
		// To represent the inner element type, we can create a dummy child
		// or set the ElementType. UIR currently expects a primitive ElementType.
		// For nested lists or objects, we attach an annotation for the deep type.
		innerNode := lowerTypeRef("element", t.InnerType)
		node.ElementType = innerNode.Type
		node.SetAnnotation("gql_type", t.InnerType.NamedType)
		node.AddChild(innerNode)
		return node
	}

	node := uir.NewNode(mapGraphQLType(t.NamedType), name, nil)
	if t.IsNonNull {
		node.SetAnnotation("nonNull", "true")
	}
	node.SetAnnotation("gql_type", t.NamedType)
	return node
}

func mapGraphQLType(gqlType string) uir.UIRType {
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
