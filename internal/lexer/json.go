package lexer

import (
	"encoding/json"
	"omni-schema/internal/uir"
)

// ParseJSON takes a raw JSON payload and unmarshals it, recursively converting
// the Go types into a Universal Intermediate Representation (UIR) Node graph.
func ParseJSON(data []byte) (*uir.Node, error) {
	var parsed any
	err := json.Unmarshal(data, &parsed)
	if err != nil {
		return nil, err
	}

	return buildUIRNode("root", parsed), nil
}

func buildUIRNode(key string, val any) *uir.Node {
	switch v := val.(type) {
	case map[string]any:
		n := uir.NewNode(uir.TypeMap, key, nil)
		for k, childVal := range v {
			n.AddChild(buildUIRNode(k, childVal))
		}
		return n
	case []any:
		n := uir.NewNode(uir.TypeArray, key, nil)
		for _, childVal := range v {
			n.AddChild(buildUIRNode("", childVal))
		}
		return n
	case string:
		return uir.NewNode(uir.TypeString, key, v)
	case float64:
		return uir.NewNode(uir.TypeFloat64, key, v)
	case bool:
		return uir.NewNode(uir.TypeBoolean, key, v)
	default:
		return uir.NewNode(uir.TypeUnknown, key, v)
	}
}
