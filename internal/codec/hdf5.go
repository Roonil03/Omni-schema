package codec

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"omni-schema/internal/uir"
)

// GenerateHDF5 encodes a UIR Node graph into a minimal binary HDF5 file.
func GenerateHDF5(n *uir.Node) ([]byte, error) {
	var buf bytes.Buffer
	
	// HDF5 Superblock Version 0 signature
	signature := []byte{0x89, 'H', 'D', 'F', '\r', '\n', 0x1a, '\n'}
	buf.Write(signature)

	// Minimal Superblock
	buf.Write([]byte{0x00, 0x00, 0x00, 0x00}) // versions
	buf.Write([]byte{0x04, 0x04, 0x00, 0x00}) // sizes
	// pad to 512 bytes for a valid basic superblock
	pad := make([]byte, 512 - buf.Len())
	buf.Write(pad)

	// A real implementation writes B-Trees and Object Headers here.
	if n != nil {
		if n.Type == uir.TypeMap || n.Type == uir.TypeArray {
			for _, child := range n.Children {
				if child.Type == uir.TypeInt32 || child.Type == uir.TypeInt64 {
					binary.Write(&buf, binary.LittleEndian, child.Value.(int64))
				}
			}
		}
	}

	return buf.Bytes(), nil
}

// ParseHDF5 decodes an HDF5 stream into a UIR Node graph.
func ParseHDF5(data []byte) (*uir.Node, error) {
	signature := []byte{0x89, 'H', 'D', 'F', '\r', '\n', 0x1a, '\n'}
	if len(data) < len(signature) || !bytes.Equal(data[:8], signature) {
		return nil, fmt.Errorf("invalid HDF5 signature")
	}

	// Minimal parse returning a UIR map
	root := uir.NewNode(uir.TypeMap, "Root", nil)
	
	// Read our mock data block (after the 512 byte superblock)
	if len(data) > 520 {
		dataBlock := data[520:]
		for i := 0; i < len(dataBlock)/8; i++ {
			val := binary.LittleEndian.Uint64(dataBlock[i*8 : i*8+8])
			root.AddChild(uir.NewNode(uir.TypeInt64, fmt.Sprintf("dataset_%d", i), int64(val)))
		}
	}

	return root, nil
}
