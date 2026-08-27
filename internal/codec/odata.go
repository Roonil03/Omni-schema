package codec

import (
	"encoding/json"
	"fmt"
	"omni-schema/internal/lexer"
	"omni-schema/internal/uir"
)

// GenerateOData encodes a UIR Node graph into an OData v4 compliant JSON response payload.
func GenerateOData(n *uir.Node) ([]byte, error) {
	// First generate the standard JSON payload
	innerJSONBytes, err := GenerateJSON(n)
	if err != nil {
		return nil, err
	}

	var innerData interface{}
	if err := json.Unmarshal(innerJSONBytes, &innerData); err != nil {
		return nil, err
	}

	// Wrap in OData payload structure
	odataPayload := map[string]interface{}{
		"@odata.context": "$metadata#EntitySet",
		"value":          innerData,
	}

	return json.Marshal(odataPayload)
}

// ParseOData decodes an OData JSON response payload into a UIR Node graph.
func ParseOData(data []byte) (*uir.Node, error) {
	var payload struct {
		Context string      `json:"@odata.context"`
		Value   interface{} `json:"value"`
	}

	if err := json.Unmarshal(data, &payload); err != nil {
		// Fallback to standard json parse if it's not a valid OData envelope
		return lexer.ParseJSON(data)
	}

	if payload.Value == nil {
		return nil, fmt.Errorf("odata payload missing value field")
	}

	valueBytes, err := json.Marshal(payload.Value)
	if err != nil {
		return nil, err
	}

	// Use standard JSON lexer on the inner value
	return lexer.ParseJSON(valueBytes)
}
