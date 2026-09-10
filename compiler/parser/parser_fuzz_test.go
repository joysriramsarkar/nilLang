package parser

import (
	"testing"

	"github.com/joysriramsarkar/nilLang/compiler/lexer"
)

func FuzzParser(f *testing.F) {
	f.Add("let x = 10;")
	f.Add("fn foo(a, b) { return a + b; }")
	f.Add("if (x > 5) { 1 } else { 2 };")
	f.Add("while (x < 10) { x += 1; }")
	f.Add("let arr = [1, 2, 3];")
	f.Add("let h = {\"a\": 1};")
	f.Add("component C { state x = 0; }")
	f.Add("let s = \"hello \\(world)\";")
	f.Add("x = 10;")
	f.Add("const PI = 3.14;")
	f.Add("!")
	f.Add("(((((")
	f.Add("fn { return }")

	f.Fuzz(func(t *testing.T, input string) {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("Parser panicked on input %q: %v", input, r)
			}
		}()

		l := lexer.New(input)
		p := New(l)
		_ = p.ParseProgram()
	})
}
