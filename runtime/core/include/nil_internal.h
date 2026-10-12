/* nil_internal.h — declarations shared inside libnilcore, hidden from compiled code.
 *
 * # Why these are not in nil.h
 *
 * Everything here is a collector internal or a helper whose signature exists only
 * so two .c files agree on it. Compiled code never calls any of it, and putting it
 * in the public header would make it part of the interface that generated code is
 * expected to use — which is a promise this library would then have to keep.
 *
 * The mismatch this file prevents is not hypothetical: `nil_destroy` was declared
 * as `void *` in one file and `NilObject *` in another. Both compile, both link,
 * and the bug only shows up on a platform where the two are not the same width.
 */

#ifndef NIL_INTERNAL_H
#define NIL_INTERNAL_H

#include "nil.h"

/* ── Object allocation (gc.c) ─────────────────────────────────────────── */

/* Allocates a zeroed object of `bytes` and links it into the collection chain.
 *
 * Zeroed because the object is linked in before its fields are written, and a
 * collection between those two points would read a `next` that is whatever the
 * allocator returned. */
void *nil_object_new(uint8_t kind, size_t bytes);

NilValue nil_object_pointer(void *object);
NilObject *nil_object_header(NilValue value);
void nil_destroy(NilObject *object);

/* nil_object_kind is the object's kind, or -1 when the value is not an object.
 *
 * # Why every predicate goes through this
 *
 * `nil_object_header` returns NULL for anything that is not a tagged pointer,
 * including a value whose tag says "pointer" but whose bits are all zero — the
 * tagged null. Reading `header->kind` on that is a null dereference, and the
 * value that triggers it is not exotic: it is the bit pattern `NIL_TAG_PTR`, which
 * is also the pattern a small integer would have had if integers used four tag
 * bits instead of three.
 *
 * Returning -1 rather than a kind makes the safe path the short one. A predicate
 * written as `kind(value) == NIL_OBJ_STRING` cannot forget the check, where a
 * predicate written as `header(value)->kind == NIL_OBJ_STRING` can — and it did.
 */
int nil_object_kind(NilValue value);

/* ── Temporary protection (gc.c) ──────────────────────────────────────── */

/* Pins a value across an operation that can allocate.
 *
 * The pin list is cleared by `nil_gc_unpin_all`, so a pin is scoped to the
 * operation and never to a scope the caller has to remember. */
void nil_gc_pin(NilValue value);
void nil_gc_unpin_all(void);
void nil_gc_pin_string(NilValue value);

/* Releases one permanent pin.
 *
 * Permanent pins exist for values that live for the whole run — string literals,
 * and callbacks C holds. A callback has an end, so it has a release; a string
 * literal does not, so it does not need one.
 *
 * Releases the *first* matching pin, because two pins are two promises and one of
 * them being withdrawn is not a reason to withdraw the other. */
void nil_gc_unpin(NilValue value);

/* ── The FFI's buffer scope (runtime/ffi/src/ffi.c) ────────────────────── */

/* Allocates inside the innermost open FFI scope and reports how many bytes it got.
 *
 * Internal rather than public on purpose: generated code has `nil_ffi_buffer`, which
 * answers the question a caller that already knows its size asks, and this answers the
 * one a *callee* asks when the answer's size is only known after it is built. See
 * `nilffi.h` for what a scope is and why a call opens one.
 *
 * Declared here rather than in `nilffi.h` so that the boundary's frozen surface does
 * not grow a function no generated file can use. */
char *nil_ffi_scope_alloc(int64_t length, size_t *written);

/* ── Errors (value.c) ─────────────────────────────────────────────────── */

/* Records a type failure and returns nil, so a caller can `return
 * type_error(...)` from a function whose result is a value. */
NilValue nil_type_error(const char *operation, NilValue left, NilValue right);

/* Records a failure without returning anything.
 *
 * Exported rather than static because the collections have failures of their own —
 * a map whose table is full, an index that is out of range — and a message that
 * is set in one file and read in another has to go through the same buffer the
 * public `nil_last_error` reads. */
void nil_set_error(int code, const char *message);

/* ── Predicates (value.c) ─────────────────────────────────────────────── */

int nil_is_inline_int(NilValue value);
int nil_is_int_box(NilValue value);
int nil_is_float(NilValue value);
int nil_is_string(NilValue value);
int nil_is_number(NilValue value);
int nil_is_bool(NilValue value);

/* `nil_truthy` is declared in nil.h with an `int64_t` return and is deliberately
 * not redeclared here. It is listed because the predicates below are the same
 * question asked a cheaper way, and a reader comparing them should see that both
 * agree about what counts as true. */

const char *nil_type_name(NilValue value);

/* ── Bounds errors (collections.c) ────────────────────────────────────── */

/* Reports an out-of-range index and exits. Shared so an array and a string give
 * the same message shape and the same `NIL_ERR_INDEX` code. */
void nil_bounds_error(const char *kind, int64_t index, int64_t length);

/* How many bytes the character at `bytes` occupies, and where character `index`
 * starts within a string.
 *
 * Exposed because string indexing has to move in *characters* while the
 * representation stores *bytes*, and a second implementation of that conversion
 * would be a second set of answers about where a character is. The runtime's own
 * tests check the conversion; a copy in the collections file would not be covered
 * by them. See ADR-0044.
 */
size_t utf8_sequence_length(const char *bytes, size_t remaining);
size_t nil_string_byte_offset(NilString *string, int64_t index);

#endif /* NIL_INTERNAL_H */