package uir

import "strings"

// TypeExprKind classifies a GraphQL-style (and generic) type expression node.
type TypeExprKind int

const (
	TypeExprNamed TypeExprKind = iota
	TypeExprList
	TypeExprNonNull
)

// TypeExpr is a first-class nested type expression.
// Example: [[User!]!]! becomes
//
//	NonNull -> List -> NonNull -> List -> NonNull -> Named(User)
type TypeExpr struct {
	Kind TypeExprKind
	Name string
	Of   *TypeExpr
}

func NamedType(name string) *TypeExpr {
	return &TypeExpr{Kind: TypeExprNamed, Name: name}
}

func ListOf(inner *TypeExpr) *TypeExpr {
	return &TypeExpr{Kind: TypeExprList, Of: inner}
}

func NonNullOf(inner *TypeExpr) *TypeExpr {
	return &TypeExpr{Kind: TypeExprNonNull, Of: inner}
}

func (t *TypeExpr) IsNonNull() bool {
	return t != nil && t.Kind == TypeExprNonNull
}

func (t *TypeExpr) UnwrapNonNull() *TypeExpr {
	if t != nil && t.Kind == TypeExprNonNull {
		return t.Of
	}
	return t
}

func (t *TypeExpr) NamedLeaf() string {
	for cur := t; cur != nil; cur = cur.Of {
		if cur.Kind == TypeExprNamed {
			return cur.Name
		}
	}
	return ""
}

func (t *TypeExpr) String() string {
	if t == nil {
		return ""
	}
	switch t.Kind {
	case TypeExprNamed:
		return t.Name
	case TypeExprList:
		return "[" + t.Of.String() + "]"
	case TypeExprNonNull:
		return t.Of.String() + "!"
	default:
		return "?"
	}
}

// ParseGraphQLTypeExpr parses strings such as "[[User!]!]!".
func ParseGraphQLTypeExpr(s string) *TypeExpr {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	expr, rest := parseTypeExpr(s)
	if strings.TrimSpace(rest) != "" {
		return expr
	}
	return expr
}

func parseTypeExpr(s string) (*TypeExpr, string) {
	s = strings.TrimLeft(s, " \t")
	var expr *TypeExpr
	if strings.HasPrefix(s, "[") {
		inner, rest := parseTypeExpr(s[1:])
		rest = strings.TrimLeft(rest, " \t")
		if strings.HasPrefix(rest, "]") {
			rest = rest[1:]
		}
		expr = ListOf(inner)
		s = rest
	} else {
		i := 0
		for i < len(s) && (s[i] == '_' || s[i] == '.' || (s[i] >= 'A' && s[i] <= 'Z') || (s[i] >= 'a' && s[i] <= 'z') || (s[i] >= '0' && s[i] <= '9')) {
			i++
		}
		expr = NamedType(s[:i])
		s = s[i:]
	}
	s = strings.TrimLeft(s, " \t")
	for strings.HasPrefix(s, "!") {
		expr = NonNullOf(expr)
		s = strings.TrimLeft(s[1:], " \t")
	}
	return expr, s
}
