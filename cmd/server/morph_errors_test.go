package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"omni-schema/internal/registry"
)

func TestMorphRejectsUnknownFormatsAsBadRequests(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{name: "source", url: "/morph/unknown/json", want: "unsupported source format"},
		{name: "target", url: "/morph/json/unknown", want: "unsupported target format"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, tt.url, strings.NewReader(`{"id":1}`))
			rr := httptest.NewRecorder()
			morphHandler(rr, req)
			if rr.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
			}
			if !strings.Contains(rr.Body.String(), tt.want) {
				t.Fatalf("expected %q in error, got %q", tt.want, rr.Body.String())
			}
		})
	}
}

func TestSchemaAwareProtobufRejectsEmptyProjection(t *testing.T) {
	registry.Default = registry.NewRegistry()
	t.Cleanup(func() { registry.Default = registry.NewRegistry() })

	protoSchema := []byte(`syntax = "proto3";
message User {
  int32 id = 1;
  string name = 2;
}`)
	root, err := parseSchema("protobuf", protoSchema)
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}
	if _, err := registry.Default.Register("user_schema", "protobuf", protoSchema, root); err != nil {
		t.Fatalf("register schema: %v", err)
	}

	req := httptest.NewRequest(
		http.MethodPost,
		"/morph/json/protobuf?schema=user_schema&type=User",
		bytes.NewBufferString(`{"out_of_schema":"value"}`),
	)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	morphHandler(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d with %d-byte body: %q", rr.Code, rr.Body.Len(), rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "empty payload") {
		t.Fatalf("expected empty-payload error, got %q", rr.Body.String())
	}
}

func TestProductionDevEventsRequireAuthAndExplicitEnable(t *testing.T) {
	t.Setenv("OMNI_ENV", "production")
	t.Setenv("OMNI_API_TOKEN", "test-secret")
	t.Setenv("OMNI_DEV_EVENTS", "0")
	handler := newMux()
	payload := `{"type":"updated","data":{"id":1}}`

	unauthorized := httptest.NewRequest(http.MethodPost, "/dev/events", strings.NewReader(payload))
	unauthorized.Header.Set("X-Forwarded-For", "203.0.113.10")
	unauthorizedRR := httptest.NewRecorder()
	handler.ServeHTTP(unauthorizedRR, unauthorized)
	if unauthorizedRR.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated production injection: expected 401, got %d", unauthorizedRR.Code)
	}

	authorized := httptest.NewRequest(http.MethodPost, "/dev/events", strings.NewReader(payload))
	authorized.Header.Set("X-API-Token", "test-secret")
	authorized.Header.Set("X-Forwarded-For", "203.0.113.10")
	authorizedRR := httptest.NewRecorder()
	handler.ServeHTTP(authorizedRR, authorized)
	if authorizedRR.Code != http.StatusForbidden {
		t.Fatalf("disabled production injection: expected 403, got %d", authorizedRR.Code)
	}
}
