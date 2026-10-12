package mir

// OptimizeDeadCodeElimination removes unreachable basic blocks and dead instructions
func OptimizeDeadCodeElimination(fn *Function) bool {
	changed := false

	// Step 1: Remove unreachable basic blocks using CFG analysis
	cfg := BuildCFG(fn)
	reachable := cfg.ReachableBlocks()

	if len(reachable) < len(fn.Blocks) {
		newBlocks := make([]*BasicBlock, 0, len(reachable))
		for _, bb := range fn.Blocks {
			if reachable[bb.ID] {
				newBlocks = append(newBlocks, bb)
			} else {
				changed = true
			}
		}
		fn.Blocks = newBlocks
	}

	// Step 2: Identify used operands across the function
	usedOperands := make(map[string]bool)
	for _, bb := range fn.Blocks {
		for _, inst := range bb.Instructions {
			recordUsedOperands(inst, usedOperands)
		}
		if bb.Terminator != nil {
			recordTerminatorUsedOperands(bb.Terminator, usedOperands)
		}
	}

	// Step 3: Prune instructions whose destination temporary is never read
	for _, bb := range fn.Blocks {
		newInsts := make([]Instruction, 0, len(bb.Instructions))
		for _, inst := range bb.Instructions {
			dest := getDestOperand(inst)
			if temp, isTemp := dest.(TempOperand); isTemp {
				if !usedOperands[temp.String()] {
					// Unused temporary, safe to prune if side-effect free
					if isSideEffectFree(inst) {
						changed = true
						continue
					}
				}
			}
			newInsts = append(newInsts, inst)
		}
		bb.Instructions = newInsts
	}

	return changed
}

// recordUsedOperands marks every value an instruction reads.
//
// # Why the list is exhaustive by hand
//
// Dead code elimination asks "does anything read this?" and answers "no" for
// anything it does not recognise reading. That default is wrong in a way that
// does not announce itself: an instruction the switch has never heard of is
// treated as reading *nothing*, so every temporary it consumes is declared dead
// and deleted — while the instruction itself survives, because deleting *it* would
// need the effects check.
//
// The result is a program that is internally consistent and answers wrongly.
// `SelectInst` and `AggregateInst` were both missing from this switch when they
// were added, and the symptom was a `??` whose guard had vanished, leaving a read
// of a slot nothing had written.
//
// So a new instruction with operands must be added here. There is a test for that
// claim rather than a note: `TestDeadCodeEliminationKeepsEveryOperandKind`.
func recordUsedOperands(inst Instruction, used map[string]bool) {
	switch i := inst.(type) {
	case AssignInst:
		mark(used, i.Src)
	case StoreVarInst:
		mark(used, i.Src)
	case BinaryOpInst:
		mark(used, i.Left)
		mark(used, i.Right)
	case UnaryOpInst:
		mark(used, i.Right)
	case CallInst:
		for _, arg := range i.Args {
			mark(used, arg)
		}
	case AggregateInst:
		for _, elem := range i.Elems {
			mark(used, elem)
		}
	case SelectInst:
		// All three, and the condition most of all: it is what decides which arm
		// runs, so dropping it changes the answer rather than just the timing.
		mark(used, i.Cond)
		mark(used, i.True)
		mark(used, i.False)
	}
}

// mark records one operand as read, ignoring an absent one.
//
// An absent operand is not a read of the empty name, which would mark a variable
// called "" alive and nothing else — a mistake that costs nothing and confuses
// whoever reads the output next.
func mark(used map[string]bool, op Operand) {
	if op == nil {
		return
	}
	used[op.String()] = true
}

func recordTerminatorUsedOperands(term Terminator, used map[string]bool) {
	switch t := term.(type) {
	case ReturnTerminator:
		if t.Value != nil {
			used[t.Value.String()] = true
		}
	case BranchTerminator:
		used[t.Cond.String()] = true
	}
}

func getDestOperand(inst Instruction) Operand {
	switch i := inst.(type) {
	case AssignInst:
		return i.Dest
	case BinaryOpInst:
		return i.Dest
	case UnaryOpInst:
		return i.Dest
	case LoadVarInst:
		return i.Dest
	case CallInst:
		return i.Dest
	case AggregateInst:
		return i.Dest
	case SelectInst:
		return i.Dest
	}
	return nil
}

func isSideEffectFree(inst Instruction) bool {
	switch inst.(type) {
	case CallInst:
		return false // Calls may have side effects
	case StoreVarInst:
		return false // Storing variables alters state
	default:
		// Building a list or a hash allocates and can therefore fail; reading a
		// field on a null receiver fails. Neither is removable on the grounds
		// that it is "just an expression" — a pass that assumed so would delete
		// the allocation whose failure the program relied on being told about.
		//
		// They are side-effect free with respect to *state*, which is what
		// dead-code elimination here is allowed to reason about. Error behaviour is
		// a separate question, and leaving these in place costs nothing that the
		// hot path depends on.
		return true
	}
}
