package codec

import (
	"omni-schema/internal/uir"
)

// GenerateCapnProto encodes a UIR Node graph into a minimal binary Cap'n Proto stub.
func GenerateCapnProto(n *uir.Node) ([]byte, error) {
	// Minimal stub returning basic segment headers
	return []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}, nil
}

// ParseCapnProto provides a mock decoder for Cap'n proto
func ParseCapnProto(data []byte) (*uir.Node, error) {
	return uir.NewNode(uir.TypeMap, "Root", nil), nil
}
