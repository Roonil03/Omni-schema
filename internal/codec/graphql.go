package codec

import (
	"bytes"
	"encoding/json"
	"fmt"

	"omni-schema/internal/lexer"
	"omni-schema/internal/lower"
	"omni-schema/internal/uir"
)

// GenerateGraphQLResult takes a UIR Node graph and synthesizes a valid GraphQL JSON response payload.
// Example: {"data": { ... }}
func GenerateGraphQLResult(n *uir.Node) ([]byte, error) {
	// First, generate the inner JSON representation of the UIR graph.
	innerJSON, err := GenerateJSON(n)
	if err != nil {
		return nil, err
	}

	// Wrap the JSON in a GraphQL "data" envelope.
	var buf []byte
	buf = append(buf, []byte(`{"data":`)...)
	buf = append(buf, innerJSON...)
	buf = append(buf, []byte(`}`)...)

	return buf, nil
}

// ParseGraphQLResult takes a GraphQL JSON response payload and extracts the inner "data" object,
// decoding it into a UIR Node graph.
func ParseGraphQLResult(data []byte) (*uir.Node, error) {
	var payload struct {
		Data   json.RawMessage `json:"data"`
		Errors []interface{}   `json:"errors"`
	}

	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("failed to parse graphql payload: %v", err)
	}

	if len(payload.Errors) > 0 {
		return nil, fmt.Errorf("graphql payload contains errors")
	}

	if len(payload.Data) == 0 {
		return nil, fmt.Errorf("graphql payload missing data field")
	}

	// Leverage the existing JSON parser for the inner data.
	return lexer.ParseJSON(payload.Data)
}

// ParseGraphQL accepts either a GraphQL result envelope {"data":...} or GraphQL SDL.
func ParseGraphQL(data []byte) (*uir.Node, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) > 0 && trimmed[0] == '{' {
		n, err := ParseGraphQLResult(data)
		if err == nil {
			return n, nil
		}
	}
	l := &lexer.GraphQLLexer{}
	doc, err := l.Parse(string(data))
	if err != nil {
		return nil, fmt.Errorf("graphql: not a result JSON or SDL: %v", err)
	}
	if doc == nil || len(doc.Definitions) == 0 {
		return nil, fmt.Errorf("graphql: empty document")
	}
	return lower.LowerGraphQL(doc), nil
}
