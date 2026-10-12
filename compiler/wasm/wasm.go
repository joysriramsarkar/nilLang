package wasm

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joysriramsarkar/nilLang/compiler/mir"
)

// WebAssembly value types.
const (
	vtI32 byte = 0x7f
	vtI64 byte = 0x7e
	vtF32 byte = 0x7d
	vtF64 byte = 0x7c
)

// Binary opcodes used by the emitter.
const (
	opUnreachable       = 0x00
	opBlock             = 0x02
	opLoop              = 0x03
	opIf                = 0x04
	opElse              = 0x05
	opEnd               = 0x0b
	opBr                = 0x0c
	opBrIf              = 0x0d
	opReturn            = 0x0f
	opCall              = 0x10
	opDrop              = 0x1a
	opLocalGet          = 0x20
	opLocalSet          = 0x21
	opLocalTee          = 0x22
	opI32Const          = 0x41
	opI64Const          = 0x42
	opF64Const          = 0x44
	opI64Eqz            = 0x50
	opI64Eq             = 0x51
	opI64Ne             = 0x52
	opI64LtS            = 0x53
	opI64GtS            = 0x55
	opI64LeS            = 0x57
	opI64GeS            = 0x59
	opF64Eq             = 0x61
	opF64Ne             = 0x62
	opF64Lt             = 0x63
	opF64Gt             = 0x64
	opF64Le             = 0x65
	opF64Ge             = 0x66
	opI64Add            = 0x7c
	opI64Sub            = 0x7d
	opI64Mul            = 0x7e
	opI64DivS           = 0x7f
	opI64RemS           = 0x80
	opI64And            = 0x83
	opI64Or             = 0x84
	opF64Add            = 0xa0
	opF64Sub            = 0xa1
	opF64Mul            = 0xa2
	opF64Div            = 0xa3
	opF64Neg            = 0x9a
	opF64ConvertI64S    = 0xb9
	opI32WrapI64        = 0xa7
	opI64ExtendI32U     = 0xad
	opI64ReinterpretF64 = 0xbd
	opF64ReinterpretI64 = 0xbf
)

// Block types used by the emitter: empty, or a single result value.
const (
	btEmpty = 0x40
)

// WasmModule represents a compiled WebAssembly module
type WasmModule struct {
	WAT    string
	Binary []byte
}

// Compiler translates MIR programs to WebAssembly
type Compiler struct{}

func NewCompiler() *Compiler {
	return &Compiler{}
}

// WasmValType maps a Nilang-level type name to its WebAssembly value type
// (L7.4). Dynamic values in emitted code are uniformly i64 — integers ride
// directly, floats ride bit-cast (see the reinterpret sandwich in theBinaryOp
// emitters), and strings ride as linear-memory offsets — so function
// signatures stay (i64*) -> i64. This mapping is what host signatures (the
// puts import takes an i32), memory addresses (i32) and any future static
// specialization use instead of guessing a byte.
func WasmValType(name string) byte {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "i32", "int32", "bool", "boolean", "string", "byte":
		return vtI32
	case "f32", "float32":
		return vtF32
	case "f64", "float64", "float", "double":
		return vtF64
	default:
		return vtI64
	}
}

// watValTypeName renders a value-type byte for WAT text.
func watValTypeName(t byte) string {
	switch t {
	case vtI32:
		return "i32"
	case vtF32:
		return "f32"
	case vtF64:
		return "f64"
	default:
		return "i64"
	}
}

// strTable deduplicates string constants into one data segment.
type strTable struct {
	offsets map[string]uint32
	bytes   []byte
}

func newStrTable() *strTable {
	return &strTable{offsets: map[string]uint32{}}
}

// intern records s and returns its offset in the segment.
func (t *strTable) intern(s string) uint32 {
	if off, ok := t.offsets[s]; ok {
		return off
	}
	off := uint32(len(t.bytes))
	t.offsets[s] = off
	t.bytes = append(t.bytes, s...)
	return off
}

// collectStrings walks every function so data offsets are final before any
// code is emitted.
func collectStrings(prog *mir.Program) *strTable {
	table := newStrTable()
	visit := func(fn *mir.Function) {
		if fn == nil {
			return
		}
		for _, bb := range fn.Blocks {
			for _, inst := range bb.Instructions {
				for _, op := range instructionOperands(inst) {
					if c, ok := op.(mir.ConstOperand); ok {
						if s, ok := c.Value.(string); ok {
							table.intern(s)
						}
					}
				}
			}
			if ret, ok := bb.Terminator.(mir.ReturnTerminator); ok && ret.Value != nil {
				if c, ok := ret.Value.(mir.ConstOperand); ok {
					if s, ok := c.Value.(string); ok {
						table.intern(s)
					}
				}
			}
		}
	}
	for _, name := range sortedFunctionNames(prog) {
		visit(prog.Functions[name])
	}
	visit(prog.Main)
	return table
}

// instructionOperands lists the value operands an instruction reads, for the
// string-collection pass.
func instructionOperands(inst mir.Instruction) []mir.Operand {
	switch i := inst.(type) {
	case mir.BinaryOpInst:
		return []mir.Operand{i.Left, i.Right}
	case mir.UnaryOpInst:
		return []mir.Operand{i.Right}
	case mir.AssignInst:
		return []mir.Operand{i.Src}
	case mir.SelectInst:
		return []mir.Operand{i.Cond, i.True, i.False}
	case mir.LoadVarInst:
		return nil
	case mir.StoreVarInst:
		return []mir.Operand{i.Src}
	case mir.CallInst:
		return i.Args
	case mir.AggregateInst:
		return i.Elems
	case mir.SetIndexInst:
		return []mir.Operand{i.Container, i.Key, i.Value}
	default:
		return nil
	}
}

// sortedFunctionNames lists user functions deterministically. The program
// map iterates randomly, and a function index that moved between builds
// would make every call edge point somewhere else.
func sortedFunctionNames(prog *mir.Program) []string {
	var names []string
	for name, fn := range prog.Functions {
		if fn == nil || fn.Foreign {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// locals assigns every parameter, variable and temporary a dense local index
// (L7.2). Parameters come first in declaration order, then every other name
// in first-appearance order over the block layout, so both emitters agree
// without sharing state.
type locals struct {
	index   map[string]int
	kinds   map[string]bool // true when the slot holds float bits
	params  int
	isParam map[string]bool
}

func newLocals() *locals {
	return &locals{index: map[string]int{}, kinds: map[string]bool{}, isParam: map[string]bool{}}
}

// varSlot resolves a source variable to its slot name. A parameter and the
// variable of the same name are one slot: MIR reads parameters through
// LoadVar, so without the alias the body would read zero. Assigning the
// variable writes the parameter slot, which is the language's semantics for
// parameter assignment.
func (l *locals) varSlot(name string) string {
	if l.isParam[name] {
		return "p_" + name
	}
	return "v_" + name
}

// slotOf canonicalises an operand to its slot name, aliasing a variable
// that shares a parameter's name onto the parameter slot.
func (l *locals) slotOf(op mir.Operand) (string, bool) {
	switch o := op.(type) {
	case mir.VarOperand:
		return l.varSlot(o.Name), true
	case mir.TempOperand:
		return fmt.Sprintf("t_%d", o.ID), true
	default:
		return "", false
	}
}

// allocate walks one function and numbers its slots.
func (l *locals) allocate(fn *mir.Function) {
	for _, p := range fn.Params {
		name := "p_" + p
		if _, ok := l.index[name]; !ok {
			l.index[name] = len(l.index)
		}
		l.isParam[p] = true
	}
	l.params = len(fn.Params)
	declare := func(op mir.Operand) {
		if name, ok := l.slotOf(op); ok {
			if _, exists := l.index[name]; !exists {
				l.index[name] = len(l.index)
			}
		}
	}
	for _, bb := range fn.Blocks {
		for _, inst := range bb.Instructions {
			switch i := inst.(type) {
			case mir.BinaryOpInst:
				declare(i.Dest)
				declare(i.Left)
				declare(i.Right)
			case mir.UnaryOpInst:
				declare(i.Dest)
				declare(i.Right)
			case mir.AssignInst:
				declare(i.Dest)
				declare(i.Src)
			case mir.SelectInst:
				declare(i.Dest)
				declare(i.Cond)
				declare(i.True)
				declare(i.False)
			case mir.LoadVarInst:
				declare(i.Dest)
				l.declareVar(i.Name)
			case mir.StoreVarInst:
				declare(i.Src)
				l.declareVar(i.Name)
			case mir.CallInst:
				if i.Dest != nil {
					declare(i.Dest)
				}
				for _, a := range i.Args {
					declare(a)
				}
			case mir.AggregateInst:
				declare(i.Dest)
				for _, e := range i.Elems {
					declare(e)
				}
			case mir.SetIndexInst:
				if i.Dest != nil {
					declare(i.Dest)
				}
				declare(i.Container)
				declare(i.Key)
				declare(i.Value)
			}
		}
		if ret, ok := bb.Terminator.(mir.ReturnTerminator); ok && ret.Value != nil {
			declare(ret.Value)
		}
		if br, ok := bb.Terminator.(mir.BranchTerminator); ok {
			declare(br.Cond)
		}
	}
}

// declareVar numbers the slot for a source variable name.
func (l *locals) declareVar(name string) {
	slot := l.varSlot(name)
	if _, ok := l.index[slot]; !ok {
		l.index[slot] = len(l.index)
	}
}

// isFloatOperand reports whether an operand carries float bits: a float
// literal, or a slot previously written by a float-producing instruction.
func (l *locals) isFloatOperand(op mir.Operand) bool {
	switch o := op.(type) {
	case mir.ConstOperand:
		_, ok := o.Value.(float64)
		return ok
	default:
		if name, ok := l.slotOf(op); ok {
			return l.kinds[name]
		}
		return false
	}
}

// setKind records the float-ness of a destination slot.
func (l *locals) setKind(dest mir.Operand, isFloat bool) {
	if name, ok := l.slotOf(dest); ok {
		l.kinds[name] = isFloat
	}
}

// count returns the number of locals beyond the parameters.
func (l *locals) count() int {
	return len(l.index) - l.params
}

// watName renders a slot name for WAT text.
func (l *locals) watName(op mir.Operand) string {
	if name, ok := l.slotOf(op); ok {
		return "$" + name
	}
	return "$_"
}

// binIndex resolves a slot name to its binary local index.
func (l *locals) binIndex(op mir.Operand) uint64 {
	if name, ok := l.slotOf(op); ok {
		if idx, exists := l.index[name]; exists {
			return uint64(idx)
		}
	}
	return 0
}

// paramWATName renders a function parameter for WAT text.
func paramWATName(p string) string { return "$p_" + p }

// ─── SHARED EMISSION HELPERS ─────────────────────────────────────────────

// floatBinOp reports whether op is a float-capable binary operator lowered
// with the reinterpret sandwich, and if so its f64 opcode pair.
func floatBinOp(op string) (byte, bool) {
	switch op {
	case "+":
		return opF64Add, true
	case "-":
		return opF64Sub, true
	case "*":
		return opF64Mul, true
	case "/":
		return opF64Div, true
	default:
		return 0, false
	}
}

// floatCmpOp maps a comparison to its f64 opcode. The result is i32 and must
// be extended to i64 by the caller.
func floatCmpOp(op string) (byte, bool) {
	switch op {
	case "==":
		return opF64Eq, true
	case "!=":
		return opF64Ne, true
	case "<":
		return opF64Lt, true
	case "<=":
		return opF64Le, true
	case ">":
		return opF64Gt, true
	case ">=":
		return opF64Ge, true
	default:
		return 0, false
	}
}

// intBinOp maps an operator to its i64 opcode.
func intBinOp(op string) byte {
	switch op {
	case "-":
		return opI64Sub
	case "*":
		return opI64Mul
	case "/":
		return opI64DivS
	case "%":
		return opI64RemS
	case "==":
		return opI64Eq
	case "!=":
		return opI64Ne
	case "<":
		return opI64LtS
	case "<=":
		return opI64LeS
	case ">":
		return opI64GtS
	case ">=":
		return opI64GeS
	case "&&":
		return opI64And
	case "||":
		return opI64Or
	default:
		return opI64Add
	}
}

// isComparisonOp reports whether op produces an i32 truth value rather than
// an i64 number. Both emitters extend the result to i64 on store.
func isComparisonOp(op string) bool {
	switch op {
	case "==", "!=", "<", "<=", ">", ">=":
		return true
	default:
		return false
	}
}

// watBinOp renders an operator for WAT text.
func watBinOp(op string, isFloat bool) string {
	if isFloat {
		switch op {
		case "+":
			return "f64.add"
		case "-":
			return "f64.sub"
		case "*":
			return "f64.mul"
		case "/":
			return "f64.div"
		case "==":
			return "f64.eq"
		case "!=":
			return "f64.ne"
		case "<":
			return "f64.lt"
		case "<=":
			return "f64.le"
		case ">":
			return "f64.gt"
		case ">=":
			return "f64.ge"
		}
	}
	switch op {
	case "-":
		return "i64.sub"
	case "*":
		return "i64.mul"
	case "/":
		return "i64.div_s"
	case "%":
		return "i64.rem_s"
	case "==":
		return "i64.eq"
	case "!=":
		return "i64.ne"
	case "<":
		return "i64.lt_s"
	case "<=":
		return "i64.le_s"
	case ">":
		return "i64.gt_s"
	case ">=":
		return "i64.ge_s"
	case "&&":
		return "i64.and"
	case "||":
		return "i64.or"
	default:
		return "i64.add"
	}
}

// watEscape renders a string for a WAT data segment.
func watEscape(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\t':
			b.WriteString(`\t`)
		default:
			if c < 0x20 || c > 0x7e {
				fmt.Fprintf(&b, `\%.2x`, c)
			} else {
				b.WriteByte(c)
			}
		}
	}
	return b.String()
}

// ─── CONTROL-FLOW ANALYSIS ───────────────────────────────────────────────

// fnView indexes one function's blocks for the region emitters.
type fnView struct {
	fn       *mir.Function
	blocks   map[string]*mir.BasicBlock
	order    []string
	pos      map[string]int
	isHeader map[string]bool
}

func newFnView(fn *mir.Function) *fnView {
	v := &fnView{
		fn:       fn,
		blocks:   map[string]*mir.BasicBlock{},
		pos:      map[string]int{},
		isHeader: map[string]bool{},
	}
	for i, bb := range fn.Blocks {
		v.blocks[bb.ID] = bb
		v.order = append(v.order, bb.ID)
		v.pos[bb.ID] = i
	}
	for _, id := range v.order {
		for _, target := range terminatorTargets(v.blocks[id].Terminator) {
			if pos, ok := v.pos[target]; ok && pos <= v.pos[id] {
				v.isHeader[target] = true
			}
		}
	}
	return v
}

// terminatorTargets lists the blocks a terminator can jump to.
func terminatorTargets(term mir.Terminator) []string {
	switch t := term.(type) {
	case mir.BranchTerminator:
		return []string{t.TrueTarget, t.FalseTarget}
	case mir.JumpTerminator:
		return []string{t.Target}
	default:
		return nil
	}
}

// loopBodyExit decomposes a loop header ending in a branch into its body and
// exit blocks.
//
// The body is the arm that holds a back edge: a block positioned after the
// header which jumps to it. Plain reachability cannot answer this, because in
// a nested loop the exit arm reaches the inner header too — through the
// enclosing loop (exit_6 jumps to the outer cond, which re-enters the inner
// one). Only an edge from below the header counts, and the walk never enters
// the header itself, so an outer cycle cannot disguise an exit as a body.
func (v *fnView) loopBodyExit(header string) (body, exit string, ok bool) {
	bb, found := v.blocks[header]
	if !found {
		return "", "", false
	}
	branch, isBranch := bb.Terminator.(mir.BranchTerminator)
	if !isBranch {
		return "", "", false
	}
	headerPos, positioned := v.pos[header]
	if !positioned {
		return "", "", false
	}
	holdsBackEdge := func(arm string) bool {
		visited := map[string]bool{}
		stack := []string{arm}
		for len(stack) > 0 {
			id := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if id == header || visited[id] {
				continue
			}
			visited[id] = true
			next, found := v.blocks[id]
			if !found {
				continue
			}
			for _, target := range terminatorTargets(next.Terminator) {
				if target == header {
					if pos, ok := v.pos[id]; ok && pos > headerPos {
						return true
					}
					continue
				}
				stack = append(stack, target)
			}
		}
		return false
	}
	trueIsBody := holdsBackEdge(branch.TrueTarget)
	falseIsBody := holdsBackEdge(branch.FalseTarget)
	switch {
	case trueIsBody && !falseIsBody:
		return branch.TrueTarget, branch.FalseTarget, true
	case falseIsBody && !trueIsBody:
		return branch.FalseTarget, branch.TrueTarget, true
	default:
		return "", "", false
	}
}

// CompileToWAT compiles a MIR program to WebAssembly Text (WAT) format
func (c *Compiler) CompileToWAT(prog *mir.Program) (string, error) {
	strtab := collectStrings(prog)
	var b strings.Builder
	b.WriteString("(module\n")
	b.WriteString("  (import \"env\" \"puts\" (func $puts (param i32)))\n")
	pages := uint32(1)
	if need := uint32(len(strtab.bytes)/65536 + 1); need > pages {
		pages = need
	}
	fmt.Fprintf(&b, "  (memory (export \"memory\") %d)\n", pages)
	if len(strtab.bytes) > 0 {
		fmt.Fprintf(&b, "  (data (i32.const 0) \"%s\")\n", watEscape(string(strtab.bytes)))
	}
	b.WriteByte('\n')

	funcIndex := map[string]int{}
	idx := 1 // 0 is the puts import
	for _, name := range sortedFunctionNames(prog) {
		funcIndex[name] = idx
		idx++
	}
	mainIndex := -1
	if prog.Main != nil {
		mainIndex = idx
	}
	_ = mainIndex

	for _, name := range sortedFunctionNames(prog) {
		fn := prog.Functions[name]
		body, err := compileFuncWAT(fn, "$"+name, funcIndex, strtab, false)
		if err != nil {
			return "", err
		}
		b.WriteString(body)
	}
	if prog.Main != nil {
		body, err := compileFuncWAT(prog.Main, "$main", funcIndex, strtab, true)
		if err != nil {
			return "", err
		}
		b.WriteString(body)
	}

	b.WriteString(")\n")
	return b.String(), nil
}

// compileFuncWAT renders one MIR function. Exported (main) functions carry
// the export annotation; internal ones are plain.
func compileFuncWAT(fn *mir.Function, watName string, funcIndex map[string]int, strtab *strTable, exported bool) (string, error) {
	loc := newLocals()
	loc.allocate(fn)
	var b strings.Builder
	if exported {
		fmt.Fprintf(&b, "  (func %s (export \"main\") (result i64)\n", watName)
	} else {
		fmt.Fprintf(&b, "  (func %s", watName)
		for _, p := range fn.Params {
			fmt.Fprintf(&b, " (param %s i64)", paramWATName(p))
		}
		b.WriteString(" (result i64)\n")
	}
	// Locals beyond the parameters (L7.2).
	names := make([]string, 0, len(loc.index))
	for name := range loc.index {
		isParam := false
		for _, p := range fn.Params {
			if name == "p_"+p {
				isParam = true
				break
			}
		}
		if !isParam {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	for _, name := range names {
		fmt.Fprintf(&b, "    (local $%s i64)\n", name)
	}
	e := &watEmitter{view: newFnView(fn), loc: loc, funcIndex: funcIndex, strings: strtab, visited: map[string]bool{}, labels: map[string]string{}}
	if len(fn.Blocks) > 0 {
		e.emitSeq(fn.Blocks[0].ID)
		b.WriteString(e.out.String())
	}
	// Every path must leave an i64: a function that falls off the end
	// returns zero. Code after a return is unreachable but valid.
	b.WriteString("    i64.const 0\n")
	b.WriteString("  )\n\n")
	return b.String(), nil
}

// watEmitter walks one function's regions into WAT text.
type watEmitter struct {
	view      *fnView
	loc       *locals
	funcIndex map[string]int
	strings   *strTable
	out       strings.Builder
	visited   map[string]bool
	labels    map[string]string
}

// emitSeq emits a block and whatever linearly follows it.
func (e *watEmitter) emitSeq(id string) {
	cur := id
	for cur != "" {
		if e.visited[cur] {
			if label, ok := e.labels[cur]; ok {
				fmt.Fprintf(&e.out, "    br %s\n", label)
			} else {
				// No structured target: trap loudly rather than fall through
				// into whatever textually follows, which would silently run
				// the wrong block. Reducible MIR never reaches here.
				fmt.Fprintf(&e.out, "    unreachable ;; no structured target for jump to %s\n", cur)
			}
			return
		}
		bb, ok := e.view.blocks[cur]
		if !ok {
			fmt.Fprintf(&e.out, "    unreachable ;; unknown block %s\n", cur)
			return
		}
		e.visited[cur] = true
		e.emitInstructions(bb)
		switch term := bb.Terminator.(type) {
		case mir.ReturnTerminator:
			if term.Value != nil {
				e.emitOperand(term.Value)
			} else {
				e.out.WriteString("    i64.const 0\n")
			}
			e.out.WriteString("    return\n")
			return
		case mir.JumpTerminator:
			target := term.Target
			if e.view.isHeader[target] && !e.visited[target] {
				e.emitLoop(target)
				cur = e.view.loopExitOf(target)
				continue
			}
			if label, ok := e.labels[target]; ok {
				fmt.Fprintf(&e.out, "    br %s\n", label)
				return
			}
			cur = target
			continue
		case mir.BranchTerminator:
			if e.view.isHeader[cur] {
				if body, exit, ok := e.view.loopBodyExit(cur); ok {
					_ = body
					e.emitLoop(cur)
					cur = exit
					continue
				}
			}
			cur = e.emitBranch(term)
			continue
		default:
			// No terminator: fall through to the next block in layout order.
			cur = e.nextInOrder(cur)
			continue
		}
	}
}

// nextInOrder returns the unvisited block following id in layout order, or
// "" when control falls off the end.
func (e *watEmitter) nextInOrder(id string) string {
	pos, ok := e.view.pos[id]
	if !ok {
		return ""
	}
	for i := pos + 1; i < len(e.view.order); i++ {
		if !e.visited[e.view.order[i]] {
			return e.view.order[i]
		}
	}
	return ""
}

// emitArm emits one if-arm and returns the block it stopped at: the merge
// the trailing jump names, or "" when the arm diverges (return) or jumps
// out through an enclosing label.
func (e *watEmitter) emitArm(id string) string {
	cur := id
	for cur != "" {
		if e.visited[cur] {
			if label, ok := e.labels[cur]; ok {
				fmt.Fprintf(&e.out, "    br %s\n", label)
			} else {
				fmt.Fprintf(&e.out, "    unreachable ;; no structured target for jump to %s\n", cur)
			}
			return ""
		}
		bb, ok := e.view.blocks[cur]
		if !ok {
			e.out.WriteString("    unreachable\n")
			return ""
		}
		e.visited[cur] = true
		e.emitInstructions(bb)
		switch term := bb.Terminator.(type) {
		case mir.ReturnTerminator:
			if term.Value != nil {
				e.emitOperand(term.Value)
			} else {
				e.out.WriteString("    i64.const 0\n")
			}
			e.out.WriteString("    return\n")
			return ""
		case mir.JumpTerminator:
			target := term.Target
			if label, ok := e.labels[target]; ok {
				fmt.Fprintf(&e.out, "    br %s\n", label)
				return ""
			}
			if e.view.isHeader[target] && !e.visited[target] {
				e.emitLoop(target)
				cur = e.view.loopExitOf(target)
				continue
			}
			// A forward jump out of the arm names the merge: stop and let
			// the if's continuation emit it.
			return target
		case mir.BranchTerminator:
			if e.view.isHeader[cur] {
				if _, _, ok := e.view.loopBodyExit(cur); ok {
					e.emitLoop(cur)
					cur = e.view.loopExitOf(cur)
					continue
				}
			}
			next := e.emitBranch(term)
			if next == "" {
				return ""
			}
			cur = next
			continue
		default:
			cur = e.nextInOrder(cur)
			continue
		}
	}
	return ""
}

// emitBranch renders cond as an if/else over the two arm regions (L7.1) and
// returns the block emission continues with.
func (e *watEmitter) emitBranch(term mir.BranchTerminator) string {
	e.emitOperand(term.Cond)
	e.out.WriteString("    i32.wrap_i64\n")
	e.out.WriteString("    if\n")
	thenNext := e.emitArm(term.TrueTarget)
	e.out.WriteString("    else\n")
	elseNext := e.emitArm(term.FalseTarget)
	e.out.WriteString("    end\n")
	if thenNext == elseNext {
		return thenNext
	}
	if thenNext == "" {
		return elseNext
	}
	if elseNext == "" {
		return thenNext
	}
	fmt.Fprintf(&e.out, "    ;; divergent arms %s / %s\n", thenNext, elseNext)
	return ""
}

// emitLoop renders a loop header's branch as a block/loop pair (L7.1):
//
//	block $exit_h
//	  loop $loop_h
//	    <cond code>
//	    br_if $exit_h (when cond is false)
//	    <body>
//	    br $loop_h
func (e *watEmitter) emitLoop(header string) {
	loopLabel := "$loop_" + header
	exitLabel := "$exit_" + header
	body, exit, ok := e.view.loopBodyExit(header)
	if !ok {
		// A header without a decomposable branch (e.g. an infinite loop
		// whose condition folded away): still a loop, with no exit.
		fmt.Fprintf(&e.out, "    (block %s\n", exitLabel)
		fmt.Fprintf(&e.out, "      (loop %s\n", loopLabel)
		saved := e.labels
		e.labels = copyLabels(e.labels)
		e.labels[header] = loopLabel
		e.emitSeq(header)
		e.labels = saved
		fmt.Fprintf(&e.out, "        br %s\n", loopLabel)
		e.out.WriteString("      )\n")
		e.out.WriteString("    )\n")
		return
	}
	fmt.Fprintf(&e.out, "    (block %s\n", exitLabel)
	fmt.Fprintf(&e.out, "      (loop %s\n", loopLabel)
	// The condition lives in the header block; emit it inline without
	// following its branch.
	if bb, found := e.view.blocks[header]; found {
		e.visited[header] = true
		e.emitInstructions(bb)
		if branch, isBranch := bb.Terminator.(mir.BranchTerminator); isBranch {
			e.emitOperand(branch.Cond)
			e.out.WriteString("        i64.eqz\n")
			fmt.Fprintf(&e.out, "        br_if %s\n", exitLabel)
		}
	}
	saved := e.labels
	e.labels = copyLabels(e.labels)
	e.labels[header] = loopLabel
	e.labels[exit] = exitLabel
	// The arm reports "": it returned or jumped out through a label, so
	// control never falls through and the back edge below is unreachable
	// but valid. Any other stop means the body fell through to it, and the
	// back edge is the loop's.
	if next := e.emitArm(body); next == "" {
		e.out.WriteString("      ;; arm diverged; back edge unreachable\n")
	} else {
		fmt.Fprintf(&e.out, "        br %s\n", loopLabel)
	}
	e.labels = saved
	e.out.WriteString("      )\n")
	e.out.WriteString("    )\n")
}

// loopExitOf answers which block follows a loop: the branch's exit arm, or
// the next layout block when the header has no decomposable branch.
func (v *fnView) loopExitOf(header string) string {
	if _, exit, ok := v.loopBodyExit(header); ok {
		return exit
	}
	if pos, ok := v.pos[header]; ok && pos+1 < len(v.order) {
		return v.order[pos+1]
	}
	return ""
}

func copyLabels(in map[string]string) map[string]string {
	out := make(map[string]string, len(in)+2)
	for k, v := range in {
		out[k] = v
	}
	return out
}

// emitInstructions renders a block's instructions; every sequence is stack
// neutral except where noted, because values flow through locals.
func (e *watEmitter) emitInstructions(bb *mir.BasicBlock) {
	if bb == nil {
		return
	}
	fmt.Fprintf(&e.out, "    ;; block %s\n", bb.ID)
	for _, inst := range bb.Instructions {
		e.emitInstruction(inst)
	}
}

func (e *watEmitter) emitInstruction(inst mir.Instruction) {
	switch i := inst.(type) {
	case mir.StoreVarInst:
		e.emitOperand(i.Src)
		fmt.Fprintf(&e.out, "    local.set $%s\n", e.loc.varSlot(i.Name))
		e.loc.setKind(mir.VarOperand{Name: i.Name}, e.loc.isFloatOperand(i.Src))
	case mir.LoadVarInst:
		fmt.Fprintf(&e.out, "    local.get $%s\n", e.loc.varSlot(i.Name))
		fmt.Fprintf(&e.out, "    local.set %s\n", e.loc.watName(i.Dest))
		e.loc.setKind(i.Dest, e.loc.isFloatOperand(mir.VarOperand{Name: i.Name}))
	case mir.AssignInst:
		e.emitOperand(i.Src)
		fmt.Fprintf(&e.out, "    local.set %s\n", e.loc.watName(i.Dest))
		e.loc.setKind(i.Dest, e.loc.isFloatOperand(i.Src))
	case mir.BinaryOpInst:
		e.emitBinaryOp(i)
	case mir.UnaryOpInst:
		e.emitUnaryOp(i)
	case mir.SelectInst:
		e.emitOperand(i.Cond)
		e.out.WriteString("    i32.wrap_i64\n")
		e.out.WriteString("    if (result i64)\n")
		e.out.WriteString("    (then\n")
		e.emitOperand(i.True)
		e.out.WriteString("    )\n")
		e.out.WriteString("    (else\n")
		e.emitOperand(i.False)
		e.out.WriteString("    )\n")
		e.out.WriteString("    end\n")
		fmt.Fprintf(&e.out, "    local.set %s\n", e.loc.watName(i.Dest))
		e.loc.setKind(i.Dest, e.loc.isFloatOperand(i.True) || e.loc.isFloatOperand(i.False))
	case mir.CallInst:
		e.emitCall(i)
	case mir.AggregateInst:
		// L7.5 stub: lists and hashes have no ABI in linear memory yet, so
		// the elements are evaluated for their side effects and the slot
		// takes zero. Memory and the data section below are real; aggregate
		// allocation is the documented gap.
		for _, elem := range i.Elems {
			e.emitOperand(elem)
			e.out.WriteString("    drop\n")
		}
		kind := "list"
		if i.Hash {
			kind = "hash"
		}
		fmt.Fprintf(&e.out, "    ;; aggregate %s: no linear-memory ABI yet\n", kind)
		e.out.WriteString("    i64.const 0\n")
		fmt.Fprintf(&e.out, "    local.set %s\n", e.loc.watName(i.Dest))
		e.loc.setKind(i.Dest, false)
	case mir.SetIndexInst:
		// L7.5 stub: no container ABI yet, so the address and key are
		// discarded and the slot takes the stored value. The tee keeps the
		// assignment-as-expression value the IR promises.
		e.emitOperand(i.Container)
		e.out.WriteString("    drop\n")
		e.emitOperand(i.Key)
		e.out.WriteString("    drop\n")
		e.emitOperand(i.Value)
		if i.Dest != nil {
			fmt.Fprintf(&e.out, "    local.tee %s\n", e.loc.watName(i.Dest))
			e.loc.setKind(i.Dest, e.loc.isFloatOperand(i.Value))
		}
		e.out.WriteString("    drop\n")
	default:
		fmt.Fprintf(&e.out, "    ;; unsupported %T\n", inst)
	}
}

// emitBinaryOp renders left, right and the operator, then stores the result.
// Float arithmetic rides the reinterpret sandwich so every local stays i64:
// int sides convert, float-bits sides reinterpret, and the f64 result is
// reinterpreted back. Comparisons extend their i32 result to i64.
func (e *watEmitter) emitBinaryOp(i mir.BinaryOpInst) {
	leftFloat := e.loc.isFloatOperand(i.Left)
	rightFloat := e.loc.isFloatOperand(i.Right)
	if opcode, ok := floatBinOp(i.Op); ok && (leftFloat || rightFloat) {
		_ = opcode
		e.emitOperand(i.Left)
		if leftFloat {
			e.out.WriteString("    f64.reinterpret_i64\n")
		} else {
			e.out.WriteString("    f64.convert_i64_s\n")
		}
		e.emitOperand(i.Right)
		if rightFloat {
			e.out.WriteString("    f64.reinterpret_i64\n")
		} else {
			e.out.WriteString("    f64.convert_i64_s\n")
		}
		fmt.Fprintf(&e.out, "    %s\n", watBinOp(i.Op, true))
		e.out.WriteString("    i64.reinterpret_f64\n")
		fmt.Fprintf(&e.out, "    local.set %s\n", e.loc.watName(i.Dest))
		e.loc.setKind(i.Dest, true)
		return
	}
	if opcode, ok := floatCmpOp(i.Op); ok && (leftFloat || rightFloat) {
		_ = opcode
		e.emitOperand(i.Left)
		if leftFloat {
			e.out.WriteString("    f64.reinterpret_i64\n")
		} else {
			e.out.WriteString("    f64.convert_i64_s\n")
		}
		e.emitOperand(i.Right)
		if rightFloat {
			e.out.WriteString("    f64.reinterpret_i64\n")
		} else {
			e.out.WriteString("    f64.convert_i64_s\n")
		}
		fmt.Fprintf(&e.out, "    %s\n", watBinOp(i.Op, true))
		e.out.WriteString("    i64.extend_i32_u\n")
		fmt.Fprintf(&e.out, "    local.set %s\n", e.loc.watName(i.Dest))
		e.loc.setKind(i.Dest, false)
		return
	}
	e.emitOperand(i.Left)
	e.emitOperand(i.Right)
	fmt.Fprintf(&e.out, "    %s\n", watBinOp(i.Op, false))
	if isComparisonOp(i.Op) {
		e.out.WriteString("    i64.extend_i32_u\n")
	}
	fmt.Fprintf(&e.out, "    local.set %s\n", e.loc.watName(i.Dest))
	e.loc.setKind(i.Dest, false)
}

func (e *watEmitter) emitUnaryOp(i mir.UnaryOpInst) {
	switch i.Op {
	case "-":
		if e.loc.isFloatOperand(i.Right) {
			e.emitOperand(i.Right)
			e.out.WriteString("    f64.reinterpret_i64\n")
			e.out.WriteString("    f64.neg\n")
			e.out.WriteString("    i64.reinterpret_f64\n")
			fmt.Fprintf(&e.out, "    local.set %s\n", e.loc.watName(i.Dest))
			e.loc.setKind(i.Dest, true)
			return
		}
		e.out.WriteString("    i64.const 0\n")
		e.emitOperand(i.Right)
		e.out.WriteString("    i64.sub\n")
		fmt.Fprintf(&e.out, "    local.set %s\n", e.loc.watName(i.Dest))
		e.loc.setKind(i.Dest, false)
	default:
		// "!" and anything else behave as truthiness: eqz maps zero to one.
		e.emitOperand(i.Right)
		e.out.WriteString("    i64.eqz\n")
		e.out.WriteString("    i64.extend_i32_u\n")
		fmt.Fprintf(&e.out, "    local.set %s\n", e.loc.watName(i.Dest))
		e.loc.setKind(i.Dest, false)
	}
}

// emitCall renders args and a call (L7.3). Known user functions call by
// index; console output calls the puts import with an i32 wrap; anything
// else is a stub zero with a comment, because inventing a result would be
// worse than admitting the builtin is unimplemented.
func (e *watEmitter) emitCall(i mir.CallInst) {
	if idx, ok := e.funcIndex[i.Callee]; ok {
		for _, arg := range i.Args {
			e.emitOperand(arg)
		}
		fmt.Fprintf(&e.out, "    call $%s ;; funcidx %d\n", i.Callee, idx)
		e.finishCallValue(i.Dest)
		return
	}
	switch i.Callee {
	case "puts", "print", "println":
		if len(i.Args) > 0 {
			e.emitOperand(i.Args[0])
		} else {
			e.out.WriteString("    i64.const 0\n")
		}
		e.out.WriteString("    i32.wrap_i64\n")
		e.out.WriteString("    call $puts\n")
		if i.Dest != nil {
			e.out.WriteString("    i64.const 0\n")
			fmt.Fprintf(&e.out, "    local.set %s\n", e.loc.watName(i.Dest))
			e.loc.setKind(i.Dest, false)
		}
	default:
		for _, arg := range i.Args {
			e.emitOperand(arg)
			e.out.WriteString("    drop\n")
		}
		fmt.Fprintf(&e.out, "    ;; unimplemented builtin %q: stub zero\n", i.Callee)
		if i.Dest != nil {
			e.out.WriteString("    i64.const 0\n")
			fmt.Fprintf(&e.out, "    local.set %s\n", e.loc.watName(i.Dest))
			e.loc.setKind(i.Dest, false)
		}
	}
}

// finishCallValue stores or drops a user-function result. Every emitted
// function returns i64, so the value is always there.
func (e *watEmitter) finishCallValue(dest mir.Operand) {
	if dest != nil {
		fmt.Fprintf(&e.out, "    local.set %s\n", e.loc.watName(dest))
		e.loc.setKind(dest, false)
	} else {
		e.out.WriteString("    drop\n")
	}
}

func (e *watEmitter) emitOperand(op mir.Operand) {
	switch o := op.(type) {
	case mir.ConstOperand:
		e.emitConst(o.Value)
	case mir.VarOperand:
		slot := e.loc.varSlot(o.Name)
		if _, ok := e.loc.index[slot]; !ok {
			fmt.Fprintf(&e.out, "    ;; use of undefined variable %q\n", o.Name)
			e.out.WriteString("    i64.const 0\n")
			return
		}
		fmt.Fprintf(&e.out, "    local.get $%s\n", slot)
	case mir.TempOperand:
		name := fmt.Sprintf("t_%d", o.ID)
		if _, ok := e.loc.index[name]; !ok {
			e.out.WriteString("    ;; use of undefined temp\n")
			e.out.WriteString("    i64.const 0\n")
			return
		}
		fmt.Fprintf(&e.out, "    local.get $%s\n", name)
	default:
		e.out.WriteString("    i64.const 0\n")
	}
}

// emitConst pushes a constant. Floats ride bit-cast to i64 and strings ride
// as data-segment offsets extended to i64, so the stack shape never varies.
func (e *watEmitter) emitConst(value interface{}) {
	switch v := value.(type) {
	case int64:
		fmt.Fprintf(&e.out, "    i64.const %d\n", v)
	case int:
		fmt.Fprintf(&e.out, "    i64.const %d\n", v)
	case float64:
		fmt.Fprintf(&e.out, "    f64.const %g\n", v)
		e.out.WriteString("    i64.reinterpret_f64\n")
	case bool:
		if v {
			e.out.WriteString("    i64.const 1\n")
		} else {
			e.out.WriteString("    i64.const 0\n")
		}
	case string:
		off := e.strings.intern(v)
		fmt.Fprintf(&e.out, "    i32.const %d\n", off)
		e.out.WriteString("    i64.extend_i32_u\n")
	case nil:
		e.out.WriteString("    i64.const 0\n")
	default:
		e.out.WriteString("    i64.const 0\n")
	}
}

// CompileToBinary generates standard WebAssembly binary (.wasm)
//
// Section order follows the specification: type, import, function, memory,
// export, code, data (L7.6). Function indices match the WAT emitter: 0 is
// the puts import, then user functions in sorted order, then main.
func (c *Compiler) CompileToBinary(prog *mir.Program) ([]byte, error) {
	strtab := collectStrings(prog)
	userNames := sortedFunctionNames(prog)

	funcIndex := map[string]int{}
	idx := 1
	for _, name := range userNames {
		funcIndex[name] = idx
		idx++
	}
	hasMain := prog.Main != nil
	mainIdx := -1
	if hasMain {
		mainIdx = idx
	}

	var buf bytes.Buffer

	// Magic + version.
	buf.Write([]byte{0x00, 0x61, 0x73, 0x6d})
	buf.Write([]byte{0x01, 0x00, 0x00, 0x00})

	// Type section (ID 1): type 0 is (i32) -> () for puts; every user
	// function and main share (i64*) -> i64 per arity (L7.4).
	arityList := distinctArities(prog, userNames, hasMain)
	typeBytes := encodeSection(1, func(b *bytes.Buffer) {
		writeVarUint(b, uint64(1+len(arityList)))
		b.WriteByte(0x60)
		writeVarUint(b, 1)
		b.WriteByte(vtI32)
		writeVarUint(b, 0)
		for _, arity := range arityList {
			b.WriteByte(0x60)
			writeVarUint(b, uint64(arity))
			for i := 0; i < arity; i++ {
				b.WriteByte(vtI64)
			}
			writeVarUint(b, 1)
			b.WriteByte(vtI64)
		}
	})
	buf.Write(typeBytes)

	typeOfArity := func(arity int) uint64 {
		for i, a := range arityList {
			if a == arity {
				return uint64(1 + i)
			}
		}
		return 1
	}

	// Import section (ID 2): env.puts as func 0.
	importSec := encodeSection(2, func(b *bytes.Buffer) {
		writeVarUint(b, 1)
		writeString(b, "env")
		writeString(b, "puts")
		b.WriteByte(0x00)
		writeVarUint(b, 0)
	})
	buf.Write(importSec)

	// Function section (ID 3): user functions then main.
	funcSec := encodeSection(3, func(b *bytes.Buffer) {
		writeVarUint(b, uint64(len(userNames)+boolCount(hasMain)))
		for _, name := range userNames {
			writeVarUint(b, typeOfArity(len(prog.Functions[name].Params)))
		}
		if hasMain {
			writeVarUint(b, typeOfArity(len(prog.Main.Params)))
		}
	})
	buf.Write(funcSec)

	// Memory section (ID 5): one linear memory for string data (L7.5).
	pages := uint64(1)
	if need := uint64(len(strtab.bytes)/65536 + 1); need > pages {
		pages = need
	}
	memorySec := encodeSection(5, func(b *bytes.Buffer) {
		writeVarUint(b, 1)
		b.WriteByte(0x00)
		writeVarUint(b, pages)
	})
	buf.Write(memorySec)

	// Export section (ID 7): memory and main.
	exportSec := encodeSection(7, func(b *bytes.Buffer) {
		count := 1 + boolCount(hasMain)
		writeVarUint(b, uint64(count))
		writeString(b, "memory")
		b.WriteByte(0x02)
		writeVarUint(b, 0)
		if hasMain {
			writeString(b, "main")
			b.WriteByte(0x00)
			writeVarUint(b, uint64(mainIdx))
		}
	})
	buf.Write(exportSec)

	// Code section (ID 10).
	codeSec := encodeSection(10, func(b *bytes.Buffer) {
		writeVarUint(b, uint64(len(userNames)+boolCount(hasMain)))
		for _, name := range userNames {
			emitFuncBody(b, prog.Functions[name], funcIndex, strtab)
		}
		if hasMain {
			emitFuncBody(b, prog.Main, funcIndex, strtab)
		}
	})
	buf.Write(codeSec)

	// Data section (ID 11): the interned strings.
	if len(strtab.bytes) > 0 {
		dataSec := encodeSection(11, func(b *bytes.Buffer) {
			writeVarUint(b, 1)
			b.WriteByte(0x00)
			b.WriteByte(opI32Const)
			writeVarUint(b, 0)
			b.WriteByte(opEnd)
			writeVarUint(b, uint64(len(strtab.bytes)))
			b.Write(strtab.bytes)
		})
		buf.Write(dataSec)
	}

	return buf.Bytes(), nil
}

// distinctArities lists every parameter count used, ascending.
func distinctArities(prog *mir.Program, userNames []string, hasMain bool) []int {
	seen := map[int]bool{}
	var out []int
	add := func(arity int) {
		if !seen[arity] {
			seen[arity] = true
			out = append(out, arity)
		}
	}
	for _, name := range userNames {
		add(len(prog.Functions[name].Params))
	}
	if hasMain {
		add(len(prog.Main.Params))
	}
	sort.Ints(out)
	return out
}

// boolCount renders a presence flag as 0 or 1 for section counts.
func boolCount(b bool) int {
	if b {
		return 1
	}
	return 0
}

// emitFuncBody writes one code entry: locals followed by the region
// expression and end.
func emitFuncBody(b *bytes.Buffer, fn *mir.Function, funcIndex map[string]int, strtab *strTable) {
	loc := newLocals()
	loc.allocate(fn)
	var body bytes.Buffer
	if n := loc.count(); n > 0 {
		writeVarUint(&body, 1)
		writeVarUint(&body, uint64(n))
		body.WriteByte(vtI64)
	} else {
		writeVarUint(&body, 0)
	}
	e := &binEmitter{view: newFnView(fn), loc: loc, funcIndex: funcIndex, strings: strtab, visited: map[string]bool{}, labels: map[string]uint32{}, depths: map[string]uint32{}}
	if len(fn.Blocks) > 0 {
		e.emitSeq(fn.Blocks[0].ID)
	}
	body.Write(e.out.Bytes())
	// Fall-off-the-end zero, mirroring the WAT emitter.
	body.WriteByte(opI64Const)
	writeVarInt(&body, 0)
	body.WriteByte(opEnd)

	writeVarUint(b, uint64(body.Len()))
	b.Write(body.Bytes())
}

// binEmitter mirrors watEmitter into opcodes. Labels map block IDs to the
// relative depth their br targets: depth 0 is the innermost enclosing
// block/loop.
type binEmitter struct {
	view      *fnView
	loc       *locals
	funcIndex map[string]int
	strings   *strTable
	out       bytes.Buffer
	visited   map[string]bool
	labels    map[string]uint32
	depths    map[string]uint32
	stack     []string
}

// pushLabel opens a block/loop scope for id.
func (e *binEmitter) pushLabel(id string) {
	e.stack = append(e.stack, id)
	e.refreshDepths()
}

// popLabel closes the innermost scope.
func (e *binEmitter) popLabel() {
	if len(e.stack) > 0 {
		e.stack = e.stack[:len(e.stack)-1]
	}
	e.refreshDepths()
}

func (e *binEmitter) refreshDepths() {
	e.depths = map[string]uint32{}
	for i, id := range e.stack {
		e.depths[id] = uint32(len(e.stack) - 1 - i)
	}
}

// emitSeq emits a block and whatever linearly follows it.
func (e *binEmitter) emitSeq(id string) {
	cur := id
	for cur != "" {
		if e.visited[cur] {
			if depth, ok := e.depths[cur]; ok {
				e.out.WriteByte(opBr)
				writeVarUint(&e.out, uint64(depth))
			} else {
				e.out.WriteByte(0x00) // unreachable: no structured target
			}
			return
		}
		bb, ok := e.view.blocks[cur]
		if !ok {
			e.out.WriteByte(0x00) // unreachable
			return
		}
		e.visited[cur] = true
		e.emitInstructions(bb)
		switch term := bb.Terminator.(type) {
		case mir.ReturnTerminator:
			if term.Value != nil {
				e.emitOperand(term.Value)
			} else {
				e.out.WriteByte(opI64Const)
				writeVarInt(&e.out, 0)
			}
			e.out.WriteByte(opReturn)
			return
		case mir.JumpTerminator:
			target := term.Target
			if e.view.isHeader[target] && !e.visited[target] {
				e.emitLoop(target)
				cur = e.view.loopExitOf(target)
				continue
			}
			if _, ok := e.depths[target]; ok {
				e.out.WriteByte(opBr)
				writeVarUint(&e.out, uint64(e.depths[target]))
				return
			}
			cur = target
			continue
		case mir.BranchTerminator:
			if e.view.isHeader[cur] {
				if _, _, ok := e.view.loopBodyExit(cur); ok {
					e.emitLoop(cur)
					cur = e.view.loopExitOf(cur)
					continue
				}
			}
			cur = e.emitBranch(term)
			continue
		default:
			cur = e.nextInOrder(cur)
			continue
		}
	}
}

func (e *binEmitter) nextInOrder(id string) string {
	pos, ok := e.view.pos[id]
	if !ok {
		return ""
	}
	for i := pos + 1; i < len(e.view.order); i++ {
		if !e.visited[e.view.order[i]] {
			return e.view.order[i]
		}
	}
	return ""
}

// emitArm emits one if-arm and returns the block it stopped at.
func (e *binEmitter) emitArm(id string) string {
	cur := id
	for cur != "" {
		if e.visited[cur] {
			if depth, ok := e.depths[cur]; ok {
				e.out.WriteByte(opBr)
				writeVarUint(&e.out, uint64(depth))
			} else {
				e.out.WriteByte(0x00) // unreachable: no structured target
			}
			return ""
		}
		bb, ok := e.view.blocks[cur]
		if !ok {
			e.out.WriteByte(0x00)
			return ""
		}
		e.visited[cur] = true
		e.emitInstructions(bb)
		switch term := bb.Terminator.(type) {
		case mir.ReturnTerminator:
			if term.Value != nil {
				e.emitOperand(term.Value)
			} else {
				e.out.WriteByte(opI64Const)
				writeVarInt(&e.out, 0)
			}
			e.out.WriteByte(opReturn)
			return ""
		case mir.JumpTerminator:
			target := term.Target
			if depth, ok := e.depths[target]; ok {
				e.out.WriteByte(opBr)
				writeVarUint(&e.out, uint64(depth))
				return ""
			}
			if e.view.isHeader[target] && !e.visited[target] {
				e.emitLoop(target)
				cur = e.view.loopExitOf(target)
				continue
			}
			return target
		case mir.BranchTerminator:
			if e.view.isHeader[cur] {
				if _, _, ok := e.view.loopBodyExit(cur); ok {
					e.emitLoop(cur)
					cur = e.view.loopExitOf(cur)
					continue
				}
			}
			next := e.emitBranch(term)
			if next == "" {
				return ""
			}
			cur = next
			continue
		default:
			cur = e.nextInOrder(cur)
			continue
		}
	}
	return ""
}

// emitBranch renders cond, an if/else pair and end (0x04/0x05/0x0b), and
// returns the block emission continues with.
func (e *binEmitter) emitBranch(term mir.BranchTerminator) string {
	e.emitOperand(term.Cond)
	e.out.WriteByte(opI32WrapI64)
	e.out.WriteByte(opIf)
	e.out.WriteByte(btEmpty)
	thenNext := e.emitArm(term.TrueTarget)
	e.out.WriteByte(opElse)
	elseNext := e.emitArm(term.FalseTarget)
	e.out.WriteByte(opEnd)
	if thenNext == elseNext {
		return thenNext
	}
	if thenNext == "" {
		return elseNext
	}
	if elseNext == "" {
		return thenNext
	}
	return ""
}

// emitLoop renders a block/loop pair (0x02/0x03) with br (0x0c) and br_if
// (0x0d) edges (L7.1).
func (e *binEmitter) emitLoop(header string) {
	body, exit, ok := e.view.loopBodyExit(header)
	if !ok {
		e.out.WriteByte(opBlock)
		e.out.WriteByte(btEmpty)
		e.pushLabel("block_" + header)
		e.out.WriteByte(opLoop)
		e.out.WriteByte(btEmpty)
		e.pushLabel(header)
		e.emitSeq(header)
		e.out.WriteByte(opBr)
		writeVarUint(&e.out, uint64(e.depths[header]))
		e.popLabel()
		e.out.WriteByte(opEnd)
		e.popLabel()
		e.out.WriteByte(opEnd)
		return
	}
	e.out.WriteByte(opBlock)
	e.out.WriteByte(btEmpty)
	e.pushLabel(exit)
	e.out.WriteByte(opLoop)
	e.out.WriteByte(btEmpty)
	e.pushLabel(header)
	if bb, found := e.view.blocks[header]; found {
		e.visited[header] = true
		e.emitInstructions(bb)
		if branch, isBranch := bb.Terminator.(mir.BranchTerminator); isBranch {
			e.emitOperand(branch.Cond)
			e.out.WriteByte(opI64Eqz)
			e.out.WriteByte(opBrIf)
			writeVarUint(&e.out, uint64(e.depths[exit]))
		}
	}
	// The back edge is unreachable but valid when the arm diverged
	// (returned or jumped out through a label); otherwise it is the loop's.
	e.emitArm(body)
	e.out.WriteByte(opBr)
	writeVarUint(&e.out, uint64(e.depths[header]))
	e.popLabel()
	e.out.WriteByte(opEnd)
	e.popLabel()
	e.out.WriteByte(opEnd)
}

func (e *binEmitter) emitInstructions(bb *mir.BasicBlock) {
	if bb == nil {
		return
	}
	for _, inst := range bb.Instructions {
		e.emitInstruction(inst)
	}
}

func (e *binEmitter) emitInstruction(inst mir.Instruction) {
	switch i := inst.(type) {
	case mir.StoreVarInst:
		e.emitOperand(i.Src)
		e.out.WriteByte(opLocalSet)
		writeVarUint(&e.out, uint64(e.loc.index[e.loc.varSlot(i.Name)]))
		e.loc.setKind(mir.VarOperand{Name: i.Name}, e.loc.isFloatOperand(i.Src))
	case mir.LoadVarInst:
		e.out.WriteByte(opLocalGet)
		writeVarUint(&e.out, uint64(e.loc.index[e.loc.varSlot(i.Name)]))
		e.out.WriteByte(opLocalSet)
		writeVarUint(&e.out, e.loc.binIndex(i.Dest))
		e.loc.setKind(i.Dest, e.loc.isFloatOperand(mir.VarOperand{Name: i.Name}))
	case mir.AssignInst:
		e.emitOperand(i.Src)
		e.out.WriteByte(opLocalSet)
		writeVarUint(&e.out, e.loc.binIndex(i.Dest))
		e.loc.setKind(i.Dest, e.loc.isFloatOperand(i.Src))
	case mir.BinaryOpInst:
		e.emitBinaryOp(i)
	case mir.UnaryOpInst:
		e.emitUnaryOp(i)
	case mir.SelectInst:
		e.emitOperand(i.Cond)
		e.out.WriteByte(opI32WrapI64)
		e.out.WriteByte(opIf)
		e.out.WriteByte(vtI64)
		e.emitOperand(i.True)
		e.out.WriteByte(opElse)
		e.emitOperand(i.False)
		e.out.WriteByte(opEnd)
		e.out.WriteByte(opLocalSet)
		writeVarUint(&e.out, e.loc.binIndex(i.Dest))
		e.loc.setKind(i.Dest, e.loc.isFloatOperand(i.True) || e.loc.isFloatOperand(i.False))
	case mir.CallInst:
		e.emitCall(i)
	case mir.AggregateInst:
		for _, elem := range i.Elems {
			e.emitOperand(elem)
			e.out.WriteByte(opDrop)
		}
		e.out.WriteByte(opI64Const)
		writeVarInt(&e.out, 0)
		e.out.WriteByte(opLocalSet)
		writeVarUint(&e.out, e.loc.binIndex(i.Dest))
		e.loc.setKind(i.Dest, false)
	case mir.SetIndexInst:
		e.emitOperand(i.Container)
		e.out.WriteByte(opDrop)
		e.emitOperand(i.Key)
		e.out.WriteByte(opDrop)
		e.emitOperand(i.Value)
		if i.Dest != nil {
			e.out.WriteByte(opLocalTee)
			writeVarUint(&e.out, e.loc.binIndex(i.Dest))
			e.loc.setKind(i.Dest, e.loc.isFloatOperand(i.Value))
		}
		e.out.WriteByte(opDrop)
	default:
		e.out.WriteByte(opI64Const)
		writeVarInt(&e.out, 0)
		e.out.WriteByte(opDrop)
	}
}

func (e *binEmitter) emitBinaryOp(i mir.BinaryOpInst) {
	leftFloat := e.loc.isFloatOperand(i.Left)
	rightFloat := e.loc.isFloatOperand(i.Right)
	emitSide := func(op mir.Operand, isFloat bool) {
		e.emitOperand(op)
		if isFloat {
			e.out.WriteByte(opF64ReinterpretI64)
		} else {
			e.out.WriteByte(opF64ConvertI64S)
		}
	}
	if opcode, ok := floatBinOp(i.Op); ok && (leftFloat || rightFloat) {
		emitSide(i.Left, leftFloat)
		emitSide(i.Right, rightFloat)
		e.out.WriteByte(opcode)
		e.out.WriteByte(opI64ReinterpretF64)
		e.out.WriteByte(opLocalSet)
		writeVarUint(&e.out, e.loc.binIndex(i.Dest))
		e.loc.setKind(i.Dest, true)
		return
	}
	if opcode, ok := floatCmpOp(i.Op); ok && (leftFloat || rightFloat) {
		emitSide(i.Left, leftFloat)
		emitSide(i.Right, rightFloat)
		e.out.WriteByte(opcode)
		e.out.WriteByte(opI64ExtendI32U)
		e.out.WriteByte(opLocalSet)
		writeVarUint(&e.out, e.loc.binIndex(i.Dest))
		e.loc.setKind(i.Dest, false)
		return
	}
	e.emitOperand(i.Left)
	e.emitOperand(i.Right)
	e.out.WriteByte(intBinOp(i.Op))
	if isComparisonOp(i.Op) {
		e.out.WriteByte(opI64ExtendI32U)
	}
	e.out.WriteByte(opLocalSet)
	writeVarUint(&e.out, e.loc.binIndex(i.Dest))
	e.loc.setKind(i.Dest, false)
}

func (e *binEmitter) emitUnaryOp(i mir.UnaryOpInst) {
	if i.Op == "-" && !e.loc.isFloatOperand(i.Right) {
		e.out.WriteByte(opI64Const)
		writeVarInt(&e.out, 0)
		e.emitOperand(i.Right)
		e.out.WriteByte(opI64Sub)
		e.out.WriteByte(opLocalSet)
		writeVarUint(&e.out, e.loc.binIndex(i.Dest))
		e.loc.setKind(i.Dest, false)
		return
	}
	if i.Op == "-" {
		e.emitOperand(i.Right)
		e.out.WriteByte(opF64ReinterpretI64)
		e.out.WriteByte(opF64Neg)
		e.out.WriteByte(opI64ReinterpretF64)
		e.out.WriteByte(opLocalSet)
		writeVarUint(&e.out, e.loc.binIndex(i.Dest))
		e.loc.setKind(i.Dest, true)
		return
	}
	e.emitOperand(i.Right)
	e.out.WriteByte(opI64Eqz)
	e.out.WriteByte(opI64ExtendI32U)
	e.out.WriteByte(opLocalSet)
	writeVarUint(&e.out, e.loc.binIndex(i.Dest))
	e.loc.setKind(i.Dest, false)
}

// emitCall renders args and a call (0x10, L7.3).
func (e *binEmitter) emitCall(i mir.CallInst) {
	if funcIdx, ok := e.funcIndex[i.Callee]; ok {
		for _, arg := range i.Args {
			e.emitOperand(arg)
		}
		e.out.WriteByte(opCall)
		writeVarUint(&e.out, uint64(funcIdx))
		if i.Dest != nil {
			e.out.WriteByte(opLocalSet)
			writeVarUint(&e.out, e.loc.binIndex(i.Dest))
			e.loc.setKind(i.Dest, false)
		} else {
			e.out.WriteByte(opDrop)
		}
		return
	}
	switch i.Callee {
	case "puts", "print", "println":
		if len(i.Args) > 0 {
			e.emitOperand(i.Args[0])
		} else {
			e.out.WriteByte(opI64Const)
			writeVarInt(&e.out, 0)
		}
		e.out.WriteByte(opI32WrapI64)
		e.out.WriteByte(opCall)
		writeVarUint(&e.out, 0)
		if i.Dest != nil {
			e.out.WriteByte(opI64Const)
			writeVarInt(&e.out, 0)
			e.out.WriteByte(opLocalSet)
			writeVarUint(&e.out, e.loc.binIndex(i.Dest))
			e.loc.setKind(i.Dest, false)
		}
	default:
		for _, arg := range i.Args {
			e.emitOperand(arg)
			e.out.WriteByte(opDrop)
		}
		e.out.WriteByte(opI64Const)
		writeVarInt(&e.out, 0)
		if i.Dest != nil {
			e.out.WriteByte(opLocalSet)
			writeVarUint(&e.out, e.loc.binIndex(i.Dest))
			e.loc.setKind(i.Dest, false)
		} else {
			e.out.WriteByte(opDrop)
		}
	}
}

func (e *binEmitter) emitOperand(op mir.Operand) {
	switch o := op.(type) {
	case mir.ConstOperand:
		e.emitConst(o.Value)
	case mir.VarOperand:
		e.out.WriteByte(opLocalGet)
		writeVarUint(&e.out, uint64(e.loc.index[e.loc.varSlot(o.Name)]))
	case mir.TempOperand:
		e.out.WriteByte(opLocalGet)
		writeVarUint(&e.out, e.loc.binIndex(op))
	default:
		e.out.WriteByte(opI64Const)
		writeVarInt(&e.out, 0)
	}
}

func (e *binEmitter) emitConst(value interface{}) {
	switch v := value.(type) {
	case int64:
		e.out.WriteByte(opI64Const)
		writeVarInt(&e.out, v)
	case int:
		e.out.WriteByte(opI64Const)
		writeVarInt(&e.out, int64(v))
	case float64:
		e.out.WriteByte(opF64Const)
		var scratch [8]byte
		binary.LittleEndian.PutUint64(scratch[:], math.Float64bits(v))
		e.out.Write(scratch[:])
		e.out.WriteByte(opI64ReinterpretF64)
	case bool:
		e.out.WriteByte(opI64Const)
		if v {
			writeVarInt(&e.out, 1)
		} else {
			writeVarInt(&e.out, 0)
		}
	case string:
		e.out.WriteByte(opI32Const)
		writeVarUint(&e.out, uint64(e.strings.intern(v)))
		e.out.WriteByte(opI64ExtendI32U)
	case nil:
		e.out.WriteByte(opI64Const)
		writeVarInt(&e.out, 0)
	default:
		e.out.WriteByte(opI64Const)
		writeVarInt(&e.out, 0)
	}
}

func encodeSection(id byte, writeContent func(b *bytes.Buffer)) []byte {
	var content bytes.Buffer
	writeContent(&content)

	var sec bytes.Buffer
	sec.WriteByte(id)
	writeVarUint(&sec, uint64(content.Len()))
	sec.Write(content.Bytes())
	return sec.Bytes()
}

func writeVarUint(b *bytes.Buffer, val uint64) {
	for {
		bByte := byte(val & 0x7F)
		val >>= 7
		if val != 0 {
			bByte |= 0x80
		}
		b.WriteByte(bByte)
		if val == 0 {
			break
		}
	}
}

func writeVarInt(b *bytes.Buffer, val int64) {
	more := true
	for more {
		bByte := byte(val & 0x7F)
		val >>= 7
		sign := bByte & 0x40
		if (val == 0 && sign == 0) || (val == -1 && sign != 0) {
			more = false
		} else {
			bByte |= 0x80
		}
		b.WriteByte(bByte)
	}
}

func writeString(b *bytes.Buffer, s string) {
	writeVarUint(b, uint64(len(s)))
	b.WriteString(s)
}

func (c *Compiler) Compile(prog *mir.Program) (*WasmModule, error) {
	wat, err := c.CompileToWAT(prog)
	if err != nil {
		return nil, err
	}
	bin, err := c.CompileToBinary(prog)
	if err != nil {
		return nil, err
	}
	return &WasmModule{
		WAT:    wat,
		Binary: bin,
	}, nil
}
