package bootstrap

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestNilEmitterProducesDeterministicPortableIR(t *testing.T) {
	source := "let total = 1 + 2 * 3; if (total > 5) { println(total); }"
	compile := func() map[string]any {
		return objectToGo(t, runNilPhases(t,
			[]string{"lexer.nil", "parser.nil", "analyzer.nil", "emitter.nil"},
			"nilCompile(bootstrapInput)", source,
		)).(map[string]any)
	}
	first := compile()
	second := compile()
	if first["ok"] != true || first["stage"] != "emit" {
		t.Fatalf("compile failed: %#v", first)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("same source produced different bootstrap IR")
	}
	encoded, err := json.Marshal(first)
	if err != nil {
		t.Fatal(err)
	}
	if len(encoded) == 0 {
		t.Fatal("bootstrap IR encoded to an empty artifact")
	}
}

func TestNilCompilerStopsAfterSemanticErrors(t *testing.T) {
	result := objectToGo(t, runNilPhases(t,
		[]string{"lexer.nil", "parser.nil", "analyzer.nil", "emitter.nil"},
		"nilCompile(bootstrapInput)", "println(missing);",
	)).(map[string]any)
	if result["ok"] != false || result["stage"] != "analyze" {
		t.Fatalf("unexpected failed compilation: %#v", result)
	}
}
