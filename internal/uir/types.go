package uir

// UIRType defines the fundamental types in the Unified Intermediate Representation.
// This enforces the Universal Type Mapping Matrix.
type UIRType int

const (
	TypeUnknown UIRType = iota
	TypeNull            // UIR_Null: OpenAPI null, GraphQL Null, JSON null, MessagePack nil
	TypeString          // UIR_String: OpenAPI string, GraphQL String, Protobuf string, OData Edm.String, Avro string
	TypeBytes           // UIR_Bytes: OpenAPI string(byte), Protobuf bytes, MessagePack bin, Avro bytes
	TypeInt32           // UIR_Int32: OpenAPI integer, GraphQL Int, Protobuf int32, OData Edm.Int32, Avro int
	TypeInt64           // UIR_Int64: OpenAPI integer, GraphQL Int, Protobuf int64, OData Edm.Int64, Avro long
	TypeUInt32          // UIR_UInt32: Protobuf uint32, MessagePack uint32
	TypeUInt64          // UIR_UInt64: Protobuf uint64, MessagePack uint64
	TypeFloat64         // UIR_Float64: OpenAPI number, GraphQL Float, Protobuf double, OData Edm.Double, Avro double
	TypeBoolean         // UIR_Boolean: OpenAPI boolean, GraphQL Boolean, Protobuf bool, OData Edm.Boolean, Avro boolean
	TypeArray           // UIR_Array[T]: OpenAPI array, GraphQL [T], Protobuf repeated_T, OData Collection(T), Avro array
	TypeMap             // UIR_Map[K,V]: OpenAPI object, GraphQL List_of_Pairs, Protobuf map<K,V>, OData Open_Type, Avro map
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
	case TypeFloat64:
		return "UIR_Float64"
	case TypeBoolean:
		return "UIR_Boolean"
	case TypeArray:
		return "UIR_Array"
	case TypeMap:
		return "UIR_Map"
	default:
		return "UIR_Unknown"
	}
}
