package typecheck

import (
	"testing"

	"github.com/joysriramsarkar/nilLang/compiler/lexer"
	"github.com/joysriramsarkar/nilLang/compiler/parser"
)

func FuzzTypeChecker(f *testing.F) {
	f.Add("let x = 10;")
	f.Add("let x: Int = 10;")
	f.Add("const MAX = 100;")
	f.Add("fn add(a: Int, b: Int) -> Int { return a + b; }")
	f.Add("let items = [1, 2, 3];")
	f.Add("let h = {\"key\": \"value\"};")
	f.Add("x = 20;")

	f.Fuzz(func(t *testing.T, input string) {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("TypeChecker panicked on input %q: %v", input, r)
			}
		}()

		l := lexer.New(input)
		p := parser.New(l)
		prog := p.ParseProgram()
		if len(p.Errors()) == 0 && prog != nil {
			checker := NewChecker()
			_ = checker.CheckProgram(prog)
		}
	})
}
