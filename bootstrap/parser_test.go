package bootstrap

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"testing"

	"github.com/joysriramsarkar/nilLang/compiler/ast"
	"github.com/joysriramsarkar/nilLang/compiler/evaluator"
	"github.com/joysriramsarkar/nilLang/compiler/lexer"
	"github.com/joysriramsarkar/nilLang/compiler/object"
	"github.com/joysriramsarkar/nilLang/compiler/parser"
)

func TestNilParserMatchesBootstrapParser(t *testing.T) {
	source := "let result = -a + 2 * (3 + 4); return result >= 10 && true != false;"
	want := normalizeProgram(t, parseWithGo(t, source))
	got := objectToGo(t, runNilBootstrap(t, "parser.nil", "nilParse", source))

	gotProgram := got.(map[string]any)
	if errors := gotProgram["errors"].([]any); len(errors) != 0 {
		t.Fatalf("Nil parser errors: %v", errors)
	}
	delete(gotProgram, "errors")
	if !reflect.DeepEqual(gotProgram, want) {
		t.Fatalf("normalized AST differs\nNil: %#v\nGo:  %#v", gotProgram, want)
	}
}

func TestNilParserReportsStructuredErrors(t *testing.T) {
	got := objectToGo(t, runNilBootstrap(t, "parser.nil", "nilParse", "let = 1;"))
	program := got.(map[string]any)
	errors := program["errors"].([]any)
	if len(errors) == 0 {
		t.Fatal("Nil parser accepted malformed let statement")
	}
	first := errors[0].(map[string]any)
	if first["message"] != "expected IDENT, got =" || first["line"] != int64(1) || first["column"] != int64(5) {
		t.Fatalf("unexpected diagnostic: %#v", first)
	}
}

func TestNilParserProceduralCoreParity(t *testing.T) {
	source := `
let transform = fn named(value, scale) {
    let items = [value, scale * 2];
    let record = {"items": items, "ok": true};
    record["items"] = items;
    while (value < 3) { value = value + 1; }
    return record.items[0];
};
if (transform(1, 2) == 1) { "yes"; } else { "no"; };
`
	want := normalizeProgram(t, parseWithGo(t, source))
	got := objectToGo(t, runNilBootstrap(t, "parser.nil", "nilParse", source)).(map[string]any)
	if errors := got["errors"].([]any); len(errors) != 0 {
		t.Fatalf("Nil parser errors: %v", errors)
	}
	delete(got, "errors")
	sortHashPairs(got)
	sortHashPairs(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("procedural AST differs: %s", firstDifference("Program", got, want))
	}
}

func sortHashPairs(val any) {
	switch v := val.(type) {
	case map[string]any:
		if v["kind"] == "HashLiteral" {
			if pairs, ok := v["pairs"].([]any); ok {
				sort.Slice(pairs, func(i, j int) bool {
					pi, _ := pairs[i].(map[string]any)
					pj, _ := pairs[j].(map[string]any)
					return fmt.Sprint(pi["key"]) < fmt.Sprint(pj["key"])
				})
			}
		}
		for _, child := range v {
			sortHashPairs(child)
		}
	case []any:
		for _, elem := range v {
			sortHashPairs(elem)
		}
	}
}

func firstDifference(path string, got, want any) string {
	if reflect.DeepEqual(got, want) {
		return ""
	}
	gotMap, gotIsMap := got.(map[string]any)
	wantMap, wantIsMap := want.(map[string]any)
	if gotIsMap && wantIsMap {
		for key, wantValue := range wantMap {
			gotValue, ok := gotMap[key]
			if !ok {
				return fmt.Sprintf("%s.%s missing", path, key)
			}
			if difference := firstDifference(path+"."+key, gotValue, wantValue); difference != "" {
				return difference
			}
		}
		for key := range gotMap {
			if _, ok := wantMap[key]; !ok {
				return fmt.Sprintf("%s.%s unexpected", path, key)
			}
		}
	}
	gotSlice, gotIsSlice := got.([]any)
	wantSlice, wantIsSlice := want.([]any)
	if gotIsSlice && wantIsSlice {
		if len(gotSlice) != len(wantSlice) {
			return fmt.Sprintf("%s length=%d, want %d", path, len(gotSlice), len(wantSlice))
		}
		for index := range wantSlice {
			if difference := firstDifference(fmt.Sprintf("%s[%d]", path, index), gotSlice[index], wantSlice[index]); difference != "" {
				return difference
			}
		}
	}
	return fmt.Sprintf("%s=%#v, want %#v", path, got, want)
}

func TestNilParserParsesBootstrapSources(t *testing.T) {
	_, currentFile, _, _ := runtime.Caller(0)
	directory := filepath.Dir(currentFile)
	for _, fileName := range []string{"lexer.nil", "parser.nil"} {
		source, err := os.ReadFile(filepath.Join(directory, fileName))
		if err != nil {
			t.Fatal(err)
		}
		got := objectToGo(t, runNilBootstrap(t, "parser.nil", "nilParse", string(source))).(map[string]any)
		if errors := got["errors"].([]any); len(errors) != 0 {
			t.Errorf("self-parse %s: %v", fileName, errors)
		}
	}
}

func runNilBootstrap(t *testing.T, fileName, functionName, source string) object.Object {
	t.Helper()
	_, currentFile, _, _ := runtime.Caller(0)
	directory := filepath.Dir(currentFile)
	lexerSource, err := os.ReadFile(filepath.Join(directory, "lexer.nil"))
	if err != nil {
		t.Fatal(err)
	}
	phaseSource, err := os.ReadFile(filepath.Join(directory, fileName))
	if err != nil {
		t.Fatal(err)
	}

	input := string(lexerSource) + "\n" + string(phaseSource) + "\n" + functionName + "(bootstrapInput);"
	parsed := parser.New(lexer.New(input))
	program := parsed.ParseProgram()
	if len(parsed.Errors()) > 0 {
		t.Fatalf("parse %s: %v", fileName, parsed.Errors())
	}

	environment := object.NewEnvironment()
	for name, builtin := range evaluator.Builtins {
		environment.Set(name, builtin)
	}
	environment.Set("bootstrapInput", &object.String{Value: source})
	result := evaluator.Eval(program, environment)
	if result != nil && result.Type() == object.ERROR_OBJ {
		t.Fatalf("run %s: %s", fileName, result.Inspect())
	}
	return result
}

func parseWithGo(t *testing.T, source string) *ast.Program {
	t.Helper()
	goParser := parser.New(lexer.New(source))
	program := goParser.ParseProgram()
	if len(goParser.Errors()) > 0 {
		t.Fatalf("Go parser errors: %v", goParser.Errors())
	}
	return program
}

func normalizeProgram(t *testing.T, program *ast.Program) map[string]any {
	t.Helper()
	statements := make([]any, len(program.Statements))
	for index, statement := range program.Statements {
		statements[index] = normalizeNode(t, statement)
	}
	return map[string]any{"kind": "Program", "statements": statements}
}

func normalizeNode(t *testing.T, node ast.Node) map[string]any {
	t.Helper()
	base := func(kind string, tokenLiteral string, line, column int) map[string]any {
		return map[string]any{"kind": kind, "literal": tokenLiteral, "line": int64(line), "column": int64(column)}
	}
	switch node := node.(type) {
	case *ast.LetStatement:
		result := base("LetStatement", node.Token.Literal, node.Token.Line, node.Token.Column)
		result["name"] = normalizeNode(t, node.Name)
		result["value"] = normalizeNode(t, node.Value)
		return result
	case *ast.ReturnStatement:
		result := base("ReturnStatement", node.Token.Literal, node.Token.Line, node.Token.Column)
		result["value"] = normalizeNode(t, node.ReturnValue)
		return result
	case *ast.AssignStatement:
		result := base("AssignStatement", node.Token.Literal, node.Token.Line, node.Token.Column)
		result["name"] = normalizeNode(t, node.Name)
		result["value"] = normalizeNode(t, node.Value)
		return result
	case *ast.IndexAssignStatement:
		result := map[string]any{"kind": "IndexAssignStatement", "literal": "="}
		result["left"] = normalizeNode(t, node.Left)
		result["index"] = normalizeNode(t, node.Index)
		result["value"] = normalizeNode(t, node.Value)
		return result
	case *ast.WhileStatement:
		result := base("WhileStatement", node.Token.Literal, node.Token.Line, node.Token.Column)
		result["condition"] = normalizeNode(t, node.Condition)
		result["body"] = normalizeNode(t, node.Body)
		return result
	case *ast.BlockStatement:
		result := base("BlockStatement", node.Token.Literal, node.Token.Line, node.Token.Column)
		statements := make([]any, len(node.Statements))
		for index, statement := range node.Statements {
			statements[index] = normalizeNode(t, statement)
		}
		result["statements"] = statements
		return result
	case *ast.ExpressionStatement:
		result := base("ExpressionStatement", node.Token.Literal, node.Token.Line, node.Token.Column)
		result["expression"] = normalizeNode(t, node.Expression)
		return result
	case *ast.Identifier:
		result := base("Identifier", node.Token.Literal, node.Token.Line, node.Token.Column)
		result["value"] = node.Value
		return result
	case *ast.IntegerLiteral:
		return base("IntegerLiteral", node.Token.Literal, node.Token.Line, node.Token.Column)
	case *ast.FloatLiteral:
		return base("FloatLiteral", node.Token.Literal, node.Token.Line, node.Token.Column)
	case *ast.StringLiteral:
		result := base("StringLiteral", node.Token.Literal, node.Token.Line, node.Token.Column)
		result["value"] = node.Value
		return result
	case *ast.Boolean:
		result := base("Boolean", node.Token.Literal, node.Token.Line, node.Token.Column)
		result["value"] = node.Value
		return result
	case *ast.NullLiteral:
		return base("NullLiteral", node.Token.Literal, node.Token.Line, node.Token.Column)
	case *ast.PrefixExpression:
		result := base("PrefixExpression", node.Token.Literal, node.Token.Line, node.Token.Column)
		result["operator"] = node.Operator
		result["right"] = normalizeNode(t, node.Right)
		return result
	case *ast.InfixExpression:
		result := base("InfixExpression", node.Token.Literal, node.Token.Line, node.Token.Column)
		result["operator"] = node.Operator
		result["left"] = normalizeNode(t, node.Left)
		result["right"] = normalizeNode(t, node.Right)
		return result
	case *ast.IfExpression:
		result := base("IfExpression", node.Token.Literal, node.Token.Line, node.Token.Column)
		result["condition"] = normalizeNode(t, node.Condition)
		result["consequence"] = normalizeNode(t, node.Consequence)
		if node.Alternative == nil {
			result["alternative"] = nil
		} else {
			result["alternative"] = normalizeNode(t, node.Alternative)
		}
		return result
	case *ast.FunctionLiteral:
		result := base("FunctionLiteral", node.Token.Literal, node.Token.Line, node.Token.Column)
		parameters := make([]any, len(node.Parameters))
		for index, parameter := range node.Parameters {
			parameters[index] = normalizeNode(t, parameter)
		}
		result["parameters"] = parameters
		result["body"] = normalizeNode(t, node.Body)
		result["name"] = node.Name
		return result
	case *ast.CallExpression:
		result := base("CallExpression", node.Token.Literal, node.Token.Line, node.Token.Column)
		result["function"] = normalizeNode(t, node.Function)
		arguments := make([]any, len(node.Arguments))
		for index, argument := range node.Arguments {
			arguments[index] = normalizeNode(t, argument)
		}
		result["arguments"] = arguments
		return result
	case *ast.ArrayLiteral:
		result := base("ArrayLiteral", node.Token.Literal, node.Token.Line, node.Token.Column)
		elements := make([]any, len(node.Elements))
		for index, element := range node.Elements {
			elements[index] = normalizeNode(t, element)
		}
		result["elements"] = elements
		return result
	case *ast.HashLiteral:
		result := base("HashLiteral", node.Token.Literal, node.Token.Line, node.Token.Column)
		pairs := make([]any, 0, len(node.Pairs))
		for key, value := range node.Pairs {
			pairs = append(pairs, map[string]any{"key": normalizeNode(t, key), "value": normalizeNode(t, value)})
		}
		result["pairs"] = pairs
		return result
	case *ast.IndexExpression:
		result := base("IndexExpression", node.Token.Literal, node.Token.Line, node.Token.Column)
		result["left"] = normalizeNode(t, node.Left)
		result["index"] = normalizeNode(t, node.Index)
		return result
	case *ast.DotExpression:
		result := base("DotExpression", node.Token.Literal, node.Token.Line, node.Token.Column)
		result["left"] = normalizeNode(t, node.Left)
		result["member"] = normalizeNode(t, node.Member)
		return result
	default:
		t.Fatalf("unsupported Go AST node %T", node)
		return nil
	}
}

func objectToGo(t *testing.T, value object.Object) any {
	t.Helper()
	switch value := value.(type) {
	case *object.String:
		return value.Value
	case *object.Integer:
		return value.Value
	case *object.Boolean:
		return value.Value
	case *object.Null:
		return nil
	case *object.Array:
		result := make([]any, len(value.Elements))
		for index, element := range value.Elements {
			result[index] = objectToGo(t, element)
		}
		return result
	case *object.Hash:
		result := make(map[string]any, len(value.Pairs))
		for _, pair := range value.Pairs {
			key, ok := pair.Key.(*object.String)
			if !ok {
				t.Fatalf("non-string AST key %T", pair.Key)
			}
			result[key.Value] = objectToGo(t, pair.Value)
		}
		return result
	default:
		t.Fatalf("unsupported bootstrap object %T (%s)", value, fmt.Sprint(value))
		return nil
	}
}

func runNilPhases(t *testing.T, fileNames []string, expression string, source string) object.Object {
	t.Helper()
	_, currentFile, _, _ := runtime.Caller(0)
	directory := filepath.Dir(currentFile)
	var input string
	for _, fileName := range fileNames {
		phaseSource, err := os.ReadFile(filepath.Join(directory, fileName))
		if err != nil {
			t.Fatal(err)
		}
		input += string(phaseSource) + "\n"
	}
	input += expression + ";"
	parsed := parser.New(lexer.New(input))
	program := parsed.ParseProgram()
	if len(parsed.Errors()) > 0 {
		t.Fatalf("parse bootstrap phases: %v", parsed.Errors())
	}
	environment := object.NewEnvironment()
	for name, builtin := range evaluator.Builtins {
		environment.Set(name, builtin)
	}
	environment.Set("bootstrapInput", &object.String{Value: source})
	result := evaluator.Eval(program, environment)
	if result != nil && result.Type() == object.ERROR_OBJ {
		t.Fatalf("run bootstrap phases: %s", result.Inspect())
	}
	return result
}