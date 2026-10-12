package run

import (
	"fmt"

	"github.com/joysriramsarkar/nilLang/compiler/ast"
	"github.com/joysriramsarkar/nilLang/compiler/frontend"
	"github.com/joysriramsarkar/nilLang/compiler/hir"
	"github.com/joysriramsarkar/nilLang/compiler/llvm"
	"github.com/joysriramsarkar/nilLang/compiler/mir"
)

// astProgram is the lowered program the evaluator and the VM execute.
//
// An alias rather than the concrete type so a caller reading this file does not
// have to know which pipeline stage the canonical form lives at — and so that
// moving the canonical form does not change every signature here.
type astProgram = *ast.Program

// lower runs the frontend gate and returns the lowered program all three engines
// share.
//
// # Why every engine goes through the same gate
//
// Because a conformance suite that fed the interpreter one program and the
// compiler another would be comparing two different languages. The gate is
// `frontend.Check`, which is the same path the CLI takes, so a program that runs
// here is a program a user can run.
func lower(source string) (astProgram, bool) {
	res := frontend.Check(source, frontend.Options{})
	if !res.OK {
		return nil, false
	}
	return res.Lowered, true
}

// Lower is `lower` for a caller that wants the program itself.
func Lower(source string) (astProgram, bool) { return lower(source) }

// mirOf lowers a program to MIR.
//
// It is the same two passes the MIR backend uses — HIR then MIR — so a program
// that runs ahead of time went through the same canonical IR the other engines'
// bytecode was generated from.
func mirOf(program astProgram) *mir.Program {
	optimized := hir.NewOptimizer().Optimize(hir.NewLowerer().LowerProgram(program))
	return mir.NewLowerer().LowerHIR(optimized)
}

// Emit renders a program as LLVM IR.
//
// # Why the optimiser runs on the way in
//
// Because the VM's MIR path runs it and the compiler's should not be the only
// stage where a program's meaning could change. An emitter that saw different HIR
// than the bytecode emitter would be tested against different programs than the one
// the user wrote.
func Emit(program astProgram, target llvm.Target) (string, error) {
	if program == nil {
		return "", fmt.Errorf("cannot emit IR for a program the frontend rejected")
	}
	return llvm.NewEmitter(target).Emit(mirOf(program))
}

// EmitMIR renders an already-lowered MIR program.
func EmitMIR(program *mir.Program, target llvm.Target) (string, error) {
	if program == nil {
		return "", fmt.Errorf("cannot emit IR for nil MIR")
	}
	return llvm.NewEmitter(target).Emit(program)
}
