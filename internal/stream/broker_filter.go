package stream

import (
	"omni-schema/internal/ast"
	"omni-schema/internal/uir"
)

// filterBySelection recursively prunes a UIR node to keep only the fields specified in the GraphQL selection set.
func filterBySelection(n *uir.Node, selections []ast.GraphQLSelection) *uir.Node {
	if n == nil || n.Type != uir.TypeMap {
		return n
	}

	filtered := uir.NewNode(uir.TypeMap, n.Key, n.Parent)
	
	// Fast lookup for requested fields
	requested := make(map[string]*ast.GraphQLField)
	for _, sel := range selections {
		if field, ok := sel.(*ast.GraphQLField); ok {
			requested[field.Name] = field
		}
	}

	for _, child := range n.Children {
		if fieldSel, ok := requested[child.Key]; ok {
			// If it's a nested object and the selection has sub-selections, recurse
			if child.Type == uir.TypeMap && len(fieldSel.Selections) > 0 {
				subFiltered := filterBySelection(child, fieldSel.Selections)
				subFiltered.Parent = filtered
				filtered.Children = append(filtered.Children, subFiltered)
			} else {
				// Keep as-is
				clone := *child
				clone.Parent = filtered
				filtered.Children = append(filtered.Children, &clone)
			}
		}
	}
	return filtered
}
