package lexer

import (
	"strings"
	"text/scanner"

	"omni-schema/internal/ast"
)

// ProtoLexer is a custom lexer/parser for Protobuf utilizing text/scanner.
type ProtoLexer struct {
	scan scanner.Scanner
}

// Parse parses a .proto string.
func (l *ProtoLexer) Parse(input string) (*ast.ProtoFile, error) {
	l.scan.Init(strings.NewReader(input))
	l.scan.Mode = scanner.ScanIdents | scanner.ScanStrings | scanner.ScanInts | scanner.ScanFloats | scanner.ScanComments
	
	file := &ast.ProtoFile{}
	
	for tok := l.scan.Scan(); tok != scanner.EOF; tok = l.scan.Scan() {
		text := l.scan.TokenText()
		if text == "message" {
			msg := &ast.ProtoMessage{}
			file.Messages = append(file.Messages, msg)
		}
	}
	
	return file, nil
}
