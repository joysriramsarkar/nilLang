package tests

import (
	"github.com/joysriramsarkar/nilLang/compiler/ast"
	"github.com/joysriramsarkar/nilLang/compiler/compiler"
)

// compileWithoutLowering compiles a source program with no lowering pass.
//
// It exists so a test can check that a backend refuses a construct it cannot
// execute. That is what a bug in the lowering pass would look like from the
// backend's side: the node reaching it intact, with nothing downstream to notice.
func compileWithoutLowering(prog *ast.Program) error {
	comp := compiler.New()
	return comp.Compile(prog)
}
