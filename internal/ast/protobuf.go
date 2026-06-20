package ast

// ProtoFile represents a parsed .proto file.
type ProtoFile struct {
	Syntax   string
	Package  string
	Messages []*ProtoMessage
	Services []*ProtoService
}

// ProtoMessage represents a message definition in protobuf.
type ProtoMessage struct {
	Name   string
	Fields []*ProtoField
	Oneofs []*ProtoOneof
}

// ProtoField represents a single field in a message.
type ProtoField struct {
	Repeated bool
	Type     string
	Name     string
	Tag      int
}

// ProtoOneof represents a oneof union in protobuf.
type ProtoOneof struct {
	Name   string
	Fields []*ProtoField
}

// ProtoService represents an RPC service.
type ProtoService struct {
	Name string
	RPCs []*ProtoRPC
}

// ProtoRPC represents an rpc method within a service.
type ProtoRPC struct {
	Name     string
	Request  string
	Response string
}
