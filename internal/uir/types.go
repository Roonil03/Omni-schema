package uir

// UIRType defines the fundamental types in the Unified Intermediate Representation.
// This enforces the Universal Type Mapping Matrix and preserves protocol-specific
// numeric / logical distinctions required for faithful re-encoding.
type UIRType int

const (
	TypeUnknown UIRType = iota
	TypeNull            // UIR_Null
	TypeString          // UIR_String
	TypeBytes           // UIR_Bytes
	TypeInt32           // UIR_Int32
	TypeInt64           // UIR_Int64
	TypeUInt32          // UIR_UInt32
	TypeUInt64          // UIR_UInt64
	TypeSInt32          // protobuf sint32 (zigzag)
	TypeSInt64          // protobuf sint64 (zigzag)
	TypeFixed32         // protobuf fixed32
	TypeFixed64         // protobuf fixed64
	TypeSFixed32        // protobuf sfixed32
	TypeSFixed64        // protobuf sfixed64
	TypeFloat32         // protobuf float / parquet FLOAT
	TypeFloat64         // UIR_Float64
	TypeBoolean         // UIR_Boolean
	TypeArray           // UIR_Array[T]
	TypeMap             // UIR_Map[K,V]
	TypeEnum            // GraphQL/Protobuf enum
	TypeUnion           // GraphQL union / Avro union
	TypeInterface       // GraphQL interface
	TypeDefinition      // named type declaration
	TypeRef             // named type reference
	TypeTimestamp       // logical timestamp
	TypeDate            // logical date
	TypeTime            // logical time-of-day
	TypeDuration        // logical duration
	TypeDecimal         // logical decimal
)

func (t UIRType) String() string {
	switch t {
	case TypeNull:
		return "UIR_Null"
	case TypeString:
		return "UIR_String"
	case TypeBytes:
		return "UIR_Bytes"
	case TypeInt32:
		return "UIR_Int32"
	case TypeInt64:
		return "UIR_Int64"
	case TypeUInt32:
		return "UIR_UInt32"
	case TypeUInt64:
		return "UIR_UInt64"
	case TypeSInt32:
		return "UIR_SInt32"
	case TypeSInt64:
		return "UIR_SInt64"
	case TypeFixed32:
		return "UIR_Fixed32"
	case TypeFixed64:
		return "UIR_Fixed64"
	case TypeSFixed32:
		return "UIR_SFixed32"
	case TypeSFixed64:
		return "UIR_SFixed64"
	case TypeFloat32:
		return "UIR_Float32"
	case TypeFloat64:
		return "UIR_Float64"
	case TypeBoolean:
		return "UIR_Boolean"
	case TypeArray:
		return "UIR_Array"
	case TypeMap:
		return "UIR_Map"
	case TypeEnum:
		return "UIR_Enum"
	case TypeUnion:
		return "UIR_Union"
	case TypeInterface:
		return "UIR_Interface"
	case TypeDefinition:
		return "UIR_Definition"
	case TypeRef:
		return "UIR_Ref"
	case TypeTimestamp:
		return "UIR_Timestamp"
	case TypeDate:
		return "UIR_Date"
	case TypeTime:
		return "UIR_Time"
	case TypeDuration:
		return "UIR_Duration"
	case TypeDecimal:
		return "UIR_Decimal"
	default:
		return "UIR_Unknown"
	}
}

// IsInteger reports whether t is a signed or unsigned integer family type.
func (t UIRType) IsInteger() bool {
	switch t {
	case TypeInt32, TypeInt64, TypeUInt32, TypeUInt64,
		TypeSInt32, TypeSInt64, TypeFixed32, TypeFixed64, TypeSFixed32, TypeSFixed64:
		return true
	default:
		return false
	}
}

// IsFloat reports whether t is a floating-point type.
func (t UIRType) IsFloat() bool {
	return t == TypeFloat32 || t == TypeFloat64
}

// IsLogical reports whether t is a logical/temporal/decimal type.
func (t UIRType) IsLogical() bool {
	switch t {
	case TypeTimestamp, TypeDate, TypeTime, TypeDuration, TypeDecimal:
		return true
	default:
		return false
	}
}
