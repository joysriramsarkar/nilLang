package bootstrap

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/joysriramsarkar/nilLang/compiler/evaluator"
	"github.com/joysriramsarkar/nilLang/compiler/lexer"
	"github.com/joysriramsarkar/nilLang/compiler/object"
	"github.com/joysriramsarkar/nilLang/compiler/parser"
)

func TestNilLexerMatchesBootstrapLexer(t *testing.T) {
	fixture := "let answer = 42;\n// comment\nanswer += 3.5;\nlet label = \"nil\\nlang\";\nif (answer >= 45 && answer != 0) { answer = answer - 1; } /* done */"
	want := lexWithGo(fixture)
	got := lexWithNil(t, fixture)

	if len(got.Elements) != len(want) {
		t.Fatalf("token count differs: Nil=%d Go=%d", len(got.Elements), len(want))
	}
	for index, expected := range want {
		actual, ok := got.Elements[index].(*object.Hash)
		if !ok {
			t.Fatalf("token %d is %T, want HASH", index, got.Elements[index])
		}
		assertHashField(t, actual, "type", string(expected.Type), index)
		assertHashField(t, actual, "literal", expected.Literal, index)
		assertHashField(t, actual, "line", expected.Line, index)
		assertHashField(t, actual, "column", expected.Column, index)
	}
}

func lexWithGo(source string) []struct {
	Type    string
	Literal string
	Line    int
	Column  int
} {
	goLexer := lexer.New(source)
	var tokens []struct {
		Type    string
		Literal string
		Line    int
		Column  int
	}
	for {
		token := goLexer.NextToken()
		tokens = append(tokens, struct {
			Type    string
			Literal string
			Line    int
			Column  int
		}{string(token.Type), token.Literal, token.Line, token.Column})
		if token.Type == "EOF" {
			return tokens
		}
	}
}

func lexWithNil(t *testing.T, source string) *object.Array {
	t.Helper()
	_, currentFile, _, _ := runtime.Caller(0)
	bootstrapSource, err := os.ReadFile(filepath.Join(filepath.Dir(currentFile), "lexer.nil"))
	if err != nil {
		t.Fatal(err)
	}

	parsed := parser.New(lexer.New(string(bootstrapSource) + "\nnilLex(bootstrapInput);"))
	program := parsed.ParseProgram()
	if len(parsed.Errors()) > 0 {
		t.Fatalf("parse bootstrap lexer: %v", parsed.Errors())
	}

	environment := object.NewEnvironment()
	for name, builtin := range evaluator.Builtins {
		environment.Set(name, builtin)
	}
	environment.Set("bootstrapInput", &object.String{Value: source})
	result := evaluator.Eval(program, environment)
	if result != nil && result.Type() == object.ERROR_OBJ {
		t.Fatalf("run bootstrap lexer: %s", result.Inspect())
	}
	array, ok := result.(*object.Array)
	if !ok {
		t.Fatalf("Nil lexer returned %T (%v), want ARRAY", result, result)
	}
	return array
}

func assertHashField(t *testing.T, hash *object.Hash, key string, expected any, tokenIndex int) {
	t.Helper()
	pair, ok := hash.Pairs[(&object.String{Value: key}).HashKey()]
	if !ok {
		t.Fatalf("token %d has no %q field", tokenIndex, key)
	}
	switch value := pair.Value.(type) {
	case *object.String:
		if value.Value != expected {
			t.Errorf("token %d %s=%q, want %q", tokenIndex, key, value.Value, expected)
		}
	case *object.Integer:
		if value.Value != int64(expected.(int)) {
			t.Errorf("token %d %s=%d, want %d", tokenIndex, key, value.Value, expected)
		}
	default:
		t.Fatalf("token %d %s has unsupported value %T", tokenIndex, key, pair.Value)
	}
}