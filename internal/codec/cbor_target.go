package codec

import (
	"fmt"
	"math"

	"omni-schema/internal/uir"
)

// ValidateCBORTarget rejects values that the target subset would silently
// truncate or discard. Broader target models require a separate codec extension.
func ValidateCBORTarget(n *uir.Node, target string) error {
	target = NormalizeFormat(target)
	if n != nil && n.Type == uir.TypeArray && (target == "parquet" || target == "hdf5" || target == "avro") && len(n.Children) > 0 {
		first := n.Children[0]
		if first == nil || first.Type != uir.TypeMap {
			return fmt.Errorf("%s requires an array of records", target)
		}
		for _, row := range n.Children {
			if row == nil || row.Type != uir.TypeMap || len(row.Children) != len(first.Children) {
				return fmt.Errorf("%s requires homogeneous records", target)
			}
			for _, field := range first.Children {
				other := row.ChildByKey(field.Key)
				if other == nil || other.Type != field.Type {
					return fmt.Errorf("%s requires consistent field types in all records", target)
				}
			}
		}
	}
	return validateCBORTarget(n, target, 0)
}

func validateCBORTarget(n *uir.Node, target string, depth int) error {
	if n == nil {
		return nil
	}
	if depth > cborMaxDepth {
		return fmt.Errorf("cbor: nesting exceeds %d", cborMaxDepth)
	}
	if target == "graphql" {
		if depth == 0 && n.Type != uir.TypeMap {
			return fmt.Errorf("graphql SDL requires an object")
		}
		if n.Type == uir.TypeMap && len(n.Children) == 0 {
			return fmt.Errorf("graphql SDL cannot represent an empty object")
		}
		if n.Parent != nil && n.Parent.Type == uir.TypeMap && !graphqlFieldName(n.Key) {
			return fmt.Errorf("graphql: invalid field name %q", n.Key)
		}
	}
	if target == "json" || target == "odata" {
		var f float64
		switch v := n.Value.(type) {
		case float32:
			f = float64(v)
		case float64:
			f = v
		}
		if math.IsNaN(f) || math.IsInf(f, 0) {
			return fmt.Errorf("%s cannot represent non-finite floating-point values", target)
		}
	}
	if target == "avro" || target == "parquet" || target == "hdf5" {
		if depth == 0 && n.Type != uir.TypeMap && n.Type != uir.TypeArray {
			return fmt.Errorf("%s requires a record or array of records", target)
		}
		if depth == 0 && n.Type == uir.TypeArray {
			for _, child := range n.Children {
				if child.Type != uir.TypeMap {
					return fmt.Errorf("%s requires an array of records", target)
				}
			}
		}
		fieldDepth := 1
		if n.Parent != nil && n.Parent.Type == uir.TypeMap && n.Parent.Parent != nil && n.Parent.Parent.Type == uir.TypeArray {
			fieldDepth = 2
		}
		if depth >= fieldDepth && n.Type != uir.TypeMap && n.Type != uir.TypeArray {
			if target == "parquet" && n.Type == uir.TypeBytes {
				return fmt.Errorf("parquet subset cannot preserve native byte strings")
			}
			if v, ok := n.Value.(uint64); ok && v > math.MaxInt64 {
				return fmt.Errorf("%s cannot represent uint64 above MaxInt64", target)
			}
			if target == "parquet" || target == "hdf5" {
				if n.Type == uir.TypeNull {
					return fmt.Errorf("%s subset cannot preserve explicit null fields", target)
				}
			}
		}
		if depth > 0 && (n.Type == uir.TypeMap || n.Type == uir.TypeArray) && !(depth == 1 && n.Type == uir.TypeMap && n.Parent != nil && n.Parent.Type == uir.TypeArray) {
			return fmt.Errorf("%s subset cannot preserve nested containers", target)
		}
	}
	if (target == "protobuf" || target == "capnproto") && n.Type == uir.TypeNull {
		return fmt.Errorf("%s cannot preserve explicit null values", target)
	}
	if target == "capnproto" && depth == 0 && n.Type != uir.TypeMap {
		return fmt.Errorf("capnproto requires a struct object")
	}
	for _, child := range n.Children {
		if err := validateCBORTarget(child, target, depth+1); err != nil {
			return err
		}
	}
	return nil
}

func graphqlFieldName(key string) bool {
	if key == "" || len(key) >= 2 && key[:2] == "__" {
		return false
	}
	for i, c := range key {
		if !(c == '_' || c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || i > 0 && c >= '0' && c <= '9') {
			return false
		}
	}
	return true
}
