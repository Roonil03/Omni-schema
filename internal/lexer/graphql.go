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
		} else if text == "query" || text == "mutation" || text == "subscription" {
			opDef, err := l.parseOperationDefinition(text)
			if err != nil {
				return nil, err
			}
			doc.Definitions = append(doc.Definitions, opDef)
		} else {
			// skip unknown for now or break
			l.next()
		}
	}

	return doc, nil
}

func (l *GraphQLLexer) next() {
	l.tok = l.scan.Scan()
}

func (l *GraphQLLexer) parseTypeDefinition() (*ast.GraphQLTypeDefinition, error) {
	l.next() // consume 'type'
	if l.tok != scanner.Ident {
		return nil, fmt.Errorf("expected type name")
	}
	def := &ast.GraphQLTypeDefinition{Name: l.scan.TokenText()}
	l.next() // consume type name

	if l.scan.TokenText() != "{" {
		return nil, fmt.Errorf("expected '{' after type name %s", def.Name)
	}
	l.next() // consume '{'

	for l.tok != scanner.EOF && l.scan.TokenText() != "}" {
		if l.tok != scanner.Ident {
			return nil, fmt.Errorf("expected field name")
		}
		field := &ast.GraphQLFieldDefinition{Name: l.scan.TokenText()}
		l.next() // consume field name

		if l.scan.TokenText() != ":" {
			return nil, fmt.Errorf("expected ':' after field name %s", field.Name)
		}
		l.next() // consume ':'

		// Parse type
		if l.scan.TokenText() == "[" {
			field.IsList = true
			l.next()
		}

		if l.tok != scanner.Ident {
			return nil, fmt.Errorf("expected field type")
		}
		field.Type = l.scan.TokenText()
		l.next() // consume type

		if l.scan.TokenText() == "!" {
			// Inner non-null, skip for now
			l.next()
		}

		if field.IsList {
			if l.scan.TokenText() != "]" {
				return nil, fmt.Errorf("expected ']' after list type")
			}
			l.next()
		}

		if l.scan.TokenText() == "!" {
			field.NonNull = true
			l.next()
		}

		def.Fields = append(def.Fields, field)
	}

	if l.scan.TokenText() != "}" {
		return nil, fmt.Errorf("expected '}' at end of type")
	}
	l.next() // consume '}'

	return def, nil
}

func (l *GraphQLLexer) parseOperationDefinition(opType string) (*ast.GraphQLOperation, error) {
	l.next() // consume 'query', 'mutation', or 'subscription'
	op := &ast.GraphQLOperation{OperationType: opType}
	
	if l.tok == scanner.Ident {
		op.Name = l.scan.TokenText()
		l.next()
	}

	selections, err := l.parseSelectionSet()
	if err != nil {
		return nil, err
	}
	op.Selections = selections

	return op, nil
}

func (l *GraphQLLexer) parseSelectionSet() ([]ast.GraphQLSelection, error) {
	var selections []ast.GraphQLSelection
	l.next() // consume '{'

	for l.tok != scanner.EOF && l.scan.TokenText() != "}" {
		if l.tok != scanner.Ident {
			return nil, fmt.Errorf("expected field name in selection set, got %s", l.scan.TokenText())
		}
		
		field := &ast.GraphQLField{Name: l.scan.TokenText()}
		l.next() // consume field name

		// Check for sub-selections
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
	l.next() // consume '}'

	return selections, nil
}

