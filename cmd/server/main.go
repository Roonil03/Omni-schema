package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"strings"

	"omni-schema/internal/codec"
	"omni-schema/internal/lexer"
	"omni-schema/internal/lower"
	"omni-schema/internal/network"
	"omni-schema/internal/registry"
	"omni-schema/internal/stream"
	"omni-schema/internal/uir"
)

func main() {
	http.HandleFunc("/system/schema", schemaHandler)
	http.HandleFunc("/morph/", morphHandler)
	http.HandleFunc("/graphql/subscriptions", subscriptionHandler)
	http.HandleFunc("/dev/events", devEventHandler)

	fmt.Println("Omni-Schema Gateway starting on :8080...")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		panic(err)
	}
}

// schemaHandler parses raw schema files and registers them in the UIR memory.
func schemaHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	schemaName := r.FormValue("name")
	if schemaName == "" {
		schemaName = "default"
	}

	err := r.ParseMultipartForm(10 << 20)
	if err != nil {
		http.Error(w, "Error parsing form", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Missing file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	body, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "Error reading file", http.StatusInternalServerError)
		return
	}

	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(header.Filename), "."))
	var rootNode *uir.Node

	// Phase 1: Parse and Lower to UIR
	if ext == "graphql" || ext == "gql" {
		l := &lexer.GraphQLLexer{}
		astDoc, err := l.Parse(string(body))
		if err != nil {
			http.Error(w, fmt.Sprintf("GraphQL Parse Error: %v", err), 422)
			return
		}
		rootNode = lower.LowerGraphQL(astDoc)
	} else if ext == "proto" {
		l := &lexer.ProtoLexer{}
		astDoc, err := l.Parse(string(body))
		if err != nil {
			http.Error(w, fmt.Sprintf("Protobuf Parse Error: %v", err), 422)
			return
		}
		rootNode = lower.LowerProtobuf(astDoc)
	} else {
		http.Error(w, "Unsupported schema format", 400)
		return
	}

	// Phase 2: Register in schema registry
	meta, err := registry.Default.Register(schemaName, ext, body, rootNode)
	if err != nil {
		http.Error(w, fmt.Sprintf("Registration Error: %v", err), 500)
		return
	}

	log.Printf("Registered schema %s (version %s)", meta.Name, meta.Version)

	resp := map[string]any{
		"status":  "registered",
		"name":    meta.Name,
		"version": meta.Version,
		"format":  meta.Format,
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

// morphHandler is the primary execution endpoint.
func morphHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Determine if schema was explicitly requested
	schemaParam := r.URL.Query().Get("schema")
	var schemaMeta *registry.SchemaMetadata
	if schemaParam != "" {
		meta, ok := registry.Default.GetActive(schemaParam)
		if !ok {
			http.Error(w, fmt.Sprintf("Requested schema %s not found in registry", schemaParam), 404)
			return
		}
		schemaMeta = meta
	}

	var body []byte
	var err error
	var uploadedFilename string

	contentType := r.Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "multipart/form-data") {
		_ = r.ParseMultipartForm(10 << 20)
		file, header, fileErr := r.FormFile("file")
		if fileErr == nil {
			defer file.Close()
			body, err = io.ReadAll(file)
			if err != nil {
				http.Error(w, "Error reading uploaded file", http.StatusInternalServerError)
				return
			}
			if header != nil {
				uploadedFilename = header.Filename
			}
		} else {
			bodyStr := r.FormValue("payload")
			if bodyStr == "" {
				bodyStr = r.FormValue("data")
			}
			if bodyStr != "" {
				body = []byte(bodyStr)
			} else {
				http.Error(w, "Missing 'file' field", http.StatusBadRequest)
				return
			}
		}
	} else {
		body, err = io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Error reading body", http.StatusInternalServerError)
			return
		}
		defer r.Body.Close()
	}

	if len(body) == 0 {
		http.Error(w, "Empty payload provided", http.StatusBadRequest)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/morph")
	path = strings.TrimPrefix(path, "/")

	var source, target string
	if path != "" {
		parts := strings.Split(path, "/")
		if len(parts) >= 2 {
			source = parts[0]
			target = parts[1]
		} else if len(parts) == 1 {
			target = parts[0]
		}
	}

	if source == "" {
		source = r.URL.Query().Get("source")
	}
	if source == "" && uploadedFilename != "" {
		ext := filepath.Ext(uploadedFilename)
		if ext != "" {
			source = strings.ToLower(strings.TrimPrefix(ext, "."))
		}
	}

	if target == "" {
		target = r.URL.Query().Get("target")
	}

	if source == "" || target == "" {
		http.Error(w, "Invalid path or parameters", http.StatusBadRequest)
		return
	}

	var synthesize func() ([]byte, error)

	switch source {
	case "json":
		dataNode, parseErr := lexer.ParseJSON(body)
		if parseErr != nil {
			http.Error(w, fmt.Sprintf("Error parsing %s: %v", source, parseErr), 400)
			return
		}

		// Schema-aware projection: if a schema is registered, project the data
		// UIR against the schema UIR so that only schema-declared fields survive
		// and types are coerced to match the schema's declarations.
		outputNode := dataNode
		if schemaMeta != nil && schemaMeta.Root != nil {
			outputNode = uir.Project(dataNode, schemaMeta.Root)
		}

		switch target {
		case "graphql":
			synthesize = func() ([]byte, error) { return codec.GenerateGraphQL(outputNode) }
		case "protobuf":
			synthesize = func() ([]byte, error) { return codec.GenerateProtobuf(outputNode) }
		case "msgpack":
			synthesize = func() ([]byte, error) { return codec.GenerateMessagePack(outputNode) }
		case "parquet":
			synthesize = func() ([]byte, error) { return codec.GenerateParquet(outputNode) }
		case "capnproto":
			synthesize = func() ([]byte, error) { return codec.GenerateCapnProto(outputNode) }
		case "hdf5":
			synthesize = func() ([]byte, error) { return codec.GenerateHDF5(outputNode) }
		case "json":
			synthesize = func() ([]byte, error) { return codec.GenerateJSON(outputNode) }
		default:
			http.Error(w, fmt.Sprintf("Unsupported target format: %s", target), 400)
			return
		}
	default:
		http.Error(w, fmt.Sprintf("Unsupported source format: %s", source), 400)
		return
	}

	out, err := synthesize()
	if err != nil {
		http.Error(w, fmt.Sprintf("Error synthesizing %s: %v", target, err), 500)
		return
	}

	ext, ctype := targetFileInfo(target)
	baseName := "converted"
	if uploadedFilename != "" {
		fname := filepath.Base(uploadedFilename)
		fext := filepath.Ext(fname)
		if fext != "" {
			baseName = strings.TrimSuffix(fname, fext)
		} else if fname != "" && fname != "." && fname != "/" {
			baseName = fname
		}
	}
	filename := fmt.Sprintf("%s.%s", baseName, ext)

	w.Header().Set("Content-Type", ctype)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(out)))
	w.WriteHeader(http.StatusOK)
	w.Write(out)
}

func targetFileInfo(target string) (ext string, contentType string) {
	switch target {
	case "graphql":
		return "graphql", "application/graphql"
	case "protobuf":
		return "pb", "application/protobuf"
	case "msgpack":
		return "msgpack", "application/msgpack"
	case "parquet":
		return "parquet", "application/parquet"
	case "capnproto":
		return "capnp", "application/capnproto"
	case "hdf5":
		return "h5", "application/x-hdf5"
	case "json":
		return "json", "application/json"
	default:
		return "bin", "application/octet-stream"
	}
}

// subscriptionHandler manages the WebSocket upgrade and ties into the event broker
func subscriptionHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := network.UpgradeToWebSocket(w, r)
	if err != nil {
		http.Error(w, "WebSocket Upgrade Failed", 400)
		return
	}

	schemaParam := r.URL.Query().Get("schema")
	if schemaParam == "" {
		schemaParam = "default"
	}

	// Option A: Bind to the active schema version at connection time.
	meta, ok := registry.Default.GetActive(schemaParam)
	schemaVersion := "unknown"
	if ok {
		schemaVersion = meta.Version
	}

	// Minimal handshake protocol
	for {
		opcode, payload, err := conn.ReadMessage()
		if err != nil || opcode == network.OpClose {
			conn.Close()
			return
		}

		if opcode == network.OpText {
			var msg map[string]any
			if err := json.Unmarshal(payload, &msg); err != nil {
				continue
			}
			msgType, _ := msg["type"].(string)

			if msgType == "connection_init" {
				// ACK init
				conn.WriteMessage(network.OpText, []byte(`{"type":"connection_ack"}`))
			} else if msgType == "subscribe" {
				subID, _ := msg["id"].(string)
				if subID == "" {
					subID = "1"
				}

				sub := &stream.Subscription{
					ID:            subID,
					Conn:          conn,
					SchemaName:    schemaParam,
					SchemaVersion: schemaVersion,
					Closed:        make(chan struct{}),
				}
				
				stream.DefaultBroker.AddSubscription(sub)
				
				// Handle cleanup when connection breaks
				go func() {
					for {
						op, _, err := conn.ReadMessage()
						if err != nil || op == network.OpClose {
							stream.DefaultBroker.RemoveSubscription(sub)
							return
						}
					}
				}()
				
				return // Transfer ownership to the goroutine
			}
		}
	}
}

// devEventHandler acts as a local testing sink for events
func devEventHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var evt struct {
		Type string `json:"type"`
		Data any    `json:"data"`
	}
	if err := json.NewDecoder(r.Body).Decode(&evt); err != nil {
		http.Error(w, "Invalid JSON", 400)
		return
	}
	
	stream.DefaultBroker.Publish(evt.Type, evt.Data)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status": "published"}`))
}
