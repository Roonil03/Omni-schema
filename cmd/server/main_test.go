package main

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"omni-schema/internal/registry"
)

func TestSystemSchemaHandler(t *testing.T) {
	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)

	writer.WriteField("name", "test_schema")
	part, _ := writer.CreateFormFile("file", "test.graphql")
	part.Write([]byte(`type User { id: Int! name: String! }`))
	writer.Close()

	req := httptest.NewRequest("POST", "/system/schema", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	rr := httptest.NewRecorder()
	schemaHandler(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	if !bytes.Contains(rr.Body.Bytes(), []byte("registered")) {
		t.Errorf("handler returned unexpected body: got %v", rr.Body.String())
	}
}

func TestMorphHandler(t *testing.T) {
	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)

	part, _ := writer.CreateFormFile("file", "data.json")
	part.Write([]byte(`{"user": {"id": 1, "name": "Alice"}}`))
	writer.Close()

	req := httptest.NewRequest("POST", "/morph/json/graphql", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	rr := httptest.NewRecorder()
	morphHandler(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	if !bytes.Contains(rr.Body.Bytes(), []byte("type Root")) {
		t.Errorf("expected generated GraphQL to contain 'type Root', got %v", rr.Body.String())
	}
}

// TestSchemaAwareMorph verifies that when a schema is registered and referenced via
// ?schema=name, only the fields declared in the schema appear in the output and
// extra data fields are dropped (projection semantics).
func TestSchemaAwareMorph(t *testing.T) {
	// Reset registry to avoid cross-test pollution.
	registry.Default = registry.NewRegistry()

	// Step 1: Register a schema that only declares "id" and "name".
	schemaBody := new(bytes.Buffer)
	schemaWriter := multipart.NewWriter(schemaBody)
	schemaWriter.WriteField("name", "user_schema")
	schemaPart, _ := schemaWriter.CreateFormFile("file", "user.graphql")
	schemaPart.Write([]byte(`type User { id: Int! name: String! }`))
	schemaWriter.Close()

	schemaReq := httptest.NewRequest("POST", "/system/schema", schemaBody)
	schemaReq.Header.Set("Content-Type", schemaWriter.FormDataContentType())
	schemaRR := httptest.NewRecorder()
	schemaHandler(schemaRR, schemaReq)

	if schemaRR.Code != http.StatusOK {
		t.Fatalf("schema registration failed: %s", schemaRR.Body.String())
	}

	// Extract the version from the response.
	var regResp map[string]any
	json.Unmarshal(schemaRR.Body.Bytes(), &regResp)
	t.Logf("Registered schema: %v", regResp)

	// Step 2: Morph a JSON payload that has EXTRA fields ("email", "role")
	// with ?schema=user_schema. The extra fields should be dropped.
	morphBody := new(bytes.Buffer)
	morphWriter := multipart.NewWriter(morphBody)
	morphPart, _ := morphWriter.CreateFormFile("file", "data.json")
	morphPart.Write([]byte(`{"id": 42, "name": "Bob", "email": "bob@test.com", "role": "admin"}`))
	morphWriter.Close()

	morphReq := httptest.NewRequest("POST", "/morph/json/graphql?schema=user_schema", morphBody)
	morphReq.Header.Set("Content-Type", morphWriter.FormDataContentType())
	morphRR := httptest.NewRecorder()
	morphHandler(morphRR, morphReq)

	if morphRR.Code != http.StatusOK {
		t.Fatalf("morph handler returned %d: %s", morphRR.Code, morphRR.Body.String())
	}

	output := morphRR.Body.String()
	t.Logf("Schema-aware morph output:\n%s", output)

	// The output should contain "id" and "name" (from the schema).
	if !strings.Contains(output, "id:") {
		t.Errorf("expected output to contain 'id' field from schema, got:\n%s", output)
	}
	if !strings.Contains(output, "name:") {
		t.Errorf("expected output to contain 'name' field from schema, got:\n%s", output)
	}

	// The output should NOT contain "email" or "role" (not in schema).
	if strings.Contains(output, "email") {
		t.Errorf("expected output to NOT contain 'email' (not in schema), got:\n%s", output)
	}
	if strings.Contains(output, "role") {
		t.Errorf("expected output to NOT contain 'role' (not in schema), got:\n%s", output)
	}
}

// TestNestedJSONGraphQLGeneration verifies that deeply nested JSON produces valid
// flat GraphQL SDL with separate named type definitions rather than inline braces.
func TestNestedJSONGraphQLGeneration(t *testing.T) {
	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)

	part, _ := writer.CreateFormFile("file", "nested.json")
	part.Write([]byte(`{
		"user": {
			"name": "Alice",
			"address": {
				"city": "London",
				"country": "UK"
			}
		},
		"active": true
	}`))
	writer.Close()

	req := httptest.NewRequest("POST", "/morph/json/graphql", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	rr := httptest.NewRecorder()
	morphHandler(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("handler returned %d: %s", rr.Code, rr.Body.String())
	}

	output := rr.Body.String()
	t.Logf("Nested JSON output:\n%s", output)

	// Must contain "type Root" as the top-level type.
	if !strings.Contains(output, "type Root") {
		t.Errorf("expected 'type Root' in output:\n%s", output)
	}

	// Must NOT contain inline braces like "user: {" — that would be invalid SDL.
	if strings.Contains(output, ": {") {
		t.Errorf("output contains inline braces '{ }' which is invalid GraphQL SDL:\n%s", output)
	}

	// Each "type" keyword should be at the start of a type definition block.
	typeCount := strings.Count(output, "type ")
	if typeCount < 3 {
		t.Errorf("expected at least 3 type definitions (Root, Root_User, Root_User_Address), got %d in:\n%s", typeCount, output)
	}

	// Verify the nested types are named deterministically.
	if !strings.Contains(output, "Root_User") {
		t.Errorf("expected named type 'Root_User' for nested user object:\n%s", output)
	}
}

// TestSchemaAwareMorphMissingSchema verifies that requesting a non-existent schema
// returns a 404.
func TestSchemaAwareMorphMissingSchema(t *testing.T) {
	registry.Default = registry.NewRegistry()

	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "data.json")
	part.Write([]byte(`{"id": 1}`))
	writer.Close()

	req := httptest.NewRequest("POST", "/morph/json/graphql?schema=nonexistent", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	rr := httptest.NewRecorder()
	morphHandler(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404 for missing schema, got %d: %s", rr.Code, rr.Body.String())
	}
}
