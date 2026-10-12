/* value.c — construction, introspection and arithmetic.
 *
 * # Why arithmetic checks its operand types
 *
 * The compiler emits `add i64` for two integers and knows the answer. It still
 * calls here instead, because a value reaching an operator has not necessarily
 * been checked by anything — and `1 + "a"` has to be a type error rather than an
 * address. Checking here means one definition of what `+` means, shared with the
 * interpreter and the VM.
 *
 * See ADR-0037.
 *
 * # Why integers are inline
 *
 * The representation has three spare bits, so 61 bits of payload are available and
 * every integer a program will hold fits without touching the heap. That is why
 * an `i64` can hold a Nilang value at all, and why the common arithmetic case
 * allocates nothing.
 *
 * # The boundary
 *
 * An integer whose low three bits are already 0b011 cannot be tagged as an integer
 * — tagging it would be a no-op that reads back as the wrong thing. Those are
 * boxed instead. The boundary is a property of the representation, not a limit the
 * language imposes, which is why it is not something the compiler has to know.
 */

#if !defined(_WIN32) && !defined(_POSIX_C_SOURCE)
/* `_POSIX_C_SOURCE` before any system header, so `nanosleep` is declared when
 * compiling strictly: with `-std=c11` and no feature macro, glibc's <time.h>
 * hides it and modern compilers refuse the implicit declaration. Windows
 * needs none of this — it sleeps with `Sleep` from <windows.h> below. */
#define _POSIX_C_SOURCE 200809L
#endif

#include "nil.h"
#include "nil_internal.h"

#include <math.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#include <time.h>

#if defined(_WIN32)
#include <windows.h>
#endif

/* ── Errors ───────────────────────────────────────────────────────────── */

static char last_error[512] = {0};
static int last_error_code = NIL_OK;

const char *nil_last_error(void) { return last_error; }
int nil_last_error_code(void) { return last_error_code; }

void nil_clear_error(void) {
  last_error[0] = '\0';
  last_error_code = NIL_OK;
}

/* nil_set_error records a failure in the runtime's single error slot.
 *
 * One slot rather than per-thread state: a compiled program that has several
 * threads will need more, and this is where that change lands. Every failure in
 * the library goes through here so the buffer, the code and the message are one
 * thing rather than three.
 */
void nil_set_error(int code, const char *message) {
  last_error_code = code;
  snprintf(last_error, sizeof(last_error), "%s", message);
}

#define set_error nil_set_error

void nil_panic(int code, const char *message) {
  nil_set_error(code, message);
  fprintf(stderr, "nilang: %s\n", message);
  exit(code);
}

const char *nil_type_name(NilValue value) {
  switch (value & NIL_TAG_MASK) {
  case NIL_TAG_NIL:
    return "null";
  case NIL_TAG_FALSE:
    return "bool";
  case NIL_TAG_TRUE:
    return "bool";
  case NIL_TAG_INT:
    return "integer";
  case NIL_TAG_PTR:
    switch (nil_object_kind(value)) {
    case NIL_OBJ_STRING:
      return "string";
    case NIL_OBJ_ARRAY:
      return "array";
    case NIL_OBJ_MAP:
      return "map";
    case NIL_OBJ_INTBOX:
      return "integer";
    case NIL_OBJ_FLOATBOX:
      return "float";
    case NIL_OBJ_CLOSURE:
      return "function";
    /* A pointer tag with no address is a null pointer. Calling it "object" would
     * make an error message say "cannot add: object and integer" about a value
     * that is, in every other respect, nil. */
    case -1:
      return "null";
    }
    return "object";
  }
  return "unknown";
}

/* nil_type_error is the one message every type failure is written with.
 *
 * One function rather than one message per site, so the wording does not drift and
 * the fix is the same for a reader every time. The names come from the value, not
 * from the source, because a program that reached this point has already lost its
 * source-level names. */
NilValue nil_type_error(const char *operation, NilValue left, NilValue right) {
  char message[256];
  snprintf(message, sizeof(message), "cannot %s: %s and %s", operation, nil_type_name(left),
           nil_type_name(right));
  set_error(NIL_ERR_TYPE, message);
  return NIL_TAG_NIL;
}

/* ── Predicates ───────────────────────────────────────────────────────── */

/* These read the tag rather than the object, except where the object is needed —
 * which is the whole justification for putting the kind in the object at all: an
 * object can be checked without dereferencing its payload, so a collection never
 * has to trust a half-written object.
 *
 * Every one of them goes through `nil_object_kind`, which answers -1 for anything
 * that is not an object. None of them dereferences a header: `nil_object_header`
 * returns NULL for a value that is not a tagged pointer, and three of these used to
 * read through it anyway — which crashed on the bit pattern `NIL_TAG_PTR`. */
int nil_is_inline_int(NilValue value) { return (value & NIL_TAG_MASK) == NIL_TAG_INT; }

int nil_is_int_box(NilValue value) { return nil_object_kind(value) == NIL_OBJ_INTBOX; }

int nil_is_float(NilValue value) { return nil_object_kind(value) == NIL_OBJ_FLOATBOX; }

int nil_is_string(NilValue value) { return nil_object_kind(value) == NIL_OBJ_STRING; }

/* nil_is_number accepts both boxes and both inline forms.
 *
 * A boxed float is a number; the difference from a boxed integer only matters to
 * arithmetic that has to produce a float, and this function is asked by the places
 * that do not (truthiness, `len`, the renderer's fallback). */
int nil_is_number(NilValue value) {
  if (nil_is_inline_int(value)) {
    return 1;
  }
  int kind = nil_object_kind(value);
  return kind == NIL_OBJ_INTBOX || kind == NIL_OBJ_FLOATBOX;
}

/* ── Construction ─────────────────────────────────────────────────────── */

NilValue nil_int_new(int64_t value) {
  /* Tagging is only valid when it would not collide with the low three bits.
   * Almost every integer qualifies; the exceptions are boxed. */
  if ((value & NIL_TAG_MASK) == 0) {
    return ((NilValue)value) | NIL_TAG_INT;
  }
  NilBox *box = (NilBox *)nil_object_new(NIL_OBJ_INTBOX, sizeof(NilBox));
  if (box == NULL) {
    return NIL_TAG_NIL;
  }
  box->value = value;
  return nil_object_pointer(box);
}

int64_t nil_int_value(NilValue value) {
  if (nil_is_inline_int(value)) {
    return (int64_t)(value & ~((NilValue)NIL_TAG_MASK));
  }
  if (nil_is_int_box(value)) {
    return ((NilBox *)nil_object_header(value))->value;
  }
  /* A float asked for as an integer is a caller mistake. Returning 0 rather than
   * the bit pattern is deliberate: the bit pattern is a *number* the program never
   * wrote, and a wrong large integer is harder to notice than a zero. */
  return 0;
}

NilValue nil_float_new(double value) {
  NilBox *box = (NilBox *)nil_object_new(NIL_OBJ_FLOATBOX, sizeof(NilBox));
  if (box == NULL) {
    return NIL_TAG_NIL;
  }
  /* The bit pattern rather than the value, because a double is 64 bits and the
   * box's field is 64 signed bits: storing the value would work, but storing the
   * pattern is what makes reading it back exact. */
  memcpy(&box->value, &value, sizeof(double));
  return nil_object_pointer(box);
}

double nil_float_value(NilValue value) {
  if (!nil_is_float(value)) {
    return (double)nil_int_value(value);
  }
  double result = 0;
  memcpy(&result, &((NilBox *)nil_object_header(value))->value, sizeof(double));
  return result;
}

NilValue nil_bool_new(int64_t truthy) { return truthy ? NIL_TAG_TRUE : NIL_TAG_FALSE; }

int64_t nil_truthy(NilValue value) {
  switch (value & NIL_TAG_MASK) {
  case NIL_TAG_NIL:
    return 0;
  case NIL_TAG_TRUE:
    return 1;
  case NIL_TAG_FALSE:
    return 0;
  case NIL_TAG_INT:
    return nil_int_value(value) != 0;
  case NIL_TAG_PTR:
    switch (nil_object_kind(value)) {
    case -1:
      /* A pointer tag with no address. Not an object, so not truthy — and saying
       * so here keeps the one degenerate value from behaving like a live heap
       * object in the one place the collector cannot check it. */
      return 0;
    case NIL_OBJ_STRING:
    case NIL_OBJ_ARRAY:
    case NIL_OBJ_MAP:
      /* An empty container is false. Every language with a truthy test makes this
       * choice, and a program that treats "" as true has to say so explicitly. */
      return nil_object_header(value)->length != 0;
    case NIL_OBJ_FLOATBOX: {
      double number = nil_float_value(value);
      /* NaN is false rather than true or a crash. It is the one value where every
       * other comparison says "not equal to itself", and `if x` asking "is it
       * non-zero" is the only question that has a stable answer. */
      return number != 0.0 && number == number;
    }
    }
    return 1;
  }
  return 0;
}

NilValue nil_bool_not(NilValue value) { return nil_bool_new(!nil_truthy(value)); }

/* ── Strings ──────────────────────────────────────────────────────────── */

static NilString *string_of(NilValue value) {
  return nil_is_string(value) ? (NilString *)nil_object_header(value) : NULL;
}

/* nil_string_lit builds a string that generated code refers to by symbol.
 *
 * Pinned for the whole run, because a literal has no pointer leading to it —
 * generated code holds it in a constant and passes it as an argument, and the
 * collector sees nothing between the constant and the argument. Without the pin the
 * sweep would free a table the program is about to read.
 *
 * That is the *only* reason this function pins, which is why it is separate from
 * `nil_string_new`: every runtime-produced string went through this one, so
 * `"a" + "b"` in a loop pinned an unbounded list and nothing the program did could
 * ever reclaim it. A leak that grows for the life of the process is the one failure
 * a collector cannot have. See ADR-0045.
 */
/* build_string is the allocation both constructors below share. */
static NilValue build_string(const char *bytes, size_t length);

/* nil_string_lit builds a string that generated code refers to by symbol.
 *
 * Pinned for the whole run, because a literal has no pointer leading to it —
 * generated code holds it in a constant and passes it as an argument, and the
 * collector sees nothing between the constant and the argument. Without the pin the
 * sweep would free a table the program is about to read.
 *
 * That is the *only* reason this function pins, which is why it is separate from
 * `nil_string_new`: every runtime-produced string went through this one, so
 * `"a" + "b"` in a loop pinned an unbounded list and nothing the program did could
 * ever reclaim it. A leak that grows for the life of the process is the one failure
 * a collector cannot have. See ADR-0045.
 */
NilValue nil_string_lit(const char *bytes, size_t length) {
  NilValue value = build_string(bytes, length);
  nil_gc_pin_string(value);
  return value;
}

/* nil_string_new builds a string the program reached at run time.
 *
 * Not pinned: whatever holds this string is a variable, an array element or a map
 * value, and the collector will find it there. Pinning it as well would make it
 * immortal and the pin list would grow for the life of the process.
 */
NilValue nil_string_new(const char *bytes, size_t length) {
  return build_string(bytes, length);
}

/* build_string is the allocation both constructors share. */
static NilValue build_string(const char *bytes, size_t length) {
  NilString *string = (NilString *)nil_object_new(NIL_OBJ_STRING, sizeof(NilString));
  if (string == NULL) {
    return NIL_TAG_NIL;
  }
  string->header.length = (uint32_t)length;
  string->data = (char *)malloc(length + 1);
  if (string->data == NULL) {
    set_error(NIL_ERR_OOM, "out of memory allocating a string");
    return NIL_TAG_NIL;
  }
  if (length != 0) {
    memcpy(string->data, bytes, length);
  }
  string->data[length] = '\0';
  return nil_object_pointer(string);
}

/* nil_string_len counts *characters*, not bytes.
 *
 * Nilang's strings are Unicode and the interpreter counts runes. A runtime that
 * counted bytes would disagree on every string containing a non-ASCII character,
 * and the disagreement would look like an encoding bug in the language rather than
 * in this function. */
int64_t nil_string_len(NilValue value) {
  NilString *string = string_of(value);
  if (string == NULL) {
    return 0;
  }
  int64_t characters = 0;
  for (uint32_t i = 0; i < string->header.length; i++) {
    /* A UTF-8 continuation byte is 10xxxxxx; a character starts on anything else.
     * Counting bytes instead answers 6 for "héllo", which is five characters. */
    if (((unsigned char)string->data[i] & 0xC0) != 0x80) {
      characters++;
    }
  }
  return characters;
}

/* utf8_sequence_length is how many bytes the character at `bytes` occupies.
 *
 * It reads the leading byte rather than decoding, because the only question asked
 * of it is "how far do I move to the next character", and the answer follows from
 * the leading byte's pattern alone. The `remaining` clamp is what keeps a
 * truncated string — which a program can build by indexing — from reading past the
 * end: a declared length longer than the bytes is a claim, and the clamp treats it
 * as a bound rather than a promise. */
size_t utf8_sequence_length(const char *bytes, size_t remaining) {
  unsigned char lead = (unsigned char)bytes[0];
  size_t width = 1;
  if ((lead & 0xE0) == 0xC0) {
    width = 2;
  } else if ((lead & 0xF0) == 0xE0) {
    width = 3;
  } else if ((lead & 0xF8) == 0xF0) {
    width = 4;
  }
  return width < remaining ? width : remaining;
}

size_t nil_string_byte_offset(NilString *string, int64_t index) {
  size_t offset = 0;
  for (int64_t character = 0; character < index; character++) {
    if (offset >= string->header.length) {
      return offset;
    }
    offset += utf8_sequence_length(string->data + offset, string->header.length - offset);
  }
  return offset;
}

int64_t nil_string_index(NilValue value, int64_t index) {
  NilString *string = string_of(value);
  if (string == NULL) {
    set_error(NIL_ERR_TYPE, "cannot index a value that is not a string");
    return 0;
  }
  if (index < 0) {
    set_error(NIL_ERR_INDEX, "string index is negative");
    return 0;
  }
  size_t offset = nil_string_byte_offset(string, index);
  if (offset >= string->header.length) {
    char message[128];
    snprintf(message, sizeof(message), "string index %lld is out of range", (long long)index);
    set_error(NIL_ERR_INDEX, message);
    return 0;
  }

  size_t width = utf8_sequence_length(string->data + offset, string->header.length - offset);
  /* The code point, not the lead byte. `int(c)` for 'a' and for 'é' should agree
   * about what a character is; returning the byte of a two-byte character would
   * make the first 256 answers meaningful and every one after it wrong. */
  unsigned int code = (unsigned char)string->data[offset];
  if (width == 2 && offset + 1 < string->header.length) {
    code = ((code & 0x1F) << 6) | ((unsigned char)string->data[offset + 1] & 0x3F);
  } else if (width == 3 && offset + 2 < string->header.length) {
    code = ((code & 0x0F) << 12) | (((unsigned char)string->data[offset + 1] & 0x3F) << 6) |
           ((unsigned char)string->data[offset + 2] & 0x3F);
  } else if (width == 4 && offset + 3 < string->header.length) {
    code = ((code & 0x07) << 18) | (((unsigned char)string->data[offset + 1] & 0x3F) << 12) |
           (((unsigned char)string->data[offset + 2] & 0x3F) << 6) |
           ((unsigned char)string->data[offset + 3] & 0x3F);
  }
  return (int64_t)code;
}

/* The three helpers below, declared here because `nil_string_concat` names them and
 * they name each other. */
static int is_textual(NilValue value);
static NilValue to_text(NilValue value);
static NilValue concat_bytes(NilValue left, const char *leftBytes, size_t leftLength,
                             NilValue right, const char *rightBytes, size_t rightLength);

NilValue nil_string_concat(NilValue left, NilValue right) {
  NilString *a = string_of(left);
  NilString *b = string_of(right);

  if (a != NULL && b != NULL) {
    return concat_bytes(left, a->data, a->header.length, right, b->data, b->header.length);
  }

  /* A string beside a number is concatenation, not a type error — `"count: " + 42`
   * is a thing every language with `+` on strings does, and the interpreter does
   * it. The number is rendered by the same renderer `print` uses, so a float
   * concatenates exactly as it prints and a bool reads as `true`. */
  if (a != NULL && is_textual(right)) {
    NilValue converted = to_text(right);
    if (converted == NIL_TAG_NIL) {
      return NIL_TAG_NIL;
    }
    NilString *text = string_of(converted);
    return concat_bytes(left, a->data, a->header.length, converted, text->data,
                        text->header.length);
  }
  if (b != NULL && is_textual(left)) {
    NilValue converted = to_text(left);
    if (converted == NIL_TAG_NIL) {
      return NIL_TAG_NIL;
    }
    NilString *text = string_of(converted);
    return concat_bytes(converted, text->data, text->header.length, right, b->data,
                        b->header.length);
  }

  /* Two non-strings: hand it to arithmetic, which knows the real types and will
   * give a better message than this function could. */
  return nil_int_add(left, right);
}

/* is_textual reports whether a value has a sensible one-line form.
 *
 * Only numbers, booleans and nil. A list does not: rendering it as `[1, 2]` would
 * make `"total: " + [1, 2]` a program that appears to work and produces nonsense,
 * which is worse than refusing.
 */
static int is_textual(NilValue value) {
  return nil_is_number(value) || nil_is_bool(value) || value == NIL_TAG_NIL;
}

/* A boolean is a tag, so it is recognised from the tag rather than from an object.
 * Written out here because `nil_bool_new` builds one and every other predicate in
 * the runtime is a question about an object. */
int nil_is_bool(NilValue value) {
  return (value & NIL_TAG_MASK) == NIL_TAG_FALSE || (value & NIL_TAG_MASK) == NIL_TAG_TRUE;
}

/* to_text renders a scalar as a string, using the runtime's own renderer.
 *
 * The renderer is `nil_print`'s, so a float concatenates exactly as it prints —
 * which is the property that keeps `"x: " + 2.5` and `print(2.5)` in agreement, and
 * they are the same rule in every language that has both.
 */
static NilValue to_text(NilValue value) {
  char buffer[128];
  int64_t written = nil_render(value, buffer, (int64_t)sizeof(buffer));
  return nil_string_new(buffer, (size_t)written);
}

/* nil_to_str is `str(x)` — the rendered text of any value.
 *
 * # Why it is not `to_text`
 *
 * `to_text` renders into 128 bytes, which is the right budget for the operator
 * that uses it: `"a" + 1` is a concatenation of two short things, and a program
 * that concatenated a 5000-byte list into a string wanted the truncation. `str`
 * is different — it is the way a program asks what something *is*, as text, and a
 * function that answers "the first 127 characters" is not answering.
 *
 * So the buffer grows until the render fits. A string is returned as itself, which
 * is both the obvious answer and the cheapest: `str(s)` on a string is `s`, and
 * copying it would allocate for no reason.
 */
NilValue nil_to_str(NilValue value) {
  if (nil_is_string(value)) {
    return value;
  }

  size_t capacity = 128;
  for (;;) {
    char *buffer = (char *)malloc(capacity);
    if (buffer == NULL) {
      nil_panic(NIL_ERR_OOM, "out of memory rendering a value as text");
    }
    int64_t written = nil_render(value, buffer, (int64_t)capacity);
    /* `nil_render` reports what it wrote, not what it needed, so a value that
     * fills the buffer exactly is indistinguishable from one that was cut short.
     * Rendering one byte short makes the two differ: if the value needs more than
     * the space, the final byte is gone. */
    if ((size_t)written + 1 < capacity) {
      NilValue result = nil_string_new(buffer, (size_t)written);
      free(buffer);
      return result;
    }
    free(buffer);
    capacity *= 2;
  }
}

/* nil_is_null is `isNull(x)`.
 *
 * It exists because `??` and `?.` both ask it, and a program should be able to ask
 * it too without having to write `x == null`. It answers with a real Boolean
 * rather than 0 or 1, because 1 is the *tag* of false in this representation — a
 * raw 1 reaching a program would read as false, and the answer to "is this null?"
 * being false is the worst answer to get backwards.
 */
NilValue nil_is_null(NilValue value) {
  return nil_bool_new(value == NIL_TAG_NIL);
}

/* concat_bytes joins two byte sequences into a new string.
 *
 * Both sources are pinned across the allocation: a collection triggered by it would
 * otherwise be free to free the bytes being copied from. The pin list is cleared on
 * the way out, so a pin here is scoped to this one call.
 */
static NilValue concat_bytes(NilValue left, const char *leftBytes, size_t leftLength,
                             NilValue right, const char *rightBytes, size_t rightLength) {
  nil_gc_pin(left);
  nil_gc_pin(right);

  size_t total = leftLength + rightLength;
  NilString *result = (NilString *)nil_object_new(NIL_OBJ_STRING, sizeof(NilString));
  if (result == NULL) {
    nil_gc_unpin_all();
    return NIL_TAG_NIL;
  }

  result->header.length = (uint32_t)total;
  result->data = (char *)malloc(total + 1);
  if (result->data == NULL) {
    nil_gc_unpin_all();
    set_error(NIL_ERR_OOM, "out of memory concatenating strings");
    return NIL_TAG_NIL;
  }
  if (leftLength != 0) {
    memcpy(result->data, leftBytes, leftLength);
  }
  if (rightLength != 0) {
    memcpy(result->data + leftLength, rightBytes, rightLength);
  }
  result->data[total] = '\0';

  nil_gc_unpin_all();
  return nil_object_pointer(result);
}

NilValue nil_substr(NilValue value, int64_t start, int64_t length) {
  NilString *string = string_of(value);
  if (string == NULL) {
    set_error(NIL_ERR_TYPE, "substr expects a string");
    return NIL_TAG_NIL;
  }

  int64_t characters = nil_string_len(value);
  /* A negative start counts from the end, and a negative length runs that far
   * back from it. Both are one adjustment rather than a second set of rules, which
   * is what makes "drop the last three characters" possible without a helper. */
  if (start < 0) {
    start += characters;
  }
  if (start < 0) {
    start = 0;
  }
  if (start > characters) {
    start = characters;
  }
  if (length < 0) {
    /* A negative length reaches that far back from the end, which is what makes
     * "drop the last three characters" possible: `substr(s, 0, -3)` on "abcdef" is
     * "abc". There is no `+1` — the end is `characters - |length|`, and the length
     * is the difference between that and the start. */
    length = characters - start + length;
    if (length < 0) {
      length = 0;
    }
  }
  if (start + length > characters) {
    length = characters - start;
  }

  size_t from = nil_string_byte_offset(string, start);
  size_t to = from;
  for (int64_t i = 0; i < length && to < string->header.length; i++) {
    to += utf8_sequence_length(string->data + to, string->header.length - to);
  }
  return nil_string_new(string->data + from, to - from);
}

/* nil_substr_from is `substr(s, start)` — the two-argument form, to the end.
 *
 * A separate entry point rather than a sentinel length, because every sentinel is a
 * number that has to be written out in the IR and remembered by whoever reads it.
 * `INT64_MAX` would work and would also be an overflow waiting for the next change
 * to the clamping above: `start + length` wraps negative, the clamp stops firing,
 * and a length of nine quintillion is left to be defended against by a loop bound.
 *
 * Passing the whole length works because `nil_substr` clamps the start first and
 * then shortens any length that would run past the end — so `characters` is
 * "at least to the end", and the arithmetic downstream is already written for it.
 */
NilValue nil_substr_from(NilValue value, int64_t start) {
  if (string_of(value) == NULL) {
    set_error(NIL_ERR_TYPE, "substr expects a string");
    return NIL_TAG_NIL;
  }
  return nil_substr(value, start, nil_string_len(value));
}

int64_t nil_string_find(NilValue haystack, NilValue needle) {
  NilString *h = string_of(haystack);
  NilString *n = string_of(needle);
  if (h == NULL || n == NULL || n->header.length == 0 || n->header.length > h->header.length) {
    return -1;
  }

  /* Every match position is computed, not just the first. The count is of
   * characters, so a needle at byte 5 inside a string with two multi-byte
   * characters before it is character index 3 — the number a program indexing
   * with `substr` expects, not the one it would have to convert. */
  size_t character = 0;
  for (uint32_t i = 0; i + n->header.length <= h->header.length;) {
    if (memcmp(h->data + i, n->data, n->header.length) == 0) {
      return (int64_t)character;
    }
    i += (uint32_t)utf8_sequence_length(h->data + i, h->header.length - i);
    character++;
  }
  return -1;
}

int64_t nil_string_split_count(NilValue value) {
  /* The count of pieces depends on the separator, and the emitter passes it. When
   * it is not passed, the character count is what a caller asking "how many pieces
   * will this make" can do nothing useful with anyway. */
  NilString *string = string_of(value);
  return string == NULL ? 0 : nil_string_len(value);
}

/* The two case converters share one body.
 *
 * ASCII only, and deliberately: case conversion is locale-sensitive across the
 * full Unicode range, and a partial implementation that leaves accented letters
 * alone is less surprising than one that gets them wrong. A program that needs
 * `İ` has a table of its own. */
static NilValue convert_case(NilValue value, int upper) {
  NilString *string = string_of(value);
  if (string == NULL) {
    return value;
  }
  nil_gc_pin(value);

  NilString *result = (NilString *)nil_object_new(NIL_OBJ_STRING, sizeof(NilString));
  if (result == NULL) {
    nil_gc_unpin_all();
    return NIL_TAG_NIL;
  }
  result->header.length = string->header.length;
  result->data = (char *)malloc(string->header.length + 1);
  if (result->data == NULL) {
    nil_gc_unpin_all();
    return NIL_TAG_NIL;
  }
  memcpy(result->data, string->data, string->header.length);
  for (uint32_t i = 0; i < result->header.length; i++) {
    char c = result->data[i];
    if (upper && c >= 'a' && c <= 'z') {
      result->data[i] = (char)(c - 'a' + 'A');
    } else if (!upper && c >= 'A' && c <= 'Z') {
      result->data[i] = (char)(c - 'A' + 'a');
    }
  }
  result->data[result->header.length] = '\0';
  nil_gc_unpin_all();
  return nil_object_pointer(result);
}

NilValue nil_string_to_upper(NilValue value) { return convert_case(value, 1); }
NilValue nil_string_to_lower(NilValue value) { return convert_case(value, 0); }

NilValue nil_string_trim(NilValue value) {
  NilString *string = string_of(value);
  if (string == NULL) {
    return value;
  }
  uint32_t start = 0;
  uint32_t end = string->header.length;
  while (start < end && (string->data[start] == ' ' || string->data[start] == '\t' ||
                         string->data[start] == '\n' || string->data[start] == '\r')) {
    start++;
  }
  while (end > start && (string->data[end - 1] == ' ' || string->data[end - 1] == '\t' ||
                         string->data[end - 1] == '\n' || string->data[end - 1] == '\r')) {
    end--;
  }
  return nil_string_new(string->data + start, end - start);
}

NilValue nil_char_from_int(int64_t code) {
  char buffer[4];
  unsigned int value = (unsigned int)code;
  size_t length;

  if (value < 0x80) {
    buffer[0] = (char)value;
    length = 1;
  } else if (value < 0x800) {
    buffer[0] = (char)(0xC0 | (value >> 6));
    buffer[1] = (char)(0x80 | (value & 0x3F));
    length = 2;
  } else if (value < 0x10000) {
    buffer[0] = (char)(0xE0 | (value >> 12));
    buffer[1] = (char)(0x80 | ((value >> 6) & 0x3F));
    buffer[2] = (char)(0x80 | (value & 0x3F));
    length = 3;
  } else {
    buffer[0] = (char)(0xF0 | (value >> 18));
    buffer[1] = (char)(0x80 | ((value >> 12) & 0x3F));
    buffer[2] = (char)(0x80 | ((value >> 6) & 0x3F));
    buffer[3] = (char)(0x80 | (value & 0x3F));
    length = 4;
  }
  /* A code point outside Unicode, or a surrogate, would produce bytes that are not
   * valid UTF-8. The empty string is returned instead, because an invalid string
   * in a program is a bug that should be obvious when it is printed and not a
   * crash in the middle of a comparison. */
  if (value > 0x10FFFF || (value >= 0xD800 && value <= 0xDFFF)) {
    return nil_string_new("", 0);
  }
  return nil_string_new(buffer, length);
}

/* ── Arithmetic ───────────────────────────────────────────────────────── */

/* The five integer implementations.
 *
 * Separate from the dispatchers below rather than inlined into them, so that each
 * is one expression with no type test in it — and so that the dispatch decision is
 * made once, in one place, for every operator.
 *
 * Declared before the dispatchers because each dispatcher names its own
 * implementation as a value, which is what keeps the pair together.
 */
static NilValue nil_int_add_impl(NilValue a, NilValue b);
static NilValue nil_int_sub_impl(NilValue a, NilValue b);
static NilValue nil_int_mul_impl(NilValue a, NilValue b);
static NilValue nil_int_div_impl(NilValue a, NilValue b);
static NilValue nil_int_mod_impl(NilValue a, NilValue b);

static int as_int_pair(const char *name, NilValue a, NilValue b, int64_t *left,
                       int64_t *right) {
  /* A float beside an integer is not an error: the integer is promoted. This is
   * the one implicit conversion the language has, and having it here means the
   * interpreter and this runtime cannot disagree about whether `1 + 2.5` works. */
  if (nil_is_float(a) || nil_is_float(b)) {
    nil_type_error(name, a, b);
    return 0;
  }
  if (!nil_is_number(a) || !nil_is_number(b)) {
    nil_type_error(name, a, b);
    return 0;
  }
  *left = nil_int_value(a);
  *right = nil_int_value(b);
  return 1;
}

#define AS_INT_PAIR(name, a, b, left, right)                                         \
  do {                                                                               \
    if (!as_int_pair(name, a, b, &(left), &(right))) {                               \
      return NIL_TAG_NIL;                                                            \
    }                                                                                \
  } while (0)

/* nil_int_add and its five siblings dispatch on the operands' types.
 *
 * # Why the dispatch is here and not in the compiler
 *
 * The compiler emits the *same* call for `1 + 2` and `1.5 + 2.0` because at MIR
 * level an operator is an operator: the types are in the representation, not in the
 * instruction. Emitting `add i64` is impossible — the value is tagged — and
 * emitting a different function per operand type would mean the compiler
 * understanding the tagging.
 *
 * So the one function per operator checks, promotes an integer beside a float, and
 * calls the float path. This is the same reasoning as ADR-0037, applied to the
 * choice between two runtime functions rather than between an instruction and a
 * call.
 */
static NilValue on_ints(const char *name, NilValue a, NilValue b,
                        NilValue (*onInt)(NilValue, NilValue),
                        NilValue (*onFloat)(NilValue, NilValue)) {
  /* A string beside anything is concatenation, and this is the only place that can
   * know it: the compiler emits `+` as one call whether it is joining numbers or
   * joining text, because at MIR level `s + 42` and `1 + 2` are the same
   * instruction. */
  if (nil_is_string(a) || nil_is_string(b)) {
    return nil_string_concat(a, b);
  }

  if (nil_is_float(a) || nil_is_float(b)) {
    return onFloat(a, b);
  }

  int64_t left, right;
  if (!as_int_pair(name, a, b, &left, &right)) {
    return NIL_TAG_NIL;
  }
  (void)left;
  (void)right;
  return onInt(a, b);
}

NilValue nil_int_add(NilValue a, NilValue b) {
  return on_ints("add", a, b, nil_int_add_impl, nil_float_add);
}

NilValue nil_int_sub(NilValue a, NilValue b) {
  return on_ints("subtract", a, b, nil_int_sub_impl, nil_float_sub);
}

NilValue nil_int_mul(NilValue a, NilValue b) {
  return on_ints("multiply", a, b, nil_int_mul_impl, nil_float_mul);
}

NilValue nil_int_div(NilValue a, NilValue b) {
  return on_ints("divide", a, b, nil_int_div_impl, nil_float_div);
}

NilValue nil_int_mod(NilValue a, NilValue b) {
  /* `%` on a float is a remainder, and the language has no float remainder operator;
   * the caller's `as_float_pair` refuses it with a message naming both types. */
  if (nil_is_float(a) || nil_is_float(b)) {
    nil_type_error("remainder", a, b);
    return NIL_TAG_NIL;
  }
  return nil_int_mod_impl(a, b);
}


NilValue nil_int_add_impl(NilValue a, NilValue b) {
  int64_t left, right;
  if (!as_int_pair("add", a, b, &left, &right)) {
    return NIL_TAG_NIL;
  }
  return nil_int_new(left + right);
}

NilValue nil_int_sub_impl(NilValue a, NilValue b) {
  int64_t left, right;
  if (!as_int_pair("subtract", a, b, &left, &right)) {
    return NIL_TAG_NIL;
  }
  return nil_int_new(left - right);
}

NilValue nil_int_mul_impl(NilValue a, NilValue b) {
  int64_t left, right;
  if (!as_int_pair("multiply", a, b, &left, &right)) {
    return NIL_TAG_NIL;
  }
  return nil_int_new(left * right);
}

NilValue nil_int_div_impl(NilValue a, NilValue b) {
  int64_t left, right;
  if (!as_int_pair("divide", a, b, &left, &right)) {
    return NIL_TAG_NIL;
  }
  if (right == 0) {
    nil_panic(NIL_ERR_DIV_ZERO, "division by zero");
  }
  /* Truncating, matching the interpreter. Flooring would make `-7 / 2` equal -4
   * where every other engine says -3, and the two disagreeing is exactly what this
   * runtime exists to prevent. */
  return nil_int_new(left / right);
}

NilValue nil_int_mod_impl(NilValue a, NilValue b) {
  int64_t left, right;
  if (!as_int_pair("remainder", a, b, &left, &right)) {
    return NIL_TAG_NIL;
  }
  if (right == 0) {
    nil_panic(NIL_ERR_DIV_ZERO, "remainder by zero");
  }
  /* C's `%` truncates like `/` does, so the two agree. A floor-based modulo would
   * give `-7 % 2 == 1`, which is not what the interpreter reports. */
  return nil_int_new(left % right);
}

static int as_float_pair(const char *name, NilValue a, NilValue b, double *left,
                         double *right) {
  if (!nil_is_number(a) || !nil_is_number(b)) {
    nil_type_error(name, a, b);
    return 0;
  }
  *left = nil_is_float(a) ? nil_float_value(a) : (double)nil_int_value(a);
  *right = nil_is_float(b) ? nil_float_value(b) : (double)nil_int_value(b);
  return 1;
}

#define AS_FLOAT_PAIR(name, a, b, left, right)                                       \
  do {                                                                               \
    if (!as_float_pair(name, a, b, &(left), &(right))) {                             \
      return NIL_TAG_NIL;                                                            \
    }                                                                                \
  } while (0)

NilValue nil_float_add(NilValue a, NilValue b) {
  double left, right;
  AS_FLOAT_PAIR("add", a, b, left, right);
  return nil_float_new(left + right);
}

NilValue nil_float_sub(NilValue a, NilValue b) {
  double left, right;
  AS_FLOAT_PAIR("subtract", a, b, left, right);
  return nil_float_new(left - right);
}

NilValue nil_float_mul(NilValue a, NilValue b) {
  double left, right;
  AS_FLOAT_PAIR("multiply", a, b, left, right);
  return nil_float_new(left * right);
}

NilValue nil_float_div(NilValue a, NilValue b) {
  double left, right;
  AS_FLOAT_PAIR("divide", a, b, left, right);
  if (right == 0.0) {
    nil_panic(NIL_ERR_DIV_ZERO, "division by zero");
  }
  return nil_float_new(left / right);
}

/* ── Comparison ───────────────────────────────────────────────────────── */

/* One function for every operator, with the operator as data.
 *
 * The compiler emits the same three-argument call for `==`, `<` and `>=`, so the
 * operator has to travel as an argument. The switch is here rather than at each
 * call site so the operator code is validated once — a code the compiler invented
 * would otherwise read past the end of a table. */
static int compare(int64_t left, int64_t right, int64_t op) {
  switch (op) {
  case 0:
    return left == right;
  case 1:
    return left != right;
  case 2:
    return left < right;
  case 3:
    return left <= right;
  case 4:
    return left > right;
  case 5:
    return left >= right;
  default:
    set_error(NIL_ERR_INTERNAL, "unknown comparison operator");
    return 0;
  }
}

/* nil_logic applies `&&` or `||` and returns a Nilang `Bool`.
 *
 * # Why this is a runtime call rather than a branch in the emitted code
 *
 * Two reasons, and the second is the one that costs.
 *
 * The first is the usual one: a Nilang value is tagged, so the answer is a *value*
 * and not a bit, and every program has to reach the runtime to get one.
 *
 * The second is that the emitter cannot compute the answer. By the time it holds
 * both operands it holds their *values*, and asking whether each is true has already
 * discarded which side was which — so `a || b` compiled as "is the left side true"
 * answers the wrong question whenever the left is false and the right is true. The
 * emitter cannot recover the distinction, because a value does not carry it.
 *
 * The codes match the comparison operator codes: 0 is `and`, 1 is `or`. One
 * numbering for both, because both are "the same operator with different arguments",
 * and a caller that had to remember two would eventually pass the wrong one.
 */
int64_t nil_logic(NilValue left, NilValue right, int64_t op) {
  int leftTruthy = nil_truthy(left) != 0;
  int rightTruthy = nil_truthy(right) != 0;

  switch (op) {
  case 0:
    return nil_bool_new(leftTruthy && rightTruthy);
  case 1:
    return nil_bool_new(leftTruthy || rightTruthy);
  default:
    set_error(NIL_ERR_INTERNAL, "unknown logical operator");
    return NIL_TAG_NIL;
  }
}

int64_t nil_int_cmp(NilValue a, NilValue b, int64_t op) {
  /* A string beside anything is a string comparison, for the same reason `+` is a
   * concatenation above: the operator is one operator in MIR, and only the runtime
   * can tell what it is being applied to. */
  if (nil_is_string(a) || nil_is_string(b)) {
    return nil_str_cmp(a, b, op);
  }

  /* Nil answers equality, and only equality.
   *
   * This is not an extra case — it is the interpreter's own rule. A pair that
   * matches no other case falls through to `case operator == ": return left ==
   * right`, which is pointer identity, and two nulls are the same pointer. So
   * `null == null` is true, `null == 1` is false, and `null != 1` is true.
   *
   * It has to be written down because every lookup in the language can answer nil
   * — a missing map key, a read past the end of a list, a field that is not there —
   * so `h[k] == null` is the ordinary way to ask whether a key was present. An
   * engine that made that a type error would make the most common test in the
   * language unanswerable.
   */
  if (a == NIL_TAG_NIL || b == NIL_TAG_NIL) {
    if (op != 0 && op != 1) {
      nil_type_error("compare", a, b);
      return NIL_TAG_NIL;
    }
    int same = (a == NIL_TAG_NIL) && (b == NIL_TAG_NIL);
    return nil_bool_new(op == 0 ? same : !same);
  }

  /* Two booleans are compared as themselves.
   *
   * Equality is the only operator they answer. `true < false` is not an ordering of
   * anything — the interpreter refuses it, and a language that let it would have to
   * say which of the two booleans is smaller. The interpreter's own rule for a
   * non-numeric pair is pointer identity, which for two tags is exactly "the same
   * tag".
   */
  if (nil_is_bool(a) && nil_is_bool(b)) {
    if (op != 0 && op != 1) {
      nil_type_error("compare", a, b);
      return NIL_TAG_NIL;
    }
    int same = nil_truthy(a) == nil_truthy(b);
    return nil_bool_new(op == 0 ? same : !same);
  }

  if (!nil_is_number(a) || !nil_is_number(b)) {
    nil_type_error("compare", a, b);
    return NIL_TAG_NIL;
  }

  /* A float and an integer are compared numerically, not as their representations.
   * `5 == 5.0` is true in every language that has both, and comparing the boxed
   * bits would make it false — and would make `1 == 1.0` depend on whether the
   * integer happened to be inlined. */
  if (nil_is_float(a) || nil_is_float(b)) {
    double left = nil_is_float(a) ? nil_float_value(a) : (double)nil_int_value(a);
    double right = nil_is_float(b) ? nil_float_value(b) : (double)nil_int_value(b);
    if (left < right) {
      return nil_bool_new(compare(-1, 0, op));
    }
    if (left > right) {
      return nil_bool_new(compare(1, 0, op));
    }
    return nil_bool_new(compare(0, 0, op));
  }
  return nil_bool_new(compare(nil_int_value(a), nil_int_value(b), op));
}

int64_t nil_str_cmp(NilValue a, NilValue b, int64_t op) {
  NilString *left = string_of(a);
  NilString *right = string_of(b);

  /* One side is a string and the other is not: both are rendered and the *text* is
   * compared.
   *
   * That is the interpreter's rule, and it is the reason `"1" == 1` is true. It
   * reads as surprising until one notices what else follows from it — the same rule
   * makes `1 == "1"`, `"2.5" == 2.5` and `"true" == true` all true, which is
   * consistent, checkable, and what a program comparing a rendered value with what
   * it parsed actually means. The alternative — requiring the types to match — would
   * make `"1" == 1` false, and every program that reads a number out of a record
   * and compares it with a literal would need a conversion it does not have.
   */
  if (left == NULL || right == NULL) {
    NilValue converted = to_text(left == NULL ? a : b);
    if (converted == NIL_TAG_NIL) {
      return NIL_TAG_NIL;
    }

    NilString *text = string_of(converted);
    /* The string is whichever side was *not* converted, which is the opposite of
     * whichever side was null. Taking the null side instead gives a NULL pointer,
     * and the comparison then reads whatever is at address zero. */
    NilString *other = left == NULL ? right : left;
    size_t firstLength, secondLength;
    const char *firstBytes, *secondBytes;

    if (left == NULL) {
      firstBytes = text->data;
      firstLength = text->header.length;
      secondBytes = other->data;
      secondLength = other->header.length;
    } else {
      firstBytes = other->data;
      firstLength = other->header.length;
      secondBytes = text->data;
      secondLength = text->header.length;
    }

    if (firstLength != secondLength ||
        memcmp(firstBytes, secondBytes, firstLength) != 0) {
      return nil_bool_new(op == 1 || op == 2 || op == 4);
    }
    return nil_bool_new(op == 0 || op == 3 || op == 5);
  }

  /* Byte comparison, then a length tie-break.
   *
   * `memcmp` alone is not an ordering: "ab" and "abc" compare equal by their
   * shared prefix, so sorting them would produce an order that changes when an
   * unrelated string is added. Comparing the length afterwards is what makes the
   * result total. */
  size_t shortest = left->header.length < right->header.length ? left->header.length
                                                              : right->header.length;
  int order = shortest == 0 ? 0 : memcmp(left->data, right->data, shortest);
  if (order == 0) {
    order = (left->header.length > right->header.length) -
            (left->header.length < right->header.length);
  }
  return nil_bool_new(compare(order, 0, op));
}

/* ── Math ─────────────────────────────────────────────────────────────── */

static double as_double(NilValue value) {
  return nil_is_float(value) ? nil_float_value(value) : (double)nil_int_value(value);
}

static NilValue check_number(const char *name, NilValue value) {
  if (!nil_is_number(value)) {
    nil_type_error(name, value, NIL_TAG_NIL);
    return NIL_TAG_NIL;
  }
  return value;
}

NilValue nil_sqrt(NilValue value) {
  if (check_number("sqrt", value) == NIL_TAG_NIL) {
    return NIL_TAG_NIL;
  }
  return nil_float_new(sqrt(as_double(value)));
}

NilValue nil_abs(NilValue value) {
  if (check_number("abs", value) == NIL_TAG_NIL) {
    return NIL_TAG_NIL;
  }
  /* An integer stays an integer. Returning a float would make `abs` change a
   * program's types depending on nothing, and every caller would have to cope. */
  if (nil_is_float(value)) {
    double operand = nil_float_value(value);
    return nil_float_new(operand < 0 ? -operand : operand);
  }
  int64_t operand = nil_int_value(value);
  return nil_int_new(operand < 0 ? -operand : operand);
}

NilValue nil_floor(NilValue value) {
  if (check_number("floor", value) == NIL_TAG_NIL) {
    return NIL_TAG_NIL;
  }
  return nil_float_new(floor(as_double(value)));
}

NilValue nil_ceil(NilValue value) {
  if (check_number("ceil", value) == NIL_TAG_NIL) {
    return NIL_TAG_NIL;
  }
  return nil_float_new(ceil(as_double(value)));
}

NilValue nil_round(NilValue value) {
  if (check_number("round", value) == NIL_TAG_NIL) {
    return NIL_TAG_NIL;
  }
  return nil_float_new(round(as_double(value)));
}

NilValue nil_pow(NilValue base, NilValue exponent) {
  if (check_number("pow", base) == NIL_TAG_NIL || check_number("pow", exponent) == NIL_TAG_NIL) {
    return NIL_TAG_NIL;
  }
  return nil_float_new(pow(as_double(base), as_double(exponent)));
}

NilValue nil_min_int(NilValue a, NilValue b) {
  int64_t left = nil_int_value(a);
  int64_t right = nil_int_value(b);
  return nil_int_new(left < right ? left : right);
}

NilValue nil_max_int(NilValue a, NilValue b) {
  int64_t left = nil_int_value(a);
  int64_t right = nil_int_value(b);
  return nil_int_new(left > right ? left : right);
}

NilValue nil_min_float(NilValue a, NilValue b) {
  double left = nil_float_value(a);
  double right = nil_float_value(b);
  return nil_float_new(left < right ? left : right);
}

NilValue nil_max_float(NilValue a, NilValue b) {
  double left = nil_float_value(a);
  double right = nil_float_value(b);
  return nil_float_new(left > right ? left : right);
}

/* ── Time ─────────────────────────────────────────────────────────────── */

int64_t nil_time_now_millis(void) {
  /* The wall clock, not a monotonic interval: a program that prints a timestamp
   * wants a time it can read, and the interpreter's `time()` returns that. */
  return (int64_t)time(NULL) * 1000;
}

int64_t nil_time_sleep_millis(int64_t millis) {
  if (millis <= 0) {
    return 0;
  }
#if defined(_WIN32)
  Sleep((DWORD)millis);
#else
  struct timespec request;
  request.tv_sec = (time_t)(millis / 1000);
  request.tv_nsec = (long)((millis % 1000) * 1000000L);
  nanosleep(&request, NULL);
#endif
  return millis;
}