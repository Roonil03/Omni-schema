package uir

// Node represents a node in the UIR Control Flow Graph (CFG) or Data Graph.
// It acts as the single source of truth for routing, type conversion, and data mapping.
type Node struct {
	Type  UIRType
	Key   string
	Value any

	Children []*Node
	Parent   *Node `json:"-"`

	ElementType UIRType
	TypeExpr    *TypeExpr

	Presence     Presence
	Cardinality  FieldCardinality
	DefaultValue any

	TypeAnnotations map[string]string
}

// NewNode initializes a new UIR Node. (Note: AllocNode in memory.go is preferred for high-throughput)
func NewNode(t UIRType, key string, val any) *Node {
	p := PresencePresent
	if t == TypeNull || val == nil && (t != TypeMap && t != TypeArray && t != TypeDefinition) {
		if t == TypeNull {
			p = PresenceNull
		}
	}
	return &Node{
		Type:            t,
		Key:             key,
		Value:           val,
		Presence:        p,
		TypeAnnotations: make(map[string]string),
	}
}

// AddChild appends a child node to the current node and establishes the parent link.
func (n *Node) AddChild(child *Node) {
	child.Parent = n
	n.Children = append(n.Children, child)
}

// SetAnnotation adds polymorphic type metadata.
func (n *Node) SetAnnotation(key, value string) {
	if n.TypeAnnotations == nil {
		n.TypeAnnotations = make(map[string]string)
	}
	n.TypeAnnotations[key] = value
}

func (n *Node) Annotation(key string) string {
	if n == nil || n.TypeAnnotations == nil {
		return ""
	}
	return n.TypeAnnotations[key]
}

func (n *Node) ChildByKey(key string) *Node {
	if n == nil {
		return nil
	}
	for _, c := range n.Children {
		if c.Key == key {
			return c
		}
	}
	return nil
}

func (n *Node) CloneShallow() *Node {
	if n == nil {
		return nil
	}
	out := NewNode(n.Type, n.Key, n.Value)
	out.ElementType = n.ElementType
	out.TypeExpr = n.TypeExpr
	out.Presence = n.Presence
	out.Cardinality = n.Cardinality
	out.DefaultValue = n.DefaultValue
	for k, v := range n.TypeAnnotations {
		out.SetAnnotation(k, v)
	}
	return out
}

func (n *Node) FindNamedType(name string) *Node {
	if n == nil {
		return nil
	}
	if n.Key == name && (n.Type == TypeMap || n.Type == TypeEnum || n.Type == TypeUnion || n.Type == TypeInterface || n.Type == TypeDefinition) {
		return n
	}
	for _, c := range n.Children {
		if found := c.FindNamedType(name); found != nil {
			return found
		}
	}
	return nil
}
