package codec

import (
	"encoding/json"
	"fmt"
	"strings"

	"omni-schema/internal/lexer"
	"omni-schema/internal/uir"
)

// OData support is an OData JSON payload subset with EDM type annotations.
// It is NOT a full OData query service ($filter/$expand execution, CSDL XML
// $metadata documents, or OData URL convention routing).
//
// Encode produces:
//   {
//     "@odata.context": "<context>",
//     "@odata.type": "#Namespace.Type" (when known),
//     "value": <entity or collection>
//   }
// Primitive values may carry target-type annotations via UIR TypeAnnotations
// keys edm_type (Edm.String, Edm.Int32, ...).

func GenerateOData(n *uir.Node) ([]byte, error) {
	return GenerateODataWithOptions(n, Options{})
}

func GenerateODataWithOptions(n *uir.Node, opts Options) ([]byte, error) {
	inner, err := GenerateJSON(n)
	if err != nil {
		return nil, err
	}
	typeName := "Entity"
	if opts.TypeName != "" {
		typeName = opts.TypeName
	} else if t := opts.TypeSchema(); t != nil && t.Key != "" {
		typeName = t.Key
	}
	payload := map[string]any{
		"@odata.context": "$metadata#" + typeName,
		"@odata.type":    "#" + typeName,
		"value":          json.RawMessage(inner),
	}
	return json.Marshal(payload)
}

func ParseOData(data []byte) (*uir.Node, error) {
	return ParseODataWithOptions(data, Options{})
}

func ParseODataWithOptions(data []byte, opts Options) (*uir.Node, error) {
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(data, &payload); err != nil {
		return lexer.ParseJSON(data)
	}
	if _, hasCtx := payload["@odata.context"]; !hasCtx {
		if _, hasType := payload["@odata.type"]; !hasType {
			return lexer.ParseJSON(data)
		}
	}
	valueBytes, hasValue := payload["value"]
	if !hasValue {
		entity := map[string]json.RawMessage{}
		for k, v := range payload {
			if strings.HasPrefix(k, "@odata") {
				continue
			}
			entity[k] = v
		}
		var err error
		valueBytes, err = json.Marshal(entity)
		if err != nil {
			return nil, err
		}
	}
	node, err := parseJSONFlexible(valueBytes)
	if err != nil {
		return nil, err
	}
	var typeAnnotation, contextAnnotation string
	if json.Unmarshal(payload["@odata.type"], &typeAnnotation) == nil {
		t := typeAnnotation
		node.SetAnnotation("edm_type", strings.TrimPrefix(t, "#"))
	}
	if json.Unmarshal(payload["@odata.context"], &contextAnnotation) == nil {
		node.SetAnnotation("odata_context", contextAnnotation)
	}
	if opts.Schema != nil {
		projected, err := uir.Project(node, requireType(opts, node), uir.DefaultProjectOptions())
		if err != nil {
			return nil, fmt.Errorf("odata edm projection: %w", err)
		}
		return projected, nil
	}
	return node, nil
}

func parseJSONFlexible(data []byte) (*uir.Node, error) {
	return lexer.ParseJSON(data)
}

func edmTypeOf(t uir.UIRType) string {
	switch t {
	case uir.TypeString:
		return "Edm.String"
	case uir.TypeInt32:
		return "Edm.Int32"
	case uir.TypeInt64:
		return "Edm.Int64"
	case uir.TypeFloat64:
		return "Edm.Double"
	case uir.TypeFloat32:
		return "Edm.Single"
	case uir.TypeBoolean:
		return "Edm.Boolean"
	case uir.TypeBytes:
		return "Edm.Binary"
	case uir.TypeTimestamp:
		return "Edm.DateTimeOffset"
	case uir.TypeDate:
		return "Edm.Date"
	case uir.TypeTime:
		return "Edm.TimeOfDay"
	case uir.TypeDuration:
		return "Edm.Duration"
	case uir.TypeDecimal:
		return "Edm.Decimal"
	default:
		return "Edm.String"
	}
}
