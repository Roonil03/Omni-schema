package uir

// CompatEntry describes whether a source UIR type can become a target UIR type.
type CompatEntry struct {
	Allowed  bool
	Lossless bool
	Safe     bool
	Note     string
}

// LookupCompat returns the central semantic type compatibility entry.
func LookupCompat(src, dst UIRType) CompatEntry {
	if src == dst {
		return CompatEntry{Allowed: true, Lossless: true, Safe: true, Note: "identity"}
	}
	key := [2]UIRType{src, dst}
	if e, ok := compatTable[key]; ok {
		return e
	}
	if src.IsInteger() && dst.IsInteger() {
		if integerWidth(dst) >= integerWidth(src) && unsignedness(src) == unsignedness(dst) {
			return CompatEntry{Allowed: true, Lossless: true, Safe: true, Note: "integer widen"}
		}
		return CompatEntry{Allowed: true, Lossless: false, Safe: false, Note: "integer narrow or signedness change; overflow checked"}
	}
	if src.IsInteger() && dst.IsFloat() {
		return CompatEntry{Allowed: true, Lossless: false, Safe: dst == TypeFloat64, Note: "int to float"}
	}
	if src.IsFloat() && dst.IsInteger() {
		return CompatEntry{Allowed: true, Lossless: false, Safe: false, Note: "float to int requires integral finite value"}
	}
	if src == TypeFloat32 && dst == TypeFloat64 {
		return CompatEntry{Allowed: true, Lossless: true, Safe: true, Note: "float widen"}
	}
	if src == TypeFloat64 && dst == TypeFloat32 {
		return CompatEntry{Allowed: true, Lossless: false, Safe: false, Note: "float64 to float32 may lose precision"}
	}
	if src == TypeBytes && dst == TypeString {
		return CompatEntry{Allowed: true, Lossless: false, Safe: true, Note: "bytes to text uses BytesPolicy"}
	}
	if src == TypeString && dst == TypeBytes {
		return CompatEntry{Allowed: true, Lossless: false, Safe: true, Note: "text to bytes uses BytesPolicy"}
	}
	if src == TypeEnum && dst == TypeString || src == TypeString && dst == TypeEnum {
		return CompatEntry{Allowed: true, Lossless: true, Safe: true, Note: "enum/string"}
	}
	if src.IsLogical() && (dst == TypeString || dst.IsLogical()) {
		return CompatEntry{Allowed: true, Lossless: true, Safe: true, Note: "logical type as string"}
	}
	if src == TypeNull {
		return CompatEntry{Allowed: true, Lossless: true, Safe: true, Note: "null"}
	}
	return CompatEntry{Allowed: false, Note: "unsupported"}
}

func integerWidth(t UIRType) int {
	switch t {
	case TypeInt32, TypeUInt32, TypeSInt32, TypeFixed32, TypeSFixed32:
		return 32
	default:
		return 64
	}
}

func unsignedness(t UIRType) int {
	switch t {
	case TypeUInt32, TypeUInt64, TypeFixed32, TypeFixed64:
		return 1
	default:
		return 0
	}
}

var compatTable = map[[2]UIRType]CompatEntry{
	{TypeInt32, TypeInt64}:     {true, true, true, "widen"},
	{TypeInt32, TypeFloat64}:   {true, true, true, "exact in float64"},
	{TypeBoolean, TypeInt32}:   {true, true, true, "bool as 0/1"},
	{TypeBoolean, TypeInt64}:   {true, true, true, "bool as 0/1"},
}
