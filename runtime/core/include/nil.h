/* nil.h — libnilcore: the Nilang runtime for ahead-of-time compiled programs.
 *
 * # What this header promises
 *
 * Generated code includes this and nothing else. Every function here is part of
 * the interface that a compiler may emit a call to, so each one is a promise this
 * library has to keep: change a signature and every object ever compiled against
 * it breaks in a way that links cleanly and misbehaves at runtime.
 *
 * The counterpart rule is that nothing *else* is public. Collector internals and
 * helpers whose signature exists only so two `.c` files agree on it live in
 * `nil_internal.h`, which generated code does not include — see that file for why.
 *
 * # One value type, on purpose
 *
 * `NilValue` is an `i64` and every function takes and returns one. A string, an
 * integer, a boolean, a list, a map and a function are all a single tagged word,
 * so a function taking three different kinds of thing takes three `i64`s.
 *
 * This is what lets compiled code hold a value in a register, pass it to a runtime
 * call and get one back, with no boxing protocol and no second representation on
 * the other side of every call. It is also the reason the collector cannot move
 * objects — see ADR-0041.
 *
 * # The tag bits
 *
 * Three bits at the top of the word say what a value is, leaving 61 bits of
 * payload. That is why an integer up to 2^60 fits without touching the heap, and
 * why `nil_int_new` answers an ordinary function call rather than an allocation.
 *
 * The one consequence worth writing down: `1` is the tag of *false* and `2` is the
 * tag of *true*, so a raw `0` or `1` arriving from a comparison or a truthiness
 * test is not a Boolean and must never be treated as one. `nil_truthy` returns
 * `int64_t` rather than `int` so that this mistake is a type error at the call
 * site rather than a wrong answer at runtime.
 *
 * `NIL_TAG_PTR` on its own is the tagged null: a value that says "pointer" and
 * carries no address. It must never be dereferenced, which is why every predicate
 * goes through `nil_object_kind` rather than reading a header.
 *
 * # Errors
 *
 * Two kinds. A *type* failure sets the error and answers nil, because the caller
 * may be able to do something else with the value. Anything else stops the
 * program, because a program that has asked for something impossible and received
 * nil will carry on producing wrong answers. The exit codes are stable; see below.
 */

#ifndef NIL_H
#define NIL_H

#include <stddef.h>
#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

/* ── Representation ───────────────────────────────────────────────────── */

/* A Nilang value: a tagged word.
 *
 * The type is `uint64_t` rather than a pointer so that a value is the same size
 * and the same alignment as an LLVM `i64`, which is the type generated code
 * carries it in. */
typedef uint64_t NilValue;

/* The header every heap object starts with.
 *
 * Objects are allocated with their own kind and a link into the collection chain
 * already in place, so the collector can see an object before its fields are
 * written.
 *
 * `length` is in the common header rather than in the sized types because
 * truthiness reads it through `nil_object_header`, which returns a `NilObject *`
 * and knows nothing about what kind of thing it is holding. A header that could
 * not be asked "how long are you" would mean that question had to be answered
 * twice — once here for a container that has a length, and once at every use for
 * one that does not. The kinds without a length pay four bytes they do not use. */
typedef struct NilObject {
  uint8_t kind;  /* one of NIL_OBJ_* */
  int marked;    /* collector's mark bit */
  struct NilObject *next;
  uint32_t length;
} NilObject;

/* A string: the header, then its bytes, null-terminated.
 *
 * The terminator is one byte the length does not count, so `data` can be handed
 * to anything that takes a C string while `length` stays the truth about how many
 * bytes belong to the value. */
typedef struct NilString {
  NilObject header;
  char *data;
} NilString;

/* A box for a number too wide to be inline. Integers reach this only past 61
 * bits; floats always do. */
typedef struct NilBox {
  NilObject object;
  int64_t value;
} NilBox;

/* A list: the header, then its elements. */
typedef struct NilArray {
  NilObject header;
  NilValue *data;
  size_t capacity;
} NilArray;

/* A map: the header, then three parallel arrays and one of slot indices.
 *
 * Open-addressed, with `occupied` saying which slots hold something. An empty slot
 * is always empty — a key is never removed, only added or replaced — which is what
 * lets a probe sequence stop at the first empty slot. See collections.c for why
 * that removes the need for tombstones.
 *
 * `order` holds the slot of each key in the order that key was first inserted, so
 * `order[0 .. count)` is the map's *written* order and everything else is a
 * consequence of hashing.
 *
 * # Why the order is kept at all
 *
 * Because a struct is a map, and a struct's fields print in the order they were
 * declared — which is the one order a reader has to be able to rely on. So is a
 * hash literal: SEMANTICS.md §1 requires its entries to be evaluated and printed
 * left to right.
 *
 * The table's own order is a hash order, and printing in it made `struct P { b: Int,
 * a: Int }` come out as `{a: 2, b: 1}` under the compiler and `{b: 1, a: 2}` under
 * the interpreters. Two engines, one language, two answers to the same program —
 * found because a skip in the conformance suite said the compiler had no struct
 * lowering at all, which had stopped being true some time before and so had stopped
 * being checked.
 *
 * One array of `size_t` per entry. It costs eight bytes an entry and buys a property
 * a program can print; the alternative is two engines disagreeing about what a value
 * looks like. */
typedef struct NilMap {
  NilObject header;
  NilValue *keys;
  NilValue *values;
  unsigned char *occupied;
  size_t *order;
  size_t count;
  size_t capacity;
} NilMap;

/* A function value: a C function plus whatever it captured.
 *
 * `arity` is how many arguments it takes. It is here rather than inferred because
 * nothing can infer it: the C side holds a pointer and calls through it, and the
 * only moment the two could disagree is the moment of the call — after which the
 * answer is whatever was in the registers. Checking here turns a wrong argument
 * count into a message instead of a crash. */
typedef struct NilClosure {
  NilObject object;
  NilValue (*function)(NilValue *args, int64_t count);
  NilValue *captures;
  size_t captureCount;
  int64_t arity;
} NilClosure;

/* A growable byte buffer, used to build a string without knowing its length
 * first. Not a Nilang value: it is a tool the runtime uses to make one. */
typedef struct NilBuilder {
  char *data;
  size_t length;
  size_t capacity;
} NilBuilder;

/* ── Tags ─────────────────────────────────────────────────────────────── */

#define NIL_TAG_MASK 7u
#define NIL_TAG_NIL 0u   /* null: no payload at all */
#define NIL_TAG_FALSE 1u /* false */
#define NIL_TAG_TRUE 2u  /* true */
#define NIL_TAG_INT 3u   /* an integer inline, 61 bits of payload */
#define NIL_TAG_PTR 4u   /* a pointer, or the tagged null when it carries none */

/* ── Object kinds ─────────────────────────────────────────────────────── */

/* The kind byte in a NilObject header.
 *
 * Kinds are numbered from 1 so that a zeroed object — which is what the allocator
 * hands back before anything is written — reads as "not an object" rather than as
 * a valid kind. */
#define NIL_OBJ_STRING 1
#define NIL_OBJ_ARRAY 2
#define NIL_OBJ_MAP 3
#define NIL_OBJ_CLOSURE 4
#define NIL_OBJ_INTBOX 5
#define NIL_OBJ_FLOATBOX 6

/* ── Exit codes ───────────────────────────────────────────────────────── */

/* The process exit status for each kind of failure.
 *
 * Stable, because a build system reads them. The split that matters is 0 and 1
 * against the rest: anything below NIL_ERR_INDEX is a mistake in the program and
 * anything from NIL_ERR_INDEX up is a runtime limit it walked into. */
#define NIL_OK 0
#define NIL_ERR_TYPE 1
#define NIL_ERR_DIV_ZERO 2
#define NIL_ERR_INDEX 3
#define NIL_ERR_KEY 4
#define NIL_ERR_OOM 5
#define NIL_ERR_INTERNAL 6

/* ── Errors ───────────────────────────────────────────────────────────── */

/* The last failure's code, and its message.
 *
 * One slot rather than one per call: a type failure is reported and the caller
 * carries on, so there is no call stack to unwind and nothing to keep per frame.
 * The program that reads this is one that is about to stop anyway. */
int nil_last_error_code(void);
const char *nil_last_error(void);
void nil_clear_error(void);

/* Records a failure and stops the program.
 *
 * For everything a type error is not: an allocation that failed, a position that
 * does not exist, a value asked to be something it cannot be. A program that has
 * asked for something impossible and received nil will carry on producing wrong
 * answers, so this does not return. */
void nil_panic(int code, const char *message);

/* ── Construction and inspection ──────────────────────────────────────── */

/* Integers are inline while they fit: 61 bits of payload, so every integer a
 * program is likely to hold never touches the heap. Past that it is boxed. */
NilValue nil_int_new(int64_t value);
int64_t nil_int_value(NilValue value);
NilValue nil_float_new(double value);
double nil_float_value(NilValue value);
NilValue nil_bool_new(int64_t truthy);

/* Truthiness as an `int64_t`, and deliberately not an `int`.
 *
 * A raw `1` is not `true` — the tag of `true` is 2 — so this returns a *question's*
 * answer and the caller builds the Boolean. Returning `int` would let a caller
 * pass the answer straight on as a value, which is a wrong answer rather than a
 * compile error. */
int64_t nil_truthy(NilValue value);
NilValue nil_bool_not(NilValue value);

/* Strings.
 *
 * `nil_string_lit` pins its bytes for the rest of the run; `nil_string_new` does
 * not. That difference is the whole reason there are two: generated code holds
 * literals in constants that are never freed, while a string built at runtime by
 * concatenation or conversion would leak without bound if it were pinned. */
NilValue nil_string_lit(const char *bytes, size_t length);
NilValue nil_string_new(const char *bytes, size_t length);
int64_t nil_string_len(NilValue value);
int64_t nil_string_index(NilValue value, int64_t index);
NilValue nil_string_concat(NilValue left, NilValue right);

/* String operations.
 *
 * `substr` moves in *characters* while the representation stores bytes; `find`
 * reports a character index for the same reason, so a program indexing with one
 * and searching with the other gets answers that agree. */
NilValue nil_substr(NilValue s, int64_t start, int64_t length);

/* `substr(s, start)` — the two-argument form, to the end of the string.
 *
 * Separate rather than a sentinel length, so that no caller has to write a magic
 * number into generated code to mean "as far as it goes". */
NilValue nil_substr_from(NilValue s, int64_t start);
int64_t nil_string_find(NilValue haystack, NilValue needle);
NilValue nil_string_to_upper(NilValue s);
NilValue nil_string_to_lower(NilValue s);
NilValue nil_string_trim(NilValue s);
NilValue nil_char_from_int(int64_t code);

/* `str(x)` — the rendered text of any value.
 *
 * A string is returned as itself. Anything else is rendered into a buffer that
 * grows until it fits, because `str` is how a program asks what something is and
 * an answer cut to 128 bytes does not answer. */
NilValue nil_to_str(NilValue value);

/* `isNull(x)` — a real Boolean, because 1 is the tag of false in this
 * representation and a raw 1 here would read as the opposite of the answer. */
NilValue nil_is_null(NilValue value);

/* ── Collections ──────────────────────────────────────────────────────── */

/* Lists.
 *
 * The primitives. A read past the end is an error here because a caller that asks
 * for element 10 of a three-element list has asked for something impossible; the
 * *language's* subscript operator checks first and answers nil, and is
 * `nil_index_get` below. Both answers are right; they answer different questions.
 */
NilValue nil_array_new(int64_t length);
NilValue nil_array_get(NilValue array, NilValue index);
NilValue nil_array_set(NilValue array, NilValue index, NilValue value);
int64_t nil_array_len(NilValue array);
void nil_array_push(NilValue array, NilValue value);

/* `xs.first` and `xs.last`.
 *
 * Nil on an empty list, not an error: asking for the first element of nothing has
 * no answer, and a program that walks a list should not have to check its length
 * first to find out whether there was anything there. `xs[0]` on an empty list *is*
 * an error, because that one is a mistake rather than a question. */
NilValue nil_first(NilValue list);
NilValue nil_last(NilValue list);

/* `rest(xs)` — everything after the first element, as a *new* list.
 *
 * Nil on an empty list, for the same reason `first` and `last` answer nil: there
 * is no answer, and nil is what "no answer" means. New rather than shared,
 * because `rest(xs)[0] = 9` must not be a way to change `xs`. */
NilValue nil_rest(NilValue list);

NilValue nil_map_new(int64_t capacity);
NilValue nil_map_get(NilValue map, NilValue key);
NilValue nil_map_set(NilValue map, NilValue key, NilValue value);
int64_t nil_map_len(NilValue map);

/* `container[key]`, whatever the container is.
 *
 * The runtime dispatches rather than the compiler choosing: at MIR level `m["k"]`
 * and `xs[1]` are one instruction, so a compiler that picked would be guessing.
 * Same reasoning as arithmetic being a runtime call (ADR-0037, ADR-0043).
 *
 * The bound on a list read is nil rather than an error, and the write's is an
 * error rather than nil, on purpose. A read is a question — "what is at position
 * 10?" — and for a three-element list the answer is "nothing there", which is
 * what nil means; it is also what a missing key answers, so a lookup that may miss
 * needs no check either way. A write past the end cannot be carried out at all,
 * and growing the list silently would change something the program can also read,
 * so it stops. */
NilValue nil_index_get(NilValue container, NilValue key);
NilValue nil_index_set(NilValue container, NilValue key, NilValue value);

/* `object.name`, which is a map lookup.
 *
 * Not the same call as `nil_index_get`, because a member that is not there is a
 * mistake while a subscript past the end is a question — and answering nil for
 * both would make the two indistinguishable. */
NilValue nil_get_field(NilValue object, NilValue name);

/* ── Functions ────────────────────────────────────────────────────────── */

NilValue nil_closure_new(NilValue (*function)(NilValue *args, int64_t count));
NilValue nil_call(NilValue callee, NilValue args);

/* ── Arithmetic ───────────────────────────────────────────────────────── */

/* Every operator is a call.
 *
 * The compiler emits `add i64` for two integers and knows the answer. It calls
 * here anyway, because a value reaching an operator has not necessarily been
 * checked by anything, and `1 + "a"` has to be a type error rather than an
 * address. Checking here means one definition of what `+` means, shared by every
 * engine. See ADR-0037.
 *
 * Integer division and remainder truncate toward zero, matching C and the
 * interpreter, and not floor. */
NilValue nil_int_add(NilValue a, NilValue b);
NilValue nil_int_sub(NilValue a, NilValue b);
NilValue nil_int_mul(NilValue a, NilValue b);
NilValue nil_int_div(NilValue a, NilValue b);
NilValue nil_int_mod(NilValue a, NilValue b);
NilValue nil_float_add(NilValue a, NilValue b);
NilValue nil_float_sub(NilValue a, NilValue b);
NilValue nil_float_mul(NilValue a, NilValue b);
NilValue nil_float_div(NilValue a, NilValue b);
NilValue nil_min_int(NilValue a, NilValue b);
NilValue nil_max_int(NilValue a, NilValue b);
NilValue nil_min_float(NilValue a, NilValue b);
NilValue nil_max_float(NilValue a, NilValue b);
NilValue nil_sqrt(NilValue value);
NilValue nil_abs(NilValue value);
NilValue nil_floor(NilValue value);
NilValue nil_ceil(NilValue value);
NilValue nil_round(NilValue value);
NilValue nil_pow(NilValue base, NilValue exponent);

/* ── Comparison ───────────────────────────────────────────────────────── */

/* `op` is an index into the ordering: 0 `==`, 1 `!=`, 2 `<`, 3 `<=`, 4 `>`, 5 `>=`.
 *
 * Every comparison answers a real Boolean and every one of them returns `int64_t`
 * rather than `int`. That is not decoration: a raw `1` is not `true` in this
 * representation, and a function declared to return `int` invites a caller to use
 * its answer as a value, which is silently the wrong value.
 *
 * The rules, which are the interpreter's and not this runtime's invention:
 *
 *   - two numbers compare as numbers, with a float on either side promoting both;
 *   - two strings compare as text;
 *   - a string beside anything else compares the *rendered* text of both, which is
 *     why `"1" == 1` is true;
 *   - two booleans answer equality and only equality;
 *   - null answers equality against anything, true for null and false for a
 *     number, because every lookup in the language can answer null and `x == null`
 *     is how a program asks whether one found anything;
 *   - anything else is a type error.
 */
int64_t nil_int_cmp(NilValue a, NilValue b, int64_t op);
int64_t nil_str_cmp(NilValue a, NilValue b, int64_t op);

/* `&&` and `||` as one operation, with `op` 0 for `&&` and 1 for `||`.
 *
 * Both operands are evaluated. Short-circuiting is a property of the compiler, not
 * of the runtime call, and a language whose `&&` does not short-circuit has a
 * different meaning for every expression containing one. */
int64_t nil_logic(NilValue left, NilValue right, int64_t op);

/* ── Output ───────────────────────────────────────────────────────────── */

/* Renders a value into `buffer`, truncating at `capacity`, and returns the number
 * of bytes written.
 *
 * Returns what it *wrote*, not what it needed, so a value that exactly fills the
 * buffer is indistinguishable from one that was cut short. Callers that need the
 * whole thing render into something that grows — see `nil_to_str`. */
int64_t nil_render(NilValue value, char *buffer, int64_t capacity);
void nil_print(NilValue value);
void nil_println(NilValue value);

/* ── Building strings ────────────────────────────────────────────────── */

/* A builder accumulates bytes without knowing the length first, which is what
 * string interpolation is: the result's length is a function of the values, and
 * none of them is known before the others are rendered.
 *
 * `interp_*` rather than one `interp(value)`, because the four cases are not the
 * same code with a different format — a string is copied, a number is rendered,
 * and nil is three characters that no number would produce. */
NilBuilder *nil_builder_new(void);
NilValue nil_builder_finish(NilBuilder *builder);
void nil_builder_free(NilBuilder *builder);
void nil_interp_int(NilBuilder *builder, NilValue value);
void nil_interp_float(NilBuilder *builder, NilValue value);
void nil_interp_str(NilBuilder *builder, NilValue value);
void nil_interp_bool(NilBuilder *builder, NilValue value);
void nil_interp_nil(NilBuilder *builder);

/* ── Time ─────────────────────────────────────────────────────────────── */

int64_t nil_time_now_millis(void);
int64_t nil_time_sleep_millis(int64_t millis);

/* ── Allocation and collection ────────────────────────────────────────── */

/* Raw allocation. Always zeroed — the `zeroed` argument exists for a future that
 * needs it not to be, and a call that passes 0 expecting uninitialised memory
 * would be reading a promise this function does not make. */
NilValue nil_alloc(int64_t size, int64_t zeroed);

/* Explicit roots.
 *
 * A root is a *pointer to a slot*, not a value, because a root is a location the
 * collector reads: registering the value instead would make a reassignment of the
 * local invisible to the collector, which is the common case in a loop. */
void nil_gc_root_register(NilValue *slot);
void nil_gc_root_unregister(NilValue *slot);

/* The collector runs on allocation once the live count passes a threshold. These
 * make that policy visible to a program that needs to: a long loop can lower the
 * threshold, and a test can disable collection entirely to make a leak obvious
 * rather than hidden by a collection that reclaims it. */
void nil_gc_collect(void);
void nil_gc_set_threshold(int64_t objects);
void nil_gc_disable(void);
void nil_gc_enable(void);
int64_t nil_gc_object_count(void);

#ifdef __cplusplus
}
#endif

#endif /* NIL_H */
