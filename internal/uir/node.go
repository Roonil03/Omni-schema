package uir

// Node represents a node in the UIR Control Flow Graph (CFG) or Data Graph.
// It acts as the single source of truth for routing, type conversion, and data mapping.
type Node struct {
	Type  UIRType
	Key   string
	Value any

	// Edges to other nodes in the graph
	Children []*Node
	Parent   *Node // Used to handle cyclic references

	// Metadata for generic collections (e.g., Array of TypeInt32)
	ElementType UIRType

	// Used for polymorphic mappings or union types
	TypeAnnotations map[string]string
}

// NewNode initializes a new UIR Node. (Note: AllocNode in memory.go is preferred for high-throughput)
func NewNode(t UIRType, key string, val any) *Node {
	return &Node{
		Type:            t,
		Key:             key,
		Value:           val,
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
