package mir

// Branch simplification and copy propagation.
//
// # Why these belong together
//
// Both ask the same question of an instruction: *is this value already known?*
//
// One answers "yes, it is the same as another value" and substitutes it; the other
// answers "yes, and it is a constant, so the branch can be decided now". Folding a
// branch but not propagating the copy that made it foldable — or the reverse —
// leaves one of the two cases running twice as often as it needs to, and the pair
// reaches a fixed point in half the iterations.
//
// # What is deliberately *not* done here
//
// Neither pass removes an instruction whose destination is unused. That is dead
// code elimination's job, and it is the one pass that must know whether an
// instruction has effects. This file only rewrites operands and terminators.

// OptimizeBranchSimplification rewrites branches and copies whose value is already
// known, and reports whether anything changed.
func OptimizeBranchSimplification(fn *Function) bool {
	if fn == nil {
		return false
	}

	changed := false
	// A copy map is rebuilt per block. A `Dest` defined in one block cannot be
	// substituted into another, because control may reach the use without passing
	// through the definition — and a substitution that assumed otherwise would be
	// a wrong answer rather than a slow one.
	//
	// Rebuilding per block also keeps the pass correct in the presence of loops, where
	// a value defined in the entry block may be overwritten before a use.
	for _, block := range fn.Blocks {
		if block == nil {
			continue
		}
		if simplifyBlock(block) {
			changed = true
		}
	}
	return changed
}

// simplifyBlock runs both rewrites over one block.
func simplifyBlock(block *BasicBlock) bool {
	changed := false
	copies := copyMapFor(block)

	for idx, inst := range block.Instructions {
		// A use is rewritten with what was known *before* this instruction, and
		// the instruction then joins the map.
		//
		// Doing both in the other order lets an instruction rewrite its own
		// destination to a value it is supposed to produce: `_t1 = load a` reads
		// `a`, and adding `_t1 = a` to the map *first* means the same instruction's
		// own operand becomes `_t1` — a read of the uninitialised slot it was about
		// to write. The MIR reads `_t2 = _t1 ? 10 : 20` with `_t1` nowhere defined,
		// and the program answers with the wrong arm.
		if replaced, ok := substituteCopies(inst, copies); ok {
			block.Instructions[idx] = replaced
			inst = replaced
			changed = true
		}

		if dest, src, isCopy := asCopy(inst); isCopy {
			if known, already := copies[operandName(src)]; already {
				copies[dest] = known
			} else {
				copies[dest] = src
			}
			continue
		}

		// Writing to a variable invalidates every fact known about it. Without
		// this, `store a, 2` after `_t0 = load a` leaves `_t0 = a` in the map, and
		// a later use of `_t0` is rewritten to read `a` — which now holds 2, not
		// the value `_t0` was given. The MIR is self-consistent and the answer is
		// wrong, which is the worst of the two failure modes.
		if store, isStore := inst.(StoreVarInst); isStore {
			delete(copies, store.Name)
		}
	}

	if block.Terminator != nil {
		if resolved, ok := simplifyTerminator(block.Terminator, copies); ok {
			block.Terminator = resolved
			changed = true
		}
	}

	return changed
}

// copyMapFor is the set of values already known at the top of a block.
//
// Only two things can be known before the block runs: a constant operand written
// directly, and a `VarOperand` — a function parameter or a global, whose value at
// the start of a block is whatever the caller passed and which the block itself
// has not yet changed.
func copyMapFor(block *BasicBlock) map[string]Operand {
	copies := make(map[string]Operand)
	for _, inst := range block.Instructions {
		if dest, src, isCopy := asCopy(inst); isCopy {
			copies[dest] = src
		}
	}
	// Collapse chains: `_t2 = _t1; _t1 = _t0` must leave `_t2` pointing at `_t0`,
	// not at `_t1`, or the substitution would rewrite a use to another temp that
	// the caller then has to rewrite again.
	for name, target := range copies {
		if next, alsoKnown := copies[operandName(target)]; alsoKnown {
			copies[name] = next
		}
	}
	return copies
}

// asCopy reports whether an instruction is `_dest = <something already named>`
// and nothing else.
//
// It returns the destination *name* and the operand it names, because the copy
// map is keyed by name — an operand is what a use will be rewritten to, and a use
// finds its way through `operandName`.
func asCopy(inst Instruction) (dest string, src Operand, ok bool) {
	switch i := inst.(type) {
	case AssignInst:
		d, isDest := i.Dest.(TempOperand)
		s, isSrc := i.Src.(TempOperand)
		if isDest && isSrc && d.ID != s.ID {
			return d.String(), s, true
		}
	case SelectInst:
		// A select is a copy when *both* arms are the condition's own source —
		// which is what a coalesce lowers to: `_t2 = _t1 ? _t1 : _t3` is not a
		// copy, but `_t2 = _t1 ? _t1 : _t1` is, and treating a one-armed select as
		// a copy would substitute the condition for the arm and lose the whole
		// point of the select.
		//
		// Recognising this matters: without it, a `??` whose fallback has been
		// propagated away leaves its condition unreadable, and the value the guard
		// produced is read from a slot nothing wrote.
		d, isDest := i.Dest.(TempOperand)
		s, isSrc := i.Cond.(TempOperand)
		if isDest && isSrc && d.ID != s.ID {
			t, trueIsCond := i.True.(TempOperand)
			f, falseIsCond := i.False.(TempOperand)
			if trueIsCond && falseIsCond && t.ID == s.ID && f.ID == s.ID {
				return d.String(), s, true
			}
		}
	}
	return "", nil, false
}

// substituteCopies rewrites every operand of an instruction that names a known
// value.
//
// It returns the rewritten instruction and whether anything changed, so the caller
// can tell an untouched instruction from one that happened to be identical.
func substituteCopies(inst Instruction, copies map[string]Operand) (Instruction, bool) {
	if len(copies) == 0 {
		return inst, false
	}

	rewritten, changed := rewriteOperands(inst, func(op Operand) (Operand, bool) {
		name := operandName(op)
		if name == "" {
			return op, false
		}
		replacement, known := copies[name]
		if !known || replacement == op {
			return op, false
		}
		return replacement, true
	})
	return rewritten, changed
}

// simplifyTerminator decides a branch whose condition is already known.
//
// `branch <const> ? a : b` has one reachable arm. Both arms are kept rather than
// collapsed, because deciding that the unreachable one is dead is a claim about
// side effects — and this pass is not the one that gets to make it.
func simplifyTerminator(term Terminator, copies map[string]Operand) (Terminator, bool) {
	branch, ok := term.(BranchTerminator)
	if !ok {
		return term, false
	}

	constant, isConst := copies[operandName(branch.Cond)]
	if !isConst {
		if _, isLiteral := branch.Cond.(ConstOperand); isLiteral {
			constant = branch.Cond
			isConst = true
		}
	}
	if !isConst {
		return term, false
	}

	taken, truthy := branchTruth(constant)
	if !truthy {
		return term, false
	}

	// Both arms are kept, and only the condition is pinned. Removing an arm here
	// would need to know whether the arm had effects; leaving it means a later
	// unreachable-block pass removes it if it truly had none, which it can
	// determine safely.
	resolved := branch
	resolved.Cond = ConstOperand{Value: taken}
	return resolved, true
}

// branchTruth is the run-time meaning of a constant condition.
func branchTruth(op Operand) (value bool, known bool) {
	c, ok := op.(ConstOperand)
	if !ok {
		return false, false
	}
	switch v := c.Value.(type) {
	case bool:
		return v, true
	case int64:
		return v != 0, true
	case float64:
		return v != 0, true
	case string:
		return v != "", true
	case nil:
		return false, true
	}
	return false, false
}

// operandName is the name an operand can be substituted through, or "" when it has
// none.
func operandName(op Operand) string {
	switch v := op.(type) {
	case TempOperand:
		return v.String()
	case VarOperand:
		return v.Name
	}
	return ""
}

// rewriteOperands applies a substitution to every operand an instruction has.
//
// Every pass that rewrites operands needs this list, and the instructions are
// value types rather than an interface with accessors — so a pass that adds an
// instruction and forgets to update this function would substitute silently less,
// which is exactly the kind of omission that only shows up as a slow program.
func rewriteOperands(inst Instruction, replace func(Operand) (Operand, bool)) (Instruction, bool) {
	changed := false

	rewrite := func(op Operand) Operand {
		if op == nil {
			return nil
		}
		if next, ok := replace(op); ok {
			changed = true
			return next
		}
		return op
	}

	switch i := inst.(type) {
	case AssignInst:
		i.Src = rewrite(i.Src)
		return i, changed
	case LoadVarInst:
		return i, changed
	case StoreVarInst:
		i.Src = rewrite(i.Src)
		return i, changed
	case BinaryOpInst:
		i.Left = rewrite(i.Left)
		i.Right = rewrite(i.Right)
		return i, changed
	case UnaryOpInst:
		i.Right = rewrite(i.Right)
		return i, changed
	case AggregateInst:
		elems := make([]Operand, len(i.Elems))
		for idx, e := range i.Elems {
			elems[idx] = rewrite(e)
		}
		i.Elems = elems
		return i, changed
	case SelectInst:
		i.True = rewrite(i.True)
		i.False = rewrite(i.False)
		return i, changed
	case CallInst:
		args := make([]Operand, len(i.Args))
		for idx, a := range i.Args {
			args[idx] = rewrite(a)
		}
		i.Args = args
		return i, changed
	}
	return inst, false
}
