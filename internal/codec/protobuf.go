package codec

import (
	"encoding/binary"
	"fmt"
	"math"
	"strconv"
	"unicode/utf8"

	"omni-schema/internal/uir"
)

func GenerateProtobuf(n *uir.Node) ([]byte, error) {
	return GenerateProtobufWithOptions(n, Options{})
}

func GenerateProtobufWithOptions(n *uir.Node, opts Options) ([]byte, error) {
	if n == nil {
		return nil, nil
	}
	schema := requireType(opts, n)
	if opts.TypeName != "" && opts.Schema != nil && schema == nil {
		return nil, fmt.Errorf("schema type %q not found", opts.TypeName)
	}
	if schema != nil && schema.Type == uir.TypeMap && n.Type == uir.TypeMap {
		return encodeProtoMessage(n, schema)
	}
	if n.Type == uir.TypeMap {
		return encodeProtoMessage(n, n)
	}
	return encodeProtoField(1, n)
}

func encodeProtoMessage(n, schema *uir.Node) ([]byte, error) {
	var buf []byte
	if schema != nil && len(schema.Children) > 0 {
		index := map[string]*uir.Node{}
		for _, c := range n.Children {
			index[c.Key] = c
		}
		for i, field := range schema.Children {
			child, ok := index[field.Key]
			if !ok {
				if num := field.Annotation("proto_number"); num != "" {
					child, ok = index[num]
				}
			}
			if !ok || child == nil || child.Presence == uir.PresenceMissing || child.Type == uir.TypeNull {
				continue
			}
			tag := protoTag(field, uint64(i+1))
			copyProtoMeta(child, field)
			b, err := encodeProtoField(tag, child)
			if err != nil {
				return nil, err
			}
			buf = append(buf, b...)
		}
		return buf, nil
	}
	for i, child := range n.Children {
		tag := protoTag(child, uint64(i+1))
		b, err := encodeProtoField(tag, child)
		if err != nil {
			return nil, err
		}
		buf = append(buf, b...)
	}
	return buf, nil
}

func copyProtoMeta(dst, src *uir.Node) {
	if dst.Annotation("proto_type") == "" {
		if t := src.Annotation("proto_type"); t != "" {
			dst.SetAnnotation("proto_type", t)
		}
	}
	if dst.Annotation("proto_number") == "" {
		if t := src.Annotation("proto_number"); t != "" {
			dst.SetAnnotation("proto_number", t)
		}
	}
}

func protoTag(n *uir.Node, fallback uint64) uint64 {
	if n == nil {
		return fallback
	}
	if tagStr := n.Annotation("proto_number"); tagStr != "" {
		if t, err := strconv.ParseUint(tagStr, 10, 64); err == nil {
			return t
		}
	}
	return fallback
}

func encodeProtoField(tag uint64, n *uir.Node) ([]byte, error) {
	if n.Type == uir.TypeArray {
		var buf []byte
		for _, child := range n.Children {
			b, err := encodeProtoField(tag, child)
			if err != nil {
				return nil, err
			}
			buf = append(buf, b...)
		}
		return buf, nil
	}

	protoType := n.Annotation("proto_type")
	if protoType == "" {
		protoType = inferProtoType(n)
	}

	switch protoType {
	case "sint32", "sint64":
		signed := asInt64(n.Value)
		zig := uint64((signed << 1) ^ (signed >> 63))
		return append(encodeVarint(tag<<3), encodeVarint(zig)...), nil
	case "fixed32":
		return appendFixed32(tag, uint32(asUint64(n.Value))), nil
	case "sfixed32":
		return appendFixed32(tag, uint32(asInt64(n.Value))), nil
	case "fixed64":
		return appendFixed64(tag, asUint64(n.Value)), nil
	case "sfixed64":
		return appendFixed64(tag, uint64(asInt64(n.Value))), nil
	case "float":
		var bits uint32
		switch v := n.Value.(type) {
		case float32:
			bits = math.Float32bits(v)
		case float64:
			bits = math.Float32bits(float32(v))
		}
		return appendFixed32(tag, bits), nil
	case "double":
		f, _ := n.Value.(float64)
		return appendFixed64(tag, math.Float64bits(f)), nil
	case "bool":
		v := uint64(0)
		if b, ok := n.Value.(bool); ok && b {
			v = 1
		}
		return append(encodeVarint(tag<<3), encodeVarint(v)...), nil
	case "string":
		s, _ := n.Value.(string)
		return appendLen(tag, []byte(s)), nil
	case "bytes":
		b, _ := n.Value.([]byte)
		if b == nil {
			if s, ok := n.Value.(string); ok {
				b = []byte(s)
			}
		}
		return appendLen(tag, b), nil
	}

	switch n.Type {
	case uir.TypeInt32, uir.TypeInt64, uir.TypeUInt32, uir.TypeUInt64, uir.TypeSInt32, uir.TypeSInt64, uir.TypeEnum:
		return append(encodeVarint(tag<<3), encodeVarint(asUint64(n.Value))...), nil
	case uir.TypeFixed32, uir.TypeSFixed32, uir.TypeFloat32:
		return encodeProtoField(tag, annotate(n, "proto_type", protoTypeOr(n, "fixed32")))
	case uir.TypeFixed64, uir.TypeSFixed64:
		return appendFixed64(tag, asUint64(n.Value)), nil
	case uir.TypeString, uir.TypeTimestamp, uir.TypeDate, uir.TypeTime, uir.TypeDuration, uir.TypeDecimal:
		s, _ := n.Value.(string)
		return appendLen(tag, []byte(s)), nil
	case uir.TypeBytes:
		b, _ := n.Value.([]byte)
		return appendLen(tag, b), nil
	case uir.TypeBoolean:
		v := uint64(0)
		if b, ok := n.Value.(bool); ok && b {
			v = 1
		}
		return append(encodeVarint(tag<<3), encodeVarint(v)...), nil
	case uir.TypeFloat64:
		f, _ := n.Value.(float64)
		return appendFixed64(tag, math.Float64bits(f)), nil
	case uir.TypeMap, uir.TypeUnion, uir.TypeInterface, uir.TypeDefinition:
		msg, err := GenerateProtobuf(n)
		if err != nil {
			return nil, err
		}
		return appendLen(tag, msg), nil
	}
	return nil, nil
}

func protoTypeOr(n *uir.Node, def string) string {
	if t := n.Annotation("proto_type"); t != "" {
		return t
	}
	return def
}

func annotate(n *uir.Node, k, v string) *uir.Node {
	n.SetAnnotation(k, v)
	return n
}

func inferProtoType(n *uir.Node) string {
	switch n.Type {
	case uir.TypeSInt32:
		return "sint32"
	case uir.TypeSInt64:
		return "sint64"
	case uir.TypeFixed32:
		return "fixed32"
	case uir.TypeFixed64:
		return "fixed64"
	case uir.TypeSFixed32:
		return "sfixed32"
	case uir.TypeSFixed64:
		return "sfixed64"
	case uir.TypeFloat32:
		return "float"
	case uir.TypeFloat64:
		return "double"
	case uir.TypeBytes:
		return "bytes"
	case uir.TypeString:
		return "string"
	case uir.TypeBoolean:
		return "bool"
	case uir.TypeUInt32:
		return "uint32"
	case uir.TypeUInt64:
		return "uint64"
	case uir.TypeInt32:
		return "int32"
	case uir.TypeInt64:
		return "int64"
	default:
		return ""
	}
}

func appendFixed32(tag uint64, v uint32) []byte {
	buf := encodeVarint((tag << 3) | 5)
	var b [4]byte
	binary.LittleEndian.PutUint32(b[:], v)
	return append(buf, b[:]...)
}

func appendFixed64(tag uint64, v uint64) []byte {
	buf := encodeVarint((tag << 3) | 1)
	var b [8]byte
	binary.LittleEndian.PutUint64(b[:], v)
	return append(buf, b[:]...)
}

func appendLen(tag uint64, payload []byte) []byte {
	buf := encodeVarint((tag << 3) | 2)
	buf = append(buf, encodeVarint(uint64(len(payload)))...)
	return append(buf, payload...)
}

func encodeVarint(v uint64) []byte {
	var buf []byte
	for v >= 1<<7 {
		buf = append(buf, uint8(v&0x7f|0x80))
		v >>= 7
	}
	buf = append(buf, uint8(v))
	return buf
}

func ParseProtobuf(data []byte) (*uir.Node, error) {
	return ParseProtobufWithOptions(data, Options{})
}

func ParseProtobufWithOptions(data []byte, opts Options) (*uir.Node, error) {
	schema := requireType(opts, nil)
	if opts.TypeName != "" && opts.Schema != nil && schema == nil {
		return nil, fmt.Errorf("schema type %q not found", opts.TypeName)
	}
	root := uir.NewNode(uir.TypeMap, "Root", nil)
	if schema != nil && schema.Key != "" && schema.Key != "proto_root" && schema.Key != "graphql_root" {
		root.Key = schema.Key
	}
	err := parseProtobufMessage(data, root, schema)
	return root, err
}

func parseProtobufMessage(data []byte, parent *uir.Node, schema *uir.Node) error {
	fieldsByTag := map[uint64]*uir.Node{}
	if schema != nil {
		for i, f := range schema.Children {
			if n := protoTag(f, uint64(i+1)); n != 0 {
				fieldsByTag[n] = f
			}
		}
	}

	for len(data) > 0 {
		tagAndWire, n, err := decodeVarint(data)
		if err != nil {
			return err
		}
		data = data[n:]
		tag := tagAndWire >> 3
		wireType := tagAndWire & 7
		fieldSchema := fieldsByTag[tag]
		key := fmt.Sprintf("%d", tag)
		if fieldSchema != nil {
			key = fieldSchema.Key
		}

		switch wireType {
		case 0:
			val, n, err := decodeVarint(data)
			if err != nil {
				return err
			}
			data = data[n:]
			parent.AddChild(decodeVarintField(key, val, fieldSchema))
		case 1:
			if len(data) < 8 {
				return fmt.Errorf("unexpected EOF reading 64-bit wire type")
			}
			bits := binary.LittleEndian.Uint64(data[:8])
			data = data[8:]
			parent.AddChild(decodeFixed64Field(key, bits, fieldSchema))
		case 2:
			l, n, err := decodeVarint(data)
			if err != nil {
				return err
			}
			data = data[n:]
			if len(data) < int(l) {
				return fmt.Errorf("unexpected EOF reading length-delimited wire type")
			}
			chunk := data[:l]
			data = data[l:]
			child, err := decodeLenField(key, chunk, fieldSchema)
			if err != nil {
				return err
			}
			parent.AddChild(child)
		case 5:
			if len(data) < 4 {
				return fmt.Errorf("unexpected EOF reading 32-bit wire type")
			}
			bits := binary.LittleEndian.Uint32(data[:4])
			data = data[4:]
			parent.AddChild(decodeFixed32Field(key, bits, fieldSchema))
		default:
			return fmt.Errorf("unsupported wire type: %d", wireType)
		}
	}
	return nil
}

func decodeVarintField(key string, val uint64, schema *uir.Node) *uir.Node {
	protoType := ""
	t := uir.TypeInt64
	if schema != nil {
		protoType = schema.Annotation("proto_type")
		t = schema.Type
		key = schema.Key
	}
	node := uir.NewNode(t, key, nil)
	if protoType != "" {
		node.SetAnnotation("proto_type", protoType)
	}
	if schema != nil {
		if num := schema.Annotation("proto_number"); num != "" {
			node.SetAnnotation("proto_number", num)
		}
	}
	switch protoType {
	case "bool":
		node.Type = uir.TypeBoolean
		node.Value = val != 0
	case "sint32":
		node.Type = uir.TypeSInt32
		node.Value = int32(unzigzag(val))
	case "sint64":
		node.Type = uir.TypeSInt64
		node.Value = unzigzag(val)
	case "uint32":
		node.Type = uir.TypeUInt32
		node.Value = uint32(val)
	case "uint64":
		node.Type = uir.TypeUInt64
		node.Value = val
	case "int32":
		node.Type = uir.TypeInt32
		node.Value = int32(val)
	case "enum":
		node.Type = uir.TypeEnum
		node.Value = int32(val)
		if schema != nil {
			for _, ev := range schema.Children {
				if fmt.Sprint(ev.Value) == fmt.Sprint(int32(val)) || ev.Annotation("enum_number") == fmt.Sprint(val) {
					node.Value = ev.Key
					break
				}
			}
		}
	default:
		if t == uir.TypeBoolean {
			node.Value = val != 0
		} else if t == uir.TypeUInt32 || t == uir.TypeUInt64 {
			node.Value = val
			if t == uir.TypeUInt32 {
				node.Value = uint32(val)
			}
		} else if t == uir.TypeInt32 {
			node.Value = int32(val)
		} else {
			node.Type = uir.TypeInt64
			node.Value = int64(val)
			if protoType != "" {
				node.SetAnnotation("proto_type", protoType)
			}
		}
	}
	return node
}

func unzigzag(val uint64) int64 {
	return int64(val>>1) ^ -int64(val&1)
}

func decodeFixed64Field(key string, bits uint64, schema *uir.Node) *uir.Node {
	protoType := ""
	if schema != nil {
		protoType = schema.Annotation("proto_type")
		key = schema.Key
	}
	switch protoType {
	case "double":
		n := uir.NewNode(uir.TypeFloat64, key, math.Float64frombits(bits))
		n.SetAnnotation("proto_type", "double")
		return n
	case "fixed64":
		n := uir.NewNode(uir.TypeFixed64, key, bits)
		n.SetAnnotation("proto_type", "fixed64")
		return n
	case "sfixed64":
		n := uir.NewNode(uir.TypeSFixed64, key, int64(bits))
		n.SetAnnotation("proto_type", "sfixed64")
		return n
	default:
		n := uir.NewNode(uir.TypeFloat64, key, math.Float64frombits(bits))
		return n
	}
}

func decodeFixed32Field(key string, bits uint32, schema *uir.Node) *uir.Node {
	protoType := ""
	if schema != nil {
		protoType = schema.Annotation("proto_type")
		key = schema.Key
	}
	switch protoType {
	case "float":
		n := uir.NewNode(uir.TypeFloat32, key, math.Float32frombits(bits))
		n.SetAnnotation("proto_type", "float")
		return n
	case "sfixed32":
		n := uir.NewNode(uir.TypeSFixed32, key, int32(bits))
		n.SetAnnotation("proto_type", "sfixed32")
		return n
	default:
		n := uir.NewNode(uir.TypeFixed32, key, bits)
		if protoType != "" {
			n.SetAnnotation("proto_type", protoType)
		}
		return n
	}
}

func decodeLenField(key string, chunk []byte, schema *uir.Node) (*uir.Node, error) {
	protoType := ""
	if schema != nil {
		protoType = schema.Annotation("proto_type")
		key = schema.Key
	}
	switch protoType {
	case "string":
		n := uir.NewNode(uir.TypeString, key, string(chunk))
		n.SetAnnotation("proto_type", "string")
		return n, nil
	case "bytes":
		cp := append([]byte(nil), chunk...)
		n := uir.NewNode(uir.TypeBytes, key, cp)
		n.SetAnnotation("proto_type", "bytes")
		return n, nil
	}
	if schema != nil && (schema.Type == uir.TypeMap || schema.Type == uir.TypeDefinition) {
		msg := uir.NewNode(uir.TypeMap, key, nil)
		if err := parseProtobufMessage(chunk, msg, schema); err != nil {
			return nil, err
		}
		return msg, nil
	}
	if schema != nil && schema.Type == uir.TypeArray && protoType == "" {
		// packed repeated scalars are not distinguished without packed=true
	}
	msgNode := uir.NewNode(uir.TypeMap, key, nil)
	err := parseProtobufMessage(chunk, msgNode, nil)
	if err == nil && len(msgNode.Children) > 0 {
		return msgNode, nil
	}
	if utf8.Valid(chunk) {
		return uir.NewNode(uir.TypeString, key, string(chunk)), nil
	}
	return uir.NewNode(uir.TypeBytes, key, append([]byte(nil), chunk...)), nil
}

func decodeVarint(data []byte) (uint64, int, error) {
	var v uint64
	var shift uint
	for i, b := range data {
		if b < 0x80 {
			if i > 9 || i == 9 && b > 1 {
				return 0, 0, fmt.Errorf("varint overflow")
			}
			return v | uint64(b)<<shift, i + 1, nil
		}
		v |= uint64(b&0x7f) << shift
		shift += 7
	}
	return 0, 0, fmt.Errorf("unexpected EOF reading varint")
}

func asInt64(v any) int64 {
	switch n := v.(type) {
	case int64:
		return n
	case int32:
		return int64(n)
	case int:
		return int64(n)
	case uint64:
		return int64(n)
	case uint32:
		return int64(n)
	case float64:
		return int64(n)
	default:
		return 0
	}
}

func asUint64(v any) uint64 {
	switch n := v.(type) {
	case uint64:
		return n
	case uint32:
		return uint64(n)
	case int64:
		return uint64(n)
	case int32:
		return uint64(n)
	case int:
		return uint64(n)
	case float64:
		return uint64(n)
	default:
		return 0
	}
}
