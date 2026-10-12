package tests

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joysriramsarkar/nilLang/compiler/ast"
	"github.com/joysriramsarkar/nilLang/compiler/compiler"
	"github.com/joysriramsarkar/nilLang/compiler/evaluator"
	"github.com/joysriramsarkar/nilLang/compiler/frontend"
	"github.com/joysriramsarkar/nilLang/compiler/object"
	"github.com/joysriramsarkar/nilLang/compiler/vm"
	pkgcompiler "github.com/joysriramsarkar/nilLang/pkg/compiler"
)

// Module visibility: `export` and `private`.
//
// # What is being tested
//
// A module's interface is what it exports. An importer can reach an exported name
// and must not be able to reach a private one, and the two engines must agree on
// which is which. If they disagree, the same program behaves differently
// depending on whether it ran interpreted or compiled — the kind of difference
// that is found in production rather than in a test.
//
// # The migration rule
//
// A file that contains no `export` at all exports everything, so a module written
// before visibility existed keeps working unchanged. A file that does use `export`
// exports exactly what it marked. That rule is observable in both directions and
// both directions are tested, because a rule tested only in the direction that
// adds features is a rule that can quietly break the other one.

// moduleFixture is a temp directory holding a module and the program that
// imports it, together with the source directory both engines need to resolve a
// relative path.
type moduleFixture struct {
	dir     string
	global  string
	program *ast.Program
}

// newModuleFixture writes the named files into a fresh temp directory and parses
// entry as the importing program.
//
// The lowering pass runs here for the same reason evalSource does it: `nil run`
// compiles the lowered program, so a test that compiled the source would be
// testing a path the CLI never takes.
func newModuleFixture(t *testing.T, global string, files map[string]string, entry string) *moduleFixture {
	t.Helper()

	dir := t.TempDir()
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	res := frontend.Check(entry, frontend.Options{Filename: "main.nil", SourceDir: dir})
	if len(res.ParseErrors) > 0 {
		t.Fatalf("parse errors: %v", res.ParseErrors)
	}
	if !res.OK {
		msgs := make([]string, 0, len(res.Diagnostics))
		for _, d := range res.Diagnostics {
			msgs = append(msgs, d.Message)
		}
		t.Fatalf("type errors: %s", strings.Join(msgs, "; "))
	}

	program := res.Program
	if res.Lowered != nil {
		program = res.Lowered
	}

	return &moduleFixture{dir: dir, global: global, program: program}
}

// run executes the fixture in the named engine and returns the value of the
// global the caller asked about.
func (f *moduleFixture) run(t *testing.T, engine string) (string, error) {
	t.Helper()

	switch engine {
	case "evaluator":
		evaluator.ClearModuleCache()
		evaluator.PushScriptDir(f.dir)
		defer evaluator.PopScriptDir()

		env := object.NewEnvironment()
		for builtinName, builtin := range evaluator.Builtins {
			env.Set(builtinName, builtin)
		}
		if evaluated := evaluator.Eval(f.program, env); evaluated != nil {
			if errObj, ok := evaluated.(*object.Error); ok {
				return "", fmt.Errorf("%s", errObj.Inspect())
			}
		}
		bound, ok := env.Get(f.global)
		if !ok || bound == nil {
			return "", fmt.Errorf("global %q not found", f.global)
		}
		return bound.Inspect(), nil

	case "VM":
		comp := compiler.New()
		pkgcompiler.ConfigureCompiler(comp, f.dir)
		if err := comp.Compile(f.program); err != nil {
			return "", err
		}
		bytecode := comp.Bytecode()

		slot := -1
		for i, name := range bytecode.GlobalNames {
			if name == f.global {
				slot = i
				break
			}
		}
		if slot < 0 {
			return "", fmt.Errorf("global %q not found in bytecode", f.global)
		}

		machine := vm.New(bytecode)
		if err := machine.Run(); err != nil {
			return "", err
		}
		value := machine.GetGlobal(slot)
		if value == nil {
			return "", fmt.Errorf("global %q (slot %d) is unbound after Run", f.global, slot)
		}
		return value.Inspect(), nil

	default:
		t.Fatalf("unknown engine %q", engine)
		return "", nil
	}
}

// mustRunBoth runs the fixture in both engines and requires that they agree.
//
// Agreement is the requirement, not an extra: a visibility rule enforced by only
// one engine is not enforced.
func (f *moduleFixture) mustRunBoth(t *testing.T) string {
	t.Helper()

	evalValue, evalErr := f.run(t, "evaluator")
	vmValue, vmErr := f.run(t, "VM")

	if evalErr != nil {
		t.Fatalf("evaluator: %v", evalErr)
	}
	if vmErr != nil {
		t.Fatalf("VM: %v", vmErr)
	}
	if evalValue != vmValue {
		t.Fatalf("engines disagree: evaluator=%s VM=%s", evalValue, vmValue)
	}
	return evalValue
}

// mustFailBoth runs the fixture in both engines and requires that both refuse,
// with a message naming the problem.
func (f *moduleFixture) mustFailBoth(t *testing.T, wantIn string) {
	t.Helper()

	for _, engine := range []string{"evaluator", "VM"} {
		_, err := f.run(t, engine)
		if err == nil {
			t.Fatalf("%s: the program succeeded; it must not", engine)
		}
		if wantIn != "" && !strings.Contains(err.Error(), wantIn) {
			t.Fatalf("%s: error does not mention %q: %v", engine, wantIn, err)
		}
	}
}

// engines are the two backends every visibility test runs against.
var engines = []string{"evaluator", "VM"}

// explicitModule uses `export`, so it is in explicit mode and everything
// unmarked is private.
const explicitModule = `export fn pub() -> Int { return 1; }
fn priv() -> Int { return 2; }
export let pubv = 3;
let privv = 4;
`

// legacyModule is what a module written before visibility existed looks like: no
// marker anywhere, every name reachable.
const legacyModule = `fn pub() -> Int { return 1; }
fn priv() -> Int { return 2; }
let pubv = 3;
let privv = 4;
`

// TestExportedNameIsImportable covers the ordinary case: a name the module marks
// `export` comes across and behaves the same either way.
func TestExportedNameIsImportable(t *testing.T) {
	f := newModuleFixture(t, "result", map[string]string{
		"mod.nil": explicitModule,
	}, `import { pub } from "./mod.nil";
let result = pub();`)

	if got := f.mustRunBoth(t); got != "1" {
		t.Fatalf("pub() = %s, want 1", got)
	}
}

// TestExportedLetIsImportable covers the value case, which binds through a
// different path than a function in both engines.
func TestExportedLetIsImportable(t *testing.T) {
	f := newModuleFixture(t, "result", map[string]string{
		"mod.nil": explicitModule,
	}, `import { pubv } from "./mod.nil";
let result = pubv;`)

	if got := f.mustRunBoth(t); got != "3" {
		t.Fatalf("pubv = %s, want 3", got)
	}
}

// TestPrivateNameIsNotImportable is the point of the feature.
//
// Before it, a name that was not there bound nothing and the program failed later,
// at whichever line happened to read it. Both engines now refuse at the import,
// which is where the mistake is.
func TestPrivateNameIsNotImportable(t *testing.T) {
	f := newModuleFixture(t, "result", map[string]string{
		"mod.nil": explicitModule,
	}, `import { priv } from "./mod.nil";
let result = priv();`)

	f.mustFailBoth(t, "does not export")
}

// TestPrivateLetIsNotImportable repeats the rule for a value.
func TestPrivateLetIsNotImportable(t *testing.T) {
	f := newModuleFixture(t, "result", map[string]string{
		"mod.nil": explicitModule,
	}, `import { privv } from "./mod.nil";
let result = privv;`)

	f.mustFailBoth(t, "does not export")
}

// TestMissingNameIsReportedAtTheImport uses the same code path as the private
// case. It is here because the message must name the symbol that was asked for:
// "you asked for nope, and there is no nope" is a different mistake from "nope is
// private", and a reader who cannot tell them apart will look in the wrong file.
func TestMissingNameIsReportedAtTheImport(t *testing.T) {
	f := newModuleFixture(t, "result", map[string]string{
		"mod.nil": explicitModule,
	}, `import { nope } from "./mod.nil";
let result = nope;`)

	f.mustFailBoth(t, "nope")
}

// TestEveryMissingNameIsListed checks that a single import missing two names says
// both. Failing on the first turns one edit into two round trips through the
// compiler for a mistake the whole list already answers.
func TestEveryMissingNameIsListed(t *testing.T) {
	f := newModuleFixture(t, "result", map[string]string{
		"mod.nil": explicitModule,
	}, `import { nope, alsonope } from "./mod.nil";
let result = nope;`)

	f.mustFailBoth(t, "nope")
	for _, engine := range engines {
		_, err := f.run(t, engine)
		if !strings.Contains(err.Error(), "alsonope") {
			t.Fatalf("%s: second missing name not reported: %v", engine, err)
		}
	}
}

// TestLegacyModuleExportsEverything is the migration rule's other direction, and
// the reason the rule exists.
//
// Every module written before `export` was introduced has no marker anywhere.
// Treating that as "exports nothing" would break all of them at once; treating it
// as "exports everything" changes nothing for them.
func TestLegacyModuleExportsEverything(t *testing.T) {
	f := newModuleFixture(t, "result", map[string]string{
		"mod.nil": legacyModule,
	}, `import { pub, priv, pubv, privv } from "./mod.nil";
let result = pub() + priv() + pubv + privv;`)

	if got := f.mustRunBoth(t); got != "10" {
		t.Fatalf("legacy module sum = %s, want 10", got)
	}
}

// TestLegacyModuleStaysReachableThroughAnAlias checks the same rule on the
// whole-module import form, which builds a value rather than a set of names and
// so takes a different path through both engines.
func TestLegacyModuleStaysReachableThroughAnAlias(t *testing.T) {
	f := newModuleFixture(t, "result", map[string]string{
		"mod.nil": legacyModule,
	}, `import "./mod.nil" as m;
let result = m.priv();`)

	if got := f.mustRunBoth(t); got != "2" {
		t.Fatalf("m.priv() = %s, want 2", got)
	}
}

// TestPrivateIsUsableInsideTheModule is what private means: not part of the
// interface, still part of the implementation. Without this the feature would be
// "delete the declaration", which is not what anyone wants from it.
func TestPrivateIsUsableInsideTheModule(t *testing.T) {
	f := newModuleFixture(t, "result", map[string]string{
		"mod.nil": `private fn helper() -> Int { return 5; }
export fn pub() -> Int { return helper() + 1; }
`,
	}, `import { pub } from "./mod.nil";
let result = pub();`)

	if got := f.mustRunBoth(t); got != "6" {
		t.Fatalf("pub() = %s, want 6", got)
	}
}

// TestWholeModuleImportExposesExportedNames covers `import "./mod.nil" as m`.
// The module arrives as a value rather than a set of names, so what it contains is
// decided by what the loader put in it rather than by what the importer asked
// for.
func TestWholeModuleImportExposesExportedNames(t *testing.T) {
	f := newModuleFixture(t, "result", map[string]string{
		"mod.nil": explicitModule,
	}, `import "./mod.nil" as m;
let result = m.pub() + m.pubv;`)

	if got := f.mustRunBoth(t); got != "4" {
		t.Fatalf("m.pub() + m.pubv = %s, want 4", got)
	}
}

// TestExportIsTransparentAtTheDeclaration checks that marking a declaration does
// not change what it does inside its own file.
//
// Each module marks a declaration and then *uses* it itself, exporting the result
// as the witness. If the wrapper leaked into evaluation or compilation the marked
// declaration would stop working the moment anyone marked it — and it would work
// right up until that moment, so only a local use catches it.
func TestExportIsTransparentAtTheDeclaration(t *testing.T) {
	for _, decl := range []struct {
		label string
		src   string
	}{
		{"fn", "export fn f() -> Int { return 7; }\nexport let witness = f();\n"},
		{"let", "export let x = 7;\nexport let witness = x;\n"},
		{"const", "export const K = 7;\nexport let witness = K;\n"},
		{"struct", "export struct P { x: Int }\nexport fn mk() -> P { return P { x: 7 }; }\nexport let witness = mk().x;\n"},
		{"type-alias", "export type N = Int;\nexport let witness: N = 7;\n"},
		{"private-fn", "private fn f() -> Int { return 7; }\nexport let witness = f();\n"},
	} {
		t.Run(decl.label, func(t *testing.T) {
			f := newModuleFixture(t, "result", map[string]string{
				"mod.nil": decl.src,
			}, `import { witness } from "./mod.nil";
let result = witness;`)

			if got := f.mustRunBoth(t); got != "7" {
				t.Fatalf("%s witness = %s, want 7", decl.label, got)
			}
		})
	}
}

// TestExportedAndPrivateInTheSameModule checks that the two markers coexist: the
// file uses `export`, and the unmarked name is private rather than being carried
// along by the exported one.
func TestExportedAndPrivateInTheSameModule(t *testing.T) {
	f := newModuleFixture(t, "result", map[string]string{
		"mod.nil": explicitModule,
	}, `import "./mod.nil" as m;
let result = m.pub();`)

	if got := f.mustRunBoth(t); got != "1" {
		t.Fatalf("m.pub() = %s, want 1", got)
	}

	f2 := newModuleFixture(t, "result", map[string]string{
		"mod.nil": explicitModule,
	}, `import { priv } from "./mod.nil";
let result = priv();`)
	f2.mustFailBoth(t, "does not export")
}

// TestExportRejectsNonDeclarations pins the two errors, because both were possible
// to accept silently and both would leave a marker in the tree that no consumer
// knows how to read.
func TestExportRejectsNonDeclarations(t *testing.T) {
	for _, tc := range []struct {
		label string
		src   string
		want  string
	}{
		{"bare-literal", `export 42;`, "declaration"},
		{"expression", `export (1 + 2);`, "declaration"},
		{"unnamed-let", `export let = 1;`, "IDENT"},
	} {
		t.Run(tc.label, func(t *testing.T) {
			res := frontend.Check(tc.src, frontend.Options{Filename: "x.nil"})
			if len(res.ParseErrors) == 0 {
				t.Fatalf("expected a parse error for %q", tc.src)
			}
			joined := strings.Join(res.ParseErrors, "\n")
			if !strings.Contains(joined, tc.want) {
				t.Fatalf("error does not mention %q: %s", tc.want, joined)
			}
		})
	}
}

// TestExportedNamesReportsTheMode covers the helper both engines rely on. The
// second result is the migration rule itself: without it a caller cannot tell
// "this module exports nothing" from "this module predates visibility", and the
// first would have to guess.
func TestExportedNamesReportsTheMode(t *testing.T) {
	for _, tc := range []struct {
		label        string
		src          string
		wantExplicit bool
		want         []string
		notWant      []string
	}{
		{"no-marker-exports-everything", legacyModule, false, []string{"pub", "priv", "pubv", "privv"}, nil},
		{"one-marker-switches-to-explicit", explicitModule, true, []string{"pub", "pubv"}, []string{"priv", "privv"}},
		{"all-private", `private fn a() -> Int { return 1; }`, true, nil, []string{"a"}},
		{"only-private-marker-still-explicit", `private fn a() -> Int { return 1; }`, true, nil, []string{"a"}},
	} {
		t.Run(tc.label, func(t *testing.T) {
			prog := mustParse(t, tc.src)
			names, explicit := ast.ExportedNames(prog)
			if explicit != tc.wantExplicit {
				t.Fatalf("explicit = %v, want %v", explicit, tc.wantExplicit)
			}
			for _, name := range tc.want {
				if !names[name] {
					t.Fatalf("%q should be exported; got %v", name, names)
				}
			}
			for _, name := range tc.notWant {
				if names[name] {
					t.Fatalf("%q should not be exported", name)
				}
			}
		})
	}
}

// mustParse parses source for a test that looks at the tree rather than running
// it, and fails on any parse error so the caller does not have to.
func mustParse(t *testing.T, source string) *ast.Program {
	t.Helper()
	res := frontend.Check(source, frontend.Options{Filename: "x.nil"})
	if len(res.ParseErrors) > 0 {
		t.Fatalf("parse errors: %v", res.ParseErrors)
	}
	return res.Program
}
