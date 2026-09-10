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

// OptimizeFunction runs constant folding, propagation, and dead code elimination
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

		// Pass 2: Dead code & unreachable block elimination
		if OptimizeDeadCodeElimination(fn) {
			changed = true
		}

		if !changed {
			break
		}
	}
}
