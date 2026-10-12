package llvm

import (
	"fmt"
	"sort"
	"strings"

	"github.com/joysriramsarkar/nilLang/compiler/mir"
)

// Emitter turns a MIR program into textual LLVM IR.
//
// # Memory model
//
// Every MIR variable becomes an `alloca` in the function's entry block. That is the
// simplest correct model and it is deliberately not the fast one: it gives every
// slot a name and an address, so a value can be read and written by name rather
// than tracked as an SSA value that a branch has to reconcile.
//
// LLVM's `mem2reg` promotes these to registers in one pass, so writing allocas is
// not a permanent cost — it is the cost of being obviously correct, paid once at a
// known optimisation step rather than spread through a hand-written promotion that
// would be wrong on the first loop it met.
//
// # What the emitter does not do
//
// It does not allocate or free memory, and it does not know what a Nilang list is.
// Those belong to the runtime library (`L4.8`), and the emitter calls into it
// through declared functions. A compiler that also implemented the runtime would
// have two implementations of the list type, free to disagree about its layout —
// and a disagreement there is a memory error rather than a wrong answer.
type Emitter struct {
	target  Target
	program *mir.Program

	// strings are the constants emitted so far, deduplicated by value.
	strings map[string]string

	// stringOrder preserves emission order so a module reads the same way twice.
	stringOrder []string

	// globals are compile-time constants, by name.
	globals map[string]string

	// current is the function being emitted.
	current *functionState

	// ffiDecls are the extra runtime symbols this module needs declared, beyond the
	// ones every module has.
	//
	// Collected while the foreign and indirect-call code runs, because
	// `declareRuntime` runs after every function has been emitted and a module has
	// to declare what its calls reference. Collecting is also what keeps a module
	// that crosses no boundary and calls no function value from mentioning any of
	// it, so a diff of two modules shows only what actually changed.
	ffiDecls []runtimeFunction

	// ffiStructs is the type declaration a string crossing needs, or "" when no
	// string crosses this module.
	//
	// A string is the one crossing that takes *two* LLVM arguments, and the runtime
	// returns them as a struct. The type therefore has to exist in the module — and
	// it is a field rather than another entry in `moduleTypes` so that a program
	// with no string crossing carries no trace of one.
	ffiStructs string

	// ffiGuard is the ABI version symbol this module is linked against, or "" when
	// the program declares no `extern`.
	//
	// A field rather than always emitting it, because a program that never crosses
	// a boundary has no version to be wrong about — and a module that links a
	// symbol it never uses is a module whose guard can be satisfied by anything
	// that happens to export it.
	ffiGuard string

	// takenNames is every local name already used in the current function.
	//
	// LLVM shares one namespace between allocas, calls, phis and loads, so a name
	// used twice is a "multiple definition" error rather than a redefinition.
	takenNames map[string]bool

	// entryBlockID is the MIR block emitted as LLVM's `entry`.
	entryBlockID string

	// entrySymbol is the module-level name of the program's top level.
	//
	// It is not `main`, because `main` is taken by a wrapper the emitter writes
	// itself. The reason is the shape of the answer: the interpreter and the VM both
	// leave the program's value somewhere — the evaluator returns it, the VM pops it
	// — and an ahead-of-time program has to put it somewhere a process boundary can
	// see it. Printing it is that boundary.
	//
	// Naming it `main` and printing from inside the program would mean a program's
	// output and its result were the same thing, and a conformance test could not
	// tell "printed the wrong value" from "printed the right value twice".
	entrySymbol string

	// pendingLoad holds instructions that must be written *before* the one being
	// emitted.
	//
	// A constant or a variable read produces a value the next instruction consumes,
	// and in LLVM the producing instruction has to come first. Rather than thread a
	// "before" string through every operand lookup, an operand queues its
	// instruction here and the emitter flushes before writing its own.
	pendingLoad []string
}

// functionState is everything one function needs while it is being emitted.
type functionState struct {
	// slots maps a MIR variable name to the alloca that holds it.
	slots map[string]string

	// counters produce unique SSA names within a function.
	counters map[string]int

	// takenNames is every local name already used in this function.
	//
	// LLVM shares one namespace between allocas, calls, phis and loads, so a name
	// used twice is a "multiple definition of local value" error — reported
	// against the second definition, which looks entirely ordinary.
	takenNames map[string]bool

	// terminatorWritten stops a second terminator being emitted into a block that
	// already ended. MIR guarantees one terminator per block; a second one would be
	// an invalid module rather than a subtle miscompile.
	terminatorWritten bool

	// renames maps a cross-block temporary to the block-local name this block
	// writes it under.
	//
	// `assignTo` consults it rather than every call site, because a temporary
	// joined by a phi is written by ordinary instructions that know nothing about
	// joins — and a call site that forgot to consult it would emit a name LLVM had
	// already seen in another block.
	renames map[string]string
}

// EntrySymbol is the module-level name the program's top level is emitted under.
//
// It defaults to `nilang_entry` and exists so a caller can rename it — a test that
// links two modules together, for instance, needs two entry points that do not
// collide. An empty string restores the default.
func (e *Emitter) EntrySymbol(name string) *Emitter {
	if name == "" {
		name = DefaultEntrySymbol
	}
	e.entrySymbol = name
	return e
}

// DefaultEntrySymbol is the name the program's top level gets.
const DefaultEntrySymbol = "nilang_entry"

// valueType is the LLVM type of every Nilang value.
//
// One type for everything — an integer, a boolean, a string, a list, a map and a
// function are all a tagged word in a register, so a function taking three
// different types takes three `i64`s. It is a constant rather than a lookup
// because the decision was made once, in the ABI, and every backend that consumes
// these values has already agreed to it.
const valueType = "i64"

func (e *Emitter) entry() string {
	if e.entrySymbol == "" {
		return DefaultEntrySymbol
	}
	return e.entrySymbol
}

// NewEmitter returns an emitter for a target.
func NewEmitter(target Target) *Emitter {
	return &Emitter{
		target:      target,
		strings:     make(map[string]string),
		globals:     make(map[string]string),
		entrySymbol: DefaultEntrySymbol,
	}
}

// queue records an instruction to be written before the current one.
func (e *Emitter) queue(line string) {
	e.pendingLoad = append(e.pendingLoad, line)
}

// emitFallOff writes the return for a path that reaches the end of a function
// without an explicit one.
//
// Every function answers nil. That is right rather than a leftover: a function
// whose branches all return except one has genuinely produced nothing on that path,
// and returning the last thing that happened to be in a register would report a
// value the program never chose.
func (e *Emitter) emitFallOff(b *strings.Builder, fn *mir.Function) {
	_ = fn
	b.WriteString("  ret i64 0\n")
}

// isEntry reports whether a function is the program's top level.
//
// By pointer identity of the MIR function rather than by name, because a program
// may define a function called `main` and that function is not the entry.
func (e *Emitter) isEntry(fn *mir.Function) bool {
	return fn != nil && e.program != nil && fn == e.program.Main
}

// emitTerminator writes the block's terminator.
//
// A block that ends in a jump gets `br label`; one that ends in a conditional
// branch gets a real `br i1`; one that ends in a return gets `ret`. A block with no
// terminator falls through to a `ret`, because LLVM requires every block to end with
// exactly one and a module missing one is rejected for a reason that has nothing to
// do with the code around it.
func (e *Emitter) emitTerminator(b *strings.Builder, term mir.Terminator, fn *mir.Function) error {
	e.flushPending(b)

	switch t := term.(type) {
	case nil:
		e.emitFallOff(b, fn)
		return nil

	case mir.ReturnTerminator:
		// A return with a value is the one place a function's answer is produced, so
		// it is worth going to the operand for rather than assuming it is nil. A
		// program's top level ends in one of these, and so does every function that
		// says what it computes.
		if t.Value != nil {
			value, err := e.operand(t.Value)
			if err != nil {
				return err
			}
			// Flushed here, not only at the end of the terminator: a constant
			// return value queues the instruction that produces it, and the `ret`
			// refers to it by name. Without this the module said "use of undefined
			// value %kint0" over a return that looks perfectly ordinary.
			e.flushPending(b)
			fmt.Fprintf(b, "  ret i64 %s\n", value)
			break
		}
		e.emitFallOff(b, fn)

	case mir.JumpTerminator:
		fmt.Fprintf(b, "  br label %%%s\n", e.blockLabel(t.Target))

	case mir.BranchTerminator:
		cond, err := e.operand(t.Cond)
		if err != nil {
			return err
		}
		e.flushPending(b)
		// The runtime answers an `i64`, because a Nilang value is one tagged word
		// and every function in the library takes and returns one. The branch wants
		// an `i1`, so the conversion happens here — and comparing against zero is
		// the only correct test, because the runtime's true and false are the two
		// tag values and not the integers 1 and 0.
		truthy := e.tmp("truthy")
		fmt.Fprintf(b, "  %%%s = call i64 @%s(i64 %s)\n", truthy, runtimeTruthy, cond)
		condBool := e.tmp("brcond")
		fmt.Fprintf(b, "  %%%s = icmp ne i64 %%%s, 0\n", condBool, truthy)
		fmt.Fprintf(b, "  br i1 %%%s, label %%%s, label %%%s\n",
			condBool, e.blockLabel(t.TrueTarget), e.blockLabel(t.FalseTarget))

	default:
		return fmt.Errorf("llvm: no terminator for %T", term)
	}

	e.current.terminatorWritten = true
	return nil
}

// flushPending writes any instructions an operand queued.
//
// It must run before every instruction the emitter writes. An operand that produced
// an instruction and did not get flushed is a use of an undefined value, which LLVM
// reports at the point of use rather than where the value was lost.
func (e *Emitter) flushPending(b *strings.Builder) {
	for _, line := range e.pendingLoad {
		b.WriteString(line)
	}
	e.pendingLoad = e.pendingLoad[:0]
}

// Emit renders a whole program as an LLVM module.
//
// It returns an error rather than emitting a partial module, because a truncated
// `.ll` file is a file that assembles into something that runs and is wrong.
func (e *Emitter) Emit(program *mir.Program) (string, error) {
	if program == nil {
		return "", fmt.Errorf("cannot emit IR for a nil program")
	}
	e.program = program

	var b strings.Builder
	b.WriteString("; Generated by the Nilang compiler. Do not edit.\n")
	// `source_filename` takes an ordinary string, not a byte literal. Only the
	// array constants at the bottom of the module take `c"..."`, and using the
	// wrong one here makes the whole module fail to parse at line two — an error
	// about a string constant that looks correct.
	fmt.Fprintf(&b, "source_filename = %q\n", programName(program))
	fmt.Fprintf(&b, "target datalayout = %q\n", e.target.DataLayout)
	fmt.Fprintf(&b, "target triple = %q\n\n", e.target.Triple)

	// The FFI's signatures are worked out *before* anything is written, because two
	// of the things they decide — the struct a string crossing needs, and whether
	// any conversion is called at all — have to appear near the top of the module.
	//
	// Deciding them during the function loop would be too late for the types and
	// exactly right for the declarations, and a struct written after its first use
	// is a module LLVM rejects for a reason that has nothing to do with the code.
	//
	// Collected rather than written inline because the errors have to surface
	// before a partial module exists.
	signatures, err := e.foreignSignatures(program)
	if err != nil {
		return "", err
	}

	for _, decl := range moduleTypes() {
		b.WriteString(decl + "\n")
	}
	// The struct a string crossing returns, declared only when one crosses.
	//
	// Declared here rather than in `moduleTypes` because that list is what every
	// module has, and a module that crosses no string would then carry a type it
	// never uses — which makes "did adding the FFI change this program?" a question
	// the diff cannot answer.
	if e.ffiStructs != "" {
		b.WriteString(e.ffiStructs)
	}
	b.WriteString("\n")

	e.emitGlobals(&b)

	if program.Main != nil {
		if err := e.emitFunction(&b, program.Main, e.entry()); err != nil {
			return "", err
		}
		b.WriteString(e.emitProcessEntry())
		b.WriteString("\n")
	}

	// Functions are emitted in a stable order. Map iteration order would make the
	// module text differ between runs of the same program, which breaks any build
	// cache keyed on it and makes a diff of two modules unreadable.
	names := make([]string, 0, len(program.Functions))
	for name := range program.Functions {
		names = append(names, name)
	}
	sort.Strings(names)

	// Which of them are named as values rather than called, worked out before any
	// of them is written: a shim must follow the function it adapts, and emission
	// walks the functions in the program's own order, so "have I seen this one yet"
	// is not a question this loop can answer as it goes.
	usedAsValues := functionsUsedAsValues(program)

	for _, name := range names {
		fn := program.Functions[name]
		if fn == nil {
			continue
		}
		if fn.Foreign {
			// A foreign function is declared, not defined — and declared with the
			// C signature, because that is what the linker has to match.
			//
			// The symbol is the bare C name with no `nil.` prefix: the prefix exists
			// to keep *this module's* symbols traceable back to the Nilang names
			// that produced them, and a C symbol has to be the name the linker will
			// look for. Prefixing it would declare a function nothing defines, and
			// link with "undefined reference to nil_sin".
			signature := signatures[name]
			b.WriteString(foreignDeclaration(fn, signature))
			continue
		}
		if err := e.emitFunction(&b, fn, llvmSymbol(name)); err != nil {
			return "", err
		}

		// A function used as a value also needs the adapter that turns the
		// closure's calling convention into this one's.
		//
		// Emitted here, right after the function it adapts, because the shim calls
		// it and a reader of the IR should meet the thing it forwards to before the
		// thing that forwards to it.
		if usedAsValues[name] {
			e.emitCallbackShim(&b, fn, llvmSymbol(name))
		}
	}

	b.WriteString(e.declareRuntime())
	b.WriteString("\n\nattributes #0 = { noinline nounwind }\n")
	if constants := e.emitStringConstants(); constants != "" {
		b.WriteString(constants)
	}

	return b.String(), nil
}

func programName(p *mir.Program) string {
	if p.Main != nil && p.Main.Name != "" {
		return p.Main.Name
	}
	return "nil"
}

// emitProcessEntry writes the `main` the operating system calls.
//
// # Why the program's answer is printed
//
// The interpreter returns it and the VM pops it; a compiled program is a process,
// and a process has nowhere to put a value except outside itself. Printing is that
// outside, and it is the same function every other print goes through, so "the
// program printed the wrong thing" is one comparison rather than two.
//
// # Why `nil_println` and not a buffer of our own
//
// The obvious version renders into a stack buffer and prints that:
//
//	%buffer = alloca [8192 x i8]
//	%length = call i64 @nil_render(i64 %result, ptr %buffer, i64 8192)
//	call void @nil_print_str(ptr %buffer)
//
// It renders once rather than twice, and it does not link.
//
// An 8KB frame exceeds the platform's stack-probe threshold, so the compiler emits
// a call to `__chkstk` — a C runtime function that a generated object has no
// business referring to, and one that the module's target triple may not even have.
// `nil_println` has the same buffer, in C, where the compiler knows what runtime it
// is building for.
//
// The general rule this follows: **generated code allocates registers, not stack.**
// Anything that needs a buffer belongs to the runtime, which is where the platform's
// calling conventions are already dealt with.
func (e *Emitter) emitProcessEntry() string {
	var b strings.Builder
	b.WriteString("define i32 @main() {\nentry:\n")

	// The ABI guard, called first and before anything else can fail.
	//
	// In `main` rather than in the program's entry because that is where every
	// module passes anyway, and a guard placed somewhere conditional could be
	// reached by nothing. It is called rather than merely declared because a
	// declaration the linker satisfies from somewhere else would be a guard that
	// guarded nothing — and calling it costs one empty function.
	//
	// Emitted only for a module that declares an `extern`. `e.ffiGuard` is empty
	// otherwise, and a program that never crosses a boundary has no runtime version
	// to disagree with.
	if e.ffiGuard != "" {
		fmt.Fprintf(&b, "  call void @%s()\n", e.ffiGuard)
	}

	fmt.Fprintf(&b, "  %%result = call i64 @%s()\n", e.entry())
	b.WriteString("  call void @nil_println(i64 %result)\n")
	b.WriteString("  ret i32 0\n}\n")
	return b.String()
}

// llvmSymbol maps a Nilang name onto a symbol LLVM accepts.
//
// Nilang allows a name to contain characters an LLVM symbol cannot. The `nil.`
// prefix keeps the module's symbols traceable back to what produced them, and the
// escaping keeps every one of them a legal identifier — a symbol LLVM rejects is
// rejected at the point of use, which is nowhere near the name that caused it.
func llvmSymbol(name string) string {
	if name == "" {
		return "nil.anonymous"
	}
	var b strings.Builder
	b.WriteString("nil.")
	for i := 0; i < len(name); i++ {
		c := name[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9', c == '_':
			b.WriteByte(c)
		default:
			fmt.Fprintf(&b, "x%02x", c)
		}
	}
	return b.String()
}

// emitGlobals writes the compile-time constants.
//
// A `const` becomes a module-level global rather than an instruction in `main`.
// That matters more than it looks: a constant used inside a loop becomes one load
// from a fixed address instead of a call, and a constant used in a hundred places
// becomes a hundred references to one value rather than a hundred computations.
func (e *Emitter) emitGlobals(b *strings.Builder) {
	if len(e.globals) == 0 {
		return
	}
	names := make([]string, 0, len(e.globals))
	for name := range e.globals {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		fmt.Fprintf(b, "@%s = global constant %s\n", name, e.globals[name])
	}
	b.WriteString("\n")
}

// emitFunction writes one function definition.
func (e *Emitter) emitFunction(b *strings.Builder, fn *mir.Function, symbol string) error {
	e.current = &functionState{
		slots:      make(map[string]string),
		counters:   make(map[string]int),
		takenNames: make(map[string]bool),
	}
	e.pendingLoad = nil
	if len(fn.Blocks) > 0 && fn.Blocks[0] != nil {
		e.entryBlockID = fn.Blocks[0].ID
	} else {
		e.entryBlockID = ""
	}

	var header strings.Builder
	header.WriteString("define ")
	// Every function returns an `i64`, the entry included.
	//
	// Only the entry *has* to — it is the one that hands a value back to the
	// process. But a `void` function cannot be called for its result, and emitting
	// `void` for a function the program calls for a value produces a module where
	// the result is undefined: LLVM accepts it, the linker accepts it, and the
	// program reads whatever happened to be in the register.
	//
	// One signature for all of them. The cost is a returned value a caller may
	// ignore; the benefit is that no caller can be handed an undefined one.
	header.WriteString("i64 ")
	header.WriteString("@" + symbol + "(")
	header.WriteString(strings.Join(paramTypes(fn), ", "))
	header.WriteString(") {\n")

	b.WriteString(header.String())

	// The allocas go in the first block, which LLVM requires to be called
	// `entry`.
	//
	// That name is not a convention here, it is part of the module: a function
	// whose first block is named anything else is rejected with "expected
	// instruction opcode", pointing at a label that looks entirely ordinary. The
	// MIR entry block is therefore emitted *as* `entry` rather than under its own
	// name, and a branch to it is rewritten to match.
	firstBlock := ""
	if len(fn.Blocks) > 0 && fn.Blocks[0] != nil {
		firstBlock = fn.Blocks[0].ID
	}
	b.WriteString("entry:\n")

	// Every variable gets its slot before any instruction, because an instruction
	// may write a variable a later block reads, and an `alloca` placed after that
	// instruction would be out of scope.
	for _, name := range sortedVariableNames(fn) {
		slot := e.tmp("slot")
		e.current.slots[name] = slot
		fmt.Fprintf(b, "  %%%s = alloca i64\n", slot)
	}

	// Parameters are stored into their slots on entry.
	//
	// MIR hands a function its arguments as LLVM parameters rather than as
	// variables, so without this a body that reads its own parameter reports "has
	// no slot" — an error naming a name that is plainly in scope on the line
	// above it.
	for index, param := range fn.Params {
		slot, known := e.current.slots[param]
		if !known {
			// The parameter is only read, never stored, so it did not get a slot
			// from the loop above. Give it one now, before the body writes it.
			slot = e.tmp("pslot")
			e.current.slots[param] = slot
			fmt.Fprintf(b, "  %%%s = alloca i64\n", slot)
		}
		fmt.Fprintf(b, "  store i64 %%%s, ptr %%%s, align 8\n", paramSlot(index), slot)
	}

	// A MIR temp assigned in more than one block is a cross-block value, and LLVM
	// needs a `phi` to join them.
	//
	// The first version emitted `%tN = add i64 ...` in each block, which defined the
	// same name twice and produced a module LLVM rejects with "multiple definition
	// of local value" — naming a line that looks perfectly ordinary. Each block now
	// writes its own copy under a block-local name and the merge block joins them
	// with a phi.
	phis := e.crossBlockPhis(fn)
	blockNames := e.blockLocalNames(fn, phis)

	for _, block := range fn.Blocks {
		if block == nil {
			continue
		}

		label := ""
		if block.ID != firstBlock {
			label = e.blockLabel(block.ID)
			fmt.Fprintf(b, "%s:\n", label)
		}
		e.current.terminatorWritten = false

		// The phis for this block go first, before anything else: LLVM requires
		// every phi in a block to be grouped at its top.
		for _, phi := range phis {
			if phi.mergeID != block.ID {
				continue
			}
			fmt.Fprintf(b, "  %%%s = %s\n", phi.name, phi.text(blockNames))
			e.current.takenNames[phi.name] = true
		}

		e.current.renames = blockNames[block.ID]
		for _, inst := range block.Instructions {
			if err := e.emitInstruction(b, inst); err != nil {
				return err
			}
		}
		if err := e.emitTerminator(b, block.Terminator, fn); err != nil {
			return err
		}
	}

	b.WriteString("}\n\n")
	e.current = nil
	return nil
}

// paramTypes is a function's parameter list.
//
// Every parameter is an `i64` for now. That is honest rather than convenient: the
// runtime's values are tagged, so an `i64` is a pointer, a boxed number or a
// pointer to a string. Giving each Nilang type its own LLVM signature would need
// the runtime to agree with the compiler about every layout, and the two are
// written by different people at different times.
func paramTypes(fn *mir.Function) []string {
	if len(fn.Params) == 0 {
		return nil
	}
	out := make([]string, len(fn.Params))
	for i := range fn.Params {
		out[i] = TypeI64.String()
	}
	return out
}

// sortedVariableNames is every name the function stores to, sorted.
//
// Sorted because the entry block's allocas would otherwise appear in map order,
// which changes between runs of the same program.
func sortedVariableNames(fn *mir.Function) []string {
	seen := make(map[string]bool)
	for _, block := range fn.Blocks {
		if block == nil {
			continue
		}
		for _, inst := range block.Instructions {
			if store, ok := inst.(mir.StoreVarInst); ok {
				seen[store.Name] = true
			}
		}
	}
	out := make([]string, 0, len(seen))
	for name := range seen {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// paramSlot is the LLVM reference for a function's Nth parameter.
//
// # Why the number, not the Nilang name
//
// The function header writes its parameters without names, and LLVM names an
// unnamed value by position. A reference to `%arg0` — which is what this used to
// return — is a reference to a name the function does not have, and clang reports
// "use of undefined value '%arg0'" on a line that reads perfectly ordinary.
//
// The numbering is stable because it depends on what is *unnamed*: only parameters
// are, so they take `%0`, `%1`, ... in order. Every temporary and every basic block
// this emitter writes carries a name, and named things do not consume numbers.
//
// Naming parameters after their Nilang names would read better and is deliberately
// not done: a program is free to name a parameter `t0`, and `t0` is the form this
// emitter gives a temporary. The two would collide, and the collision would be
// silent in the module and fatal in the assembler.
func paramSlot(index int) string {
	return itoa(index)
}

// blockLabel is the LLVM label for a MIR block.
//
// # It returns the name *without* a colon
//
// Every use then writes `%<name>:` for a branch target and `<name>:` for a
// definition. The colon belongs to the syntax, not to the name: a helper that
// included it and a caller that also appended it produce `bb_entry_0::`, which
// LLVM reports as an undefined label — an error naming a label that does not exist
// anywhere in the module.
//
// # And it renames the entry block
//
// LLVM requires a function's first block to be named `entry`. The MIR entry block
// is called `bb_entry_0`, so a branch to it is translated rather than passed
// through: leaving the name alone produces a module that mentions a label it
// never defines.
func (e *Emitter) blockLabel(id string) string {
	if id == e.entryBlockID {
		return "entry"
	}
	return id
}

// blockLabel is the free-function form, used where no emitter state is available.
func blockLabel(id string) string {
	return id
}

// tmp is a fresh SSA name within the current function.
//
// # One counter, not one per prefix
//
// LLVM requires every local name in a function to be unique — an `alloca` and a
// `call` cannot both be `%kint0`. A counter per prefix lets them collide, and the
// error it produces ("multiple definition of local value named 'kint0'") is
// reported against the *second* definition, which is a `call` that looks entirely
// ordinary and names nothing else.
//
// One counter for the function cannot collide, and the resulting names are still
// readable because the prefix travels with them.
//
// It is per-function rather than per-module because LLVM numbers values per
// function, and a module-wide counter would produce gaps that are legal but make
// two functions' text impossible to compare.
func (e *Emitter) tmp(prefix string) string {
	if e.current == nil {
		return prefix
	}
	for e.current.counters["value"] < 1<<30 {
		n := e.current.counters["value"]
		e.current.counters["value"] = n + 1
		name := prefix + itoa(n)
		if !e.current.takenNames[name] {
			e.current.takenNames[name] = true
			return name
		}
	}
	return prefix + "overflow"
}

// runtimeFunction is one function of the runtime library: the C symbol and the
// LLVM declaration for it, written once.
//
// # Why one table and not two lists
//
// There were two — a list of declarations here and a list of names in instr.go —
// and they drifted: the module declared `i1 @nil.truthy(i64)` while the C returned
// `int`, and declared `@nil.print` while the C exported `nil_print`. Neither mistake
// shows up until a program is *linked*, which is the last possible moment.
//
// The name and the signature now sit next to each other, and
// `TestEveryRuntimeSymbolIsDeclaredAndMatchesItsHeader` reads the C header and
// checks both against it.
type runtimeFunction struct {
	// Name is the C symbol, spelled exactly as the library exports it.
	Name string

	// Result is the LLVM return type: `i64`, `void`, and so on.
	Result string

	// Params is the comma-separated parameter list, without parentheses.
	Params string
}

// Arity is how many parameters the function takes, which is what a call site needs
// to know and what a test needs to check against the header.
func (f runtimeFunction) Arity() int {
	if f.Params == "" {
		return 0
	}
	count := 1
	for i := 0; i < len(f.Params); i++ {
		if f.Params[i] == ',' {
			count++
		}
	}
	return count
}

// Decl renders the module-level declaration.
func (f runtimeFunction) Decl() string {
	return "declare " + f.Result + " @" + f.Name + "(" + f.Params + ")"
}

// runtimeFunctions is every runtime symbol a generated module may refer to.
//
// They are declared rather than defined here: they live in the runtime library, and
// a module that defined them as well would end up with two of each.
//
// # Every function takes and returns `i64`
//
// Not a preference. The representation is one tagged word, so a Nilang value fits
// an `i64` and every operation on one starts and ends as an `i64`. A function that
// returned C's `int` would return in a 32-bit register with the upper half
// undefined, and a caller that declared it `i1` would read a bit that was never
// written — a wrong answer with no diagnostic, on the branch that every `if` in
// every program goes through.
var runtimeFunctions = []runtimeFunction{
	{Name: "nil_alloc", Result: "i64", Params: "i64, i64"},
	{Name: "nil_print", Result: "void", Params: "i64"},
	{Name: "nil_println", Result: "void", Params: "i64"},
	{Name: "nil_string_new", Result: "i64", Params: "ptr, i64"},
	{Name: "nil_string_lit", Result: "i64", Params: "ptr, i64"},
	{Name: "nil_string_len", Result: "i64", Params: "i64"},
	{Name: "nil_string_concat", Result: "i64", Params: "i64, i64"},
	{Name: "nil_int_new", Result: "i64", Params: "i64"},
	{Name: "nil_float_new", Result: "i64", Params: "double"},
	{Name: "nil_int_add", Result: "i64", Params: "i64, i64"},
	{Name: "nil_int_sub", Result: "i64", Params: "i64, i64"},
	{Name: "nil_int_mul", Result: "i64", Params: "i64, i64"},
	{Name: "nil_int_div", Result: "i64", Params: "i64, i64"},
	{Name: "nil_int_mod", Result: "i64", Params: "i64, i64"},
	{Name: "nil_float_add", Result: "i64", Params: "i64, i64"},
	{Name: "nil_float_sub", Result: "i64", Params: "i64, i64"},
	{Name: "nil_float_mul", Result: "i64", Params: "i64, i64"},
	{Name: "nil_float_div", Result: "i64", Params: "i64, i64"},
	{Name: "nil_int_cmp", Result: "i64", Params: "i64, i64, i64"},
	{Name: "nil_str_cmp", Result: "i64", Params: "i64, i64, i64"},
	{Name: "nil_array_new", Result: "i64", Params: "i64"},
	{Name: "nil_array_get", Result: "i64", Params: "i64, i64"},
	{Name: "nil_array_set", Result: "i64", Params: "i64, i64, i64"},
	{Name: "nil_array_len", Result: "i64", Params: "i64"},
	{Name: "nil_index_get", Result: "i64", Params: "i64, i64"},
	{Name: "nil_index_set", Result: "i64", Params: "i64, i64, i64"},

	// Callbacks.
	//
	// Declared unconditionally rather than only for a module that uses them,
	// because a function becomes a value the moment anything *reads* one — a
	// `let apply = twice;` with no `extern` anywhere still builds a closure. A
	// module that did not declare this would fail to link on a program that never
	// mentioned C at all, which is the hardest kind of missing symbol to connect
	// back to anything.
	{Name: "nil_callback_new", Result: "i64", Params: "ptr, i64"},
	{Name: "nil_callback_invoke", Result: "i64", Params: "i64, ptr, i64"},
	{Name: "nil_callback_release", Result: "void", Params: "i64"},
	{Name: "nil_first", Result: "i64", Params: "i64"},
	{Name: "nil_last", Result: "i64", Params: "i64"},
	{Name: "nil_rest", Result: "i64", Params: "i64"},
	{Name: "nil_to_str", Result: "i64", Params: "i64"},
	{Name: "nil_is_null", Result: "i64", Params: "i64"},
	{Name: "nil_substr", Result: "i64", Params: "i64, i64, i64"},
	{Name: "nil_substr_from", Result: "i64", Params: "i64, i64"},
	{Name: "nil_map_new", Result: "i64", Params: "i64"},
	{Name: "nil_map_get", Result: "i64", Params: "i64, i64"},
	{Name: "nil_map_set", Result: "i64", Params: "i64, i64, i64"},
	{Name: "nil_map_len", Result: "i64", Params: "i64"},
	{Name: "nil_get_field", Result: "i64", Params: "i64, i64"},
	{Name: "nil_call", Result: "i64", Params: "i64, i64"},
	{Name: "nil_bool_new", Result: "i64", Params: "i64"},
	{Name: "nil_logic", Result: "i64", Params: "i64, i64, i64"},
	{Name: "nil_bool_not", Result: "i64", Params: "i64"},
	{Name: "nil_truthy", Result: "i64", Params: "i64"},
}

// runtimeSymbol reports whether a name is one the runtime exports.
//
// A module can only call what is declared, so this is also the answer to "can the
// linker resolve this?" — and being able to ask it without a link is what lets the
// tests check a whole module without a compiler.
func runtimeSymbol(name string) (runtimeFunction, bool) {
	for _, fn := range runtimeFunctions {
		if fn.Name == name {
			return fn, true
		}
	}
	return runtimeFunction{}, false
}

// declareRuntime writes the external declarations.
func (e *Emitter) declareRuntime() string {
	var b strings.Builder
	for _, fn := range runtimeFunctions {
		b.WriteString(fn.Decl())
		b.WriteString("\n")
	}

	// The FFI's conversions, declared only when an `extern` declaration needed
	// them. A module with no externs does not mention them at all, which keeps
	// every other module's text unchanged and makes "did adding FFI change this
	// program's IR?" answerable by diffing.
	seen := map[string]bool{}
	for _, fn := range e.ffiDecls {
		if seen[fn.Name] {
			continue
		}
		seen[fn.Name] = true
		b.WriteString(fn.Decl())
		b.WriteString("\n")
	}
	return b.String()
}

// emitStringConstants writes the deduplicated string constants.
func (e *Emitter) emitStringConstants() string {
	if len(e.stringOrder) == 0 {
		return ""
	}
	var b strings.Builder
	for _, s := range e.stringOrder {
		fmt.Fprintf(&b, "@nil.str.%s = private unnamed_addr constant [%d x i8] c%s\n",
			hashString(s), len(s), llvmByteString(s))
	}
	return b.String()
}

// llvmByteString writes a Go string as an LLVM `c"..."` literal, quotes included.
//
// # Why this is not `%q`
//
// Because `%q` is Go's quoting and LLVM's is a different language with a different
// escape set. The two agree on `\"` and disagree on almost everything else: Go
// writes a NUL as `\x00` and LLVM's byte strings want `\00`, so `"a\0b"` came out
// as `c"a\x00b"` — which LLVM reads as the five bytes `a`, `\`, `x`, `0`, `0`
// followed by whatever the `b` was, and the module is rejected with "constant
// expression type mismatch: got type '[6 x i8]' but expected '[3 x i8]'".
//
// The declared length and the written bytes came from two different encoders, so
// the error names a length rather than a string, and every control character in a
// program is one rejected module away. Writing the bytes here means there is one
// encoder and the count in `[N x i8]` is the count of bytes actually written.
//
// Every byte outside printable ASCII is escaped, not just the ones Go would escape.
// The two are not the same set — Go leaves a UTF-8 continuation byte alone and
// emits raw high bytes, which LLVM accepts but which make a module unreadable the
// moment it is opened in an editor that guesses the encoding.
func llvmByteString(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case c == '"':
			b.WriteString(`\"`)
		case c == '\\':
			b.WriteString(`\\`)
		case c >= 0x20 && c < 0x7F:
			b.WriteByte(c)
		default:
			fmt.Fprintf(&b, `\%02X`, c)
		}
	}
	b.WriteByte('"')
	return b.String()
}

// hashString is the suffix of a string constant's name.
//
// It is the value's own bytes, not a running counter, so the same string gets the
// same name in two different builds — which is what makes two modules comparable.
func hashString(s string) string {
	var h uint64 = 1469598103934665603
	for i := 0; i < len(s); i++ {
		h ^= uint64(s[i])
		h *= 1099511628211
	}
	return fmt.Sprintf("%x", h)
}

// itoa is a small decimal formatter.
//
// It exists because the name allocator composes names by hand, and reaching for
// `strconv` in two places to do it is the kind of duplication that ends with one
// of them using a different base.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}

// stringConstant emits a string literal into the module and returns its global's
// name.
func (e *Emitter) stringConstant(s string) string {
	if name, seen := e.strings[s]; seen {
		return name
	}
	name := "nil.str." + hashString(s)
	e.strings[s] = name
	e.stringOrder = append(e.stringOrder, s)
	return name
}
