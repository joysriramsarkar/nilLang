package mir

// CFG represents the Control Flow Graph of a MIR function
type CFG struct {
	Function *Function
	Blocks   map[string]*CFGNode
	Entry    *CFGNode
	Exit     *CFGNode
}

// CFGNode wraps a BasicBlock with control flow edge graph analysis
type CFGNode struct {
	Block        *BasicBlock
	Predecessors []*CFGNode
	Successors   []*CFGNode
}

// BuildCFG constructs control flow graph edges for the given function
func BuildCFG(fn *Function) *CFG {
	cfg := &CFG{
		Function: fn,
		Blocks:   make(map[string]*CFGNode),
	}

	for _, bb := range fn.Blocks {
		cfg.Blocks[bb.ID] = &CFGNode{
			Block:        bb,
			Predecessors: []*CFGNode{},
			Successors:   []*CFGNode{},
		}
	}

	if len(fn.Blocks) > 0 {
		cfg.Entry = cfg.Blocks[fn.Blocks[0].ID]
	}

	for _, bb := range fn.Blocks {
		current := cfg.Blocks[bb.ID]
		if bb.Terminator == nil {
			continue
		}

		switch t := bb.Terminator.(type) {
		case JumpTerminator:
			if targetNode, ok := cfg.Blocks[t.Target]; ok {
				current.Successors = append(current.Successors, targetNode)
				targetNode.Predecessors = append(targetNode.Predecessors, current)
			}
		case BranchTerminator:
			if trueNode, ok := cfg.Blocks[t.TrueTarget]; ok {
				current.Successors = append(current.Successors, trueNode)
				trueNode.Predecessors = append(trueNode.Predecessors, current)
			}
			if falseNode, ok := cfg.Blocks[t.FalseTarget]; ok {
				current.Successors = append(current.Successors, falseNode)
				falseNode.Predecessors = append(falseNode.Predecessors, current)
			}
		}
	}

	return cfg
}

// ReachableBlocks returns all basic block IDs reachable from the entry block
func (c *CFG) ReachableBlocks() map[string]bool {
	reachable := make(map[string]bool)
	if c.Entry == nil {
		return reachable
	}

	queue := []*CFGNode{c.Entry}
	reachable[c.Entry.Block.ID] = true

	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]

		for _, succ := range curr.Successors {
			if !reachable[succ.Block.ID] {
				reachable[succ.Block.ID] = true
				queue = append(queue, succ)
			}
		}
	}

	return reachable
}
