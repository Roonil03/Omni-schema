package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"

	"omni-schema/internal/codec"
	"omni-schema/internal/registry"
)

func cborHTTP(t *testing.T, url string, payload []byte) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(http.MethodPost, url, bytes.NewReader(payload))
	r.Header.Set("Content-Type", "application/octet-stream")
	w := httptest.NewRecorder()
	morphHandler(w, r)
	return w
}

func TestCBORHTTPJSONVariations(t *testing.T) {
	for _, payload := range []string{`[]`, `{}`, `null`, `true`, `false`, `"text"`, `[1,null,false,"x"]`, `{"nested":{"list":[{},[],null,1.25]},"unicode":"नमस्ते 🌏"}`, `{"int":9007199254740993,"uint":18446744073709551615,"min":-9223372036854775808}`} {
		t.Run(payload, func(t *testing.T) {
			encoded := cborHTTP(t, "/morph/json/cbor", []byte(payload))
			if encoded.Code != 200 {
				t.Fatalf("encode: %d %s", encoded.Code, encoded.Body.String())
			}
			decoded := cborHTTP(t, "/morph/cbor/json", encoded.Body.Bytes())
			if decoded.Code != 200 {
				t.Fatalf("decode: %d %s", decoded.Code, decoded.Body.String())
			}
			parse := func(raw []byte) any {
				d := json.NewDecoder(bytes.NewReader(raw))
				d.UseNumber()
				var v any
				if err := d.Decode(&v); err != nil {
					t.Fatal(err)
				}
				return v
			}
			if !reflect.DeepEqual(parse([]byte(payload)), parse(decoded.Body.Bytes())) {
				t.Fatalf("changed: %s -> %s", payload, decoded.Body.String())
			}
		})
	}
}

func TestCBORHTTPMultipartRoutes(t *testing.T) {
	for _, url := range []string{"/morph?target=cbor", "/morph/cbor"} {
		body := new(bytes.Buffer)
		writer := multipart.NewWriter(body)
		part, _ := writer.CreateFormFile("file", "sample.json")
		part.Write([]byte(`{"name":"Ada"}`))
		writer.Close()
		req := httptest.NewRequest("POST", url, body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		w := httptest.NewRecorder()
		morphHandler(w, req)
		if w.Code != 200 || w.Header().Get("Content-Type") != "application/cbor" || !strings.Contains(w.Header().Get("Content-Disposition"), "sample.cbor") {
			t.Fatalf("multipart: %d %v %s", w.Code, w.Header(), w.Body.String())
		}
		back := cborHTTP(t, "/morph/cbor/json", w.Body.Bytes())
		if back.Code != 200 || !strings.Contains(back.Body.String(), "Ada") {
			t.Fatal(back.Body.String())
		}
	}
}

func TestCBORHTTPInvalidPayloads(t *testing.T) {
	for _, vector := range []string{"", "18", "61ff", "a2616101616102", "a10102", "9f01ff", "c001", "f7", "0001", "9bffffffffffffffff"} {
		raw, _ := hex.DecodeString(vector)
		w := cborHTTP(t, "/morph/cbor/json", raw)
		if w.Code != 400 {
			t.Fatalf("%s: expected 400 got %d", vector, w.Code)
		}
	}
	for _, vector := range []string{"f97c00", "f9fc00", "f97e00"} {
		raw, _ := hex.DecodeString(vector)
		w := cborHTTP(t, "/morph/cbor/json", raw)
		if w.Code != 400 {
			t.Fatalf("non-finite %s: expected 400 got %d", vector, w.Code)
		}
	}
}

func TestCBORHTTPProtobufSchema(t *testing.T) {
	previous := registry.Default
	registry.Default = registry.NewRegistry()
	t.Cleanup(func() { registry.Default = previous })
	raw := []byte(`syntax = "proto3"; message User { int64 id = 1; string name = 2; bool ok = 3; }`)
	schema, err := parseSchema("protobuf", raw)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := registry.Default.Register("cbor_user", "protobuf", raw, schema); err != nil {
		t.Fatal(err)
	}
	input := []byte(`{"id":42,"name":"Ada","ok":true}`)
	cb := cborHTTP(t, "/morph/json/cbor", input)
	proto := cborHTTP(t, "/morph/cbor/protobuf?targetSchema=cbor_user&targetType=User", cb.Body.Bytes())
	if proto.Code != 200 {
		t.Fatalf("schema encode: %d %s", proto.Code, proto.Body.String())
	}
	output := cborHTTP(t, "/morph/protobuf/cbor?sourceSchema=cbor_user&sourceType=User", proto.Body.Bytes())
	if output.Code != 200 {
		t.Fatalf("schema decode: %d %s", output.Code, output.Body.String())
	}
	n, err := codec.ParseCBOR(output.Body.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	final, err := codec.GenerateJSON(n)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(final), `"Ada"`) || !strings.Contains(string(final), `42`) {
		t.Fatalf("schema lost values: %s", final)
	}
	// Reuse a cached projection plan concurrently; report state must be per request.
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w := cborHTTP(t, "/morph/cbor/protobuf?targetSchema=cbor_user&targetType=User", cb.Body.Bytes())
			if w.Code != 200 {
				t.Errorf("concurrent schema conversion: %d %s", w.Code, w.Body.String())
			}
		}()
	}
	wg.Wait()
	partial := cborHTTP(t, "/morph/json/cbor", []byte(`{"id":7}`))
	optional := cborHTTP(t, "/morph/cbor/protobuf?targetSchema=cbor_user&targetType=User", partial.Body.Bytes())
	if optional.Code != 200 {
		t.Fatalf("missing optional fields should be omitted: %d %s", optional.Code, optional.Body.String())
	}
}

func TestCBORHTTPLimits(t *testing.T) {
	raw := append(bytes.Repeat([]byte{0x81}, 66), 0)
	if w := cborHTTP(t, "/morph/cbor/json", raw); w.Code != 400 {
		t.Fatalf("depth limit: %d", w.Code)
	}
	tooDeepJSON := strings.Repeat("[", 66) + "0" + strings.Repeat("]", 66)
	if w := cborHTTP(t, "/morph/json/cbor", []byte(tooDeepJSON)); w.Code != 400 {
		t.Fatalf("output depth limit: %d", w.Code)
	}
	if w := cborHTTP(t, "/morph/cbor/json", bytes.Repeat([]byte{0}, (10<<20)+1)); w.Code != 400 {
		t.Fatalf("body limit: %d", w.Code)
	}
}

func TestCBORHTTPUnsupportedTargetShapes(t *testing.T) {
	for _, format := range []string{"parquet", "hdf5", "avro"} {
		for _, input := range []string{`{"nested":{"id":1}}`, `[1,2,3]`, `{"n":18446744073709551615}`, `[{"id":1},{"id":1.25}]`, `[{"id":1},{"name":"Ada"}]`} {
			cb := cborHTTP(t, "/morph/json/cbor", []byte(input))
			if cb.Code != 200 {
				t.Fatal(cb.Body.String())
			}
			w := cborHTTP(t, "/morph/cbor/"+format, cb.Body.Bytes())
			if w.Code != 400 {
				t.Fatalf("%s %s silently accepted: %d %x", format, input, w.Code, w.Body.Bytes())
			}
		}
	}
	for _, input := range []string{`1e400`, `1e-400`, `18446744073709551616`, `-9223372036854775809`, `{} {}`} {
		w := cborHTTP(t, "/morph/json/cbor", []byte(input))
		if w.Code != 400 {
			t.Fatalf("invalid JSON %s accepted", input)
		}
	}
}

func TestCBORHTTPGraphQLNames(t *testing.T) {
	for _, input := range []string{`{"invalid key":1}`, `{"__reserved":1}`, `{"nested":{}}`} {
		cb := cborHTTP(t, "/morph/json/cbor", []byte(input))
		w := cborHTTP(t, "/morph/cbor/graphql", cb.Body.Bytes())
		if w.Code != 400 {
			t.Fatalf("invalid SDL shape %s accepted: %d", input, w.Code)
		}
	}
}

func TestCBORHTTPMalformedMultipart(t *testing.T) {
	r := httptest.NewRequest("POST", "/morph/json/cbor", strings.NewReader("broken body"))
	r.Header.Set("Content-Type", "multipart/form-data; boundary=broken")
	w := httptest.NewRecorder()
	morphHandler(w, r)
	if w.Code != 400 {
		t.Fatalf("malformed multipart: %d", w.Code)
	}
}

func TestCBORHTTPMultipartSource(t *testing.T) {
	raw, _ := hex.DecodeString("a1646e616d6563416461")
	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "user.CBOR")
	part.Write(raw)
	writer.WriteField("target", "json")
	writer.Close()
	req := httptest.NewRequest("POST", "/morph", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	morphHandler(w, req)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "Ada") || !strings.Contains(w.Header().Get("Content-Disposition"), "user.json") {
		t.Fatalf("CBOR multipart source: %d %s", w.Code, w.Body.String())
	}
}
