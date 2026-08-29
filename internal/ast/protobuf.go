package ast

type ProtoFile struct {
	Syntax   string
	Package  string
	Imports  []string
	Options  map[string]string
	Messages []*ProtoMessage
	Enums    []*ProtoEnum
	Services []*ProtoService
}

type ProtoMessage struct {
	Name     string
	Fields   []*ProtoField
	Oneofs   []*ProtoOneof
	Enums    []*ProtoEnum
	Nested   []*ProtoMessage
	Reserved []string
	Maps     []*ProtoMapField
}

type ProtoField struct {
	Repeated bool
	Optional bool
	Required bool
	Type     string
	Name     string
	Tag      int
	Options  map[string]string
}

type ProtoMapField struct {
	KeyType   string
	ValueType string
	Name      string
	Tag       int
}

type ProtoOneof struct {
	Name   string
	Fields []*ProtoField
}

type ProtoEnum struct {
	Name   string
	Values []*ProtoEnumValue
}

type ProtoEnumValue struct {
	Name   string
	Number int
}

type ProtoService struct {
	Name string
	RPCs []*ProtoRPC
}

type ProtoRPC struct {
	Name     string
	Request  string
	Response string
	Options  map[string]string
}
