package mir

// Optimizer runs semantic-preserving optimization passes on MIR
type Optimizer struct {
	MaxIterations int
}

func NewOptimizer() *Optimizer {
	return &Optimizer{MaxIterations: 10}
}

// OptimizeProgram optimizes all functions in the MIR program
func (opt *Optimizer) OptimizeProgram(prog *Program) {
	if prog == nil {
		return
	}
	if prog.Main != nil {
		opt.OptimizeFunction(prog.Main)
	}
	for _, fn := range prog.Functions {
		opt.OptimizeFunction(fn)
	}
}

// OptimizeFunction runs constant folding, branch simplification, and dead code
// elimination to a fixed point.
//
// The order matters more than it looks. Folding first means simplification sees
// the constants folding produced, so a branch that has just become decidable is
// decided in the same round rather than the next. Deduction last means it removes
// the blocks simplification made unreachable.
//
// The loop is what makes the passes compose: copy propagation can expose a fold,
// a fold can expose a branch, and a decided branch can make a block dead. Running
// each once would leave whichever opportunity happened to come second.
func (opt *Optimizer) OptimizeFunction(fn *Function) {
	if fn == nil {
		return
	}

	for i := 0; i < opt.MaxIterations; i++ {
		changed := false

		// Pass 1: Constant folding & propagation
		if OptimizeConstantFolding(fn) {
			changed = true
		}

		// Pass 2: Branch simplification & copy propagation
		if OptimizeBranchSimplification(fn) {
			changed = true
		}

		// Pass 3: Redundant-computation elimination
		//
		// After copy propagation, not before: value numbering compares
		// instructions by their operands, so a `load` that propagation has already
		// replaced with the variable it read is no longer *shaped* like the load
		// beside it — and the two would not match even though they compute the same
		// value. Running it first also lets it rewrite a load into a copy of
		// itself, because the operand it compares has not yet been resolved.
		if OptimizeGVN(fn) {
			changed = true
		}

		// Pass 4: Block merging
		//
		// Last among the block-level passes, because merging consumes the
		// transitions the earlier passes made unnecessary — merging first would
		// spend the pass joining blocks that a later fold emptied anyway.
		if OptimizeBlockMerging(fn) {
			changed = true
		}

		// Pass 5: Dead code & unreachable block elimination
		if OptimizeDeadCodeElimination(fn) {
			changed = true
		}

		if !changed {
			break
		}
	}
}
