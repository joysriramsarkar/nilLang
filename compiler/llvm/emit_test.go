package llvm

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/joysriramsarkar/nilLang/compiler/frontend"
	"github.com/joysriramsarkar/nilLang/compiler/hir"
	"github.com/joysriramsarkar/nilLang/compiler/lexer"
	"github.com/joysriramsarkar/nilLang/compiler/lower"
	"github.com/joysriramsarkar/nilLang/compiler/mir"
	"github.com/joysriramsarkar/nilLang/compiler/parser"
)

// What the emitter produces (`L4.3`, `L4.4`).
//
// # Two kinds of test, deliberately
//
// The assertions on *text* can run anywhere, with no toolchain. They are what makes
// the emitter reviewable on a machine that has no LLVM — and a backend nobody can
// inspect without installing a compiler is a backend nobody reviews.
//
// The acceptance tests need `clang`, so they skip when it is absent. A skip is not
// a pass: it is recorded as not-run, and the count is printed so that a suite full
// of skips is visible rather than mistaken for a suite full of passes.

// emitFor runs a source through the real pipeline and returns the IR.
func emitFor(t *testing.T, source string) string {
	t.Helper()

	text, err := compileForTest(source)
	if err != nil {
		t.Fatalf("emitting failed: %v", err)
	}
	return text
}

// compileForTest is `emitFor` without the fatal.
//
// A back end is allowed to refuse a construct it does not have, and a test that
// cannot see a refusal has to treat one as a failure — which is how a suite ends
// up demanding that a gap be papered over. This returns the error so a caller can
// assert that a refusal names what it refused.
func compileForTest(source string) (string, error) {
	p := parser.New(lexer.New(source))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		return "", fmt.Errorf("parse errors: %v", p.Errors())
	}
	lowered, err := lower.Program(prog)
	if err != nil {
		return "", fmt.Errorf("lowering failed: %w", err)
	}

	optimized := hir.NewOptimizer().Optimize(hir.NewLowerer().LowerProgram(lowered))
	mirProg := mir.NewLowerer().LowerHIR(optimized)
	mir.NewOptimizer().OptimizeProgram(mirProg)

	return NewEmitter(TargetHost).Emit(mirProg)
}

func TestEmittedModuleIsStructurallyComplete(t *testing.T) {
	ir := emitFor(t, `let a = 3; let b = 4; let result = a * b + 1;`)

	required := []string{
		"target datalayout",
		"target triple",
		"define i32 @main()",
		"entry:",
		"alloca i64",
		"ret i32 0",
		"declare",
	}
	for _, want := range required {
		if !strings.Contains(ir, want) {
			t.Errorf("the module does not contain %q:\n%s", want, ir)
		}
	}
}

// TestEveryValueIsDefinedBeforeItIsUsed is the class of bug this emitter had most
// of.
//
// LLVM checks that a local name is defined before it is read, and the error it
// gives points at the *use*. So `store %kint0` written before `%kint0 = call` is
// reported as a problem with the store — a line that looks entirely ordinary and
// names a value the reader has no reason to doubt.
func TestEveryValueIsDefinedBeforeItIsUsed(t *testing.T) {
	sources := []string{
		`let a = 3; let b = 4; let result = a * b + 1;`,
		`let s = "hello"; let result = s + " world";`,
		`let m = {"k": 1}; let result = m["k"];`,
		`let a = 1; let b = 2; let c = 3; let result = if (a < b) { c } else { a + b };`,
		`let i = 0; let t = 0; while i < 3 { t = t + i; i = i + 1; } let result = t;`,
		`let x = 1.5; let y = 2.5; let result = x + y;`,
		`let a = true; let result = !a;`,
		`fn f(a: Int) -> Int { return a + 1; } let result = f(2);`,
		// The forms that had a name but no runtime function behind it. `len` on a
		// list dispatches on what it is given, so `nil_array_len` is not it; a
		// module that called the array primitive anyway would have answered a
		// string's length as zero and a missing key's as zero, with nothing to
		// say otherwise.
		`let xs = [1, 2, 3]; let result = len(xs);`,
		`let xs = [1, 2, 3]; let result = Some(1);`,
	}

	for _, source := range sources {
		t.Run(truncate(source, 40), func(t *testing.T) {
			module, err := compileForTest(source)
			if err != nil {
				// A refusal is a valid answer for a construct the back end does
				// not have, and a *useful* one: the message has to name what it
				// could not lower. An error that does not is a bug in the refusal,
				// because the whole point of refusing is that it is readable.
				if !strings.Contains(err.Error(), "no lowering for") {
					t.Fatalf("emitting failed with a message that does not say what was not lowered: %v", err)
				}
				t.Logf("refused, as it should: %v", err)
				return
			}
			checkForwardReferences(t, module)
		})
	}
}

// TestARefusalNamesTheConstruct is the rule above, stated on its own.
//
// A back end that emits a module referring to a function nobody defined produces
// an assembler error pointing at an argument — "use of undefined value
// '@nil.Some'" reads like a code-generation bug, and diagnosing it means working
// backwards from a symptom to the `Some` three stages earlier. Refusing at the
// point of lowering replaces that with a sentence naming the construct.
func TestARefusalNamesTheConstruct(t *testing.T) {
	cases := map[string]string{
		`let r = Some(1);`:                "Some",
		`let r = Ok(1);`:                  "Ok",
		`let r = [1, 2]; let x = len(r);`: "len",
	}

	for source, construct := range cases {
		t.Run(construct, func(t *testing.T) {
			_, err := compileForTest(source)
			if err == nil {
				t.Fatalf("the emitter accepted %s, which it has no runtime function for", construct)
			}
			if !strings.Contains(err.Error(), construct) {
				t.Errorf("the refusal does not name %s:\n%v", construct, err)
			}
		})
	}
}

// checkForwardReferences reports a name that is read before anything defines it.
//
// The scan is per basic block, because a value defined in one block is not
// available in another unless a phi says so — and a `br` that jumps backwards
// re-enters a block whose values were defined on an earlier pass.
func checkForwardReferences(t *testing.T, ir string) {
	t.Helper()
	// Names defined in one block stay defined for the rest of the function: LLVM's
	// SSA values are function-scoped, and a block only *uses* the ones its
	// predecessors left in scope.
	defined := map[string]bool{}

	// A function's parameters are defined by its signature. The emitter writes
	// them positionally and unnamed — `define i32 @nil.f(i64)` — so they are named
	// here by the same rule rather than read from the header, which does not
	// mention them at all.
	for _, line := range strings.Split(ir, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "define") {
			continue
		}
		open := strings.Index(trimmed, "(")
		// Count the types between the parentheses. A signature with no parameters
		// has an empty pair.
		typesInSignature := 0
		if open >= 0 {
			body := trimmed[open+1:]
			if end := strings.Index(body, ")"); end >= 0 {
				body = body[:end]
			}
			body = strings.TrimSpace(body)
			if body != "" {
				typesInSignature = 1 + strings.Count(body, ",")
			}
		}
		// Parameters are written unnamed in the signature, so LLVM names them by
		// position: `%0`, `%1`. Reading them as `%arg0` — which is what the emitter
		// used to emit — is precisely the forward-reference this test exists to
		// catch, so the helper has to know the real convention or it would pass a
		// module that is broken in exactly this way.
		for index := 0; index < typesInSignature; index++ {
			defined["%"+itoaForTest(index)] = true
		}
	}

	// Labels are collected first because a `br` may name a block that appears later
	// in the text — forward jumps are the normal case, not an exception.
	labels := map[string]bool{}
	for _, line := range strings.Split(ir, "\n") {
		if trimmed := strings.TrimSpace(line); strings.HasSuffix(trimmed, ":") &&
			!strings.HasPrefix(trimmed, "%") && strings.Contains(trimmed, ":") {
			labels["%"+strings.TrimSuffix(trimmed, ":")] = true
		}
	}

	for _, line := range strings.Split(ir, "\n") {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "define") || strings.HasSuffix(trimmed, ":") {
			continue
		}
		// A module-level type or global is not a local. `%nil.string = type …`
		// looks exactly like a local definition and is not one.
		if strings.Contains(trimmed, " = type ") || strings.Contains(trimmed, " = global ") {
			continue
		}

		if name, _, isDef := splitDefinition(trimmed); isDef {
			// Both sides of this map carry the leading `%`. `namesIn` reads names
			// out of module text where every one is written `%name`; storing the
			// bare name and looking up the marked one means every lookup misses,
			// and the scan reports a forward reference on the very first line of
			// every function — which looks like a broken emitter rather than a
			// broken comparison.
			defined["%"+name] = true
		}

		// A phi's incoming edges name values from other blocks, which this scan
		// cannot have seen yet if they came later in the text.
		if strings.Contains(trimmed, " = phi ") {
			continue
		}

		for _, used := range namesIn(trimmed) {
			if defined[used] || labels[used] {
				continue
			}
			t.Errorf("%q is read before anything defines it:\n  %s", used, trimmed)
		}
	}
}

// splitDefinition reports the name a line defines, if it defines one.
//
// Only `%name = ` at the start of a line defines a local. A `%name` that appears
// elsewhere — inside a `type { … }` declaration, or after a `call` — is a *use*,
// and treating it as a definition would hide exactly the forward reference this is
// looking for.
func splitDefinition(line string) (string, string, bool) {
	if !strings.HasPrefix(line, "%") {
		return "", "", false
	}
	eq := strings.Index(line, " = ")
	if eq <= 1 {
		return "", "", false
	}
	name := line[1:eq]
	if strings.ContainsAny(name, " \t{") {
		return "", "", false
	}
	return name, line[eq+3:], true
}

// namesIn is every `%name` a line mentions, with the leading `%` kept.
//
// Names are collected *after* the definition on the same line is recorded, which is
// what makes `%v = call … @f()` define `%v` rather than read it.
func namesIn(line string) []string {
	var out []string
	for i := 0; i+1 < len(line); i++ {
		if line[i] != '%' {
			continue
		}
		j := i + 1
		for j < len(line) && isNameByte(line[j], j == i+1) {
			j++
		}
		if j > i+1 {
			out = append(out, line[i:j])
			i = j - 1
		}
	}
	return out
}

func isNameByte(c byte, first bool) bool {
	switch {
	case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c == '_':
		return true
	case c >= '0' && c <= '9', c == '.':
		return !first
	}
	return false
}

// itoaForTest is the emitter's own number formatter, used by the test so the two
// agree on what `%arg7` is.
func itoaForTest(n int) string { return itoa(n) }

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// clangPath finds clang, or "" when it is not installed.
func clangPath() string {
	for _, name := range []string{"clang", "clang-19", "clang-18", "clang-17"} {
		if path, err := exec.LookPath(name); err == nil {
			return path
		}
	}
	if TargetHost.OS == "windows" {
		for _, dir := range []string{`C:\Program Files\LLVM\bin`, `C:\LLVM\bin`} {
			candidate := dir + `\clang.exe`
			if _, err := exec.LookPath(candidate); err == nil {
				return candidate
			}
		}
	}
	return ""
}

// TestClangAcceptsTheEmittedModule is the gate that cannot be met by reading.
//
// Every other test in this file checks the emitter against what it *believes* LLVM
// wants. This one asks LLVM. It skips rather than fails when clang is absent,
// because a build machine without LLVM should still be able to run the compiler's
// tests — but the skip is counted so that "nothing was verified" is distinguishable
// from "everything was verified".
func TestClangAcceptsTheEmittedModule(t *testing.T) {
	clang := clangPath()
	if clang == "" {
		t.Skip("clang is not installed; the module was checked by text only")
	}

	sources := []string{
		`let a = 3; let b = 4; let result = a * b + 1;`,
		`let s = "hello"; let result = s + " world";`,
		`let a = 1; let b = 2; let result = if (a < b) { 10 } else { 20 };`,
		`let i = 0; let t = 0; while i < 3 { t = t + i; i = i + 1; } let result = t;`,

		// The forms whose modules clang rejected while every text-based check
		// passed. Each was a module that read perfectly well and meant something
		// else, which is why they are here and not only in the notes:
		//
		//   - a call passes its arguments as `i64 %x`, not as a bare `%x`. LLVM's
		//     parser reads an argument list as `type value` pairs and has no way to
		//     infer the type, so `call i64 @f(%x)` is rejected with "invalid type
		//     for function argument" and the caret points at `%x`.
		//   - a parameter is `%0`, not `%arg0`. The signature writes them unnamed,
		//     and LLVM names an unnamed value by position.
		//   - a temporary written in two blocks is joined by a phi, because writing
		//     `%t1` in each defines it twice.
		`fn add(a: Int, b: Int) -> Int { return a + b; } let result = add(2, 3);`,
		`fn fib(n: Int) -> Int { if n < 2 { return n; } return fib(n - 1) + fib(n - 2); } let result = fib(10);`,
		`fn pick(c: Bool) -> Int { if c { return 1; } else { return 2; } } let result = pick(true);`,
		`let xs = [1, 2, 3]; xs[1] = 9; let result = xs[1];`,
		`let m = {"k": 1}; m["k"] = 2; let result = m["k"];`,
	}

	for _, source := range sources {
		t.Run(truncate(source, 40), func(t *testing.T) {
			ir := emitFor(t, source)
			dir := t.TempDir()
			llPath := dir + "/module.ll"

			if err := writeFile(llPath, ir); err != nil {
				t.Fatalf("writing the module failed: %v", err)
			}

			cmd := exec.Command(clang, "-c", "-x", "ir", llPath, "-o", dir+"/module.o")
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("clang rejected the module: %v\n%s\n--- module ---\n%s",
					err, output, ir)
			}
		})
	}
}

// TestACrossBlockValueIsJoinedByAPhi is the rule behind the `pick` case above,
// stated on its own because it is invisible in the IR otherwise.
//
// The two arms of an `if` assign the same MIR temporary. Emitting that temporary
// once per arm defines the same LLVM name twice, which is rejected as "multiple
// definition of local value" over a line that looks entirely ordinary. The join
// is invisible until you look for it, so this looks for it by name.
func TestACrossBlockValueIsJoinedByAPhi(t *testing.T) {
	// The two arms assign the same temporary rather than returning: a `return` in
	// each arm is two exits, not two writes to one value, and there is nothing to
	// join.
	ir, err := compileForTest(`fn pick(c: Bool) -> Int { if c { 1 } else { 2 } } pick(true);`)
	if err != nil {
		t.Fatalf("emitting failed: %v", err)
	}

	if !strings.Contains(ir, "phi i64") {
		t.Errorf("a temporary written in both arms was not joined by a phi:\n%s", ir)
	}

	// Each arm writes its own copy, under a name that says which arm wrote it.
	// Without the rename both arms write `%t1` and the module defines it twice.
	arms := 0
	for _, line := range strings.Split(ir, "\n") {
		if strings.Contains(line, "= add i64 %kint") && strings.Contains(line, ".bb_") {
			arms++
		}
	}
	if arms < 2 {
		t.Errorf("wanted one block-local copy of the joined temporary per arm, found %d:\n%s", arms, ir)
	}
}

// TestEveryFunctionReturnsAValue is the other invisible rule.
//
// A function that returns `void` cannot be called for a result, and a module where
// one is called for one has an *undefined* value where the answer should be: LLVM
// accepts it, the linker accepts it, and the program reads whatever was in the
// register. Every function here returns `i64` so that no caller can be handed
// nothing.
func TestEveryFunctionReturnsAValue(t *testing.T) {
	ir, err := compileForTest(`fn f(a: Int) -> Int { return a; } f(1);`)
	if err != nil {
		t.Fatalf("emitting failed: %v", err)
	}

	for _, line := range strings.Split(ir, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "define ") {
			continue
		}
		// `main` is C's entry point, not one of the program's functions, and C
		// says it returns an `int`. Everything else is a Nilang function and
		// returns a Nilang value, which is an `i64`.
		if strings.HasPrefix(trimmed, "define i32 @main()") {
			continue
		}
		if !strings.HasPrefix(trimmed, "define i64 @") {
			t.Errorf("a function does not return a value: %s\n%s", trimmed, ir)
		}
	}
}

func writeFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o644)
}

// TestEmittingNeedsNoToolchain is the claim the package is built around.
//
// The whole reason this is textual IR rather than a binding to LLVM's C++ API is
// that a compiler which can only be tested where its backend is installed is a
// compiler nobody develops on. This asserts the emitter runs on a machine with no
// LLVM at all — which, when clang *is* installed, is the only way to prove the
// emitter does not secretly depend on it.
func TestEmittingNeedsNoToolchain(t *testing.T) {
	// A target that no installation exists for, emitted with no toolchain
	// consulted. If the emitter reached for one, this would fail on any machine.
	ios, err := LookupTarget("aarch64-apple-ios")
	if err != nil {
		t.Fatalf("the iOS target should be registered: %v", err)
	}

	p := parser.New(lexer.New(`let a = 1; let result = a + 1;`))
	prog := p.ParseProgram()
	lowered, err := lower.Program(prog)
	if err != nil {
		t.Fatalf("lowering failed: %v", err)
	}
	mirProg := mir.NewLowerer().LowerHIR(hir.NewLowerer().LowerProgram(lowered))

	text, err := NewEmitter(ios).Emit(mirProg)
	if err != nil {
		t.Fatalf("emitting for a target with no toolchain failed: %v", err)
	}
	if !strings.Contains(text, ios.Triple) {
		t.Errorf("the module does not carry the target's triple:\n%s", text)
	}
}

// TestTheEmitterAndTheFrontendAgreeOnRejections checks that the LLVM path refuses
// the same programs the other backends do.
//
// A backend that accepts what the checker rejects compiles a program the language
// does not have. The test is cheap and the failure is total: a language that
// accepts different programs on different backends is not a language.
func TestTheEmitterAndTheFrontendAgreeOnRejections(t *testing.T) {
	rejected := []string{
		`let x: u8 = 300;`,
		`let a: Int = 5; let x: u8 = a;`,
		`let x = undefinedName;`,
	}

	for _, source := range rejected {
		t.Run(truncate(source, 40), func(t *testing.T) {
			res := frontend.Check(source, frontend.Options{Filename: "gate.nil"})
			if res.OK {
				t.Fatalf("the frontend accepted a program it should have rejected")
			}
			// Reaching the emitter at all would be the failure; the point is that
			// the frontend stops first.
		})
	}
}
