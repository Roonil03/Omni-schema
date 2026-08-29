package stream

import (
	"omni-schema/internal/ast"
	"omni-schema/internal/uir"
)

func filterBySelection(n *uir.Node, selections []ast.GraphQLSelection, fragments map[string]*ast.GraphQLFragmentDefinition) *uir.Node {
	if n == nil || n.Type != uir.TypeMap {
		return n
	}
	if fragments == nil {
		fragments = map[string]*ast.GraphQLFragmentDefinition{}
	}

	filtered := uir.NewNode(uir.TypeMap, n.Key, nil)
	expanded := expandSelections(selections, fragments)

	requested := make(map[string]*ast.GraphQLField)
	for _, field := range expanded {
		requested[field.Name] = field
	}

	for _, child := range n.Children {
		if fieldSel, ok := requested[child.Key]; ok {
			var node *uir.Node
			if child.Type == uir.TypeMap && len(fieldSel.Selections) > 0 {
				node = filterBySelection(child, fieldSel.Selections, fragments)
			} else {
				clone := *child
				clone.Parent = filtered
				node = &clone
			}
			if fieldSel.Alias != "" {
				node.Key = fieldSel.Alias
			}
			node.Parent = filtered
			filtered.Children = append(filtered.Children, node)
		}
	}
	return filtered
}

func expandSelections(sels []ast.GraphQLSelection, fragments map[string]*ast.GraphQLFragmentDefinition) []*ast.GraphQLField {
	var out []*ast.GraphQLField
	for _, sel := range sels {
		switch s := sel.(type) {
		case *ast.GraphQLField:
			out = append(out, s)
		case *ast.GraphQLFragmentSpread:
			if frag, ok := fragments[s.Name]; ok {
				out = append(out, expandSelections(frag.Selections, fragments)...)
			}
		case *ast.GraphQLInlineFragment:
			out = append(out, expandSelections(s.Selections, fragments)...)
		}
	}
	return out
}
