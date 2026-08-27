package lexer

import (
	"fmt"
	"strconv"
	"strings"
	"text/scanner"

	"omni-schema/internal/ast"
)

// ProtoLexer is a custom lexer/parser for Protobuf utilizing text/scanner.
type ProtoLexer struct {
	scan scanner.Scanner
	tok  rune
}

// Parse parses a .proto string.
func (l *ProtoLexer) Parse(input string) (*ast.ProtoFile, error) {
	l.scan.Init(strings.NewReader(input))
	l.scan.Mode = scanner.ScanIdents | scanner.ScanStrings | scanner.ScanInts | scanner.ScanFloats | scanner.ScanComments
	l.scan.Whitespace = 1<<'\t' | 1<<'\n' | 1<<'\r' | 1<<' '
	
	l.next()

	file := &ast.ProtoFile{}

	for l.tok != scanner.EOF {
		text := l.scan.TokenText()
		if text == "message" {
			msg, err := l.parseMessage()
			if err != nil {
				return nil, err
			}
			file.Messages = append(file.Messages, msg)
		} else {
			// Skip other tokens (syntax, package, etc. for now)
			l.next()
		}
	}

	return file, nil
}

func (l *ProtoLexer) next() {
	l.tok = l.scan.Scan()
}

func (l *ProtoLexer) parseMessage() (*ast.ProtoMessage, error) {
	l.next() // consume 'message'
	if l.tok != scanner.Ident {
		return nil, fmt.Errorf("expected message name")
	}
	msg := &ast.ProtoMessage{Name: l.scan.TokenText()}
	l.next() // consume name

	if l.scan.TokenText() != "{" {
		return nil, fmt.Errorf("expected '{' for message %s", msg.Name)
	}
	l.next() // consume '{'

	for l.tok != scanner.EOF && l.scan.TokenText() != "}" {
		text := l.scan.TokenText()
		
		isRepeated := false
		if text == "repeated" {
			isRepeated = true
			l.next()
			text = l.scan.TokenText()
		}

		if l.tok != scanner.Ident {
			return nil, fmt.Errorf("expected field type")
		}
		fieldType := text
		l.next()

		if l.tok != scanner.Ident {
			return nil, fmt.Errorf("expected field name")
		}
		fieldName := l.scan.TokenText()
		l.next()

		if l.scan.TokenText() != "=" {
			return nil, fmt.Errorf("expected '=' for field %s", fieldName)
		}
		l.next() // consume '='

		if l.tok != scanner.Int {
			return nil, fmt.Errorf("expected tag number for field %s", fieldName)
		}
		tagNum, _ := strconv.Atoi(l.scan.TokenText())
		l.next() // consume tag number

		if l.scan.TokenText() != ";" {
			return nil, fmt.Errorf("expected ';' at end of field %s", fieldName)
		}
		l.next() // consume ';'

		msg.Fields = append(msg.Fields, &ast.ProtoField{
			Repeated: isRepeated,
			Type:     fieldType,
			Name:     fieldName,
			Tag:      tagNum,
		})
	}

	if l.scan.TokenText() != "}" {
		return nil, fmt.Errorf("expected '}' at end of message")
	}
	l.next() // consume '}'

	return msg, nil
}
