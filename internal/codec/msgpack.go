package codec

import (
	"encoding/binary"
	"fmt"
	"math"

	"omni-schema/internal/uir"
)

// GenerateMessagePack encodes a UIR Node graph into a schemaless binary MessagePack byte stream.
func GenerateMessagePack(n *uir.Node) ([]byte, error) {
	if n == nil {
		return []byte{0xc0}, nil // nil
	}

	var buf []byte
	switch n.Type {
	case uir.TypeString:
		s, _ := n.Value.(string)
		l := len(s)
		if l <= 31 {
			buf = append(buf, byte(0xa0|l))
		} else if l <= math.MaxUint8 {
			buf = append(buf, 0xd9, byte(l))
		} else if l <= math.MaxUint16 {
			buf = append(buf, 0xda, byte(l>>8), byte(l))
		} else {
			buf = append(buf, 0xdb, byte(l>>24), byte(l>>16), byte(l>>8), byte(l))
		}
		buf = append(buf, []byte(s)...)

	case uir.TypeInt32, uir.TypeInt64:
		// We can just use the int64 representation for logic
		var val int64
		switch v := n.Value.(type) {
		case int64:
			val = v
		case int32:
			val = int64(v)
		case int:
			val = int64(v)
		}

		if val >= 0 && val <= 127 {
			buf = append(buf, byte(val))
		} else if val >= -32 && val <= -1 {
			buf = append(buf, byte(val))
		} else if val >= math.MinInt8 && val <= math.MaxInt8 {
			buf = append(buf, 0xd0, byte(val))
		} else if val >= math.MinInt16 && val <= math.MaxInt16 {
			buf = append(buf, 0xd1, byte(val>>8), byte(val))
		} else if val >= math.MinInt32 && val <= math.MaxInt32 {
			buf = append(buf, 0xd2, byte(val>>24), byte(val>>16), byte(val>>8), byte(val))
		} else {
			buf = append(buf, 0xd3, byte(val>>56), byte(val>>48), byte(val>>40), byte(val>>32), byte(val>>24), byte(val>>16), byte(val>>8), byte(val))
		}

	case uir.TypeFloat64:
		val, _ := n.Value.(float64)
		bits := math.Float64bits(val)
		buf = append(buf, 0xcb, byte(bits>>56), byte(bits>>48), byte(bits>>40), byte(bits>>32), byte(bits>>24), byte(bits>>16), byte(bits>>8), byte(bits))

	case uir.TypeBoolean:
		val, _ := n.Value.(bool)
		if val {
			buf = append(buf, 0xc3)
		} else {
			buf = append(buf, 0xc2)
		}

	case uir.TypeArray:
		l := len(n.Children)
		if l <= 15 {
			buf = append(buf, byte(0x90|l))
		} else if l <= math.MaxUint16 {
			buf = append(buf, 0xdc, byte(l>>8), byte(l))
		} else {
			buf = append(buf, 0xdd, byte(l>>24), byte(l>>16), byte(l>>8), byte(l))
		}
		for _, child := range n.Children {
			cb, err := GenerateMessagePack(child)
			if err != nil {
				return nil, err
			}
			buf = append(buf, cb...)
		}

	case uir.TypeMap:
		l := len(n.Children)
		if l <= 15 {
			buf = append(buf, byte(0x80|l))
		} else if l <= math.MaxUint16 {
			buf = append(buf, 0xde, byte(l>>8), byte(l))
		} else {
			buf = append(buf, 0xdf, byte(l>>24), byte(l>>16), byte(l>>8), byte(l))
		}
		for _, child := range n.Children {
			// Write key
			kb, err := GenerateMessagePack(uir.NewNode(uir.TypeString, "", child.Key))
			if err != nil {
				return nil, err
			}
			buf = append(buf, kb...)
			// Write value
			cb, err := GenerateMessagePack(child)
			if err != nil {
				return nil, err
			}
			buf = append(buf, cb...)
		}
	}
	return buf, nil
}

// ParseMessagePack decodes a msgpack byte stream into a UIR Node graph.
func ParseMessagePack(data []byte) (*uir.Node, error) {
	node, _, err := parseMessagePackValue("", data)
	return node, err
}

func parseMessagePackValue(key string, data []byte) (*uir.Node, []byte, error) {
	if len(data) == 0 {
		return nil, data, fmt.Errorf("unexpected end of msgpack data")
	}
	b := data[0]
	data = data[1:]

	// nil
	if b == 0xc0 {
		return nil, data, nil
	}
	// false
	if b == 0xc2 {
		return uir.NewNode(uir.TypeBoolean, key, false), data, nil
	}
	// true
	if b == 0xc3 {
		return uir.NewNode(uir.TypeBoolean, key, true), data, nil
	}

	// Positive FixInt
	if b <= 0x7f {
		return uir.NewNode(uir.TypeInt64, key, int64(b)), data, nil
	}
	// Negative FixInt
	if b >= 0xe0 {
		return uir.NewNode(uir.TypeInt64, key, int64(int8(b))), data, nil
	}

	// FixStr
	if b >= 0xa0 && b <= 0xbf {
		l := int(b & 0x1f)
		if len(data) < l {
			return nil, data, fmt.Errorf("invalid fixstr length")
		}
		s := string(data[:l])
		return uir.NewNode(uir.TypeString, key, s), data[l:], nil
	}

	// FixArray
	if b >= 0x90 && b <= 0x9f {
		l := int(b & 0x0f)
		return parseMessagePackArray(key, l, data)
	}

	// FixMap
	if b >= 0x80 && b <= 0x8f {
		l := int(b & 0x0f)
		return parseMessagePackMap(key, l, data)
	}

	switch b {
	// Integers
	case 0xcc: // uint 8
		if len(data) < 1 { return nil, data, fmt.Errorf("unexpected EOF") }
		return uir.NewNode(uir.TypeInt64, key, int64(data[0])), data[1:], nil
	case 0xcd: // uint 16
		if len(data) < 2 { return nil, data, fmt.Errorf("unexpected EOF") }
		v := binary.BigEndian.Uint16(data)
		return uir.NewNode(uir.TypeInt64, key, int64(v)), data[2:], nil
	case 0xce: // uint 32
		if len(data) < 4 { return nil, data, fmt.Errorf("unexpected EOF") }
		v := binary.BigEndian.Uint32(data)
		return uir.NewNode(uir.TypeInt64, key, int64(v)), data[4:], nil
	case 0xcf: // uint 64
		if len(data) < 8 { return nil, data, fmt.Errorf("unexpected EOF") }
		v := binary.BigEndian.Uint64(data)
		return uir.NewNode(uir.TypeInt64, key, int64(v)), data[8:], nil
	case 0xd0: // int 8
		if len(data) < 1 { return nil, data, fmt.Errorf("unexpected EOF") }
		return uir.NewNode(uir.TypeInt64, key, int64(int8(data[0]))), data[1:], nil
	case 0xd1: // int 16
		if len(data) < 2 { return nil, data, fmt.Errorf("unexpected EOF") }
		v := int16(binary.BigEndian.Uint16(data))
		return uir.NewNode(uir.TypeInt64, key, int64(v)), data[2:], nil
	case 0xd2: // int 32
		if len(data) < 4 { return nil, data, fmt.Errorf("unexpected EOF") }
		v := int32(binary.BigEndian.Uint32(data))
		return uir.NewNode(uir.TypeInt64, key, int64(v)), data[4:], nil
	case 0xd3: // int 64
		if len(data) < 8 { return nil, data, fmt.Errorf("unexpected EOF") }
		v := int64(binary.BigEndian.Uint64(data))
		return uir.NewNode(uir.TypeInt64, key, v), data[8:], nil

	// Floats
	case 0xca: // float 32
		if len(data) < 4 { return nil, data, fmt.Errorf("unexpected EOF") }
		v := math.Float32frombits(binary.BigEndian.Uint32(data))
		return uir.NewNode(uir.TypeFloat64, key, float64(v)), data[4:], nil
	case 0xcb: // float 64
		if len(data) < 8 { return nil, data, fmt.Errorf("unexpected EOF") }
		v := math.Float64frombits(binary.BigEndian.Uint64(data))
		return uir.NewNode(uir.TypeFloat64, key, v), data[8:], nil

	// Strings
	case 0xd9: // str 8
		if len(data) < 1 { return nil, data, fmt.Errorf("unexpected EOF") }
		l := int(data[0])
		data = data[1:]
		if len(data) < l { return nil, data, fmt.Errorf("unexpected EOF") }
		return uir.NewNode(uir.TypeString, key, string(data[:l])), data[l:], nil
	case 0xda: // str 16
		if len(data) < 2 { return nil, data, fmt.Errorf("unexpected EOF") }
		l := int(binary.BigEndian.Uint16(data))
		data = data[2:]
		if len(data) < l { return nil, data, fmt.Errorf("unexpected EOF") }
		return uir.NewNode(uir.TypeString, key, string(data[:l])), data[l:], nil
	case 0xdb: // str 32
		if len(data) < 4 { return nil, data, fmt.Errorf("unexpected EOF") }
		l := int(binary.BigEndian.Uint32(data))
		data = data[4:]
		if len(data) < l { return nil, data, fmt.Errorf("unexpected EOF") }
		return uir.NewNode(uir.TypeString, key, string(data[:l])), data[l:], nil

	// Arrays
	case 0xdc: // array 16
		if len(data) < 2 { return nil, data, fmt.Errorf("unexpected EOF") }
		l := int(binary.BigEndian.Uint16(data))
		return parseMessagePackArray(key, l, data[2:])
	case 0xdd: // array 32
		if len(data) < 4 { return nil, data, fmt.Errorf("unexpected EOF") }
		l := int(binary.BigEndian.Uint32(data))
		return parseMessagePackArray(key, l, data[4:])

	// Maps
	case 0xde: // map 16
		if len(data) < 2 { return nil, data, fmt.Errorf("unexpected EOF") }
		l := int(binary.BigEndian.Uint16(data))
		return parseMessagePackMap(key, l, data[2:])
	case 0xdf: // map 32
		if len(data) < 4 { return nil, data, fmt.Errorf("unexpected EOF") }
		l := int(binary.BigEndian.Uint32(data))
		return parseMessagePackMap(key, l, data[4:])

	default:
		return nil, data, fmt.Errorf("unsupported msgpack byte: 0x%x", b)
	}
}

func parseMessagePackArray(key string, length int, data []byte) (*uir.Node, []byte, error) {
	node := uir.NewNode(uir.TypeArray, key, nil)
	for i := 0; i < length; i++ {
		child, rem, err := parseMessagePackValue("", data)
		if err != nil {
			return nil, rem, err
		}
		data = rem
		if child != nil {
			node.AddChild(child)
		}
	}
	return node, data, nil
}

func parseMessagePackMap(key string, length int, data []byte) (*uir.Node, []byte, error) {
	node := uir.NewNode(uir.TypeMap, key, nil)
	for i := 0; i < length; i++ {
		// Read key
		kNode, rem, err := parseMessagePackValue("", data)
		if err != nil {
			return nil, rem, err
		}
		data = rem
		
		kStr := ""
		if kNode != nil && kNode.Type == uir.TypeString {
			kStr = kNode.Value.(string)
		} else {
			// MsgPack keys can be anything, but UIR maps them to strings for keys
			kStr = fmt.Sprintf("%v", kNode.Value)
		}

		// Read value
		child, rem, err := parseMessagePackValue(kStr, data)
		if err != nil {
			return nil, rem, err
		}
		data = rem
		if child != nil {
			node.AddChild(child)
		}
	}
	return node, data, nil
}
