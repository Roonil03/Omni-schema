package codec

import (
	"encoding/binary"
	"fmt"
	"math"

	"omni-schema/internal/uir"
)

// Cap'n Proto single-segment subset:
//   header: segment count-1 (0), segment size in words
//   root struct pointer
//   data section (aligned 8)
//   pointer section (Text, Data, List, nested Struct)
//
// Field layout is schema-driven when Options.Schema is provided; otherwise
// numeric fields pack into the data section in declaration order and strings
// become Text pointers.

func GenerateCapnProto(n *uir.Node) ([]byte, error) {
	return GenerateCapnProtoWithOptions(n, Options{})
}

func GenerateCapnProtoWithOptions(n *uir.Node, opts Options) ([]byte, error) {
	if n == nil {
		return []byte{0, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}, nil
	}
	schema := requireType(opts, n)
	seg, err := encodeCapnpStruct(n, schema)
	if err != nil {
		return nil, err
	}
	words := uint32((len(seg) + 7) / 8)
	for len(seg)%8 != 0 {
		seg = append(seg, 0)
	}
	header := make([]byte, 8)
	binary.LittleEndian.PutUint32(header[0:4], 0)
	binary.LittleEndian.PutUint32(header[4:8], words)
	return append(header, seg...), nil
}

func ParseCapnProto(data []byte) (*uir.Node, error) {
	return ParseCapnProtoWithOptions(data, Options{})
}

func ParseCapnProtoWithOptions(data []byte, opts Options) (*uir.Node, error) {
	if len(data) < 16 {
		return nil, fmt.Errorf("capnproto message too short")
	}
	segWords := binary.LittleEndian.Uint32(data[4:8])
	headerBytes := 8
	need := headerBytes + int(segWords)*8
	if len(data) < need {
		return nil, fmt.Errorf("capnproto truncated segment")
	}
	segment := data[headerBytes:]
	schema := requireType(opts, nil)
	return decodeCapnpStruct(segment, 0, schema)
}

func encodeCapnpStruct(n, schema *uir.Node) ([]byte, error) {
	fields := n.Children
	if schema != nil && len(schema.Children) > 0 {
		fields = make([]*uir.Node, 0, len(schema.Children))
		idx := map[string]*uir.Node{}
		for _, c := range n.Children {
			idx[c.Key] = c
		}
		for _, f := range schema.Children {
			if c, ok := idx[f.Key]; ok {
				copyProtoMeta(c, f)
				fields = append(fields, c)
			} else {
				fields = append(fields, uir.NewNode(f.Type, f.Key, nil))
			}
		}
	}

	var dataSec []byte
	type ptrJob struct {
		field *uir.Node
		slot  int
	}
	var jobs []ptrJob
	ptrCount := 0
	for _, f := range fields {
		if capnpIsPointer(f) {
			jobs = append(jobs, ptrJob{f, ptrCount})
			ptrCount++
			continue
		}
		dataSec = append(dataSec, encodeCapnpDataField(f)...)
	}
	for len(dataSec)%8 != 0 {
		dataSec = append(dataSec, 0)
	}
	dataWords := len(dataSec) / 8
	ptrSec := make([]byte, ptrCount*8)
	var extra []byte
	for _, job := range jobs {
		payload, kind := encodeCapnpPointerPayload(job.field)
		offsetWords := (ptrCount - job.slot - 1) + len(extra)/8
		var ptr uint64
		switch kind {
		case "list":
			elemCount := uint64(len(payload))
			if job.field.Type == uir.TypeString || job.field.Type == uir.TypeBytes {
				elemCount = uint64(capnpListElemCount(job.field))
			} else if job.field.Type == uir.TypeArray {
				elemCount = uint64(len(job.field.Children))
			}
			ptr = 1 | uint64(offsetWords)<<2 | uint64(2)<<32 | elemCount<<35
		case "struct":
			if len(payload) >= 8 {
				dw := binary.LittleEndian.Uint16(payload[4:6])
				pw := binary.LittleEndian.Uint16(payload[6:8])
				ptr = 0 | uint64(offsetWords)<<2 | uint64(dw)<<32 | uint64(pw)<<48
			}
		}
		binary.LittleEndian.PutUint64(ptrSec[job.slot*8:], ptr)
		extra = append(extra, payload...)
		for len(extra)%8 != 0 {
			extra = append(extra, 0)
		}
	}

	root := make([]byte, 8)
	binary.LittleEndian.PutUint16(root[4:6], uint16(dataWords))
	binary.LittleEndian.PutUint16(root[6:8], uint16(ptrCount))
	out := append(root, dataSec...)
	out = append(out, ptrSec...)
	out = append(out, extra...)
	return out, nil
}

func capnpIsPointer(f *uir.Node) bool {
	switch f.Type {
	case uir.TypeString, uir.TypeBytes, uir.TypeArray, uir.TypeMap:
		return true
	default:
		return false
	}
}

func capnpListElemCount(f *uir.Node) int {
	switch f.Type {
	case uir.TypeString:
		s, _ := f.Value.(string)
		return len(s) + 1
	case uir.TypeBytes:
		b, _ := f.Value.([]byte)
		return len(b)
	default:
		return len(f.Children)
	}
}

func encodeCapnpDataField(f *uir.Node) []byte {
	b := make([]byte, 8)
	switch f.Type {
	case uir.TypeBoolean:
		if v, ok := f.Value.(bool); ok && v {
			b[0] = 1
		}
	case uir.TypeFloat64:
		if v, ok := f.Value.(float64); ok {
			binary.LittleEndian.PutUint64(b, math.Float64bits(v))
		}
	case uir.TypeFloat32:
		var f32 float32
		switch v := f.Value.(type) {
		case float32:
			f32 = v
		case float64:
			f32 = float32(v)
		}
		binary.LittleEndian.PutUint32(b[:4], math.Float32bits(f32))
	case uir.TypeInt32, uir.TypeUInt32, uir.TypeSInt32, uir.TypeFixed32, uir.TypeSFixed32:
		binary.LittleEndian.PutUint32(b[:4], uint32(asInt64(f.Value)))
	default:
		binary.LittleEndian.PutUint64(b, uint64(asInt64(f.Value)))
	}
	return b
}

func encodeCapnpPointerPayload(f *uir.Node) ([]byte, string) {
	switch f.Type {
	case uir.TypeString:
		s, _ := f.Value.(string)
		b := append([]byte(s), 0)
		return pad8(b), "list"
	case uir.TypeBytes:
		b, _ := f.Value.([]byte)
		return pad8(append([]byte(nil), b...)), "list"
	case uir.TypeMap:
		seg, _ := encodeCapnpStruct(f, f)
		return seg, "struct"
	case uir.TypeArray:
		var payload []byte
		for _, c := range f.Children {
			if capnpIsPointer(c) {
				p, _ := encodeCapnpPointerPayload(c)
				payload = append(payload, p...)
			} else {
				payload = append(payload, encodeCapnpDataField(c)...)
			}
		}
		return pad8(payload), "list"
	default:
		return nil, "list"
	}
}

func pad8(b []byte) []byte {
	for len(b)%8 != 0 {
		b = append(b, 0)
	}
	return b
}

func decodeCapnpStruct(segment []byte, ptrOff int, schema *uir.Node) (*uir.Node, error) {
	if ptrOff+8 > len(segment) {
		return nil, fmt.Errorf("capnproto struct pointer out of range")
	}
	rootPtr := segment[ptrOff : ptrOff+8]
	if rootPtr[0]&3 != 0 {
		return nil, fmt.Errorf("expected root struct pointer")
	}
	dataWords := int(binary.LittleEndian.Uint16(rootPtr[4:6]))
	ptrWords := int(binary.LittleEndian.Uint16(rootPtr[6:8]))
	dataOff := ptrOff + 8
	ptrSec := dataOff + dataWords*8
	root := uir.NewNode(uir.TypeMap, "Root", nil)
	if schema != nil && schema.Key != "" && schema.Key != "proto_root" {
		root.Key = schema.Key
	}

	dataFields, ptrFields := splitCapnpFields(schema, dataWords, ptrWords)
	for i, f := range dataFields {
		if dataOff+i*8+8 > len(segment) {
			break
		}
		word := binary.LittleEndian.Uint64(segment[dataOff+i*8 : dataOff+i*8+8])
		name := fmt.Sprintf("field%d", i)
		t := uir.TypeInt64
		if f != nil {
			name = f.Key
			t = f.Type
		}
		root.AddChild(decodeCapnpData(name, t, word, f))
	}
	for i, f := range ptrFields {
		off := ptrSec + i*8
		if off+8 > len(segment) {
			break
		}
		ptr := binary.LittleEndian.Uint64(segment[off : off+8])
		name := fmt.Sprintf("ptr%d", i)
		if f != nil {
			name = f.Key
		}
		child, err := decodeCapnpPointer(segment, off, ptr, name, f)
		if err != nil {
			return nil, err
		}
		if child != nil {
			root.AddChild(child)
		}
	}
	if schema == nil && len(dataFields) == 0 {
		for i := 0; i < dataWords; i++ {
			val := binary.LittleEndian.Uint64(segment[dataOff+i*8 : dataOff+i*8+8])
			root.AddChild(uir.NewNode(uir.TypeInt64, fmt.Sprintf("field%d", i), int64(val)))
		}
	}
	return root, nil
}

func splitCapnpFields(schema *uir.Node, dataWords, ptrWords int) (data, ptrs []*uir.Node) {
	if schema == nil {
		for i := 0; i < dataWords; i++ {
			data = append(data, nil)
		}
		for i := 0; i < ptrWords; i++ {
			ptrs = append(ptrs, nil)
		}
		return
	}
	for _, f := range schema.Children {
		if capnpIsPointer(f) {
			ptrs = append(ptrs, f)
		} else {
			data = append(data, f)
		}
	}
	for len(data) < dataWords {
		data = append(data, nil)
	}
	for len(ptrs) < ptrWords {
		ptrs = append(ptrs, nil)
	}
	return
}

func decodeCapnpData(name string, t uir.UIRType, word uint64, schema *uir.Node) *uir.Node {
	switch t {
	case uir.TypeBoolean:
		n := uir.NewNode(uir.TypeBoolean, name, word&1 == 1)
		return n
	case uir.TypeFloat64:
		return uir.NewNode(uir.TypeFloat64, name, math.Float64frombits(word))
	case uir.TypeFloat32:
		return uir.NewNode(uir.TypeFloat32, name, math.Float32frombits(uint32(word)))
	case uir.TypeInt32:
		return uir.NewNode(uir.TypeInt32, name, int32(word))
	default:
		return uir.NewNode(uir.TypeInt64, name, int64(word))
	}
}

func decodeCapnpPointer(segment []byte, ptrPos int, ptr uint64, name string, schema *uir.Node) (*uir.Node, error) {
	kind := ptr & 3
	offset := int64(int32(ptr>>2) & 0x3FFFFFFF)
	target := ptrPos + 8 + int(offset)*8
	if kind == 1 {
		elemSize := (ptr >> 32) & 7
		count := ptr >> 35
		_ = elemSize
		if target < 0 || target > len(segment) {
			return uir.NewNode(uir.TypeString, name, ""), nil
		}
		end := target + int(count)
		if end > len(segment) {
			end = len(segment)
		}
		raw := segment[target:end]
		if schema != nil && schema.Type == uir.TypeBytes {
			return uir.NewNode(uir.TypeBytes, name, append([]byte(nil), raw...)), nil
		}
		if len(raw) > 0 && raw[len(raw)-1] == 0 {
			raw = raw[:len(raw)-1]
		}
		return uir.NewNode(uir.TypeString, name, string(raw)), nil
	}
	if kind == 0 {
		nested, err := decodeCapnpStruct(segment, ptrPos, schema)
		if err != nil {
			return nil, err
		}
		nested.Key = name
		return nested, nil
	}
	return nil, nil
}
