package lexer

import (
	"fmt"
	"strings"
	"text/scanner"

	"omni-schema/internal/ast"
)

// GraphQLLexer is a custom lexer/parser for GraphQL utilizing text/scanner.
type GraphQLLexer struct {
	scan scanner.Scanner
	tok  rune
}

// Parse parses a GraphQL document from a string.
func (l *GraphQLLexer) Parse(input string) (*ast.GraphQLDocument, error) {
	l.scan.Init(strings.NewReader(input))
	l.scan.Mode = scanner.ScanIdents | scanner.ScanStrings | scanner.ScanInts | scanner.ScanFloats
	l.scan.Whitespace = 1<<'\t' | 1<<'\n' | 1<<'\r' | 1<<' ' | 1<<','
	
	l.next() // prime the first token

	doc := &ast.GraphQLDocument{}
	for l.tok != scanner.EOF {
		text := l.scan.TokenText()
		if text == "type" {
			typeDef, err := l.parseTypeDefinition()
			if err != nil {
				return nil, err
			}
			doc.Definitions = append(doc.Definitions, typeDef)
		} else if text == "interface" {
			interfaceDef, err := l.parseInterfaceDefinition()
			if err != nil {
				return nil, err
			}
			doc.Definitions = append(doc.Definitions, interfaceDef)
		} else if text == "input" {
			inputDef, err := l.parseInputDefinition()
			if err != nil {
				return nil, err
			}
			doc.Definitions = append(doc.Definitions, inputDef)
		} else if text == "enum" {
			enumDef, err := l.parseEnumDefinition()
			if err != nil {
				return nil, err
			}
			doc.Definitions = append(doc.Definitions, enumDef)
		} else if text == "union" {
			unionDef, err := l.parseUnionDefinition()
			if err != nil {
				return nil, err
			}
			doc.Definitions = append(doc.Definitions, unionDef)
		} else if text == "query" || text == "mutation" || text == "subscription" {
			opDef, err := l.parseOperationDefinition(text)
			if err != nil {
				return nil, err
			}
			doc.Definitions = append(doc.Definitions, opDef)
		} else if text == "fragment" {
			frag, err := l.parseFragmentDefinition()
			if err != nil {
				return nil, err
			}
			doc.Definitions = append(doc.Definitions, frag)
		} else if text == "scalar" {
			l.next()
			if l.tok != scanner.Ident {
				return nil, fmt.Errorf("expected scalar name")
			}
			doc.Definitions = append(doc.Definitions, &ast.GraphQLScalarDefinition{Name: l.scan.TokenText()})
			l.next()
		} else if text == "schema" {
			sch, err := l.parseSchemaDefinition()
			if err != nil {
				return nil, err
			}
			doc.Definitions = append(doc.Definitions, sch)
		} else {
			l.next()
		}
	}

	return doc, nil
}

func (l *GraphQLLexer) next() {
	l.tok = l.scan.Scan()
}

func (l *GraphQLLexer) parseTypeRef() (*ast.GraphQLTypeRef, error) {
	ref := &ast.GraphQLTypeRef{}
	if l.scan.TokenText() == "[" {
		ref.IsList = true
		l.next() // consume '['
		inner, err := l.parseTypeRef()
		if err != nil {
			return nil, err
		}
		ref.InnerType = inner
		if l.scan.TokenText() != "]" {
			return nil, fmt.Errorf("expected ']', got %s", l.scan.TokenText())
		}
		l.next() // consume ']'
	} else {
		if l.tok != scanner.Ident {
			return nil, fmt.Errorf("expected type name, got %s", l.scan.TokenText())
		}
		ref.NamedType = l.scan.TokenText()
		l.next() // consume name
	}

	if l.scan.TokenText() == "!" {
		ref.IsNonNull = true
		l.next() // consume '!'
	}

	return ref, nil
}

func (l *GraphQLLexer) parseFields() ([]*ast.GraphQLFieldDefinition, error) {
	if l.scan.TokenText() != "{" {
		return nil, fmt.Errorf("expected '{'")
	}
	l.next() // consume '{'
	
	var fields []*ast.GraphQLFieldDefinition
	for l.tok != scanner.EOF && l.scan.TokenText() != "}" {
		if l.tok != scanner.Ident {
			return nil, fmt.Errorf("expected field name")
		}
		field := &ast.GraphQLFieldDefinition{Name: l.scan.TokenText()}
		l.next() // consume field name

		if l.scan.TokenText() == "(" {
			args, err := l.parseArgumentDefs()
			if err != nil {
				return nil, err
			}
			field.Arguments = args
		}

		if l.scan.TokenText() != ":" {
			return nil, fmt.Errorf("expected ':' after field name %s, got %s", field.Name, l.scan.TokenText())
		}
		l.next()

		typeRef, err := l.parseTypeRef()
		if err != nil {
			return nil, err
		}
		field.Type = typeRef
		field.Directives = l.parseDirectives()
		fields = append(fields, field)
	}

	if l.scan.TokenText() != "}" {
		return nil, fmt.Errorf("expected '}' at end of fields")
	}
	l.next() // consume '}'

	return fields, nil
}

func (l *GraphQLLexer) parseTypeDefinition() (*ast.GraphQLTypeDefinition, error) {
	l.next() // consume 'type'
	if l.tok != scanner.Ident {
		return nil, fmt.Errorf("expected type name")
	}
	def := &ast.GraphQLTypeDefinition{Name: l.scan.TokenText()}
	l.next()

	if l.scan.TokenText() == "implements" {
		l.next()
		for l.tok == scanner.Ident {
			if l.scan.TokenText() == "&" {
				l.next()
				continue
			}
			def.Implements = append(def.Implements, l.scan.TokenText())
			l.next()
			if l.scan.TokenText() == "&" {
				l.next()
			} else {
				break
			}
		}
	}

	fields, err := l.parseFields()
	if err != nil {
		return nil, err
	}
	def.Fields = fields

	return def, nil
}

func (l *GraphQLLexer) parseInterfaceDefinition() (*ast.GraphQLInterfaceDefinition, error) {
	l.next() // consume 'interface'
	if l.tok != scanner.Ident {
		return nil, fmt.Errorf("expected interface name")
	}
	def := &ast.GraphQLInterfaceDefinition{Name: l.scan.TokenText()}
	l.next() // consume interface name

	fields, err := l.parseFields()
	if err != nil {
		return nil, err
	}
	def.Fields = fields

	return def, nil
}

func (l *GraphQLLexer) parseInputDefinition() (*ast.GraphQLInputDefinition, error) {
	l.next() // consume 'input'
	if l.tok != scanner.Ident {
		return nil, fmt.Errorf("expected input name")
	}
	def := &ast.GraphQLInputDefinition{Name: l.scan.TokenText()}
	l.next() // consume input name

	fields, err := l.parseFields()
	if err != nil {
		return nil, err
	}
	def.Fields = fields

	return def, nil
}

func (l *GraphQLLexer) parseEnumDefinition() (*ast.GraphQLEnumDefinition, error) {
	l.next() // consume 'enum'
	if l.tok != scanner.Ident {
		return nil, fmt.Errorf("expected enum name")
	}
	def := &ast.GraphQLEnumDefinition{Name: l.scan.TokenText()}
	l.next() // consume enum name

	if l.scan.TokenText() != "{" {
		return nil, fmt.Errorf("expected '{' for enum")
	}
	l.next()

	for l.tok != scanner.EOF && l.scan.TokenText() != "}" {
		if l.tok != scanner.Ident {
			return nil, fmt.Errorf("expected enum value")
		}
		def.Values = append(def.Values, l.scan.TokenText())
		l.next()
	}
	l.next() // consume '}'

	return def, nil
}

func (l *GraphQLLexer) parseUnionDefinition() (*ast.GraphQLUnionDefinition, error) {
	l.next() // consume 'union'
	if l.tok != scanner.Ident {
		return nil, fmt.Errorf("expected union name")
	}
	def := &ast.GraphQLUnionDefinition{Name: l.scan.TokenText()}
	l.next() // consume name

	if l.scan.TokenText() != "=" {
		return nil, fmt.Errorf("expected '=' for union")
	}
	l.next()

	for l.tok != scanner.EOF {
		if l.tok != scanner.Ident {
			return nil, fmt.Errorf("expected union member type")
		}
		def.Types = append(def.Types, l.scan.TokenText())
		l.next()

		if l.scan.TokenText() == "|" {
			l.next()
		} else {
			break
		}
	}

	return def, nil
}

func (l *GraphQLLexer) parseOperationDefinition(opType string) (*ast.GraphQLOperation, error) {
	l.next() // consume 'query', 'mutation', or 'subscription'
	op := &ast.GraphQLOperation{OperationType: opType}
	
	if l.tok == scanner.Ident {
		op.Name = l.scan.TokenText()
		l.next()
	}

	if l.scan.TokenText() == "(" {
		for l.tok != scanner.EOF && l.scan.TokenText() != ")" {
			l.next()
		}
		l.next()
	}
	sels, err := l.parseSelectionSet()
	if err != nil {
		return nil, err
	}
	op.Selections = sels
	return op, nil
}

func (l *GraphQLLexer) parseArgumentDefs() ([]*ast.GraphQLArgumentDefinition, error) {
	l.next()
	var args []*ast.GraphQLArgumentDefinition
	for l.tok != scanner.EOF && l.scan.TokenText() != ")" {
		if l.tok != scanner.Ident {
			return nil, fmt.Errorf("expected argument name")
		}
		a := &ast.GraphQLArgumentDefinition{Name: l.scan.TokenText()}
		l.next()
		if l.scan.TokenText() != ":" {
			return nil, fmt.Errorf("expected ':' in argument")
		}
		l.next()
		tr, err := l.parseTypeRef()
		if err != nil {
			return nil, err
		}
		a.Type = tr
		if l.scan.TokenText() == "=" {
			l.next()
			a.DefaultValue = l.scan.TokenText()
			l.next()
		}
		args = append(args, a)
	}
	l.next()
	return args, nil
}

func (l *GraphQLLexer) parseDirectives() []ast.GraphQLDirective {
	var dirs []ast.GraphQLDirective
	for l.scan.TokenText() == "@" {
		l.next()
		d := ast.GraphQLDirective{Name: l.scan.TokenText(), Arguments: map[string]any{}}
		l.next()
		if l.scan.TokenText() == "(" {
			for l.tok != scanner.EOF && l.scan.TokenText() != ")" {
				l.next()
			}
			l.next()
		}
		dirs = append(dirs, d)
	}
	return dirs
}

func (l *GraphQLLexer) parseFragmentDefinition() (*ast.GraphQLFragmentDefinition, error) {
	l.next()
	if l.tok != scanner.Ident {
		return nil, fmt.Errorf("expected fragment name")
	}
	def := &ast.GraphQLFragmentDefinition{Name: l.scan.TokenText()}
	l.next()
	if l.scan.TokenText() != "on" {
		return nil, fmt.Errorf("expected 'on' in fragment")
	}
	l.next()
	def.TypeCond = l.scan.TokenText()
	l.next()
	sels, err := l.parseSelectionSet()
	if err != nil {
		return nil, err
	}
	def.Selections = sels
	return def, nil
}

func (l *GraphQLLexer) parseSchemaDefinition() (*ast.GraphQLSchemaDefinition, error) {
	l.next()
	if l.scan.TokenText() != "{" {
		return nil, fmt.Errorf("expected '{' after schema")
	}
	l.next()
	def := &ast.GraphQLSchemaDefinition{}
	for l.tok != scanner.EOF && l.scan.TokenText() != "}" {
		role := l.scan.TokenText()
		l.next()
		if l.scan.TokenText() != ":" {
			return nil, fmt.Errorf("expected ':' in schema")
		}
		l.next()
		name := l.scan.TokenText()
		l.next()
		switch role {
		case "query":
			def.Query = name
		case "mutation":
			def.Mutation = name
		case "subscription":
			def.Subscription = name
		}
	}
	l.next()
	return def, nil
}

func (l *GraphQLLexer) parseSelectionSet() ([]ast.GraphQLSelection, error) {
	var selections []ast.GraphQLSelection
	l.next()

	for l.tok != scanner.EOF && l.scan.TokenText() != "}" {
		if l.scan.TokenText() == "." {
			if err := l.consumeSpread(); err != nil {
				return nil, err
			}
			if l.scan.TokenText() == "on" {
				l.next()
				cond := l.scan.TokenText()
				l.next()
				subs, err := l.parseSelectionSet()
				if err != nil {
					return nil, err
				}
				selections = append(selections, &ast.GraphQLInlineFragment{TypeCond: cond, Selections: subs})
				continue
			}
			if l.tok != scanner.Ident {
				return nil, fmt.Errorf("expected fragment name after '...'")
			}
			selections = append(selections, &ast.GraphQLFragmentSpread{Name: l.scan.TokenText()})
			l.next()
			continue
		}
		if l.tok != scanner.Ident {
			return nil, fmt.Errorf("expected field name in selection set, got %s", l.scan.TokenText())
		}

		first := l.scan.TokenText()
		l.next()
		field := &ast.GraphQLField{Name: first}
		if l.scan.TokenText() == ":" {
			l.next()
			if l.tok != scanner.Ident {
				return nil, fmt.Errorf("expected field name after alias")
			}
			field.Alias = first
			field.Name = l.scan.TokenText()
			l.next()
		}
		if l.scan.TokenText() == "(" {
			field.Arguments = map[string]any{}
			l.next()
			for l.tok != scanner.EOF && l.scan.TokenText() != ")" {
				k := l.scan.TokenText()
				l.next()
				if l.scan.TokenText() == ":" {
					l.next()
				}
				field.Arguments[k] = l.scan.TokenText()
				l.next()
			}
			l.next()
		}
		if l.scan.TokenText() == "{" {
			subSelections, err := l.parseSelectionSet()
			if err != nil {
				return nil, err
			}
			field.Selections = subSelections
		}
		selections = append(selections, field)
	}

	if l.scan.TokenText() != "}" {
		return nil, fmt.Errorf("expected '}' at end of selection set")
	}
	l.next()
	return selections, nil
}

func (l *GraphQLLexer) consumeSpread() error {
	for i := 0; i < 3; i++ {
		if l.scan.TokenText() != "." {
			return fmt.Errorf("expected '...' fragment spread")
		}
		l.next()
	}
	return nil
}
