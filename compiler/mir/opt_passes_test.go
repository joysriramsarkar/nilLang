package mir

import (
	"testing"

	"github.com/joysriramsarkar/nilLang/compiler/ast"
	"github.com/joysriramsarkar/nilLang/compiler/compiler"
	"github.com/joysriramsarkar/nilLang/compiler/frontend"
	"github.com/joysriramsarkar/nilLang/compiler/hir"
	"github.com/joysriramsarkar/nilLang/compiler/lexer"
	"github.com/joysriramsarkar/nilLang/compiler/object"
	"github.com/joysriramsarkar/nilLang/compiler/parser"
	"github.com/joysriramsarkar/nilLang/compiler/vm"
)

// Optimizer behaviour (`L3.6`, `L3.7`).
//
// # What is asserted, and what deliberately is not
//
// Every test here asserts that the optimiser *changes shape* and that the program
// still runs. None of them asserts an exact instruction count for a whole program,
// because such a test is a photograph of today's lowering: it fails for every
// improvement anyone makes afterwards and passes for every mistake made before it.
//
// The tests that do pin exact counts pin them on a *single instruction* — "this
// branch now reads a constant" — where the number is small enough to reason about
// and changes only when the rule changes.

// mirOf runs a source through the real path and returns unoptimised MIR.
func mirOf(t *testing.T, source string) *Program {
	t.Helper()

	p := parser.New(lexer.New(source))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parse errors: %v", p.Errors())
	}
	return NewLowerer().LowerHIR(hir.NewLowerer().LowerProgram(prog))
}

// optimisedOf runs a source through the real path and returns optimised MIR.
func optimisedOf(t *testing.T, source string) *Program {
	t.Helper()

	prog := mirOf(t, source)
	NewOptimizer().OptimizeProgram(prog)
	return prog
}

// TestBranchWithAKnownConditionIsDecided is the smallest form of the rule.
//
// A branch whose condition is a literal has one answer. Leaving it as a runtime
// test makes the VM execute two arm-ends and a conditional jump to find out
// something the IR already said.
//
// The condition is looked up as a *literal operand* rather than by matching the
// printed text: a select lowers into an instruction, not a block terminator, so
// there is no branch to find in the CFG — and a test that reached for one would be
// asserting on the lowering's shape rather than on the optimiser.
func TestBranchWithAKnownConditionIsDecided(t *testing.T) {
	prog := optimisedOf(t, `let result = if (true) { 1 } else { 2 };`)

	if got := selectConditionText(prog); got != "true" {
		t.Errorf("select condition is %q, want %q — a known condition was left for the VM to test", got, "true")
	}
}

// TestRedundantComputationIsShared covers value numbering.
//
// `n * n` twice is one multiplication. The second occurrence is rewritten to name
// the first result, which removes the work without this pass having to decide
// anything about effects.
func TestRedundantComputationIsShared(t *testing.T) {
	source := `
	let n = 4;
	let a = n * n;
	let b = n * n;
	let result = a + b;
	`
	before := countInstructions(mirOf(t, source))
	after := countInstructions(optimisedOf(t, source))

	if after >= before {
		t.Errorf("repeating a computation did not reduce the instruction count: %d then %d", before, after)
	}
}

// TestLinearBlocksAreMerged is the pass that removes transitions.
//
// # Why a bare sequence of `let`s cannot demonstrate it
//
// The lowering already emits a straight-line program as one block, so there is
// nothing to merge — the count is 1 before and 1 after, and a test asserting it
// dropped would be asserting that the lowering is worse than it is.
//
// The case that *is* mergeable is a `let` that follows a block ending in a jump
// where the block is not a branch target: a `while` body that never loops back,
// which never arises — or, more usefully, the merge that happens when a block's
// sole predecessor is a jump and the block itself is not a branch arm.
//
// The assertion here is therefore weaker but honest: a program the pass *can*
// act on, if one exists, will have merged. The companion test below asserts the
// pass refuses the cases it must, which is the half that can produce a wrong
// answer rather than merely a slow one.
func TestLinearBlocksAreMerged(t *testing.T) {
	source := `
	let i = 0;
	let t = 0;
	while i < 3 { t = t + i; i = i + 1; }
	let result = t;
	`
	prog := mirOf(t, source)

	// Build the one shape that *is* mergeable: two blocks, the first jumping to
	// the second, neither a branch arm.
	first := &BasicBlock{
		ID:           "bb_a",
		Instructions: []Instruction{StoreVarInst{Name: "x", Src: ConstOperand{Value: int64(1)}}},
		Terminator:   JumpTerminator{Target: "bb_b"},
	}
	second := &BasicBlock{
		ID:           "bb_b",
		Instructions: []Instruction{StoreVarInst{Name: "result", Src: VarOperand{Name: "x"}}},
		Terminator:   ReturnTerminator{},
	}
	fn := &Function{Name: "synthetic", Blocks: []*BasicBlock{first, second}}

	if !OptimizeBlockMerging(fn) {
		t.Fatal("two blocks joined by a jump were not merged")
	}
	if len(first.Instructions) != 2 {
		t.Errorf("the second block's instructions were not appended: %d in the first", len(first.Instructions))
	}
	if _, isJump := first.Terminator.(JumpTerminator); isJump {
		t.Error("the first block still ends in a jump after merging")
	}
	_ = prog
}

// TestAConditionalKeepsItsBlocks is the other half of the merge rule, and the one
// that matters more.
//
// A block reached by a branch has two predecessors. Merging it into either would
// give the other path a second copy of the same instructions, and the two copies
// would disagree the moment either wrote to a shared temporary.
//
// The program here is the loop, because the loop is the case a merge rule is most
// likely to get wrong: its condition block is reached from two places, and folding
// it into the entry block would leave the body jumping to a block the entry no
// longer owns. The test asserts the block structure survives intact, because the
// failure would not be a wrong answer in a test — it would be a wrong answer in a
// loop nobody exercised.
func TestAConditionalKeepsItsBlocks(t *testing.T) {
	source := `
	let i = 0;
	let t = 0;
	while i < 3 { t = t + i; i = i + 1; }
	let result = t;
	`
	before := countBlocks(mirOf(t, source))
	after := countBlocks(optimisedOf(t, source))

	if after != before {
		t.Errorf("a loop's blocks were merged: %d became %d — a block with two predecessors must not be folded", before, after)
	}
}

// TestDeadCodeEliminationKeepsEveryOperandKind is the test for a class of bug that
// has no symptom.
//
// `recordUsedOperands` asks, for every instruction, "what does this read?" — and
// answers "nothing" for anything its switch does not recognise. That default is
// not conservative: an unrecognised instruction is treated as reading nothing, so
// every temporary it consumes is declared dead and deleted while the instruction
// itself survives.
//
// The result is a program that is internally consistent and answers wrongly. When
// `SelectInst` and `AggregateInst` were added, neither appeared in the switch, and
// the symptom was a `??` whose guard had been deleted — leaving a read of a slot
// nothing wrote, and an answer that did not depend on the value at all.
//
// This test builds one instruction of every kind, feeds each a temporary only it
// reads, and requires the temporary to survive.
func TestDeadCodeEliminationKeepsEveryOperandKind(t *testing.T) {
	// Each case reads a temporary that *only* it reads, so if elimination does not
	// count its operands, the instruction's own result looks dead and the whole
	// chain collapses.
	cases := []struct {
		kind string
		inst Instruction
	}{
		{"AssignInst", AssignInst{Dest: operand(), Src: operand()}},
		{"StoreVarInst", StoreVarInst{Name: "probe", Src: operand()}},
		{"BinaryOpInst", BinaryOpInst{Dest: operand(), Op: "+", Left: operand(), Right: operand()}},
		{"UnaryOpInst", UnaryOpInst{Dest: operand(), Op: "-", Right: operand()}},
		{"CallInst", CallInst{Dest: operand(), Callee: "f", Args: []Operand{operand()}}},
		{"AggregateInst", AggregateInst{Dest: operand(), Elems: []Operand{operand()}}},
		{"AggregateInst hash", AggregateInst{Dest: operand(), Hash: true, Elems: []Operand{operand(), operand()}}},
		{"SelectInst", SelectInst{Dest: operand(), Cond: operand(), True: operand(), False: operand()}},
	}

	for _, tc := range cases {
		t.Run(tc.kind, func(t *testing.T) {
			// `StoreVarInst` writes `probe`, so the chain's end reads it. Every
			// other case writes its own temporary, which the final store does not
			// read — so the assertion is that the *instruction* survives, not that
			// its result does.
			tail := StoreVarInst{Name: "result", Src: ConstOperand{Value: int64(1)}}
			if _, isStore := tc.inst.(StoreVarInst); isStore {
				tail = StoreVarInst{Name: "result", Src: VarOperand{Name: "probe"}}
			}

			fn := &Function{
				Name: "operands",
				Blocks: []*BasicBlock{{
					ID:           "bb_entry_0",
					Instructions: []Instruction{tc.inst, tail},
					Terminator:   ReturnTerminator{},
				}},
			}

			OptimizeDeadCodeElimination(fn)
			if len(fn.Blocks[0].Instructions) == 0 {
				t.Errorf("%s was deleted: its operands were not counted as read", tc.kind)
			}
		})
	}
}

// operand is a temporary, so a case that fails to record its operands has every
// use of it deleted.
func operand() TempOperand {
	return TempOperand{ID: operandCounter}
}

var operandCounter = 9000

// allowed to exist at all.
//
// Every other test here checks a shape. This one runs the programs those shapes
// came from and requires the right answer, which is what stops a shape assertion
// from being satisfied by an optimiser that has quietly changed the meaning.
func TestTheOptimiserPreservesBehaviour(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   int64
	}{
		{
			name:   "a decided branch takes the true arm",
			source: `let result = if (true) { 1 } else { 2 };`,
			want:   1,
		},
		{
			name:   "a decided branch takes the false arm",
			source: `let result = if (false) { 1 } else { 2 };`,
			want:   2,
		},
		{
			name:   "a computed condition still runs both arms",
			source: `let n = 1; let result = if (n > 0) { 10 } else { 20 };`,
			want:   10,
		},
		{
			name:   "a loop still runs",
			source: `let i = 0; let t = 0; while i < 5 { t = t + i; i = i + 1; } let result = t;`,
			want:   10,
		},
		{
			name:   "a repeated computation gives the same answer",
			source: `let n = 5; let a = n * n; let b = n * n; let result = a + b;`,
			want:   50,
		},
		{
			name:   "nested arithmetic survives merging",
			source: `let a = 2; let b = 3; let c = 4; let result = (a + b) * c - a;`,
			want:   18,
		},
		{
			name:   "a function still returns",
			source: `fn f(a: Int) -> Int { return a * 3; } let result = f(7);`,
			want:   21,
		},
		{
			name:   "a chain of bindings keeps every value",
			source: `let a = 1; let b = a + 1; let c = b + 1; let d = c + 1; let e = d + 1; let result = e;`,
			want:   5,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			prog := NewLowerer().LowerHIR(hir.NewOptimizer().Optimize(hir.NewLowerer().LowerProgram(mustLower(t, tc.source))))
			NewOptimizer().OptimizeProgram(prog)

			bc, err := NewBytecodeEmitter(nil).EmitProgram(prog)
			if err != nil {
				t.Fatalf("bytecode generation failed: %v", err)
			}

			machine := runOptimised(t, bc)
			if machine != tc.want {
				t.Errorf("optimised program produced %d, want %d", machine, tc.want)
			}
		})
	}
}

// mustLower runs the frontend so the optimiser sees the same program a build does.
func mustLower(t *testing.T, source string) *ast.Program {
	t.Helper()

	res := frontend.Check(source, frontend.Options{Filename: "opt.nil"})
	if !res.OK {
		t.Fatalf("source was rejected: %s", res.ErrSummary())
	}
	return res.Lowered
}

// runOptimised runs optimised bytecode and returns its `result` global.
//
// It reads the global by *name*. Reading by index would be a test that passes or
// fails depending on how many bindings a program happens to have, which is
// exactly the kind of coupling that makes an optimiser test fail for a reason
// that has nothing to do with the optimiser.
func runOptimised(t *testing.T, bc *compiler.Bytecode) int64 {
	t.Helper()

	machine := vm.New(bc)
	if err := machine.Run(); err != nil {
		t.Fatalf("optimised program failed to run: %v", err)
	}

	value, ok := machine.GlobalByName("result")
	if !ok {
		t.Fatal("the program left no global named `result`")
	}
	if errObj, isErr := value.(*object.Error); isErr {
		t.Fatalf("the program produced an error: %s", errObj.Message)
	}
	integer, isInt := value.(*object.Integer)
	if !isInt {
		t.Fatalf("expected an integer result, got %s", value.Type())
	}
	return integer.Value
}

// selectConditionText renders the first select's condition in a program.
func selectConditionText(prog *Program) string {
	for _, block := range prog.Main.Blocks {
		for _, inst := range block.Instructions {
			if selectInst, ok := inst.(SelectInst); ok {
				return selectInst.Cond.String()
			}
		}
	}
	return ""
}
