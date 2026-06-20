package lower

import (
	"omni-schema/internal/ast"
	"omni-schema/internal/uir"
)

// LowerGraphQL syntax-directs the translation of the GraphQL AST down to the UIR.
func LowerGraphQL(doc *ast.GraphQLDocument) *uir.Node {
	root := uir.NewNode(uir.TypeMap, "graphql_root", nil)
	for _, def := range doc.Definitions {
		if op, ok := def.(*ast.GraphQLOperation); ok {
			opNode := uir.NewNode(uir.TypeMap, op.Name, nil)
			opNode.SetAnnotation("operation", op.OperationType)
			root.AddChild(opNode)
		}
	}
	return root
}
