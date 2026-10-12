package llvm

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/joysriramsarkar/nilLang/compiler/ffi"
)

// headerPath is `runtime/core/include/nil.h`, found relative to this file.
//
// Found this way rather than from the working directory because a test that only
// passes when run from the repository root is a test that will not run in CI, and
// the thing it checks — the C library and the emitted declarations agreeing — is
// exactly the thing CI should be checking.
// headerPath returns the public headers the emitter's symbols must be declared in.
//
// Two files, not one, because the runtime's surface is two: `nil.h` is what
// generated code includes, and `nilffi.h` is what a *callback* crosses through.
// Checking only the first would report every FFI symbol as missing — and the fix
// that invites is putting them in `nil.h`, which would make the boundary's
// functions part of the interface that ordinary Nilang code is expected to use.
func headerPaths(t *testing.T) []string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot work out where this file is")
	}
	runtimeDir := filepath.Join(filepath.Dir(file), "..", "..", "runtime")

	var paths []string
	for _, relative := range []string{
		filepath.Join("core", "include", "nil.h"),
		filepath.Join("ffi", "include", "nilffi.h"),
	} {
		path := filepath.Join(runtimeDir, relative)
		if _, err := os.Stat(path); err != nil {
			t.Skipf("the C header is not at %s: %v", path, err)
		}
		paths = append(paths, path)
	}
	return paths
}

func headerPath(t *testing.T) string {
	return headerPaths(t)[0]
}

// cDeclaration matches a function declaration in the public header.
//
// The header declares without `extern`, which is what makes this pattern work:
// every line that looks like this is a function the library promises to export.
// The capture groups are the return type, the name, and the parameter list.
// The parameter list runs to the first `;`, not to the first `)`.
//
// A function-pointer parameter contains parentheses of its own — `NilValue
// (*function)(NilValue *args, int64_t count)` — and `[^)]*` stopped at the one
// closing the function-pointer declarator, so `nil_closure_new` did not match at
// all and was invisible to every check in this file. `[^;]*` runs to the end of
// the declaration instead, which is where it actually ends.
var cDeclaration = regexp.MustCompile(
	`(?m)^\s*((?:const\s+)?[A-Za-z_][A-Za-z0-9_]*\s*\*?)\s*(nil_[A-Za-z0-9_]+)\s*\(([^;]*)\)\s*;`)

// nilDeclarationStart finds every line that *looks* like the start of a declaration.
//
// Used to prove the parser above missed nothing. The pattern was wrong for its
// whole life in two independent ways: it could not match a pointer return, because
// the star sat after a character class that had already eaten the space; and it
// could not match a function-pointer parameter. Neither was noticed for a year,
// because the runtime had no pointer-returning declaration and no parameter with
// nested parentheses until the FFI's scope added both.
//
// That is the shape of the bug rather than a one-off: a parser that skips a
// construct is silent, and silence is indistinguishable from "there are none".
// Counting the lines that start like declarations and comparing that with the
// number parsed turns "there are none" into something checkable.
var nilDeclarationStart = regexp.MustCompile(`(?m)^\s*[A-Za-z_][A-Za-z0-9_]*\s*\*?\s*(nil_[A-Za-z0-9_]+)\s*\(`)

// cFunction is one declaration read out of the header.
type cFunction struct {
	Return  string
	Name    string
	Params  []string
	RawName string
}

// llvmType maps a C type onto the LLVM type a declaration must use.
//
// # Why the mapping is written down rather than left implicit
//
// Because "declared `i1`, C returns `int`" is not a type error anywhere — both are
// legal LLVM, the module verifies, and the linker is what fails, with a message
// about an unresolved or mismatched symbol rather than about a wrong branch. The
// table is where that mistake gets caught instead.
func llvmType(cType string) string {
	trimmed := strings.TrimSpace(cType)
	trimmed = strings.TrimPrefix(trimmed, "const ")
	trimmed = strings.TrimSpace(trimmed)

	// A pointer is a pointer whatever it points at.
	if strings.HasSuffix(trimmed, "*") {
		return "ptr"
	}

	switch trimmed {
	case "void":
		return "void"
	case "double":
		return "double"
	case "int", "unsigned", "char", "short":
		return "i32"
	case "int64_t", "uint64_t", "size_t", "NilValue":
		return "i64"
	case "int32_t", "uint32_t":
		return "i32"
	case "float":
		return "float"
	case "int8_t", "uint8_t":
		return "i8"
	}
	return "?"
}

// parameterType reduces a parameter declaration to its type.
func parameterType(parameter string) string {
	parameter = strings.TrimSpace(parameter)
	if parameter == "" || parameter == "void" {
		return "void"
	}

	// A callback typedef, named rather than spelled with a star.
	//
	// Checked before the star test because a typedef hides the star entirely: the
	// parameter reads `NilFFICallback function` and has no `*` anywhere, so without
	// this it would be read as an unknown scalar and the check would fail with a
	// question mark where the answer is a pointer.
	if isCallbackTypedef(parameter) {
		return "ptr"
	}

	// Any star anywhere makes it a pointer. Splitting on whitespace does not work
	// for a pointer, because the star is written against the *name* — `const char
	// *bytes` — and the star-less field would otherwise be read as the type.
	if strings.Contains(parameter, "*") {
		return "ptr"
	}

	fields := strings.Fields(parameter)
	if len(fields) == 0 {
		return "?"
	}
	last := fields[len(fields)-1]

	// One field means there was no name: the field *is* the type.
	if len(fields) == 1 || isTypeKeyword(last) {
		return llvmType(last)
	}
	return llvmType(fields[len(fields)-2])
}

// isCallbackTypedef reports whether a written parameter is a callback typedef with
// no star written against it.
//
// Matched on the first field, because a parameter is `<type> <name>` and the type
// is the one that says what it is.
func isCallbackTypedef(parameter string) bool {
	fields := strings.Fields(parameter)
	return len(fields) > 0 && strings.HasSuffix(fields[0], "Callback")
}

func isTypeKeyword(word string) bool {
	switch word {
	case "int", "unsigned", "char", "short", "long", "double", "float", "void",
		"NilValue", "int64_t", "size_t":
		return true
	}
	return false
}

// readHeader parses every runtime function the public header declares.
func readHeader(t *testing.T) map[string]cFunction {
	t.Helper()

	declared := make(map[string]cFunction)
	for _, path := range headerPaths(t) {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}

		matched := map[string]bool{}
		for _, match := range cDeclaration.FindAllStringSubmatch(string(content), -1) {
			name := match[2]
			rawParams := strings.TrimSpace(match[3])

			var params []string
			if rawParams != "" && rawParams != "void" {
				for _, parameter := range strings.Split(rawParams, ",") {
					params = append(params, parameterType(parameter))
				}
			}
			declared[name] = cFunction{
				Return: llvmType(match[1]),
				Name:   name,
				Params: params,
			}
			matched[name] = true
		}

		// Every declaration that looks like one must be a declaration that was
		// read. A parser that quietly skips a form is worse than no parser: the
		// symbol is reported as "no header exports it" when the header exports it,
		// and the fix that invites is to move it somewhere the parser can see.
		//
		// Compared by name rather than by count, because counting is a coincidence
		// waiting to happen — one declaration spanning two lines matches twice
		// under a pattern whose `\s*` includes a newline, and the totals stop
		// agreeing for a reason that has nothing to do with coverage.
		for _, match := range nilDeclarationStart.FindAllStringSubmatch(string(content), -1) {
			name := match[1]
			if matched[name] {
				continue
			}
			t.Errorf("%s declares %s in a form the header parser does not understand, "+
				"so every symbol written that way is invisible to every check here",
				path, name)
		}
	}

	if len(declared) == 0 {
		t.Fatal("no declarations were read out of nil.h; the pattern is wrong")
	}
	return declared
}

// TestEveryRuntimeSymbolIsDeclaredAndMatchesItsHeader is the test that would have
// caught both of the bugs this file's table was written to fix.
//
// # What went wrong without it
//
// The module declared `@nil.print` and the C exported `nil_print`, so every print
// in every compiled program was an unresolved symbol at link time. It declared
// `i1 @nil.truthy(i64)` and the C returned `int`, so every `if` read a bit nobody
// wrote. Neither error appears in a module that only has to parse, and neither has
// a message naming the cause.
//
// # Why a test and not a link
//
// A link would catch the first and not the second: the linker accepts `i1` for a
// function returning `int` and produces a program that reads garbage. Comparing
// the declarations to the header catches both, and does it in milliseconds without
// a compiler.
func TestEveryRuntimeSymbolIsDeclaredAndMatchesItsHeader(t *testing.T) {
	declared := readHeader(t)

	for _, fn := range runtimeFunctions {
		actual, exists := declared[fn.Name]
		if !exists {
			t.Errorf("%s is declared by the emitter but neither nil.h nor nilffi.h exports it, "+
				"so every program using it would fail to link", fn.Name)
			continue
		}

		if actual.Return != fn.Result {
			t.Errorf("%s returns %s in nil.h but is declared %s in the emitter; "+
				"a mismatch here is a wrong answer with no diagnostic",
				fn.Name, actual.Return, fn.Result)
		}

		if len(actual.Params) != fn.Arity() {
			t.Errorf("%s takes %d parameters in nil.h (%s) but %d are declared",
				fn.Name, len(actual.Params), strings.Join(actual.Params, ", "), fn.Arity())
			continue
		}
		for i, want := range actual.Params {
			got := strings.TrimSpace(strings.Split(fn.Params, ",")[i])
			if want != got {
				t.Errorf("%s parameter %d is %s in nil.h but %s in the emitter",
					fn.Name, i, want, got)
			}
		}
	}
}

// TestEveryFFIConversionMatchesItsHeader closes a gap this file had.
//
// The declarations above are checked against the headers, but they are the
// *runtime's* symbols. The FFI's conversions — `nil_int_unbox`, `nil_string_unbox`,
// `nil_string_copy` and the rest — are collected per module by `ffiRuntimeFunctions`
// and were never checked against anything. Changing `nil_string_unbox` to take an
// `int64_t` where it takes a `NilFFIString *` passed the whole suite: the emitter
// declared a pointer, the header exported an integer, and every program using it
// would have failed to link with a message about a symbol rather than a type.
//
// The expectations are derived from the crossing table rather than written out, so
// this tests the code that writes the declarations instead of a second copy of
// them. A test with its own list would agree with the emitter until someone added
// a type.
func TestEveryFFIConversionMatchesItsHeader(t *testing.T) {
	declared := readHeader(t)

	checked := map[string]bool{}
	for name, crossing := range ffi.Types {
		// A crossing in both directions at once is the widest thing a module can
		// use, so it produces every declaration this table can produce.
		functions, structs := ffiRuntimeFunctions(ffi.Signature{
			Params: []ffi.Crossing{crossing},
			Ret:    crossing,
		})
		if crossing.Splat && structs == "" {
			t.Errorf("%s crosses as two values but declares no struct to carry them", name)
		}

		for _, fn := range functions {
			checkAgainstHeader(t, declared, fn)
			checked[fn.Name] = true
		}
	}

	// The callback conversions are built somewhere else, so they are named here.
	// Named rather than derived because they have no `ffi.Types` entry — a function
	// type is not looked up by name, it is recognised as one.
	//
	// The list is every symbol whose name begins `nil_callback`, found rather than
	// written out: a callback function added later and left out of this list would
	// be the one symbol in the FFI no test checks, and the way to avoid that is not
	// to remember.
	for _, fn := range runtimeFunctions {
		if strings.HasPrefix(fn.Name, "nil_callback") {
			checkAgainstHeader(t, declared, fn)
			checked[fn.Name] = true
		}
	}
	if !checked[runtimeCallbackNew] {
		t.Errorf("%s is called by the emitter but no header exports it", runtimeCallbackNew)
	}

	if len(checked) == 0 {
		t.Fatal("no FFI conversion was checked; the table it is read from is empty")
	}
}

// checkAgainstHeader is the comparison both tables want, written once.
func checkAgainstHeader(t *testing.T, declared map[string]cFunction, fn runtimeFunction) {
	t.Helper()

	actual, exists := declared[fn.Name]
	if !exists {
		t.Errorf("%s is declared by the emitter but no header exports it, so every "+
			"program using it would fail to link", fn.Name)
		return
	}

	if actual.Return != fn.Result {
		t.Errorf("%s returns %s in the header but is declared %s in the emitter; "+
			"a mismatch here is a wrong answer with no diagnostic",
			fn.Name, actual.Return, fn.Result)
	}

	emitterParams := strings.Split(fn.Params, ",")
	if fn.Params == "" {
		// A function that takes nothing splits into `[""]`, which reads as one
		// parameter of an unknown type. Everything the FFI declares with no
		// arguments — the two scope calls — would be reported as a mismatch
		// against the header's own `void`, and the test would cry wolf.
		emitterParams = nil
	}
	if len(actual.Params) != len(emitterParams) {
		t.Errorf("%s takes %d parameters in the header (%s) but %d are declared",
			fn.Name, len(actual.Params), strings.Join(actual.Params, ", "), len(emitterParams))
		return
	}
	for i, want := range actual.Params {
		got := strings.TrimSpace(emitterParams[i])
		if want != got {
			t.Errorf("%s parameter %d is %s in the header but %s in the emitter",
				fn.Name, i, want, got)
		}
	}
}

// TestTheHeaderExportsEverySymbolTheEmitterCalls closes the other direction.
//
// A name used at a call site but absent from the declaration table would compile
// to a call the module does not declare, and LLVM would reject the module with a
// message about the call rather than about the missing declaration.
func TestTheHeaderExportsEverySymbolTheEmitterCalls(t *testing.T) {
	declared := readHeader(t)

	for _, name := range []string{
		runtimeAlloc, runtimePrint, runtimePrintln, runtimeStringNew, runtimeStringLit,
		runtimeStringLen, runtimeStringConcat, runtimeIntNew, runtimeFloatNew,
		runtimeIntAdd, runtimeIntSub, runtimeIntMul, runtimeIntDiv, runtimeIntMod,
		runtimeFloatAdd, runtimeFloatSub, runtimeFloatMul, runtimeFloatDiv,
		runtimeIntCompare, runtimeStringCompare, runtimeArrayNew, runtimeArrayGet,
		runtimeArraySet, runtimeMapNew, runtimeMapGet, runtimeMapSet, runtimeGetField,
		runtimeCall, runtimeTruthy,
	} {
		if _, exists := declared[name]; !exists {
			t.Errorf("%s is called by the emitter but not declared in nil.h", name)
		}
		if _, exists := runtimeSymbol(name); !exists {
			t.Errorf("%s is called by the emitter but has no declaration in runtimeFunctions", name)
		}
	}
}

// TestRuntimeNamesUseTheCSpelling guards the convention itself.
//
// The rule is that a runtime symbol is spelled exactly as the C library exports it,
// and a program symbol carries a dot. A runtime name that came to contain a dot
// would need an `__asm__` label in the C file, and `__asm__` does not exist in
// MSVC's C compiler — so the mistake costs portability before it costs correctness.
func TestRuntimeNamesUseTheCSpelling(t *testing.T) {
	for _, fn := range runtimeFunctions {
		if strings.Contains(fn.Name, ".") {
			t.Errorf("runtime symbol %q contains a dot; it must match the C name", fn.Name)
		}
		if !strings.HasPrefix(fn.Name, "nil_") {
			t.Errorf("runtime symbol %q does not start with nil_", fn.Name)
		}
	}

	// And the other side: a program symbol must not collide with a runtime one.
	for _, name := range []string{"add", "main", "println", "print"} {
		if symbol := llvmSymbol(name); strings.Contains(symbol, "nil_") {
			t.Errorf("the program function %q became %q, which is in the runtime namespace",
				name, symbol)
		}
	}
}

// TestEveryRuntimeDeclarationIsWellFormed checks the table's own syntax.
//
// `ptr` rather than `i64*` is not a style preference: opaque pointers are what
// every LLVM version from 15 onwards accepts, and a typed pointer is what a module
// using one gets rejected for — with an error about the pointer, not about the
// target it was built for.
func TestEveryRuntimeDeclarationIsWellFormed(t *testing.T) {
	for _, fn := range runtimeFunctions {
		declaration := fn.Decl()

		if !strings.HasPrefix(declaration, "declare ") {
			t.Errorf("%s does not produce a declaration: %s", fn.Name, declaration)
		}
		if strings.Contains(declaration, "*") {
			t.Errorf("%s uses a typed pointer; opaque pointers only: %s", fn.Name, declaration)
		}
		if fn.Result == "" || fn.Result == "?" {
			t.Errorf("%s has no usable result type", fn.Name)
		}
	}
}

// TestTheRuntimeHeaderIsReadable guards the test above from passing vacuously.
//
// If `nil.h` moved or the pattern stopped matching, `readHeader` would return an
// empty map and every comparison above would report "not exported" for everything
// — loud, but for the wrong reason. This test says which file it read and how many
// declarations it found, so a failure above is interpretable.
func TestTheRuntimeHeaderIsReadable(t *testing.T) {
	declared := readHeader(t)

	if len(declared) < 30 {
		t.Errorf("only %d declarations were read out of nil.h; the pattern is "+
			"probably wrong rather than the header being small", len(declared))
	}
	t.Logf("read %d declarations from %s", len(declared), filepath.Base(headerPath(t)))
}
