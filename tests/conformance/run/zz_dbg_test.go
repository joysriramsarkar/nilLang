package run

import (
	"fmt"
	"testing"

	"github.com/joysriramsarkar/nilLang/compiler/frontend"
	"github.com/joysriramsarkar/nilLang/compiler/hir"
	"github.com/joysriramsarkar/nilLang/compiler/llvm"
	"github.com/joysriramsarkar/nilLang/compiler/mir"
)

func TestDbgStr(t *testing.T) {
	source := `
		extern "C" fn nilffi_byte_length(bytes: String) -> Int;
		nilffi_byte_length("hello");
	`
	res := frontend.Check(source, frontend.Options{})
	if !res.OK {
		t.Fatalf("rejected: %s", res.ErrSummary())
	}
	m := mir.NewLowerer().LowerHIR(hir.NewOptimizer().Optimize(hir.NewLowerer().LowerProgram(res.Lowered)))
	ir, err := llvm.NewEmitter(llvm.TargetHost).Emit(m)
	if err != nil {
		t.Fatalf("emit: %v", err)
	}
	fmt.Println(ir)
}
