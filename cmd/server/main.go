package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
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
	path := os.Getenv("REGISTRY_PATH")
	if path == "" {
		path = "registry_store.json"
	}
	registry.Default.StoragePath = path
	if err := registry.Default.LoadFromFile(path); err != nil {
		fmt.Printf("Note: could not load %s: %v\n", path, err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/system/schema", withCommon(schemaHandler))
	mux.HandleFunc("/system/schema/activate", withCommon(requireWriteAuth(schemaActivateHandler)))
	mux.HandleFunc("/system/schema/deprecate", withCommon(requireWriteAuth(schemaDeprecateHandler)))
	mux.HandleFunc("/system/schema/diff", withCommon(schemaDiffHandler))
	mux.HandleFunc("/morph/", withCommon(morphHandler))
	mux.HandleFunc("/morph", withCommon(morphHandler))
	mux.HandleFunc("/graphql/subscriptions", subscriptionHandler)
	mux.HandleFunc("/dev/events", withCommon(devEventHandler))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"status": "ok", "metrics": telemetry.GetMetricsSnapshot()})
	})
	mux.HandleFunc("/readyz", readyHandler)
	mux.HandleFunc("/metrics", metricsHandler)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	handler := requestIDMiddleware(rateLimitMiddleware(mux))
	srv := &http.Server{
		Addr:           ":" + port,
		Handler:        handler,
		ReadTimeout:    15 * time.Second,
		WriteTimeout:   30 * time.Second,
		IdleTimeout:    60 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	go func() {
		telemetry.Logger.Info("Omni-Schema Gateway starting", "port", port, "registry", path)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			telemetry.Logger.Error("Server failed", "error", err)
		}
	}()

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

func withCommon(next http.HandlerFunc) http.HandlerFunc { return next }

func requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id == "" {
			id = newID()
		}
		w.Header().Set("X-Request-ID", id)
		r.Header.Set("X-Request-ID", id)
		next.ServeHTTP(w, r)
	})
}

var limiter = struct {
	mu sync.Mutex
	n  map[string]int
	t  time.Time
}{n: map[string]int{}, t: time.Now()}

func rateLimitMiddleware(next http.Handler) http.Handler {
	limit := 120
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.RemoteAddr
		limiter.mu.Lock()
		if time.Since(limiter.t) > time.Minute {
			limiter.n = map[string]int{}
			limiter.t = time.Now()
		}
		limiter.n[key]++
		n := limiter.n[key]
		limiter.mu.Unlock()
		if n > limit {
			http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func newID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func writeAuthOK(r *http.Request) bool {
	tok := os.Getenv("OMNI_API_TOKEN")
	if tok == "" {
		return true
	}
	got := r.Header.Get("Authorization")
	got = strings.TrimPrefix(got, "Bearer ")
	if got == tok {
		return true
	}
	return r.Header.Get("X-API-Token") == tok
}

func requireWriteAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !writeAuthOK(r) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

func readyHandler(w http.ResponseWriter, r *http.Request) {
	if !registry.Default.Ready() {
		http.Error(w, "registry not loaded", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"status": "ready", "registry": registry.Default.StoragePath})
}

func metricsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(telemetry.GetMetricsSnapshot())
}

func schemaHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		name := r.URL.Query().Get("name")
		vers := registry.Default.List(name)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(vers)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !writeAuthOK(r) {
		http.Error(w, "unauthorized: set OMNI_API_TOKEN or send X-API-Token", http.StatusUnauthorized)
		return
	}

	schemaName := r.FormValue("name")
	if schemaName == "" {
		schemaName = "default"
	}

	r.Body = http.MaxBytesReader(w, r.Body, 10<<20)
	if err := r.ParseMultipartForm(10 << 20); err != nil {
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
	rootNode, err := parseSchema(ext, body)
	if err != nil {
		http.Error(w, err.Error(), 422)
		return
	}

	meta, err := registry.Default.Register(schemaName, ext, body, rootNode)
	if err != nil {
		http.Error(w, fmt.Sprintf("Registration Error: %v", err), 500)
		return
	}
	telemetry.HttpRequests.Add(1)
	telemetry.SchemasRegistered.Add(1)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"status": "registered", "name": meta.Name, "version": meta.Version, "format": meta.Format,
	})
}

func parseSchema(ext string, body []byte) (*uir.Node, error) {
	switch ext {
	case "graphql", "gql":
		l := &lexer.GraphQLLexer{}
		astDoc, err := l.Parse(string(body))
		if err != nil {
			return nil, fmt.Errorf("GraphQL Parse Error: %v", err)
		}
		return lower.LowerGraphQL(astDoc), nil
	case "proto":
		l := &lexer.ProtoLexer{}
		astDoc, err := l.Parse(string(body))
		if err != nil {
			return nil, fmt.Errorf("Protobuf Parse Error: %v", err)
		}
		return lower.LowerProtobuf(astDoc), nil
	case "capnp", "capnproto":
		l := &lexer.CapnProtoLexer{}
		astDoc, err := l.Parse(string(body))
		if err != nil {
			return nil, fmt.Errorf("Cap'n Proto Parse Error: %v", err)
		}
		return lower.LowerCapnProto(astDoc), nil
	case "json", "avro":
		return lexer.ParseJSON(body)
	default:
		return nil, fmt.Errorf("Unsupported schema format")
	}
}

func schemaActivateHandler(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	ver := r.URL.Query().Get("version")
	if err := registry.Default.Activate(name, ver); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"status": "activated", "name": name, "version": ver})
}

func schemaDeprecateHandler(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	ver := r.URL.Query().Get("version")
	if err := registry.Default.Deprecate(name, ver); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"status": "deprecated"})
}

func schemaDiffHandler(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")
	a, ok1 := registry.Default.GetVersion(name, from)
	b, ok2 := registry.Default.GetVersion(name, to)
	if !ok1 || !ok2 {
		http.Error(w, "versions not found", 404)
		return
	}
	d := registry.Diff(a.Root, b.Root)
	json.NewEncoder(w).Encode(map[string]any{"diff": d, "compatibility": registry.Compatibility(a.Root, b.Root)})
}

func morphHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	telemetry.HttpRequests.Add(1)
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20)
	reqID := r.Header.Get("X-Request-ID")

	schemaParam := r.URL.Query().Get("schema")
	typeParam := r.URL.Query().Get("type")
	var schemaMeta *registry.SchemaMetadata
	if schemaParam != "" {
		meta, ok := registry.Default.GetActive(schemaParam)
		if !ok {
			http.Error(w, fmt.Sprintf("Requested schema %s not found in registry", schemaParam), 404)
			return
		}
		schemaMeta = meta
	}

	body, uploadedFilename, err := readMorphBody(w, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if len(body) == 0 {
		http.Error(w, "Empty payload provided", http.StatusBadRequest)
		return
	}

	source, target := morphFormats(r, uploadedFilename)
	if source == "" || target == "" {
		http.Error(w, "Invalid path or parameters", http.StatusBadRequest)
		return
	}

	opts := codec.Options{TypeName: typeParam}
	if schemaMeta != nil {
		opts.Schema = schemaMeta.Root
	}

	t0 := time.Now()
	dataNode, parseErr := codec.DecodePayload(source, body, opts)
	telemetry.ObserveParse(time.Since(t0))
	if parseErr != nil {
		http.Error(w, fmt.Sprintf("Error parsing %s: %v", source, parseErr), 400)
		return
	}

	outputNode := dataNode
	report := &uir.ConversionReport{}
	if schemaMeta != nil && schemaMeta.Root != nil {
		schemaTarget := resolveMorphType(schemaMeta.Root, typeParam)
		if schemaTarget == nil {
			http.Error(w, "schema type not found; pass ?type=TypeName", 400)
			return
		}
		projOpts := uir.DefaultProjectOptions()
		projOpts.UnknownFields = uir.UnknownFieldIgnore
		projOpts.EmitNullForMissing = true
		projOpts.SchemaRoot = schemaMeta.Root
		projOpts.Report = report
		t1 := time.Now()
		projected, err := uir.Project(dataNode, schemaTarget, projOpts)
		telemetry.ObserveConvert(time.Since(t1))
		if err != nil {
			telemetry.ConversionFailures.Add(1)
			http.Error(w, fmt.Sprintf("Schema validation/projection error: %v", err), 400)
			return
		}
		rootWrapper := uir.NewNode(uir.TypeMap, "root", nil)
		rootWrapper.AddChild(projected)
		outputNode = rootWrapper
	}

	t2 := time.Now()
	var out []byte
	if target == "graphql" {
		out, err = codec.GenerateGraphQLSDL(outputNode)
	} else {
		out, err = codec.EncodePayload(target, outputNode, opts)
	}
	telemetry.ObserveEncode(time.Since(t2))
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
		} else if fname != "" {
			baseName = fname
		}
	}
	w.Header().Set("Content-Type", ctype)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s.%s\"", baseName, ext))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(out)))
	w.Header().Set("X-Conversion-Kind", report.Kind.String())
	w.Header().Set("X-Request-ID", reqID)
	w.WriteHeader(http.StatusOK)
	w.Write(out)
}

func resolveMorphType(root *uir.Node, typeName string) *uir.Node {
	if typeName != "" {
		if found := root.FindNamedType(typeName); found != nil {
			return found
		}
		return nil
	}
	var firstData *uir.Node
	for _, c := range root.Children {
		kind := c.Annotation("kind")
		if kind == "schema" || kind == "service" || kind == "fragment" || kind == "scalar" {
			continue
		}
		if c.Key == "Query" || c.Key == "Mutation" || c.Key == "Subscription" {
			continue
		}
		if firstData == nil {
			firstData = c
		}
	}
	return firstData
}

func readMorphBody(w http.ResponseWriter, r *http.Request) ([]byte, string, error) {
	contentType := r.Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "multipart/form-data") {
		_ = r.ParseMultipartForm(10 << 20)
		file, header, fileErr := r.FormFile("file")
		if fileErr == nil {
			defer file.Close()
			body, err := io.ReadAll(file)
			name := ""
			if header != nil {
				name = header.Filename
			}
			return body, name, err
		}
		bodyStr := r.FormValue("payload")
		if bodyStr == "" {
			bodyStr = r.FormValue("data")
		}
		if bodyStr != "" {
			return []byte(bodyStr), "", nil
		}
		return nil, "", fmt.Errorf("Missing 'file' field")
	}
	body, err := io.ReadAll(r.Body)
	return body, "", err
}

func morphFormats(r *http.Request, uploadedFilename string) (source, target string) {
	path := strings.TrimPrefix(r.URL.Path, "/morph")
	path = strings.TrimPrefix(path, "/")
	if path != "" {
		parts := strings.Split(path, "/")
		if len(parts) >= 2 {
			source, target = parts[0], parts[1]
		} else if len(parts) == 1 {
			target = parts[0]
		}
	}
	if source == "" {
		source = r.URL.Query().Get("source")
	}
	if source == "" {
		source = r.FormValue("source")
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
	if target == "" {
		target = r.FormValue("target")
	}
	return source, target
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
	case "avro":
		return "avro", "application/avro"
	case "odata":
		return "json", "application/json"
	default:
		return "bin", "application/octet-stream"
	}
}

func subscriptionHandler(w http.ResponseWriter, r *http.Request) {
	telemetry.HttpRequests.Add(1)
	telemetry.WebSocketConns.Add(1)
	defer telemetry.WebSocketConns.Add(^uint64(0))

	conn, err := network.UpgradeToWebSocket(w, r)
	if err != nil {
		http.Error(w, "WebSocket Upgrade Failed", 400)
		return
	}
	conn.SetDeadlines(60*time.Second, 30*time.Second, 0)

	schemaParam := r.URL.Query().Get("schema")
	if schemaParam == "" {
		schemaParam = "default"
	}
	targetParam := r.URL.Query().Get("target")
	if targetParam == "" {
		targetParam = "graphql"
	}

	meta, ok := registry.Default.GetActive(schemaParam)
	schemaVersion := "unknown"
	if ok {
		schemaVersion = meta.Version
	}

	activeSubs := make(map[string]*stream.Subscription)
	defer func() {
		for _, sub := range activeSubs {
			stream.DefaultBroker.RemoveSubscription(sub)
		}
	}()

	for {
		opcode, payload, err := conn.ReadMessage()
		if err != nil {
			var pe *network.ProtocolError
			if errors.As(err, &pe) {
				return // close frame already sent and TCP closed
			}
			_ = conn.Close()
			return
		}
		if opcode == network.OpClose {
			_ = conn.HandleCloseFrame(payload)
			return
		}

		if opcode != network.OpText {
			continue
		}
		var msg map[string]any
		if err := json.Unmarshal(payload, &msg); err != nil {
			_ = conn.WriteMessage(network.OpText, []byte(`{"type":"error","payload":{"message":"invalid protocol frame (not JSON)"}}`))
			_ = conn.CloseWithCode(int(network.CloseInvalidFramePayloadData), "invalid JSON")
			return
		}
		switch msg["type"].(string) {
		case "connection_init":
			conn.WriteMessage(network.OpText, []byte(`{"type":"connection_ack"}`))
		case "subscribe", "start":
			subID, _ := msg["id"].(string)
			if subID == "" {
				subID = "1"
			}
			if existing, ok := activeSubs[subID]; ok {
				stream.DefaultBroker.RemoveSubscription(existing)
				delete(activeSubs, subID)
			}

			var requestedFields []ast.GraphQLSelection
			fragments := map[string]*ast.GraphQLFragmentDefinition{}
			responseKey := ""
			eventType := ""
			opName := ""
			payloadObj, _ := msg["payload"].(map[string]any)
			if payloadObj != nil {
				opName, _ = payloadObj["operationName"].(string)
				query, _ := payloadObj["query"].(string)
				if query != "" {
					l := &lexer.GraphQLLexer{}
					doc, err := l.Parse(query)
					if err == nil {
						for _, d := range doc.Definitions {
							if f, ok := d.(*ast.GraphQLFragmentDefinition); ok {
								fragments[f.Name] = f
							}
						}
						op, err := stream.SelectOperation(doc, opName)
						if err != nil {
							b, _ := json.Marshal(map[string]any{"type": "error", "id": subID, "payload": map[string]string{"message": err.Error()}})
							conn.WriteMessage(network.OpText, b)
							continue
						}
						requestedFields = op.Selections
						if len(op.Selections) > 0 {
							if f, ok := op.Selections[0].(*ast.GraphQLField); ok {
								eventType = f.Name
								responseKey = f.Name
								if f.Alias != "" {
									responseKey = f.Alias
								}
							}
						}
					}
				}
			}

			sub := stream.NewSubscription()
			sub.ID = subID
			sub.Conn = conn
			sub.SchemaName = schemaParam
			sub.SchemaVersion = schemaVersion
			sub.RequestedFields = requestedFields
			sub.Fragments = fragments
			sub.TargetFormat = targetParam
			sub.ResponseKey = responseKey
			sub.EventType = eventType
			sub.OpName = opName
			sub.CorrelationID = r.Header.Get("X-Request-ID")
			if sub.CorrelationID == "" {
				sub.CorrelationID = newID()
			}
			if n := r.URL.Query().Get("batchSize"); n != "" {
				fmt.Sscanf(n, "%d", &sub.BatchSize)
			}

			stream.DefaultBroker.AddSubscription(sub)
			go func(s *stream.Subscription) {
				for {
					select {
					case p := <-s.Queue:
						opCode := network.OpText
						if s.TargetFormat != "graphql" && s.TargetFormat != "json" && s.TargetFormat != "odata" {
							opCode = network.OpText
						}
						_ = opCode
						if err := s.Conn.WriteMessage(network.OpText, p); err != nil {
							return
						}
					case <-s.Closed:
						return
					}
				}
			}(sub)
			activeSubs[subID] = sub
		case "complete", "stop":
			subID, _ := msg["id"].(string)
			if existing, ok := activeSubs[subID]; ok {
				stream.DefaultBroker.RemoveSubscription(existing)
				delete(activeSubs, subID)
				conn.WriteMessage(network.OpText, []byte(fmt.Sprintf(`{"type":"complete","id":%q}`, subID)))
			}
		}
	}
}

func devEventHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if os.Getenv("OMNI_ENV") == "production" && os.Getenv("OMNI_DEV_EVENTS") != "1" {
		http.Error(w, "disabled in production", http.StatusForbidden)
		return
	}
	if os.Getenv("OMNI_DEV_EVENTS") == "0" {
		http.Error(w, "disabled", http.StatusForbidden)
		return
	}
	if !writeAuthOK(r) && os.Getenv("OMNI_API_TOKEN") != "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	telemetry.HttpRequests.Add(1)
	r.Body = http.MaxBytesReader(w, r.Body, 5<<20)

	sourceFormat := r.URL.Query().Get("source")
	if sourceFormat == "" {
		sourceFormat = r.Header.Get("X-Event-Format")
	}

	ct := r.Header.Get("Content-Type")
	if sourceFormat == "" && strings.Contains(ct, "octet-stream") {
		http.Error(w, "binary events require ?source=format", 400)
		return
	}

	if sourceFormat != "" && sourceFormat != "json" {
		body, _ := io.ReadAll(r.Body)
		eventType := r.URL.Query().Get("type")
		if eventType == "" {
			eventType = "event"
		}
		stream.DefaultBroker.Publish(sourceFormat, eventType, body)
		telemetry.EventsPublished.Add(1)
		w.Write([]byte(`{"status":"published"}`))
		return
	}

	var evt struct {
		Type   string          `json:"type"`
		Data   json.RawMessage `json:"data"`
		Format string          `json:"format"`
		ID     string          `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&evt); err != nil {
		http.Error(w, "Invalid JSON", 400)
		return
	}
	fmtSrc := evt.Format
	if fmtSrc == "" {
		fmtSrc = "json"
	}
	raw := evt.Data
	if len(raw) == 0 {
		raw = []byte("{}")
	}
	if evt.ID != "" {
		stream.DefaultBroker.PublishEvent(stream.Event{ID: evt.ID, Type: evt.Type, SourceFormat: fmtSrc, Payload: raw, Time: time.Now()})
	} else {
		stream.DefaultBroker.Publish(fmtSrc, evt.Type, raw)
	}
	telemetry.EventsPublished.Add(1)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"published"}`))
}
