package lexer

import (
	"fmt"
	"strconv"
	"strings"
	"text/scanner"

	"omni-schema/internal/ast"
)

type ProtoLexer struct {
	scan scanner.Scanner
	tok  rune
}

func (l *ProtoLexer) Parse(input string) (*ast.ProtoFile, error) {
	l.scan.Init(strings.NewReader(input))
	l.scan.Mode = scanner.ScanIdents | scanner.ScanStrings | scanner.ScanInts | scanner.ScanFloats | scanner.ScanComments
	l.scan.Whitespace = 1<<'\t' | 1<<'\n' | 1<<'\r' | 1<<' '
	l.next()

	file := &ast.ProtoFile{Options: map[string]string{}}
	for l.tok != scanner.EOF {
		text := l.text()
		switch text {
		case "syntax":
			syn, err := l.parseSyntax()
			if err != nil {
				return nil, err
			}
			file.Syntax = syn
		case "package":
			pkg, err := l.parsePackage()
			if err != nil {
				return nil, err
			}
			file.Package = pkg
		case "import":
			imp, err := l.parseImport()
			if err != nil {
				return nil, err
			}
			file.Imports = append(file.Imports, imp)
		case "option":
			k, v, err := l.parseOption()
			if err != nil {
				return nil, err
			}
			file.Options[k] = v
		case "message":
			msg, err := l.parseMessage()
			if err != nil {
				return nil, err
			}
			file.Messages = append(file.Messages, msg)
		case "enum":
			en, err := l.parseEnum()
			if err != nil {
				return nil, err
			}
			file.Enums = append(file.Enums, en)
		case "service":
			svc, err := l.parseService()
			if err != nil {
				return nil, err
			}
			file.Services = append(file.Services, svc)
		default:
			l.next()
		}
	}
	return file, nil
}

func (l *ProtoLexer) next() { l.tok = l.scan.Scan() }
func (l *ProtoLexer) text() string { return l.scan.TokenText() }

func (l *ProtoLexer) expect(s string) error {
	if l.text() != s {
		return fmt.Errorf("expected %q, got %q", s, l.text())
	}
	l.next()
	return nil
}

func (l *ProtoLexer) parseSyntax() (string, error) {
	l.next()
	if err := l.expect("="); err != nil {
		return "", err
	}
	s := strings.Trim(l.text(), `"`)
	l.next()
	_ = l.expect(";")
	return s, nil
}

func (l *ProtoLexer) parsePackage() (string, error) {
	l.next()
	var b strings.Builder
	for l.tok != scanner.EOF && l.text() != ";" {
		b.WriteString(l.text())
		l.next()
	}
	_ = l.expect(";")
	return b.String(), nil
}

func (l *ProtoLexer) parseImport() (string, error) {
	l.next()
	if l.text() == "public" || l.text() == "weak" {
		l.next()
	}
	s := strings.Trim(l.text(), `"`)
	l.next()
	_ = l.expect(";")
	return s, nil
}

func (l *ProtoLexer) parseOption() (string, string, error) {
	l.next()
	name := l.text()
	l.next()
	if l.text() == "." {
		l.next()
		name += "." + l.text()
		l.next()
	}
	if err := l.expect("="); err != nil {
		return "", "", err
	}
	val := strings.Trim(l.text(), `"`)
	l.next()
	_ = l.expect(";")
	return name, val, nil
}

func (l *ProtoLexer) parseMessage() (*ast.ProtoMessage, error) {
	l.next()
	if l.tok != scanner.Ident {
		return nil, fmt.Errorf("expected message name")
	}
	msg := &ast.ProtoMessage{Name: l.text()}
	l.next()
	if err := l.expect("{"); err != nil {
		return nil, err
	}
	for l.tok != scanner.EOF && l.text() != "}" {
		switch l.text() {
		case "message":
			nested, err := l.parseMessage()
			if err != nil {
				return nil, err
			}
			msg.Nested = append(msg.Nested, nested)
		case "enum":
			en, err := l.parseEnum()
			if err != nil {
				return nil, err
			}
			msg.Enums = append(msg.Enums, en)
		case "oneof":
			oo, err := l.parseOneof()
			if err != nil {
				return nil, err
			}
			msg.Oneofs = append(msg.Oneofs, oo)
		case "reserved":
			l.skipToSemi()
		case "option":
			_, _, err := l.parseOption()
			if err != nil {
				return nil, err
			}
		case "map":
			mf, err := l.parseMapField()
			if err != nil {
				return nil, err
			}
			msg.Maps = append(msg.Maps, mf)
		default:
			field, err := l.parseField()
			if err != nil {
				return nil, err
			}
			msg.Fields = append(msg.Fields, field)
		}
	}
	if err := l.expect("}"); err != nil {
		return nil, err
	}
	return msg, nil
}

func (l *ProtoLexer) parseField() (*ast.ProtoField, error) {
	f := &ast.ProtoField{Options: map[string]string{}}
	for l.text() == "repeated" || l.text() == "optional" || l.text() == "required" {
		switch l.text() {
		case "repeated":
			f.Repeated = true
		case "optional":
			f.Optional = true
		case "required":
			f.Required = true
		}
		l.next()
	}
	if l.tok != scanner.Ident {
		return nil, fmt.Errorf("expected field type")
	}
	f.Type = l.text()
	l.next()
	if l.tok != scanner.Ident {
		return nil, fmt.Errorf("expected field name")
	}
	f.Name = l.text()
	l.next()
	if err := l.expect("="); err != nil {
		return nil, err
	}
	if l.tok != scanner.Int {
		return nil, fmt.Errorf("expected tag number for field %s", f.Name)
	}
	f.Tag, _ = strconv.Atoi(l.text())
	l.next()
	if l.text() == "[" {
		l.skipBalanced('[', ']')
	}
	if err := l.expect(";"); err != nil {
		return nil, err
	}
	return f, nil
}

func (l *ProtoLexer) parseMapField() (*ast.ProtoMapField, error) {
	l.next()
	if err := l.expect("<"); err != nil {
		return nil, err
	}
	kt := l.text()
	l.next()
	if err := l.expect(","); err != nil {
		return nil, err
	}
	vt := l.text()
	l.next()
	if err := l.expect(">"); err != nil {
		return nil, err
	}
	name := l.text()
	l.next()
	if err := l.expect("="); err != nil {
		return nil, err
	}
	tag, _ := strconv.Atoi(l.text())
	l.next()
	_ = l.expect(";")
	return &ast.ProtoMapField{KeyType: kt, ValueType: vt, Name: name, Tag: tag}, nil
}

func (l *ProtoLexer) parseOneof() (*ast.ProtoOneof, error) {
	l.next()
	oo := &ast.ProtoOneof{Name: l.text()}
	l.next()
	if err := l.expect("{"); err != nil {
		return nil, err
	}
	for l.tok != scanner.EOF && l.text() != "}" {
		f, err := l.parseField()
		if err != nil {
			return nil, err
		}
		oo.Fields = append(oo.Fields, f)
	}
	_ = l.expect("}")
	return oo, nil
}

func (l *ProtoLexer) parseEnum() (*ast.ProtoEnum, error) {
	l.next()
	en := &ast.ProtoEnum{Name: l.text()}
	l.next()
	if err := l.expect("{"); err != nil {
		return nil, err
	}
	for l.tok != scanner.EOF && l.text() != "}" {
		if l.text() == "option" {
			_, _, _ = l.parseOption()
			continue
		}
		if l.text() == "reserved" {
			l.skipToSemi()
			continue
		}
		if l.tok != scanner.Ident {
			l.next()
			continue
		}
		name := l.text()
		l.next()
		if err := l.expect("="); err != nil {
			return nil, err
		}
		num, _ := strconv.Atoi(l.text())
		l.next()
		if l.text() == "[" {
			l.skipBalanced('[', ']')
		}
		_ = l.expect(";")
		en.Values = append(en.Values, &ast.ProtoEnumValue{Name: name, Number: num})
	}
	_ = l.expect("}")
	return en, nil
}

func (l *ProtoLexer) parseService() (*ast.ProtoService, error) {
	l.next()
	svc := &ast.ProtoService{Name: l.text()}
	l.next()
	if err := l.expect("{"); err != nil {
		return nil, err
	}
	for l.tok != scanner.EOF && l.text() != "}" {
		if l.text() != "rpc" {
			l.next()
			continue
		}
		rpc, err := l.parseRPC()
		if err != nil {
			return nil, err
		}
		svc.RPCs = append(svc.RPCs, rpc)
	}
	_ = l.expect("}")
	return svc, nil
}

func (l *ProtoLexer) parseRPC() (*ast.ProtoRPC, error) {
	l.next()
	rpc := &ast.ProtoRPC{Name: l.text(), Options: map[string]string{}}
	l.next()
	if err := l.expect("("); err != nil {
		return nil, err
	}
	if l.text() == "stream" {
		l.next()
	}
	rpc.Request = l.text()
	l.next()
	if err := l.expect(")"); err != nil {
		return nil, err
	}
	if l.text() != "returns" {
		return nil, fmt.Errorf("expected returns")
	}
	l.next()
	if err := l.expect("("); err != nil {
		return nil, err
	}
	if l.text() == "stream" {
		l.next()
	}
	rpc.Response = l.text()
	l.next()
	_ = l.expect(")")
	if l.text() == "{" {
		l.skipBalanced('{', '}')
	} else {
		_ = l.expect(";")
	}
	return rpc, nil
}

func (l *ProtoLexer) skipToSemi() {
	for l.tok != scanner.EOF && l.text() != ";" {
		l.next()
	}
	if l.text() == ";" {
		l.next()
	}
}

func (l *ProtoLexer) skipBalanced(open, close rune) {
	if l.text() != string(open) {
		return
	}
	depth := 1
	l.next()
	for l.tok != scanner.EOF && depth > 0 {
		if l.text() == string(open) {
			depth++
		} else if l.text() == string(close) {
			depth--
		}
		if depth > 0 {
			l.next()
		}
	}
	if l.text() == string(close) {
		l.next()
	}
}
