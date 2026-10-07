package codec

import (
	"encoding/binary"
	"fmt"
	"math"
	"sort"
	"strconv"
	"unicode/utf8"

	"omni-schema/internal/uir"
)

// CBOR supports the RFC 8949 definite-length, text-keyed data model.
// Tags, indefinite lengths and non-text map keys are rejected explicitly.
const cborMaxDepth = 64
const cborMaxItems = 100000

func GenerateCBOR(n *uir.Node) ([]byte, error) {
	budget := cborMaxItems
	return appendCBOR(nil, n, 0, &budget)
}

func cborHead(dst []byte, major byte, value uint64) []byte {
	switch {
	case value < 24:
		return append(dst, major<<5|byte(value))
	case value <= math.MaxUint8:
		return append(dst, major<<5|24, byte(value))
	case value <= math.MaxUint16:
		return binary.BigEndian.AppendUint16(append(dst, major<<5|25), uint16(value))
	case value <= math.MaxUint32:
		return binary.BigEndian.AppendUint32(append(dst, major<<5|26), uint32(value))
	default:
		return binary.BigEndian.AppendUint64(append(dst, major<<5|27), value)
	}
}

func appendCBOR(dst []byte, n *uir.Node, depth int, budget *int) ([]byte, error) {
	(*budget)--
	if *budget < 0 {
		return nil, fmt.Errorf("cbor: item count exceeds %d", cborMaxItems)
	}
	if depth > cborMaxDepth {
		return nil, fmt.Errorf("cbor: nesting exceeds %d", cborMaxDepth)
	}
	if n == nil || n.Type == uir.TypeNull || n.Presence == uir.PresenceNull || n.Presence == uir.PresenceMissing {
		return append(dst, 0xf6), nil
	}
	if n.Type == uir.TypeMap || n.Type == uir.TypeDefinition {
		children := make([]*uir.Node, 0, len(n.Children))
		seen := make(map[string]bool)
		for _, child := range n.Children {
			if child == nil {
				return nil, fmt.Errorf("cbor: nil map field")
			}
			if child.Presence == uir.PresenceMissing {
				continue
			}
			if seen[child.Key] {
				return nil, fmt.Errorf("cbor: duplicate key %q", child.Key)
			}
			seen[child.Key] = true
			children = append(children, child)
		}
		sort.Slice(children, func(i, j int) bool { return children[i].Key < children[j].Key })
		dst = cborHead(dst, 5, uint64(len(children)))
		for _, child := range children {
			(*budget)--
			if *budget < 0 {
				return nil, fmt.Errorf("cbor: item count exceeds %d", cborMaxItems)
			}
			if !utf8.ValidString(child.Key) {
				return nil, fmt.Errorf("cbor: invalid UTF-8 key")
			}
			dst = append(cborHead(dst, 3, uint64(len(child.Key))), child.Key...)
			var err error
			dst, err = appendCBOR(dst, child, depth+1, budget)
			if err != nil {
				return nil, err
			}
		}
		return dst, nil
	}
	if n.Type == uir.TypeArray {
		dst = cborHead(dst, 4, uint64(len(n.Children)))
		for _, child := range n.Children {
			var err error
			dst, err = appendCBOR(dst, child, depth+1, budget)
			if err != nil {
				return nil, err
			}
		}
		return dst, nil
	}
	switch value := n.Value.(type) {
	case nil:
		return append(dst, 0xf6), nil
	case bool:
		if value {
			return append(dst, 0xf5), nil
		}
		return append(dst, 0xf4), nil
	case string:
		if !utf8.ValidString(value) {
			return nil, fmt.Errorf("cbor: invalid UTF-8 text")
		}
		return append(cborHead(dst, 3, uint64(len(value))), value...), nil
	case []byte:
		return append(cborHead(dst, 2, uint64(len(value))), value...), nil
	case int:
		return cborSigned(dst, int64(value)), nil
	case int32:
		return cborSigned(dst, int64(value)), nil
	case int64:
		return cborSigned(dst, value), nil
	case uint32:
		return cborHead(dst, 0, uint64(value)), nil
	case uint64:
		return cborHead(dst, 0, value), nil
	case float32:
		return binary.BigEndian.AppendUint32(append(dst, 0xfa), math.Float32bits(value)), nil
	case float64:
		return binary.BigEndian.AppendUint64(append(dst, 0xfb), math.Float64bits(value)), nil
	default:
		return nil, fmt.Errorf("cbor: unsupported value %T", value)
	}
}

func cborSigned(dst []byte, value int64) []byte {
	if value < 0 {
		return cborHead(dst, 1, uint64(-(value + 1)))
	}
	return cborHead(dst, 0, uint64(value))
}

func ParseCBOR(data []byte) (*uir.Node, error) {
	p := cborParser{data: data}
	n, err := p.read("Root", 0)
	if err != nil {
		return nil, err
	}
	if p.pos != len(data) {
		return nil, fmt.Errorf("cbor: trailing bytes")
	}
	n.SetAnnotation("source_format", "cbor")
	return n, nil
}

type cborParser struct {
	data  []byte
	pos   int
	items int
}

func (p *cborParser) take(count uint64) ([]byte, error) {
	if count > uint64(len(p.data)-p.pos) {
		return nil, fmt.Errorf("cbor: truncated payload")
	}
	b := p.data[p.pos : p.pos+int(count)]
	p.pos += int(count)
	return b, nil
}

func (p *cborParser) read(key string, depth int) (*uir.Node, error) {
	p.items++
	if p.items > cborMaxItems {
		return nil, fmt.Errorf("cbor: item count exceeds %d", cborMaxItems)
	}
	if depth > cborMaxDepth {
		return nil, fmt.Errorf("cbor: nesting exceeds %d", cborMaxDepth)
	}
	head, err := p.take(1)
	if err != nil {
		return nil, err
	}
	major, info := head[0]>>5, head[0]&31
	var value uint64
	if info < 24 {
		value = uint64(info)
	} else {
		var count uint64
		switch info {
		case 24:
			count = 1
		case 25:
			count = 2
		case 26:
			count = 4
		case 27:
			count = 8
		default:
			return nil, fmt.Errorf("cbor: reserved or indefinite length is unsupported")
		}
		b, err := p.take(count)
		if err != nil {
			return nil, err
		}
		for _, digit := range b {
			value = value<<8 | uint64(digit)
		}
	}
	switch major {
	case 0:
		if value <= math.MaxInt64 {
			return uir.NewNode(uir.TypeInt64, key, int64(value)), nil
		}
		return uir.NewNode(uir.TypeUInt64, key, value), nil
	case 1:
		if value > math.MaxInt64 {
			return nil, fmt.Errorf("cbor: negative integer exceeds int64")
		}
		return uir.NewNode(uir.TypeInt64, key, -1-int64(value)), nil
	case 2, 3:
		b, err := p.take(value)
		if err != nil {
			return nil, err
		}
		if major == 2 {
			return uir.NewNode(uir.TypeBytes, key, append([]byte(nil), b...)), nil
		}
		if !utf8.Valid(b) {
			return nil, fmt.Errorf("cbor: invalid UTF-8 text")
		}
		return uir.NewNode(uir.TypeString, key, string(b)), nil
	case 4, 5:
		maxEntries := uint64(cborMaxItems - p.items)
		if major == 5 {
			maxEntries /= 2
		}
		if value > maxEntries {
			return nil, fmt.Errorf("cbor: item count exceeds %d", cborMaxItems)
		}
		remaining := uint64(len(p.data) - p.pos)
		if value > remaining || major == 5 && value > remaining/2 {
			return nil, fmt.Errorf("cbor: impossible container length")
		}
		kind := uir.TypeArray
		if major == 5 {
			kind = uir.TypeMap
		}
		n := uir.NewNode(kind, key, nil)
		seen := make(map[string]bool)
		for i := uint64(0); i < value; i++ {
			childKey := strconv.FormatUint(i, 10)
			if major == 5 {
				k, err := p.read("", depth+1)
				if err != nil {
					return nil, err
				}
				if k.Type != uir.TypeString {
					return nil, fmt.Errorf("cbor: map keys must be text")
				}
				childKey = k.Value.(string)
				if seen[childKey] {
					return nil, fmt.Errorf("cbor: duplicate key %q", childKey)
				}
				seen[childKey] = true
			}
			child, err := p.read(childKey, depth+1)
			if err != nil {
				return nil, err
			}
			n.AddChild(child)
		}
		if kind == uir.TypeArray && len(n.Children) > 0 {
			n.ElementType = n.Children[0].Type
		}
		return n, nil
	case 7:
		switch info {
		case 20, 21:
			return uir.NewNode(uir.TypeBoolean, key, info == 21), nil
		case 22:
			return uir.NewNode(uir.TypeNull, key, nil), nil
		case 25:
			sign := 1.0
			if value&0x8000 != 0 {
				sign = -1
			}
			exp, frac := int((value>>10)&31), float64(value&1023)
			f := sign * math.Ldexp(frac, -24)
			if exp == 31 {
				f = sign * math.Inf(1)
				if frac != 0 {
					f = math.NaN()
				}
			} else if exp != 0 {
				f = sign * math.Ldexp(1+frac/1024, exp-15)
			}
			return uir.NewNode(uir.TypeFloat64, key, f), nil
		case 26:
			return uir.NewNode(uir.TypeFloat32, key, math.Float32frombits(uint32(value))), nil
		case 27:
			return uir.NewNode(uir.TypeFloat64, key, math.Float64frombits(value)), nil
		}
	}
	return nil, fmt.Errorf("cbor: unsupported major type %d or simple value %d", major, info)
}
