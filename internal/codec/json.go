package codec

import (
	"encoding/base64"
	"encoding/json"

	"omni-schema/internal/uir"
)

func GenerateJSON(n *uir.Node) ([]byte, error) {
	if n == nil {
		return []byte("null"), nil
	}
	result := uirToInterface(n)
	return json.MarshalIndent(result, "", "  ")
}

func uirToInterface(n *uir.Node) any {
	if n == nil || n.Type == uir.TypeNull || n.Presence == uir.PresenceNull {
		return nil
	}
	if n.Presence == uir.PresenceMissing {
		return nil
	}
	if n.Type == uir.TypeMap {
		m := make(map[string]any)
		for _, child := range n.Children {
			if child.Presence == uir.PresenceMissing {
				continue
			}
			m[child.Key] = uirToInterface(child)
		}
		return m
	}
	if n.Type == uir.TypeArray {
		var arr []any
		for _, child := range n.Children {
			arr = append(arr, uirToInterface(child))
		}
		return arr
	}
	if n.Type == uir.TypeBytes {
		b, _ := n.Value.([]byte)
		return base64.StdEncoding.EncodeToString(b)
	}
	if n.Type == uir.TypeEnum {
		return n.Value
	}
	return n.Value
}
