package lexer

import (
	"strings"
	"text/scanner"
	"omni-schema/internal/ast"
)

// CapnProtoLexer is a custom lexer/parser for .capnp schemas utilizing text/scanner.
type CapnProtoLexer struct {
	scan scanner.Scanner
}

// Parse extracts Cap'n Proto structures into an AST without dependencies.
func (l *CapnProtoLexer) Parse(input string) (*ast.CapnProtoFile, error) {
	l.scan.Init(strings.NewReader(input))
	l.scan.Mode = scanner.ScanIdents | scanner.ScanStrings | scanner.ScanInts | scanner.ScanFloats
	
	file := &ast.CapnProtoFile{}
	for tok := l.scan.Scan(); tok != scanner.EOF; tok = l.scan.Scan() {
		text := l.scan.TokenText()
		if text == "struct" {
			s := &ast.CapnProtoStruct{}
			file.Structs = append(file.Structs, s)
		}
	}
	return file, nil
}
