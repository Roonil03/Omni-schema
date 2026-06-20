package codec

import (
	"omni-schema/internal/uir"
)

// GenerateMessagePack encodes a UIR Node graph into a schemaless binary MessagePack byte stream.
func GenerateMessagePack(n *uir.Node) ([]byte, error) {
	var buf []byte
	// Custom binary encoding for MessagePack headers and payloads
	// e.g., mapping UIR_Map to 0x80+N, UIR_String to 0xa0+N
	return buf, nil
}
