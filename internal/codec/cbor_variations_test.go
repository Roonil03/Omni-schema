package codec

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"math"
	"strings"
	"testing"

	"omni-schema/internal/uir"
)

func TestCBORAllFormatsValuePreservation(t *testing.T) {
	for _, fixture := range []string{
		`{"name":"Ada","id":42,"ok":true}`,
		`{"name":"नमस्ते 🌏","id":-123456,"ok":false}`,
		`{"name":"","id":9223372036854775807,"ok":true}`,
	} {
		n, err := DecodePayload("json", []byte(fixture), Options{})
		if err != nil {
			t.Fatal(err)
		}
		encoded, err := GenerateCBOR(n)
		if err != nil {
			t.Fatal(err)
		}
		n, err = ParseCBOR(encoded)
		if err != nil {
			t.Fatal(err)
		}
		for _, format := range AdvertisedFormats {
			t.Run(format+"/"+fixture, func(t *testing.T) {
				opts := Options{}
				if RequiresExternalSchema(format) {
					opts.Schema = n
				}
				out, err := EncodePayload(format, n, opts)
				if err != nil {
					t.Fatal(err)
				}
				back, err := DecodePayload(format, out, opts)
				if err != nil {
					t.Fatal(err)
				}
				if back.Type == uir.TypeArray && len(back.Children) == 1 {
					back = back.Children[0]
				}
				for _, key := range []string{"name", "id", "ok"} {
					want, got := n.ChildByKey(key), back.ChildByKey(key)
					if got == nil || fmt.Sprint(got.Value) != fmt.Sprint(want.Value) {
						t.Fatalf("%s lost %s: want %v got %v", format, key, want.Value, got)
					}
				}
				cb, err := GenerateCBOR(back)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := ParseCBOR(cb); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func TestCBORItemLimits(t *testing.T) {
	raw := append(cborHead(nil, 4, cborMaxItems), bytes.Repeat([]byte{0}, cborMaxItems)...)
	if _, err := ParseCBOR(raw); err == nil {
		t.Fatal("unbounded item count accepted")
	}
	n := uir.NewNode(uir.TypeArray, "Root", nil)
	for i := 0; i < cborMaxItems; i++ {
		n.AddChild(uir.NewNode(uir.TypeNull, "", nil))
	}
	if _, err := GenerateCBOR(n); err == nil {
		t.Fatal("unbounded output accepted")
	}
}

func TestCBORJSONValuePreservation(t *testing.T) {
	for _, raw := range []string{
		`[]`, `{}`, `null`, `true`, `false`, `""`, `"हैलो 🌏"`, `-0`,
		`{"empty":[],"nested":{"x":[null,false,{},[],"a"]}}`,
		`9007199254740993`, `9223372036854775807`, `-9223372036854775808`, `18446744073709551615`,
		`{"n":-1,"zero":0,"fraction":1.25,"tiny":1e-100,"large":1e100}`,
	} {
		t.Run(raw, func(t *testing.T) {
			n, err := DecodePayload("json", []byte(raw), Options{})
			if err != nil {
				t.Fatal(err)
			}
			before, err := GenerateJSON(n)
			if err != nil {
				t.Fatal(err)
			}
			b, err := GenerateCBOR(n)
			if err != nil {
				t.Fatal(err)
			}
			back, err := ParseCBOR(b)
			if err != nil {
				t.Fatal(err)
			}
			after, err := GenerateJSON(back)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(before, after) {
				t.Fatalf("changed JSON: %s -> %s", before, after)
			}
			if raw == `[]` && string(after) != "[]" {
				t.Fatalf("empty array became %s", after)
			}
			if !strings.Contains(raw, "{") && !strings.Contains(raw, "[") && !strings.Contains(raw, `"`) && len(raw) > 15 && string(after) != raw {
				t.Fatalf("integer changed: %s -> %s", raw, after)
			}
		})
	}
}

func TestCBORIntegerWidthBoundaries(t *testing.T) {
	for _, v := range []uint64{0, 23, 24, 255, 256, 65535, 65536, math.MaxUint32, math.MaxUint32 + 1, math.MaxInt64, math.MaxUint64} {
		n := uir.NewNode(uir.TypeUInt64, "Root", v)
		out, err := GenerateCBOR(n)
		if err != nil {
			t.Fatal(err)
		}
		back, err := ParseCBOR(out)
		if err != nil {
			t.Fatal(err)
		}
		got := uint64(0)
		switch value := back.Value.(type) {
		case uint64:
			got = value
		case int64:
			got = uint64(value)
		}
		if got != v {
			t.Fatalf("uint %d -> %v", v, back.Value)
		}
	}
	for _, v := range []int64{-1, -24, -25, -256, -257, -65536, -65537, math.MinInt64} {
		out, _ := GenerateCBOR(uir.NewNode(uir.TypeInt64, "Root", v))
		back, err := ParseCBOR(out)
		if err != nil || back.Value != v {
			t.Fatalf("int %d -> %v (%v)", v, back, err)
		}
	}
}

func TestCBORFloat32MessagePackRoundTrip(t *testing.T) {
	n := uir.NewNode(uir.TypeFloat32, "Root", float32(1.25))
	out, err := GenerateMessagePack(n)
	if err != nil {
		t.Fatal(err)
	}
	back, err := ParseMessagePack(out)
	if err != nil {
		t.Fatal(err)
	}
	switch v := back.Value.(type) {
	case float32:
		if v != 1.25 {
			t.Fatal(v)
		}
	case float64:
		if v != 1.25 {
			t.Fatal(v)
		}
	default:
		t.Fatalf("unexpected %T", v)
	}
}

func TestCBORProjectedUnsignedMessagePack(t *testing.T) {
	for _, kind := range []uir.UIRType{uir.TypeUInt32, uir.TypeFixed32} {
		n := uir.NewNode(kind, "Root", uint32(4000000000))
		out, err := GenerateMessagePack(n)
		if err != nil {
			t.Fatal(err)
		}
		back, err := ParseMessagePack(out)
		if err != nil || back.Value != uint64(4000000000) {
			t.Fatalf("projected uint32 lost: %v %v", back, err)
		}
	}
}

func TestCBORNativeValuesAcrossBinaryFormats(t *testing.T) {
	for _, format := range []string{"cbor", "msgpack", "protobuf", "capnproto", "avro", "hdf5"} {
		t.Run(format, func(t *testing.T) {
			n := uir.NewNode(uir.TypeMap, "Root", nil)
			n.AddChild(uir.NewNode(uir.TypeFloat32, "ratio", float32(1.25)))
			n.AddChild(uir.NewNode(uir.TypeFloat64, "double", float64(1.1)))
			n.AddChild(uir.NewNode(uir.TypeBytes, "blob", []byte{0, 1, 0xff}))
			cb, err := GenerateCBOR(n)
			if err != nil {
				t.Fatal(err)
			}
			decoded, err := ParseCBOR(cb)
			if err != nil {
				t.Fatal(err)
			}
			opts := Options{}
			if RequiresExternalSchema(format) {
				opts.Schema = decoded
			}
			out, err := EncodePayload(format, decoded, opts)
			if err != nil {
				t.Fatal(err)
			}
			back, err := DecodePayload(format, out, opts)
			if err != nil {
				t.Fatal(err)
			}
			if back.Type == uir.TypeArray && len(back.Children) == 1 {
				back = back.Children[0]
			}
			blob, ratio := back.ChildByKey("blob"), back.ChildByKey("ratio")
			if blob == nil || ratio == nil {
				t.Fatalf("lost binary fields: %v", back)
			}
			if got, ok := blob.Value.([]byte); !ok || !bytes.Equal(got, []byte{0, 1, 0xff}) {
				t.Fatalf("blob changed: %v", blob)
			}
			if fmt.Sprint(ratio.Value) != "1.25" {
				t.Fatalf("ratio changed: %v", ratio)
			}
			if value := back.ChildByKey("double"); value == nil || fmt.Sprint(value.Value) != "1.1" {
				t.Fatalf("double changed: %v", value)
			}
		})
	}
}

func TestCBORFloatSpecialValues(t *testing.T) {
	for _, vector := range []string{"f90000", "f98000", "f90001", "f97bff", "f97c00", "f9fc00", "f97e00", "fa80000000", "fb8000000000000000"} {
		t.Run(vector, func(t *testing.T) {
			raw, _ := hex.DecodeString(vector)
			n, err := ParseCBOR(raw)
			if err != nil {
				t.Fatal(err)
			}
			out, err := GenerateCBOR(n)
			if err != nil {
				t.Fatal(err)
			}
			back, err := ParseCBOR(out)
			if err != nil {
				t.Fatal(err)
			}
			asFloat := func(n *uir.Node) float64 {
				switch v := n.Value.(type) {
				case float32:
					return float64(v)
				case float64:
					return v
				}
				t.Fatalf("unexpected float %T", n.Value)
				return 0
			}
			a, b := asFloat(n), asFloat(back)
			if !(math.IsNaN(a) && math.IsNaN(b)) && (a != b || math.Signbit(a) != math.Signbit(b)) {
				t.Fatalf("float changed %v -> %v", a, b)
			}
		})
	}
}

func TestCBORLargeLengthsAndAllTruncations(t *testing.T) {
	for _, length := range []int{0, 23, 24, 255, 256, 65535, 65536} {
		for _, kind := range []uir.UIRType{uir.TypeString, uir.TypeBytes} {
			var value any = strings.Repeat("x", length)
			if kind == uir.TypeBytes {
				value = bytes.Repeat([]byte{0xff}, length)
			}
			raw, err := GenerateCBOR(uir.NewNode(kind, "Root", value))
			if err != nil {
				t.Fatal(err)
			}
			back, err := ParseCBOR(raw)
			if err != nil {
				t.Fatal(err)
			}
			if kind == uir.TypeString && back.Value != value {
				t.Fatal("string changed")
			}
			if kind == uir.TypeBytes && !bytes.Equal(back.Value.([]byte), value.([]byte)) {
				t.Fatal("bytes changed")
			}
		}
	}
	raw, _ := GenerateCBOR(canonical())
	for i := 0; i < len(raw); i++ {
		if _, err := ParseCBOR(raw[:i]); err == nil {
			t.Fatalf("accepted truncation at %d", i)
		}
	}
}
