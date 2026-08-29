package lower

import (
	"strconv"

	"omni-schema/internal/ast"
	"omni-schema/internal/uir"
)

func LowerProtobuf(file *ast.ProtoFile) *uir.Node {
	root := uir.NewNode(uir.TypeMap, "proto_root", nil)
	if file.Syntax != "" {
		root.SetAnnotation("syntax", file.Syntax)
	}
	if file.Package != "" {
		root.SetAnnotation("package", file.Package)
	}
	for _, imp := range file.Imports {
		root.SetAnnotation("import:"+imp, "true")
	}
	for _, en := range file.Enums {
		root.AddChild(lowerProtoEnum(en))
	}
	for _, msg := range file.Messages {
		root.AddChild(lowerProtoMessage(msg))
	}
	for _, svc := range file.Services {
		svcNode := uir.NewNode(uir.TypeMap, svc.Name, nil)
		svcNode.SetAnnotation("kind", "service")
		for _, rpc := range svc.RPCs {
			rpcNode := uir.NewNode(uir.TypeMap, rpc.Name, nil)
			rpcNode.SetAnnotation("kind", "rpc")
			rpcNode.SetAnnotation("request", rpc.Request)
			rpcNode.SetAnnotation("response", rpc.Response)
			svcNode.AddChild(rpcNode)
		}
		root.AddChild(svcNode)
	}
	return root
}

func lowerProtoMessage(msg *ast.ProtoMessage) *uir.Node {
	msgNode := uir.NewNode(uir.TypeMap, msg.Name, nil)
	msgNode.SetAnnotation("kind", "message")
	for _, en := range msg.Enums {
		msgNode.AddChild(lowerProtoEnum(en))
	}
	for _, nested := range msg.Nested {
		msgNode.AddChild(lowerProtoMessage(nested))
	}
	for _, field := range msg.Fields {
		msgNode.AddChild(lowerProtoField(field))
	}
	for _, mf := range msg.Maps {
		fieldNode := uir.NewNode(uir.TypeMap, mf.Name, nil)
		fieldNode.SetAnnotation("kind", "map")
		fieldNode.SetAnnotation("proto_type", "map<"+mf.KeyType+","+mf.ValueType+">")
		fieldNode.SetAnnotation("proto_number", strconv.Itoa(mf.Tag))
		fieldNode.SetAnnotation("key_type", mf.KeyType)
		fieldNode.SetAnnotation("value_type", mf.ValueType)
		msgNode.AddChild(fieldNode)
	}
	for _, oo := range msg.Oneofs {
		ooNode := uir.NewNode(uir.TypeUnion, oo.Name, nil)
		ooNode.SetAnnotation("kind", "oneof")
		for _, f := range oo.Fields {
			child := lowerProtoField(f)
			child.SetAnnotation("oneof", oo.Name)
			ooNode.AddChild(child)
			msgNode.AddChild(child)
		}
		msgNode.AddChild(ooNode)
	}
	return msgNode
}

func lowerProtoField(field *ast.ProtoField) *uir.Node {
	fieldNode := uir.NewNode(mapProtoType(field.Type, field.Repeated), field.Name, nil)
	if field.Repeated {
		fieldNode.ElementType = mapProtoType(field.Type, false)
	}
	fieldNode.SetAnnotation("proto_type", field.Type)
	fieldNode.SetAnnotation("proto_number", strconv.Itoa(field.Tag))
	if field.Required {
		fieldNode.Cardinality = uir.CardinalityRequired
		fieldNode.SetAnnotation("nonNull", "true")
	}
	if field.Optional {
		fieldNode.SetAnnotation("optional", "true")
	}
	return fieldNode
}

func lowerProtoEnum(en *ast.ProtoEnum) *uir.Node {
	node := uir.NewNode(uir.TypeEnum, en.Name, nil)
	node.SetAnnotation("kind", "enum")
	node.SetAnnotation("proto_type", "enum")
	for _, v := range en.Values {
		ev := uir.NewNode(uir.TypeInt32, v.Name, int32(v.Number))
		ev.SetAnnotation("enum_number", strconv.Itoa(v.Number))
		node.AddChild(ev)
	}
	return node
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
	case "int32":
		return uir.TypeInt32
	case "sint32":
		return uir.TypeSInt32
	case "sfixed32":
		return uir.TypeSFixed32
	case "uint32":
		return uir.TypeUInt32
	case "fixed32":
		return uir.TypeFixed32
	case "int64":
		return uir.TypeInt64
	case "sint64":
		return uir.TypeSInt64
	case "sfixed64":
		return uir.TypeSFixed64
	case "uint64":
		return uir.TypeUInt64
	case "fixed64":
		return uir.TypeFixed64
	case "float":
		return uir.TypeFloat32
	case "double":
		return uir.TypeFloat64
	case "bool":
		return uir.TypeBoolean
	default:
		return uir.TypeMap
	}
}
