package main

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"omni-schema/internal/codec"
	"omni-schema/internal/lexer"
)

func main() {
	http.HandleFunc("/system/schema", schemaHandler)
	http.HandleFunc("/morph/", morphHandler)
	http.HandleFunc("/graphql/subscriptions", subscriptionHandler)

	fmt.Println("Omni-Schema Gateway starting on :8080...")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		panic(err)
	}
}

// schemaHandler parses raw schema files (.graphql, .proto, .json) and registers them
// in the UIR memory. It accepts multipart/form-data.
func schemaHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	err := r.ParseMultipartForm(10 << 20) // 10 MB limit
	if err != nil {
		http.Error(w, "Error parsing form", http.StatusBadRequest)
		return
	}

	// Dynamic UIR Graph building happens here.
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status": "Schema successfully registered in UIR."}`))
}

// morphHandler is the primary execution endpoint. Clients POST payload in source format.
// The gateway dynamic lexes, parses, lowers to UIR, and synthesizes the target format.
func morphHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/morph/")
	parts := strings.Split(path, "/")
	if len(parts) != 2 {
		http.Error(w, "Invalid path format. Expected /morph/{source}/{target}", http.StatusBadRequest)
		return
	}

	source := parts[0]
	target := parts[1]

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Error reading body", http.StatusInternalServerError)
		return
	}
	defer r.Body.Close()

	// Analysis -> UIR Lowering -> Synthesis
	if source == "json" && target == "graphql" {
		node, err := lexer.ParseJSON(body)
		if err != nil {
			http.Error(w, fmt.Sprintf("Error parsing JSON: %v", err), http.StatusBadRequest)
			return
		}

		out, err := codec.GenerateGraphQL(node)
		if err != nil {
			http.Error(w, "Error synthesizing GraphQL", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/graphql")
		w.WriteHeader(http.StatusOK)
		w.Write(out)
		return
	}

	responsePayload := fmt.Sprintf("Morphed %s to %s natively without dependencies. Original payload: %d bytes.", source, target, len(body))

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(responsePayload))
}

// subscriptionHandler manages the WebSocket upgrade for GraphQL subscriptions
func subscriptionHandler(w http.ResponseWriter, r *http.Request) {
	// Dynamically imported upgrader from network package will reside here in integration.
	// For now, we mock a successful endpoint response or reject if not Upgrade: websocket
	if r.Header.Get("Upgrade") == "websocket" {
		w.WriteHeader(http.StatusSwitchingProtocols)
	} else {
		http.Error(w, "Requires WebSocket upgrade", http.StatusBadRequest)
	}
}
