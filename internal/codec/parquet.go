package codec

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"omni-schema/internal/uir"
)

// GenerateParquet encodes a UIR Node graph into a minimal binary Parquet stream.
// Note: Parquet uses Thrift for metadata. This is a minimal subset for demonstration.
func GenerateParquet(n *uir.Node) ([]byte, error) {
	if n == nil {
		return []byte("PAR1\x00\x00\x00\x00PAR1"), nil
	}

	var buf bytes.Buffer
	buf.WriteString("PAR1")

	// In a real implementation, we would write row groups, column chunks, and pages here.
	// For this subset, we serialize UIR as a contiguous block of values.
	
	// Write dummy data
	if n.Type == uir.TypeMap || n.Type == uir.TypeArray {
		for _, child := range n.Children {
			if child.Type == uir.TypeInt32 || child.Type == uir.TypeInt64 {
				binary.Write(&buf, binary.LittleEndian, child.Value.(int64))
			}
		}
	}

	// Write empty thrift footer (mock)
	footerStart := buf.Len()
	buf.Write([]byte{0x00}) // Thrift stop field
	footerLen := buf.Len() - footerStart

	binary.Write(&buf, binary.LittleEndian, uint32(footerLen))
	buf.WriteString("PAR1")

	return buf.Bytes(), nil
}

// ParseParquet decodes a Parquet file stream into a UIR Node graph.
func ParseParquet(data []byte) (*uir.Node, error) {
	if len(data) < 8 {
		return nil, fmt.Errorf("file too small to be parquet")
	}

	if string(data[:4]) != "PAR1" || string(data[len(data)-4:]) != "PAR1" {
		return nil, fmt.Errorf("invalid parquet magic bytes")
	}

	footerLen := binary.LittleEndian.Uint32(data[len(data)-8 : len(data)-4])
	if int(footerLen)+8 > len(data) {
		return nil, fmt.Errorf("invalid parquet footer length")
	}

	// In a real implementation, we would decode the Thrift footer here
	// and use the offsets to parse column pages.
	// We return a mock UIR representation of the parsed structure.
	
	root := uir.NewNode(uir.TypeMap, "Root", nil)
	
	// Read our mock data block (between "PAR1" and footer)
	dataStart := 4
	dataEnd := len(data) - 8 - int(footerLen)
	
	if dataEnd > dataStart {
		dataBlock := data[dataStart:dataEnd]
		for i := 0; i < len(dataBlock)/8; i++ {
			val := binary.LittleEndian.Uint64(dataBlock[i*8 : i*8+8])
			root.AddChild(uir.NewNode(uir.TypeInt64, fmt.Sprintf("col_%d", i), int64(val)))
		}
	}
	
	return root, nil
}
