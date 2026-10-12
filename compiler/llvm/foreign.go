package llvm

import (
	"github.com/joysriramsarkar/nilLang/compiler/ffi"
	"github.com/joysriramsarkar/nilLang/compiler/mir"
)

// foreignDeclaration writes the `declare` for a function this program calls but
// does not define.
//
// The signature is the C one, because that is what the linker has to match: the
// whole reason this declaration exists is to say "there is a symbol of this name
// with this shape over there".
//
// A string parameter is the one that takes two LLVM arguments — a pointer and a
// length — because a C string has no length and a length is the only thing that
// distinguishes an empty string from the end of one.
//
// # Why a declaration and not a definition
//
// A `define` here would be a definition, and the linker would find a duplicate
// symbol for a function that already exists somewhere else. The failure names a
// symbol nobody here wrote, so it is worth avoiding at the source.
//
// # Why the symbol is the bare C name
//
// No `nil.` prefix. The prefix exists to keep *this module's* symbols traceable
// back to the Nilang names that produced them, and a C symbol has to be the name
// the linker will look for. Prefixing it would declare a function nothing defines
// and link with "undefined reference to nil_sin".
func foreignDeclaration(fn *mir.Function, signature ffi.Signature) string {
	return foreignDeclarationWith(fn, signature)
}
