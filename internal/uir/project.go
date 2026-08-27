package uir

// Project takes a data UIR graph and a schema UIR graph and returns a new UIR graph
// containing only the fields declared in the schema, with types coerced to match the
// schema's type declarations. Fields present in the data but absent from the schema
// are dropped (projection semantics). Fields declared in the schema but absent from
// the data are emitted with zero-values if the schema marks them non-null.
//
// This is the core mechanism that turns a registered schema from a validation gate
// into a genuine transformation constraint.
func Project(data *Node, schema *Node) *Node {
	return projectNode(data, schema, schema.Key)
}

func projectNode(data *Node, schema *Node, typeName string) *Node {
	projected := NewNode(schema.Type, typeName, nil)

	// Copy schema annotations onto the projected node so codecs can use them.
	for k, v := range schema.TypeAnnotations {
		projected.SetAnnotation(k, v)
	}
	projected.ElementType = schema.ElementType

	if schema.Type != TypeMap || len(schema.Children) == 0 {
		// Leaf or scalar node: use schema type, data value.
		projected.Value = coerceValue(data, schema.Type)
		return projected
	}

	// Build a lookup index on the data children by key for O(1) matching.
	dataIndex := make(map[string]*Node, len(data.Children))
	for _, dc := range data.Children {
		dataIndex[dc.Key] = dc
	}

	for _, schemaField := range schema.Children {
		dataChild, found := dataIndex[schemaField.Key]

		if !found {
			// Field is declared in the schema but absent from the data.
			// Emit a zero-value node so the output is structurally complete.
			zeroNode := zeroValueNode(schemaField)
			projected.AddChild(zeroNode)
			continue
		}

		if schemaField.Type == TypeMap && len(schemaField.Children) > 0 {
			// Recurse: the schema defines nested structure for this field.
			childProjected := projectNode(dataChild, schemaField, schemaField.Key)
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
						projElem := projectNode(elem, schemaField, schemaField.Key+"_item")
						arrayNode.AddChild(projElem)
					} else {
						scalarElem := NewNode(schemaField.ElementType, elem.Key, coerceValue(elem, schemaField.ElementType))
						arrayNode.AddChild(scalarElem)
					}
				}
			}
			projected.AddChild(arrayNode)
		} else {
			// Scalar field: use schema type, data value.
			fieldNode := NewNode(schemaField.Type, schemaField.Key, coerceValue(dataChild, schemaField.Type))
			for ak, av := range schemaField.TypeAnnotations {
				fieldNode.SetAnnotation(ak, av)
			}
			projected.AddChild(fieldNode)
		}
	}

	return projected
}

// coerceValue extracts the value from a data node and performs basic type coercion
// to match the target UIR type. This handles the common JSON float64 → Int32 case.
func coerceValue(data *Node, targetType UIRType) any {
	if data == nil || data.Value == nil {
		return zeroForType(targetType)
	}

	val := data.Value

	switch targetType {
	case TypeInt32, TypeInt64:
		// JSON numbers are always float64; coerce to int.
		if f, ok := val.(float64); ok {
			return int64(f)
		}
		return val
	case TypeFloat64:
		if i, ok := val.(int64); ok {
			return float64(i)
		}
		return val
	case TypeString:
		if s, ok := val.(string); ok {
			return s
		}
		return val
	case TypeBoolean:
		if b, ok := val.(bool); ok {
			return b
		}
		return val
	default:
		return val
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
