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

type GraphQLTypeRef struct {
	NamedType string
	IsList    bool
	IsNonNull bool
	InnerType *GraphQLTypeRef
}

// GraphQLFieldDefinition represents a field inside a type definition.
type GraphQLFieldDefinition struct {
	Name string
	Type *GraphQLTypeRef
}

// GraphQLEnumDefinition represents an enum definition.
type GraphQLEnumDefinition struct {
	Name   string
	Values []string
}
func (*GraphQLEnumDefinition) isGraphQLDefinition() {}

// GraphQLUnionDefinition represents a union definition.
type GraphQLUnionDefinition struct {
	Name  string
	Types []string
}
func (*GraphQLUnionDefinition) isGraphQLDefinition() {}

// GraphQLInterfaceDefinition represents an interface definition.
type GraphQLInterfaceDefinition struct {
	Name   string
	Fields []*GraphQLFieldDefinition
}
func (*GraphQLInterfaceDefinition) isGraphQLDefinition() {}

// GraphQLInputDefinition represents an input type definition.
type GraphQLInputDefinition struct {
	Name   string
	Fields []*GraphQLFieldDefinition
}
func (*GraphQLInputDefinition) isGraphQLDefinition() {}
