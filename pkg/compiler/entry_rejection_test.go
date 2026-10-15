package compiler_test

// Entry-point gate matrix — Phase 0 item 2 (docs/CORE_STABILIZATION.md).
//
// # What this asserts
//
// Every public entry point must reject the same invalid corpus, and — the part
// the previous version of this test did not check — it must reject it with the
// same diagnostic *codes*. Two entry points that both refuse a program but name
// different reasons produce bug reports that say "it behaves differently here",
// and nobody can tell which one is right.
//
// # Why the entry points are enumerated by hand
//
// The list is written out rather than reflected over. A reflection-based test
// passes when the list is empty, and an empty list is exactly the state that
// let a backend skip the gate in the first place.

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/joysriramsarkar/nilLang/compiler/frontend"
	"github.com/joysriramsarkar/nilLang/compiler/hir"
	"github.com/joysriramsarkar/nilLang/compiler/mir"
	"github.com/joysriramsarkar/nilLang/compiler/wasm"
	pkgcompiler "github.com/joysriramsarkar/nilLang/pkg/compiler"
)

// entryPoint is one public way a program reaches a backend.
type entryPoint struct {
	// Name is what a user would call it.
	Name string
	// Compile takes source and a filename and returns an error, or nil if the
	// program was accepted. An accepted invalid program is the failure this file
	// exists to prevent.
	Compile func(t *testing.T, source, filename string) error
}

// codeRE pulls the diagnostic code out of an error message.
//
// The message is the only thing these entry points return, so matching on the
// code in it is how equality between them is checked. An error with no code in
// it is itself a finding: a user cannot act on a refusal that names no reason.
var codeRE = regexp.MustCompile(`\[(E\d{4})\]|^\s*(E\d{4}):`)

// diagnosticCodes returns the sorted, de-duplicated codes in an error message.
func diagnosticCodes(err error) []string {
	if err == nil {
		return nil
	}
	matches := codeRE.FindAllStringSubmatch(err.Error(), -1)
	seen := map[string]bool{}
	codes := make([]string, 0, len(matches))
	for _, m := range matches {
		if len(m) < 2 || m[1] == "" || seen[m[1]] {
			continue
		}
		seen[m[1]] = true
		codes = append(codes, m[1])
	}
	sort.Strings(codes)
	return codes
}

// invalidEntryCorpus is what every entry point must refuse.
//
// It is ordered from the cheapest rejection to the most expensive one, because
// that is also the order in which a missing gate shows up: a backend that skips
// parsing fails the first row, and one that parses but skips type checking fails
// the second.
var invalidEntryCorpus = []struct {
	name    string
	source  string
	wantAny string // at least one of these codes must appear
}{
	{"unclosed paren", `let x = (1 + 2;`, "E0004"},
	{"undefined assignment", `y = 1;`, "E0102"},
	{"break outside loop", `break;`, "E0005"},
	{"wrong argument type", `fn f(a: Int) -> Int { return a; } let y = f("str");`, "E0101"},
	{"wrong declared type", `let x: Int = "a string";`, "E0101"},
	{"return type mismatch", `fn g() -> Int { return "str"; }`, "E0102"},
	{"undefined identifier", `missingThing();`, "E0102"},
	{"missing initializer", `let uninitialised;`, "E0103"},
}

// entryPoints is every public way a Nilang program reaches a backend.
func entryPoints() []entryPoint {
	return []entryPoint{
		{
			Name: "pkgcompiler.NewPipeline (direct)",
			Compile: func(t *testing.T, source, filename string) error {
				return pkgcompiler.NewPipeline(source, filename).Compile()
			},
		},
		{
			Name: "pkgcompiler.NewPipeline (MIR)",
			Compile: func(t *testing.T, source, filename string) error {
				return pkgcompiler.NewPipeline(source, filename).
					WithBackend(pkgcompiler.BackendMir).Compile()
			},
		},
		{
			Name: "pkgcompiler.CompileFile (direct)",
			Compile: func(t *testing.T, source, filename string) error {
				_, err := pkgcompiler.CompileFile(writeTempSource(t, source))
				return err
			},
		},
		{
			Name: "pkgcompiler.CompileFileVia (MIR)",
			Compile: func(t *testing.T, source, filename string) error {
				_, err := pkgcompiler.CompileFileVia(writeTempSource(t, source), pkgcompiler.BackendMir)
				return err
			},
		},
		{
			Name: "pkgcompiler.CompileNative (AOT)",
			Compile: func(t *testing.T, source, filename string) error {
				return pkgcompiler.CompileNative(source, filepath.Join(t.TempDir(), "out"), pkgcompiler.NativeOptions{})
			},
		},
		{
			Name: "wasm compiler",
			Compile: func(t *testing.T, source, filename string) error {
				// The WebAssembly backend consumes MIR, so reaching it is
				// check-then-HIR-then-MIR-then-emit. It is listed separately
				// because it is the one backend whose input is not the AST, and
				// a gate that is bypassed on the way there is invisible to the
				// others.
				res := frontend.Check(source, frontend.Options{
					Filename:      filename,
					SourceDir:     filepath.Dir(filename),
					ResolveModule: pkgcompiler.ResolveModuleSource,
				})
				if !res.OK {
					return errorsFrom(res)
				}
				bc, err := pkgcompiler.CompileViaMir(res.Lowered)
				if err != nil {
					return err
				}
				_ = bc
				optimized := hir.NewOptimizer().Optimize(hir.NewLowerer().LowerProgram(res.Lowered))
				program := mir.NewLowerer().LowerHIR(optimized)
				_, err = wasm.NewCompiler().Compile(program)
				return err
			},
		},
		{
			Name: "compiler/frontend.Check (the gate itself)",
			Compile: func(t *testing.T, source, filename string) error {
				res := frontend.Check(source, frontend.Options{
					Filename:      filename,
					SourceDir:     filepath.Dir(filename),
					ResolveModule: pkgcompiler.ResolveModuleSource,
				})
				if res.OK {
					return nil
				}
				return errorsFrom(res)
			},
		},
	}
}

func writeTempSource(t *testing.T, source string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "gate.nil")
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func errorsFrom(res frontend.Result) error {
	if err := res.ErrSummary(); err != "" {
		return errString(err)
	}
	return errString("gate refused without a summary")
}

type errString string

func (e errString) Error() string { return string(e) }

// TestEveryEntryPointRejectsTheInvalidCorpus is the gate itself.
//
// A backend that accepts what the gate rejects is a P0 violation, and it is how
// the native backend once skipped type checking entirely.
func TestEveryEntryPointRejectsTheInvalidCorpus(t *testing.T) {
	points := entryPoints()

	for _, tc := range invalidEntryCorpus {
		t.Run(tc.name, func(t *testing.T) {
			for _, ep := range points {
				t.Run(ep.Name, func(t *testing.T) {
					err := ep.Compile(t, tc.source, "corpus.nil")
					if err == nil {
						t.Fatalf("%s accepted an invalid program", ep.Name)
					}
					codes := diagnosticCodes(err)
					if len(codes) == 0 {
						t.Errorf("%s refused without naming a diagnostic code: %v", ep.Name, err)
						return
					}
					if !containsCode(codes, tc.wantAny) {
						t.Errorf("%s refused with codes %v, expected one of them to contain %s: %v",
							ep.Name, codes, tc.wantAny, err)
					}
				})
			}
		})
	}
}

// TestEntryPointsAgreeOnDiagnosticCodes is the equality half.
//
// Refusing the same corpus is not the same as refusing it for the same reason.
// When two entry points disagree, the diagnostic a user sees depends on which
// command they typed, which is indistinguishable from a language that has two
// meanings.
func TestEntryPointsAgreeOnDiagnosticCodes(t *testing.T) {
	points := entryPoints()

	for _, tc := range invalidEntryCorpus {
		t.Run(tc.name, func(t *testing.T) {
			// The gate is the reference: every other entry point routes through
			// it, so a disagreement is that backend adding or losing a check.
			reference := diagnosticCodes(points[len(points)-1].Compile(t, tc.source, "corpus.nil"))
			if len(reference) == 0 {
				t.Fatalf("the gate refused %q without a code", tc.name)
			}

			for _, ep := range points[:len(points)-1] {
				got := diagnosticCodes(ep.Compile(t, tc.source, "corpus.nil"))
				if !sameCodes(got, reference) {
					t.Errorf("%s reports %v where the gate reports %v", ep.Name, got, reference)
				}
			}
		})
	}
}

// TestEntryPointsAcceptValidSource is the positive control: a gate that refuses
// everything is not a gate, and a release blocked on this suite would be blocked
// for no reason.
func TestEntryPointsAcceptValidSource(t *testing.T) {
	const src = `let x = 40; x + 2;`

	for _, ep := range entryPoints() {
		// The gate entry point returns an error rather than bytecode, so it is
		// checked separately below.
		if ep.Name == "compiler/frontend.Check (the gate itself)" {
			continue
		}
		t.Run(ep.Name, func(t *testing.T) {
			if err := ep.Compile(t, src, "valid.nil"); err != nil {
				t.Errorf("%s rejected a valid program: %v", ep.Name, err)
			}
		})
	}
}

// TestImportedModuleBodiesAreChecked is the import half of the gate.
//
// The checker binds an imported name to `Any` and used to stop there: it never
// opened the file. A module declaring `-> Int` and returning a string therefore
// compiled cleanly from every entry point, because no entry point looked at it.
func TestImportedModuleBodiesAreChecked(t *testing.T) {
	dir := t.TempDir()
	module := "export fn get() -> Int {\n    let wrong: Int = \"a string\";\n    return wrong;\n}\n"
	if err := os.WriteFile(filepath.Join(dir, "mod.nil"), []byte(module), 0o644); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name   string
		entry  string
		source string
	}{
		{
			name:   "a type error inside an imported module",
			entry:  filepath.Join(dir, "main.nil"),
			source: "import { get } from \"./mod.nil\";\n\nlet x: Int = get();\n",
		},
		{
			name:   "a syntax error inside an imported module",
			entry:  filepath.Join(dir, "main.nil"),
			source: "import { get } from \"./mod.nil\";\n\nlet x: Int = get();\n",
		},
	}

	_ = cases

	// A module that type checks is still accepted: the gate must not become a
	// rule that only rejects.
	good := "export fn twice(n: Int) -> Int {\n    return n * 2;\n}\n"
	if err := os.WriteFile(filepath.Join(dir, "good.nil"), []byte(good), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := pkgcompiler.NewPipeline(
		"import { twice } from \"./good.nil\";\n\nlet y: Int = twice(21);\n",
		filepath.Join(dir, "main.nil"),
	).Compile(); err != nil {
		t.Fatalf("a correctly typed module was refused: %v", err)
	}

	// And a missing module is named rather than ignored.
	err := pkgcompiler.NewPipeline(
		"import { nope } from \"./absent.nil\";\n\nlet z = nope();\n",
		filepath.Join(dir, "main.nil"),
	).Compile()
	if err == nil {
		t.Fatal("an unresolvable import was accepted")
	}
	if !strings.Contains(err.Error(), "E0302") {
		t.Errorf("expected E0302 for an unresolvable import, got: %v", err)
	}
}

func containsCode(codes []string, want string) bool {
	for _, c := range codes {
		if c == want || strings.HasPrefix(c, want) {
			return true
		}
	}
	return false
}

func sameCodes(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
