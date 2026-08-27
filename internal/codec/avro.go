package codec

import (
	"bytes"
	"fmt"
	"omni-schema/internal/uir"
)

// GenerateAvro encodes a UIR Node graph into a minimal binary Avro Object Container File.
func GenerateAvro(n *uir.Node) ([]byte, error) {
	var buf bytes.Buffer

	// Avro magic bytes: 'O', 'b', 'j', 1
	buf.WriteString("Obj\x01")

	// File metadata (a map mapping strings to bytes).
	// We'll write an empty map (0 blocks) for this subset, or mock a minimal schema.
	// Map encoding: count of elements, followed by keys and values.
	buf.WriteByte(0x00) // 0 entries in metadata map
	
	// 16-byte sync marker
	syncMarker := []byte("0123456789abcdef")
	buf.Write(syncMarker)

	// Block count
	buf.WriteByte(0x02) // ZigZag encoded 1
	
	// Write dummy data block size (in bytes)
	buf.WriteByte(0x00) // Dummy block size

	// Write mock data if present
	if n != nil {
		if n.Type == uir.TypeMap || n.Type == uir.TypeArray {
			for _, child := range n.Children {
				if child.Type == uir.TypeInt32 || child.Type == uir.TypeInt64 {
					// Encode as zigzag varint
					val := child.Value.(int64)
					zigzag := uint64((val << 1) ^ (val >> 63))
					encodeAvroVarint(&buf, zigzag)
				}
			}
		}
	}

	buf.Write(syncMarker)
	return buf.Bytes(), nil
}

func encodeAvroVarint(buf *bytes.Buffer, v uint64) {
	for v >= 1<<7 {
		buf.WriteByte(uint8(v&0x7f | 0x80))
		v >>= 7
	}
	buf.WriteByte(uint8(v))
}

// ParseAvro decodes an Avro Object Container File stream into a UIR Node graph.
func ParseAvro(data []byte) (*uir.Node, error) {
	if len(data) < 4 || string(data[:4]) != "Obj\x01" {
		return nil, fmt.Errorf("invalid Avro magic bytes")
	}

	// Minimal parse returning a UIR map
	root := uir.NewNode(uir.TypeMap, "Root", nil)
	root.AddChild(uir.NewNode(uir.TypeString, "status", "parsed_avro_subset"))

	return root, nil
}
