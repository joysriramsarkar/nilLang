package bootstrap

import "testing"

func TestNilAnalyzerAcceptsValidProgram(t *testing.T) {
	source := "let add = fn(a, b) { return a + b; }; let result = add(1, 2); println(result);"
	result := objectToGo(t, runNilPhases(t,
		[]string{"lexer.nil", "parser.nil", "analyzer.nil"},
		"nilAnalyze(nilParse(bootstrapInput))", source,
	)).(map[string]any)
	if result["ok"] != true || len(result["diagnostics"].([]any)) != 0 {
		t.Fatalf("valid program rejected: %#v", result)
	}
}

func TestNilAnalyzerReportsUndefinedAndDuplicateBindings(t *testing.T) {
	source := "let value = missing; let value = 2; other = value;"
	result := objectToGo(t, runNilPhases(t,
		[]string{"lexer.nil", "parser.nil", "analyzer.nil"},
		"nilAnalyze(nilParse(bootstrapInput))", source,
	)).(map[string]any)
	diagnostics := result["diagnostics"].([]any)
	if result["ok"] != false || len(diagnostics) != 3 {
		t.Fatalf("unexpected analysis result: %#v", result)
	}
	wantCodes := []string{"E1002", "E1001", "E1002"}
	for index, want := range wantCodes {
		if got := diagnostics[index].(map[string]any)["code"]; got != want {
			t.Errorf("diagnostic %d code=%v, want %s", index, got, want)
		}
	}
}
