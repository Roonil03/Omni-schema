package codec

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"

	"omni-schema/internal/uir"
)

// Omni Parquet subset v1
//
// Layout:
//   PAR1
//   RowGroup 0:
//     for each column:
//       ColumnChunk:
//         repeated DataPage:
//           uint8  page_type (0 = DATA_PAGE)
//           uint32 uncompressed_size
//           uint32 num_values
//           uint8  encoding (0 = PLAIN)
//           payload (PLAIN values)
//   FileMeta:
//     uint32 version (=1)
//     uint32 num_rows
//     uint32 num_columns
//     for each column:
//       uint16 name_len, name bytes
//       uint8  physical_type
//       uint64 chunk_offset
//       uint64 chunk_length
//       uint32 num_values
//   uint32 metadata_length
//   PAR1
//
// Physical types: 0 bool, 1 int32, 2 int64, 3 float, 4 double, 5 byte_array.

const (
	parquetMagic        = "PAR1"
	parquetSubsetVer    = uint32(1)
	parquetPageData     = byte(0)
	parquetEncPlain     = byte(0)
	parquetBool         = byte(0)
	parquetInt32        = byte(1)
	parquetInt64        = byte(2)
	parquetFloat        = byte(3)
	parquetDouble       = byte(4)
	parquetByteArray    = byte(5)
)

type parquetCol struct {
	name   string
	ptype  byte
	offset uint64
	length uint64
	nvals  uint32
	values []any
}

func GenerateParquet(n *uir.Node) ([]byte, error) {
	return GenerateParquetWithOptions(n, Options{})
}

func GenerateParquetWithOptions(n *uir.Node, opts Options) ([]byte, error) {
	rows := parquetRows(n)
	if len(rows) == 0 {
		var buf bytes.Buffer
		buf.WriteString(parquetMagic)
		binary.Write(&buf, binary.LittleEndian, parquetSubsetVer)
		binary.Write(&buf, binary.LittleEndian, uint32(0))
		binary.Write(&buf, binary.LittleEndian, uint32(0))
		binary.Write(&buf, binary.LittleEndian, uint32(12))
		buf.WriteString(parquetMagic)
		return buf.Bytes(), nil
	}

	schema := requireType(opts, nil)
	cols := collectParquetColumns(rows, schema)

	var body bytes.Buffer
	body.WriteString(parquetMagic)

	for i := range cols {
		start := body.Len()
		var page bytes.Buffer
		for _, v := range cols[i].values {
			writePlain(&page, cols[i].ptype, v)
		}
		payload := page.Bytes()
		body.WriteByte(parquetPageData)
		binary.Write(&body, binary.LittleEndian, uint32(len(payload)))
		binary.Write(&body, binary.LittleEndian, uint32(len(cols[i].values)))
		body.WriteByte(parquetEncPlain)
		body.Write(payload)
		cols[i].offset = uint64(start)
		cols[i].length = uint64(body.Len() - start)
		cols[i].nvals = uint32(len(cols[i].values))
	}

	metaStart := body.Len()
	binary.Write(&body, binary.LittleEndian, parquetSubsetVer)
	binary.Write(&body, binary.LittleEndian, uint32(len(rows)))
	binary.Write(&body, binary.LittleEndian, uint32(len(cols)))
	for _, c := range cols {
		name := []byte(c.name)
		binary.Write(&body, binary.LittleEndian, uint16(len(name)))
		body.Write(name)
		body.WriteByte(c.ptype)
		binary.Write(&body, binary.LittleEndian, c.offset)
		binary.Write(&body, binary.LittleEndian, c.length)
		binary.Write(&body, binary.LittleEndian, c.nvals)
	}
	metaLen := uint32(body.Len() - metaStart)
	binary.Write(&body, binary.LittleEndian, metaLen)
	body.WriteString(parquetMagic)
	return body.Bytes(), nil
}

func ParseParquet(data []byte) (*uir.Node, error) {
	return ParseParquetWithOptions(data, Options{})
}

func ParseParquetWithOptions(data []byte, opts Options) (*uir.Node, error) {
	if len(data) < 12 {
		return nil, fmt.Errorf("file too small to be parquet")
	}
	if string(data[:4]) != parquetMagic || string(data[len(data)-4:]) != parquetMagic {
		return nil, fmt.Errorf("invalid parquet magic bytes")
	}
	metaLen := binary.LittleEndian.Uint32(data[len(data)-8 : len(data)-4])
	if int(metaLen)+8 > len(data) {
		return nil, fmt.Errorf("invalid parquet footer length")
	}
	meta := data[len(data)-8-int(metaLen) : len(data)-8]
	if len(meta) < 12 {
		return nil, fmt.Errorf("truncated parquet metadata")
	}
	off := 0
	ver := binary.LittleEndian.Uint32(meta[off:])
	off += 4
	if ver != parquetSubsetVer {
		return nil, fmt.Errorf("unsupported parquet subset version %d", ver)
	}
	numRows := binary.LittleEndian.Uint32(meta[off:])
	off += 4
	numCols := binary.LittleEndian.Uint32(meta[off:])
	off += 4

	type colMeta struct {
		name   string
		ptype  byte
		offset uint64
		length uint64
		nvals  uint32
	}
	cols := make([]colMeta, 0, numCols)
	for i := uint32(0); i < numCols; i++ {
		if off+2 > len(meta) {
			return nil, fmt.Errorf("truncated column name")
		}
		nlen := int(binary.LittleEndian.Uint16(meta[off:]))
		off += 2
		if off+nlen+1+8+8+4 > len(meta) {
			return nil, fmt.Errorf("truncated column metadata")
		}
		name := string(meta[off : off+nlen])
		off += nlen
		pt := meta[off]
		off++
		coff := binary.LittleEndian.Uint64(meta[off:])
		off += 8
		clen := binary.LittleEndian.Uint64(meta[off:])
		off += 8
		nv := binary.LittleEndian.Uint32(meta[off:])
		off += 4
		cols = append(cols, colMeta{name, pt, coff, clen, nv})
	}

	colValues := make([][]any, len(cols))
	for i, c := range cols {
		chunk := data[c.offset : c.offset+c.length]
		if len(chunk) < 10 {
			return nil, fmt.Errorf("truncated column chunk %s", c.name)
		}
		if chunk[0] != parquetPageData {
			return nil, fmt.Errorf("unsupported page type in %s", c.name)
		}
		usize := binary.LittleEndian.Uint32(chunk[1:5])
		nvals := binary.LittleEndian.Uint32(chunk[5:9])
		enc := chunk[9]
		if enc != parquetEncPlain {
			return nil, fmt.Errorf("unsupported encoding in %s", c.name)
		}
		payload := chunk[10 : 10+usize]
		vals, err := readPlain(payload, c.ptype, int(nvals))
		if err != nil {
			return nil, err
		}
		colValues[i] = vals
	}

	root := uir.NewNode(uir.TypeArray, "Root", nil)
	root.ElementType = uir.TypeMap
	for r := uint32(0); r < numRows; r++ {
		row := uir.NewNode(uir.TypeMap, "", nil)
		for i, c := range cols {
			if int(r) < len(colValues[i]) {
				row.AddChild(plainToNode(c.name, c.ptype, colValues[i][r]))
			}
		}
		root.AddChild(row)
	}
	if numRows == 1 {
		return root.Children[0], nil
	}
	return root, nil
}

func parquetRows(n *uir.Node) []*uir.Node {
	if n == nil {
		return nil
	}
	if n.Type == uir.TypeArray {
		return n.Children
	}
	if n.Type == uir.TypeMap {
		if n.Key == "Root" || n.Key == "root" || n.Key == "graphql_root" || n.Key == "proto_root" {
			if len(n.Children) == 1 && n.Children[0].Type == uir.TypeMap {
				return []*uir.Node{n.Children[0]}
			}
		}
		return []*uir.Node{n}
	}
	return []*uir.Node{n}
}

func collectParquetColumns(rows []*uir.Node, schema *uir.Node) []parquetCol {
	order := []string{}
	seen := map[string]byte{}
	if schema != nil {
		for _, f := range schema.Children {
			order = append(order, f.Key)
			seen[f.Key] = parquetTypeOf(f)
		}
	}
	for _, row := range rows {
		for _, c := range row.Children {
			if _, ok := seen[c.Key]; !ok {
				order = append(order, c.Key)
				seen[c.Key] = parquetTypeOf(c)
			}
		}
	}
	cols := make([]parquetCol, len(order))
	for i, name := range order {
		cols[i] = parquetCol{name: name, ptype: seen[name], values: make([]any, len(rows))}
		for r, row := range rows {
			if ch := row.ChildByKey(name); ch != nil {
				cols[i].values[r] = ch.Value
			}
		}
	}
	return cols
}

func parquetTypeOf(n *uir.Node) byte {
	switch n.Type {
	case uir.TypeBoolean:
		return parquetBool
	case uir.TypeInt32, uir.TypeUInt32, uir.TypeSInt32, uir.TypeFixed32, uir.TypeSFixed32:
		return parquetInt32
	case uir.TypeInt64, uir.TypeUInt64, uir.TypeSInt64, uir.TypeFixed64, uir.TypeSFixed64:
		return parquetInt64
	case uir.TypeFloat32:
		return parquetFloat
	case uir.TypeFloat64:
		return parquetDouble
	default:
		return parquetByteArray
	}
}

func writePlain(buf *bytes.Buffer, ptype byte, v any) {
	switch ptype {
	case parquetBool:
		b := byte(0)
		if bv, ok := v.(bool); ok && bv {
			b = 1
		}
		buf.WriteByte(b)
	case parquetInt32:
		var x int32
		switch n := v.(type) {
		case int32:
			x = n
		case int64:
			x = int32(n)
		case uint32:
			x = int32(n)
		}
		binary.Write(buf, binary.LittleEndian, x)
	case parquetInt64:
		binary.Write(buf, binary.LittleEndian, asInt64(v))
	case parquetFloat:
		var f float32
		switch n := v.(type) {
		case float32:
			f = n
		case float64:
			f = float32(n)
		}
		binary.Write(buf, binary.LittleEndian, math.Float32bits(f))
	case parquetDouble:
		var f float64
		switch n := v.(type) {
		case float64:
			f = n
		case float32:
			f = float64(n)
		}
		binary.Write(buf, binary.LittleEndian, math.Float64bits(f))
	default:
		var s []byte
		switch n := v.(type) {
		case string:
			s = []byte(n)
		case []byte:
			s = n
		default:
			if v != nil {
				s = []byte(fmt.Sprint(v))
			}
		}
		binary.Write(buf, binary.LittleEndian, uint32(len(s)))
		buf.Write(s)
	}
}

func readPlain(payload []byte, ptype byte, n int) ([]any, error) {
	out := make([]any, 0, n)
	off := 0
	for i := 0; i < n; i++ {
		switch ptype {
		case parquetBool:
			if off >= len(payload) {
				return nil, fmt.Errorf("eof bool")
			}
			out = append(out, payload[off] != 0)
			off++
		case parquetInt32:
			if off+4 > len(payload) {
				return nil, fmt.Errorf("eof int32")
			}
			out = append(out, int32(binary.LittleEndian.Uint32(payload[off:])))
			off += 4
		case parquetInt64:
			if off+8 > len(payload) {
				return nil, fmt.Errorf("eof int64")
			}
			out = append(out, int64(binary.LittleEndian.Uint64(payload[off:])))
			off += 8
		case parquetFloat:
			if off+4 > len(payload) {
				return nil, fmt.Errorf("eof float")
			}
			out = append(out, math.Float32frombits(binary.LittleEndian.Uint32(payload[off:])))
			off += 4
		case parquetDouble:
			if off+8 > len(payload) {
				return nil, fmt.Errorf("eof double")
			}
			out = append(out, math.Float64frombits(binary.LittleEndian.Uint64(payload[off:])))
			off += 8
		default:
			if off+4 > len(payload) {
				return nil, fmt.Errorf("eof bytes len")
			}
			l := int(binary.LittleEndian.Uint32(payload[off:]))
			off += 4
			if off+l > len(payload) {
				return nil, fmt.Errorf("eof bytes")
			}
			out = append(out, string(payload[off:off+l]))
			off += l
		}
	}
	return out, nil
}

func plainToNode(name string, ptype byte, v any) *uir.Node {
	switch ptype {
	case parquetBool:
		return uir.NewNode(uir.TypeBoolean, name, v)
	case parquetInt32:
		return uir.NewNode(uir.TypeInt32, name, v)
	case parquetInt64:
		return uir.NewNode(uir.TypeInt64, name, v)
	case parquetFloat:
		return uir.NewNode(uir.TypeFloat32, name, v)
	case parquetDouble:
		return uir.NewNode(uir.TypeFloat64, name, v)
	default:
		if b, ok := v.([]byte); ok {
			return uir.NewNode(uir.TypeBytes, name, b)
		}
		return uir.NewNode(uir.TypeString, name, v)
	}
}
