package codec

import (
	"omni-schema/internal/uir"
)

// GenerateParquet encodes a UIR Node graph into a binary Parquet file stream manually.
func GenerateParquet(n *uir.Node) ([]byte, error) {
	// Minimal stub returning PAR1 magic bytes and an empty footer
	return []byte("PAR1\x00\x00\x00\x00PAR1"), nil
}

// ParseParquet provides a mock decoder for Parquet
func ParseParquet(data []byte) (*uir.Node, error) {
	return uir.NewNode(uir.TypeMap, "Root", nil), nil
}
