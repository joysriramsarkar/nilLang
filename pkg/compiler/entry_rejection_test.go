package compiler_test

// TestPublicEntriesRejectInvalidCorpus — P0.2 (docs/CORE_STABILIZATION.md).
//
// Every public compilation entry point routes through compiler/frontend, so
// every one of them must reject the same invalid corpus. A backend that
// accepts what the gate rejects is a P0 violation (this is how the native
// backend once skipped type checking entirely).

import (
	"os"
	"path/filepath"
	"testing"

	pkgcompiler "github.com/joysriramsarkar/nilLang/pkg/compiler"
)

var invalidEntryCorpus = []struct{ name, source string }{
	{"syntax error", `let x = (1 + 2;`},
	{"undefined assignment", `y = 1;`},
	{"break outside loop", `break;`},
}

func TestPublicEntriesRejectInvalidCorpus(t *testing.T) {
	for _, tc := range invalidEntryCorpus {
		t.Run(tc.name, func(t *testing.T) {
			if err := pkgcompiler.NewPipeline(tc.source, "corpus.nil").Compile(); err == nil {
				t.Error("direct pipeline accepted invalid source")
			}
			if err := pkgcompiler.NewPipeline(tc.source, "corpus.nil").WithBackend(pkgcompiler.BackendMir).Compile(); err == nil {
				t.Error("MIR pipeline accepted invalid source")
			}

			dir := t.TempDir()
			mainPath := filepath.Join(dir, "main.nil")
			if err := os.WriteFile(mainPath, []byte(tc.source), 0644); err != nil {
				t.Fatal(err)
			}
			if _, err := pkgcompiler.CompileFile(mainPath); err == nil {
				t.Error("CompileFile accepted invalid source")
			}
			if _, err := pkgcompiler.CompileFileVia(mainPath, pkgcompiler.BackendMir); err == nil {
				t.Error("CompileFileVia(MIR) accepted invalid source")
			}

			out := filepath.Join(dir, "out")
			if err := pkgcompiler.CompileNative(tc.source, out, pkgcompiler.NativeOptions{}); err == nil {
				t.Error("CompileNative accepted invalid source")
			}
		})
	}
}

// TestPublicEntriesAcceptValidSource is the positive control: the gate must
// not reject a well-typed program on any backend.
func TestPublicEntriesAcceptValidSource(t *testing.T) {
	const src = `let x = 40; x + 2;`
	if err := pkgcompiler.NewPipeline(src, "ok.nil").Compile(); err != nil {
		t.Fatalf("direct pipeline rejected valid source: %v", err)
	}
	if err := pkgcompiler.NewPipeline(src, "ok.nil").WithBackend(pkgcompiler.BackendMir).Compile(); err != nil {
		t.Fatalf("MIR pipeline rejected valid source: %v", err)
	}
}
