package lexer

import (
	"encoding/json"
	"fmt"

	"omni-schema/internal/uir"
)

// ParseJSON parses a JSON payload and maps it into a UIR Node structure directly.
func ParseJSON(data []byte) (*uir.Node, error) {
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, err
	}

	root := uir.NewNode(uir.TypeMap, "Root", nil)
	MapToUIR(root, payload)

	return root, nil
}

// MapToUIR converts a map[string]any (typically from JSON decoding) into UIR child
// nodes under the given parent. This is exported so that other packages (e.g. the
// streaming broker) can convert arbitrary event data into UIR graphs without
// re-serialising to JSON first.
func MapToUIR(parent *uir.Node, data map[string]any) {
	for k, v := range data {
		switch val := v.(type) {
		case string:
			child := uir.NewNode(uir.TypeString, k, val)
			parent.AddChild(child)
		case float64:
			child := uir.NewNode(uir.TypeFloat64, k, val)
			parent.AddChild(child)
		case bool:
			child := uir.NewNode(uir.TypeBoolean, k, val)
			parent.AddChild(child)
		case map[string]any:
			child := uir.NewNode(uir.TypeMap, k, nil)
			MapToUIR(child, val)
			parent.AddChild(child)
		case []any:
			child := uir.NewNode(uir.TypeArray, k, nil)
			child.ElementType = inferArrayElementType(val)
			for i, elem := range val {
				elemKey := fmt.Sprintf("%d", i)
				switch ev := elem.(type) {
				case map[string]any:
					elemNode := uir.NewNode(uir.TypeMap, elemKey, nil)
					MapToUIR(elemNode, ev)
					child.AddChild(elemNode)
				case string:
					child.AddChild(uir.NewNode(uir.TypeString, elemKey, ev))
				case float64:
					child.AddChild(uir.NewNode(uir.TypeFloat64, elemKey, ev))
				case bool:
					child.AddChild(uir.NewNode(uir.TypeBoolean, elemKey, ev))
				default:
					child.AddChild(uir.NewNode(uir.TypeString, elemKey, fmt.Sprintf("%v", ev)))
				}
			}
			parent.AddChild(child)
		default:
			child := uir.NewNode(uir.TypeString, k, fmt.Sprintf("%v", val))
			parent.AddChild(child)
		}
	}
}

// inferArrayElementType inspects the first element of a JSON array to determine the
// UIR element type. Falls back to TypeString for empty or heterogeneous arrays.
func inferArrayElementType(arr []any) uir.UIRType {
	if len(arr) == 0 {
		return uir.TypeString
	}
	switch arr[0].(type) {
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
