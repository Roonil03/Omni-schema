package lexer

import (
	"encoding/json"
	"fmt"

	"omni-schema/internal/uir"
)

// ParseJSON parses a JSON payload and maps it into a UIR Node structure directly.
func ParseJSON(data []byte) (*uir.Node, error) {
	var payload any
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, err
	}
	return valueToUIR("Root", payload), nil
}

func valueToUIR(key string, v any) *uir.Node {
	switch val := v.(type) {
	case nil:
		return uir.NewNode(uir.TypeNull, key, nil)
	case string:
		return uir.NewNode(uir.TypeString, key, val)
	case float64:
		if val == float64(int64(val)) && val >= -9e15 && val <= 9e15 {
			return uir.NewNode(uir.TypeInt64, key, int64(val))
		}
		return uir.NewNode(uir.TypeFloat64, key, val)
	case bool:
		return uir.NewNode(uir.TypeBoolean, key, val)
	case json.Number:
		if i, err := val.Int64(); err == nil {
			return uir.NewNode(uir.TypeInt64, key, i)
		}
		f, _ := val.Float64()
		return uir.NewNode(uir.TypeFloat64, key, f)
	case map[string]any:
		root := uir.NewNode(uir.TypeMap, key, nil)
		MapToUIR(root, val)
		return root
	case []any:
		child := uir.NewNode(uir.TypeArray, key, nil)
		child.ElementType = inferArrayElementType(val)
		for i, elem := range val {
			child.AddChild(valueToUIR(fmt.Sprintf("%d", i), elem))
		}
		return child
	default:
		return uir.NewNode(uir.TypeString, key, fmt.Sprintf("%v", val))
	}
}

// MapToUIR converts a map[string]any into UIR child nodes under the given parent.
func MapToUIR(parent *uir.Node, data map[string]any) {
	for k, v := range data {
		parent.AddChild(valueToUIR(k, v))
	}
}

// inferArrayElementType inspects the first element of a JSON array to determine the
// UIR element type. Falls back to TypeString for empty or heterogeneous arrays.
func inferArrayElementType(arr []any) uir.UIRType {
	if len(arr) == 0 {
		return uir.TypeString
	}
	switch arr[0].(type) {
	case nil:
		return uir.TypeNull
	case map[string]any:
		return uir.TypeMap
	case float64:
		return uir.TypeFloat64
	case bool:
		return uir.TypeBoolean
	case string:
		return uir.TypeString
	default:
		return uir.TypeString
	}
}
