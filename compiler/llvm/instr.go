package llvm

import (
	"fmt"
	"strings"

	"github.com/joysriramsarkar/nilLang/compiler/mir"
)

// Runtime functions this file calls, named here so a call site reads as a call.
//
// The names are the C symbols from `runtime/core/include/nil.h`, spelled exactly.
//
// # Why the underscore
//
// A program function's symbol has a dot in it — `nil.add_two`, from `llvmSymbol` —
// and a runtime function's has an underscore: `nil_int_add`. The dot is legal in an
// LLVM identifier but not in a C one, so a runtime symbol with a dot would need an
// `__asm__` label in the C library, and `__asm__` does not exist in MSVC's C
// compiler. Matching the C name instead costs a convention and buys a runtime that
// compiles with whatever C compiler the machine happens to have.
//
// The convention is worth stating because it is the only thing distinguishing the
// two namespaces when reading generated IR: an underscore is the runtime, a dot is
// the program.
//
// The signatures live in `runtimeFunctions` in emit.go, and
// `TestEveryRuntimeSymbolIsDeclaredAndMatchesItsHeader` is what keeps the two in
// step.
const (
	runtimeAlloc        = "nil_alloc"
	runtimePrint        = "nil_print"
	runtimePrintln      = "nil_println"
	runtimeStringNew    = "nil_string_new"
	runtimeStringLit    = "nil_string_lit"
	runtimeStringLen    = "nil_string_len"
	runtimeStringConcat = "nil_string_concat"

	// runtimeStringCopy is the only way bytes become a Nilang string.
	//
	// Named here rather than written as a literal in the emitter because it is
	// declared in the ABI table and called at two different places, and a literal
	// in one of them is a symbol the table does not know about — which the ABI test
	// cannot catch, because the test checks the table against the header, not the
	// emitter against the table.
	runtimeStringCopy    = "nil_string_copy"
	runtimeIntNew        = "nil_int_new"
	runtimeFloatNew      = "nil_float_new"
	runtimeIntAdd        = "nil_int_add"
	runtimeIntSub        = "nil_int_sub"
	runtimeIntMul        = "nil_int_mul"
	runtimeIntDiv        = "nil_int_div"
	runtimeIntMod        = "nil_int_mod"
	runtimeFloatAdd      = "nil_float_add"
	runtimeFloatSub      = "nil_float_sub"
	runtimeFloatMul      = "nil_float_mul"
	runtimeFloatDiv      = "nil_float_div"
	runtimeIntCompare    = "nil_int_cmp"
	runtimeStringCompare = "nil_str_cmp"
	runtimeArrayNew      = "nil_array_new"
	runtimeArrayGet      = "nil_array_get"
	runtimeArraySet      = "nil_array_set"
	runtimeIndexGet      = "nil_index_get"
	runtimeIndexSet      = "nil_index_set"
	runtimeMapNew        = "nil_map_new"
	runtimeMapGet        = "nil_map_get"
	runtimeMapSet        = "nil_map_set"
	runtimeGetField      = "nil_get_field"
	runtimeSetField      = "nil_set_field"
	runtimeCallbackNew   = "nil_callback_new"
	runtimeIntUnbox      = "nil_int_unbox"
	runtimeCall          = "nil_call"
	runtimeTruthy        = "nil_truthy"
	runtimeBoolNew       = "nil_bool_new"
	runtimeLogic         = "nil_logic"
	runtimeBoolNot       = "nil_bool_not"
)

// intOperations are the arithmetic operators, and the runtime function each one
// lowers to.
//
// # Why arithmetic is a call and not an LLVM instruction
//
// LLVM has `add`, `sub`, `mul` and `sdiv`, and using them would be faster than
// calling the runtime. It would also be wrong.
//
// A Nilang `Int` is a tagged value: it may be a boxed number, a small integer held
// inline, or a pointer, and the same `i64` may hold a `Bool`, a `String`, a list or
// a map. `add i64` on two of those produces a number, which is not what the
// language's `+` means — the language's `+` on two strings concatenates, and on a
// string and an integer is an error rather than an address.
//
// A direct `add` would have to know which of those it was looking at, which means
// it would have to understand the runtime's tagging. Two implementations of "what
// does `+` mean" is exactly what the interpreter, the VM and the compiler should
// not have. The call keeps the definition in one place.
var intOperations = map[string]string{
	"+": runtimeIntAdd,
	"-": runtimeIntSub,
	"*": runtimeIntMul,
	"/": runtimeIntDiv,
	"%": runtimeIntMod,
}

var floatOperations = map[string]string{
	"+": runtimeFloatAdd,
	"-": runtimeFloatSub,
	"*": runtimeFloatMul,
	"/": runtimeFloatDiv,
}

// comparisons are the ordering operators and the comparison code each becomes.
//
// The code is passed to the runtime rather than being an LLVM `icmp` because the
// operands are tagged values: `icmp` on two pointers compares addresses, and a
// program comparing two strings compares their contents.
var comparisons = map[string]int{
	"==": 0,
	"!=": 1,
	"<":  2,
	"<=": 3,
	">":  4,
	">=": 5,
}

// emitInstruction writes one MIR instruction.
func (e *Emitter) emitInstruction(b *strings.Builder, inst mir.Instruction) error {
	e.flushPending(b)

	if err := e.emitOne(b, inst); err != nil {
		return err
	}
	e.flushPending(b)
	return nil
}

func (e *Emitter) emitOne(b *strings.Builder, inst mir.Instruction) error {
	switch i := inst.(type) {
	case mir.AssignInst:
		return e.emitAssign(b, i)

	case mir.StoreVarInst:
		return e.emitStoreVar(b, i)

	case mir.LoadVarInst:
		return e.emitLoadVar(b, i)

	case mir.BinaryOpInst:
		return e.emitBinaryOp(b, i)

	case mir.UnaryOpInst:
		return e.emitUnaryOp(b, i)

	case mir.CallInst:
		return e.emitCall(b, i)

	case mir.AggregateInst:
		return e.emitAggregate(b, i)

	case mir.SetIndexInst:
		return e.emitSetIndex(b, i)

	case mir.SelectInst:
		return e.emitSelect(b, i)

	case mir.JumpTerminator:
		return nil
	}

	return fmt.Errorf("llvm: no instruction for %T", inst)
}

func (e *Emitter) emitAssign(b *strings.Builder, i mir.AssignInst) error {
	value, err := e.operand(i.Src)
	if err != nil {
		return err
	}
	// Flushed before the destination is written, not only at the end of the
	// instruction. A constant source queues the instruction that produces it, and
	// the destination refers to it by name — without this the destination read a
	// value that had not been defined yet, and the `%%` in the IR said so.
	e.flushPending(b)
	// `value` is already a reference with its `%` — `operand` returns one. Adding
	// another produced `%%kint0`, which names a *global* `kint0` rather than the
	// local, and the module then failed to assemble over a symbol nobody defined.
	return e.assignTo(b, i.Dest, value)
}

func (e *Emitter) emitStoreVar(b *strings.Builder, i mir.StoreVarInst) error {
	slot, ok := e.current.slots[i.Name]
	if !ok {
		return fmt.Errorf("llvm: storing to %q, which has no slot", i.Name)
	}
	value, err := e.operand(i.Src)
	if err != nil {
		return err
	}
	// The value's own instructions are flushed *here*, not by the caller.
	//
	// The caller flushed before this instruction was looked at, which is too
	// early: the constant or load that produces this instruction's operand was
	// only queued while rendering it. Writing `store kint0` before
	// `%kint0 = call ...` is a forward reference, and LLVM reports it at the
	// `store` — a line that looks perfectly ordinary and says nothing about the
	// value it names.
	e.flushPending(b)
	// The value carries an explicit type.
	//
	// A literal already has one — `store i64 5, ptr %slot` — so the omission is
	// invisible until the value is an SSA name, where clang 22 rejects
	// `store %v, ptr %slot` with "expected value token" pointing at a line that
	// looks completely ordinary. A store of a literal works and a store of a
	// computed value does not, which is the kind of difference that reads as a
	// compiler bug until you try it both ways.
	fmt.Fprintf(b, "  store i64 %s, ptr %%%s, align 8\n", value, slot)
	return nil
}

func (e *Emitter) emitLoadVar(b *strings.Builder, i mir.LoadVarInst) error {
	slot, ok := e.current.slots[i.Name]
	if !ok {
		return fmt.Errorf("llvm: loading %q, which has no slot", i.Name)
	}
	tmp := e.tmp("load")
	fmt.Fprintf(b, "  %%%s = load i64, ptr %%%s, align 8\n", tmp, slot)
	return e.assignTo(b, i.Dest, "%"+tmp)
}

// emitBinaryOp writes an arithmetic or comparison instruction.
//
// The operator decides the runtime function rather than the operand types, because
// the *runtime* knows the types: the same `i64` holds an integer, a string and a
// pointer, and only something that understands the tagging can say which `+` means.
func (e *Emitter) emitBinaryOp(b *strings.Builder, i mir.BinaryOpInst) error {
	left, err := e.operand(i.Left)
	if err != nil {
		return err
	}
	// Flushed between the operands, not only after both. A constant operand queues
	// the instruction that produces it, and reading the right operand can flush
	// that queue — putting the left operand's definition after the left operand's
	// use.
	e.flushPending(b)

	right, err := e.operand(i.Right)
	if err != nil {
		return err
	}
	// Both operands' instructions are flushed before the operation that consumes
	// them. A constant or a load queued while rendering either operand has to be
	// written first, or the operation refers to a value that does not exist yet.
	e.flushPending(b)

	if fn, isArithmetic := intOperations[i.Op]; isArithmetic {
		tmp := e.tmp("bin")
		fmt.Fprintf(b, "  %%%s = call i64 @%s(i64 %s, i64 %s)\n", tmp, fn, left, right)
		return e.assignTo(b, i.Dest, "%"+tmp)
	}

	if _, isFloat := floatOperations[i.Op]; isFloat {
		fn := floatOperations[i.Op]
		tmp := e.tmp("fbin")
		fmt.Fprintf(b, "  %%%s = call i64 @%s(i64 %s, i64 %s)\n", tmp, fn, left, right)
		return e.assignTo(b, i.Dest, "%"+tmp)
	}

	if code, isComparison := comparisons[i.Op]; isComparison {
		// Comparison against a literal is a string comparison when the literal is a
		// string, and an integer comparison otherwise. Both are runtime calls,
		// because an `icmp` on two tagged values compares addresses.
		fn := runtimeIntCompare
		if isStringLiteral(i.Right) || isStringLiteral(i.Left) {
			fn = runtimeStringCompare
		}
		tmp := e.tmp("cmp")
		fmt.Fprintf(b, "  %%%s = call i64 @%s(i64 %s, i64 %s, i64 %d)\n",
			tmp, fn, left, right, code)
		return e.assignTo(b, i.Dest, "%"+tmp)
	}

	if i.Op == "[]" || i.Op == "." {
		// Subscript and field access reach the runtime as one call each. They are
		// spelled as binary operations in MIR because they take two operands, but
		// a Nilang value is tagged and an LLVM `getelementptr` on two `i64`s would
		// do arithmetic on addresses rather than fetch anything.
		//
		// Both go to the same runtime function, which decides what it is holding.
		// The compiler cannot: at MIR level `m["k"]` and `xs[1]` are the same
		// instruction, so emitting a call to `nil_array_get` made every map lookup
		// fail with "cannot index a value that is not an array". See ADR-0043.
		//
		// `.` is a string literal rather than a variable, so it is the same call
		// with the member's name pushed.
		fn := runtimeIndexGet
		if i.Op == "." {
			fn = runtimeGetField
		}
		tmp := e.tmp("idx")
		fmt.Fprintf(b, "  %%%s = call i64 @%s(i64 %s, i64 %s)\n", tmp, fn, left, right)
		return e.assignTo(b, i.Dest, "%"+tmp)
	}

	code := -1
	switch i.Op {
	case "&&":
		code = 0
	case "||":
		code = 1
	}
	if code >= 0 {
		// Short-circuit evaluation needs a basic block, which the instruction's
		// single result slot cannot express. The runtime evaluates both sides and
		// the *result* is the same; the difference shows only for a right-hand side
		// with an effect, which `&&` in this language does not have because every
		// builtin that can fail stops the program rather than returning a value.
		//
		// Truthiness answers 1 or 0, and those are *not* a Nilang boolean: 1 is
		// `NIL_TAG_FALSE` and 2 is `NIL_TAG_TRUE`. So the runtime applies the
		// operator and builds the Boolean in one step.
		//
		// The emitter cannot apply the operator itself: by the time it has the
		// operands it has their *values*, and `nil_truthy` has already discarded
		// which side was which. `a || b` emitted as "is the left side true" answers
		// the wrong question whenever the left side is false and the right is true.
		tmp := e.tmp("logic")
		fmt.Fprintf(b, "  %%%s = call i64 @%s(i64 %s, i64 %s, i64 %d)\n",
			tmp, runtimeLogic, left, right, code)
		return e.assignTo(b, i.Dest, "%"+tmp)
	}

	return fmt.Errorf("llvm: no lowering for binary operator %q", i.Op)
}

func (e *Emitter) emitUnaryOp(b *strings.Builder, i mir.UnaryOpInst) error {
	right, err := e.operand(i.Right)
	if err != nil {
		return err
	}
	// Same reason as `emitAssign`: a constant operand queues the instruction that
	// produces it, and the operation below refers to that name.
	e.flushPending(b)

	switch i.Op {
	case "-":
		// Negation is `0 - x`, and it calls the same runtime function as any other
		// subtraction so that a value's type is checked once — in one place.
		//
		// The zero is the *tagged* nil, which is what `nil_int_new(0)` would build:
		// a bare `i64 0` has tag `NIL_TAG_NIL` too, but that is an accident of the
		// encoding rather than a decision, and writing it as `0` would be correct
		// today and inscrutable tomorrow.
		zero := e.tmp("zero")
		e.flushPending(b)
		fmt.Fprintf(b, "  %%%s = call i64 @%s(i64 0)\n", zero, runtimeIntNew)
		tmp := e.tmp("neg")
		fmt.Fprintf(b, "  %%%s = call i64 @%s(i64 %%%s, i64 %s)\n", tmp, runtimeIntSub, zero, right)
		return e.assignTo(b, i.Dest, "%"+tmp)

	case "!":
		// Truthiness answers 1 or 0, and those are *not* a Nilang boolean — 1 is the
		// tag of false. So the answer becomes a Boolean, and the only way to do that
		// correctly is to let the runtime build it.
		tmp := e.tmp("not")
		fmt.Fprintf(b, "  %%%s = call i64 @%s(i64 %s)\n", tmp, runtimeBoolNot, right)
		return e.assignTo(b, i.Dest, "%"+tmp)
	}

	return fmt.Errorf("llvm: no lowering for unary operator %q", i.Op)
}

// emitSetIndex writes `container[key] = value` as one runtime call.
//
// One call for every container because the representation decides what it is
// holding, not the compiler — the same reasoning as the read side (ADR-0043). A
// member write is not a separate call because the parser folded it into a
// subscript before MIR saw it; the read side *is* separate, because there MIR does
// still carry `.`.
func (e *Emitter) emitSetIndex(b *strings.Builder, i mir.SetIndexInst) error {
	fn := runtimeIndexSet

	container, err := e.operand(i.Container)
	if err != nil {
		return err
	}
	e.flushPending(b)

	key, err := e.operand(i.Key)
	if err != nil {
		return err
	}
	e.flushPending(b)

	value, err := e.operand(i.Value)
	if err != nil {
		return err
	}
	e.flushPending(b)

	tmp := e.tmp("set")
	fmt.Fprintf(b, "  %%%s = call %s @%s(%s %s, %s %s, %s %s)\n",
		tmp, valueType, fn, valueType, container, valueType, key, valueType, value)
	return e.assignTo(b, i.Dest, "%"+tmp)
}

func (e *Emitter) emitCall(b *strings.Builder, i mir.CallInst) error {
	args := make([]string, 0, len(i.Args))
	for _, arg := range i.Args {
		value, err := e.operand(arg)
		if err != nil {
			return err
		}
		// Flushed per argument, not once at the end. A constant argument queues
		// the instruction that produces it, and the *next* argument's operand may
		// flush it — which would write the definition after the call that uses it.
		e.flushPending(b)
		// Typed, always. LLVM's textual parser reads a call's argument list as
		// `type value` pairs and has no way to infer the type: `call i64 @f(%x)`
		// is rejected with "invalid type for function argument" and points the
		// caret at `%x`, which reads as though `%x` were malformed when it is
		// perfectly ordinary.
		//
		// Every argument in this module is an `i64` because every value is one
		// tagged word, so the type is a constant rather than something to look
		// up. It is written here rather than in `operand` because an operand is
		// used in many positions — an operand of a binary operator carries its
		// type from the operator, and a call argument carries its own.
		args = append(args, valueType+" "+value)
	}
	e.flushPending(b)

	// A named function is called directly; only a value that came from a variable
	// goes through the runtime's indirect call.
	//
	// Calling every function through `nil_call` is what this used to do, and it is
	// wrong twice over: the callee was never turned into a callable value, so
	// `nil_call` answered nil for every call in the language, and the arguments
	// were passed as a Nilang array rather than as parameters, so a function
	// reading a parameter read the wrong thing.
	//
	// The direct call is also the fast one, and it is what makes a compiled call
	// cheaper than an interpreted one — which is the entire reason for compiling.
	if i.Callee != "" && isRuntimeSymbol(i.Callee) {
		symbol, err := builtinSymbol(i.Callee, len(i.Args))
		if err != nil {
			return err
		}
		tmp := e.tmp("call")
		fmt.Fprintf(b, "  %%%s = call %s @%s(%s)\n",
			tmp, valueType, symbol, strings.Join(args, ", "))
		return e.assignTo(b, i.Dest, "%"+tmp)
	}

	// A callee is one of four things, and only the last is a gap. See `call.go`
	// for why each is dispatched separately.
	if i.Callee != "" {
		switch {
		case e.isForeign(i.Callee):
			return e.emitForeignCall(b, i, args)
		case e.moduleDefines(i.Callee):
			tmp := e.tmp("call")
			fmt.Fprintf(b, "  %%%s = call %s @%s(%s)\n",
				tmp, valueType, llvmSymbol(i.Callee), strings.Join(args, ", "))
			return e.assignTo(b, i.Dest, "%"+tmp)
		default:
			// Anything left is either a variable holding a function value — which
			// is a call the runtime makes — or a name nothing answers for.
			if _, isVariable := e.current.slots[i.Callee]; isVariable {
				return e.emitIndirectCall(b, i, args)
			}
			return fmt.Errorf("llvm: no lowering for a call to %s: "+
				"it is not a function this module defines, not a foreign one, not a "+
				"variable holding a function, and not a builtin with a runtime function",
				i.Callee)
		}
	}

	tmp := e.tmp("call")
	fmt.Fprintf(b, "  %%%s = call %s @%s(%s)\n", tmp, valueType, runtimeCall, strings.Join(args, ", "))
	return e.assignTo(b, i.Dest, "%"+tmp)
}

// runtimeBuiltins are the program-visible names that resolve to a runtime call.
//
// A program writes `xs.first`; the runtime exports `nil_first`. The mapping is one
// prefix, applied here rather than in the MIR lowerer — the lowerer cannot know
// which of a program's identifiers are functions and which are builtins, and a
// program may shadow either.
//
// The list is explicit rather than derived, because "does `nil_` + this name exist
// in the runtime" is the wrong question: a program may define its own `first`, and
// a name that happens to collide with a runtime symbol is the *program's*.
// A builtin is listed only once there is a runtime function that is its exact
// meaning — not something close. `push` returns a *new* list while `nil_array_set`
// changes one in place, so mapping `push` onto the array primitive would be a
// program that compiles, runs, and aliases; `len` dispatches on the kind of thing
// it is given, which `nil_array_len` does not. Both are left out until the
// function that answers them properly exists, and a call to them is refused with a
// message naming the construct — which is a far better outcome than a wrong answer.
//
// The list is explicit rather than derived by prefixing `nil_`, because "does
// `nil_` + this name exist in the runtime" is the wrong question: a program may
// define its own `first`, `str` or `last`, and a name that happens to collide with
// a runtime symbol is the *program's*. A program that defines `fn str(x)` must get
// its own function, and a lookup that guessed would silently give it the
// runtime's instead — in a language where `str` is a perfectly good name.
var runtimeBuiltins = map[string]string{
	"first":  "nil_first",
	"last":   "nil_last",
	"rest":   "nil_rest",
	"str":    "nil_to_str",
	"isNull": "nil_is_null",
	"substr": "nil_substr",
}

// isRuntimeSymbol reports whether a callee name is one of the runtime's own.
func isRuntimeSymbol(name string) bool {
	if _, isRuntime := runtimeSymbol(name); isRuntime {
		return true
	}
	_, isBuiltin := runtimeBuiltins[name]
	return isBuiltin
}

// builtinSymbol is the C symbol a builtin call reaches, given how many arguments
// the call passed.
//
// The arity is part of the question because `substr` has two forms — `substr(s, at)`
// and `substr(s, at, length)` — and they are different runtime functions rather
// than one with a sentinel. See `nil_substr_from` for why.
//
// Every other builtin takes a fixed number of arguments, and a call with the wrong
// count is refused here rather than emitted: the assembler would catch it as a type
// error pointing at an argument, which says nothing about which builtin and how
// many arguments it wanted.
func builtinSymbol(name string, arity int) (string, error) {
	symbol, isBuiltin := runtimeBuiltins[name]
	if !isBuiltin {
		return "", fmt.Errorf("llvm: %s is not a builtin with a runtime function", name)
	}

	switch name {
	case "substr":
		switch arity {
		case 2:
			return "nil_substr_from", nil
		case 3:
			return "nil_substr", nil
		default:
			return "", fmt.Errorf("llvm: no lowering for substr with %d arguments: it takes 2 or 3", arity)
		}
	}

	want := builtinArity[name]
	if want != 0 && arity != want {
		return "", fmt.Errorf("llvm: no lowering for %s with %d arguments: it takes %d", name, arity, want)
	}
	return symbol, nil
}

// builtinArity is each builtin's fixed argument count. A builtin absent from the
// table has no fixed count — `substr` is the only one today, and it is handled
// above because its two forms are different functions.
var builtinArity = map[string]int{
	"first":  1,
	"last":   1,
	"rest":   1,
	"str":    1,
	"isNull": 1,
}

// moduleDefines reports whether this module defines a function under a name.
//
// A call is emitted directly only if something answers for it. Checking costs one
// map lookup and turns "the assembler will tell me" into a message that says which
// construct was not lowered.
func (e *Emitter) moduleDefines(name string) bool {
	if e.program == nil {
		return false
	}
	if _, found := e.program.Functions[name]; found {
		return true
	}
	return false
}

// isForeign reports whether a name is an `extern` declaration rather than a
// function this program defines.
//
// It matters for exactly one thing: the symbol a call resolves to. Both are
// otherwise identical at a call site — same arity, same types — so a call has to
// ask which kind of function it is calling in order to name it.
func (e *Emitter) isForeign(name string) bool {
	if e.program == nil {
		return false
	}
	fn, found := e.program.Functions[name]
	return found && fn != nil && fn.Foreign
}

// emitAggregate builds a list or a map literal.
//
// # Why the elements are stored rather than evaluated and dropped
//
// The first version called `e.operand(elem)` for each element — to type-check them
// — and then threw the values away, building an array of `nil`s. Nothing failed:
// the module was valid, the program ran, and `let a = [10, 20, 30]; a[2]` printed
// `null` instead of `30`. Every list and every map literal was wrong, and a test
// that checked only the *count* of elements could not have seen it.
//
// So each element is stored into the container as it is built. Writing one at a
// time is the honest way to do it: the runtime owns the layout, and a compiler that
// emitted a packed struct would have to know a layout the runtime defines.
func (e *Emitter) emitAggregate(b *strings.Builder, i mir.AggregateInst) error {
	elements := i.Elems
	newFunction, setFunction := runtimeArrayNew, runtimeIndexSet
	pairs := i.Hash
	if pairs {
		newFunction, setFunction = runtimeMapNew, runtimeMapSet
	}

	// A map literal is a flat list of alternating keys and values, so its capacity
	// is half its length.
	count := len(elements)
	if pairs {
		count /= 2
	}

	tmp := e.tmp("agg")
	fmt.Fprintf(b, "  %%%s = call i64 @%s(i64 %d)\n", tmp, newFunction, count)

	for index := 0; index < len(elements); index++ {
		key, err := e.operand(elements[index])
		if err != nil {
			return err
		}
		e.flushPending(b)

		// A list stores one value per position. A map stores a key and its value
		// together, which is why the value is read here rather than on the next
		// turn: the runtime's call takes both.
		if pairs {
			if index%2 != 0 {
				continue
			}
			if index+1 >= len(elements) {
				return fmt.Errorf(
					"llvm: a map literal has a key with no value at position %d", index)
			}
			value, err := e.operand(elements[index+1])
			if err != nil {
				return err
			}
			e.flushPending(b)
			fmt.Fprintf(b, "  %%%s = call i64 @%s(i64 %%%s, i64 %s, i64 %s)\n",
				e.tmp("set"), setFunction, tmp, key, value)
			continue
		}

		// The index is a *Nilang value*, not a number.
		//
		// Passing the bare integer is the single most expensive mistake available
		// in this emitter: `nil_int_new(2)` returns a heap box, because 2's low
		// three bits are not the tag, while the literal `2` has tag `NIL_TAG_TRUE`.
		// The runtime's `nil_int_value` therefore read every raw index as zero, all
		// three writes landed on element 0, and `a[2]` read an element nothing had
		// ever written — so `[10, 20, 30][2]` answered `null` and the program
		// exited successfully.
		//
		// Nothing about the IR was wrong, which is what made it hard to see.
		indexOperand, err := e.constant(int64(index))
		if err != nil {
			return err
		}
		e.flushPending(b)

		fmt.Fprintf(b, "  %%%s = call i64 @%s(i64 %%%s, i64 %s, i64 %s)\n",
			e.tmp("set"), setFunction, tmp, indexOperand, key)
	}

	return e.assignTo(b, i.Dest, "%"+tmp)
}

// emitSelect writes a two-way choice as a real branch.
//
// Both arms are emitted, because a select's arms are arbitrary expressions and only
// one of them may run. The branch is on the condition's truthiness rather than on
// an `i1`, because a Nilang condition is a tagged value and the question "is this
// true" is one only the runtime can answer.
func (e *Emitter) emitSelect(b *strings.Builder, i mir.SelectInst) error {
	cond, err := e.operand(i.Cond)
	if err != nil {
		return err
	}

	// The runtime answers an `i64` and the phi wants an `i1`, so the comparison
	// happens here. See the same two lines in `emitTerminator`.
	truthy := e.tmp("seltruthy")
	fmt.Fprintf(b, "  %%%s = call i64 @%s(i64 %s)\n", truthy, runtimeTruthy, cond)
	condBool := e.tmp("selcond")
	fmt.Fprintf(b, "  %%%s = icmp ne i64 %%%s, 0\n", condBool, truthy)

	thenLabel := e.tmp("then")
	elseLabel := e.tmp("else")
	mergeLabel := e.tmp("merge")

	// Each arm is evaluated *before* the branch, into a temp of its own.
	//
	// A phi's incoming value must already exist at the top of its block, and LLVM
	// requires phis to be at the top. Evaluating the arm after the branch would put
	// a constant between the label and the phi. Evaluating both arms up front
	// costs a call that is then discarded — which `mem2reg` and `instcombine`
	// remove, and which is cheaper than an IR LLVM will not accept.
	trueIncoming, err := e.armValue(i.True)
	if err != nil {
		return err
	}
	falseIncoming, err := e.armValue(i.False)
	if err != nil {
		return err
	}
	e.flushPending(b)

	fmt.Fprintf(b, "  br i1 %%%s, label %%%s, label %%%s\n",
		condBool, thenLabel, elseLabel)

	fmt.Fprintf(b, "%s:\n", thenLabel)
	truePhi := e.tmp("phi_then")
	fmt.Fprintf(b, "  %%%s = phi i64 [ %s, %%%s ]\n", truePhi, trueIncoming, e.blockLabel(e.entryBlockID))
	fmt.Fprintf(b, "  br label %%%s\n", mergeLabel)

	fmt.Fprintf(b, "%s:\n", elseLabel)
	falsePhi := e.tmp("phi_else")
	fmt.Fprintf(b, "  %%%s = phi i64 [ %s, %%%s ]\n", falsePhi, falseIncoming, e.blockLabel(e.entryBlockID))
	fmt.Fprintf(b, "  br label %%%s\n", mergeLabel)

	fmt.Fprintf(b, "%s:\n", mergeLabel)
	// The two arms are joined by a phi of their own, in the merge block, which is
	// the only place a value from two blocks can legally meet.
	//
	// It is written at the top of the block, which is where LLVM requires every phi
	// to be, and it *is* the destination — so it is named for the rest of the
	// function here rather than through `assignTo`, which would define a second
	// local with the same value and trip "multiple definition of local value".
	if dest, isTemp := i.Dest.(mir.TempOperand); isTemp {
		e.current.takenNames["t"+itoa(dest.ID)] = true
		fmt.Fprintf(b, "  %%t%d = phi i64 [ %%%s, %%%s ], [ %%%s, %%%s ]\n",
			dest.ID, truePhi, thenLabel, falsePhi, elseLabel)
		return nil
	}

	joined := e.tmp("phi_merge")
	fmt.Fprintf(b, "  %%%s = phi i64 [ %%%s, %%%s ], [ %%%s, %%%s ]\n",
		joined, truePhi, thenLabel, falsePhi, elseLabel)
	return e.assignTo(b, i.Dest, "%"+joined)
}

// assignTo binds a value to a destination temp.
// armValue evaluates one arm of a select and returns the value its phi names.
//
// The evaluation is queued rather than written immediately, so both arms are
// emitted before the branch. See `emitSelect` for why a phi's incoming value
// cannot be produced after the label that introduces it.
func (e *Emitter) armValue(op mir.Operand) (string, error) {
	return e.operand(op)
}

// assignTo binds a value to a destination temp.
//
// # It has to come before anything else in a block
//
// A phi is the only instruction that may not be preceded by another one — LLVM
// requires every phi in a block to be grouped at its top, because their incoming
// edges say nothing about the order instructions run in. So when a destination is
// a phi, the move into it is written *immediately*, before the block's next
// instruction is reached.
func (e *Emitter) assignTo(b *strings.Builder, dest mir.Operand, value string) error {
	if dest == nil {
		return nil
	}
	temp, ok := dest.(mir.TempOperand)
	if !ok {
		return fmt.Errorf("llvm: cannot assign to %s, which is not a temporary", dest.String())
	}
	// The value arrives as an operand *reference*, which already carries its `%`.
	//
	// The move is `add ... 0` rather than a `copy` instruction. LLVM has no `copy`
	// — a name moves into a register by being produced by something, and the
	// cheapest thing that produces an existing value is an addition of zero. A
	// `copy` opcode reads as a plausible instruction to anyone who has not looked
	// it up, and clang rejects it with "expected instruction opcode" pointing at
	// the word itself.
	//
	// A temporary written in more than one block is written under a block-local
	// name, and the merge block joins the copies with a phi. Writing the same
	// `%t1` in two blocks defines it twice, which LLVM rejects over a line that
	// looks ordinary.
	key := "t" + itoa(temp.ID)
	if renamed, ok := e.current.renames[key]; ok {
		fmt.Fprintf(b, "  %s = add i64 %s, 0\n", renamed, value)
		return nil
	}

	e.current.takenNames[key] = true
	fmt.Fprintf(b, "  %%t%d = add i64 %s, 0\n", temp.ID, value)
	return nil
}

// operand renders a MIR operand as an LLVM value *reference*.
//
// The reference includes the leading `%`. Returning it without one would mean
// every use site wrote `%%%s` for a temp and `%%s` for a constant, and a
// mismatch between the two produces module text that is wrong in a way no amount
// of reading the emitter explains: `store kint0` stores the *global* named
// `kint0`, which does not exist.
func (e *Emitter) operand(op mir.Operand) (string, error) {
	switch v := op.(type) {
	case mir.TempOperand:
		key := fmt.Sprintf("t%d", v.ID)
		if e.current != nil {
			if renamed, ok := e.current.renames[key]; ok {
				return renamed, nil
			}
		}
		return fmt.Sprintf("%%t%d", v.ID), nil

	case mir.ConstOperand:
		return e.constant(v.Value)

	case mir.VarOperand:
		// A name that is a function is a *value* naming that function, not a slot.
		// Reaching here means a program used a function as a value — passing it to
		// C, or binding it to a name — and the value it means is a closure.
		if e.moduleDefines(v.Name) {
			return e.emitCallbackValue(v.Name)
		}

		slot, ok := e.current.slots[v.Name]
		if !ok {
			return "", fmt.Errorf("llvm: reading %q, which has neither a slot nor a function of that name",
				v.Name)
		}
		tmp := e.tmp("vload")
		e.queue(fmt.Sprintf("  %%%s = load i64, ptr %%%s, align 8\n", tmp, slot))
		return "%" + tmp, nil
	}

	return "", fmt.Errorf("llvm: no value for %s", op.String())
}

// constant renders a constant operand.
//
// An integer is emitted as a call into the runtime rather than as an `i64` literal,
// because a Nilang integer is a tagged value and `42` is not the representation of
// one. Building it through the runtime keeps a single definition of what the
// representation is — the same reason arithmetic is a call.
func (e *Emitter) constant(value any) (string, error) {
	switch v := value.(type) {
	case nil:
		/* `0`, not `null`.
		 *
		 * `null` is a pointer in LLVM and an `i64` value here, so `ret i64 null` is
		 * "null must be a pointer type" — an error that reads as though the module
		 * were about pointers, when it is about one constant.
		 *
		 * Zero is `NIL_TAG_NIL`, the tag of a null Nilang value, so the number and
		 * the meaning agree. Written as a number rather than a named constant because
		 * LLVM has no way to name this module's tags without a global per tag, which
		 * would be a global per constant.
		 */
		return "0", nil

	case bool:
		/* The *tag*, not LLVM's `true`/`false`.
		 *
		 * A Nilang boolean is a tagged value like every other — `NIL_TAG_FALSE` is 1
		 * and `NIL_TAG_TRUE` is 2 — and LLVM's own `true` is `i1 1`. Writing `ret
		 * i64 true` assembled only because LLVM read the bare `true` as a one-bit
		 * constant promoted to 64 bits, so the program returned the integer 1, which
		 * is the tag of *false*. A program whose answer was `true` printed `false`.
		 *
		 * The tags are written as numbers rather than named so the connection to
		 * `nil.h` is visible at the point where someone is tempted to write `true`.
		 */
		if v {
			return "2", nil
		}
		return "1", nil

	case int64:
		tmp := e.tmp("kint")
		e.queue(fmt.Sprintf("  %%%s = call i64 @%s(i64 %d)\n", tmp, runtimeIntNew, v))
		return "%" + tmp, nil

	case float64:
		tmp := e.tmp("kflt")
		e.queue(fmt.Sprintf("  %%%s = call i64 @%s(double %s)\n", tmp, runtimeFloatNew, floatLiteral(v)))
		return "%" + tmp, nil

	case string:
		tmp := e.tmp("kstr")
		e.queue(fmt.Sprintf("  %%%s = call i64 @%s(ptr @%s, i64 %d)\n",
			tmp, runtimeStringLit, e.stringConstant(v), len(v)))
		return "%" + tmp, nil
	}

	return "", fmt.Errorf("llvm: no constant for a value of type %T", value)
}

// isStringLiteral reports whether an operand is a string constant.
//
// It exists for one decision — whether a comparison goes to the string or the
// integer runtime — and it looks at the literal rather than at a type the compiler
// inferred, because the inferred type is `Any` for most expressions and a
// comparison against a string would take the integer path.
func isStringLiteral(op mir.Operand) bool {
	c, ok := op.(mir.ConstOperand)
	if !ok {
		return false
	}
	_, isString := c.Value.(string)
	return isString
}
