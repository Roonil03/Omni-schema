package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"omni-schema/internal/ast"
	"omni-schema/internal/codec"
	"omni-schema/internal/lexer"
	"omni-schema/internal/lower"
	"omni-schema/internal/network"
	"omni-schema/internal/registry"
	"omni-schema/internal/stream"
	"omni-schema/internal/telemetry"
	"omni-schema/internal/uir"
)

func main() {
	// Attempt to load persistent registry
	registry.Default.StoragePath = "registry_store.json"
	if err := registry.Default.LoadFromFile("registry_store.json"); err != nil {
		fmt.Printf("Note: could not load registry_store.json: %v\n", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/system/schema", schemaHandler)
	mux.HandleFunc("/morph/", morphHandler)
	mux.HandleFunc("/graphql/subscriptions", subscriptionHandler)
	mux.HandleFunc("/dev/events", devEventHandler)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"status": "ok", "metrics": telemetry.GetMetricsSnapshot()})
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ready"))
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	srv := &http.Server{
		Addr:           ":" + port,
		Handler:        mux,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		IdleTimeout:    60 * time.Second,
		MaxHeaderBytes: 1 << 20, // 1MB
	}

	go func() {
		telemetry.Logger.Info("Omni-Schema Gateway starting", "port", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			telemetry.Logger.Error("Server failed", "error", err)
		}
	}()

	// Wait for interrupt signal to gracefully shut down the server with a timeout of 5 seconds.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("Server forced to shutdown:", err)
	}

	log.Println("Server exiting")
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

	r.Body = http.MaxBytesReader(w, r.Body, 10<<20) // 10MB limit
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
	switch ext {
	case "graphql", "gql":
		l := &lexer.GraphQLLexer{}
		astDoc, err := l.Parse(string(body))
		if err != nil {
			http.Error(w, fmt.Sprintf("GraphQL Parse Error: %v", err), 422)
			return
		}
		rootNode = lower.LowerGraphQL(astDoc)
	case "proto":
		l := &lexer.ProtoLexer{}
		astDoc, err := l.Parse(string(body))
		if err != nil {
			http.Error(w, fmt.Sprintf("Protobuf Parse Error: %v", err), 422)
			return
		}
		rootNode = lower.LowerProtobuf(astDoc)
	default:
		http.Error(w, "Unsupported schema format", 400)
		return
	}

	// Phase 2: Register in schema registry (which now auto-persists)
	meta, err := registry.Default.Register(schemaName, ext, body, rootNode)
	if err != nil {
		http.Error(w, fmt.Sprintf("Registration Error: %v", err), 500)
		return
	}

	telemetry.HttpRequests.Add(1)
	telemetry.SchemasRegistered.Add(1)

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

	telemetry.HttpRequests.Add(1)
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20) // 10MB limit

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

	decoder, err := codec.GetDecoder(source)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	encoder, err := codec.GetEncoder(target)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	dataNode, parseErr := decoder.Decode(body)
	if parseErr != nil {
		http.Error(w, fmt.Sprintf("Error parsing %s: %v", source, parseErr), 400)
		return
	}

	// Schema-aware projection: if a schema is registered, project the data
	// UIR against the schema UIR so that only schema-declared fields survive
	// and types are coerced to match the schema's declarations.
	outputNode := dataNode
	if schemaMeta != nil && schemaMeta.Root != nil {
		schemaTarget := schemaMeta.Root
		if len(schemaTarget.Children) > 0 {
			schemaTarget = schemaTarget.Children[0]
		}
		opts := uir.ProjectOptions{
			UnknownFields:      uir.UnknownFieldIgnore,
			EmitNullForMissing: true,
		}
		projected, err := uir.Project(dataNode, schemaTarget, opts)
		if err != nil {
			telemetry.ConversionFailures.Add(1)
			http.Error(w, fmt.Sprintf("Schema validation/projection error: %v", err), 400)
			return
		}
		// Re-wrap the projected node in a root map if the original schemaTarget was a child,
		// to preserve the target format structure if needed, or just use projected directly.
		// Actually, codec.GenerateGraphQL expects a root node with type definitions.
		rootWrapper := uir.NewNode(uir.TypeMap, "root", nil)
		rootWrapper.AddChild(projected)
		outputNode = rootWrapper
	}

	out, err := encoder.Encode(outputNode)
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
	telemetry.HttpRequests.Add(1)
	telemetry.WebSocketConns.Add(1)
	defer telemetry.WebSocketConns.Add(^uint64(0)) // decrement on exit

	conn, err := network.UpgradeToWebSocket(w, r)
	if err != nil {
		http.Error(w, "WebSocket Upgrade Failed", 400)
		return
	}

	schemaParam := r.URL.Query().Get("schema")
	if schemaParam == "" {
		schemaParam = "default"
	}

	targetParam := r.URL.Query().Get("target")
	if targetParam == "" {
		targetParam = "graphql"
	}

	// Option A: Bind to the active schema version at connection time.
	meta, ok := registry.Default.GetActive(schemaParam)
	schemaVersion := "unknown"
	if ok {
		schemaVersion = meta.Version
	}

	activeSubs := make(map[string]*stream.Subscription)
	defer func() {
		for _, sub := range activeSubs {
			close(sub.Closed)
			stream.DefaultBroker.RemoveSubscription(sub)
		}
	}()

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
				// Send protocol error
				conn.WriteMessage(network.OpText, []byte(`{"type":"error", "payload":{"message":"invalid protocol frame (not JSON)"}}`))
				conn.Close()
				return
			}
			msgType, _ := msg["type"].(string)

			switch msgType {
			case "connection_init":
				// ACK init
				conn.WriteMessage(network.OpText, []byte(`{"type":"connection_ack"}`))
			case "subscribe", "start":
				subID, _ := msg["id"].(string)
				if subID == "" {
					subID = "1"
				}

				if existing, ok := activeSubs[subID]; ok {
					close(existing.Closed)
					stream.DefaultBroker.RemoveSubscription(existing)
					delete(activeSubs, subID)
				}

				var requestedFields []ast.GraphQLSelection
				
				payloadObj, ok := msg["payload"].(map[string]any)
				if ok {
					query, _ := payloadObj["query"].(string)
					if query != "" {
						l := &lexer.GraphQLLexer{}
						doc, err := l.Parse(query)
						if err == nil && len(doc.Definitions) > 0 {
							if op, ok := doc.Definitions[0].(*ast.GraphQLOperation); ok {
								requestedFields = op.Selections
							}
						}
					}
				}

				sub := &stream.Subscription{
					ID:              subID,
					Conn:            conn,
					SchemaName:      schemaParam,
					SchemaVersion:   schemaVersion,
					RequestedFields: requestedFields,
					Queue:           make(chan []byte, 100),
					Closed:          make(chan struct{}),
					TargetFormat:    targetParam,
				}
				
				stream.DefaultBroker.AddSubscription(sub)
				
				// Dedicated writer goroutine per subscriber
				go func(s *stream.Subscription) {
					for {
						select {
						case p := <-s.Queue:
							opCode := network.OpText
							if s.TargetFormat != "graphql" && s.TargetFormat != "json" {
								opCode = network.OpBinary
							}
							if err := s.Conn.WriteMessage(opCode, p); err != nil {
								return // network failure, let reader loop detect and cleanup
							}
						case <-s.Closed:
							return // cleanly exit writer when reader closes
						}
					}
				}(sub)
				
				activeSubs[subID] = sub
			case "complete", "stop":
				subID, _ := msg["id"].(string)
				if existing, ok := activeSubs[subID]; ok {
					close(existing.Closed)
					stream.DefaultBroker.RemoveSubscription(existing)
					delete(activeSubs, subID)
				}
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
	
	telemetry.HttpRequests.Add(1)
	r.Body = http.MaxBytesReader(w, r.Body, 5<<20) // 5MB limit
	
	var evt struct {
		Type string `json:"type"`
		Data any    `json:"data"`
	}
	if err := json.NewDecoder(r.Body).Decode(&evt); err != nil {
		http.Error(w, "Invalid JSON", 400)
		return
	}
	rawBytes, _ := json.Marshal(evt.Data)
	stream.DefaultBroker.Publish("json", evt.Type, rawBytes)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status": "published"}`))
}
