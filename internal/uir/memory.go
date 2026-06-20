package uir

import "sync"

var nodePool = sync.Pool{
	New: func() interface{} {
		return &Node{
			TypeAnnotations: make(map[string]string),
		}
	},
}

// AllocNode retrieves a UIR Node from the object pool to minimize garbage collection overhead
// during high-throughput translation.
func AllocNode() *Node {
	n := nodePool.Get().(*Node)
	return n
}

// FreeNode returns a UIR Node back to the pool, clearing its state to prevent memory leaks.
// It avoids deep recursive freeing directly to prevent double-free issues in DAG/cyclic structures.
// A specialized traversal should be used to free entire graphs safely.
func FreeNode(n *Node) {
	n.Type = TypeUnknown
	n.Key = ""
	n.Value = nil
	
	// Retain the capacity of the slice but clear elements
	if n.Children != nil {
		n.Children = n.Children[:0]
	}
	
	n.Parent = nil
	n.ElementType = TypeUnknown
	
	for k := range n.TypeAnnotations {
		delete(n.TypeAnnotations, k)
	}
	
	nodePool.Put(n)
}
