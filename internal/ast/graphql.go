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
func (*GraphQLOperation) isGraphQLDefinition() {}

// GraphQLTypeDefinition represents a type definition (e.g., type User { ... }).
type GraphQLTypeDefinition struct {
	Name   string
	Fields []*GraphQLFieldDefinition
}
func (*GraphQLTypeDefinition) isGraphQLDefinition() {}

type GraphQLSelection interface {
	isGraphQLSelection()
}

// GraphQLField represents a field requested in an operation.
type GraphQLField struct {
	Alias      string
	Name       string
	Arguments  map[string]any
	Selections []GraphQLSelection
}
func (*GraphQLField) isGraphQLSelection() {}

// GraphQLFieldDefinition represents a field inside a type definition.
type GraphQLFieldDefinition struct {
	Name     string
	Type     string
	IsList   bool
	NonNull  bool
}
