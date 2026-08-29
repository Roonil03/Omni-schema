package lower

import (
	"strconv"
	"strings"

	"omni-schema/internal/ast"
	"omni-schema/internal/uir"
)

func LowerCapnProto(file *ast.CapnProtoFile) *uir.Node {
	root := uir.NewNode(uir.TypeMap, "capnp_root", nil)
	for _, s := range file.Structs {
		node := uir.NewNode(uir.TypeMap, s.Name, nil)
		node.SetAnnotation("kind", "struct")
		node.SetAnnotation("type", "struct")
		for _, f := range s.Fields {
			fn := uir.NewNode(mapCapnpType(f.Type), f.Name, nil)
			fn.SetAnnotation("capnp_type", f.Type)
			fn.SetAnnotation("capnp_ordinal", strconv.Itoa(f.Id))
			if strings.HasPrefix(strings.ToLower(f.Type), "list(") {
				fn.Type = uir.TypeArray
			}
			node.AddChild(fn)
		}
		root.AddChild(node)
	}
	return root
}

func mapCapnpType(t string) uir.UIRType {
	switch strings.TrimSpace(t) {
	case "Text":
		return uir.TypeString
	case "Data":
		return uir.TypeBytes
	case "Bool":
		return uir.TypeBoolean
	case "Int8", "Int16", "Int32":
		return uir.TypeInt32
	case "Int64":
		return uir.TypeInt64
	case "UInt8", "UInt16", "UInt32":
		return uir.TypeUInt32
	case "UInt64":
		return uir.TypeUInt64
	case "Float32":
		return uir.TypeFloat32
	case "Float64":
		return uir.TypeFloat64
	default:
		return uir.TypeMap
	}
}
