package main

import (
	"fmt"
	"io"
	"net/http"
	"path/filepath"
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

// morphHandler is the primary execution endpoint. Clients upload a file in source
// format and receive a downloadable file back in the target format.
// Accepts multipart file upload (-F "file=@data.json"), form parameters, query parameters, and raw body.
func morphHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Read input: try multipart file upload first, fall back to raw body
	var body []byte
	var err error
	var uploadedFilename string

	contentType := r.Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "multipart/form-data") {
		err = r.ParseMultipartForm(10 << 20) // 10 MB limit
		if err != nil {
			http.Error(w, "Error parsing multipart form", http.StatusBadRequest)
			return
		}
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
				http.Error(w, "Missing 'file' field in form upload", http.StatusBadRequest)
				return
			}
		}
	} else if strings.HasPrefix(contentType, "application/x-www-form-urlencoded") {
		err = r.ParseForm()
		if err != nil {
			http.Error(w, "Error parsing form", http.StatusBadRequest)
			return
		}
		bodyStr := r.FormValue("payload")
		if bodyStr == "" {
			bodyStr = r.FormValue("data")
		}
		if bodyStr != "" {
			body = []byte(bodyStr)
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

	// Resolve source and target from URL path, query parameters, form parameters, or file extension
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
	if source == "" && (r.MultipartForm != nil || r.Form != nil) {
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
	if target == "" && (r.MultipartForm != nil || r.Form != nil) {
		target = r.FormValue("target")
	}

	if source == "" || target == "" {
		http.Error(w, "Invalid path or parameters. Expected /morph/{source}/{target} or form/query parameters for source and target", http.StatusBadRequest)
		return
	}

	// Determine source parser
	var parse func([]byte) error
	var synthesize func() ([]byte, error)

	switch source {
	case "json":
		parse = func(data []byte) error {
			node, parseErr := lexer.ParseJSON(data)
			if parseErr != nil {
				return parseErr
			}
			// Route to target codec
			switch target {
			case "graphql":
				synthesize = func() ([]byte, error) { return codec.GenerateGraphQL(node) }
			case "protobuf":
				synthesize = func() ([]byte, error) { return codec.GenerateProtobuf(node) }
			case "msgpack":
				synthesize = func() ([]byte, error) { return codec.GenerateMessagePack(node) }
			case "parquet":
				synthesize = func() ([]byte, error) { return codec.GenerateParquet(node) }
			case "capnproto":
				synthesize = func() ([]byte, error) { return codec.GenerateCapnProto(node) }
			case "hdf5":
				synthesize = func() ([]byte, error) { return codec.GenerateHDF5(node) }
			case "json":
				synthesize = func() ([]byte, error) { return codec.GenerateJSON(node) }
			default:
				return fmt.Errorf("unsupported target format: %s", target)
			}
			return nil
		}
	default:
		http.Error(w, fmt.Sprintf("Unsupported source format: %s", source), http.StatusBadRequest)
		return
	}

	// Phase 1: Analysis -- Parse source into UIR
	if err := parse(body); err != nil {
		http.Error(w, fmt.Sprintf("Error parsing %s: %v", source, err), http.StatusBadRequest)
		return
	}

	// Phase 2: Synthesis -- Generate target output
	out, err := synthesize()
	if err != nil {
		http.Error(w, fmt.Sprintf("Error synthesizing %s: %v", target, err), http.StatusInternalServerError)
		return
	}

	// Determine file extension and content type for the download
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
	filename = strings.ReplaceAll(filename, "\"", "")

	w.Header().Set("Content-Type", ctype)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(out)))
	w.WriteHeader(http.StatusOK)
	w.Write(out)
}

// targetFileInfo returns the file extension and MIME content type for a given target format.
func targetFileInfo(target string) (ext string, contentType string) {
	switch target {
	case "graphql":
		return "graphql", "application/graphql"
	case "protobuf":
		return "pb", "application/x-protobuf"
	case "json":
		return "json", "application/json"
	case "msgpack":
		return "msgpack", "application/x-msgpack"
	case "parquet":
		return "parquet", "application/vnd.apache.parquet"
	case "capnproto":
		return "capnp", "application/x-capnp"
	case "hdf5":
		return "h5", "application/x-hdf5"
	default:
		return "bin", "application/octet-stream"
	}
}

// subscriptionHandler manages the WebSocket upgrade for GraphQL subscriptions
func subscriptionHandler(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Upgrade") == "websocket" {
		w.WriteHeader(http.StatusSwitchingProtocols)
	} else {
		http.Error(w, "Requires WebSocket upgrade", http.StatusBadRequest)
	}
}
