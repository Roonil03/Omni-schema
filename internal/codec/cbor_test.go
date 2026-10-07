package codec

import (
	"bytes"
	"encoding/hex"
	"math"
	"testing"

	"omni-schema/internal/uir"
)

func TestCBORRFCVectors(t *testing.T) {
	// RFC 8949 Appendix A vectors, checked independently of our encoder.
	for _, vector := range []struct {
		hex   string
		value any
	}{
		{"00", int64(0)}, {"1818", int64(24)}, {"1903e8", int64(1000)},
		{"20", int64(-1)}, {"3903e7", int64(-1000)},
		{"f4", false}, {"f5", true}, {"f6", nil}, {"6449455446", "IETF"},
		{"f93e00", float64(1.5)}, {"fa47c35000", float32(100000)},
		{"fb3ff199999999999a", float64(1.1)},
		{"1bffffffffffffffff", uint64(math.MaxUint64)}, {"3b7fffffffffffffff", int64(math.MinInt64)},
	} {
		t.Run(vector.hex, func(t *testing.T) {
			raw, _ := hex.DecodeString(vector.hex)
			n, err := ParseCBOR(raw)
			if err != nil || n.Value != vector.value {
				t.Fatalf("decode: %v %v", n, err)
			}
			out, err := GenerateCBOR(n)
			if err != nil {
				t.Fatal(err)
			}
			back, err := ParseCBOR(out)
			if err != nil || back.Value != vector.value {
				t.Fatalf("roundtrip: %v %v", back, err)
			}
		})
	}
}

func TestCBORContainersAndBytes(t *testing.T) {
	raw, _ := hex.DecodeString("a261618301f6f5616243010203")
	n, err := ParseCBOR(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(n.ChildByKey("a").Children) != 3 || !bytes.Equal(n.ChildByKey("b").Value.([]byte), []byte{1, 2, 3}) {
		t.Fatal("containers or bytes lost")
	}
	out, err := GenerateCBOR(n)
	if err != nil || !bytes.Equal(raw, out) {
		t.Fatalf("encode: %x %v", out, err)
	}
	for _, kind := range []uir.UIRType{uir.TypeMap, uir.TypeArray} {
		empty := uir.NewNode(kind, "Root", nil)
		raw, err := GenerateCBOR(empty)
		if err != nil {
			t.Fatal(err)
		}
		back, err := ParseCBOR(raw)
		if err != nil || back.Type != kind || len(back.Children) != 0 {
			t.Fatalf("empty: %v %v", back, err)
		}
	}
}

func TestCBORRejectsInvalidInput(t *testing.T) {
	for _, vector := range []string{"", "18", "61ff", "a2616101616102", "a10102", "9f01ff", "c001", "f7", "f800", "1c", "0001", "9bffffffffffffffff", "3bffffffffffffffff"} {
		t.Run(vector, func(t *testing.T) {
			raw, _ := hex.DecodeString(vector)
			if _, err := ParseCBOR(raw); err == nil {
				t.Fatalf("accepted %x", raw)
			}
		})
	}
	raw := append(bytes.Repeat([]byte{0x81}, 66), 0)
	if _, err := ParseCBOR(raw); err == nil {
		t.Fatal("accepted excessive nesting")
	}
}

func FuzzCBOR(f *testing.F) {
	f.Add([]byte{0xa1, 0x61, 'a', 0x01})
	f.Fuzz(func(t *testing.T, raw []byte) {
		n, err := ParseCBOR(raw)
		if err != nil {
			return
		}
		out, err := GenerateCBOR(n)
		if err != nil {
			t.Fatalf("accepted input cannot encode: %v", err)
		}
		back, err := ParseCBOR(out)
		if err != nil {
			t.Fatalf("encoder produced invalid output: %v", err)
		}
		stable, err := GenerateCBOR(back)
		if err != nil || !bytes.Equal(out, stable) {
			t.Fatalf("unstable roundtrip: %x -> %x (%v)", out, stable, err)
		}
	})
}

func BenchmarkCBORRoundTrip(b *testing.B) {
	n := canonical()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		out, err := GenerateCBOR(n)
		if err != nil {
			b.Fatal(err)
		}
		if _, err := ParseCBOR(out); err != nil {
			b.Fatal(err)
		}
	}
}
