package codec

import (
	"encoding/json"

	"omni-schema/internal/uir"
)

// GenerateJSON takes a UIR Node graph and synthesizes a valid JSON byte stream.
func GenerateJSON(n *uir.Node) ([]byte, error) {
	// Reconstruct a map[string]any representation from the UIR graph
	// then marshal it into standard JSON.
	
	if n == nil {
		return []byte("null"), nil
	}

	result := uirToInterface(n)
	return json.MarshalIndent(result, "", "  ")
}

func uirToInterface(n *uir.Node) any {
	if n.Type == uir.TypeMap {
		m := make(map[string]any)
		for _, child := range n.Children {
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

	// For scalars, return the actual underlying value directly
	return n.Value
}
