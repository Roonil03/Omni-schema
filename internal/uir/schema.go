package uir

import "fmt"

func skipSchemaWrapper(n *Node) bool {
	if n == nil {
		return true
	}
	switch n.Key {
	case "proto_root", "graphql_root", "capnp_root", "schema":
		return true
	}
	switch n.Annotation("kind") {
	case "schema", "service", "fragment", "scalar", "rpc":
		return true
	}
	return false
}

func isRootOpType(n *Node) bool {
	switch n.Key {
	case "Query", "Mutation", "Subscription":
		return true
	default:
		return false
	}
}

// ResolvePayloadType returns the named payload type or errors if missing/ambiguous.
func ResolvePayloadType(schema *Node, typeName string) (*Node, error) {
	if schema == nil {
		return nil, nil
	}
	if typeName != "" {
		if found := schema.FindNamedType(typeName); found != nil {
			return found, nil
		}
		return nil, fmt.Errorf("schema type %q not found", typeName)
	}
	root := schema
	if skipSchemaWrapper(schema) || schema.Key == "Root" && len(schema.Children) > 0 && schema.Children[0].Type == TypeMap {
		candidates := payloadCandidates(schema)
		if len(candidates) == 0 && skipSchemaWrapper(schema) {
			candidates = payloadCandidatesFromChildren(schema)
		}
		if len(candidates) == 0 {
			return schema, nil
		}
		if len(candidates) > 1 {
			return nil, fmt.Errorf("ambiguous schema type; pass type=TypeName (candidates: %s)", candidateNames(candidates))
		}
		return candidates[0], nil
	}
	_ = root
	return schema, nil
}

func payloadCandidates(schema *Node) []*Node {
	return payloadCandidatesFromChildren(schema)
}

func payloadCandidatesFromChildren(schema *Node) []*Node {
	var out []*Node
	for _, c := range schema.Children {
		if skipSchemaWrapper(c) || isRootOpType(c) {
			continue
		}
		out = append(out, c)
	}
	return out
}

func candidateNames(ns []*Node) string {
	s := ""
	for i, n := range ns {
		if i > 0 {
			s += ", "
		}
		s += n.Key
	}
	return s
}

type FieldChange struct {
	Path    string `json:"path"`
	Kind    string `json:"kind"` // added, removed, changed
	OldType string `json:"oldType,omitempty"`
	NewType string `json:"newType,omitempty"`
}

func DiffRecursive(oldN, newN *Node) []FieldChange {
	var out []FieldChange
	diffWalk("", oldN, newN, &out)
	return out
}

func diffWalk(path string, a, b *Node, out *[]FieldChange) {
	if a == nil && b == nil {
		return
	}
	if a == nil {
		*out = append(*out, FieldChange{Path: path, Kind: "added", NewType: b.Type.String()})
		return
	}
	if b == nil {
		*out = append(*out, FieldChange{Path: path, Kind: "removed", OldType: a.Type.String()})
		return
	}
	if a.Type != b.Type || a.Annotation("gql_type") != b.Annotation("gql_type") || a.Annotation("proto_type") != b.Annotation("proto_type") {
		if path != "" {
			*out = append(*out, FieldChange{Path: path, Kind: "changed", OldType: a.Type.String(), NewType: b.Type.String()})
		}
	}
	ai := map[string]*Node{}
	bi := map[string]*Node{}
	for _, c := range a.Children {
		ai[c.Key] = c
	}
	for _, c := range b.Children {
		bi[c.Key] = c
	}
	seen := map[string]bool{}
	for k, bv := range bi {
		seen[k] = true
		p := joinPath(path, k)
		av, ok := ai[k]
		if !ok {
			*out = append(*out, FieldChange{Path: p, Kind: "added", NewType: bv.Type.String()})
			diffWalk(p, nil, bv, out)
			continue
		}
		diffWalk(p, av, bv, out)
	}
	for k, av := range ai {
		if seen[k] {
			continue
		}
		p := joinPath(path, k)
		*out = append(*out, FieldChange{Path: p, Kind: "removed", OldType: av.Type.String()})
		diffWalk(p, av, nil, out)
	}
}

func joinPath(parent, key string) string {
	if parent == "" {
		return key
	}
	return parent + "." + key
}

// CompatibilityClass is backward | forward | full | breaking.
func CompatibilityClass(oldN, newN *Node) string {
	changes := DiffRecursive(oldN, newN)
	added, removed, changed := 0, 0, 0
	for _, c := range changes {
		switch c.Kind {
		case "added":
			added++
		case "removed":
			removed++
		case "changed":
			changed++
		}
	}
	if removed == 0 && changed == 0 {
		if added == 0 {
			return "full"
		}
		return "backward"
	}
	if added == 0 && changed == 0 {
		return "forward"
	}
	return "breaking"
}
