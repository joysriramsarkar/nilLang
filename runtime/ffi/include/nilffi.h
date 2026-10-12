/* ffi.h — the boundary between a compiled Nilang program and C.
 *
 * # What this file is
 *
 * A Nilang value is a *tagged word*: three bits say what it is, the rest is the
 * payload. C has no such convention — a C `int64_t` is 64 bits of number and
 * nothing else. So a value cannot be handed to a C function as it is.
 *
 * `nilffi_double(21)` written in C receives the *tagged word* for 21, which is
 * `171`, and answers `342`; `342` read back as a Nilang value has tag bits `6`,
 * which name nothing, and the program prints `null`. A C function returning 42 is
 * read as a Nilang value with tag bits `2` — the tag of `true` — and the program
 * prints `true`.
 *
 * Nothing is wrong with either side. They are two conventions for the same 64 bits
 * and this file is where they meet.
 *
 * # The rule
 *
 * Every value crossing the boundary is converted on the way out and converted back
 * on the way in. There is no exception and no "it happens to work" case, because a
 * value that happens to work is one whose low three bits happen to be zero.
 *
 * A backend that skips the conversion produces a program that compiles, links,
 * runs, and answers a number that is wrong — and that is the worst failure this
 * file exists to prevent, because it survives every test that does not check the
 * answer.
 *
 * # Which type crosses as what
 *
 * These are fixed, and they are the ABI. Changing one is not a compatible change:
 * a program compiled against the old rule calls a C function with the wrong
 * convention and gets back something the caller reads as a different type.
 *
 *   Nilang    crosses as        notes
 *   --------  ---------------  --------------------------------------------
 *   Int       int64_t           exact for the 61 bits an inline integer has
 *   Float     double            IEEE-754, no conversion of precision
 *   Bool      int               0 or 1; a C int is not a Nilang Boolean
 *   Byte      uint8_t
 *   String    (ptr, len) pair   UTF-8 bytes, length in bytes; see L5.4
 *
 * Anything else has no crossing yet and is refused by name rather than converted
 * to something plausible.
 *
 * # Aggregates do not cross by value, in either direction
 *
 * A struct or union is never an argument and never a return. A function that takes
 * one is handed a `const <Name> *` — a pointer to a buffer the **compiler** fills
 * from the Nilang struct's fields and the **callee** only reads. A function that
 * would return one is refused at the declaration, by name.
 *
 * The buffer is the one below (`nil_ffi_buffer`), so it lives exactly as long as the
 * call and is released when the call returns. A callee must not free it and must not
 * keep it: the same rule as any other argument here.
 *
 * A Nilang struct crosses with **C's layout** — each field at the next offset
 * satisfying its own alignment, the struct's size rounded up to its own alignment.
 * You do not write the struct: `nil header` emits the typedef from that same layout,
 * so including the generated header gives you the struct the program actually has.
 * A `String` field is two members, the pointer then `size_t` the length.
 *
 * A struct coming *back* is refused rather than routed, and the reason is ownership
 * rather than difficulty: nothing builds a Nilang struct out of C-owned bytes, so a
 * pointer would hand the program a tagged word pointing at memory the GC does not
 * own. A *string* comes back through `NilFFIString` because the runtime knows how to
 * build a string from a pointer and a length. There is no such thing yet for a struct.
 *
 * This is the one rule here that is not about types, and it is written down because
 * it looks negotiable. A struct's *size and alignment* are agreed — every compiler
 * targeting an ABI computes them the same way, and `nil_ffi_sizeof` exists so that
 * is a thing to check rather than assume. What is not agreed is **how a struct
 * return is classified**: whether it comes back in registers or through a hidden
 * pointer is a per-compiler decision.
 *
 * On one machine, with two compilers in the same link:
 *
 *   clang, x86_64-pc-windows-msvc   returns {ptr, i64} in RAX:RDX
 *   gcc,   x86_64-w64-mingw32       returns it through a pointer in RCX
 *
 * Both correct. Both present. Nothing at the call site can tell which one it got.
 * So a struct crosses through memory the caller owns, which has one meaning
 * everywhere.
 *
 * This is why `nil_string_unbox` writes through a pointer instead of returning a
 * struct: the first version did return one, linked cleanly on a machine holding both
 * compilers, and answered every string with length zero. Not a warning, not an
 * error — a wrong answer, produced by two correct compilers.
 *
 * # Why these names
 *
 * `nil_` rather than `nilffi_`, because they are the FFI's own surface and not the
 * language's. A program calling `nil_int_new` is using the runtime; a program
 * calling `nil_int_box` is crossing a boundary, and the two deserve to be
 * distinguishable when reading a stack trace. The version guard above breaks that
 * rule deliberately: `nil_ffi_abi_v1` says "FFI" *and* the version, because the
 * linker needs both.
 */

#ifndef NIL_FFI_H
#define NIL_FFI_H

#include <stddef.h>
#include <stdint.h>

#include "nil.h"

#ifdef __cplusplus
extern "C" {
#endif

/* ── Version ──────────────────────────────────────────────────────────── */

/* The version of this file's contract.
 *
 * # Why it is a number and not a date
 *
 * Because it is not a date. A date says when this was written, and every later
 * addition changes it, which means every later addition breaks every object built
 * before it — including additions that changed nothing at the boundary. A version
 * changes only when something a compiled object already depends on changes.
 *
 * # What counts as a change
 *
 * Adding a function. Changing a signature. Changing what a type crosses as.
 * Changing who owns what. Anything a C author writes against.
 *
 * Adding a comment, a new error message, or an implementation of an existing
 * promise does not. Those change no compiled object's meaning.
 *
 * # What each version added
 *
 * `1` — the conversions, the buffer scope, callbacks, the layout queries and the
 *      version guard.
 * `2` — platform channels: `nil_encode`, `nil_decode`, and the six `nil_channel_*`
 *      entry points. Additive, so a v1 program still links and still means the same
 *      thing; raised because the header now promises something v1 did not, and a
 *      reader comparing versions should be able to see that.
 *
 * Note that the message *format* carries its own version, `NIL_CHANNEL_ABI_VERSION`,
 * and separately from this one. A message is a value that crosses, and changing how a
 * value is written is a different kind of break from adding a way to send one: the
 * first needs both sides upgraded together, the second needs nothing. */
#define NIL_FFI_ABI_VERSION 2

/* What this runtime actually is, at run time.
 *
 * The constant above is what *you* were compiled against; this is what is loaded.
 * They are the same number on a matching runtime, and on a mismatched one this is
 * the number that tells you which.
 *
 * Exists alongside the link-time guard rather than instead of it: the guard turns
 * most mismatches into a build failure, which is the good outcome, and this covers
 * the ones that slip past — a library that exports both versions, or a program that
 * found the symbol somewhere unexpected. */
int64_t nil_ffi_version(void);

/* The link-time guard, one symbol per version.
 *
 * # Why a symbol and not a check
 *
 * Because a check reports a mismatch at run time, and a mismatch at run time is a
 * program that runs and answers wrongly. A missing symbol is a *build* failure that
 * names the symbol, on the machine that will not work, before anybody depends on
 * the answer.
 *
 * # Why the version is in the name
 *
 * So the linker does the version check. `nil_ffi_abi_v1` and `nil_ffi_abi_v2` are
 * different symbols, so a v1 object against a v2-only runtime fails to resolve —
 * there is no "compatible" case for the linker to guess at. A single symbol that
 * both versions define would defeat the entire arrangement.
 *
 * The body is empty and always will be. It is called for its existence, and a body
 * would only be somewhere for the two halves to disagree.
 *
 * Generated code calls this in every module that declares an `extern`, so a program
 * that never crosses a boundary never links against it and never pays for it. */
void nil_ffi_abi_v2(void);

/* ── Layout ───────────────────────────────────────────────────────────── */

/* What to measure. */
typedef enum NilFFILayout {
  NIL_FFI_LAYOUT_VALUE = 0,    /* NilValue — the tagged word that crosses   */
  NIL_FFI_LAYOUT_OBJECT = 1,   /* NilObject — the header every object has  */
  NIL_FFI_LAYOUT_STRING = 2,   /* NilString                               */
  NIL_FFI_LAYOUT_CLOSURE = 3,  /* NilClosure                              */
  NIL_FFI_LAYOUT_CALLBACK = 4, /* NilFFICallback — what a C side stores    */
  NIL_FFI_LAYOUT_CROSSING = 5  /* NilFFIString — the struct a string uses  */
} NilFFILayout;

/* How many bytes a runtime type occupies, or -1 for one this file does not know.
 *
 * # Why a program should ask rather than write `sizeof`
 *
 * Because these are the *runtime's* types, and the runtime is a shared library that
 * a program does not include the source of. A program that writes `sizeof
 * (NilString)` has assumed that the compiler it is using and the runtime it linked
 * agree about the layout — and where that is true is a fact to be tested, not one
 * to be assumed.
 *
 * This is the part of the boundary that *is* agreed. `sizeof` and `alignof` are
 * properties of a type, and every compiler targeting the same ABI computes them the
 * same way. Struct *returns* are not: whether a small aggregate comes back in
 * registers or through a hidden pointer is a per-compiler decision, which is why
 * nothing in this file returns one. See ADR-0072 and the note on aggregates below.
 *
 * Refused rather than answered for an unknown selector. A default would be a guess
 * about a number a program is about to allocate with. */
int64_t nil_ffi_sizeof(NilFFILayout what);

/* How strictly a runtime type is aligned, or -1 for one this file does not know.
 *
 * Alignment, not size, is what catches a mistake here: a type of the right size
 * and the wrong alignment still crashes, and only on the code that happened to be
 * unaligned. */
int64_t nil_ffi_alignof(NilFFILayout what);

/* ── Numbers ──────────────────────────────────────────────────────────── */

/* A Nilang integer to a C integer, and back.
 *
 * Exact both ways for the 61 bits an inline integer carries. An integer too large
 * for that is boxed on the Nilang side, and unboxing it reads the box — so the
 * crossing is exact for every integer the language can hold, not just the common
 * ones.
 *
 * A value that is not an integer converts to 0 rather than to its bit pattern.
 * That is deliberate and it is the *unboxed* behaviour, not a silent truncation:
 * the bit pattern is a number the program never wrote, and a wrong large integer
 * is much harder to notice than a zero.
 */
int64_t nil_int_unbox(NilValue value);
NilValue nil_int_box(int64_t value);

/* A Nilang float to a C double, and back.
 *
 * Bit-exact: the value is copied rather than converted, so a double that made the
 * crossing unchanged comes back unchanged.
 *
 * A float is always boxed, because a double does not fit in 61 bits and there is
 * no inline form for it. That is why `nil_float_unbox` is not simply a shift.
 */
double nil_float_unbox(NilValue value);
NilValue nil_float_box(double value);

/* ── Booleans ─────────────────────────────────────────────────────────── */

/* A Nilang boolean to a C int, and back.
 *
 * 0 and 1 out, because that is what a C `if` tests and what every C API that
 * returns a status means. A Boolean *in*, because a C function returning 2 or -1
 * is returning something truthy and refusing it would make ordinary C APIs
 * unusable. A zero back becomes `false` rather than a tag whose bits are zero,
 * which would be `null`.
 */
int nil_bool_unbox(NilValue value);
NilValue nil_bool_box(int value);

/* ── Bytes ────────────────────────────────────────────────────────────── */

uint8_t nil_byte_unbox(NilValue value);
NilValue nil_byte_box(uint8_t value);

/* ── Callbacks ────────────────────────────────────────────────────────── */

/* A Nilang function crosses to C as a *closure object*: a heap value holding a
 * function pointer and whatever that function captured.
 *
 * # Why a heap object and not a bare pointer
 *
 * Because a function is not all a closure is. `let factor = 3; fn scaled(n) { n *
 * factor }` captures `factor`, and `factor` is a heap value the collector may
 * reclaim. A bare function pointer carries nothing about what it needs, so nothing
 * can keep those captures alive, and the callback reads freed memory on a call that
 * may not happen for hours.
 *
 * # Why the collector does not have to move it
 *
 * It does not move anything (ADR-0041), so the pointer C holds stays valid for as
 * long as the object does. That is what makes the whole thing cheap: the C side
 * stores one word and never tells the collector it is there. What it must not do
 * is let the object be *collected*, and that is what the pin below is for.
 */

/* The shape of every Nilang function as C sees it.
 *
 * Named, rather than spelled out at each use, for two reasons. A C side declaring
 * its own callback variable needs a type it can write; and a function-pointer
 * parameter spelled out is long enough that it wraps across lines, which puts a
 * `)` in the middle of the declaration and breaks any tool that reads these
 * headers by pattern — including this project's own ABI check.
 */
typedef NilValue (*NilFFICallback)(NilValue *args, int64_t count);

/* Builds a callback from a function pointer and the number of arguments it takes.
 *
 * Pins the result for as long as C holds it, so the pointer C was given stays
 * valid and the captures stay alive. The pin ends at `nil_callback_release`, and
 * not before — a callback C still holds is one a collection must not reclaim.
 *
 * A non-positive arity is refused. It is the one number that cannot be recovered
 * from anywhere else: the function pointer says nothing about its own signature,
 * and a callback that answers "how many arguments" with a guess is a callback that
 * reads past the end of an array. */
NilValue nil_callback_new(NilFFICallback function, int64_t arity);

/* Calls a callback from C.
 *
 * The portable form, and the one the tests use: no function-pointer round trip is
 * involved, so nothing here depends on a function pointer being convertible to an
 * integer — which is not guaranteed by C and is not true on every platform.
 *
 * Returns nil and records a type error for anything that is not a callback, rather
 * than stopping: a callback arriving wrong is a caller's mistake, and the caller
 * may well be able to do something else instead. */
NilValue nil_callback_invoke(NilValue callback, NilValue *args, int64_t count);

/* The same call, as a function pointer C may store and call later.
 *
 * # Why a callback crosses as two values, not one
 *
 * Because a C function pointer carries no context, and a callback must know *which*
 * Nilang function it is. Giving it context means either passing the context
 * alongside the pointer — which is what this is — or generating a fresh piece of
 * machine code per callback so that the pointer alone carries it.
 *
 * The first is one function and a documented convention. The second is an
 * executable-memory allocator and a different one per architecture, and a
 * generated trampoline that reads a stack slot the compiler was wrong about is a
 * crash with no stack trace pointing at anything anybody wrote. So:
 *
 *     NilValue (*trampoline)(NilValue self, NilValue *args, int64_t count) =
 *         nil_callback_trampoline;
 *
 * and the call is `trampoline(callback, args, count)` where `callback` is the
 * context. Store both. Pass both. Every portable FFI does this.
 *
 * # Why this is a *known* function rather than one the runtime hands out
 *
 * Because its address is a compile-time constant. C can write `nil_callback_trampoline`
 * by name and never call anything to ask for it, so there is no function-pointer
 * value anywhere in the ABI, and nothing depends on one surviving a round trip
 * through an integer.
 */
NilValue nil_callback_trampoline(NilValue self, NilValue *args, int64_t count);

/* How many arguments a callback takes, or -1 when the value is not a callback.
 *
 * For the C side's own check before calling: a callback whose arity does not match
 * what C is about to pass would otherwise read past the end of its argument array,
 * and the crash would name nothing. */
int64_t nil_callback_arity(NilValue callback);

/* Releases a callback C has finished with.
 *
 * # When it must be called
 *
 * Exactly once the C side will not call the callback again. Until then the pin
 * holds, and the object cannot be collected — which is the point.
 *
 * Releasing early is a use-after-free waiting for a call that has not happened
 * yet. Forgetting to release is a leak, which is a smaller failure: it costs
 * memory and the program keeps working.
 *
 * Releasing twice, or releasing something that is not a callback, is ignored rather
 * than fatal. C code holding a pointer may legitimately be unsure, and stopping a
 * program over a double release leaves it in a worse state than continuing. */
void nil_callback_release(NilValue callback);

/* A string crosses as a pointer and a length — two arguments, never one.
 *
 * Never one, because a C string has no length and the length is the only thing
 * that distinguishes an empty string from the end of one. A single `const char *`
 * would make "the empty string" and "a pointer past the end" the same value, and
 * every C function written to take a length would have to guess.
 *
 * The length is in **bytes**, not characters. A C function counting bytes and a
 * Nilang program counting characters disagree about any string outside ASCII, and
 * the C side has to be the one that is right: it is handing over a buffer, and a
 * buffer has a size in bytes.
 */
typedef struct NilFFIString {
  const char *bytes;
  size_t length;
} NilFFIString;

/* A string crosses **out** as two arguments and comes back **through a pointer the
 * caller owns**. Nothing returns a `NilFFIString` by value.
 *
 * That rule is not caution. Whether a 16-byte struct comes back in registers or
 * through a hidden pointer is *not* a convention two compilers share: clang
 * assembling a module with an `x86_64-pc-windows-msvc` triple returns `{ptr, i64}`
 * in RAX:RDX, while GCC for `x86_64-w64-mingw32` returns it through a pointer the
 * caller passes in RCX. Both are correct implementations of their own ABI, both
 * appear in the same link, and nothing at the call site can tell which one it got.
 *
 * That is not hypothetical — it is what this file was written against. The first
 * version of `nil_string_unbox` returned the struct, linked cleanly on a machine
 * with both compilers, and answered every string with length zero: the half in
 * RAX:RDX was read from a callee that had written through a pointer.
 *
 * So the caller allocates, the callee fills, and both sides agree because neither
 * side had to know a register convention at all. A pointer argument is just another
 * argument; a struct return is the one thing in this file whose meaning depends on
 * who compiled it.
 *
 * A C function that answers a Nilang string with a `NilFFIString *out` parameter
 * follows the same rule, which is why the fixtures answer this way too.
 *
 * ## Who owns the bytes a function answers with
 *
 * **The callee keeps them.** The crossing copies them into a Nilang-owned buffer
 * immediately after the call returns, so the bytes have to still be there at that
 * moment — and a callee that frees before returning has freed them one instruction
 * too early. This is a borrow, and the borrower is the crossing.
 *
 * That reads like a rule a C author will forget, and they will. So a function that
 * *builds* its answer asks for the bytes from `nil_ffi_buffer` instead, and the
 * runtime reclaims them when the call returns — which turns "remember not to free"
 * into "never allocated it yourself". A function whose answer it does not own — a
 * `static` string, a pointer into a library's table, the very pointer it was
 * handed — hands that pointer over unchanged and the crossing leaves it alone.
 *
 * Both mistakes are safe: one leaks, the other is a no-op. See the ownership table
 * near the bottom of this file. */

/* The Nilang string to a pointer and a length.
 *
 * An empty string crosses as a null pointer and a zero length, which every C API
 * written to take a length handles and which `memcpy(dst, NULL, 0)` is defined to
 * do. Reporting a non-null pointer to nothing would be worse: a C function that
 * checks for null before reading would treat it as a real string and then read
 * address zero.
 *
 * The pointer is to the string's own bytes and stays valid exactly as long as the
 * string does — see `nil_string_copy` for the case where a caller needs more than
 * that.
 *
 * Always fills `out`, never fails: a value that is not a string answers the same
 * as an empty one, so there is no "half written" case to report. The earlier
 * objection to an out-parameter — that it cannot say only one field was filled —
 * was correct in general and did not apply here, because this function has exactly
 * one answer and no way to fail.
 */
void nil_string_unbox(NilValue value, NilFFIString *out);

/* Copies a C buffer into a Nilang-owned one and returns it as a string.
 *
 * This is the inverse of `nil_string_unbox` and the only way bytes become a Nilang
 * string. It copies rather than borrows, so the C side may free its buffer as soon
 * as it returns. That is the price of not having to specify when a borrow ends,
 * which every caller would otherwise have to know — and a caller that gets it
 * wrong produces a string that reads freed memory, which is the failure this whole
 * file exists to prevent.
 *
 * For the case where the bytes must outlive the string they came from: a value C
 * built and will free, say. Without this the only options are to append to a
 * `NilBuilder` by hand or to keep the buffer alive forever.
 *
 * A null pointer with a zero length is the empty string, not an error: it is the
 * other half of what `nil_string_unbox` produces for one. A null pointer with a
 * *non-zero* length is answered nil, because that is not a string and handing
 * `strlen` a null would be worse than saying so. */
NilValue nil_string_copy(const char *bytes, size_t length);

/* ── Buffers ──────────────────────────────────────────────────────────── */

/* Writable bytes the runtime owns and reclaims.
 *
 * A C function that answers a Nilang string with bytes it built itself cannot use
 * `malloc` and cannot use a `static` buffer either: the first leaks, because the
 * crossing copies the bytes and nobody is left to free them, and the second can only
 * hold an answer whose size is known when the program was written. A function that
 * formats a number into a string has no fixed size.
 *
 * This is the answer to that, and it exists so that "remember not to free" becomes
 * "never allocated it yourself".
 *
 * The bytes are valid until the foreign call that asked for them returns. That is
 * exactly as long as the crossing needs them — it copies immediately, before
 * anything else can run — so the lifetime needs no arithmetic and no bookkeeping on
 * the C side.
 *
 * `length` is in bytes. A NUL terminator is written when there is room for one, so
 * a function that hands the buffer to `printf` does not have to think about it; the
 * returned `length` never counts it.
 *
 * Answers NULL for a negative or absurd length, which is a caller that has lost
 * track rather than one asking for a large string. */
char *nil_ffi_buffer(int64_t length);

/* Marks where this call's buffers begin, and hands back the struct to cross through.
 *
 * Called by the caller, before the foreign call, and paired with
 * `nil_ffi_scope_close` after it. Not a call a C author writes — it is here because
 * generated code makes it, and because a C author who allocates and frees by hand is
 * solving a problem this file already solved.
 *
 * # Why it returns the struct rather than the caller allocating one
 *
 * Because the caller cannot allocate one safely. An `alloca` written at a call site
 * is inside whatever block that call is in, so a call in a loop body allocated once
 * per iteration and the stack grew for as long as the loop ran — a stack overflow
 * that only appears at a million iterations, on the one platform whose stack probe
 * happens to notice. That is not a bug that shows up in testing; it is the kind that
 * ships.
 *
 * Hoisting the `alloca` into the entry block fixes the growth and costs a separate
 * concept: the struct's lifetime is no longer the frame's, it is the scope's, and
 * something has to keep track of which scope. The runtime is already keeping track
 * of one, for the buffers. So the struct joins them — one scope, one lifetime, one
 * thing to get wrong instead of two.
 *
 * # One struct for the whole call
 *
 * The same struct serves the arguments and the return. Arguments are unboxed and
 * read out into SSA values *before* the call, so by the time the callee fills it
 * for its answer nothing is still pointing into it — and two string arguments get
 * two rounds through one slot rather than two slots, which is what makes "one per
 * call" possible.
 *
 * # Why a scope rather than a flag
 *
 * Because scopes nest: a C function may call a callback, and that callback may be a
 * whole Nilang program that makes its own string-returning call. Each opens its own
 * and each closes its own. */
NilFFIString *nil_ffi_scope_open(void);

/* Ends the buffer scope opened for one foreign call.
 *
 * Called by the caller, not by the callee: the crossing knows the answer has been
 * copied, and the callee does not. It frees every buffer allocated since the
 * matching `nil_ffi_scope_open`.
 *
 * A callee that answers with a `static` buffer, or with the pointer it was handed,
 * is untouched: the scope only frees what came out of `nil_ffi_buffer`. Both
 * mistakes are safe — one leaks, the other is a no-op — which is the property that
 * matters here. See the ownership table below. */
void nil_ffi_scope_close(void);

/* How many buffers are outstanding, across every open scope.
 *
 * For a test, and for a C author who wants to see whether a leak is happening. A
 * program that calls a string-returning C function in a loop answers the same
 * number on every iteration; a program that leaks answers a larger one each time. */
int64_t nil_ffi_buffer_outstanding(void);

/* ── Who owns what ────────────────────────────────────────────────────── */

/* The whole ownership contract in one place, because the alternative is a C author
 * reading five functions and assembling the rules in their head — and getting one
 * of them wrong in a way that only shows up as a corrupted heap three calls later.
 *
 * | you have                            | it is owned by     | it dies when              | you must     |
 * |-------------------------------------|--------------------|--------------------------|-------------|
 * | a string out                        | the Nilang string  | with the Nilang string    | nothing     |
 * | a buffer from `nil_ffi_buffer`      | the runtime        | at `nil_ffi_scope_close`  | nothing     |
 * | the struct from `nil_ffi_scope_open`| the runtime        | at `nil_ffi_scope_close`  | nothing     |
 * | a callback from `nil_callback_new`  | the runtime        | at `nil_callback_release` | release it  |
 * | a `NilValue` of any kind            | the collector      | at the next collection    | nothing     |
 *
 * Two rules fall out of that table, and they are the ones people get wrong:
 *
 * **Nothing a C function receives needs freeing.** A string out is the Nilang
 * string's own bytes. A callback is the collector's. There is no `nil_free`, and
 * that is deliberate — the collector owns every `NilValue` and frees it at the next
 * sweep, so freeing one by hand leaves a pointer in the allocation chain that the
 * *next* sweep follows. A use-after-free that happens later and names nothing.
 *
 * **Nothing a C function returns needs freeing either**, provided it built the
 * answer with `nil_ffi_buffer`. If it did not — because the answer is a `static`
 * string, or a pointer into a table the library owns, or the very pointer it was
 * handed — then the callee owns it and the callee knows that. The crossing copies
 * either way and frees only what `nil_ffi_buffer` produced.
 *
 * The asymmetry is the point. A caller cannot tell which of the two it got, so it
 * must not free either. A callee knows exactly what it built, so it is the one the
 * rules are written for. */

/* Whether a value crosses at all.
 *
 * # Who asks this
 *
 * C code holding a `NilValue` — which is what a callback is handed — and about to
 * hand it to another C function as a crossing. The value came from a program, so it
 * could be anything the language has: an integer, a string, a list, a closure. Only
 * some of those have a crossing, and handing over one that has not means C reads a
 * bit pattern as something it is not.
 *
 * A list answers 0 because there is no crossing for a list. So does a closure the
 * program built and did not register — a callback crosses as a *handle*, but that
 * handle is made by `nil_callback_new` and not by the program. Asking is cheap, and
 * the alternative is `nil_int_unbox` on something that is not an integer, which is
 * the exact bug this FFI has spent its life refusing to produce.
 *
 * A backend does not ask this. It refuses an unsupported *type* at the declaration,
 * where the programmer can see it. This is about values, which arrive at run time
 * from a program and cannot be checked any earlier.
 *
 * Answers 1 rather than "yes", so the question reads as a predicate at a call site
 * that is mostly deciding between this and a refusal. */
int nil_ffi_can_cross(NilValue value);

/* ── Platform channels ────────────────────────────────────────────────── */

/* A host and a guest exchanging messages, over time rather than through a call.
 *
 * # What this is not
 *
 * Not a `Channel`. The language's `Channel` is a Go-style buffer inside one process,
 * with no encoding and no host; a platform channel has a host on the other end that
 * is not this program and does not share its types. See `docs/spec` §9.18.
 *
 * # The shape of one message
 *
 * Three parts: a `token` the host chooses, a `method` name, and one `argument`
 * value. The token comes back on the reply untouched, which is how a host matches an
 * answer to the question that asked for it — the channel itself does not look at it.
 *
 * # Who blocks
 *
 * `nil_channel_take_reply` blocks the host. `nil_channel_serve` blocks the guest. A
 * post never blocks on either — it takes the channel's lock, appends, and signals —
 * so a host thread may post at any time, including while the guest is inside a
 * handler, and including long after the call that created the channel returned.
 *
 * That last property is the whole feature. Everything else in this file answers
 * within one call; a channel is how a host starts something.
 *
 * # What a message may contain
 *
 * `null`, `true`, `false`, an integer, a float, a string, a list, or a map —
 * written by `nil_encode` and read by `nil_decode`. A closure has no meaning outside
 * the program that made it and is **refused**, at the post rather than at the decode,
 * so the host is told when it happens rather than receiving something unreadable. */
#define NIL_CHANNEL_ABI_VERSION 1

/* How many bytes `nil_encode` will write for a value, or -1 if it cannot cross.
 *
 * Separate from the encode itself because the caller needs a buffer before there is
 * anything to put in one. */
int64_t nil_encoded_size(NilValue value);

/* Writes a value's message form into `out`, answering how many bytes it wrote.
 *
 * Answers -1 rather than a short write when the value cannot cross or the buffer is
 * too small, so a caller can never mistake a refusal for a message. */
int64_t nil_encode(NilValue value, char *out, int64_t capacity);

/* Reads a message back into a value, answering nil and setting the error if the bytes
 * are not a well-formed message.
 *
 * Every read is bounds-checked and a count inside a message is checked against the
 * bytes that are actually left, because the sender is not obliged to be this
 * version. Trailing bytes are refused: they mean the two sides disagree about what
 * the message is, and reading the part both understand would hide exactly that. */
NilValue nil_decode(const char *bytes, int64_t length);

/* Opens a channel, answering its handle. Zero is never a channel.
 *
 * A handle rather than a `NilValue` because a channel has an end — `nil_channel_close`
 * — and a value the collector owns cannot have one without teaching the collector
 * about it. This is the same bargain `nil_callback_release` makes. */
int64_t nil_channel_new(void);

/* Queues a message for the guest, answering how many bytes it took.
 *
 * May be called from any thread at any time, including while the guest is running a
 * handler, and long after the call that opened the channel returned.
 *
 * The method and the argument are **bytes**, not values, and that is the whole design
 * rather than an inconvenience:
 *
 *   - A host that had to build a `NilValue` would have to be built against the
 *     runtime's representation to send anything, which is the one thing a host is not.
 *   - Building one here would touch the collector, whose pin list and root set are
 *     process-wide rather than per-thread. A host thread doing that while the guest
 *     thread collects is two threads corrupting one list, and it is an access
 *     violation rather than a subtle failure.
 *
 * The wire form is a four-byte big-endian method length, the method's bytes, and then
 * the argument's own encoded bytes. A host can produce that in five lines, and
 * `nil_encode` produces the argument half if it has a value to hand.
 *
 * Answers -1 and sets the error if the channel is closed or not open — a refusal rather
 * than a drop, because a host posting into a closed channel has a bug and should be
 * told. */
int64_t nil_channel_post(int64_t handle, int64_t token, const char *method, int64_t method_length,
                         const char *argument, int64_t argument_length);

/* Answers the guest's reply, copied into the caller's buffer scope, and reports the
 * token it was posted under.
 *
 * Blocks until one is queued or the channel is closed. The copy is not optional: a
 * queued message belongs to the queue and is freed the moment it is taken, so handing
 * over a pointer to it would be handing over memory that is already gone.
 *
 * The answer arrives as bytes for the same reason the message went out as bytes.
 * `nil_decode` turns it into a value, on whichever thread asks. */
int64_t nil_channel_take_reply(int64_t handle, int64_t *token, NilFFIString *out);

/* Serves the channel until it is closed, calling `handler` for each message.
 *
 * `handler` is a callback of arity 2, called as `handler(method, argument)` and
 * answering the value to send back. The method is a string and the argument is
 * whatever the host encoded; the answer is encoded the same way, so both sides agree
 * without either sharing a representation.
 *
 * The arity is checked once, here, because this is the only moment where the count and
 * a signature can be compared.
 *
 * Blocks the calling thread and runs the handler **on that thread** — guest code
 * never runs on a host's thread, which is the property that makes it safe for a host
 * to post without knowing what the guest is doing.
 *
 * # Why only one thread may serve
 *
 * Because the runtime's collector is not thread-safe: its pin list and its root set
 * are process-wide. `nil_channel_post` and `nil_channel_take_reply` are safe from any
 * thread precisely because they touch neither — they move bytes. Anything that
 * *builds* a value is the guest's, and the guest is one thread.
 *
 * Answers 0 when the channel closed and every message was answered, -1 if the handler
 * was refused, a message could not be decoded, or its answer could not cross. */
int64_t nil_channel_serve(int64_t handle, NilValue handler);

/* Closes the channel, waking whoever is waiting on either side and discarding anything
 * still queued.
 *
 * A close says the conversation is over; a host that has not taken its replies has its
 * own bug, and dropping them is how the host finds out. Holding the channel open until
 * the queues drained would instead mean a guest that exits without being closed leaves
 * it open for ever, which is worse than a lost message.
 *
 * Answers 1 if the channel was open, 0 if it was not — so closing twice is ordinary
 * code, not an error, and so "was it open" is the question the host actually asked.
 *
 * The handle stops working the moment this returns. A host must not post afterwards,
 * and a stale handle is *detected* rather than obeyed: handles carry a generation, so a
 * handle from a closed channel names nothing rather than somebody else's channel. */
int64_t nil_channel_close(int64_t handle);

/* Channels open times a thousand, plus messages queued — what a leak test reads.
 *
 * One number rather than two so that "a program did not leak" is one claim, and the
 * thousands are large enough that the two halves never overlap. */
int64_t nil_channel_outstanding(void);

#ifdef __cplusplus
}
#endif

#endif /* NIL_FFI_H */
