package llvm

import (
	"fmt"
	"sort"
	"strings"

	"github.com/joysriramsarkar/nilLang/compiler/ffi"
	"github.com/joysriramsarkar/nilLang/compiler/mir"
)

// The crossing table itself lives in `compiler/ffi`, not here.
//
// What a type becomes at the boundary is a fact about the pair of languages, and
// two things now write it down: the `declare` this package emits and the
// prototype the generated C header carries. While the table lived here the header
// generator had either to import this package or copy it, and a copy is a second
// implementation of "what is a string at the boundary" — free to disagree with the
// first, in a way that compiles and links and is wrong. See ADR-0079.
//
// So the names below are the package's own — `ffi.Crossing`, `ffi.Signature` — with
// no local alias. An alias would read as though this package had a second kind of
// crossing of its own, and a reader would have to work out whether the two were the
// same thing. They are; there is one.

// ffiStructPrefix is the name prefix for the struct a string crossing travels
// through.
//
// It is a prefix rather than a fixed name because two functions in one module can
// both take a string, and two values in one expression can both be converted — so
// `%nilffi.str0`, `%nilffi.str1`, and so on. One fixed name would define it twice,
// which LLVM rejects as a redefinition of a struct.
func ffiStructPrefix() string { return "%nilffi.str" }

// The buffer scope a string-returning call runs inside.
//
// Two calls rather than one because they have to bracket the foreign call, and the
// callee is not the one who may open it: a callee that pushed a mark would leave
// the matching close with nothing to pop. See ADR-0075.
const (
	ffiScopeOpen  = "nil_ffi_scope_open"
	ffiScopeClose = "nil_ffi_scope_close"

	// ffiScopeBuffer asks the open scope for `length` writable bytes.
	//
	// The same allocator a C function uses to build an answer, rather than a second
	// one. A struct parameter's buffer has the same lifetime — this call — and two
	// allocators for one lifetime is one more thing that can be freed at the wrong
	// moment or not at all.
	ffiScopeBuffer = "nil_ffi_buffer"
)

// ffiABIGuard is the symbol whose *name* carries the FFI's version.
//
// The version is in the name rather than in a value so that the linker does the
// checking: `nil_ffi_abi_v1` and `nil_ffi_abi_v2` are different symbols, so an
// object built against v1 fails to resolve against a v2-only runtime, naming the
// symbol. A single symbol both versions define would defeat the whole arrangement,
// and a run-time comparison would report a mismatch only once the program was
// already running.
//
// The number after the `v` is the version, and it must equal `NIL_FFI_ABI_VERSION`
// in `nilffi.h`. Nothing checks that mechanically — it cannot, the two are in
// different languages — so a test compares them. See ADR-0082.
//
// That test is not a formality. Raising `NIL_FFI_ABI_VERSION` for the channel work
// (L5.8) broke exactly here: the header said 2, the emitter still emitted `v1`, and
// `TestTheVersionInTheGuardIsTheVersionInTheHeader` said so by printing both numbers
// side by side. A version guard that is allowed to drift is not a guard.
const ffiABIGuard = "nil_ffi_abi_v2"

// splatOut writes the two LLVM arguments a splatted crossing needs, given a Nilang
// value held in `raw`, and the struct to write them into.
//
// The struct is the scope's, not an `alloca` of the call site's own. An `alloca`
// written where the call is lands inside that call's block, so a call in a loop body
// allocated once per iteration and the stack grew until it did not. See
// `nil_ffi_scope_open`.
func (e *Emitter) splatOut(b *strings.Builder, raw string, crossing ffi.Crossing, slot string) []string {
	fmt.Fprintf(b, "  call void @%s(%s %s, ptr %%%s)\n",
		crossing.Unbox, valueType, raw, slot)

	filled := e.tmp("strval")
	fmt.Fprintf(b, "  %%%s = load %s, ptr %%%s\n", filled, ffiStructPrefix(), slot)

	pointer := e.tmp("strptr")
	fmt.Fprintf(b, "  %%%s = extractvalue %s %%%s, 0\n", pointer, ffiStructPrefix(), filled)

	length := e.tmp("strlen")
	fmt.Fprintf(b, "  %%%s = extractvalue %s %%%s, 1\n", length, ffiStructPrefix(), filled)

	return []string{"ptr %" + pointer, "i64 %" + length}
}

// convertOut emits the conversion of one Nilang value into a C argument.
//
// The result is **typed**, because LLVM's textual parser reads a call's argument
// list as `type value` pairs and has no way to infer the type. An untyped reference
// is rejected with "invalid type for function argument" and the caret points at the
// value, which reads as though the value were malformed.
//
// The conversion is a call to the runtime, which is where the representation lives.
// Inline would be a second implementation, and the two would drift.
func (e *Emitter) convertOut(b *strings.Builder, value string, crossing ffi.Crossing) string {
	if crossing.Unbox == "" {
		return crossing.LLVM + " " + value
	}
	tmp := e.tmp("unbox")
	fmt.Fprintf(b, "  %%%s = call %s @%s(%s %s)\n", tmp, crossing.LLVM, crossing.Unbox, valueType, value)
	return crossing.LLVM + " %" + tmp
}

// convertIn emits the conversion of a C result back into a Nilang value.
func (e *Emitter) convertIn(b *strings.Builder, raw string, crossing ffi.Crossing) string {
	if crossing.Box == "" {
		return raw
	}
	tmp := e.tmp("box")
	fmt.Fprintf(b, "  %%%s = call %s @%s(%s %s)\n", tmp, valueType, crossing.Box, crossing.LLVM, raw)
	return "%" + tmp
}

// functionsUsedAsValues is every function the program names as a value rather
// than calls.
//
// A pre-pass rather than a flag set during emission, because a shim has to be
// emitted after the function it adapts and the emitter walks functions in the
// program's own order — so "have I seen this one yet" is not a question emission
// can answer.
//
// Only functions actually used as values get a shim. A module with no callbacks is
// then byte-identical to one written before callbacks existed, which is what makes
// "did adding callbacks change this program?" answerable by diffing.
func functionsUsedAsValues(program *mir.Program) map[string]bool {
	used := map[string]bool{}

	var scanExpression func(op mir.Operand)
	scanExpression = func(op mir.Operand) {
		if name, isVariable := op.(mir.VarOperand); isVariable {
			used[name.Name] = true
		}
	}

	for _, fn := range functionsToScan(program) {
		if fn == nil || fn.Foreign {
			continue
		}
		for _, block := range fn.Blocks {
			if block == nil {
				continue
			}
			for _, inst := range block.Instructions {
				switch i := inst.(type) {
				case mir.CallInst:
					// A call's callee is a *call*, not a value. Naming it here
					// would give every called function a shim it never needs.
					for _, arg := range i.Args {
						scanExpression(arg)
					}
				case mir.AssignInst:
					scanExpression(i.Src)
				case mir.BinaryOpInst:
					scanExpression(i.Left)
					scanExpression(i.Right)
				case mir.UnaryOpInst:
					scanExpression(i.Right)
				case mir.SetIndexInst:
					scanExpression(i.Container)
					scanExpression(i.Key)
					scanExpression(i.Value)
				case mir.AggregateInst:
					for _, element := range i.Elems {
						scanExpression(element)
					}
				case mir.SelectInst:
					scanExpression(i.Cond)
					scanExpression(i.True)
					scanExpression(i.False)
				case mir.StoreVarInst:
					scanExpression(i.Src)
				case mir.LoadVarInst:
					scanExpression(i.Dest)
				}
			}
			if term, isTerm := block.Terminator.(mir.ReturnTerminator); isTerm {
				scanExpression(term.Value)
			}
		}
	}

	// Only the ones that really are functions.
	for name := range used {
		if fn, found := program.Functions[name]; !found || fn == nil {
			delete(used, name)
		}
	}
	return used
}

// foreignSignatures works out every foreign function's crossing, and the runtime
// symbols and types that follow from it.
//
// # Why this runs before anything is written
//
// Because two of its answers belong near the top of the module: the struct a string
// crossing needs, and whether any conversion is called at all. Deciding them while
// the functions were being emitted was too late for the types — a struct written
// after its first use is a module LLVM rejects, and the error names a use rather
// than the missing declaration.
//
// It also means a bad signature is refused before a partial module exists, so the
// message arrives with no half-written module attached to it.
func (e *Emitter) foreignSignatures(program *mir.Program) (map[string]ffi.Signature, error) {
	signatures := map[string]ffi.Signature{}

	names := make([]string, 0, len(program.Functions))
	for name, fn := range program.Functions {
		if fn != nil && fn.Foreign {
			names = append(names, name)
		}
	}
	// Sorted so that the declarations a module ends up with do not depend on map
	// order — the same reason the rest of the module's ordered output is sorted.
	sort.Strings(names)

	for _, name := range names {
		fn := program.Functions[name]
		signature, err := ffi.SignatureFor(e.program, fn)
		if err != nil {
			return nil, err
		}
		signatures[name] = signature

		functions, structs := ffiRuntimeFunctions(signature)
		e.ffiDecls = append(e.ffiDecls, functions...)
		if structs != "" && e.ffiStructs == "" {
			e.ffiStructs = structs
		}
	}

	// The ABI guard goes in every module that declares an `extern`, and in none
	// that does not.
	//
	// Its whole job is to be an undefined symbol when the runtime is a version
	// this module was not built against, so that a mismatch is a link error on the
	// machine that will not work rather than a program that runs and answers
	// wrongly. A program that never crosses a boundary has no mismatch to detect
	// and should not link a symbol it will never use — which is also why this is
	// here, once the signatures are known, rather than in the static table every
	// module carries.
	if len(signatures) > 0 {
		e.ffiGuard = ffiABIGuard
		e.ffiDecls = append(e.ffiDecls, runtimeFunction{Name: ffiABIGuard, Result: "void", Params: ""})
	}
	return signatures, nil
}

// functionsToScan is every function body the program has.
//
// # Why `Main` is here and not just `Functions`
//
// Because the program's top level is not a member of `Functions` — it is `Main` —
// and it is where most of the interesting calls are. Scanning only `Functions`
// found no function used as a value in any program whose top level was the only one
// mentioning it, which is every program: the shim was never emitted and the
// assembler rejected `ptr @nil.double_it.cb` as an undefined value.
//
// Two lists saying what a program contains is one of those mistakes that reads as
// correct right up until it does not.
func functionsToScan(program *mir.Program) []*mir.Function {
	out := make([]*mir.Function, 0, len(program.Functions)+1)
	if program.Main != nil {
		out = append(out, program.Main)
	}
	for _, fn := range program.Functions {
		out = append(out, fn)
	}
	return out
}

// emitCallbackValue builds the closure a function name means when it is used as a
// value.
//
// # Why a closure and not a bare pointer
//
// Because a function is not all a closure is. A function that captured variables
// needs them kept alive, and a bare `i64` pointer carries nothing about what it
// needs — so the collector would be free to reclaim a capture while C still held
// the callback. The closure holds the captures and is pinned for as long as C has
// it, which is what makes the pointer C stores stay valid.
//
// # Why the pointer is taken rather than called
//
// `ptr @nil.name` is the function's address as a value. Writing `i64 @nil.name`
// would call it, and calling it here would run a Nilang function during the
// takes. A callback that guessed would read past the end of C's argument array.
//
// The pointer taken is the *shim's*, not the function's: a compiled Nilang
// function takes its arguments in registers and a closure is invoked with an array
// and a count, and calling one as the other is how every callback ended up
// answering 0. See `callbackShimName`.
//
// Queued rather than written, because `operand` has no builder and every other
// value it produces queues the instruction that defines it.
func (e *Emitter) emitCallbackValue(name string) (string, error) {
	fn := e.program.Functions[name]
	if fn == nil {
		return "", fmt.Errorf("llvm: %q is neither a variable nor a function", name)
	}

	tmp := e.tmp("callback")
	e.queue(fmt.Sprintf("  %%%s = call %s @%s(ptr @%s, %s %d)\n",
		tmp, valueType, runtimeCallbackNew, callbackShimName(llvmSymbol(name)), valueType, len(fn.Params)))
	return "%" + tmp, nil
}

// emitStructOut writes a struct parameter: a buffer the runtime owns, with each
// field converted and stored at the offset C will read it from.
//
// # Why the buffer is the runtime's
//
// Because an `alloca` at the call site lands in whatever block the call is in, so a
// call inside a loop body allocated once per iteration and the stack grew until it
// did not. The buffer scope already exists for exactly this shape — one per call,
// freed when the call is done, correct under nesting — and a struct's fields may
// include strings that unbox through the same scope.
//
// # Why byte offsets and not a declared LLVM struct type
//
// Because the offsets are the ones *we* computed and the ones C's compiler computed,
// and writing them literally says so. A declared `%nil.struct.Point = type { i64,
// i64 }` would have to repeat the padding rule inside LLVM's type syntax to match,
// and a struct type whose declared layout disagrees with the stores into it is a
// disagreement nobody would see.
//
// `getelementptr i8` with a byte offset needs no type at all and encodes the offset
// we actually used.
func (e *Emitter) emitStructOut(b *strings.Builder, raw string, crossing ffi.Crossing, slot string) string {
	layout := crossing.Struct
	if layout == nil {
		// Unreachable: the caller has already refused a crossing with no layout, and
		// a nil dereference here would be a crash rather than a message.
		return "ptr null"
	}

	buffer := e.tmp("struct")
	fmt.Fprintf(b, "  %%%s = call ptr @%s(i64 %d)\n", buffer, ffiScopeBuffer, layout.Size)

	for _, field := range layout.Fields {
		address := e.tmp("field")
		fmt.Fprintf(b, "  %%%s = getelementptr inbounds i8, ptr %%%s, i64 %d\n",
			address, buffer, field.Offset)

		// The field's *value* is not the struct's value — it is one component of a
		// hash, read by name and in declaration order. That is what makes the order
		// in `mir.Program.Types` load-bearing rather than tidy.
		component := e.fieldValue(b, raw, field)

		if field.Crossing.Splat {
			// Two words: the pointer and the length, in that order and eight bytes
			// apart, which is what `nil_string_unbox` fills and what the C struct
			// declares.
			halves := e.splatOut(b, component, field.Crossing, slot)

			ptrAddress := e.tmp("fieldptr")
			fmt.Fprintf(b, "  %%%s = getelementptr inbounds i8, ptr %%%s, i64 %d\n",
				ptrAddress, buffer, field.Offset)
			fmt.Fprintf(b, "  store %s, ptr %%%s, align 8\n", halves[0], ptrAddress)

			lenAddress := e.tmp("fieldlen")
			fmt.Fprintf(b, "  %%%s = getelementptr inbounds i8, ptr %%%s, i64 %d\n",
				lenAddress, buffer, field.Offset+8)
			fmt.Fprintf(b, "  store %s, ptr %%%s, align 8\n", halves[1], lenAddress)
			continue
		}

		value := e.convertOut(b, component, field.Crossing)
		fmt.Fprintf(b, "  store %s, ptr %%%s, align %d\n", value, address, field.Crossing.Alignment)
	}

	return "ptr %" + buffer
}

// fieldValue reads one field out of a struct value.
//
// The struct's value is a hash at run time, so the field is reached by the runtime's
// own accessor rather than by arithmetic — a hash holds the fields and the
// declaration says which is which and in what order, which is exactly why the
// declaration had to survive lowering separately.
//
// The result is an **LLVM value**, `%name`, not a bare name: what comes back is
// handed straight to `convertOut`, which expects the same form every other argument
// arrives in. Returning a bare name produced `call i64 @nil_int_unbox(i64 field15)`
// and clang answered "invalid type for function argument" — pointing at a conversion
// that looks completely ordinary in every other call in the module.
//
// The *call* spells the type on both arguments, and that is not tidiness. An
// argument's type may be omitted when the callee is defined in the same module,
// because the parser can look it up; for a `declare`d function it cannot, and
// `call i64 @nil_get_field(%t1, i64 %key18)` is rejected with "invalid type for
// function argument" — pointing at the *value*, which is perfectly valid, and
// naming neither the call nor the declaration. Every other conversion in this
// package already spells it; this one did not.
func (e *Emitter) fieldValue(b *strings.Builder, raw string, field ffi.Field) string {
	key := e.tmp("key")
	fmt.Fprintf(b, "  %%%s = call %s @%s(ptr @%s, i64 %d)\n",
		key, valueType, runtimeStringLit, e.stringConstant(field.Name), len(field.Name))

	value := e.tmp("field")
	fmt.Fprintf(b, "  %%%s = call %s @%s(%s %s, %s %%%s)\n",
		value, valueType, runtimeGetField, valueType, raw, valueType, key)
	return "%" + value
}

// takeSplat reads the two halves out of the scope's crossing struct.
//
// Written out rather than reused, because `splatOut` returns the halves as
// *arguments* — `ptr %x, i64 %y` — and a store wants the bare name. Turning one into
// the other by trimming a prefix is a string operation on a name, and a name is the
// one thing in a module where a silent mistake is hardest to see.
func (e *Emitter) takeSplat(b *strings.Builder, slot, pointer, length string) {
	filled := e.tmp("sv")
	fmt.Fprintf(b, "  %%%s = load %s, ptr %%%s\n", filled, ffiStructPrefix(), slot)
	fmt.Fprintf(b, "  %%%s = extractvalue %s %%%s, 0\n", pointer, ffiStructPrefix(), filled)
	fmt.Fprintf(b, "  %%%s = extractvalue %s %%%s, 1\n", length, ffiStructPrefix(), filled)
}

// splatIn writes the call that asks `callee` to fill a string, and returns the
// Nilang value built from what it wrote.
//
// The mirror of `splatOut`, and for the same reason: the caller allocates the
// struct and the callee fills it, so neither side is relying on how a struct
// return would have been classified.
//
// The copy afterwards is what lets C hand its buffer over at all — see
// `nil_string_copy`.
//
// The scope around the call is what lets a C author stop thinking about freeing
// anything. `nil_ffi_scope_open` marks where the buffers start and
// `nil_ffi_scope_close` frees everything allocated since, so a callee that built
// its answer with `nil_ffi_buffer` never frees it and a callee that handed over a
// `static` string is left alone. Without the close the program would leak one
// buffer per call, and the leak would be invisible until it was fatal — see ADR-0075.
//
// The close is written *after* the copy rather than immediately after the call,
// because the copy is what reads the bytes. Reclaiming them first would be a
// use-after-free with a one-instruction delay.
//
// `callee` is passed rather than read from the signature because the signature's
// `box` is the *conversion*, not the function being called. Using it here produced
// `call void @nil_string_copy(ptr, i64, i64, ptr)` — a call to a two-argument
// function with four — which linked, because the symbol existed, and answered
// nothing at all. The declaration and the call have to be written from the same
// answer; a place where they can disagree is a place where they will.
func (e *Emitter) splatIn(b *strings.Builder, converted []string, callee, slot string) string {
	// The caller's slot is the *last* parameter, after every declared one. It is
	// appended rather than prepended because a C function may take arguments after
	// the ones the caller converted and putting the slot first would silently pass
	// a pointer where a value belongs.
	args := append(append([]string{}, converted...), "ptr %"+slot)
	fmt.Fprintf(b, "  call void @%s(%s)\n", callee, strings.Join(args, ", "))

	filled := e.tmp("retval")
	fmt.Fprintf(b, "  %%%s = load %s, ptr %%%s\n", filled, ffiStructPrefix(), slot)

	pointer := e.tmp("retptr")
	fmt.Fprintf(b, "  %%%s = extractvalue %s %%%s, 0\n", pointer, ffiStructPrefix(), filled)

	length := e.tmp("retlen")
	fmt.Fprintf(b, "  %%%s = extractvalue %s %%%s, 1\n", length, ffiStructPrefix(), filled)

	tmp := e.tmp("strbox")
	fmt.Fprintf(b, "  %%%s = call %s @%s(ptr %%%s, i64 %%%s)\n",
		tmp, valueType, runtimeStringCopy, pointer, length)

	fmt.Fprintf(b, "  call void @%s()\n", ffiScopeClose)
	return "%" + tmp
}

// foreignDeclarationWith writes the `declare` for a function whose crossing is known.
//
// The signature is the C one — `declare double @sin(double)` — and not the Nilang
// one. Declaring `i64` and converting at the call site would also work, and would
// put the C ABI's burden on every back end rather than in one place. The
// declaration is exactly where a reader looks to see what a foreign function
// really is, so it should be true.
//
// A function that answers a string declares `void` and takes the struct as its
// last parameter — the caller's, not the callee's. That is the whole point: see
// ADR-0072.
func foreignDeclarationWith(fn *mir.Function, signature ffi.Signature) string {
	params := make([]string, 0, len(signature.Params))
	for _, crossing := range signature.Params {
		if crossing.Splat {
			params = append(params, crossing.LLVM, "i64")
			continue
		}
		params = append(params, crossing.LLVM)
	}

	if signature.Ret.Splat {
		// Appended rather than joined, because a function taking nothing else has
		// no parameters to join onto — `Join("", ", ")` leaves the leading comma
		// and LLVM answers "expected type" at the character after the open paren,
		// which reads as though the `ptr` were misspelled.
		params = append(params, "ptr")
		return fmt.Sprintf("declare void @%s(%s)\n", fn.Name, strings.Join(params, ", "))
	}

	ret := signature.Ret.LLVM
	if ret == "void" {
		return fmt.Sprintf("declare void @%s(%s)\n", fn.Name, strings.Join(params, ", "))
	}
	return fmt.Sprintf("declare %s @%s(%s)\n", ret, fn.Name, strings.Join(params, ", "))
}

// ffiRuntimeFunctions is every runtime symbol the FFI conversions need.
//
// Declared once so that a module using `extern` declares what it calls. Missing one
// produces a module that assembles on a machine where the symbol happens to exist
// and fails to link on one where it does not — the worst place for it to fail,
// because those two machines are the developer's and the user's.
//
// The struct a string crossing needs is declared alongside them, because a type
// cannot be declared with a function and the module has to have one to allocate.
func ffiRuntimeFunctions(signature ffi.Signature) (functions []runtimeFunction, structs string) {
	seen := map[string]bool{}

	add := func(symbol, result string, params string) {
		if symbol == "" || seen[symbol] {
			return
		}
		seen[symbol] = true
		functions = append(functions, runtimeFunction{Name: symbol, Result: result, Params: params})
	}

	// The struct is declared as soon as *anything* splats, in either direction:
	// `nil_string_unbox` writes into one and the scope's own is one, so every
	// function that touches a string needs the type — and a module that uses a
	// type nobody declared is rejected over a line that reads perfectly
	// ordinarily.
	needsStruct := func() {
		if structs == "" {
			structs = ffiStructPrefix() + " = type { ptr, i64 }\n"
		}
	}

	// The struct and the scope are needed as soon as *anything* splats, in either
	// direction.
	//
	// Either, not just a string *return*. A function that merely *takes* a string
	// still needs both: the struct to unbox into, and the scope that owns it. The
	// first version declared the scope only for a splatted return, and a program
	// whose extern took a string and answered with an integer emitted
	// `call void @nil_ffi_scope_close()` against nothing — rejected by the
	// assembler with "use of undefined value", pointing at a call that reads
	// perfectly ordinarily.
	needsScope := func() {
		needsStruct()
		// `nil_ffi_scope_open` *returns* a pointer rather than being `void`: the
		// struct to cross through belongs to the scope, because an `alloca` at the
		// call site would land in the loop body and grow the stack per iteration.
		// And `nil_ffi_buffer` is how a struct parameter gets its own storage inside
		// that scope — the same allocator a C function builds an answer with, so
		// there is one lifetime rather than two.
		add(ffiScopeOpen, "ptr", "")
		add(ffiScopeClose, "void", "")
		add(ffiScopeBuffer, "ptr", valueType)
	}

	for _, crossing := range signature.Params {
		if crossing.Splat || crossing.Struct != nil {
			// Both for the same reason: both need the scope. A splatted argument
			// unboxes through the scope's struct; a struct argument takes its buffer
			// from the scope. A struct whose fields are all integers would otherwise
			// declare no scope at all and then call one — rejected as an undefined
			// value, pointing at a call that reads perfectly ordinarily.
			needsScope()
		}
		if crossing.Splat {
			add(crossing.Unbox, "void", valueType+", ptr")
			continue
		}
		if crossing.Struct != nil {
			// Each *field* is converted, so each field's unboxing is called. A
			// `Point` of two integers calls `nil_int_unbox` twice, and declaring it
			// once because it is "the Int unbox" is exactly as many declarations as
			// it has calls — which is the rule.
			//
			// Declared from the layout rather than from the struct's declaration
			// because the layout is what says which fields *cross*. A field with no
			// crossing has already been refused; there is nothing left here to
			// discover.
			for _, field := range crossing.Struct.Fields {
				if field.Crossing.Splat {
					add(field.Crossing.Unbox, "void", valueType+", ptr")
					continue
				}
				add(field.Crossing.Unbox, field.Crossing.LLVM, valueType)
			}
			continue
		}
		add(crossing.Unbox, crossing.LLVM, valueType)
	}
	if signature.Ret.Splat {
		needsScope()
		// The copy is what the call site builds the returned string with, so it is
		// declared here rather than being assumed to be there for every module.
		add(runtimeStringCopy, valueType, "ptr, i64")
		return functions, structs
	}
	if signature.Ret.Box != "" {
		add(signature.Ret.Box, valueType, signature.Ret.LLVM)
	}
	return functions, structs
}
