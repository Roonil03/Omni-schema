package codec

import (
	"fmt"
	"omni-schema/internal/lexer"
	"omni-schema/internal/uir"
)

type Encoder interface {
	Encode(node *uir.Node) ([]byte, error)
}

type Decoder interface {
	Decode(data []byte) (*uir.Node, error)
}

// EncoderFunc allows using simple functions as Encoders.
type EncoderFunc func(*uir.Node) ([]byte, error)

func (f EncoderFunc) Encode(node *uir.Node) ([]byte, error) {
	return f(node)
}

// DecoderFunc allows using simple functions as Decoders.
type DecoderFunc func([]byte) (*uir.Node, error)

func (f DecoderFunc) Decode(data []byte) (*uir.Node, error) {
	return f(data)
}

var (
	Encoders = make(map[string]Encoder)
	Decoders = make(map[string]Decoder)
)

func RegisterEncoder(name string, enc Encoder) {
	Encoders[name] = enc
}

func RegisterDecoder(name string, dec Decoder) {
	Decoders[name] = dec
}

func GetEncoder(name string) (Encoder, error) {
	if enc, ok := Encoders[name]; ok {
		return enc, nil
	}
	return nil, fmt.Errorf("unsupported target format: %s", name)
}

func GetDecoder(name string) (Decoder, error) {
	if dec, ok := Decoders[name]; ok {
		return dec, nil
	}
	return nil, fmt.Errorf("unsupported source format: %s", name)
}

func init() {
	// Register the built-in JSON lexer as the JSON decoder.
	RegisterDecoder("json", DecoderFunc(lexer.ParseJSON))
	RegisterDecoder("protobuf", DecoderFunc(ParseProtobuf))
	RegisterDecoder("msgpack", DecoderFunc(ParseMessagePack))
	RegisterDecoder("capnproto", DecoderFunc(ParseCapnProto))
	RegisterDecoder("parquet", DecoderFunc(ParseParquet))
	RegisterDecoder("hdf5", DecoderFunc(ParseHDF5))

	// Register all existing built-in encoders
	RegisterEncoder("graphql", EncoderFunc(GenerateGraphQL))
	RegisterEncoder("protobuf", EncoderFunc(GenerateProtobuf))
	RegisterEncoder("msgpack", EncoderFunc(GenerateMessagePack))
	RegisterEncoder("parquet", EncoderFunc(GenerateParquet))
	RegisterEncoder("capnproto", EncoderFunc(GenerateCapnProto))
	RegisterEncoder("hdf5", EncoderFunc(GenerateHDF5))
	RegisterEncoder("json", EncoderFunc(GenerateJSON))
}
