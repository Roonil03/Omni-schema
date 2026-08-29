package registry

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"omni-schema/internal/lexer"
	"omni-schema/internal/lower"
	"omni-schema/internal/uir"
)

const StorageFormatVersion = 2

type SchemaMetadata struct {
	Tenant     string    `json:"tenant,omitempty"`
	Name       string    `json:"name"`
	Version    string    `json:"version"`
	Format     string    `json:"format"`
	RawContent []byte    `json:"raw_content"`
	Root       *uir.Node `json:"-"`
	Timestamp  time.Time `json:"timestamp"`
	Deprecated bool      `json:"deprecated,omitempty"`
	Active     bool      `json:"active,omitempty"`
}

type persistedRegistry struct {
	FormatVersion int               `json:"format_version"`
	Active        map[string]string `json:"active"`
	Schemas       []*SchemaMetadata `json:"schemas"`
}

type Registry struct {
	mu          sync.RWMutex
	schemas     map[string][]*SchemaMetadata
	active      map[string]string
	StoragePath string
	loaded      bool
}

var Default = NewRegistry()

func NewRegistry() *Registry {
	return &Registry{
		schemas: make(map[string][]*SchemaMetadata),
		active:  make(map[string]string),
	}
}

func (r *Registry) Ready() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.loaded || r.StoragePath == ""
}

func Namespaced(tenant, name string) string {
	if tenant == "" {
		tenant = "_"
	}
	return tenant + "/" + name
}

func (r *Registry) Register(name, format string, rawContent []byte, root *uir.Node) (*SchemaMetadata, error) {
	if name == "" {
		return nil, fmt.Errorf("schema name cannot be empty")
	}
	canonicalBytes, err := json.Marshal(root)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal UIR for semantic hash: %w", err)
	}
	hash := sha256.Sum256(canonicalBytes)
	versionHash := hex.EncodeToString(hash[:])

	r.mu.Lock()
	defer r.mu.Unlock()

	if activeHash, ok := r.active[name]; ok && activeHash == versionHash {
		versions := r.schemas[name]
		for _, v := range versions {
			if v.Version == versionHash {
				return v, nil
			}
		}
	}

	meta := &SchemaMetadata{
		Name:       name,
		Version:    versionHash,
		Format:     format,
		RawContent: rawContent,
		Root:       root,
		Timestamp:  time.Now(),
		Active:     true,
	}
	if i := strings.Index(name, "/"); i >= 0 {
		meta.Tenant = name[:i]
	} else {
		meta.Tenant = "_"
	}

	prevActive, hasPrevActive := r.active[name]
	for _, v := range r.schemas[name] {
		v.Active = false
	}
	r.schemas[name] = append(r.schemas[name], meta)
	r.active[name] = versionHash

	if r.StoragePath != "" {
		if err := r.saveToFileLocked(r.StoragePath); err != nil {
			r.schemas[name] = r.schemas[name][:len(r.schemas[name])-1]
			if hasPrevActive {
				r.active[name] = prevActive
			} else {
				delete(r.active, name)
			}
			return nil, fmt.Errorf("persistence failed, rolling back: %w", err)
		}
	}
	return meta, nil
}

func (r *Registry) GetActive(name string) (*SchemaMetadata, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	activeHash, ok := r.active[name]
	if !ok {
		return nil, false
	}
	for _, v := range r.schemas[name] {
		if v.Version == activeHash {
			return v, true
		}
	}
	return nil, false
}

func (r *Registry) GetVersion(name, version string) (*SchemaMetadata, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, v := range r.schemas[name] {
		if v.Version == version {
			return v, true
		}
	}
	return nil, false
}

func (r *Registry) Activate(name, version string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	found := false
	for _, v := range r.schemas[name] {
		if v.Version == version {
			found = true
			v.Active = true
			v.Deprecated = false
		} else {
			v.Active = false
		}
	}
	if !found {
		return fmt.Errorf("version not found")
	}
	r.active[name] = version
	if r.StoragePath != "" {
		return r.saveToFileLocked(r.StoragePath)
	}
	return nil
}

func (r *Registry) Deprecate(name, version string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, v := range r.schemas[name] {
		if v.Version == version {
			v.Deprecated = true
			if r.StoragePath != "" {
				return r.saveToFileLocked(r.StoragePath)
			}
			return nil
		}
	}
	return fmt.Errorf("version not found")
}

func (r *Registry) DeleteVersion(name, version string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	vers := r.schemas[name]
	out := vers[:0]
	for _, v := range vers {
		if v.Version != version {
			out = append(out, v)
		}
	}
	if len(out) == 0 {
		delete(r.schemas, name)
		delete(r.active, name)
	} else {
		r.schemas[name] = out
		if r.active[name] == version {
			r.active[name] = out[len(out)-1].Version
			out[len(out)-1].Active = true
		}
	}
	if r.StoragePath != "" {
		return r.saveToFileLocked(r.StoragePath)
	}
	return nil
}

func (r *Registry) List(name string) []*SchemaMetadata {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return append([]*SchemaMetadata(nil), r.schemas[name]...)
}

func Diff(a, b *uir.Node) map[string][]string {
	changes := uir.DiffRecursive(a, b)
	res := map[string][]string{"added": {}, "removed": {}, "changed": {}}
	for _, c := range changes {
		res[c.Kind] = append(res[c.Kind], c.Path)
	}
	return res
}

func Compatibility(oldN, newN *uir.Node) string {
	return uir.CompatibilityClass(oldN, newN)
}

func (r *Registry) SaveToFile(filename string) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.saveToFileLocked(filename)
}

func (r *Registry) saveToFileLocked(filename string) error {
	var all []*SchemaMetadata
	for _, versions := range r.schemas {
		all = append(all, versions...)
	}
	doc := persistedRegistry{
		FormatVersion: StorageFormatVersion,
		Active:        r.active,
		Schemas:       all,
	}
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	tempFile := filename + ".tmp"
	if err := os.WriteFile(tempFile, data, 0644); err != nil {
		return err
	}
	return os.Rename(tempFile, filename)
}

func (r *Registry) LoadFromFile(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		if os.IsNotExist(err) {
			r.mu.Lock()
			r.loaded = true
			r.mu.Unlock()
			return nil
		}
		return err
	}
	defer file.Close()
	data, err := io.ReadAll(file)
	if err != nil {
		return err
	}

	var wrap persistedRegistry
	var legacy []*SchemaMetadata
	if err := json.Unmarshal(data, &wrap); err != nil || wrap.FormatVersion == 0 && wrap.Schemas == nil {
		if err2 := json.Unmarshal(data, &legacy); err2 != nil {
			return err
		}
		wrap.Schemas = legacy
		wrap.FormatVersion = 1
		wrap.Active = map[string]string{}
		for _, m := range legacy {
			wrap.Active[m.Name] = m.Version
		}
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	r.schemas = make(map[string][]*SchemaMetadata)
	r.active = wrap.Active
	if r.active == nil {
		r.active = map[string]string{}
	}
	for _, meta := range wrap.Schemas {
		root, err := rebuildRoot(meta)
		if err != nil {
			return fmt.Errorf("schema %s@%s reconstruction failed: %w", meta.Name, meta.Version, err)
		}
		if root == nil {
			return fmt.Errorf("schema %s@%s reconstruction produced no UIR (format %s)", meta.Name, meta.Version, meta.Format)
		}
		meta.Root = root
		r.schemas[meta.Name] = append(r.schemas[meta.Name], meta)
		if _, ok := r.active[meta.Name]; !ok {
			r.active[meta.Name] = meta.Version
		}
	}
	r.loaded = true
	return nil
}

func rebuildRoot(meta *SchemaMetadata) (*uir.Node, error) {
	switch meta.Format {
	case "graphql", "gql":
		l := &lexer.GraphQLLexer{}
		doc, err := l.Parse(string(meta.RawContent))
		if err != nil {
			return nil, err
		}
		return lower.LowerGraphQL(doc), nil
	case "proto", "protobuf":
		l := &lexer.ProtoLexer{}
		doc, err := l.Parse(string(meta.RawContent))
		if err != nil {
			return nil, err
		}
		return lower.LowerProtobuf(doc), nil
	case "json", "avro", "odata":
		return lexer.ParseJSON(meta.RawContent)
	case "capnp", "capnproto":
		l := &lexer.CapnProtoLexer{}
		doc, err := l.Parse(string(meta.RawContent))
		if err != nil {
			return nil, err
		}
		return lower.LowerCapnProto(doc), nil
	default:
		return nil, fmt.Errorf("unsupported persisted schema format %q", meta.Format)
	}
}
