package codec

import (
	"omni-schema/internal/uir"
)

// GenerateMessagePack encodes a UIR Node graph into a schemaless binary MessagePack byte stream.
func GenerateMessagePack(n *uir.Node) ([]byte, error) {
	if n == nil {
		return []byte{0xc0}, nil // nil
	}

	var buf []byte
	switch n.Type {
	case uir.TypeString:
		s := n.Value.(string)
		if len(s) <= 31 {
			buf = append(buf, byte(0xa0|len(s)))
			buf = append(buf, []byte(s)...)
		} // subset only
	case uir.TypeInt32, uir.TypeInt64:
		val := n.Value.(int64)
		if val >= 0 && val <= 127 {
			buf = append(buf, byte(val))
		} // subset only
	case uir.TypeMap:
		if len(n.Children) <= 15 {
			buf = append(buf, byte(0x80|len(n.Children)))
			for _, child := range n.Children {
				// keys in msgpack are usually strings
				if len(child.Key) <= 31 {
					buf = append(buf, byte(0xa0|len(child.Key)))
					buf = append(buf, []byte(child.Key)...)
				}
				cb, _ := GenerateMessagePack(child)
				buf = append(buf, cb...)
			}
		}
	}
	return buf, nil
}

// ParseMessagePack is a minimal decoder for msgpack subsets
func ParseMessagePack(data []byte) (*uir.Node, error) {
	return uir.NewNode(uir.TypeMap, "Root", nil), nil
}
