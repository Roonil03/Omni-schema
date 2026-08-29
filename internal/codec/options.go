package codec

import (
	"fmt"

	"omni-schema/internal/uir"
)

type Options struct {
	Schema      *uir.Node
	TypeName    string
	Bytes       uir.BytesPolicy
	Lossiness   uir.LossinessPolicy
	RequireType bool
}

func (o Options) TypeSchema() *uir.Node {
	if o.Schema == nil {
		return nil
	}
	if o.TypeName == "" {
		return o.Schema
	}
	return o.Schema.FindNamedType(o.TypeName)
}

type SchemaAwareDecoder interface {
	DecodeWithOptions(data []byte, opts Options) (*uir.Node, error)
}

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

func DecodePayload(format string, data []byte, opts Options) (*uir.Node, error) {
	format = NormalizeFormat(format)
	if RequiresExternalSchema(format) && opts.RequireType {
		if opts.Schema == nil {
			return nil, fmt.Errorf("%s decoding requires a registered schema", format)
		}
		if _, err := uir.ResolvePayloadType(opts.Schema, opts.TypeName); err != nil {
			return nil, err
		}
	}
	if opts.TypeName != "" && opts.Schema != nil && opts.Schema.FindNamedType(opts.TypeName) == nil {
		return nil, fmt.Errorf("schema type %q not found", opts.TypeName)
	}
	dec, err := GetDecoder(format)
	if err != nil {
		return nil, err
	}
	if sd, ok := dec.(SchemaAwareDecoder); ok {
		return sd.DecodeWithOptions(data, opts)
	}
	return dec.Decode(data)
}

func EncodePayload(format string, node *uir.Node, opts Options) ([]byte, error) {
	format = NormalizeFormat(format)
	if RequiresExternalSchema(format) && opts.RequireType {
		if opts.Schema == nil {
			return nil, fmt.Errorf("%s encoding requires a registered schema", format)
		}
		if _, err := uir.ResolvePayloadType(opts.Schema, opts.TypeName); err != nil {
			return nil, err
		}
	}
	if opts.TypeName != "" && opts.Schema != nil && opts.Schema.FindNamedType(opts.TypeName) == nil {
		return nil, fmt.Errorf("schema type %q not found", opts.TypeName)
	}
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
	if opts.TypeName != "" {
		if opts.Schema == nil {
			return fallback
		}
		return opts.Schema.FindNamedType(opts.TypeName)
	}
	if opts.Schema != nil {
		t, err := uir.ResolvePayloadType(opts.Schema, "")
		if err == nil && t != nil {
			return t
		}
	}
	return fallback
}

func unsupported(format, detail string) error {
	return fmt.Errorf("%s: %s", format, detail)
}
