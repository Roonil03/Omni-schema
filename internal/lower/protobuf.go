package lower

import (
	"omni-schema/internal/ast"
	"omni-schema/internal/uir"
)

// LowerProtobuf syntax-directs the translation of a parsed .proto file down to the UIR.
func LowerProtobuf(file *ast.ProtoFile) *uir.Node {
	root := uir.NewNode(uir.TypeMap, "proto_root", nil)
	for _, msg := range file.Messages {
		msgNode := uir.NewNode(uir.TypeMap, msg.Name, nil)
		msgNode.SetAnnotation("type", "message")
		root.AddChild(msgNode)
	}
	return root
}
