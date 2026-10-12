package mir

// Instruction counting, for the optimiser's own gate.
//
// `L3.6` asks for a measured speed-up, and a speed-up is measured in something.
// Instruction count is not that something — it is a proxy, and a proxy that can
// move the wrong way: an optimiser that removes a `store` and a `load` trades them
// for a global access each time, which is fewer instructions and no faster code.
//
// So this file counts two things. Instructions, because it is the cheapest thing
// that moves and the clearest thing to assert on. And blocks, because block
// transitions are what merging actually removes and what the VM actually pays for
// at a branch.
//
// Neither number is a claim about speed. They are claims about *shape*, and a test
// on shape is one that keeps holding when the thing being optimised changes.

// countInstructions is the total number of MIR instructions in a program.
func countInstructions(prog *Program) int {
	total := 0
	if prog.Main != nil {
		total += countFunctionInstructions(prog.Main)
	}
	for _, fn := range prog.Functions {
		total += countFunctionInstructions(fn)
	}
	return total
}

func countFunctionInstructions(fn *Function) int {
	total := 0
	for _, block := range fn.Blocks {
		total += len(block.Instructions)
	}
	return total
}

// countBlocks is the number of basic blocks a program is split into.
//
// It is the number merging reduces, and — for the same reason a jump costs more
// than an instruction — the one that tracks how much control flow a run has to
// follow.
func countBlocks(prog *Program) int {
	total := 0
	if prog.Main != nil {
		total += len(prog.Main.Blocks)
	}
	for _, fn := range prog.Functions {
		total += len(fn.Blocks)
	}
	return total
}
