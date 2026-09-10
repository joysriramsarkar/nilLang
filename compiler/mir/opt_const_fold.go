package mir

// OptimizeConstantFolding performs constant folding and constant propagation across MIR blocks
func OptimizeConstantFolding(fn *Function) bool {
	changed := false

	for _, bb := range fn.Blocks {
		constMap := make(map[string]Operand) // maps Temp/Var operand string -> ConstOperand
		newInsts := make([]Instruction, 0, len(bb.Instructions))

		for _, inst := range bb.Instructions {
			// Step 1: Replace operands with propagated constants if available
			inst = substituteConstants(inst, constMap)

			// Step 2: Fold operations if all operands are constants
			foldedInst, foldedConst := foldInstruction(inst)
			if foldedConst != nil {
				changed = true
				destStr := getDestString(inst)
				if destStr != "" {
					constMap[destStr] = *foldedConst
				}
				newInsts = append(newInsts, foldedInst)
				continue
			}

			// If it's an assignment to a constant, record for propagation
			if assign, ok := inst.(AssignInst); ok {
				if c, isConst := assign.Src.(ConstOperand); isConst {
					constMap[assign.Dest.String()] = c
				}
			}

			newInsts = append(newInsts, inst)
		}
		bb.Instructions = newInsts

		// Step 3: Fold branch terminator if condition is constant
		if branch, ok := bb.Terminator.(BranchTerminator); ok {
			cond := branch.Cond
			if c, ok := constMap[cond.String()]; ok {
				cond = c
			}
			if c, ok := cond.(ConstOperand); ok {
				if b, ok := c.Value.(bool); ok {
					changed = true
					if b {
						bb.Terminator = JumpTerminator{Target: branch.TrueTarget}
					} else {
						bb.Terminator = JumpTerminator{Target: branch.FalseTarget}
					}
				}
			}
		}
	}

	return changed
}

func substituteConstants(inst Instruction, constMap map[string]Operand) Instruction {
	switch i := inst.(type) {
	case AssignInst:
		if c, ok := constMap[i.Src.String()]; ok {
			return AssignInst{Dest: i.Dest, Src: c}
		}
	case StoreVarInst:
		if c, ok := constMap[i.Src.String()]; ok {
			return StoreVarInst{Name: i.Name, Src: c}
		}
	case BinaryOpInst:
		left := i.Left
		if c, ok := constMap[i.Left.String()]; ok {
			left = c
		}
		right := i.Right
		if c, ok := constMap[i.Right.String()]; ok {
			right = c
		}
		return BinaryOpInst{Dest: i.Dest, Op: i.Op, Left: left, Right: right}
	case UnaryOpInst:
		right := i.Right
		if c, ok := constMap[i.Right.String()]; ok {
			right = c
		}
		return UnaryOpInst{Dest: i.Dest, Op: i.Op, Right: right}
	}
	return inst
}

func foldInstruction(inst Instruction) (Instruction, *ConstOperand) {
	switch i := inst.(type) {
	case BinaryOpInst:
		leftC, leftIsConst := i.Left.(ConstOperand)
		rightC, rightIsConst := i.Right.(ConstOperand)
		if leftIsConst && rightIsConst {
			val, ok := evaluateBinaryConst(i.Op, leftC.Value, rightC.Value)
			if ok {
				c := ConstOperand{Value: val}
				return AssignInst{Dest: i.Dest, Src: c}, &c
			}
		}
	case UnaryOpInst:
		rightC, rightIsConst := i.Right.(ConstOperand)
		if rightIsConst {
			val, ok := evaluateUnaryConst(i.Op, rightC.Value)
			if ok {
				c := ConstOperand{Value: val}
				return AssignInst{Dest: i.Dest, Src: c}, &c
			}
		}
	}
	return inst, nil
}

func evaluateBinaryConst(op string, left, right any) (any, bool) {
	// Integer arithmetic
	if l, ok := left.(int64); ok {
		if r, ok := right.(int64); ok {
			switch op {
			case "+":
				return l + r, true
			case "-":
				return l - r, true
			case "*":
				return l * r, true
			case "/":
				if r != 0 {
					return l / r, true
				}
			case "%":
				if r != 0 {
					return l % r, true
				}
			case "==":
				return l == r, true
			case "!=":
				return l != r, true
			case "<":
				return l < r, true
			case "<=":
				return l <= r, true
			case ">":
				return l > r, true
			case ">=":
				return l >= r, true
			}
		}
	}

	// Floating point arithmetic
	if l, ok := toFloat(left); ok {
		if r, ok := toFloat(right); ok {
			switch op {
			case "+":
				return l + r, true
			case "-":
				return l - r, true
			case "*":
				return l * r, true
			case "/":
				if r != 0.0 {
					return l / r, true
				}
			case "==":
				return l == r, true
			case "!=":
				return l != r, true
			case "<":
				return l < r, true
			case "<=":
				return l <= r, true
			case ">":
				return l > r, true
			case ">=":
				return l >= r, true
			}
		}
	}

	// String operations
	if l, ok := left.(string); ok {
		if r, ok := right.(string); ok {
			switch op {
			case "+":
				return l + r, true
			case "==":
				return l == r, true
			case "!=":
				return l != r, true
			}
		}
	}

	// Boolean operations
	if l, ok := left.(bool); ok {
		if r, ok := right.(bool); ok {
			switch op {
			case "&&":
				return l && r, true
			case "||":
				return l || r, true
			case "==":
				return l == r, true
			case "!=":
				return l != r, true
			}
		}
	}

	return nil, false
}

func evaluateUnaryConst(op string, val any) (any, bool) {
	switch op {
	case "-":
		if i, ok := val.(int64); ok {
			return -i, true
		}
		if f, ok := val.(float64); ok {
			return -f, true
		}
	case "!":
		if b, ok := val.(bool); ok {
			return !b, true
		}
	}
	return nil, false
}

func toFloat(val any) (float64, bool) {
	if f, ok := val.(float64); ok {
		return f, true
	}
	return 0, false
}

func getDestString(inst Instruction) string {
	switch i := inst.(type) {
	case AssignInst:
		return i.Dest.String()
	case BinaryOpInst:
		return i.Dest.String()
	case UnaryOpInst:
		return i.Dest.String()
	case LoadVarInst:
		return i.Dest.String()
	case CallInst:
		if i.Dest != nil {
			return i.Dest.String()
		}
	}
	return ""
}
