package codec

import (
	"bytes"
	"encoding/json"
	"fmt"

	"omni-schema/internal/uir"
)

// Avro Object Container File (null codec) subset.
// Magic Obj\x01, metadata map containing avro.schema + avro.codec=null,
// 16-byte sync, then one or more data blocks of binary-encoded records.

func GenerateAvro(n *uir.Node) ([]byte, error) {
	return GenerateAvroWithOptions(n, Options{})
}

func GenerateAvroWithOptions(n *uir.Node, opts Options) ([]byte, error) {
	records := parquetRows(n)
	schemaJSON := avroSchemaJSON(opts.TypeSchema(), records)
	syncMarker := []byte("omni-schema-sync") // 16 bytes

	var buf bytes.Buffer
	buf.WriteString("Obj\x01")
	writeAvroMap(&buf, map[string][]byte{
		"avro.schema": []byte(schemaJSON),
		"avro.codec":  []byte("null"),
	})
	buf.Write(syncMarker)

	var block bytes.Buffer
	for _, rec := range records {
		if err := encodeAvroRecord(&block, rec, opts.TypeSchema()); err != nil {
			return nil, err
		}
	}
	writeAvroLong(&buf, int64(len(records)))
	writeAvroLong(&buf, int64(block.Len()))
	buf.Write(block.Bytes())
	buf.Write(syncMarker)
	return buf.Bytes(), nil
}

func ParseAvro(data []byte) (*uir.Node, error) {
	return ParseAvroWithOptions(data, Options{})
}

func ParseAvroWithOptions(data []byte, opts Options) (*uir.Node, error) {
	if len(data) < 4 || string(data[:4]) != "Obj\x01" {
		return nil, fmt.Errorf("invalid Avro magic bytes")
	}
	rest := data[4:]
	meta, rest, err := readAvroMap(rest)
	if err != nil {
		return nil, err
	}
	if len(rest) < 16 {
		return nil, fmt.Errorf("truncated avro sync marker")
	}
	sync := rest[:16]
	rest = rest[16:]

	var schema any
	if s, ok := meta["avro.schema"]; ok {
		_ = json.Unmarshal(s, &schema)
	}

	root := uir.NewNode(uir.TypeArray, "Root", nil)
	for len(rest) > 0 {
		if len(rest) >= 16 && bytes.Equal(rest, sync) {
			break
		}
		count, n, err := readAvroLong(rest)
		if err != nil {
			return nil, err
		}
		rest = rest[n:]
		size, n, err := readAvroLong(rest)
		if err != nil {
			return nil, err
		}
		rest = rest[n:]
		if count < 0 {
			count = -count
			// skip long-form block size already read as size
		}
		if int(size) > len(rest) {
			return nil, fmt.Errorf("truncated avro block")
		}
		block := rest[:size]
		rest = rest[size:]
		for i := int64(0); i < count; i++ {
			rec, rem, err := decodeAvroValue(block, schema, opts.TypeSchema())
			if err != nil {
				return nil, err
			}
			block = rem
			if rec != nil {
				root.AddChild(rec)
			}
		}
		if len(rest) < 16 || !bytes.Equal(rest[:16], sync) {
			return nil, fmt.Errorf("missing avro sync marker")
		}
		rest = rest[16:]
	}
	if len(root.Children) == 1 {
		return root.Children[0], nil
	}
	return root, nil
}

func avroSchemaJSON(schema *uir.Node, records []*uir.Node) string {
	if schema != nil && schema.Type == uir.TypeMap && len(schema.Children) > 0 {
		return string(mustJSON(avroSchemaFromUIR(schema)))
	}
	if len(records) > 0 {
		return string(mustJSON(avroSchemaFromUIR(records[0])))
	}
	return `{"type":"record","name":"Root","fields":[]}`
}

func avroSchemaFromUIR(n *uir.Node) map[string]any {
	fields := []any{}
	src := n
	if n.Type != uir.TypeMap && n.Type == uir.TypeArray && len(n.Children) > 0 {
		src = n.Children[0]
	}
	for _, c := range src.Children {
		fields = append(fields, map[string]any{
			"name": c.Key,
			"type": []any{"null", avroTypeName(c)},
		})
	}
	name := n.Key
	if name == "" || name == "Root" {
		name = "Root"
	}
	return map[string]any{"type": "record", "name": name, "fields": fields}
}

func avroTypeName(n *uir.Node) string {
	switch n.Type {
	case uir.TypeBoolean:
		return "boolean"
	case uir.TypeInt32, uir.TypeUInt32, uir.TypeSInt32:
		return "int"
	case uir.TypeInt64, uir.TypeUInt64, uir.TypeSInt64:
		return "long"
	case uir.TypeFloat32:
		return "float"
	case uir.TypeFloat64:
		return "double"
	case uir.TypeBytes:
		return "bytes"
	default:
		return "string"
	}
}

func encodeAvroRecord(buf *bytes.Buffer, rec *uir.Node, schema *uir.Node) error {
	fields := rec.Children
	if schema != nil && len(schema.Children) > 0 {
		fields = schema.Children
	}
	index := map[string]*uir.Node{}
	for _, c := range rec.Children {
		index[c.Key] = c
	}
	for _, f := range fields {
		child := index[f.Key]
		if child == nil || child.Type == uir.TypeNull || child.Presence == uir.PresenceMissing {
			writeAvroLong(buf, 0) // union index 0 = null
			continue
		}
		writeAvroLong(buf, 1) // union index 1 = value
		if err := encodeAvroScalar(buf, child); err != nil {
			return err
		}
	}
	return nil
}

func encodeAvroScalar(buf *bytes.Buffer, n *uir.Node) error {
	switch n.Type {
	case uir.TypeBoolean:
		b, _ := n.Value.(bool)
		if b {
			buf.WriteByte(1)
		} else {
			buf.WriteByte(0)
		}
	case uir.TypeInt32, uir.TypeInt64, uir.TypeSInt32, uir.TypeSInt64, uir.TypeUInt32, uir.TypeUInt64:
		writeAvroLong(buf, asInt64(n.Value))
	case uir.TypeFloat64:
		f, _ := n.Value.(float64)
		writeAvroDouble(buf, f)
	case uir.TypeFloat32:
		var f float64
		switch v := n.Value.(type) {
		case float32:
			f = float64(v)
		case float64:
			f = v
		}
		writeAvroDouble(buf, f)
	case uir.TypeBytes:
		b, _ := n.Value.([]byte)
		writeAvroBytes(buf, b)
	default:
		s, _ := n.Value.(string)
		writeAvroBytes(buf, []byte(s))
	}
	return nil
}

func decodeAvroValue(data []byte, schema any, typeSchema *uir.Node) (*uir.Node, []byte, error) {
	rec := uir.NewNode(uir.TypeMap, "Root", nil)
	fields := []map[string]any{}
	if m, ok := schema.(map[string]any); ok {
		if fl, ok := m["fields"].([]any); ok {
			for _, f := range fl {
				if fm, ok := f.(map[string]any); ok {
					fields = append(fields, fm)
				}
			}
		}
	}
	if len(fields) == 0 && typeSchema != nil {
		for _, c := range typeSchema.Children {
			fields = append(fields, map[string]any{"name": c.Key, "type": []any{"null", avroTypeName(c)}})
		}
	}
	if len(fields) == 0 {
		return rec, data, nil
	}
	for _, f := range fields {
		name, _ := f["name"].(string)
		idx, n, err := readAvroLong(data)
		if err != nil {
			return nil, data, err
		}
		data = data[n:]
		if idx == 0 {
			rec.AddChild(uir.NewNode(uir.TypeNull, name, nil))
			continue
		}
		typ := avroUnionBranch(f["type"], int(idx))
		node, rest, err := decodeAvroScalar(name, typ, data)
		if err != nil {
			return nil, data, err
		}
		data = rest
		rec.AddChild(node)
	}
	return rec, data, nil
}

func avroUnionBranch(t any, idx int) string {
	if arr, ok := t.([]any); ok {
		if idx >= 0 && idx < len(arr) {
			if s, ok := arr[idx].(string); ok {
				return s
			}
		}
		if idx < len(arr) {
			if m, ok := arr[idx].(map[string]any); ok {
				if s, ok := m["type"].(string); ok {
					return s
				}
			}
		}
	}
	if s, ok := t.(string); ok {
		return s
	}
	return "string"
}

func decodeAvroScalar(name, typ string, data []byte) (*uir.Node, []byte, error) {
	switch typ {
	case "null":
		return uir.NewNode(uir.TypeNull, name, nil), data, nil
	case "boolean":
		if len(data) < 1 {
			return nil, data, fmt.Errorf("eof avro bool")
		}
		return uir.NewNode(uir.TypeBoolean, name, data[0] != 0), data[1:], nil
	case "int":
		v, n, err := readAvroLong(data)
		if err != nil {
			return nil, data, err
		}
		return uir.NewNode(uir.TypeInt32, name, int32(v)), data[n:], nil
	case "long":
		v, n, err := readAvroLong(data)
		if err != nil {
			return nil, data, err
		}
		return uir.NewNode(uir.TypeInt64, name, v), data[n:], nil
	case "float", "double":
		if len(data) < 8 {
			return nil, data, fmt.Errorf("eof avro double")
		}
		bits := uint64(data[0]) | uint64(data[1])<<8 | uint64(data[2])<<16 | uint64(data[3])<<24 |
			uint64(data[4])<<32 | uint64(data[5])<<40 | uint64(data[6])<<48 | uint64(data[7])<<56
		f := floatFromBits(bits)
		return uir.NewNode(uir.TypeFloat64, name, f), data[8:], nil
	case "bytes":
		b, rest, err := readAvroBytes(data)
		if err != nil {
			return nil, data, err
		}
		return uir.NewNode(uir.TypeBytes, name, b), rest, nil
	default:
		b, rest, err := readAvroBytes(data)
		if err != nil {
			return nil, data, err
		}
		return uir.NewNode(uir.TypeString, name, string(b)), rest, nil
	}
}

func writeAvroMap(buf *bytes.Buffer, m map[string][]byte) {
	writeAvroLong(buf, int64(len(m)))
	for k, v := range m {
		writeAvroBytes(buf, []byte(k))
		writeAvroBytes(buf, v)
	}
	writeAvroLong(buf, 0)
}

func readAvroMap(data []byte) (map[string][]byte, []byte, error) {
	out := map[string][]byte{}
	for {
		n, size, err := readAvroLong(data)
		if err != nil {
			return nil, data, err
		}
		data = data[size:]
		if n == 0 {
			return out, data, nil
		}
		if n < 0 {
			n = -n
			_, s2, err := readAvroLong(data)
			if err != nil {
				return nil, data, err
			}
			data = data[s2:]
		}
		for i := int64(0); i < n; i++ {
			k, rest, err := readAvroBytes(data)
			if err != nil {
				return nil, data, err
			}
			v, rest, err := readAvroBytes(rest)
			if err != nil {
				return nil, data, err
			}
			out[string(k)] = v
			data = rest
		}
	}
}

func writeAvroLong(buf *bytes.Buffer, v int64) {
	zigzag := uint64((v << 1) ^ (v >> 63))
	encodeAvroVarint(buf, zigzag)
}

func readAvroLong(data []byte) (int64, int, error) {
	var v uint64
	var shift uint
	for i, b := range data {
		v |= uint64(b&0x7f) << shift
		if b < 0x80 {
			u := v
			return int64(u>>1) ^ -int64(u&1), i + 1, nil
		}
		shift += 7
		if i > 9 {
			return 0, 0, fmt.Errorf("avro long overflow")
		}
	}
	return 0, 0, fmt.Errorf("eof avro long")
}

func writeAvroBytes(buf *bytes.Buffer, b []byte) {
	writeAvroLong(buf, int64(len(b)))
	buf.Write(b)
}

func readAvroBytes(data []byte) ([]byte, []byte, error) {
	n, size, err := readAvroLong(data)
	if err != nil {
		return nil, data, err
	}
	data = data[size:]
	if n < 0 || int(n) > len(data) {
		return nil, data, fmt.Errorf("eof avro bytes")
	}
	return append([]byte(nil), data[:n]...), data[n:], nil
}

func writeAvroDouble(buf *bytes.Buffer, f float64) {
	bits := mathFloat64bits(f)
	var b [8]byte
	b[0] = byte(bits)
	b[1] = byte(bits >> 8)
	b[2] = byte(bits >> 16)
	b[3] = byte(bits >> 24)
	b[4] = byte(bits >> 32)
	b[5] = byte(bits >> 40)
	b[6] = byte(bits >> 48)
	b[7] = byte(bits >> 56)
	buf.Write(b[:])
}

func encodeAvroVarint(buf *bytes.Buffer, v uint64) {
	for v >= 1<<7 {
		buf.WriteByte(uint8(v&0x7f | 0x80))
		v >>= 7
	}
	buf.WriteByte(uint8(v))
}

func mustJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

func mathFloat64bits(f float64) uint64 {
	return floatBits(f)
}
