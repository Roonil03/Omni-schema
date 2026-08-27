package uir

import (
	"fmt"
	"math"
)

// Project takes a data UIR graph and a schema UIR graph and returns a new UIR graph
// containing only the fields declared in the schema, with types coerced to match the
// schema's type declarations. Fields present in the data but absent from the schema
// are dropped (projection semantics). Fields declared in the schema but absent from
// the data are evaluated strictly: if marked nonNull, it returns an error. If optional,
// they are dropped.
//
// This is the core mechanism that turns a registered schema from a validation gate
// into a genuine transformation constraint.
func Project(data *Node, schema *Node) (*Node, error) {
	return projectNode(data, schema, schema.Key)
}

func projectNode(data *Node, schema *Node, typeName string) (*Node, error) {
	projected := NewNode(schema.Type, typeName, nil)

	// Copy schema annotations onto the projected node so codecs can use them.
	for k, v := range schema.TypeAnnotations {
		projected.SetAnnotation(k, v)
	}
	projected.ElementType = schema.ElementType

	if schema.Type != TypeMap || len(schema.Children) == 0 {
		// Leaf or scalar node: use schema type, data value.
		val, err := coerceValue(data, schema.Type)
		if err != nil {
			return nil, fmt.Errorf("field '%s': %w", schema.Key, err)
		}
		projected.Value = val
		return projected, nil
	}

	// Build a lookup index on the data children by key for O(1) matching.
	dataIndex := make(map[string]*Node, len(data.Children))
	for _, dc := range data.Children {
		dataIndex[dc.Key] = dc
	}

	for _, schemaField := range schema.Children {
		dataChild, found := dataIndex[schemaField.Key]
		
		if !found {
			// Alias-aware fallback: try matching by protobuf tag number if present
			if protoNum, ok := schemaField.TypeAnnotations["proto_number"]; ok {
				dataChild, found = dataIndex[protoNum]
			}
		}

		if !found {
			// Field is declared in the schema but absent from the data.
			if schemaField.TypeAnnotations["nonNull"] == "true" {
				return nil, fmt.Errorf("missing required field: %s", schemaField.Key)
			}
			// Optional field, just drop it.
			continue
		}

		if schemaField.Type == TypeMap && len(schemaField.Children) > 0 {
			// Recurse: the schema defines nested structure for this field.
			childProjected, err := projectNode(dataChild, schemaField, schemaField.Key)
			if err != nil {
				return nil, err
			}
			projected.AddChild(childProjected)
		} else if schemaField.Type == TypeArray {
			// For arrays, project each element if the schema defines element structure.
			arrayNode := NewNode(TypeArray, schemaField.Key, nil)
			arrayNode.ElementType = schemaField.ElementType
			for ak, av := range schemaField.TypeAnnotations {
				arrayNode.SetAnnotation(ak, av)
			}

			if dataChild.Type == TypeArray {
				for _, elem := range dataChild.Children {
					if schemaField.ElementType == TypeMap && len(schemaField.Children) > 0 {
						projElem, err := projectNode(elem, schemaField, schemaField.Key+"_item")
						if err != nil {
							return nil, err
						}
						arrayNode.AddChild(projElem)
					} else {
						val, err := coerceValue(elem, schemaField.ElementType)
						if err != nil {
							return nil, fmt.Errorf("array element '%s': %w", schemaField.Key, err)
						}
						scalarElem := NewNode(schemaField.ElementType, elem.Key, val)
						arrayNode.AddChild(scalarElem)
					}
				}
			}
			projected.AddChild(arrayNode)
		} else {
			// Scalar field: use schema type, data value.
			val, err := coerceValue(dataChild, schemaField.Type)
			if err != nil {
				return nil, fmt.Errorf("field '%s': %w", schemaField.Key, err)
			}
			fieldNode := NewNode(schemaField.Type, schemaField.Key, val)
			for ak, av := range schemaField.TypeAnnotations {
				fieldNode.SetAnnotation(ak, av)
			}
			projected.AddChild(fieldNode)
		}
	}

	return projected, nil
}

// coerceValue extracts the value from a data node and performs strict type coercion
// to match the target UIR type, avoiding silent precision loss or overflows.
func coerceValue(data *Node, targetType UIRType) (any, error) {
	if data == nil || data.Value == nil {
		return zeroForType(targetType), nil
	}

	val := data.Value

	switch targetType {
	case TypeInt32, TypeInt64:
		if f, ok := val.(float64); ok {
			// Check for fractional loss
			if f != math.Trunc(f) {
				return nil, fmt.Errorf("cannot safely coerce float %v to int: precision loss", f)
			}
			// Check bounds for int64
			if f > math.MaxInt64 || f < math.MinInt64 {
				return nil, fmt.Errorf("cannot safely coerce float %v to int: integer overflow", f)
			}
			if targetType == TypeInt32 {
				if f > math.MaxInt32 || f < math.MinInt32 {
					return nil, fmt.Errorf("cannot safely coerce float %v to int32: integer overflow", f)
				}
			}
			return int64(f), nil
		}
		if i, ok := val.(int64); ok {
			return i, nil
		}
		return val, nil
	case TypeFloat64:
		if i, ok := val.(int64); ok {
			return float64(i), nil
		}
		if f, ok := val.(float64); ok {
			return f, nil
		}
		return val, nil
	case TypeString:
		if s, ok := val.(string); ok {
			return s, nil
		}
		return val, nil
	case TypeBoolean:
		if b, ok := val.(bool); ok {
			return b, nil
		}
		return val, nil
	default:
		return val, nil
	}
}

// zeroForType returns the zero-value for a given UIR type.
func zeroForType(t UIRType) any {
	switch t {
	case TypeString:
		return ""
	case TypeInt32, TypeInt64:
		return int64(0)
	case TypeFloat64:
		return float64(0)
	case TypeBoolean:
		return false
	default:
		return nil
	}
}

// zeroValueNode creates a minimal node with zero-value data for a schema field
// that was absent from the input data.
func zeroValueNode(schemaField *Node) *Node {
	n := NewNode(schemaField.Type, schemaField.Key, zeroForType(schemaField.Type))
	for k, v := range schemaField.TypeAnnotations {
		n.SetAnnotation(k, v)
	}
	n.ElementType = schemaField.ElementType

	// For nested maps, recursively create zero-value children.
	if schemaField.Type == TypeMap {
		for _, child := range schemaField.Children {
			n.AddChild(zeroValueNode(child))
		}
	}
	return n
}
