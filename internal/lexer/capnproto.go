package lexer

import (
	"fmt"
	"strconv"
	"strings"
	"text/scanner"

	"omni-schema/internal/ast"
)

type CapnProtoLexer struct {
	scan scanner.Scanner
	tok  rune
}

func (l *CapnProtoLexer) Parse(input string) (*ast.CapnProtoFile, error) {
	l.scan.Init(strings.NewReader(input))
	l.scan.Mode = scanner.ScanIdents | scanner.ScanStrings | scanner.ScanInts | scanner.ScanFloats
	l.scan.Whitespace = 1<<'\t' | 1<<'\n' | 1<<'\r' | 1<<' '
	l.next()
	file := &ast.CapnProtoFile{}
	for l.tok != scanner.EOF {
		if l.text() == "struct" {
			s, err := l.parseStruct()
			if err != nil {
				return nil, err
			}
			file.Structs = append(file.Structs, s)
			continue
		}
		l.next()
	}
	return file, nil
}

func (l *CapnProtoLexer) next() { l.tok = l.scan.Scan() }
func (l *CapnProtoLexer) text() string { return l.scan.TokenText() }

func (l *CapnProtoLexer) parseStruct() (*ast.CapnProtoStruct, error) {
	l.next()
	if l.tok != scanner.Ident {
		return nil, fmt.Errorf("expected struct name")
	}
	s := &ast.CapnProtoStruct{Name: l.text()}
	l.next()
	if l.text() != "{" {
		return nil, fmt.Errorf("expected '{' after struct %s", s.Name)
	}
	l.next()
	for l.tok != scanner.EOF && l.text() != "}" {
		if l.tok != scanner.Ident {
			l.next()
			continue
		}
		field := &ast.CapnProtoField{Name: l.text()}
		l.next()
		if l.text() == "@" {
			l.next()
			if l.tok == scanner.Int {
				field.Id, _ = strconv.Atoi(l.text())
				l.next()
			}
		}
		if l.text() == ":" {
			l.next()
			var typ strings.Builder
			for l.tok != scanner.EOF && l.text() != ";" && l.text() != "}" {
				typ.WriteString(l.text())
				l.next()
				if l.text() == ";" {
					break
				}
			}
			field.Type = typ.String()
		}
		if l.text() == ";" {
			l.next()
		}
		s.Fields = append(s.Fields, field)
	}
	if l.text() == "}" {
		l.next()
	}
	return s, nil
}
