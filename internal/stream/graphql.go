package stream

import (
	"fmt"

	"omni-schema/internal/ast"
	"omni-schema/internal/uir"
)

type RootField struct {
	Name       string
	Alias      string
	ReturnType string
	Selections []ast.GraphQLSelection
}

func SelectSubscription(doc *ast.GraphQLDocument, operationName string) (*ast.GraphQLOperation, error) {
	op, err := SelectOperation(doc, operationName)
	if err != nil {
		return nil, err
	}
	if op.OperationType != "subscription" {
		return nil, fmt.Errorf("operation must be a subscription, got %q", op.OperationType)
	}
	return op, nil
}

func RootFieldsFromOp(op *ast.GraphQLOperation, schema *uir.Node) ([]RootField, error) {
	if op == nil || len(op.Selections) == 0 {
		return nil, fmt.Errorf("subscription has no root fields")
	}
	var subType *uir.Node
	if schema != nil {
		subType = schema.FindNamedType("Subscription")
	}
	var out []RootField
	for _, sel := range op.Selections {
		f, ok := sel.(*ast.GraphQLField)
		if !ok {
			return nil, fmt.Errorf("subscription root must be a field")
		}
		rf := RootField{Name: f.Name, Alias: f.Alias, Selections: f.Selections}
		if rf.Alias == "" {
			rf.Alias = f.Name
		}
		if subType != nil {
			field := subType.ChildByKey(f.Name)
			if field == nil {
				return nil, fmt.Errorf("field %q is not in Subscription", f.Name)
			}
			rf.ReturnType = field.Annotation("gql_type")
			if expr := field.Annotation("gql_type_expr"); expr != "" && rf.ReturnType == "" {
				rf.ReturnType = uir.ParseGraphQLTypeExpr(expr).NamedLeaf()
			}
		} else if schema != nil {
			return nil, fmt.Errorf("schema has no Subscription type")
		}
		out = append(out, rf)
	}
	return out, nil
}

func ValidateSelections(n *uir.Node, sels []ast.GraphQLSelection, fragments map[string]*ast.GraphQLFragmentDefinition) error {
	if n == nil || len(sels) == 0 {
		return nil
	}
	expanded := expandSelections(sels, fragments)
	index := map[string]*uir.Node{}
	for _, c := range n.Children {
		index[c.Key] = c
	}
	for _, f := range expanded {
		child, ok := index[f.Name]
		if !ok {
			return fmt.Errorf("unknown field %q on type %s", f.Name, n.Key)
		}
		if len(f.Selections) > 0 {
			target := child
			if named := child.Annotation("gql_type"); named != "" && child.Parent != nil {
				root := child
				for root.Parent != nil {
					root = root.Parent
				}
				if t := root.FindNamedType(named); t != nil {
					target = t
				}
			}
			if err := ValidateSelections(target, f.Selections, fragments); err != nil {
				return err
			}
		}
	}
	return nil
}

func ReturnTypeForEvent(schema *uir.Node, fieldName string) (*uir.Node, error) {
	if schema == nil {
		return nil, fmt.Errorf("no schema")
	}
	sub := schema.FindNamedType("Subscription")
	if sub == nil {
		return nil, fmt.Errorf("schema has no Subscription type")
	}
	field := sub.ChildByKey(fieldName)
	if field == nil {
		return nil, fmt.Errorf("Subscription has no field %q", fieldName)
	}
	named := field.Annotation("gql_type")
	if named == "" && field.TypeExpr != nil {
		named = field.TypeExpr.NamedLeaf()
	}
	if named == "" {
		return field, nil
	}
	t := schema.FindNamedType(named)
	if t == nil {
		return nil, fmt.Errorf("return type %q for %s not found", named, fieldName)
	}
	return t, nil
}
