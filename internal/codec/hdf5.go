package codec

import (
	"omni-schema/internal/uir"
)

// GenerateHDF5 encodes a UIR Node graph into a minimal binary HDF5 stub.
func GenerateHDF5(n *uir.Node) ([]byte, error) {
	// Minimal stub returning HDF5 magic bytes (\x89HDF\r\n\x1a\n)
	return []byte("\x89HDF\r\n\x1a\n"), nil
}

// ParseHDF5 provides a mock decoder for HDF5
func ParseHDF5(data []byte) (*uir.Node, error) {
	return uir.NewNode(uir.TypeMap, "Root", nil), nil
}
