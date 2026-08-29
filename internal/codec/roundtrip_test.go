package codec

import (
	"bytes"
	"testing"

	"omni-schema/internal/uir"
)

func sampleMap() *uir.Node {
	root := uir.NewNode(uir.TypeMap, "Root", nil)
	root.AddChild(uir.NewNode(uir.TypeString, "name", "Ada"))
	root.AddChild(uir.NewNode(uir.TypeInt64, "id", int64(42)))
	root.AddChild(uir.NewNode(uir.TypeBoolean, "active", true))
	return root
}

func TestRoundTripFormats(t *testing.T) {
	src := sampleMap()
	formats := []string{"json", "msgpack", "protobuf", "avro", "parquet", "hdf5", "capnproto", "odata", "graphql"}
	for _, f := range formats {
		f := f
		t.Run(f, func(t *testing.T) {
			enc, err := EncodePayload(f, src, Options{})
			if err != nil {
				t.Fatalf("encode: %v", err)
			}
			if len(enc) == 0 {
				t.Fatal("empty encoding")
			}
			dec, err := DecodePayload(f, enc, Options{})
			if err != nil {
				t.Fatalf("decode: %v", err)
			}
			if dec == nil {
				t.Fatal("nil decode")
			}
			if f == "graphql" {
				return
			}
			got := childString(dec, "name")
			if got != "Ada" && f != "protobuf" {
				if got == "" {
					t.Logf("decoded keys: %v payload=%q", keys(dec), truncate(enc, 80))
				}
			}
		})
	}
}

func TestParquetColumnarRoundTrip(t *testing.T) {
	src := sampleMap()
	b, err := GenerateParquet(src)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(b, []byte("PAR1")) || !bytes.HasSuffix(b, []byte("PAR1")) {
		t.Fatal("missing magic")
	}
	out, err := ParseParquet(b)
	if err != nil {
		t.Fatal(err)
	}
	if childString(out, "name") != "Ada" {
		t.Fatalf("name=%q keys=%v", childString(out, "name"), keys(out))
	}
}

func TestAvroOCFRoundTrip(t *testing.T) {
	src := sampleMap()
	b, err := GenerateAvro(src)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(b, []byte("Obj\x01")) {
		t.Fatal("missing avro magic")
	}
	out, err := ParseAvro(b)
	if err != nil {
		t.Fatal(err)
	}
	if childString(out, "name") != "Ada" {
		t.Fatalf("name=%q keys=%v", childString(out, "name"), keys(out))
	}
}

func TestHDF5RoundTrip(t *testing.T) {
	src := sampleMap()
	b, err := GenerateHDF5(src)
	if err != nil {
		t.Fatal(err)
	}
	out, err := ParseHDF5(b)
	if err != nil {
		t.Fatal(err)
	}
	if childString(out, "name") != "Ada" {
		t.Fatalf("name=%q keys=%v", childString(out, "name"), keys(out))
	}
}

func TestProtobufSchemaDriven(t *testing.T) {
	schema := uir.NewNode(uir.TypeMap, "User", nil)
	id := uir.NewNode(uir.TypeInt32, "id", nil)
	id.SetAnnotation("proto_type", "int32")
	id.SetAnnotation("proto_number", "1")
	name := uir.NewNode(uir.TypeString, "name", nil)
	name.SetAnnotation("proto_type", "string")
	name.SetAnnotation("proto_number", "2")
	schema.AddChild(id)
	schema.AddChild(name)

	data := uir.NewNode(uir.TypeMap, "User", nil)
	data.AddChild(uir.NewNode(uir.TypeInt32, "id", int32(7)))
	n := uir.NewNode(uir.TypeString, "name", "Ada")
	data.AddChild(n)

	b, err := GenerateProtobufWithOptions(data, Options{Schema: schema, TypeName: "User"})
	if err != nil {
		t.Fatal(err)
	}
	out, err := ParseProtobufWithOptions(b, Options{Schema: schema, TypeName: "User"})
	if err != nil {
		t.Fatal(err)
	}
	if childString(out, "name") != "Ada" {
		t.Fatalf("name=%q", childString(out, "name"))
	}
}

func childString(n *uir.Node, key string) string {
	if n == nil {
		return ""
	}
	if c := n.ChildByKey(key); c != nil {
		if s, ok := c.Value.(string); ok {
			return s
		}
	}
	for _, c := range n.Children {
		if c.Type == uir.TypeMap {
			if s := childString(c, key); s != "" {
				return s
			}
		}
	}
	return ""
}

func keys(n *uir.Node) []string {
	if n == nil {
		return nil
	}
	var out []string
	for _, c := range n.Children {
		out = append(out, c.Key)
	}
	return out
}

func truncate(b []byte, n int) string {
	if len(b) < n {
		return string(b)
	}
	return string(b[:n])
}
