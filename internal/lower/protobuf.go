package lower

import (
	"strconv"
	"omni-schema/internal/ast"
	"omni-schema/internal/uir"
)

// LowerProtobuf syntax-directs the translation of a parsed .proto file down to the UIR.
func LowerProtobuf(file *ast.ProtoFile) *uir.Node {
	root := uir.NewNode(uir.TypeMap, "proto_root", nil)
	for _, msg := range file.Messages {
		msgNode := uir.NewNode(uir.TypeMap, msg.Name, nil)
		msgNode.SetAnnotation("kind", "message")
		
		for _, field := range msg.Fields {
			fieldNode := uir.NewNode(mapProtoType(field.Type, field.Repeated), field.Name, nil)
			if field.Repeated {
				fieldNode.ElementType = mapProtoType(field.Type, false)
			}
			fieldNode.SetAnnotation("proto_type", field.Type)
			fieldNode.SetAnnotation("proto_number", strconv.Itoa(field.Tag))
			msgNode.AddChild(fieldNode)
		}
		
		root.AddChild(msgNode)
	}
	return root
}

func mapProtoType(protoType string, repeated bool) uir.UIRType {
	if repeated {
		return uir.TypeArray
	}
	switch protoType {
	case "bytes":
		return uir.TypeBytes
	case "string":
		return uir.TypeString
	case "int32", "sint32", "sfixed32":
		return uir.TypeInt32
	case "uint32", "fixed32":
		return uir.TypeUInt32
	case "int64", "sint64", "sfixed64":
		return uir.TypeInt64
	case "uint64", "fixed64":
		return uir.TypeUInt64
	case "float", "double":
		return uir.TypeFloat64
	case "bool":
		return uir.TypeBoolean
	default:
		// Map complex message types to TypeMap
		return uir.TypeMap
	}
}
