/* callbackfixture.c — C that stores a Nilang callback and calls it back.
 *
 * # Why this file exists
 *
 * A callback's every part is invisible until C actually invokes it. The declaration
 * compiles, the closure is built, the pointer is stored — and the program is correct
 * right up to the moment C decides to use it.
 *
 * So this file is the far side of that: it takes a callback, keeps it, and calls it
 * long after the call that delivered it returned. A pin scoped to the registration
 * would be gone by then, and the program would read freed memory on a call the
 * person did not choose.
 *
 * # Why it stores the callback rather than calling it once
 *
 * Because storing it is what C actually does. `qsort` keeps a comparator for the
 * length of a sort; a platform channel keeps a handler until the app closes. A
 * fixture that called its callback immediately would test the easy case and miss
 * the one that breaks.
 */

#include "nilffi.h"

/* The callback C was given, and the arity it reported.
 *
 * Static rather than a parameter because C has nowhere to put it that outlives the
 * call — which is precisely the point being tested. */
static NilValue held_callback = NIL_TAG_NIL;
static int64_t held_arity = -1;

/* A second callback slot, for a callback of a different arity.
 *
 * Two rather than one so a program can register handlers of different shapes and
 * ask each what it reported — which is how the arity is shown to come from the
 * function rather than from anywhere else. */
static NilValue held_pair_callback = NIL_TAG_NIL;
static int64_t held_pair_arity = -1;

/* How many times the callback has been called.
 *
 * A counter rather than a flag, so a test can tell "called once" from "called
 * again", which are different failures. */
static int64_t call_count = 0;

void nilffi_register(NilValue callback) {
  held_callback = callback;
  held_arity = nil_callback_arity(callback);
}

void nilffi_register_pair(NilValue callback) {
  held_pair_callback = callback;
  held_pair_arity = nil_callback_arity(callback);
}

/* Calls the pair callback with two arguments. */
int64_t nilffi_fire_pair(int64_t first, int64_t second) {
  NilValue a = nil_int_box(first);
  NilValue b = nil_int_box(second);
  NilValue args[2] = {a, b};
  return nil_int_unbox(nil_callback_invoke(held_pair_callback, args, 2));
}

/* Calls the held callback with one argument, and returns what it answered.
 *
 * # Why both edges convert
 *
 * The rule is that every value crossing the boundary is converted on the way out
 * and converted back on the way in — and a callback invocation is *inside* Nilang,
 * so neither edge of it is a boundary at all.
 *
 * That leaves this function with two real conversions and no ambiguity:
 *
 *   - the argument arrives from Nilang as a plain `int64_t` and is **boxed** before
 *     the callback sees it. Passing it straight through would hand a Nilang
 *     function the number 21 rather than the *tagged value* for 21 — two different
 *     words, and the function would read the low three bits of a number nobody
 *     tagged;
 *   - the callback's answer is already a Nilang value, because that is what a
 *     Nilang function produces, and returning it as an `int64_t` means **unboxing**
 *     it. Returning it unchanged would have the caller's `convertIn` box it a
 *     second time, and a value that is boxed twice is a number nobody wrote.
 *
 * So a C function that calls back into Nilang and returns the result converts at
 * both of its own edges, and the callback's body never learns a boundary exists.
 */
int64_t nilffi_fire(int64_t argument) {
  call_count++;
  NilValue boxed = nil_int_box(argument);
  return nil_int_unbox(nil_callback_invoke(held_callback, &boxed, 1));
}

/* Calls the held callback twice in a row without re-registering.
 *
 * This is the case a pin scoped to one call fails: the first call's pin list has
 * been cleared by the time the second runs, and a collection in between would have
 * been free to take the closure. */
int64_t nilffi_fire_twice(int64_t first, int64_t second) {
  call_count += 2;
  NilValue a = nil_int_box(first);
  NilValue b = nil_int_box(second);
  int64_t ra = nil_int_unbox(nil_callback_invoke(held_callback, &a, 1));
  int64_t rb = nil_int_unbox(nil_callback_invoke(held_callback, &b, 1));
  return ra + rb;
}

/* Calls the callback with the wrong number of arguments.
 *
 * The C side is deliberately wrong here. A callback's arity is the only record of
 * how many arguments it takes — the function pointer says nothing about its own
 * signature — so this checks that the arity is *enforced*, and that saying so is a
 * message rather than a read past the end of a one-element array.
 *
 * A refused call answers nil, and `nil_int_unbox(nil)` is 0, so the program sees a
 * 0 rather than a crash. */
int64_t nilffi_fire_wrong_arity(int64_t argument) {
  NilValue boxed = nil_int_box(argument);
  NilValue answer = nil_callback_invoke(held_callback, &boxed, 2);
  return answer == NIL_TAG_NIL ? -1 : nil_int_unbox(answer);
}

/* The arity the callback reported, for a program to print. */
int64_t nilffi_reported_arity(void) {
  return held_arity;
}

/* The arity the second callback reported. */
int64_t nilffi_reported_pair_arity(void) {
  return held_pair_arity;
}

/* How many times the callback has been called. */
int64_t nilffi_call_count(void) {
  return call_count;
}

/* Releases the callback and confirms the release was seen.
 *
 * Returns how many callbacks the runtime still holds, so a program can check that
 * a release actually happened rather than trusting that it did.
 *
 * "Is this a callback" is asked through `nil_callback_arity` rather than through a
 * collector internal. The internal would answer it too — it is the same question —
 * but reaching past `nilffi.h` to ask it would mean this file, which stands in for
 * somebody's C, needed to know how the runtime stores its values. */
int64_t nilffi_release(void) {
  if (nil_callback_arity(held_callback) < 0) {
    return -1;
  }
  int64_t before = nil_gc_object_count();
  nil_callback_release(held_callback);
  int64_t after = nil_gc_object_count();

  /* The object is unreachable-but-not-yet after the release; the next collection
   * reclaims it. Comparing counts here is what distinguishes "the pin is gone" from
   * "the object is gone", and they are not the same moment. */
  (void)before;
  return after <= before ? 1 : 0;
}

/* Releases twice, which is a caller's mistake rather than a fatal one. */
int64_t nilffi_release_twice(void) {
  nil_callback_release(held_callback);
  nil_callback_release(held_callback);
  return 1;
}

/* Releases something that is not a callback at all. */
int64_t nilffi_release_not_a_callback(void) {
  nil_callback_release(NIL_TAG_NIL);
  nil_callback_release(nil_int_new(1));
  return 1;
}

/* Builds a collection and throws it away, so a program can be sure a real
 * collection ran while the callback was held.
 *
 * Without this a test that merely calls a callback twice might never have
 * triggered a collection, and a lifetime bug would pass on a program that happened
 * not to allocate. */
int64_t nilffi_churn(int64_t rounds) {
  for (int64_t i = 0; i < rounds; i++) {
    NilValue temporary = nil_array_new(64);
    nil_array_set(temporary, nil_int_new(0), nil_int_new(i));
    nil_array_set(temporary, nil_int_new(1), nil_int_new(i));
  }
  return nil_gc_object_count();
}

/* Hands a value to C without converting it, and keeps it.
 *
 * The shape every callback has: a Nilang value arrives, C wants to treat it as a
 * number. It cannot assume it *is* one — the program may have answered with a
 * string, a list, or nothing at all — and `nil_int_unbox` on a list reads a
 * pointer's low bits as a number.
 *
 * The value is **kept** rather than returned, and that is the point. Returning it
 * would mean crossing it, and the crossing is what the question is being asked
 * about: a value sent back as a `String` arrives in C as an empty string whatever
 * it was, so `nil_ffi_can_cross` would be asked about the empty string and answer
 * yes every time. C only ever holds the original when nothing has converted it, so
 * the fixture does the same.
 *
 * This is also why the callback's argument is boxed before the invocation: passing
 * it through unchanged would hand the function a number rather than a tagged value.
 */
static NilValue held_value = NIL_TAG_NIL;

void nilffi_hand_back(NilValue callback) {
  NilValue boxed = nil_int_box(0);
  held_value = nil_callback_invoke(callback, &boxed, 1);
}

/* Whether the value C is holding crosses at all.
 *
 * The whole of `nil_ffi_can_cross` behind one call, so a program can check what it
 * was handed and print the answer. Returns 1 for something that crosses and 0 for
 * something that does not.
 *
 * A list answers 0: there is no crossing for a list, and handing one to C would mean
 * C reading a pointer's low bits as a number. */
int64_t nilffi_can_cross_held(void) {
  return nil_ffi_can_cross(held_value) ? 1 : 0;
}

/* How many bytes the held value is, when it is read as a string.
 *
 * Asked through `nil_string_unbox` rather than by reaching into the value's
 * representation — `nil_is_string` and `nil_object_header` are the runtime's own, in
 * `nil_internal.h`, which is exactly where a C author must not be looking. Reaching
 * past the boundary here would have been the first thing in this fixture to do so,
 * and it is how a boundary stops meaning anything.
 *
 * The unbox answers `{NULL, 0}` for anything that is not a string, which is the same
 * as an empty string. That is not a gap: both mean "there is nothing here to read",
 * and a caller about to read bytes wants that answer, not a question about types.
 *
 * -1 means the unbox itself could not answer, which for a held value means the
 * unbox was never run. */
int64_t nilffi_held_length(void) {
  NilFFIString out;
  nil_string_unbox(held_value, &out);
  return (int64_t)out.length;
}
