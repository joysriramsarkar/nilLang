/* ffi.c — the conversions at the boundary between Nilang and C.
 *
 * # Why these are here and not in the language
 *
 * A tagged word and a C integer are two conventions for the same 64 bits. Which
 * convention a value is in is a fact about where it came from, so the conversion
 * has to happen where that fact is known — on the edge — and nowhere else.
 *
 * Putting it in the compiler instead would mean two implementations of it, one per
 * back end, and a program whose answer depends on which back end built it.
 *
 * # Why they are this simple
 *
 * They are simple because everything hard has already happened. An inline integer
 * is a shift; a boxed one is a load; a float is a memcpy. The difficult part of
 * an FFI is not the conversion and it is worth being plain about that: it is
 * deciding what crosses, and pinning that down so a change to it is visible rather
 * than silent. That decision is in `nilffi.h`.
 */

#include "nilffi.h"

/* The collector's internals, because a conversion has to be able to tell what a
 * tagged word is holding.
 *
 * This file is part of libnilcore rather than something generated code includes,
 * so it may use `nil_internal.h`. Generated code includes `nilffi.h` alone and
 * gets only the conversions — which is the right split, because reading a value's
 * representation is what a conversion does and nothing else should. */
#include "nil_internal.h"

#include <stdlib.h>
#include <string.h>

/* ── Version ──────────────────────────────────────────────────────────── */

/* Empty, and it stays empty.
 *
 * This symbol exists to be *resolved*, not to be called: a v2 object against a
 * runtime that no longer exports it fails at link, naming the symbol, before
 * anything runs. A body would be somewhere for the two halves to disagree.
 *
 * The name carries the version, and the version is `NIL_FFI_ABI_VERSION` — so this
 * name and that number have to be changed together. `nilffi.h` says which version
 * added what, and `TestTheVersionInTheGuardIsTheVersionInTheHeader` fails if the two
 * drift apart. It did once, and the test said so by printing both numbers. */
void nil_ffi_abi_v2(void) {}

int64_t nil_ffi_version(void) { return (int64_t)NIL_FFI_ABI_VERSION; }

/* ── Layout ───────────────────────────────────────────────────────────── */

/* One table for both questions.
 *
 * A struct rather than a `switch` in each function, because the failure mode of
 * having two is exactly the failure this is about: `sizeof` says 32 and `alignof`
 * says 7, and the program that trusts both allocates a string that is the right
 * size and the wrong alignment — a fault that appears on whatever code happened to
 * be unaligned and on nothing else.
 *
 * The numbers are `sizeof` and `_Alignof` of the runtime's own types, taken by the
 * same compiler that built this file. They are not written down here because a
 * written-down number is a second source of truth for something the compiler knows
 * exactly.
 */
typedef struct NilFFILayoutEntry {
  NilFFILayout which;
  int64_t size;
  int64_t alignment;
} NilFFILayoutEntry;

static const NilFFILayoutEntry nil_ffi_layout[] = {
    {NIL_FFI_LAYOUT_VALUE, (int64_t)sizeof(NilValue), (int64_t)_Alignof(NilValue)},
    {NIL_FFI_LAYOUT_OBJECT, (int64_t)sizeof(NilObject), (int64_t)_Alignof(NilObject)},
    {NIL_FFI_LAYOUT_STRING, (int64_t)sizeof(NilString), (int64_t)_Alignof(NilString)},
    {NIL_FFI_LAYOUT_CLOSURE, (int64_t)sizeof(NilClosure), (int64_t)_Alignof(NilClosure)},
    {NIL_FFI_LAYOUT_CALLBACK, (int64_t)sizeof(NilFFICallback), (int64_t)_Alignof(NilFFICallback)},
    {NIL_FFI_LAYOUT_CROSSING, (int64_t)sizeof(NilFFIString), (int64_t)_Alignof(NilFFIString)},
};

static const NilFFILayoutEntry *layout_entry(NilFFILayout what) {
  for (size_t i = 0; i < sizeof(nil_ffi_layout) / sizeof(nil_ffi_layout[0]); i++) {
    if (nil_ffi_layout[i].which == what) {
      return &nil_ffi_layout[i];
    }
  }
  return NULL;
}

int64_t nil_ffi_sizeof(NilFFILayout what) {
  const NilFFILayoutEntry *entry = layout_entry(what);
  /* Refused rather than answered. A default would be a guess about a number a
   * program is about to allocate with, and the guess is what a caller that did not
   * check would have made anyway. */
  return entry == NULL ? -1 : entry->size;
}

int64_t nil_ffi_alignof(NilFFILayout what) {
  const NilFFILayoutEntry *entry = layout_entry(what);
  return entry == NULL ? -1 : entry->alignment;
}

int64_t nil_int_unbox(NilValue value) {
  /* The same two cases `nil_int_value` handles, reached through the ABI's own
   * name. It is spelled out rather than delegated so that reading this file
   * answers "what happens to a value at the boundary" without a second lookup —
   * and so that a change to the language's internal representation does not
   * silently change the FFI's. */
  if (nil_is_inline_int(value)) {
    return (int64_t)(value & ~((NilValue)NIL_TAG_MASK));
  }
  if (nil_is_int_box(value)) {
    return ((NilBox *)nil_object_header(value))->value;
  }
  return 0;
}

NilValue nil_int_box(int64_t value) {
  return nil_int_new(value);
}

double nil_float_unbox(NilValue value) {
  double result = 0;
  if (nil_is_float(value)) {
    memcpy(&result, &((NilBox *)nil_object_header(value))->value, sizeof(double));
    return result;
  }
  /* An integer crossing as a double is promoted rather than refused. The C side
   * asked for a `double` and a program that passed an integer is asking for the
   * number, not for a type error at the boundary — and 2^53 is the largest integer
   * a double holds exactly, which is more than the language's own Float prints
   * distinctly anyway. */
  return (double)nil_int_unbox(value);
}

NilValue nil_float_box(double value) {
  return nil_float_new(value);
}

int nil_bool_unbox(NilValue value) {
  return nil_truthy(value) != 0 ? 1 : 0;
}

NilValue nil_bool_box(int value) {
  return nil_bool_new(value);
}

uint8_t nil_byte_unbox(NilValue value) {
  return (uint8_t)nil_int_unbox(value);
}

NilValue nil_byte_box(uint8_t value) {
  return nil_int_new((int64_t)value);
}

/* ── Strings ──────────────────────────────────────────────────────────── */

void nil_string_unbox(NilValue value, NilFFIString *out) {
  NilString *string = (NilString *)nil_object_header(value);

  if (out == NULL) {
    return;
  }

  if (nil_is_string(value) && string != NULL) {
    out->bytes = string->data;
    out->length = string->header.length;
    return;
  }

  /* An empty string crosses as a null pointer and a zero length, which is what
   * every C API that takes a length handles and what `memcpy` is defined to do
   * with. Reporting a non-null pointer to nothing would be worse: a C function
   * that checks for null before reading would treat it as a real string and then
   * read address zero. */
  out->bytes = NULL;
  out->length = 0;
}

NilValue nil_string_copy(const char *bytes, size_t length) {
  if (bytes == NULL) {
    /* A null pointer with no length is the empty string — the other half of what
     * `nil_string_unbox` produces for one. A null pointer *with* a length is a
     * caller who has lost track of the buffer, and handing it to `strlen` would be
     * worse than saying so. */
    if (length == 0) {
      return nil_string_lit("", 0);
    }
    nil_set_error(NIL_ERR_TYPE, "a string of non-zero length was given a null pointer");
    return NIL_TAG_NIL;
  }
  return nil_string_new(bytes, length);
}

/* ── Buffers ──────────────────────────────────────────────────────────── */

/* The outstanding buffers, as a stack with a mark on each open scope.
 *
 * # Why a stack and not a free list
 *
 * Because the scopes nest, and only a stack survives that. A C function can call a
 * callback, that callback can be a whole Nilang program, and that program can call
 * another string-returning C function — so an inner scope closes while the outer
 * one is still holding a buffer it is about to read.
 *
 * A free list has no way to tell those apart: the inner close would take the
 * outer's buffer, and the outer function would return a pointer into freed memory
 * having done everything correctly. The mark is what makes `close` mean "free what
 * was allocated since *my* open", and a mark only means anything on a stack.
 *
 * # Why the blocks are individually allocated rather than bump-allocated out of one
 *
 * Because a single block would have to be grown, and growing copies the bytes that
 * a caller is holding a pointer into. A function that asks for ten bytes and then
 * asks for ten more must keep the first ten valid, or the caller's `realloc`-shaped
 * code becomes a use-after-free that depends on the allocator. One block per
 * request is what makes that impossible. */
/* # Why the scope owns the crossing struct rather than the caller allocating one
 *
 * Because the caller cannot allocate one safely. An `alloca` written at a call site
 * lands in whatever block that call is in, so a call inside a loop body allocated
 * once per iteration and the stack grew for as long as the loop ran. On Windows the
 * linker noticed — clang emitted a `__chkstk` stack probe against an MSVC triple and
 * the mingw linker had no such symbol — which is the *lucky* outcome: elsewhere it
 * is a stack overflow at whatever iteration exhausts the stack, on the one input
 * large enough to reach it.
 *
 * Hoisting the `alloca` into the entry block would fix the growth and split the
 * lifetime in two: the buffers belong to the scope, the struct belongs to the frame,
 * and something has to reconcile them. One scope owning both is one lifetime, and
 * one thing to get wrong rather than two. */
typedef struct NilFFIScope {
  struct NilFFIScope *previous;
  size_t mark;
} NilFFIScope;

typedef struct NilFFIBlock {
  struct NilFFIBlock *next;
  char *bytes;
  size_t length;
} NilFFIBlock;

/* The struct one open scope crosses through.
 *
 * Held here rather than returned by value because a returned struct's meaning
 * depends on which compiler produced the other half — see the note in `nilffi.h`
 * about clang and GCC disagreeing on a 16-byte struct return. */
static NilFFIString scope_answer = {NULL, 0};

static NilFFIBlock *blocks = NULL;
static size_t block_count = 0;
static NilFFIScope *scopes = NULL;

NilFFIString *nil_ffi_scope_open(void) {
  NilFFIScope *scope = (NilFFIScope *)malloc(sizeof(NilFFIScope));
  if (scope == NULL) {
    nil_set_error(NIL_ERR_OOM, "out of memory opening an FFI buffer scope");
    return &scope_answer;
  }
  scope->previous = scopes;
  scope->mark = block_count;
  scopes = scope;

  /* Zeroed on open rather than left holding the last call's answer. A callee that
   * returns without touching the struct would otherwise hand back the *previous*
   * call's bytes, which is a buffer this scope is about to free. */
  scope_answer.bytes = NULL;
  scope_answer.length = 0;
  return &scope_answer;
}

char *nil_ffi_buffer(int64_t length) {
  /* Refused rather than clamped. A negative length is a caller that has lost track
   * of its own arithmetic, and a clamped answer would be a short buffer that looks
   * correctly sized — the caller writes past the end and finds out much later. */
  if (length < 0) {
    nil_set_error(NIL_ERR_INTERNAL, "a negative buffer length was asked for");
    return NULL;
  }

  NilFFIBlock *block = (NilFFIBlock *)malloc(sizeof(NilFFIBlock));
  if (block == NULL) {
    nil_set_error(NIL_ERR_OOM, "out of memory allocating an FFI buffer");
    return NULL;
  }

  /* One byte over, always, so a terminator fits and a caller using the buffer with
   * `printf` does not have to ask for room it might not get. The recorded length
   * never counts it — a NUL is not part of the string. */
  block->length = (size_t)length;
  block->bytes = (char *)malloc((size_t)length + 1);
  if (block->bytes == NULL) {
    free(block);
    nil_set_error(NIL_ERR_OOM, "out of memory allocating an FFI buffer");
    return NULL;
  }
  block->bytes[length] = '\0';

  block->next = blocks;
  blocks = block;
  block_count++;
  return block->bytes;
}

void nil_ffi_scope_close(void) {
  if (scopes == NULL) {
    /* No scope to close. Freeing everything here would take the buffers of whatever
     * call is further out on the stack, so this is deliberately a no-op rather than
     * a best guess. Generated code opens and closes in pairs; a call with no open
     * scope is a bug in the caller, not something to guess about. */
    return;
  }

  NilFFIScope *scope = scopes;
  scopes = scope->previous;

  while (block_count > scope->mark) {
    NilFFIBlock *block = blocks;
    if (block == NULL) {
      break;
    }
    blocks = block->next;
    free(block->bytes);
    free(block);
    block_count--;
  }
  free(scope);

  /* Cleared so a stale pointer into the previous answer reads as empty rather than
   * as the bytes of a call that has finished. The struct itself is static and is
   * reused by the next scope; the *contents* are what must not survive it. */
  scope_answer.bytes = NULL;
  scope_answer.length = 0;
}

int64_t nil_ffi_buffer_outstanding(void) { return (int64_t)block_count; }

/* nil_ffi_scope_alloc allocates inside the innermost open scope and hands back the
 * pointer *and* the length.
 *
 * # Why it returns the length rather than only the pointer
 *
 * Because `nil_ffi_buffer` is for a caller that already knows how many bytes it wants,
 * and every caller of it writes exactly as many as it asked for. The other kind of
 * answer — a callee that sizes the answer and says how big it turned out to be — has
 * no way to say that through a bare `char *`, and a caller that guesses is a caller
 * that eventually over-reads a buffer.
 *
 * # Why it is here and not in `channel.c`
 *
 * Because the scope owns every buffer allocated during a call, and a second
 * allocator would be a second lifetime. A channel's answer is allocated during a call,
 * so it is freed by that call's `nil_ffi_scope_close`, exactly like a string answer is.
 *
 * Internal rather than public: generated code has `nil_ffi_buffer` and no use for
 * this, so declaring it in `nilffi.h` would widen the boundary for a question only
 * this library asks. See `nil_internal.h`.
 */
char *nil_ffi_scope_alloc(int64_t length, size_t *written) {
  char *bytes = nil_ffi_buffer(length);
  if (bytes == NULL) {
    return NULL;
  }
  /* The block's own record, not the argument: a negative length is refused above, and
   * a caller reading this back should never see a value the allocator did not
   * allocate. */
  if (written != NULL) {
    *written = (size_t)length;
  }
  return bytes;
}

/* ── Callbacks ──────────────────────────────────────────────────────────── */

NilValue nil_callback_new(NilFFICallback function, int64_t arity) {
  if (function == NULL || arity <= 0) {
    /* Refused rather than accepted. The function pointer says nothing about its own
     * signature, so this number is the only record of how many arguments the
     * callback takes — and a callback that answers the question with a guess reads
     * past the end of an array when it is called. */
    nil_set_error(NIL_ERR_INTERNAL, "a callback needs a function and a positive argument count");
    return NIL_TAG_NIL;
  }
  NilValue callback = nil_closure_new(function);
  if (callback == NIL_TAG_NIL) {
    return callback;
  }

  ((NilClosure *)nil_object_header(callback))->arity = arity;

  /* Pinned permanently, not for the duration of this call.
   *
   * The other pin in this file is scoped to one operation and cleared on the way
   * out. A callback is the other kind: C holds this pointer for as long as it
   * intends to call it, which is not bounded by anything the FFI can see, and a
   * collection in between would free a function that is about to be called. */
  nil_gc_pin_string(callback);
  return callback;
}

NilValue nil_callback_invoke(NilValue callback, NilValue *args, int64_t count) {
  if (nil_object_kind(callback) != NIL_OBJ_CLOSURE) {
    return NIL_TAG_NIL;
  }

  NilClosure *closure = (NilClosure *)nil_object_header(callback);
  if (closure->function == NULL) {
    return NIL_TAG_NIL;
  }

  /* Checked here rather than trusted, because the argument count is the one thing
   * the two sides can get wrong and the consequence of getting it wrong is reading
   * past the end of a buffer C allocated. */
  if (count != closure->arity) {
    nil_set_error(NIL_ERR_INTERNAL, "callback called with the wrong number of arguments");
    return NIL_TAG_NIL;
  }

  /* The callback and its arguments are pinned across the call. The callback can
   * allocate — it is a whole Nilang program — and a collection in the middle of it
   * would be free to free the arguments it is reading. */
  nil_gc_pin(callback);
  if (args != NULL) {
    for (int64_t i = 0; i < count; i++) {
      nil_gc_pin(args[i]);
    }
  }
  NilValue result = closure->function(args, count);
  nil_gc_unpin_all();
  return result;
}

NilValue nil_callback_trampoline(NilValue self, NilValue *args, int64_t count) {
  return nil_callback_invoke(self, args, count);
}

int64_t nil_callback_arity(NilValue callback) {
  if (nil_object_kind(callback) != NIL_OBJ_CLOSURE) {
    return -1;
  }
  return ((NilClosure *)nil_object_header(callback))->arity;
}

void nil_callback_release(NilValue callback) {
  if (nil_object_kind(callback) != NIL_OBJ_CLOSURE) {
    return;
  }
  /* Unpinned rather than freed. The collector owns the memory, and freeing an
   * object the chain still points at would leave the next collection following a
   * pointer to freed memory — which is a use-after-free that happens later and
   * names nothing.
   *
   * The object is unreachable-but-not-yet after this, and the next collection
   * reclaims it. That is the collector's whole job, and doing it here would be a
   * second implementation of "what is reachable". */
  nil_gc_unpin(callback);
}

int nil_ffi_can_cross(NilValue value) {
  switch (value & NIL_TAG_MASK) {
  case NIL_TAG_NIL:
  case NIL_TAG_FALSE:
  case NIL_TAG_TRUE:
  case NIL_TAG_INT:
    return 1;
  case NIL_TAG_PTR:
    switch (nil_object_kind(value)) {
    case NIL_OBJ_STRING:
    case NIL_OBJ_INTBOX:
    case NIL_OBJ_FLOATBOX:
      return 1;
    default:
      return 0;
    }
  default:
    return 0;
  }
}
