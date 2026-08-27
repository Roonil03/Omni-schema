package registry

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"omni-schema/internal/uir"
)

// SchemaMetadata represents a registered schema in the UIR format.
type SchemaMetadata struct {
	Name      string
	Version   string
	Format    string
	Root      *uir.Node
	Timestamp time.Time
}

// Registry manages in-memory schema registrations atomically.
type Registry struct {
	mu sync.RWMutex
	// schemas maps a schema name to an ordered list of versions
	schemas map[string][]*SchemaMetadata
	// active maps a schema name to its latest version hash
	active map[string]string
}

// Default is the global registry instance.
var Default = NewRegistry()

// NewRegistry initializes a new schema registry.
func NewRegistry() *Registry {
	return &Registry{
		schemas: make(map[string][]*SchemaMetadata),
		active:  make(map[string]string),
	}
}

// Register stores a parsed schema. It hashes the raw content to identify identical versions.
// If the content is identical to the active version, it returns the existing one to avoid duplicates.
func (r *Registry) Register(name, format string, rawContent []byte, root *uir.Node) (*SchemaMetadata, error) {
	if name == "" {
		return nil, fmt.Errorf("schema name cannot be empty")
	}

	hash := sha1.Sum(rawContent)
	versionHash := hex.EncodeToString(hash[:])[:8] // 8-char short hash

	r.mu.Lock()
	defer r.mu.Unlock()

	// Check if this exact version is already the active one
	if activeHash, ok := r.active[name]; ok && activeHash == versionHash {
		versions := r.schemas[name]
		if len(versions) > 0 {
			return versions[len(versions)-1], nil
		}
	}

	meta := &SchemaMetadata{
		Name:      name,
		Version:   versionHash,
		Format:    format,
		Root:      root,
		Timestamp: time.Now(),
	}

	r.schemas[name] = append(r.schemas[name], meta)
	r.active[name] = versionHash

	return meta, nil
}

// GetActive retrieves the latest version of a registered schema.
func (r *Registry) GetActive(name string) (*SchemaMetadata, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	versions, exists := r.schemas[name]
	if !exists || len(versions) == 0 {
		return nil, false
	}
	return versions[len(versions)-1], true
}

// GetVersion retrieves a specific version of a registered schema.
func (r *Registry) GetVersion(name, version string) (*SchemaMetadata, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	versions, exists := r.schemas[name]
	if !exists {
		return nil, false
	}
	for _, v := range versions {
		if v.Version == version {
			return v, true
		}
	}
	return nil, false
}
