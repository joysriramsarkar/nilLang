package mir

// Block merging and global value numbering.
//
// # What merging buys
//
// Every block the lowering creates ends with a jump, and every jump is a branch
// the VM must resolve at run time. A linear chain of `bb_then` → `bb_merge` →
// `bb_next` costs three block transitions to express what is three instructions.
//
// Merging a block into its single predecessor removes the transition, and with it
// the jump. This is the pass that turns MIR's readability into MIR's speed, and
// it is why `L3.6`'s gate is measured in per-cent rather than in instruction
// count: nothing is computed differently, fewer *transitions* happen.
//
// # What merging must not do
//
// A block with two predecessors cannot merge into either of them. Doing so would
// give the other predecessor a copy of the block's instructions, and the two copies
// would then have to agree on every temporary they share — which is precisely the
// aliasing this pass exists to avoid.

// OptimizeBlockMerging joins a block to its sole predecessor when the predecessor
// ends in an unconditional jump to it.
//
// It reports whether anything changed, so the driver can tell a converged program
// from one still being worked on.
// OptimizeBlockMerging joins a block into its sole predecessor when the
// predecessor ends in an unconditional jump to it.
//
// It reports whether anything changed, so the driver can tell a converged program
// from one still being worked on.
func OptimizeBlockMerging(fn *Function) bool {
	if fn == nil {
		return false
	}

	changed := false
	for _, block := range fn.Blocks {
		if block == nil || len(block.Instructions) == 0 {
			continue
		}
		pred := solePredecessor(fn, block)
		if pred == nil {
			continue
		}

		// The predecessor's jump has to land on this block *specifically*. A
		// predecessor that reaches this block only through a branch is not a
		// merge candidate even when it is the sole one: folding a branch target
		// into its predecessor would delete the branch that chose between them.
		jump, isJump := pred.Terminator.(JumpTerminator)
		if !isJump || jump.Target != block.ID {
			continue
		}

		pred.Instructions = append(pred.Instructions, block.Instructions...)
		pred.Terminator = block.Terminator
		block.Instructions = nil
		block.Terminator = nil
		changed = true
	}
	return changed
}

// solePredecessor is the one block that jumps to `target`, or nil when it has
// none or more than one.
//
// # Branch targets count too
//
// A block reached by a `branch` has as many predecessors as the branch has arms
// that point at it. Ignoring branch edges entirely would make every loop's exit
// block look unreachable from the CFG's point of view, and merging the *entry*
// block into the loop header would then be attempted on a block that the loop
// also reaches — producing two copies of the header's instructions and two
// definitions of the same temporary.
//
// So both edges are counted. A block with one predecessor that ends in a jump can
// merge; a block with two, or one that is entered by a branch, cannot.
func solePredecessor(fn *Function, target *BasicBlock) *BasicBlock {
	var found *BasicBlock
	for _, block := range fn.Blocks {
		if block == nil || block == target {
			continue
		}

		arrives := false
		switch term := block.Terminator.(type) {
		case JumpTerminator:
			arrives = term.Target == target.ID
		case BranchTerminator:
			arrives = term.TrueTarget == target.ID || term.FalseTarget == target.ID
		}
		if !arrives {
			continue
		}

		if found != nil {
			return nil
		}
		found = block
	}
	return found
}

// OptimizeGVN replaces a repeated computation of the same value with the
// temporary that already holds it.
//
// # Scope, and why it stops where it does
//
// The key is the *whole* instruction — operator, both operands, the callee — so
// two operations match only when they are the same operation on the same values.
// The operand names are compared, not the values behind them, which is what makes
// this sound: a temporary's value is fixed once it is defined, so "same name" is
// "same value".
//
// It runs per block and does not cross block boundaries. A value computed in one
// arm of a branch is not available in the other, and a global table would need a
// dominance check to know the difference. Per-block still removes the common case
// — a repeated computation within one expression or one statement — and never
// guesses.
//
// Nothing is eliminated here. The first computation stays and later uses name it,
// so the redundancy goes without any risk of removing something that had effects.
func OptimizeGVN(fn *Function) bool {
	if fn == nil {
		return false
	}

	changed := false
	for _, block := range fn.Blocks {
		if block == nil {
			continue
		}
		if eliminateRedundantComputation(block) {
			changed = true
		}
	}
	return changed
}

func eliminateRedundantComputation(block *BasicBlock) bool {
	// computed maps an instruction's shape to the temporary that already holds
	// its result, and seen marks the *first* instruction with that shape.
	computed := make(map[string]Operand)
	firstSeen := make(map[string]bool)

	changed := false
	for idx, inst := range block.Instructions {
		key := instructionKey(inst)

		// The replacement has to have been computed *strictly earlier* in this
		// block. Recording before checking would make a computation its own
		// replacement: `_t2 = _t2` computes the same value as itself, so the
		// instruction's own destination would be waiting for it. That is not a
		// no-op — it is an uninitialised read, and the program fails at run time
		// with a nil operand inside an operation that looks perfectly ordinary.
		//
		// `sameOperand` guards the same thing from the other side, for the case
		// where a block genuinely computes the same shape twice into the *same*
		// temporary.
		if existing, already := computed[key]; already && firstSeen[key] {
			if dest := getDestOperand(inst); dest != nil && !sameOperand(dest, existing) {
				// A second computation of the same value becomes a copy of the
				// first result. It is not deleted: deleting it would be dead code
				// elimination's decision, and this pass has not established that the
				// computation was free.
				block.Instructions[idx] = AssignInst{Dest: dest, Src: existing}
				changed = true
				continue
			}
		}

		if dest := getDestOperand(inst); dest != nil {
			computed[key] = dest
			firstSeen[key] = true
		}
	}
	return changed
}

// sameOperand reports whether two operands name the same thing.
func sameOperand(a, b Operand) bool {
	if a == nil || b == nil {
		return false
	}
	return a.String() == b.String()
}

// instructionKey is the shape of an instruction: everything that decides what it
// computes.
//
// It returns "" for an instruction with no destination, which cannot be a
// candidate for reuse in the first place.
func instructionKey(inst Instruction) string {
	switch i := inst.(type) {
	case AssignInst:
		return "assign|" + operandKey(i.Src)
	case BinaryOpInst:
		return "bin|" + i.Op + "|" + operandKey(i.Left) + "|" + operandKey(i.Right)
	case UnaryOpInst:
		return "unary|" + i.Op + "|" + operandKey(i.Right)
	case LoadVarInst:
		return "load|" + i.Name
	case CallInst:
		key := "call|" + i.Callee
		for _, arg := range i.Args {
			key += "|" + operandKey(arg)
		}
		return key
	case AggregateInst:
		key := "agg"
		if i.Hash {
			key = "hash"
		}
		for _, e := range i.Elems {
			key += "|" + operandKey(e)
		}
		return key
	}
	return ""
}

// operandKey names an operand for comparison.
func operandKey(op Operand) string {
	switch v := op.(type) {
	case TempOperand:
		return "t" + v.String()
	case VarOperand:
		return "v" + v.Name
	case ConstOperand:
		return "c" + v.String()
	}
	return "?"
}
