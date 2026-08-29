package uir

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"math"
	"strconv"
	"strings"
)

type UnknownFieldPolicy int

const (
	UnknownFieldIgnore UnknownFieldPolicy = iota
	UnknownFieldStrict
	UnknownFieldPreserve
)

type LossinessPolicy int

const (
	LossyStrict     LossinessPolicy = iota // No precision loss, no overflow
	LossySafe                              // widening coercions only
	LossyPermissive                        // allow truncation with report
)

type NameMatchPolicy int

const (
	NameMatchExact NameMatchPolicy = iota
	NameMatchFold
	NameMatchSnakeCamel
)

type ProjectOptions struct {
	UnknownFields      UnknownFieldPolicy
	EmitNullForMissing bool
	Lossiness          LossinessPolicy
	Bytes              BytesPolicy
	NameMatch          NameMatchPolicy
	SchemaRoot         *Node
	Report             *ConversionReport
}

func DefaultProjectOptions() ProjectOptions {
	return ProjectOptions{
		UnknownFields:      UnknownFieldIgnore,
		EmitNullForMissing: false,
		Lossiness:          LossyStrict,
		Bytes:              BytesBase64,
		NameMatch:          NameMatchSnakeCamel,
		Report:             &ConversionReport{},
	}
}

// Project takes a data UIR graph and a schema UIR graph and returns a new UIR graph
// containing only the fields declared in the schema, with types coerced to match the
// schema's type declarations.
func Project(data *Node, schema *Node, opts ...ProjectOptions) (*Node, error) {
	opt := DefaultProjectOptions()
	if len(opts) > 0 {
		opt = opts[0]
		if opt.Report == nil {
			opt.Report = &ConversionReport{}
		}
		if opt.Bytes == "" {
			opt.Bytes = BytesBase64
		}
	}
	root := opt.SchemaRoot
	if root == nil && schema != nil {
		root = schema
		for root.Parent != nil {
			root = root.Parent
		}
		opt.SchemaRoot = root
	}
	return projectNode(data, schema, schema.Key, opt)
}

func projectNode(data *Node, schema *Node, typeName string, opt ProjectOptions) (*Node, error) {
	if schema == nil {
		return data, nil
	}

	if schema.Type == TypeRef {
		resolved := resolveRef(schema, opt.SchemaRoot)
		if resolved != nil {
			schema = resolved
		}
	}

	if schema.Type != TypeMap && schema.Type != TypeArray && schema.Type != TypeUnion && schema.Type != TypeInterface && schema.Type != TypeDefinition {
		projected := NewNode(schema.Type, typeName, nil)
		copyMeta(projected, schema)
		if data == nil || data.Presence == PresenceMissing {
			return applyMissing(schema, typeName, opt)
		}
		if data.Type == TypeNull || data.Presence == PresenceNull {
			if schema.Cardinality == CardinalityRequired || schema.Annotation("nonNull") == "true" {
				return nil, fmt.Errorf("field '%s': required field is null", schema.Key)
			}
			projected.Type = TypeNull
			projected.Presence = PresenceNull
			return projected, nil
		}
		val, kind, err := coerceValue(data, schema.Type, opt)
		if err != nil {
			return nil, fmt.Errorf("field '%s': %w", schema.Key, err)
		}
		opt.Report.Add(kind, schema.Key, "")
		projected.Value = val
		projected.Presence = PresencePresent
		return projected, nil
	}

	if schema.Type == TypeArray {
		return projectArray(data, schema, typeName, opt)
	}

	projected := NewNode(TypeMap, typeName, nil)
	copyMeta(projected, schema)

	if data == nil || data.Presence == PresenceMissing {
		return applyMissing(schema, typeName, opt)
	}

	dataIndex := indexDataChildren(data)
	matchedData := make(map[string]bool)

	for _, schemaField := range schema.Children {
		dataChild, found := lookupDataField(dataIndex, schemaField, opt.NameMatch)
		if !found {
			child, err := applyMissing(schemaField, schemaField.Key, opt)
			if err != nil {
				return nil, err
			}
			if child != nil {
				projected.AddChild(child)
			}
			continue
		}
		matchedData[dataChild.Key] = true

		targetSchema := schemaField
		if schemaField.Type == TypeRef || schemaField.Type == TypeMap && schemaField.Annotation("gql_type") != "" && len(schemaField.Children) == 0 {
			if resolved := resolveNamed(schemaField.Annotation("gql_type"), opt.SchemaRoot); resolved != nil {
				targetSchema = resolved.CloneShallow()
				targetSchema.Key = schemaField.Key
				targetSchema.Cardinality = schemaField.Cardinality
				for k, v := range schemaField.TypeAnnotations {
					targetSchema.SetAnnotation(k, v)
				}
				targetSchema.Children = resolved.Children
				targetSchema.Type = resolved.Type
			}
		}

		childProjected, err := projectNode(dataChild, targetSchema, schemaField.Key, opt)
		if err != nil {
			return nil, err
		}
		if childProjected != nil {
			childProjected.Key = schemaField.Key
			projected.AddChild(childProjected)
		}
	}

	if opt.UnknownFields != UnknownFieldIgnore {
		for _, dc := range data.Children {
			if !matchedData[dc.Key] {
				if opt.UnknownFields == UnknownFieldStrict {
					return nil, fmt.Errorf("unknown field in data: %s", dc.Key)
				}
				projected.AddChild(dc)
			}
		}
	}

	return projected, nil
}

func projectArray(data *Node, schema *Node, typeName string, opt ProjectOptions) (*Node, error) {
	arrayNode := NewNode(TypeArray, typeName, nil)
	copyMeta(arrayNode, schema)

	if data == nil || data.Presence == PresenceMissing {
		return applyMissing(schema, typeName, opt)
	}
	if data.Type != TypeArray {
		return nil, fmt.Errorf("field '%s': expected array", typeName)
	}

	elemSchema := schema
	if len(schema.Children) > 0 {
		elemSchema = schema.Children[0]
	}

	for _, elem := range data.Children {
		if schema.ElementType == TypeMap || elemSchema.Type == TypeMap || elemSchema.Type == TypeRef {
			projElem, err := projectNode(elem, elemSchema, schema.Key+"_item", opt)
			if err != nil {
				return nil, err
			}
			arrayNode.AddChild(projElem)
		} else {
			val, kind, err := coerceValue(elem, schema.ElementType, opt)
			if err != nil {
				return nil, fmt.Errorf("array element '%s': %w", schema.Key, err)
			}
			opt.Report.Add(kind, schema.Key, "")
			scalarElem := NewNode(schema.ElementType, elem.Key, val)
			arrayNode.AddChild(scalarElem)
		}
	}
	return arrayNode, nil
}

func applyMissing(schema *Node, typeName string, opt ProjectOptions) (*Node, error) {
	if schema.DefaultValue != nil || schema.Annotation("default") != "" {
		def := schema.DefaultValue
		if def == nil {
			def = schema.Annotation("default")
			tmp := NewNode(TypeString, schema.Key, def)
			v, _, err := coerceValue(tmp, schema.Type, ProjectOptions{Lossiness: LossySafe, Bytes: opt.Bytes, Report: opt.Report})
			if err == nil {
				def = v
			}
		}
		n := NewNode(schema.Type, typeName, def)
		copyMeta(n, schema)
		n.Presence = PresenceDefaulted
		return n, nil
	}
	required := schema.Cardinality == CardinalityRequired || schema.Annotation("nonNull") == "true"
	if required {
		return nil, fmt.Errorf("missing required field: %s", schema.Key)
	}
	if opt.EmitNullForMissing {
		n := NewNode(TypeNull, typeName, nil)
		n.Presence = PresenceNull
		copyMeta(n, schema)
		n.Type = TypeNull
		return n, nil
	}
	n := NewNode(schema.Type, typeName, nil)
	copyMeta(n, schema)
	n.Presence = PresenceMissing
	return nil, nil
}

func copyMeta(dst, src *Node) {
	if src == nil {
		return
	}
	for k, v := range src.TypeAnnotations {
		dst.SetAnnotation(k, v)
	}
	dst.ElementType = src.ElementType
	dst.TypeExpr = src.TypeExpr
	dst.Cardinality = src.Cardinality
	dst.DefaultValue = src.DefaultValue
}

func resolveRef(schema, root *Node) *Node {
	name := schema.Annotation("gql_type")
	if name == "" {
		name = schema.Key
	}
	return resolveNamed(name, root)
}

func resolveNamed(name string, root *Node) *Node {
	if name == "" || root == nil {
		return nil
	}
	return root.FindNamedType(name)
}

func indexDataChildren(data *Node) map[string]*Node {
	idx := make(map[string]*Node)
	if data == nil {
		return idx
	}
	for _, dc := range data.Children {
		idx[dc.Key] = dc
	}
	return idx
}

func lookupDataField(idx map[string]*Node, schemaField *Node, policy NameMatchPolicy) (*Node, bool) {
	if n, ok := idx[schemaField.Key]; ok {
		return n, true
	}
	if protoNum := schemaField.Annotation("proto_number"); protoNum != "" {
		if n, ok := idx[protoNum]; ok {
			return n, true
		}
	}
	if alias := schemaField.Annotation("alias"); alias != "" {
		if n, ok := idx[alias]; ok {
			return n, true
		}
	}
	if policy == NameMatchExact {
		return nil, false
	}
	want := normalizeName(schemaField.Key)
	for k, n := range idx {
		if normalizeName(k) == want {
			return n, true
		}
		if policy == NameMatchFold && strings.EqualFold(k, schemaField.Key) {
			return n, true
		}
	}
	return nil, false
}

func normalizeName(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r == '_' || r == '-' {
			continue
		}
		if r >= 'A' && r <= 'Z' {
			b.WriteRune(r + 32)
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func coerceValue(data *Node, targetType UIRType, opt ProjectOptions) (any, ConversionKind, error) {
	if data == nil || data.Value == nil || data.Type == TypeNull {
		return zeroForType(targetType), ConversionLossless, nil
	}
	if targetType == TypeBytes {
		return coerceBytes(data, opt)
	}
	if targetType == TypeString && data.Type == TypeBytes {
		return bytesToText(data.Value, opt)
	}

	entry := LookupCompat(data.Type, targetType)
	if !entry.Allowed && data.Type != targetType && !sameNumericFamily(data.Type, targetType) {
		if opt.Lossiness == LossyPermissive {
			opt.Report.Add(ConversionLossy, data.Key, fmt.Sprintf("%s -> %s forced", data.Type, targetType))
		} else if data.Type != TypeUnknown {
			// still attempt numeric/string paths below
		}
	}

	val := data.Value
	kind := ConversionLossless
	if data.Type != targetType && data.Type != TypeUnknown {
		if entry.Lossless {
			kind = ConversionLossless
		} else if entry.Safe {
			kind = ConversionSafeCoerce
		} else {
			kind = ConversionLossy
		}
	}

	switch targetType {
	case TypeInt32, TypeSInt32, TypeSFixed32:
		i, k, err := toInt64(val, opt.Lossiness)
		if err != nil {
			return nil, k, err
		}
		if i < math.MinInt32 || i > math.MaxInt32 {
			if opt.Lossiness != LossyPermissive {
				return nil, ConversionUnsupported, fmt.Errorf("cannot coerce %v to int32: overflow", val)
			}
			opt.Report.Add(ConversionLossy, data.Key, "int32 overflow truncated")
		}
		return int32(i), mergeKind(kind, k), nil
	case TypeInt64, TypeSInt64, TypeSFixed64:
		i, k, err := toInt64(val, opt.Lossiness)
		if err != nil {
			return nil, k, err
		}
		return i, mergeKind(kind, k), nil
	case TypeUInt32, TypeFixed32:
		u, k, err := toUint64(val, opt.Lossiness)
		if err != nil {
			return nil, k, err
		}
		if u > math.MaxUint32 {
			if opt.Lossiness != LossyPermissive {
				return nil, ConversionUnsupported, fmt.Errorf("cannot coerce %v to uint32: overflow", val)
			}
			opt.Report.Add(ConversionLossy, data.Key, "uint32 overflow truncated")
		}
		return uint32(u), mergeKind(kind, k), nil
	case TypeUInt64, TypeFixed64:
		u, k, err := toUint64(val, opt.Lossiness)
		if err != nil {
			return nil, k, err
		}
		return u, mergeKind(kind, k), nil
	case TypeFloat32:
		f, k, err := toFloat64(val, opt.Lossiness)
		if err != nil {
			return nil, k, err
		}
		if opt.Lossiness == LossyStrict && float64(float32(f)) != f && !math.IsInf(f, 0) && !math.IsNaN(f) {
			return nil, ConversionUnsupported, fmt.Errorf("cannot safely coerce float64 %v to float32: precision loss", f)
		}
		if float64(float32(f)) != f {
			k = ConversionLossy
		}
		return float32(f), mergeKind(kind, k), nil
	case TypeFloat64:
		f, k, err := toFloat64(val, opt.Lossiness)
		if err != nil {
			return nil, k, err
		}
		return f, mergeKind(kind, k), nil
	case TypeString, TypeEnum, TypeTimestamp, TypeDate, TypeTime, TypeDuration, TypeDecimal:
		if s, ok := val.(string); ok {
			return s, kind, nil
		}
		return fmt.Sprint(val), ConversionSafeCoerce, nil
	case TypeBoolean:
		switch v := val.(type) {
		case bool:
			return v, kind, nil
		case int64:
			return v != 0, ConversionSafeCoerce, nil
		case string:
			b, err := strconv.ParseBool(v)
			if err != nil {
				return nil, ConversionUnsupported, fmt.Errorf("cannot coerce %q to bool", v)
			}
			return b, ConversionSafeCoerce, nil
		default:
			return val, kind, nil
		}
	case TypeNull:
		return nil, kind, nil
	default:
		return val, kind, nil
	}
}

func coerceBytes(data *Node, opt ProjectOptions) (any, ConversionKind, error) {
	switch v := data.Value.(type) {
	case []byte:
		return v, ConversionLossless, nil
	case string:
		switch opt.Bytes {
		case BytesHex:
			b, err := hex.DecodeString(v)
			if err != nil {
				return nil, ConversionUnsupported, fmt.Errorf("invalid hex bytes: %w", err)
			}
			return b, ConversionSafeCoerce, nil
		case BytesReject:
			return nil, ConversionUnsupported, fmt.Errorf("refusing to interpret string as bytes")
		default:
			b, err := base64.StdEncoding.DecodeString(v)
			if err != nil {
				return []byte(v), ConversionLossy, nil
			}
			return b, ConversionSafeCoerce, nil
		}
	default:
		return nil, ConversionUnsupported, fmt.Errorf("cannot coerce %T to bytes", data.Value)
	}
}

func bytesToText(val any, opt ProjectOptions) (any, ConversionKind, error) {
	b, ok := val.([]byte)
	if !ok {
		return fmt.Sprint(val), ConversionSafeCoerce, nil
	}
	switch opt.Bytes {
	case BytesHex:
		return hex.EncodeToString(b), ConversionSafeCoerce, nil
	case BytesReject:
		return nil, ConversionUnsupported, fmt.Errorf("refusing to encode bytes as text")
	case BytesCustomScalar:
		return base64.StdEncoding.EncodeToString(b), ConversionSafeCoerce, nil
	default:
		return base64.StdEncoding.EncodeToString(b), ConversionSafeCoerce, nil
	}
}

func toInt64(val any, lossiness LossinessPolicy) (int64, ConversionKind, error) {
	switch v := val.(type) {
	case int64:
		return v, ConversionLossless, nil
	case int32:
		return int64(v), ConversionLossless, nil
	case int:
		return int64(v), ConversionLossless, nil
	case uint32:
		return int64(v), ConversionLossless, nil
	case uint64:
		if v > math.MaxInt64 {
			if lossiness != LossyPermissive {
				return 0, ConversionUnsupported, fmt.Errorf("cannot safely coerce uint64 %d to int64: overflow", v)
			}
			return int64(v), ConversionLossy, nil
		}
		return int64(v), ConversionSafeCoerce, nil
	case float32:
		return floatToInt64(float64(v), lossiness)
	case float64:
		return floatToInt64(v, lossiness)
	case string:
		i, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return 0, ConversionUnsupported, fmt.Errorf("cannot coerce %q to int", v)
		}
		return i, ConversionSafeCoerce, nil
	case bool:
		if v {
			return 1, ConversionSafeCoerce, nil
		}
		return 0, ConversionSafeCoerce, nil
	default:
		return 0, ConversionUnsupported, fmt.Errorf("cannot coerce %T to int", val)
	}
}

func floatToInt64(f float64, lossiness LossinessPolicy) (int64, ConversionKind, error) {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return 0, ConversionUnsupported, fmt.Errorf("cannot coerce non-finite float %v to int", f)
	}
	if f != math.Trunc(f) {
		if lossiness == LossyStrict {
			return 0, ConversionUnsupported, fmt.Errorf("cannot safely coerce float %v to int: precision loss", f)
		}
		if lossiness == LossySafe {
			return 0, ConversionUnsupported, fmt.Errorf("cannot safely coerce float %v to int: rounding required", f)
		}
		if f > math.MaxInt64 || f < math.MinInt64 {
			return 0, ConversionUnsupported, fmt.Errorf("cannot coerce float %v to int: overflow", f)
		}
		return int64(f), ConversionLossy, nil
	}
	if f > math.MaxInt64 || f < math.MinInt64 {
		return 0, ConversionUnsupported, fmt.Errorf("cannot coerce float %v to int: overflow", f)
	}
	return int64(f), ConversionSafeCoerce, nil
}

func toUint64(val any, lossiness LossinessPolicy) (uint64, ConversionKind, error) {
	switch v := val.(type) {
	case uint64:
		return v, ConversionLossless, nil
	case uint32:
		return uint64(v), ConversionLossless, nil
	case int64:
		if v < 0 {
			if lossiness != LossyPermissive {
				return 0, ConversionUnsupported, fmt.Errorf("cannot safely coerce negative int to uint")
			}
			return uint64(v), ConversionLossy, nil
		}
		return uint64(v), ConversionSafeCoerce, nil
	case int32:
		if v < 0 {
			if lossiness != LossyPermissive {
				return 0, ConversionUnsupported, fmt.Errorf("cannot safely coerce negative int to uint")
			}
			return uint64(v), ConversionLossy, nil
		}
		return uint64(v), ConversionSafeCoerce, nil
	case float64:
		if v < 0 || v != math.Trunc(v) {
			if lossiness != LossyPermissive {
				return 0, ConversionUnsupported, fmt.Errorf("cannot safely coerce float %v to uint", v)
			}
			return uint64(v), ConversionLossy, nil
		}
		if v > float64(math.MaxUint64) {
			return 0, ConversionUnsupported, fmt.Errorf("cannot coerce float %v to uint: overflow", v)
		}
		return uint64(v), ConversionSafeCoerce, nil
	default:
		i, k, err := toInt64(val, lossiness)
		if err != nil {
			return 0, k, err
		}
		if i < 0 {
			if lossiness != LossyPermissive {
				return 0, ConversionUnsupported, fmt.Errorf("cannot safely coerce negative int to uint")
			}
			return uint64(i), ConversionLossy, nil
		}
		return uint64(i), k, nil
	}
}

func toFloat64(val any, lossiness LossinessPolicy) (float64, ConversionKind, error) {
	switch v := val.(type) {
	case float64:
		return v, ConversionLossless, nil
	case float32:
		return float64(v), ConversionSafeCoerce, nil
	case int64:
		f := float64(v)
		if lossiness == LossyStrict && int64(f) != v {
			return 0, ConversionUnsupported, fmt.Errorf("cannot safely coerce int64 %d to float64: precision loss", v)
		}
		if int64(f) != v {
			return f, ConversionLossy, nil
		}
		return f, ConversionSafeCoerce, nil
	case int32:
		return float64(v), ConversionLossless, nil
	case uint64:
		f := float64(v)
		if lossiness == LossyStrict && uint64(f) != v {
			return 0, ConversionUnsupported, fmt.Errorf("cannot safely coerce uint64 %d to float64: precision loss", v)
		}
		return f, ConversionSafeCoerce, nil
	case uint32:
		return float64(v), ConversionLossless, nil
	case string:
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return 0, ConversionUnsupported, fmt.Errorf("cannot coerce %q to float", v)
		}
		return f, ConversionSafeCoerce, nil
	default:
		return 0, ConversionUnsupported, fmt.Errorf("cannot coerce %T to float", val)
	}
}

func mergeKind(a, b ConversionKind) ConversionKind {
	if b > a {
		return b
	}
	return a
}

func sameNumericFamily(a, b UIRType) bool {
	return (a.IsInteger() || a.IsFloat()) && (b.IsInteger() || b.IsFloat())
}

func zeroForType(t UIRType) any {
	switch t {
	case TypeNull:
		return nil
	case TypeBytes:
		return []byte{}
	case TypeString, TypeEnum, TypeTimestamp, TypeDate, TypeTime, TypeDuration, TypeDecimal:
		return ""
	case TypeInt32, TypeSInt32, TypeSFixed32:
		return int32(0)
	case TypeInt64, TypeSInt64, TypeSFixed64:
		return int64(0)
	case TypeUInt32, TypeFixed32:
		return uint32(0)
	case TypeUInt64, TypeFixed64:
		return uint64(0)
	case TypeFloat32:
		return float32(0)
	case TypeFloat64:
		return float64(0)
	case TypeBoolean:
		return false
	default:
		return nil
	}
}
