package codec

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"

	"omni-schema/internal/uir"
)

// Omni HDF5 subset
//
// Valid HDF5 signature + Superblock Version 0, followed by a contiguous named
// dataset object-header table. This is not a full B-tree/symbol-table HDF5
// implementation; it is a documented subset sufficient for UIR round-trips:
//
//   [8] signature 89 'H' 'D' 'F' \r \n 1a \n
//   superblock v0 (offsets/lengths = 8)
//   object directory:
//     uint32 ndatasets
//     for each dataset:
//       uint16 name_len, name
//       uint8  dtype (0 int32, 1 int64, 2 float32, 3 float64, 4 uint8-bytes, 5 utf8, 6 bool)
//       uint64 nelems
//       uint64 data_offset
//   raw contiguous dataset payloads
//
// Superblock fields are populated so a parser can locate the directory offset.

var hdf5Signature = []byte{0x89, 'H', 'D', 'F', '\r', '\n', 0x1a, '\n'}

func GenerateHDF5(n *uir.Node) ([]byte, error) {
	return GenerateHDF5WithOptions(n, Options{})
}

func GenerateHDF5WithOptions(n *uir.Node, _ Options) ([]byte, error) {
	rows := parquetRows(n)
	type ds struct {
		name  string
		dtype byte
		vals  []any
	}
	var datasets []ds
	if len(rows) == 0 {
		datasets = nil
	} else {
		order := []string{}
		seen := map[string]byte{}
		for _, row := range rows {
			for _, c := range row.Children {
				if _, ok := seen[c.Key]; !ok {
					order = append(order, c.Key)
					seen[c.Key] = hdf5TypeOf(c)
				}
			}
		}
		for _, name := range order {
			d := ds{name: name, dtype: seen[name], vals: make([]any, len(rows))}
			for i, row := range rows {
				if ch := row.ChildByKey(name); ch != nil {
					d.vals[i] = ch.Value
				}
			}
			datasets = append(datasets, d)
		}
	}

	var dir bytes.Buffer
	binary.Write(&dir, binary.LittleEndian, uint32(len(datasets)))
	payloads := make([][]byte, len(datasets))
	for i, d := range datasets {
		var p bytes.Buffer
		for _, v := range d.vals {
			writeHDF5Value(&p, d.dtype, v)
		}
		payloads[i] = p.Bytes()
		nb := []byte(d.name)
		binary.Write(&dir, binary.LittleEndian, uint16(len(nb)))
		dir.Write(nb)
		dir.WriteByte(d.dtype)
		binary.Write(&dir, binary.LittleEndian, uint64(len(d.vals)))
		binary.Write(&dir, binary.LittleEndian, uint64(0)) // placeholder offset
	}

	const superblockSize = 56
	dirOff := uint64(superblockSize)
	dataOff := dirOff + uint64(dir.Len())

	patched := dir.Bytes()
	cursor := 4
	running := dataOff
	for i := range datasets {
		nlen := int(binary.LittleEndian.Uint16(patched[cursor:]))
		cursor += 2 + nlen + 1 + 8
		binary.LittleEndian.PutUint64(patched[cursor:], running)
		cursor += 8
		running += uint64(len(payloads[i]))
	}

	header := make([]byte, superblockSize)
	copy(header, hdf5Signature)
	header[8] = 0
	header[12] = 8
	header[13] = 8
	binary.LittleEndian.PutUint64(header[40:48], running)
	binary.LittleEndian.PutUint64(header[48:56], dirOff)

	var buf bytes.Buffer
	buf.Write(header)
	buf.Write(patched)
	for _, p := range payloads {
		buf.Write(p)
	}
	return buf.Bytes(), nil
}

func ParseHDF5(data []byte) (*uir.Node, error) {
	return ParseHDF5WithOptions(data, Options{})
}

func ParseHDF5WithOptions(data []byte, _ Options) (*uir.Node, error) {
	if len(data) < 16 || !bytes.Equal(data[:8], hdf5Signature) {
		return nil, fmt.Errorf("invalid HDF5 signature")
	}
	if data[8] != 0 {
		return nil, fmt.Errorf("unsupported HDF5 superblock version %d", data[8])
	}
	if len(data) < 56 {
		return nil, fmt.Errorf("truncated HDF5 superblock")
	}
	dirOff := binary.LittleEndian.Uint64(data[48:56])
	if dirOff >= uint64(len(data)) {
		return nil, fmt.Errorf("invalid HDF5 directory offset")
	}
	dir := data[dirOff:]
	if len(dir) < 4 {
		return nil, fmt.Errorf("truncated HDF5 directory")
	}
	nd := binary.LittleEndian.Uint32(dir[:4])
	off := 4
	type meta struct {
		name  string
		dtype byte
		n     uint64
		doff  uint64
	}
	var cols []meta
	for i := uint32(0); i < nd; i++ {
		if off+2 > len(dir) {
			return nil, fmt.Errorf("truncated dataset name")
		}
		nlen := int(binary.LittleEndian.Uint16(dir[off:]))
		off += 2
		if off+nlen+1+16 > len(dir) {
			return nil, fmt.Errorf("truncated dataset header")
		}
		name := string(dir[off : off+nlen])
		off += nlen
		dt := dir[off]
		off++
		ne := binary.LittleEndian.Uint64(dir[off:])
		off += 8
		do := binary.LittleEndian.Uint64(dir[off:])
		off += 8
		cols = append(cols, meta{name, dt, ne, do})
	}

	nRows := 0
	if len(cols) > 0 {
		nRows = int(cols[0].n)
	}
	values := make([][]any, len(cols))
	for i, c := range cols {
		payload := data[c.doff:]
		vals, err := readHDF5Values(payload, c.dtype, int(c.n))
		if err != nil {
			return nil, err
		}
		values[i] = vals
	}
	if nRows == 1 {
		row := uir.NewNode(uir.TypeMap, "Root", nil)
		for i, c := range cols {
			if len(values[i]) > 0 {
				row.AddChild(hdf5ToNode(c.name, c.dtype, values[i][0]))
			}
		}
		return row, nil
	}
	root := uir.NewNode(uir.TypeArray, "Root", nil)
	for r := 0; r < nRows; r++ {
		row := uir.NewNode(uir.TypeMap, "", nil)
		for i, c := range cols {
			if r < len(values[i]) {
				row.AddChild(hdf5ToNode(c.name, c.dtype, values[i][r]))
			}
		}
		root.AddChild(row)
	}
	return root, nil
}

func hdf5TypeOf(n *uir.Node) byte {
	switch n.Type {
	case uir.TypeInt32, uir.TypeUInt32, uir.TypeSInt32, uir.TypeFixed32, uir.TypeSFixed32:
		return 0
	case uir.TypeInt64, uir.TypeUInt64, uir.TypeSInt64, uir.TypeFixed64, uir.TypeSFixed64:
		return 1
	case uir.TypeBoolean:
		return 6
	case uir.TypeFloat32:
		return 2
	case uir.TypeFloat64:
		return 3
	case uir.TypeBytes:
		return 4
	default:
		return 5
	}
}

func writeHDF5Value(buf *bytes.Buffer, dtype byte, v any) {
	switch dtype {
	case 0:
		var x int32
		switch n := v.(type) {
		case int32:
			x = n
		case int64:
			x = int32(n)
		case bool:
			if n {
				x = 1
			}
		}
		binary.Write(buf, binary.LittleEndian, x)
	case 6:
		var x byte
		if b, ok := v.(bool); ok && b {
			x = 1
		}
		buf.WriteByte(x)
	case 1:
		binary.Write(buf, binary.LittleEndian, asInt64(v))
	case 2:
		var f float32
		switch n := v.(type) {
		case float32:
			f = n
		case float64:
			f = float32(n)
		}
		binary.Write(buf, binary.LittleEndian, math.Float32bits(f))
	case 3:
		var f float64
		switch n := v.(type) {
		case float64:
			f = n
		case float32:
			f = float64(n)
		}
		binary.Write(buf, binary.LittleEndian, math.Float64bits(f))
	case 4:
		b, _ := v.([]byte)
		binary.Write(buf, binary.LittleEndian, uint32(len(b)))
		buf.Write(b)
	default:
		s := ""
		if v != nil {
			if t, ok := v.(string); ok {
				s = t
			} else {
				s = fmt.Sprint(v)
			}
		}
		b := []byte(s)
		binary.Write(buf, binary.LittleEndian, uint32(len(b)))
		buf.Write(b)
	}
}

func readHDF5Values(payload []byte, dtype byte, n int) ([]any, error) {
	out := make([]any, 0, n)
	off := 0
	for i := 0; i < n; i++ {
		switch dtype {
		case 0:
			if off+4 > len(payload) {
				return nil, fmt.Errorf("eof hdf5 int32")
			}
			out = append(out, int32(binary.LittleEndian.Uint32(payload[off:])))
			off += 4
		case 1:
			if off+8 > len(payload) {
				return nil, fmt.Errorf("eof hdf5 int64")
			}
			out = append(out, int64(binary.LittleEndian.Uint64(payload[off:])))
			off += 8
		case 2:
			if off+4 > len(payload) {
				return nil, fmt.Errorf("eof hdf5 float32")
			}
			out = append(out, math.Float32frombits(binary.LittleEndian.Uint32(payload[off:])))
			off += 4
		case 3:
			if off+8 > len(payload) {
				return nil, fmt.Errorf("eof hdf5 float64")
			}
			out = append(out, math.Float64frombits(binary.LittleEndian.Uint64(payload[off:])))
			off += 8
		case 6:
			if off >= len(payload) {
				return nil, fmt.Errorf("eof hdf5 bool")
			}
			out = append(out, payload[off] != 0)
			off++
		default:
			if off+4 > len(payload) {
				return nil, fmt.Errorf("eof hdf5 len")
			}
			l := int(binary.LittleEndian.Uint32(payload[off:]))
			off += 4
			if off+l > len(payload) {
				return nil, fmt.Errorf("eof hdf5 bytes")
			}
			if dtype == 4 {
				cp := append([]byte(nil), payload[off:off+l]...)
				out = append(out, cp)
			} else {
				out = append(out, string(payload[off:off+l]))
			}
			off += l
		}
	}
	return out, nil
}

func hdf5ToNode(name string, dtype byte, v any) *uir.Node {
	switch dtype {
	case 0:
		return uir.NewNode(uir.TypeInt32, name, v)
	case 1:
		return uir.NewNode(uir.TypeInt64, name, v)
	case 2:
		return uir.NewNode(uir.TypeFloat32, name, v)
	case 3:
		return uir.NewNode(uir.TypeFloat64, name, v)
	case 4:
		return uir.NewNode(uir.TypeBytes, name, v)
	case 6:
		b, _ := v.(bool)
		return uir.NewNode(uir.TypeBoolean, name, b)
	default:
		return uir.NewNode(uir.TypeString, name, v)
	}
}
