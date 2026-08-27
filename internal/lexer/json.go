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
	mapToUIR(root, payload)
	
	return root, nil
}

func mapToUIR(parent *uir.Node, data map[string]any) {
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
			mapToUIR(child, val)
			parent.AddChild(child)
		case []any:
			child := uir.NewNode(uir.TypeArray, k, nil)
			// Simple fallback for array of unknown
			child.ElementType = uir.TypeString 
			parent.AddChild(child)
		default:
			child := uir.NewNode(uir.TypeString, k, fmt.Sprintf("%v", val))
			parent.AddChild(child)
		}
	}
}
