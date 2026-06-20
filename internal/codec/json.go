package codec

import (
	"encoding/json"
	"omni-schema/internal/uir"
)

// GenerateJSON takes a UIR Node tree and generates a native JSON byte stream
// relying entirely on the standard Go library.
func GenerateJSON(n *uir.Node) ([]byte, error) {
	out := make(map[string]any)
	for _, child := range n.Children {
		out[child.Key] = child.Value
	}
	return json.Marshal(out)
}
