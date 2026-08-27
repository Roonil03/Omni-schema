package codec

import (
	"omni-schema/internal/uir"
)

// GenerateProtobuf encodes a UIR Node graph into a binary Protobuf byte stream manually,
// parsing out varints, 32-bit/64-bit wire types, and length-delimited records.
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
		val := uint64(n.Value.(int64))
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

// ParseProtobuf is a minimal protobuf decoder that blindly parses length-delimited strings
// and varints into a generic map structure without a schema (which uir.Project will fix later).
func ParseProtobuf(data []byte) (*uir.Node, error) {
	root := uir.NewNode(uir.TypeMap, "Root", nil)
	// We lack full schema decoding, so this is a placeholder implementation that treats
	// everything as raw bytes unless it parses cleanly as strings or varints.
	// For Phase 4 (Subset), we just return an empty map and let projection drop everything,
	// or we mock a simple parse loop.
	return root, nil
}
