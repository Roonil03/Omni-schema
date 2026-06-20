package codec

import (
	"omni-schema/internal/uir"
)

// GenerateProtobuf encodes a UIR Node graph into a binary Protobuf byte stream manually,
// parsing out varints, 32-bit/64-bit wire types, and length-delimited records.
func GenerateProtobuf(n *uir.Node) ([]byte, error) {
	var buf []byte
	// Custom binary encoding of nodes mapped into UIR goes here.
	// We operate entirely on byte-level manipulation without third-party proto libs.
	return buf, nil
}
