package cheader

import (
	"strings"
	"testing"

	"github.com/joysriramsarkar/nilLang/compiler/frontend"
	"github.com/joysriramsarkar/nilLang/compiler/hir"
	"github.com/joysriramsarkar/nilLang/compiler/llvm"
	"github.com/joysriramsarkar/nilLang/compiler/mir"
)

// lowerFor is the front end on its own, for the cases where generating the header
// is the thing being tested and must not be allowed to abort the test.
func lowerFor(t *testing.T, source string) *mir.Program {
	t.Helper()
	checked := frontend.Check(source, frontend.Options{})
	if !checked.OK {
		t.Fatalf("the program was rejected: %s", checked.ErrSummary())
	}
	optimized := hir.NewOptimizer().Optimize(hir.NewLowerer().LowerProgram(checked.Lowered))
	return mir.NewLowerer().LowerHIR(optimized)
}

// generateFor is the whole pipeline a header has to agree with.
//
// Through the front end rather than built from source text, because a header
// written from the text would be a second parser and the two would disagree about
// something eventually — a type alias, a lowered construct, a name the checker
// rewrote.
func generateFor(t *testing.T, source, project string) (header string, program *mir.Program) {
	t.Helper()

	checked := frontend.Check(source, frontend.Options{})
	if !checked.OK {
		t.Fatalf("the program was rejected: %s", checked.ErrSummary())
	}
	optimized := hir.NewOptimizer().Optimize(hir.NewLowerer().LowerProgram(checked.Lowered))
	program = mir.NewLowerer().LowerHIR(optimized)

	header, err := Generate(program, project)
	if err != nil {
		t.Fatalf("generating the header failed: %v", err)
	}
	return header, program
}

// emitFor is the module the same program produces.
func emitFor(t *testing.T, program *mir.Program) string {
	t.Helper()
	module, err := llvm.NewEmitter(llvm.TargetHost).Emit(program)
	if err != nil {
		t.Fatalf("emitting failed: %v", err)
	}
	return module
}

// TestTheHeaderStatesTheRealCSignature is the whole point.
//
// A C author reads this file to find out what the program expects. A prototype
// that says one pointer where the module passes two does not fail — it compiles,
// links, and reads a length out of a pointer, which is the failure this FFI has
// spent its life refusing to produce.
func TestTheHeaderStatesTheRealCSignature(t *testing.T) {
	header, _ := generateFor(t, `
		extern "C" fn greet(name: String) -> String;
		extern "C" fn add(a: Int, b: Int) -> Int;
		extern "C" fn scale(value: Float, by: Float) -> Float;
		extern "C" fn flag(yes: Bool) -> Byte;
		extern "C" fn handler(cb: fn(Int) -> Int) -> Void;
		extern "C" fn nothing() -> Void;
	`, "example")

	for _, want := range []string{
		"void greet(const char *name, size_t name_len, NilFFIString *out);",
		"int64_t add(int64_t a, int64_t b);",
		"double scale(double value, double by);",
		"uint8_t flag(int yes);",
		"void handler(NilFFICallback cb);",
		"void nothing(void);",
	} {
		if !strings.Contains(header, want) {
			t.Errorf("the header does not declare %q:\n%s", want, header)
		}
	}
}

// TestTheHeaderAndTheModuleAgreeOnEveryParameterCount is the check that matters
// most, because it is the one a reader cannot do by eye.
//
// The module's `declare` says how many arguments actually cross. The header says
// how many C expects. If they differ, the C implementation is written to a shape
// the compiler does not call — and nothing anywhere says so.
//
// Compared as counts rather than as types because the two are spelled
// differently by design: `const char *` in C and `ptr` in LLVM are the same thing
// written twice, and a test that translated between them would be testing its own
// translation table.
func TestTheHeaderAndTheModuleAgreeOnEveryParameterCount(t *testing.T) {
	source := `
		extern "C" fn greet(name: String) -> String;
		extern "C" fn measure(text: String) -> Int;
		extern "C" fn add(a: Int, b: Int) -> Int;
		extern "C" fn handler(cb: fn(Int) -> Int) -> Void;
		extern "C" fn nothing() -> Void;
		extern "C" fn join(head: String, tail: String, at: Int) -> String;
	`
	header, program := generateFor(t, source, "example")
	module := emitFor(t, program)

	for _, name := range []string{"greet", "measure", "add", "handler", "nothing", "join"} {
		declared := declarationFor(t, module, name)
		prototype := prototypeFor(t, header, name)

		want := strings.Count(declared, ",") + 1
		if declared == "" || declared == "()" {
			want = 0
		}
		got := countHeaderParams(t, prototype)
		if got != want {
			t.Errorf("%s takes %d arguments in the module but the header declares %d:\n"+
				"  module:   %s\n  header:   %s", name, want, got, declared, prototype)
		}
	}
}

// TestAStringsTwoParametersHaveDifferentNames is a syntax error caught as one.
//
// A string becomes a pointer and a length, but the Nilang declaration names only
// one of them. Printing the same name twice gives `size_t name` beside
// `const char *name`, which C rejects with "redefinition of parameter" — an error
// that names neither the string nor the crossing that made it two.
func TestAStringsTwoParametersHaveDifferentNames(t *testing.T) {
	header, _ := generateFor(t, `
		extern "C" fn greet(name: String) -> String;
	`, "example")

	prototype := prototypeFor(t, header, "greet")
	names := parameterNames(prototype)
	if len(names) != 3 {
		t.Fatalf("greet has %d named parameters, wanted 3:\n%s", len(names), prototype)
	}

	seen := map[string]bool{}
	for _, name := range names {
		if seen[name] {
			t.Errorf("greet names two parameters %q:\n%s", name, prototype)
		}
		seen[name] = true
	}
}

// TestACKeywordIsRefusedRatherThanPrinted is a header C cannot compile.
//
// `extern "C" fn register(handler: fn(Int) -> Int) -> Void;` is a legal Nilang
// program. It is not a legal C function, and the name is the symbol the function is
// called by, so it cannot be mangled either. The only honest answer is to refuse it
// where it is written — which is what the compiler does, by name.
func TestACKeywordIsRefusedRatherThanPrinted(t *testing.T) {
	// Chosen from the C keywords that are *also* legal Nilang identifiers.
	// `while` and `for` are C keywords too, but the Nilang parser takes them first,
	// so a program naming one never reaches this check — it is refused earlier,
	// which is the right place, and there is nothing here to test.
	refused := []string{"register", "int", "restrict", "volatile", "goto", "unsigned"}
	accepted := []string{"nine_lives", "register_it", "int64_t", "restrictive"}

	for _, name := range refused {
		t.Run(name, func(t *testing.T) {
			program := lowerFor(t, "extern \"C\" fn "+name+"(n: Int) -> Int;\n")

			header, err := Generate(program, "example")
			if err == nil {
				t.Fatalf("%q was accepted and written to a header:\n%s", name, header)
			}
			if !strings.Contains(err.Error(), "C keyword") {
				t.Errorf("the refusal does not say it is a keyword:\n%v", err)
			}
			// The refusal has to name the function, or the reader is left looking
			// for something that is not there.
			if !strings.Contains(err.Error(), name) {
				t.Errorf("the refusal does not name the function:\n%v", err)
			}
			// And it has to say the name cannot be changed, because "use a
			// different name" is the obvious wrong advice: the name *is* the symbol.
			if !strings.Contains(err.Error(), "cannot be changed") {
				t.Errorf("the refusal suggests a fix that would break the link:\n%v", err)
			}
		})
	}

	for _, name := range accepted {
		t.Run(name, func(t *testing.T) {
			program := lowerFor(t, "extern \"C\" fn "+name+"(n: Int) -> Int;\n")
			if _, err := Generate(program, "example"); err != nil {
				t.Errorf("%q is not a C keyword and was refused anyway: %v", name, err)
			}
		})
	}
}

// TestAProgramWithNoExternsStillGetsAValidHeader is the case a reader is most
// likely to be confused by.
//
// An empty file looks like a failed build rather than an answer. A header that
// includes the FFI's own definitions and says so is a header somebody can include
// unconditionally.
func TestAProgramWithNoExternsStillGetsAValidHeader(t *testing.T) {
	header, _ := generateFor(t, `let a = 1; a + 1;`, "quiet")

	if !strings.Contains(header, "#ifndef") || !strings.Contains(header, "#endif") {
		t.Errorf("the header is not guarded, so including it twice is an error:\n%s", header)
	}
	if !strings.Contains(header, `#include "nilffi.h"`) {
		t.Errorf("the header does not include the FFI's own definitions:\n%s", header)
	}
	for _, line := range strings.Split(header, "\n") {
		if strings.HasSuffix(strings.TrimSpace(line), ");") && !strings.HasPrefix(line, "#") {
			t.Errorf("a program with no externs produced a prototype: %s", line)
		}
	}
}

// TestTheHeaderIsTheSameEveryTime is what makes it cacheable.
//
// A build keyed on this file would rebuild whenever map order changed, and two
// developers on the same commit would see different diffs for the same program.
func TestTheHeaderIsTheSameEveryTime(t *testing.T) {
	source := `
		extern "C" fn zebra(a: String) -> Int;
		extern "C" fn alpha(a: String) -> Int;
		extern "C" fn middle(a: String) -> Int;
	`
	first, _ := generateFor(t, source, "ordered")
	for i := 0; i < 8; i++ {
		again, _ := generateFor(t, source, "ordered")
		if again != first {
			t.Fatalf("two runs produced different headers:\n--- first ---\n%s\n--- again ---\n%s",
				first, again)
		}
	}

	// Sorted, so the order is a decision rather than an accident of the map.
	alpha := strings.Index(first, "alpha")
	middle := strings.Index(first, "middle")
	zebra := strings.Index(first, "zebra")
	if alpha > middle || middle > zebra {
		t.Errorf("the declarations are not in a stable order:\n%s", first)
	}
}

// TestTheHeaderSaysNotToEdit is the rule that keeps it derived.
//
// A header somebody edited looks edited, and the edit is undone by the next build
// with no message. The first line is what a reader sees before deciding whether to
// bother.
func TestTheHeaderSaysNotToEdit(t *testing.T) {
	header, _ := generateFor(t, `extern "C" fn f(n: Int) -> Int;`, "example")
	if !strings.Contains(header, "Do not edit") {
		t.Errorf("the header does not say it is generated:\n%s", header)
	}
}

// TestAGuardIsValidForAnyProjectName guards the guard.
//
// A project called `my-app` must not produce `#ifndef MY-APP`: that is not an
// identifier, and the failure arrives at the point somebody includes the file, as a
// message about a token they cannot see.
func TestAGuardIsValidForAnyProjectName(t *testing.T) {
	for _, name := range []string{"greeting", "my-app", "my.app", "123", "", "a b"} {
		guard := guardName(name)
		if !strings.HasPrefix(guard, "NILANG_") {
			t.Errorf("%q produced the guard %q, which does not start with the prefix", name, guard)
		}
		for _, r := range guard {
			ok := r == '_' || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
			if !ok {
				t.Errorf("%q produced the guard %q, which has %q in it", name, guard, r)
				break
			}
		}
	}
}

// declarationFor finds one function's `declare` in a module.
func declarationFor(t *testing.T, module, name string) string {
	t.Helper()
	for _, line := range strings.Split(module, "\n") {
		if strings.HasPrefix(line, "declare ") && strings.Contains(line, "@"+name+"(") {
			params := line[strings.Index(line, "@"+name+"(")+len("@"+name)+1:]
			return strings.TrimSuffix(params[:strings.Index(params, ")")], "")
		}
	}
	t.Fatalf("the module declares no %q:\n%s", name, module)
	return ""
}

// prototypeFor finds one function's prototype in a header.
func prototypeFor(t *testing.T, header, name string) string {
	t.Helper()
	for _, line := range strings.Split(header, "\n") {
		if !strings.HasSuffix(line, ");") {
			continue
		}
		if strings.Contains(line, name+"(") {
			return line
		}
	}
	t.Fatalf("the header declares no %q:\n%s", name, header)
	return ""
}

// countHeaderParams counts a prototype's parameters, treating `(void)` as none.
func countHeaderParams(t *testing.T, prototype string) int {
	t.Helper()
	open := strings.Index(prototype, "(")
	close := strings.LastIndex(prototype, ")")
	if open < 0 || close < open {
		t.Fatalf("%q has no parameter list", prototype)
	}
	inner := strings.TrimSpace(prototype[open+1 : close])
	if inner == "" || inner == "void" {
		return 0
	}

	count := 1
	depth := 0
	for _, r := range inner {
		switch r {
		case '(', '<':
			depth++
		case ')', '>':
			depth--
		case ',':
			if depth == 0 {
				count++
			}
		}
	}
	return count
}

// parameterNames reads the names out of a prototype.
//
// Read from the end of each parameter rather than by splitting on spaces, because
// `const char *name` has no space to split on and a pointer is the one parameter
// whose name the naive reading gets wrong.
func parameterNames(prototype string) []string {
	open := strings.Index(prototype, "(")
	close := strings.LastIndex(prototype, ")")
	if open < 0 || close < open {
		return nil
	}
	inner := strings.TrimSpace(prototype[open+1 : close])
	if inner == "" || inner == "void" {
		return nil
	}

	parts := strings.Split(inner, ",")
	names := make([]string, 0, len(parts))
	for _, part := range parts {
		fields := strings.Fields(strings.TrimSpace(part))
		if len(fields) == 0 {
			continue
		}
		names = append(names, fields[len(fields)-1])
	}
	return names
}
