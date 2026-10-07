package lexer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"

	"omni-schema/internal/uir"
)

// ParseJSON parses a JSON payload and maps it into a UIR Node structure directly.
func ParseJSON(data []byte) (*uir.Node, error) {
	var payload any
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&payload); err != nil {
		return nil, err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return nil, fmt.Errorf("JSON must contain exactly one value")
	}
	if err := validateJSONNumbers(payload, 0); err != nil {
		return nil, err
	}
	return valueToUIR("Root", payload), nil
}

func validateJSONNumbers(value any, depth int) error {
	if depth > 512 {
		return fmt.Errorf("JSON nesting exceeds 512")
	}
	switch v := value.(type) {
	case json.Number:
		if !strings.ContainsAny(string(v), ".eE") {
			if _, err := v.Int64(); err == nil {
				return nil
			}
			if _, err := strconv.ParseUint(string(v), 10, 64); err == nil {
				return nil
			}
			return fmt.Errorf("JSON integer %s exceeds supported 64-bit range", v)
		}
		f, err := v.Float64()
		if err != nil {
			return fmt.Errorf("invalid JSON number: %w", err)
		}
		mantissa := strings.SplitN(strings.ToLower(string(v)), "e", 2)[0]
		if f == 0 && strings.ContainsAny(mantissa, "123456789") {
			return fmt.Errorf("JSON number %s underflows float64", v)
		}
	case []any:
		for _, child := range v {
			if err := validateJSONNumbers(child, depth+1); err != nil {
				return err
			}
		}
	case map[string]any:
		for _, child := range v {
			if err := validateJSONNumbers(child, depth+1); err != nil {
				return err
			}
		}
	}
	return nil
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
		if string(val) == "-0" {
			return uir.NewNode(uir.TypeFloat64, key, math.Copysign(0, -1))
		}
		if i, err := val.Int64(); err == nil {
			return uir.NewNode(uir.TypeInt64, key, i)
		}
		if u, err := strconv.ParseUint(string(val), 10, 64); err == nil {
			return uir.NewNode(uir.TypeUInt64, key, u)
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
		if len(child.Children) > 0 {
			child.ElementType = child.Children[0].Type
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
