// Package llvm emits textual LLVM IR from MIR, and knows which toolchain turns
// that IR into a binary.
//
// # Why text and not a binding
//
// The obvious way to target LLVM from Go is a binding to its C++ API. It is also
// the way that makes the compiler depend on a specific LLVM version, a specific
// C++ toolchain, and a specific platform's shared-library conventions. A language
// that wants to build for eight targets does not want those constraints on the
// *compiler*; it wants them on the *machine doing the build*.
//
// Textual IR is a documented interchange format. Anything that can read `.ll` —
// `llc`, `clang`, `lli`, a web service, a future version of LLVM — can consume
// what this package writes, and none of those has to be present to build it.
//
// # What this package does not claim
//
// Writing correct IR and having a compiler accept it are different claims. The
// emitter is tested against the text it produces, which is the part that can be
// checked without a toolchain. Whether a given `llc` accepts that text is checked
// by the integration tests, and only where one is installed.
package llvm
