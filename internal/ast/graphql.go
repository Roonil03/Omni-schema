package ast

// GraphQLDocument represents a parsed GraphQL document.
type GraphQLDocument struct {
	Definitions []GraphQLDefinition
}

type GraphQLDefinition interface {
	isGraphQLDefinition()
}

// GraphQLOperation represents an operation like query, mutation, or subscription.
type GraphQLOperation struct {
	OperationType string // "query", "mutation", "subscription"
	Name          string
	Selections    []GraphQLSelection
}

func (GraphQLOperation) isGraphQLDefinition() {}

type GraphQLSelection interface {
	isGraphQLSelection()
}

// GraphQLField represents a field requested in an operation.
type GraphQLField struct {
	Alias        string
	Name         string
	Arguments    map[string]any
	Selections   []GraphQLSelection
}

func (GraphQLField) isGraphQLSelection() {}
