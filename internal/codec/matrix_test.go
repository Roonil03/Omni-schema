package codec

import (
	"bytes"
	"testing"

	"omni-schema/internal/uir"
)

func canonical() *uir.Node {
	root := uir.NewNode(uir.TypeMap, "Root", nil)
	root.AddChild(uir.NewNode(uir.TypeString, "name", "Ada"))
	root.AddChild(uir.NewNode(uir.TypeInt64, "id", int64(42)))
	root.AddChild(uir.NewNode(uir.TypeBoolean, "ok", true))
	return root
}

func TestFullConversionMatrix(t *testing.T) {
	src := canonical()
	for _, from := range AdvertisedFormats {
		encoded, err := encodeForMatrix(from, src)
		if err != nil {
			t.Fatalf("encode %s: %v", from, err)
		}
		decoded, err := DecodePayload(from, encoded, Options{})
		if err != nil {
			t.Fatalf("decode %s: %v", from, err)
		}
		for _, to := range AdvertisedFormats {
			out, err := encodeForMatrix(to, decoded)
			if err != nil {
				t.Fatalf("%s->%s encode: %v", from, to, err)
			}
			if _, err := DecodePayload(to, out, Options{}); err != nil {
				t.Fatalf("%s->%s decode: %v", from, to, err)
			}
		}
	}
}

func encodeForMatrix(format string, n *uir.Node) ([]byte, error) {
	if format == "graphql" {
		return GenerateGraphQLResult(n)
	}
	return EncodePayload(format, n, Options{})
}

func TestGraphQLSDLRoundTripParse(t *testing.T) {
	sdl, err := GenerateGraphQLSDL(canonical())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(sdl, []byte("type Root")) {
		t.Fatalf("expected type Root in SDL:\n%s", sdl)
	}
	n, err := ParseGraphQL(sdl)
	if err != nil {
		t.Fatal(err)
	}
	if n == nil || len(n.Children) == 0 {
		t.Fatal("expected lowered SDL types")
	}
	if _, err := ParseGraphQL([]byte(`{"data":{"name":"Ada","id":42,"ok":true}}`)); err != nil {
		t.Fatal(err)
	}
}

func TestODataJSONSubset(t *testing.T) {
	raw := []byte(`{"@odata.context":"$metadata#User","@odata.type":"#User","value":{"id":1,"name":"A"}}`)
	n, err := ParseOData(raw)
	if err != nil {
		t.Fatal(err)
	}
	if n.ChildByKey("name") == nil && childString(n, "name") == "" {
		t.Fatalf("expected name from value object, keys=%v", keys(n))
	}
	out, err := GenerateOData(canonical())
	if err != nil {
		t.Fatal(err)
	}
	back, err := ParseOData(out)
	if err != nil {
		t.Fatal(err)
	}
	_ = back
}

func TestIndependentParquetHDF5Avro(t *testing.T) {
	src := canonical()
	p, _ := GenerateParquet(src)
	if string(p[:4]) != "PAR1" {
		t.Fatal("parquet magic")
	}
	if _, err := ParseParquet(p); err != nil {
		t.Fatal(err)
	}
	h, _ := GenerateHDF5(src)
	if h[0] != 0x89 {
		t.Fatal("hdf5 sig")
	}
	if _, err := ParseHDF5(h); err != nil {
		t.Fatal(err)
	}
	a, _ := GenerateAvro(src)
	if string(a[:4]) != "Obj\x01" {
		t.Fatal("avro magic")
	}
	if _, err := ParseAvro(a); err != nil {
		t.Fatal(err)
	}
}

func TestMissingSchemaTypeFails(t *testing.T) {
	schema := uir.NewNode(uir.TypeMap, "proto_root", nil)
	schema.AddChild(uir.NewNode(uir.TypeMap, "User", nil))
	_, err := DecodePayload("protobuf", []byte{0x08, 0x01}, Options{Schema: schema, TypeName: "Nope"})
	if err == nil {
		t.Fatal("expected missing type error")
	}
}

func FuzzJSONAndMsgpack(f *testing.F) {
	f.Add([]byte(`{"a":1}`))
	f.Add([]byte{0x80})
	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = DecodePayload("json", data, Options{})
		_, _ = DecodePayload("msgpack", data, Options{})
		_, _ = DecodePayload("protobuf", data, Options{})
		_, _ = ParseParquet(data)
		_, _ = ParseHDF5(data)
		_, _ = ParseAvro(data)
		_, _ = ParseCapnProto(data)
	})
}

func BenchmarkJSONRoundTrip(b *testing.B) {
	src := canonical()
	for i := 0; i < b.N; i++ {
		enc, _ := GenerateJSON(src)
		_, _ = DecodePayload("json", enc, Options{})
	}
}

func BenchmarkProject(b *testing.B) {
	src := canonical()
	schema := canonical()
	opts := uir.DefaultProjectOptions()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = uir.Project(src, schema, opts)
	}
}
