package codec

import (
	"encoding/binary"
	"fmt"
	"math"
	"unicode/utf8"

	"omni-schema/internal/uir"
)

// GenerateProtobuf encodes a UIR Node graph into a binary Protobuf byte stream manually.
func GenerateProtobuf(n *uir.Node) ([]byte, error) {
	if n == nil {
		return nil, nil
	}
	// For a map (message), encode each child with a sequential tag
	if n.Type == uir.TypeMap {
		var buf []byte
		for i, child := range n.Children {
			tag := uint64(i + 1) // basic sequential tag if none in annotations
			childBytes, err := encodeProtoField(tag, child)
			if err != nil {
				return nil, err
			}
			buf = append(buf, childBytes...)
		}
		return buf, nil
	}
	return encodeProtoField(1, n)
}

func encodeProtoField(tag uint64, n *uir.Node) ([]byte, error) {
	var buf []byte
	switch n.Type {
	case uir.TypeInt32, uir.TypeInt64:
		// wire type 0 (varint)
		var val uint64
		switch v := n.Value.(type) {
		case int64: val = uint64(v)
		case int32: val = uint64(v)
		case int: val = uint64(v)
		case uint64: val = v
		}
		buf = append(buf, encodeVarint(tag<<3)...)
		buf = append(buf, encodeVarint(val)...)
	case uir.TypeString:
		// wire type 2 (length-delimited)
		val := []byte(n.Value.(string))
		buf = append(buf, encodeVarint((tag<<3)|2)...)
		buf = append(buf, encodeVarint(uint64(len(val)))...)
		buf = append(buf, val...)
	case uir.TypeBoolean:
		// wire type 0 (varint)
		val := uint64(0)
		if n.Value.(bool) {
			val = 1
		}
		buf = append(buf, encodeVarint(tag<<3)...)
		buf = append(buf, encodeVarint(val)...)
	case uir.TypeFloat64:
		// wire type 1 (64-bit)
		val := n.Value.(float64)
		bits := math.Float64bits(val)
		buf = append(buf, encodeVarint((tag<<3)|1)...)
		var b [8]byte
		binary.LittleEndian.PutUint64(b[:], bits)
		buf = append(buf, b[:]...)
	case uir.TypeMap:
		// wire type 2 (length-delimited embedded message)
		msg, err := GenerateProtobuf(n)
		if err != nil {
			return nil, err
		}
		buf = append(buf, encodeVarint((tag<<3)|2)...)
		buf = append(buf, encodeVarint(uint64(len(msg)))...)
		buf = append(buf, msg...)
	}
	return buf, nil
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

// ParseProtobuf decodes protobuf wire format without a schema into a UIR Node graph.
func ParseProtobuf(data []byte) (*uir.Node, error) {
	root := uir.NewNode(uir.TypeMap, "Root", nil)
	err := parseProtobufMessage(data, root)
	return root, err
}

func parseProtobufMessage(data []byte, parent *uir.Node) error {
	for len(data) > 0 {
		tagAndWire, n, err := decodeVarint(data)
		if err != nil {
			return err
		}
		data = data[n:]
		tag := tagAndWire >> 3
		wireType := tagAndWire & 7

		key := fmt.Sprintf("%d", tag)

		switch wireType {
		case 0: // Varint
			val, n, err := decodeVarint(data)
			if err != nil {
				return err
			}
			data = data[n:]
			parent.AddChild(uir.NewNode(uir.TypeInt64, key, int64(val)))
		case 1: // 64-bit
			if len(data) < 8 {
				return fmt.Errorf("unexpected EOF reading 64-bit wire type")
			}
			bits := binary.LittleEndian.Uint64(data[:8])
			data = data[8:]
			val := math.Float64frombits(bits)
			parent.AddChild(uir.NewNode(uir.TypeFloat64, key, val))
		case 2: // Length-delimited
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

			// Attempt recursive parse for nested messages
			msgNode := uir.NewNode(uir.TypeMap, key, nil)
			err = parseProtobufMessage(chunk, msgNode)
			if err == nil && len(msgNode.Children) > 0 {
				parent.AddChild(msgNode)
			} else {
				// Fallback to string if valid utf8, else just treat as string
				if utf8.Valid(chunk) {
					parent.AddChild(uir.NewNode(uir.TypeString, key, string(chunk)))
				} else {
					parent.AddChild(uir.NewNode(uir.TypeString, key, fmt.Sprintf("base64:%x", chunk)))
				}
			}
		case 5: // 32-bit
			if len(data) < 4 {
				return fmt.Errorf("unexpected EOF reading 32-bit wire type")
			}
			bits := binary.LittleEndian.Uint32(data[:4])
			data = data[4:]
			parent.AddChild(uir.NewNode(uir.TypeInt32, key, int32(bits)))
		default:
			return fmt.Errorf("unsupported wire type: %d", wireType)
		}
	}
	return nil
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
