package llvm

import (
	"fmt"
	"sort"
	"strings"

	"github.com/joysriramsarkar/nilLang/compiler/mir"
)

// A phi is a cross-block join of a MIR temporary.
//
// # Why this exists
//
// A MIR temporary assigned in two blocks — the two arms of an `if`, say — is two
// writes of one value. Emitting `%t1 = add i64 ...` in each block defines the same
// name twice, and LLVM rejects that with "multiple definition of local value",
// naming a line that looks entirely ordinary.
//
// So each block writes its own copy under a block-local name, and the merge block
// joins them with a phi. The join is the only place the two can legally meet,
// which is the same reason a phi's incoming value must already exist.
//
// # Why a variable is not treated this way
//
// A MIR *variable* is a slot, not a temporary: it has one `alloca` and every block
// loads and stores that same address. Giving it a phi per block would be correct
// and pointless — the alloca already joins the blocks, and a phi on top would be
// joining what is already joined.
type crossBlockPhi struct {
	// temp is the MIR temporary this phi joins.
	temp mir.TempOperand

	// name is the LLVM local the phi defines.
	name string

	// mergeID is the MIR block the phi is written in.
	mergeID string

	// incoming maps each writer's block to the value arriving from it.
	incoming map[string]string

	// labels maps each writer's block to the LLVM label it is written under.
	labels map[string]string
}

// key is the temporary's name, which is what a block-local rename is keyed by.
func (p crossBlockPhi) key() string { return fmt.Sprintf("t%d", p.temp.ID) }

// text renders the phi with one incoming value per writer.
//
// Sorted because an unsorted phi would make two builds of the same program differ,
// which breaks any cache keyed on the module and makes a diff of two modules
// unreadable — the same reason every other ordered output in this emitter is sorted.
func (p crossBlockPhi) text(blockNames map[string]map[string]string) string {
	writers := make([]string, 0, len(p.incoming))
	for id := range p.incoming {
		writers = append(writers, id)
	}
	sort.Strings(writers)

	parts := make([]string, 0, len(writers))
	for _, id := range writers {
		value := p.incoming[id]
		if renamed, known := blockNames[id][p.key()]; known {
			value = renamed
		}
		parts = append(parts, fmt.Sprintf(" %s, %%%s ", value, p.labels[id]))
	}
	if len(parts) == 0 {
		return "phi i64 []"
	}
	return "phi i64 [" + strings.Join(parts, "], [") + " ]"
}

// blockLabelFor is the label a block is written under.
//
// The first block of a function must be called `entry` — not by convention but
// because LLVM requires it, and rejects the module with "expected instruction
// opcode" pointing at a label that looks entirely ordinary otherwise. The emitter
// owns the entry block's name, so it answers that; the free `blockLabel` here is
// the emitter method, reached through the receiver.
func (e *Emitter) blockLabelFor(id string) string { return e.blockLabel(id) }

// crossBlockPhis finds every temporary written in more than one block and joins
// them at the block their writers both reach.
//
// # Which block a phi belongs in
//
// The one both writers reach: the merge point. Writing it in either arm would be
// wrong — LLVM requires a phi's incoming value to be the one live on the edge into
// that block, and a phi placed before the value exists is a use of nothing.
//
// A writer whose blocks do not share a successor gets no phi, and the emitter
// refuses the program rather than guessing where the two are supposed to meet.
func (e *Emitter) crossBlockPhis(fn *mir.Function) []crossBlockPhi {
	writers := map[string][]string{}
	targets := map[string]map[string]bool{}

	for _, block := range fn.Blocks {
		if block == nil {
			continue
		}
		if targets[block.ID] == nil {
			targets[block.ID] = map[string]bool{}
		}
		for _, next := range terminatorTargets(block.Terminator) {
			targets[block.ID][next] = true
		}
		for _, inst := range block.Instructions {
			dest := instructionDest(inst)
			if dest == nil {
				continue
			}
			key := fmt.Sprintf("t%d", dest.ID)
			writers[key] = append(writers[key], block.ID)
		}
	}

	keys := make([]string, 0, len(writers))
	for key := range writers {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var phis []crossBlockPhi
	for _, key := range keys {
		blocks := writers[key]
		if len(blocks) < 2 {
			continue
		}

		merge := sharedSuccessor(blocks, targets)
		if merge == "" {
			continue
		}

		temp, ok := tempByKey(fn, key)
		if !ok {
			continue
		}

		phi := crossBlockPhi{
			temp:     temp,
			name:     key + ".phi",
			mergeID:  merge,
			incoming: map[string]string{},
			labels:   map[string]string{},
		}
		for _, block := range blocks {
			phi.incoming[block] = "%" + key
			phi.labels[block] = e.blockLabel(block)
		}
		phis = append(phis, phi)
	}
	return phis
}

// sharedSuccessor is the one block every writer jumps to, or "" when there is none.
func sharedSuccessor(writers []string, targets map[string]map[string]bool) string {
	for candidate := range targets[writers[0]] {
		shared := true
		for _, writer := range writers[1:] {
			if !targets[writer][candidate] {
				shared = false
				break
			}
		}
		if shared {
			return candidate
		}
	}
	return ""
}

// tempByKey recovers the temporary a writer key names.
func tempByKey(fn *mir.Function, key string) (mir.TempOperand, bool) {
	for _, block := range fn.Blocks {
		if block == nil {
			continue
		}
		for _, inst := range block.Instructions {
			if dest := instructionDest(inst); dest != nil && fmt.Sprintf("t%d", dest.ID) == key {
				return *dest, true
			}
		}
	}
	return mir.TempOperand{}, false
}

// blockLocalNames gives every writing block its own copy of each cross-block
// temporary.
//
// Without this the two arms of an `if` both write `%t1` and LLVM rejects the
// module. With it each arm writes `%t1.bb_then` and the merge block joins them —
// names a reader of the IR would guess, because the block that defined it is in
// the name.
func (e *Emitter) blockLocalNames(fn *mir.Function, phis []crossBlockPhi) map[string]map[string]string {
	names := map[string]map[string]string{}
	if len(phis) == 0 {
		return names
	}

	for _, block := range fn.Blocks {
		if block == nil {
			continue
		}
		names[block.ID] = map[string]string{}

		for _, phi := range phis {
			// The merge block *defines* the joined name, so a read of the
			// temporary there is a read of the phi.
			if phi.mergeID == block.ID {
				names[block.ID][phi.key()] = "%" + phi.name
				continue
			}
			if _, wrote := phi.incoming[block.ID]; !wrote {
				continue
			}
			names[block.ID][phi.key()] = fmt.Sprintf("%%%s.%s", phi.key(), block.ID)
		}
	}
	return names
}

// instructionDest is the temporary an instruction writes, or nil.
func instructionDest(inst mir.Instruction) *mir.TempOperand {
	switch i := inst.(type) {
	case mir.AssignInst:
		return tempOf(i.Dest)
	case mir.BinaryOpInst:
		return tempOf(i.Dest)
	case mir.UnaryOpInst:
		return tempOf(i.Dest)
	case mir.CallInst:
		return tempOf(i.Dest)
	case mir.AggregateInst:
		return tempOf(i.Dest)
	case mir.SelectInst:
		return tempOf(i.Dest)
	}
	return nil
}

func tempOf(op mir.Operand) *mir.TempOperand {
	if temp, ok := op.(mir.TempOperand); ok {
		return &temp
	}
	return nil
}

// terminatorTargets is every block a terminator can branch to.
func terminatorTargets(term mir.Terminator) []string {
	switch t := term.(type) {
	case mir.JumpTerminator:
		return []string{t.Target}
	case mir.BranchTerminator:
		return []string{t.TrueTarget, t.FalseTarget}
	}
	return nil
}
