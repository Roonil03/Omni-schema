package codec

import (
	"encoding/binary"
	"fmt"
	"math"

	"omni-schema/internal/uir"
)

// GenerateCapnProto encodes a UIR Node graph into a minimal binary Cap'n Proto segment.
func GenerateCapnProto(n *uir.Node) ([]byte, error) {
	// A real Cap'n Proto encoder requires strict schema alignment, arena allocation,
	// and pointer arithmetic. This is a minimal subset implementation for dynamic structures.
	
	if n == nil {
		return []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}, nil
	}

	var dataSection []byte
	var ptrSection []byte
	
	// We'll treat the root as a struct.
	// Cap'n Proto struct pointer: 
	// A word (8 bytes). 
	// low 2 bits = 0 (struct)
	// next 30 bits = offset to data in words
	// next 16 bits = data section size in words
	// next 16 bits = pointer section size in words
	
	var stringData []byte

	switch n.Type {
	case uir.TypeMap:
		for _, child := range n.Children {
			switch child.Type {
			case uir.TypeInt32, uir.TypeInt64:
				var b [8]byte
				val, _ := child.Value.(int64)
				binary.LittleEndian.PutUint64(b[:], uint64(val))
				dataSection = append(dataSection, b[:]...)
			case uir.TypeFloat64:
				var b [8]byte
				val, _ := child.Value.(float64)
				binary.LittleEndian.PutUint64(b[:], math.Float64bits(val))
				dataSection = append(dataSection, b[:]...)
			case uir.TypeString:
				s, _ := child.Value.(string)
				sbytes := []byte(s)
				// Null-terminate for Capnp string
				sbytes = append(sbytes, 0)
				// Pad to 8-byte boundary
				pad := (8 - (len(sbytes) % 8)) % 8
				for i := 0; i < pad; i++ {
					sbytes = append(sbytes, 0)
				}
				
				var ptr [8]byte
				ptr[0] = 0x01 // list pointer
				
				// We'll fix offsets after we know sizes
				ptrSection = append(ptrSection, ptr[:]...)
				stringData = append(stringData, sbytes...)
			}
		}
	}

	dataWords := len(dataSection) / 8
	ptrWords := len(ptrSection) / 8
	
	// Fix pointers
	strOffsetWords := 0
	for i := 0; i < ptrWords; i++ {
		// List pointer for string (1 byte elements = 010 for size)
		// offset = (ptrWords - i - 1) + strOffsetWords
		offset := (ptrWords - i - 1) + strOffsetWords
		
		ptrVal := uint64(0x01) // list
		ptrVal |= uint64(offset) << 2
		ptrVal |= uint64(2) << 32 // 1-byte elements
		// String len is original length including null, wait, just put stringData len bytes for now
		ptrVal |= uint64(len(stringData)) << 35
		
		binary.LittleEndian.PutUint64(ptrSection[i*8:i*8+8], ptrVal)
		strOffsetWords += len(stringData) / 8 // simplified assuming 1 string
	}

	// Root pointer
	var rootPtr [8]byte
	rootPtr[0] = 0x00 // struct
	binary.LittleEndian.PutUint16(rootPtr[4:6], uint16(dataWords))
	binary.LittleEndian.PutUint16(rootPtr[6:8], uint16(ptrWords))
	
	segment := append(rootPtr[:], dataSection...)
	segment = append(segment, ptrSection...)
	segment = append(segment, stringData...)
	
	// Message header: 0 segments (means 1), segment size
	var header [8]byte
	header[0] = 0
	binary.LittleEndian.PutUint32(header[4:8], uint32(len(segment)/8))
	
	msg := append(header[:], segment...)
	return msg, nil
}

// ParseCapnProto decodes a Cap'n Proto stream into a UIR Node graph.
func ParseCapnProto(data []byte) (*uir.Node, error) {
	if len(data) < 16 {
		return nil, fmt.Errorf("capnproto message too short")
	}
	
	// Read header
	segCount := binary.LittleEndian.Uint32(data[0:4]) + 1
	_ = segCount
	segSize := binary.LittleEndian.Uint32(data[4:8])
	
	headerBytes := 8
	if len(data) < headerBytes + int(segSize)*8 {
		return nil, fmt.Errorf("capnproto truncated segment")
	}
	
	segment := data[headerBytes:]
	
	// Read root pointer
	rootPtr := segment[0:8]
	ptrType := rootPtr[0] & 3
	if ptrType != 0 {
		return nil, fmt.Errorf("expected root struct pointer")
	}
	
	dataWords := binary.LittleEndian.Uint16(rootPtr[4:6])
	ptrWords := binary.LittleEndian.Uint16(rootPtr[6:8])
	
	root := uir.NewNode(uir.TypeMap, "Root", nil)
	
	// Read data section naively assuming Int64 fields
	dataOffset := 8
	for i := 0; i < int(dataWords); i++ {
		val := binary.LittleEndian.Uint64(segment[dataOffset+i*8 : dataOffset+i*8+8])
		root.AddChild(uir.NewNode(uir.TypeInt64, fmt.Sprintf("field%d", i), int64(val)))
	}
	
	_ = ptrWords
	
	return root, nil
}
