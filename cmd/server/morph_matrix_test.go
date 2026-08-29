package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"omni-schema/internal/codec"
	"omni-schema/internal/registry"
	"omni-schema/internal/uir"
)

func TestHTTPMorphMatrix(t *testing.T) {
	t.Setenv("OMNI_ENV", "")
	t.Setenv("OMNI_API_TOKEN", "")
	registry.Default = registry.NewRegistry()
	srv := httptest.NewServer(newMux())
	defer srv.Close()

	canonical := []byte(`{"name":"Ada","id":42,"ok":true}`)
	encoded := map[string][]byte{"json": canonical}
	for _, from := range codec.AdvertisedFormats {
		if from == "json" {
			continue
		}
		encoded[from] = postMorph(t, srv.URL, "json", from, canonical)
		if len(encoded[from]) == 0 {
			t.Fatalf("json->%s empty body", from)
		}
	}

	for _, from := range codec.AdvertisedFormats {
		for _, to := range codec.AdvertisedFormats {
			body := postMorph(t, srv.URL, from, to, encoded[from])
			if to == "graphql" {
				if !bytes.Contains(body, []byte("type ")) {
					t.Fatalf("%s->%s: expected GraphQL SDL, got %q", from, to, truncateBytes(body, 120))
				}
				continue
			}
			n, err := codec.DecodePayload(to, body, codec.Options{})
			if err != nil {
				t.Fatalf("%s->%s decode: %v body=%q", from, to, err, truncateBytes(body, 80))
			}
			if from == "graphql" || codec.RequiresExternalSchema(from) || codec.RequiresExternalSchema(to) {
				continue
			}
			if childStringHTTP(n, "name") != "Ada" {
				t.Fatalf("%s->%s: name=%q keys=%v", from, to, childStringHTTP(n, "name"), keysHTTP(n))
			}
		}
	}
}

func postMorph(t *testing.T, base, from, to string, payload []byte) []byte {
	t.Helper()
	url := fmt.Sprintf("%s/morph/%s/%s", base, from, to)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s->%s request: %v", from, to, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("%s->%s status %d: %s", from, to, resp.StatusCode, body)
	}
	return body
}

func childStringHTTP(n *uir.Node, key string) string {
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
			if s := childStringHTTP(c, key); s != "" {
				return s
			}
		}
	}
	return ""
}

func keysHTTP(n *uir.Node) []string {
	if n == nil {
		return nil
	}
	var out []string
	for _, c := range n.Children {
		out = append(out, c.Key)
	}
	return out
}

func truncateBytes(b []byte, n int) string {
	if len(b) < n {
		return string(b)
	}
	return string(b[:n])
}
