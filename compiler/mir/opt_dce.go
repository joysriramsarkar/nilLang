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

func recordUsedOperands(inst Instruction, used map[string]bool) {
	switch i := inst.(type) {
	case AssignInst:
		used[i.Src.String()] = true
	case StoreVarInst:
		used[i.Src.String()] = true
	case BinaryOpInst:
		used[i.Left.String()] = true
		used[i.Right.String()] = true
	case UnaryOpInst:
		used[i.Right.String()] = true
	case CallInst:
		for _, arg := range i.Args {
			used[arg.String()] = true
		}
	}
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
		return true
	}
}
