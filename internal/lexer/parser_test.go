package lexer

import (
	"strings"
	"testing"
)

func TestGraphQLNestedTypeExpr(t *testing.T) {
	l := &GraphQLLexer{}
	doc, err := l.Parse("type Q { users: [[User!]!]! }")
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Definitions) != 1 {
		t.Fatalf("defs %d", len(doc.Definitions))
	}
}

func TestGraphQLAliasAndFragment(t *testing.T) {
	l := &GraphQLLexer{}
	src := `
fragment TransactionFields on Transaction { id status }
subscription A {
  latest: transactionUpdated { ...TransactionFields }
}
subscription B { ping { id } }
`
	doc, err := l.Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Definitions) < 3 {
		t.Fatalf("got %d defs", len(doc.Definitions))
	}
}

func TestGraphQLInvalid(t *testing.T) {
	l := &GraphQLLexer{}
	_, err := l.Parse("type {")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestProtoFullSubset(t *testing.T) {
	l := &ProtoLexer{}
	src := `
syntax = "proto3";
package demo;
import "google/protobuf/timestamp.proto";
option java_package = "demo";
enum Status { UNKNOWN = 0; OK = 1; }
message User {
  int32 id = 1;
  string name = 2;
  repeated string tags = 3;
  map<string, int32> scores = 4;
  oneof handle { string email = 5; string phone = 6; }
  enum Role { USER = 0; ADMIN = 1; }
  message Addr { string city = 1; }
  reserved 10;
}
service UserService {
  rpc Get(User) returns (User);
}
`
	file, err := l.Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	if file.Syntax != "proto3" || file.Package != "demo" {
		t.Fatalf("%s %s", file.Syntax, file.Package)
	}
	if len(file.Messages) != 1 || len(file.Enums) != 1 || len(file.Services) != 1 {
		t.Fatalf("messages=%d enums=%d svcs=%d", len(file.Messages), len(file.Enums), len(file.Services))
	}
	if !strings.Contains(file.Imports[0], "timestamp") {
		t.Fatal(file.Imports)
	}
}
