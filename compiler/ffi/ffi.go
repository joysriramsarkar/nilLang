// Package ffi is what a Nilang type is *to* C.
//
// # Why this is a package and not part of a back end
//
// Because the crossing is a fact about the pair of languages, not about any one
// back end or any one output. `Int` crosses as `int64_t` because C has `int64_t`
// and a Nilang integer is a number; nothing about `Int` on its own implies that,
// and LLVM is not what makes it true.
//
// Keeping the table here means two things can be written from one answer — the
// `declare` the LLVM back end emits and the prototype the generated C header
// carries — without either being able to drift from the other. When the table
// lived inside the LLVM package, the header generator had either to import that
// package or to copy the table, and a copy is a second implementation of "what is
// a string at the boundary". See ADR-0079.
package ffi

import (
	"fmt"
	"strings"

	"github.com/joysriramsarkar/nilLang/compiler/mir"
)

// Crossing is how one Nilang type crosses the boundary to C.
//
// Two spellings because there are two audiences: the LLVM back end writes a
// declaration and a call, and a C header writes a prototype, and a prototype a C
// compiler rejects is a header nobody can use. They come from one entry so the
// two cannot disagree — a generated header that said `int64_t` where the module
// declared `i64` would compile, link, and be wrong.
type Crossing struct {
	// LLVM is the type as written in the declaration and passed in the call.
	LLVM string

	// C is the type as written in a C prototype.
	//
	// `const` is part of it rather than a modifier bolted on afterwards, because
	// whether a pointer is const is part of what the function promises: `const
	// char *` says the callee will not write through it, and a header that lost
	// that would be asking the reader to take it on trust.
	C string

	// Unbox turns a Nilang value into that type on the way out, or "" when the
	// type crosses unchanged.
	Unbox string

	// Box turns a value of that type back into a Nilang value on the way in, or
	// "" when it crosses back unchanged.
	Box string

	// Splat is true when the type crosses as *two* C arguments.
	//
	// Only a string, and only because a C string has no length: a length is the
	// only thing that distinguishes the empty string from the end of one, so
	// passing a bare pointer would make the two the same value and every C
	// function written to take a length would have to guess.
	//
	// The two halves travel through a caller-allocated struct rather than a
	// returned one, because a returned struct's meaning depends on which compiler
	// produced the other half: clang returns a 16-byte struct in RAX:RDX while GCC
	// for `x86_64-w64-mingw32` returns it through a pointer in RCX. A pointer
	// argument is just another argument, so nothing has to know which convention
	// won. See ADR-0072.
	Splat bool

	// Params is how many C arguments one Nilang argument becomes.
	//
	// A field rather than `1` and a special case, because the two places that
	// write the argument list — the emitter's call and the header's prototype —
	// must agree, and a field they both read cannot disagree.
	Params int

	// Size and Alignment are how many bytes the value occupies in a struct, and how
	// strictly it must line up.
	//
	// # They are read only for struct fields
	//
	// For a parameter or a return they mean nothing: a parameter is in a register
	// or on the stack and has no offset. They are here because a struct crossing is
	// laid out from them, and a layout algorithm needs each field's width.
	//
	// # They live here rather than in a second table
	//
	// Because the emitter and the layout would each otherwise keep one, and two
	// tables of "how wide is an Int" is one too many. The first time a type gained a
	// width, one of the two would have been missed and a struct would have been laid
	// out wrongly for a reason nobody could see from the code that did it.
	//
	// The sizes are the C ones, because the struct they describe is a C struct —
	// see `LayoutFor`.
	Size      int64
	Alignment int64

	// Struct is the field layout when this crossing is a struct, and nil otherwise.
	//
	// A pointer rather than a value for two reasons that have nothing to do with each
	// other: nil is how "this crossing is not a struct" is expressed, which `Layout`
	// cannot say on its own, and a pointer keeps `Crossing` cheap to copy — it is
	// copied into a slice, a map and a parameter list on every foreign call.
	//
	// It is *not* one layout per type. Each `ForIn` computes a fresh one, so two
	// parameters of the same struct carry two equal layouts. Nothing writes to a layout
	// after it is built, which is what makes that safe, and a struct with two unequal
	// copies is a thing the tests check for rather than a thing the type prevents.
	//
	// A struct crosses as a `ptr`. As a *field* that would be a bare pointer, which
	// is not the thing anyone wants; as an *argument* it is exactly right, because
	// the C function receives a `const Point *` and reads a buffer the caller filled.
	// The difference between those two is why the shape lives here rather than
	// being folded into `LLVM`, and why a struct is refused as a return.
	Struct *Layout
}

// Types is the crossing table, keyed by the lowercased type name.
//
// Lowercased because the lookup is: a program is free to write `Int` or `int`, and
// two spellings of one type must not cross differently — the crossing depends on
// the type, not on how somebody capitalised it.
//
// Every entry names two runtime functions, so the module declares them and the
// conversion has exactly one implementation. An inline conversion written in a back
// end would be a second implementation of "what is an integer" and the two would
// drift — which they would, quietly, the first time the representation changed.
//
// # What is not here, and why that is the right answer
//
// Every compound type is absent, and so is every record and every list. A list
// crosses as a struct the runtime would have to describe, a block crosses as a
// closure that is already handled, and neither has a definition here.
//
// So those are **refused by name** rather than converted to something plausible. A
// plausible conversion is the worst outcome: it produces a program that compiles,
// links, runs, and reads a length out of a pointer.
// The sizes are the C ones, and they are the whole of what makes a crossing
// usable as a struct field. A string is two words — a pointer and a length — which
// is what a C `struct { const char *bytes; size_t length; }` is; everything else is
// one. There is no field type smaller than a byte, because there is no crossing
// narrower than one.
var Types = map[string]Crossing{
	"int":   {LLVM: "i64", C: "int64_t", Unbox: "nil_int_unbox", Box: "nil_int_box", Params: 1, Size: 8, Alignment: 8},
	"float": {LLVM: "double", C: "double", Unbox: "nil_float_unbox", Box: "nil_float_box", Params: 1, Size: 8, Alignment: 8},
	"bool":  {LLVM: "i32", C: "int", Unbox: "nil_bool_unbox", Box: "nil_bool_box", Params: 1, Size: 4, Alignment: 4},
	"byte":  {LLVM: "i8", C: "uint8_t", Unbox: "nil_byte_unbox", Box: "nil_byte_box", Params: 1, Size: 1, Alignment: 1},
	"void":  {LLVM: "void", C: "void", Params: 0, Size: 0, Alignment: 1},

	// A string, which is the only crossing that takes two arguments.
	//
	// The declaration says `ptr, i64` and the call sends a pointer and a length.
	// The runtime's unbox writes both into a struct the caller allocated, so the
	// module never has to know a register convention — see `Splat`.
	//
	// Sixteen bytes in a struct, which is a C `{const char *, size_t}` and not an
	// accident of this implementation.
	"string": {
		LLVM: "ptr", C: "const char *", Unbox: "nil_string_unbox", Box: "nil_string_copy",
		Splat: true, Params: 2, Size: 16, Alignment: 8,
	},
}

// Callback is the crossing of a Nilang function.
//
// It crosses as one `i64` — the closure handle — and the C spelling is a typedef
// rather than a function pointer, because a bare function pointer carries no
// context and a callback has to know which function it is. See `nilffi.h` for the
// two-value form a C side stores.
var Callback = Crossing{LLVM: "i64", C: "NilFFICallback", Params: 1, Size: 8, Alignment: 8}

// VoidReturn is the crossing of a function that returns nothing.
//
// It is a value rather than an empty field because "returns void" and "no
// annotation was written" are the same thing to C, and a back end should not have
// to tell them apart to write the declaration.
var VoidReturn = Crossing{LLVM: "void", C: "void", Params: 0, Size: 0, Alignment: 1}

// Supported is what a refusal message says is available.
//
// Named explicitly rather than derived from the table's keys, because the keys are
// lowercased for lookup and the message should read like prose. A reader who gets
// this diagnostic should learn what they *can* write, not what a map contains.
const Supported = "Int, Float, Bool, Byte, String, and any function type as a callback"

// For returns the crossing for a written type annotation.
//
// The annotation is a *string* because that is all the front end hands over, so
// this is where `std.Int` and `Int` stop being two spellings.
func For(annotation string) (Crossing, bool) {
	name := annotation
	if dot := strings.LastIndex(name, "."); dot >= 0 {
		name = name[dot+1:]
	}
	name = strings.ToLower(name)

	if isFunctionAnnotation(name) {
		return Callback, true
	}
	crossing, found := Types[name]
	return crossing, found
}

// isFunctionAnnotation reports whether an annotation writes a function type.
//
// The shape rather than a list, because the list would be every signature in the
// language and would need adding to every time one is added. An annotation is a
// function type when it opens with `fn` and a bracket somewhere after it — a
// parameter list in front and, usually, a return type after.
//
// It is a shape check and therefore a heuristic, and a heuristic that guesses
// produces a module that calls C with the wrong thing in a register. So it is
// deliberately strict: `fn` on its own, or an annotation with a bracket before it,
// is not a function type and falls through to the refusal.
//
// The strictness is load-bearing and it was written the hard way. The obvious
// version — "starts with `fn` and contains a bracket somewhere" — accepts `fn`,
// `fnord`, and `FnPtr`, and each of those would be handed a callback's crossing
// when it is not a function type at all. Getting that wrong does not produce a
// refusal; it produces a module that calls C with a closure where a number belongs.
func isFunctionAnnotation(name string) bool {
	if !strings.HasPrefix(name, "fn") {
		return false
	}
	open := strings.Index(name, "(")
	if open < 0 {
		return false
	}
	closed := strings.Index(name, ")")
	if closed < open {
		return false
	}
	rest := strings.TrimSpace(name[closed+1:])
	return rest == "" || strings.HasPrefix(rest, "->")
}

// ForIn returns the crossing for a written type annotation, including a struct the
// program declared.
//
// `For` first, then the program's own type table, because a struct is not a
// built-in: `Point` means whatever *this* program declared, and there is no way to
// know that without the declarations.
//
// The order matters and is not a preference. A program that declared a type called
// `Int` must not be able to shadow the built-in — the shadowing would be invisible,
// and one `Int` would then cross as eight bytes and another as a pointer.
//
// Returns the reason rather than a bare "no". "no crossing for Point" sends the
// reader to the struct; "Point's field items is a List<Int>" sends them to the line.
func ForIn(program *mir.Program, annotation string) (Crossing, error) {
	if crossing, found := For(annotation); found {
		return crossing, nil
	}

	name := annotation
	if dot := strings.LastIndex(name, "."); dot >= 0 {
		name = name[dot+1:]
	}
	declaration, declared := program.Types[name]
	if !declared {
		return Crossing{}, fmt.Errorf(
			"no crossing for %s: it is neither a built-in type nor a struct this "+
				"program declared (defined crossings: %s)", DescribeMissing(annotation), Supported)
	}
	if declaration.Kind != mir.KindStruct {
		return Crossing{}, fmt.Errorf(
			"%s is a %s, and a %s does not cross; an enum crosses as its payload's "+
				"type", name, declaration.Kind, declaration.Kind)
	}

	layout, err := LayoutOf(program, name)
	if err != nil {
		return Crossing{}, err
	}

	return Crossing{
		LLVM:      "ptr",
		C:         "const " + layout.Name + " *",
		Params:    1,
		Size:      layout.Size,
		Alignment: layout.Alignment,
		Struct:    &layout,
	}, nil
}

// Signature is one foreign function's whole crossing.
type Signature struct {
	Params []Crossing
	Ret    Crossing
}

// SignatureFor builds the crossing for a foreign function, or refuses.
//
// # Why the refusal happens here
//
// At the declaration, with the name in the message, rather than at the call site
// with a wrong number in a register. An annotation the programmer wrote that the
// FFI does not support is a gap in the FFI, and it should read like one — "there
// is no crossing for Array yet" is a fact; "expected i64" at a call two hundred
// lines later is not.
//
// It also checks the function's *name*, which is not a type question at all and is
// here for the same reason: it is a fact about crossing the boundary, and both the
// emitter and the generated header have to know it. See `ValidateName`.
func SignatureFor(program *mir.Program, fn *mir.Function) (Signature, error) {
	if err := ValidateName(fn.Name); err != nil {
		return Signature{}, err
	}

	var signature Signature

	for index, annotation := range fn.ParamTypes {
		crossing, err := ForIn(program, annotation)
		if err != nil {
			return Signature{}, fmt.Errorf("%v: extern %q takes it at parameter %d",
				err, fn.Name, index+1)
		}
		signature.Params = append(signature.Params, crossing)
	}

	signature.Ret = VoidReturn
	if fn.ReturnType != "" && fn.ReturnType != "Void" {
		crossing, err := ForIn(program, fn.ReturnType)
		if err != nil {
			return Signature{}, fmt.Errorf("%v: extern %q returns it", err, fn.Name)
		}
		// Refused rather than given a pointer.
		//
		// A struct *parameter* is a buffer the caller fills, and the runtime can fill
		// one. A struct *return* would need the runtime to build a Nilang struct out
		// of C's bytes, and nothing does that yet. Answering with a pointer would hand
		// the program a tagged word pointing at memory it does not own — the failure
		// this whole package exists to prevent.
		//
		// Refused by name, because "structs do not come back yet" is a fact about the
		// FFI and not about the programmer's declaration.
		if crossing.Struct != nil {
			return Signature{}, fmt.Errorf(
				"extern %q returns %s, and a struct cannot cross back yet.\n"+
					"A struct goes *into* C as a pointer to a buffer the caller fills, "+
					"and that is one direction only. Nothing builds a Nilang struct from "+
					"C-owned bytes, so answering with a pointer would hand the program a "+
					"tagged word pointing at memory it does not own.",
				fn.Name, crossing.Struct.Name)
		}
		signature.Ret = crossing
	}

	return signature, nil
}

// cKeywords are the words a C compiler will not accept as a function's name.
//
// The list is C89's, not C23's, deliberately: a header that is valid under the
// newest standard and invalid under the oldest is a header that fails on the
// machine somebody else is building on. Adding a keyword here costs one line and
// refuses a name somebody could have spelled differently; omitting one costs a
// build that fails with a message about a declaration that looks correct.
var cKeywords = map[string]bool{
	"auto": true, "break": true, "case": true, "char": true, "const": true,
	"continue": true, "default": true, "do": true, "double": true, "else": true,
	"enum": true, "extern": true, "float": true, "for": true, "goto": true,
	"if": true, "inline": true, "int": true, "long": true, "register": true,
	"restrict": true, "return": true, "short": true, "signed": true,
	"sizeof": true, "static": true, "struct": true, "switch": true,
	"typedef": true, "union": true, "unsigned": true, "void": true,
	"volatile": true, "while": true,
	// C99 onward, which every compiler that can build a Nilang program accepts.
	"bool": true, "complex": true, "imaginary": true,
	"_Alignas": true, "_Alignof": true, "_Atomic": true, "_Bool": true,
	"_Generic": true, "_Noreturn": true, "_Static_assert": true, "_Thread_local": true,
}

// ValidateName refuses a foreign function's name C could not use.
//
// # Why this is refused rather than mangled
//
// Because the name is not ours. A foreign function is a symbol that already exists
// in some C library, and the program declared it *under the name that library uses*
// — that is the whole contract. Renaming it would produce a module calling a symbol
// that does not exist, which links to an undefined reference on some platforms and
// silently binds to something else on others.
//
// # What it costs to get wrong
//
// A generated header that declares `void register(NilFFICallback);` is rejected by
// a C compiler with "variable or field 'NilFFICallback' declared void" and
// "'register' redeclared as different kind of symbol" — two errors that name
// neither the function nor the keyword, because the reader has no idea a C keyword
// is what they hit. Refusing at the declaration names both.
func ValidateName(name string) error {
	if name == "" {
		return fmt.Errorf("extern \"C\" function has no name; the name is the symbol it is called by")
	}

	// Checked before the keyword list so that `register_2` is not caught by it: the
	// test has to be the whole word, which `map` lookup already is. The character
	// walk is separate because a name that is not an identifier at all is refused
	// for a different reason and should say so.
	for i, r := range name {
		valid := r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(i > 0 && r >= '0' && r <= '9')
		if !valid {
			return fmt.Errorf("extern \"C\" function is named %q, which is not a C identifier; "+
				"the name is the symbol the function is called by, so it cannot be changed",
				name)
		}
	}

	if cKeywords[name] {
		return fmt.Errorf("extern \"C\" function is named %q, which is a C keyword; "+
			"a C compiler cannot declare or define it, and the name is the symbol the "+
			"function is called by, so it cannot be changed", name)
	}
	return nil
}

// CIdentifier turns a Nilang name into one a C header can print.
//
// Used for the *parameters* of a generated prototype rather than for the function
// itself, because a parameter name is documentation and a function's name is a
// symbol. A parameter with no name is not a documentation nicety though — in a C
// declaration `NilFFICallback` alone is a *declarator*, so the compiler reads it as
// the name of a variable of an unknown type and rejects the whole header.
func CIdentifier(name string, fallback string) string {
	if ValidateName(name) != nil {
		return fallback
	}
	return name
}

// DescribeMissing names an annotation that has no crossing.
func DescribeMissing(annotation string) string {
	if annotation == "" {
		return "an unannotated parameter"
	}
	return fmt.Sprintf("%q", annotation)
}
