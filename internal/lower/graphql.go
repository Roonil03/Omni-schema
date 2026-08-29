package lower

import (
	"omni-schema/internal/ast"
	"omni-schema/internal/uir"
)

func LowerGraphQL(doc *ast.GraphQLDocument) *uir.Node {
	root := uir.NewNode(uir.TypeMap, "graphql_root", nil)
	for _, def := range doc.Definitions {
		switch d := def.(type) {
		case *ast.GraphQLTypeDefinition:
			typeNode := uir.NewNode(uir.TypeMap, d.Name, nil)
			typeNode.SetAnnotation("kind", "type")
			if len(d.Implements) > 0 {
				typeNode.SetAnnotation("implements", joinComma(d.Implements))
			}
			lowerFields(typeNode, d.Fields)
			root.AddChild(typeNode)

		case *ast.GraphQLInterfaceDefinition:
			typeNode := uir.NewNode(uir.TypeInterface, d.Name, nil)
			typeNode.SetAnnotation("kind", "interface")
			lowerFields(typeNode, d.Fields)
			root.AddChild(typeNode)

		case *ast.GraphQLInputDefinition:
			typeNode := uir.NewNode(uir.TypeMap, d.Name, nil)
			typeNode.SetAnnotation("kind", "input")
			lowerFields(typeNode, d.Fields)
			root.AddChild(typeNode)

		case *ast.GraphQLEnumDefinition:
			typeNode := uir.NewNode(uir.TypeEnum, d.Name, nil)
			typeNode.SetAnnotation("kind", "enum")
			for _, val := range d.Values {
				typeNode.AddChild(uir.NewNode(uir.TypeString, val, val))
			}
			root.AddChild(typeNode)

		case *ast.GraphQLUnionDefinition:
			typeNode := uir.NewNode(uir.TypeUnion, d.Name, nil)
			typeNode.SetAnnotation("kind", "union")
			for _, t := range d.Types {
				ref := uir.NewNode(uir.TypeRef, t, nil)
				ref.SetAnnotation("gql_type", t)
				typeNode.AddChild(ref)
			}
			root.AddChild(typeNode)

		case *ast.GraphQLScalarDefinition:
			n := uir.NewNode(uir.TypeString, d.Name, nil)
			n.SetAnnotation("kind", "scalar")
			root.AddChild(n)

		case *ast.GraphQLSchemaDefinition:
			n := uir.NewNode(uir.TypeMap, "schema", nil)
			n.SetAnnotation("kind", "schema")
			n.SetAnnotation("query", d.Query)
			n.SetAnnotation("mutation", d.Mutation)
			n.SetAnnotation("subscription", d.Subscription)
			root.AddChild(n)

		case *ast.GraphQLFragmentDefinition:
			n := uir.NewNode(uir.TypeMap, d.Name, nil)
			n.SetAnnotation("kind", "fragment")
			n.SetAnnotation("on", d.TypeCond)
			root.AddChild(n)

		case *ast.GraphQLOperation:
			opNode := uir.NewNode(uir.TypeMap, d.Name, nil)
			opNode.SetAnnotation("operation", d.OperationType)
			root.AddChild(opNode)
		}
	}
	return root
}

func joinComma(ss []string) string {
	out := ""
	for i, s := range ss {
		if i > 0 {
			out += ","
		}
		out += s
	}
	return out
}

func lowerFields(parent *uir.Node, fields []*ast.GraphQLFieldDefinition) {
	for _, field := range fields {
		fieldNode := lowerTypeRef(field.Name, field.Type)
		if len(field.Arguments) > 0 {
			fieldNode.SetAnnotation("has_args", "true")
			for _, a := range field.Arguments {
				arg := lowerTypeRef(a.Name, a.Type)
				arg.SetAnnotation("kind", "argument")
				fieldNode.AddChild(arg)
			}
		}
		parent.AddChild(fieldNode)
	}
}

func lowerTypeRef(name string, t *ast.GraphQLTypeRef) *uir.Node {
	expr := typeRefToExpr(t)
	node := uir.NewNode(mapGraphQLType(namedOf(t)), name, nil)
	node.TypeExpr = expr
	if t != nil && t.IsNonNull {
		node.Cardinality = uir.CardinalityRequired
		node.SetAnnotation("nonNull", "true")
	}
	if t != nil && t.IsList {
		node.Type = uir.TypeArray
		if t.InnerType != nil {
			inner := lowerTypeRef("element", t.InnerType)
			node.ElementType = inner.Type
			node.SetAnnotation("gql_type", t.InnerType.NamedType)
			node.AddChild(inner)
		}
	} else if t != nil {
		node.SetAnnotation("gql_type", t.NamedType)
		if mapGraphQLType(t.NamedType) == uir.TypeMap {
			node.Type = uir.TypeRef
			node.SetAnnotation("kind", "ref")
		}
	}
	if expr != nil {
		node.SetAnnotation("gql_type_expr", expr.String())
	}
	return node
}

func typeRefToExpr(t *ast.GraphQLTypeRef) *uir.TypeExpr {
	if t == nil {
		return nil
	}
	var expr *uir.TypeExpr
	if t.IsList {
		expr = uir.ListOf(typeRefToExpr(t.InnerType))
	} else {
		expr = uir.NamedType(t.NamedType)
	}
	if t.IsNonNull {
		expr = uir.NonNullOf(expr)
	}
	return expr
}

func namedOf(t *ast.GraphQLTypeRef) string {
	for cur := t; cur != nil; cur = cur.InnerType {
		if cur.NamedType != "" {
			return cur.NamedType
		}
	}
	return ""
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
		return uir.TypeMap
	}
}
