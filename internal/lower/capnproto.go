package lower

import (
	"omni-schema/internal/ast"
	"omni-schema/internal/uir"
)

// LowerCapnProto maps Cap'n Proto definitions down to the Universal Intermediate Representation.
func LowerCapnProto(file *ast.CapnProtoFile) *uir.Node {
	root := uir.NewNode(uir.TypeMap, "capnp_root", nil)
	for _, s := range file.Structs {
		node := uir.NewNode(uir.TypeMap, s.Name, nil)
		node.SetAnnotation("type", "struct")
		root.AddChild(node)
	}
	return root
}
