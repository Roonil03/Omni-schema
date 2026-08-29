package codec

import (
	"fmt"

	"omni-schema/internal/uir"
)

// Options supplies schema and policy context to codecs that cannot decode
// faithfully from the byte stream alone.
type Options struct {
	Schema     *uir.Node
	TypeName   string
	Bytes      uir.BytesPolicy
	Lossiness  uir.LossinessPolicy
}

func (o Options) TypeSchema() *uir.Node {
	if o.Schema == nil {
		return nil
	}
	if o.TypeName == "" {
		return o.Schema
	}
	if found := o.Schema.FindNamedType(o.TypeName); found != nil {
		return found
	}
	return o.Schema
}

// SchemaAwareDecoder is implemented by codecs whose decode path depends on an
// external schema (Protobuf, Cap'n Proto, Avro, Parquet, HDF5, GraphQL).
type SchemaAwareDecoder interface {
	DecodeWithOptions(data []byte, opts Options) (*uir.Node, error)
}

// SchemaAwareEncoder is implemented by codecs that need schema/type metadata
// to emit a correct wire layout.
type SchemaAwareEncoder interface {
	EncodeWithOptions(node *uir.Node, opts Options) ([]byte, error)
}

type schemaDecoderFunc func([]byte, Options) (*uir.Node, error)

func (f schemaDecoderFunc) Decode(data []byte) (*uir.Node, error) {
	return f(data, Options{})
}

func (f schemaDecoderFunc) DecodeWithOptions(data []byte, opts Options) (*uir.Node, error) {
	return f(data, opts)
}

type schemaEncoderFunc func(*uir.Node, Options) ([]byte, error)

func (f schemaEncoderFunc) Encode(node *uir.Node) ([]byte, error) {
	return f(node, Options{})
}

func (f schemaEncoderFunc) EncodeWithOptions(node *uir.Node, opts Options) ([]byte, error) {
	return f(node, opts)
}

// DecodePayload looks up a decoder and, when possible, supplies schema context.
func DecodePayload(format string, data []byte, opts Options) (*uir.Node, error) {
	dec, err := GetDecoder(format)
	if err != nil {
		return nil, err
	}
	if sd, ok := dec.(SchemaAwareDecoder); ok {
		return sd.DecodeWithOptions(data, opts)
	}
	return dec.Decode(data)
}

// EncodePayload looks up an encoder and, when possible, supplies schema context.
func EncodePayload(format string, node *uir.Node, opts Options) ([]byte, error) {
	enc, err := GetEncoder(format)
	if err != nil {
		return nil, err
	}
	if se, ok := enc.(SchemaAwareEncoder); ok {
		return se.EncodeWithOptions(node, opts)
	}
	return enc.Encode(node)
}

func requireType(opts Options, fallback *uir.Node) *uir.Node {
	if t := opts.TypeSchema(); t != nil && t != opts.Schema {
		return t
	}
	if opts.Schema != nil && len(opts.Schema.Children) == 1 && opts.TypeName == "" {
		return opts.Schema.Children[0]
	}
	if t := opts.TypeSchema(); t != nil {
		return t
	}
	return fallback
}

func unsupported(format, detail string) error {
	return fmt.Errorf("%s: %s", format, detail)
}
