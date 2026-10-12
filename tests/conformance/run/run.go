// Package run compares what the three engines do with the same program.
//
// # Why a comparison and not three separate tests
//
// Three engines that each have their own passing test suite agree with themselves.
// What matters about a language with three engines is that they agree with *each
// other*, and that is not a property any one of them can test. A value the
// interpreter rounds one way and the compiler rounds another is a bug in both and a
// bug in neither, so it has to be tested from outside.
//
// # The comparison is of rendered text
//
// Not of Go values. The ahead-of-time engine is a separate process; there is no Go
// value to compare. So the program runs in all three, and what is compared is what
// each one *shows* — the interpreter's `Inspect`, the VM's `Inspect`, and the
// compiled program's own output through the runtime's renderer.
//
// # Three renderers is a real cost
//
// Every engine renders text with its own code: the interpreter uses Go's `%g` for
// floats, the VM does whatever its object package does, and the C runtime has its
// own. They agree only if they are made to, which is why a rendering difference
// here is reported as a conformance failure rather than dismissed as formatting.
// The alternative — comparing parsed values — would hide exactly the disagreements
// a user would notice.
package run

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joysriramsarkar/nilLang/compiler/aot"
	"github.com/joysriramsarkar/nilLang/compiler/compiler"
	"github.com/joysriramsarkar/nilLang/compiler/evaluator"
	"github.com/joysriramsarkar/nilLang/compiler/llvm"
	"github.com/joysriramsarkar/nilLang/compiler/object"
	"github.com/joysriramsarkar/nilLang/compiler/toolchain"
	"github.com/joysriramsarkar/nilLang/compiler/vm"
)

// Outcome is what one engine did with a program.
type Outcome struct {
	// Engine names which of the three produced this.
	Engine string

	// Text is what the engine showed. For the interpreter and the VM it is the
	// object's `Inspect`; for the compiled program it is what the process printed.
	Text string

	// Failed is true when the engine reported an error.
	//
	// A separate field rather than an empty Text, because `""` is a perfectly good
	// value — the empty string — and treating it as "no answer" would make a
	// program whose answer *is* the empty string look like a crash.
	Failed bool

	// Message is the error's text, when there is one.
	Message string

	// Skipped is non-empty when this engine could not run the program at all, and
	// says why.
	//
	// # Why a skip is reported rather than hidden
	//
	// The ahead-of-time backend implements a subset of the language so far. A test
	// that quietly ignored the cases it could not run would show a green bar while
	// three quarters of the language had never been compiled — and the number it
	// would show would be the number that happened to work, which is the one number
	// nobody should trust.
	//
	// So the skips are counted, printed, and asserted against a list that a reader
	// can check. Reducing that list is the work.
	Skipped string

	// Module is the IR the compiled program was built from, kept for a failure
	// message.
	Module string
}

// Evaluator runs a program in the tree-walking interpreter.
func Evaluator(source string) Outcome {
	outcome := Outcome{Engine: "evaluator"}

	program, ok := lower(source)
	if !ok {
		outcome.Skipped = "the frontend rejected it"
		return outcome
	}

	// A closure with recover, because a Go stack overflow from deeply recursive
	// Nilang code takes the whole test process with it otherwise — and a conformance
	// run that dies on one case reports nothing about the other two hundred.
	func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				outcome.Failed = true
				outcome.Message = fmt.Sprintf("stack overflow: %v", recovered)
			}
		}()
		outcome.Text, outcome.Failed, outcome.Message = render(evaluator.Eval(program, object.NewEnvironment()))
	}()
	return outcome
}

// VM runs a program in the stack virtual machine.
//
// The VM compiles from the same lowered program the evaluator uses. It is not the
// MIR path — that is a separate engine with its own comparison, in the MIR tests —
// because the question here is whether the two mature engines and the new one agree,
// and adding a fourth variable would make a disagreement impossible to attribute.
func VM(program astProgram) Outcome {
	outcome := Outcome{Engine: "vm"}

	comp := compiler.New()
	if err := comp.Compile(program); err != nil {
		outcome.Failed = true
		outcome.Message = err.Error()
		return outcome
	}

	machine := vm.New(comp.Bytecode())
	if err := machine.Run(); err != nil {
		outcome.Failed = true
		outcome.Message = err.Error()
		return outcome
	}
	outcome.Text, outcome.Failed, outcome.Message = render(machine.LastPoppedStackElem())
	return outcome
}

// AOT compiles, links and runs a program, returning what it printed.
func AOT(t *testing.T, source, name string) Outcome {
	outcome := Outcome{Engine: "aot"}

	program, ok := lower(source)
	if !ok {
		outcome.Skipped = "the frontend rejected it"
		return outcome
	}

	module, err := Emit(program, llvm.TargetHost)
	if err != nil {
		outcome.Skipped = err.Error()
		return outcome
	}
	outcome.Module = module

	tc := toolchain.DiscoverToolchain(llvm.TargetHost)
	if _, available := tc.Linker(); !available {
		outcome.Skipped = "no linker on this machine"
		return outcome
	}

	dir := t.TempDir()
	output := filepath.Join(dir, "program")
	if filepath.Ext(output) == "" {
		output += ".exe"
	}

	// The runtime cache is shared across cases rather than built per case: four C
	// files compiled once instead of once per conformance case, which is the
	// difference between a suite that takes a minute and one that takes an hour.
	runtimeDir := filepath.Join(sharedTempDir(t), "nilcore")

	if _, err := aot.Build(module, llvm.TargetHost, tc, aot.Options{
		OutputPath: output,
		RuntimeDir: runtimeDir,
		// The module is kept so a failure can name the line that was wrong. It is
		// kept in a directory that outlives the test run, because the error arrives
		// after the temporary directory has already been cleaned — and an error
		// pointing at a file nobody can open is an error nobody can act on.
		KeepModule: true,
	}); err != nil {
		outcome.Skipped = "linking failed: " + firstLines(err.Error(), 8)
		return outcome
	}

	runOutput, runErr := exec.Command(output).CombinedOutput()
	if runErr != nil {
		// A program that fails at run time is not a "skip": the compiled engine
		// reached a different answer than the others, and saying so is the whole
		// point of running it.
		outcome.Failed = true
		outcome.Message = strings.TrimSpace(string(runOutput))
		return outcome
	}
	outcome.Text = strings.TrimRight(string(runOutput), "\r\n")
	return outcome
}

// sharedTempDir is one directory for the whole package's runtime cache.
func sharedTempDir(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(os.TempDir(), "nilang-aot-conformance")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("cannot create the shared runtime cache: %v", err)
	}
	return dir
}

// render turns an engine's result into comparable text.
func render(obj object.Object) (string, bool, string) {
	if obj == nil {
		return "null", false, ""
	}
	switch err := obj.(type) {
	case *object.RuntimeError:
		return "", true, err.Message
	case *object.Error:
		return "", true, err.Message
	}
	return obj.Inspect(), false, ""
}

// firstLines keeps the opening of a multi-line message.
//
// # Why several lines and not one
//
// A compiler's error is three lines: the message, the offending source, and a caret.
// The first line alone says what happened but not where, and a skip recorded as
// "linking failed: lld-link: error: undefined symbol" is indistinguishable from
// twenty other undefined symbols. The whole point of recording the reason is being
// able to act on it later.
func firstLines(text string, count int) string {
	lines := strings.Split(strings.TrimSpace(text), "\n")
	if len(lines) > count {
		lines = lines[:count]
		lines = append(lines, "...")
	}
	return strings.Join(lines, " | ")
}
