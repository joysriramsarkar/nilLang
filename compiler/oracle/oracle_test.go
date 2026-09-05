package oracle

import (
	"testing"
)

func TestCompilerOracleSymbols(t *testing.T) {
	o := NewCompilerOracle()

	typesList := o.ListTypes()
	if len(typesList) == 0 {
		t.Fatalf("Expected non-empty types list")
	}

	sym, found := o.FindSymbol("puts")
	if !found {
		t.Fatalf("Expected builtin 'puts' in oracle")
	}
	if sym.State != StateStable {
		t.Errorf("Expected puts to be STABLE, got %s", sym.State)
	}

	sig := o.GenerateSignature("puts")
	if sig == "" {
		t.Errorf("Expected signature for puts")
	}
}

func TestCheckExpression(t *testing.T) {
	o := NewCompilerOracle()

	res, err := o.CheckExpression("let x = 10 + 20;")
	if err != nil {
		t.Fatalf("CheckExpression error: %v", err)
	}
	if !res.Valid {
		t.Errorf("Expected valid expression check, got errors: %v", res.Diagnostics)
	}
}

func TestVerifiedNoveltyPipeline(t *testing.T) {
	o := NewCompilerOracle()
	engine := NewVerifiedNoveltyEngine(o)

	proposal := &NoveltyProposal{
		Name:         "QuickSort",
		Author:       "AI-Developer",
		SourceCode:   "let sort = fn(arr) { return arr; };",
		Tests:        "assert(true, \"ok\");",
		Capabilities: []string{"Network"},
		State:        StateExperimental,
	}

	report := engine.Verify(proposal)
	if report.FinalState != StateVerified {
		t.Fatalf("Expected proposal to be VERIFIED, got %s", report.FinalState)
	}
	if len(report.Stages) != 6 {
		t.Errorf("Expected 6 verification stages, got %d", len(report.Stages))
	}

	// Verify it registered in Oracle
	sym, found := o.FindSymbol("QuickSort")
	if !found || sym.State != StateVerified {
		t.Errorf("Expected QuickSort to be registered as VERIFIED in oracle")
	}
}
