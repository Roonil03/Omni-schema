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
	RegisterDecoder("json", DecoderFunc(lexer.ParseJSON))
	RegisterDecoder("protobuf", schemaDecoderFunc(ParseProtobufWithOptions))
	RegisterDecoder("msgpack", DecoderFunc(ParseMessagePack))
	RegisterDecoder("capnproto", schemaDecoderFunc(ParseCapnProtoWithOptions))
	RegisterDecoder("parquet", schemaDecoderFunc(ParseParquetWithOptions))
	RegisterDecoder("hdf5", schemaDecoderFunc(ParseHDF5WithOptions))
	RegisterDecoder("graphql", DecoderFunc(ParseGraphQLResult))
	RegisterDecoder("avro", schemaDecoderFunc(ParseAvroWithOptions))
	RegisterDecoder("odata", schemaDecoderFunc(ParseODataWithOptions))

	RegisterEncoder("graphql", EncoderFunc(GenerateGraphQLResult))
	RegisterEncoder("graphql_sdl", EncoderFunc(GenerateGraphQLSDL))
	RegisterEncoder("protobuf", schemaEncoderFunc(GenerateProtobufWithOptions))
	RegisterEncoder("msgpack", EncoderFunc(GenerateMessagePack))
	RegisterEncoder("parquet", schemaEncoderFunc(GenerateParquetWithOptions))
	RegisterEncoder("capnproto", schemaEncoderFunc(GenerateCapnProtoWithOptions))
	RegisterEncoder("hdf5", schemaEncoderFunc(GenerateHDF5WithOptions))
	RegisterEncoder("json", EncoderFunc(GenerateJSON))
	RegisterEncoder("avro", schemaEncoderFunc(GenerateAvroWithOptions))
	RegisterEncoder("odata", schemaEncoderFunc(GenerateODataWithOptions))
}
