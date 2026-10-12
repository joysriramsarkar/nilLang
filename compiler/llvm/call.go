package llvm

import (
	"fmt"
	"strings"

	"github.com/joysriramsarkar/nilLang/compiler/ffi"
	"github.com/joysriramsarkar/nilLang/compiler/mir"
)

// resolveCall works out what a call's callee actually is, and calls it.
//
// # The three cases, and why they are three
//
// A callee name is one of: a function this module defines, a function the linker
// resolves, a variable holding a function value, or none of those.
//
// The last used to be emitted as a direct call anyway, producing a module that
// referred to `@nil.Some` and an assembler error pointing at an argument. Refusing
// names the construct instead — but refusing is only correct for the last case, and
// the second and third used to be refused *too*:
//
//   - a foreign call used to be refused because a foreign name is not a module
//     function, so `extern "C" fn sin(...)` could be declared and never called;
//   - a call through a function value used to be refused because the name is a
//     variable, so `let apply = twice; apply(21);` built a closure nothing could
//     call.
//
// Both were "correct" refusals of the wrong thing. A refusal has to name what is
// missing, and naming the wrong thing sends the reader after a construct that is
// present and working.

// callbackShimName is the adapter emitted for a function used as a value.
//
// # Why an adapter at all
//
// A Nilang function is compiled as `i64 @nil.f(i64, i64)` — its arguments in
// registers, in order. A closure is invoked as `i64 (*)(NilValue *args, int64_t
// count)` — an array and a count. The two signatures do not match, and storing one
// pointer and calling it as the other is what made every callback answer 0: the
// callee read an argument count where C had passed a pointer.
//
// So each function used as a value gets a shim of the closure shape that pulls its
// arguments out of the array and forwards them. One shim per function rather than
// one shared, because C cannot forward a variable number of arguments and the only
// portable way is to have the right number of moves compiled in.
//
// # Why the shim converts nothing
//
// Because it does not need to. The arguments in the array are already Nilang values
// — the C side built them with `nil_int_box`, or received them from the program —
// and the result is a Nilang value already. A shim that also converted would need
// each parameter's type, which MIR does not carry for an ordinary function, and a
// shim that guessed would convert 3 into a *tagged* 3, which is a different number.
//
// Conversion belongs where a value crosses, and here the C side is the one crossing.
//
// # Why `%a0` and `%v0` rather than the emitter's counter
//
// Because they are locals of a different function. The counter is per-function
// state, and these names cannot collide with anything in the shim's own body — there
// is nothing else in it.
func callbackShimName(symbol string) string { return symbol + ".cb" }

// emitCallbackShim writes the adapter for one function.
//
// Only emitted for functions actually used as a value, so a module with no
// callbacks is byte-identical to one written before callbacks existed. That is what
// makes "did adding callbacks change this program?" answerable by diffing.
func (e *Emitter) emitCallbackShim(b *strings.Builder, fn *mir.Function, symbol string) {
	arity := len(fn.Params)

	fmt.Fprintf(b, "define %s @%s(ptr %%args, i64 %%count) {\nentry:\n",
		valueType, callbackShimName(symbol))

	forwarded := make([]string, 0, arity)
	for index := 0; index < arity; index++ {
		fmt.Fprintf(b, "  %%a%d = getelementptr i64, ptr %%args, i64 %d\n", index, index)
		fmt.Fprintf(b, "  %%v%d = load i64, ptr %%a%d, align 8\n", index, index)
		forwarded = append(forwarded, fmt.Sprintf("%s %%v%d", valueType, index))
	}

	fmt.Fprintf(b, "  %%r = call %s @%s(%s)\n",
		valueType, symbol, strings.Join(forwarded, ", "))
	b.WriteString("  ret i64 %r\n}\n\n")
}

// emitForeignCall calls a function the linker resolves.
//
// The arguments are converted on the way out and the result on the way in, because
// a Nilang value is a tagged word and C's is not.
//
// The symbol is the bare C name. The `nil.` prefix is for symbols this module
// *defines*, where it makes the IR traceable back to the Nilang name that produced
// them; a C symbol has to be the name the linker will look for, or the call
// resolves to nothing and the module is rejected with "use of undefined value".
// A foreign call that touches a string at all runs inside a scope.
//
// The scope owns the crossing struct *and* whatever buffers the callee builds its
// answer in, and the close frees both. Opened before the first conversion rather
// than by the callee, because a callee does not know it is being called from a
// crossing and one that tried to push a mark would leave the close with nothing to
// pop.
func (e *Emitter) emitForeignCall(b *strings.Builder, i mir.CallInst, args []string) error {
	signature, err := ffi.SignatureFor(e.program, e.program.Functions[i.Callee])
	if err != nil {
		return err
	}

	// One scope for the whole call: the arguments are unboxed into it and read out
	// into SSA values before the call, and the callee then fills the same struct
	// for its answer. Two struct slots would be simpler to reason about and would
	// need two allocas, which is the thing that cannot be done safely.
	touchesString := signature.Ret.Splat
	slot := ""
	if touchesString {
		slot = e.openScope(b)
	}

	converted := make([]string, 0, len(signature.Params))
	for index, crossing := range signature.Params {
		if index >= len(args) {
			break
		}
		raw := strings.TrimPrefix(args[index], valueType+" ")

		if crossing.Struct != nil {
			// A struct's buffer comes from the same scope its string fields unbox
			// through — one scope per call, one place that frees, and the nesting
			// already worked out for strings.
			touchesString = true
			if slot == "" {
				slot = e.openScope(b)
			}
			converted = append(converted, e.emitStructOut(b, raw, crossing, slot))
			continue
		}
		if crossing.Splat {
			touchesString = true
			if slot == "" {
				slot = e.openScope(b)
			}
			// One call, two arguments. The runtime writes both halves into a struct
			// it owns for the duration of this call, so nothing here depends on
			// which compiler compiled the runtime — see ADR-0072.
			converted = append(converted, e.splatOut(b, raw, crossing, slot)...)
			continue
		}
		converted = append(converted, e.convertOut(b, raw, crossing))
	}

	// A `void` call is written without a name, because LLVM rejects a named
	// instruction that returns nothing: "instructions returning void cannot have a
	// name". The destination temp is then never written, which is correct only
	// because a function declared `-> Void` has no value to read — the checker
	// rejects a program that uses one, and the MIR's destination is a temporary
	// nothing reads.
	//
	// Writing `nil_int_new(0)` into it instead would be worse than leaving it: it
	// would make a program's read of a value that does not exist answer a number
	// rather than being the undefined value it is.
	if signature.Ret.LLVM == "void" {
		fmt.Fprintf(b, "  call void @%s(%s)\n", i.Callee, strings.Join(converted, ", "))
		// Closed here as well as in `splatIn`, because a `-> Void` function that
		// takes a string has converted something into the scope and nothing else
		// would ever close it. An unbalanced scope does not fail loudly — the next
		// call's close frees this one's buffers and the leak is a number that
		// climbs for the life of the process.
		if touchesString && !signature.Ret.Splat {
			fmt.Fprintf(b, "  call void @%s()\n", ffiScopeClose)
		}
		return nil
	}

	// A string return is filled into the struct the scope owns and then copied into
	// a Nilang string. The copy is what lets C hand its buffer over at all — see
	// `nil_string_copy`.
	//
	// The declared return of such a function is `void`, but `signature.Ret.LLVM` is
	// still `"ptr"` — that field is the *element* type, and the declaration is
	// written from the splat flag. So this is reached by asking about the crossing,
	// not by looking at what the declaration says.
	if signature.Ret.Splat {
		return e.assignTo(b, i.Dest, e.splatIn(b, converted, i.Callee, slot))
	}

	tmp := e.tmp("fficall")
	fmt.Fprintf(b, "  %%%s = call %s @%s(%s)\n",
		tmp, signature.Ret.LLVM, i.Callee, strings.Join(converted, ", "))
	if touchesString {
		fmt.Fprintf(b, "  call void @%s()\n", ffiScopeClose)
	}
	return e.assignTo(b, i.Dest, e.convertIn(b, "%"+tmp, signature.Ret))
}

// openScope opens a buffer scope and returns the name of the handle holding the
// struct to cross through.
func (e *Emitter) openScope(b *strings.Builder) string {
	handle := e.tmp("scope")
	fmt.Fprintf(b, "  %%%s = call ptr @%s()\n", handle, ffiScopeOpen)
	return handle
}

// emitIndirectCall calls a function value held in a variable.
//
// The arguments go into a Nilang array and `nil_call` is given that, rather than
// being passed one by one.
//
// That is a real cost — an allocation and a store per argument on every indirect
// call — and it buys one calling convention rather than two. A C-side variadic call
// would be a second convention for the runtime and the compiler to agree on, and
// the two would drift the first time either changed. When the indirect call turns
// out to matter, `nil_call` is the one place to make faster, because it is the only
// place the array is built.
func (e *Emitter) emitIndirectCall(b *strings.Builder, i mir.CallInst, args []string) error {
	slot, known := e.current.slots[i.Callee]
	if !known {
		return fmt.Errorf("llvm: calling %q, which is neither a function nor a variable", i.Callee)
	}

	// Declared here rather than in the module's fixed table, because a program that
	// never calls a function value does not use them. A missing declaration is
	// worse than a superfluous one in every case: it fails to link, and it fails to
	// link on the user's machine rather than the developer's.
	e.ffiDecls = append(e.ffiDecls,
		runtimeFunction{Name: runtimeIntUnbox, Result: valueType, Params: valueType})

	callee := e.tmp("callee")
	e.queue(fmt.Sprintf("  %%%s = load %s, ptr %%%s, align 8\n", callee, valueType, slot))
	e.flushPending(b)

	// The counts are *unboxed* on the way to the runtime.
	//
	// `nil_int_new` answers a tagged value — 1 is stored as 11 — and `nil_array_new`
	// treats its argument as a number of elements. Handing it the tagged value made
	// a one-element array eleven elements long, and a larger count into an
	// allocation large enough to fail: "out of memory allocating an array", from a
	// program that asked for one element. A tagged value crossing where a plain
	// number is wanted is the whole failure this FFI exists to prevent, and it was
	// here, in the FFI's own code.
	count := e.tmp("argc")
	e.queue(fmt.Sprintf("  %%%s = call %s @%s(%s %d)\n", count, valueType, runtimeIntNew, valueType, len(args)))
	e.flushPending(b)

	countValue := e.tmp("argcn")
	e.queue(fmt.Sprintf("  %%%s = call %s @%s(%s %%%s)\n",
		countValue, valueType, runtimeIntUnbox, valueType, count))
	e.flushPending(b)

	array := e.tmp("argv")
	e.queue(fmt.Sprintf("  %%%s = call %s @%s(%s %%%s)\n", array, valueType, runtimeArrayNew, valueType, countValue))
	e.flushPending(b)

	for index, arg := range args {
		key := e.tmp("argk")
		e.queue(fmt.Sprintf("  %%%s = call %s @%s(%s %d)\n", key, valueType, runtimeIntNew, valueType, index))
		e.flushPending(b)

		keyValue := e.tmp("argkv")
		e.queue(fmt.Sprintf("  %%%s = call %s @%s(%s %%%s)\n",
			keyValue, valueType, runtimeIntUnbox, valueType, key))
		e.flushPending(b)

		// `arg` already carries its type: every argument was rendered as a typed
		// reference when the operand was resolved, because LLVM's parser reads an
		// argument list as `type value` pairs. Adding `valueType` again produced
		// `i64 i64 %x`, which parses as a type followed by another type.
		element := e.tmp("arge")
		e.queue(fmt.Sprintf("  %%%s = call %s @%s(%s %%%s, %s %%%s, %s)\n",
			element, valueType, runtimeArraySet, valueType, array, valueType, keyValue, arg))
		e.flushPending(b)
	}

	tmp := e.tmp("icall")
	fmt.Fprintf(b, "  %%%s = call %s @%s(%s %%%s, %s %%%s)\n",
		tmp, valueType, runtimeCall, valueType, callee, valueType, array)
	return e.assignTo(b, i.Dest, "%"+tmp)
}
