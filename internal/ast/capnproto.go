package ast

// CapnProtoFile represents a parsed .capnp file.
type CapnProtoFile struct {
	Structs []*CapnProtoStruct
}

// CapnProtoStruct represents a struct definition in Cap'n Proto.
type CapnProtoStruct struct {
	Name   string
	Fields []*CapnProtoField
}

// CapnProtoField represents a field inside a struct.
type CapnProtoField struct {
	Name string
	Type string
	Id   int
}
