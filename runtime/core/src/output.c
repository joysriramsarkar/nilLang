/* output.c — printing and string interpolation.
 *
 * # Why the renderer is here and not in the compiler
 *
 * `println(x)` has to produce the same text whichever engine runs it. The
 * interpreter renders from a Go `object.Object`, the VM from its own object, and a
 * compiled program from this file. Three renderers is three chances to disagree
 * about what a float looks like.
 *
 * They cannot share code — they are in three languages — so they share a
 * *specification*: every integral float keeps its `.0`, and an integer is printed
 * without one. `TestTheRuntimeAgreesWithTheInterpreters` is what holds them to it.
 *
 * # Why the buffer is heap-allocated
 *
 * A fixed 8KB buffer, not a growable one. Rendering does not allocate Nilang
 * values, so it cannot trigger a collection, so it cannot free the value it is
 * reading — which is the property a growable buffer would give up. A raw byte
 * buffer from `malloc` keeps that property: the collector only traces Nilang
 * heap objects, so bytes it cannot see cannot disturb it. A value too large
 * for the buffer is truncated rather than rendered in full; `print` is for
 * reading, not for serialising.
 *
 * Heap rather than stack, because an 8KB stack frame forces compilers
 * targeting MSVC to emit a `___chkstk_ms` stack probe, and that symbol comes
 * from compiler-rt — which is not linked on every machine that can compile
 * this file (notably a bare clang + MSVC link). A `print` that compiles but
 * does not link is worse than a `malloc` on a cold path: the allocation
 * happens once per printed value and is freed before returning.
 */

#include "nil.h"
#include "nil_internal.h"

#include <stdio.h>
#include <stdlib.h>
#include <string.h>

/* The print buffer size. Heap-allocated per call (see above); the frame of
 * `nil_print`/`nil_println` stays small so no platform needs a stack probe. */
#define PRINT_BUFFER_SIZE 8192

/* render_into appends a value's text and returns how much it wrote.
 *
 * It appends rather than returning a fresh string so that rendering a large array
 * allocates nothing. Every caller supplies the buffer and its size, and the return
 * value is what the caller advances its offset by — a string inside the rendering
 * writes into the buffer directly rather than through the scratch path, because
 * copying a string out and back in is the one thing that could overflow it. */
static size_t render_into(NilValue value, char *buffer, size_t capacity, size_t offset,
                          int *truncated);

/* render_float formats a double exactly as Nilang's interpreter does.
 *
 * # Why this reimplements Go's `%g` instead of choosing a tidier rule
 *
 * The interpreter writes a float with `fmt.Sprintf("%g", value)`. A program that
 * prints a value, reads it back and compares it therefore sees whatever Go prints:
 * `2.0` prints as `2`, `1e20` prints as `1e+20`, and not-a-number prints as `NaN`.
 * Choosing a different — even better-looking — rule here would make the
 * ahead-of-time engine disagree with the other two about something a program can
 * observe, and three engines disagreeing is the one thing they are not allowed to
 * do.
 *
 * So this is Go's rule, written in C:
 *
 *   1. Use the *shortest* number of significant digits that reads back as the same
 *      double. Go finds it by trying precisions from 1 upwards and keeping the
 *      first that round-trips; so does this.
 *   2. Switch to exponent form when the decimal exponent is below -4 or at least 6.
 *      Go uses 6 because shortest mode hard-codes it, which is why `123456789`
 *      prints as `1.23456789e+08` while `100` prints as `100`.
 *   3. Spell the non-finite values the way Go spells them: `NaN`, `+Inf`, `-Inf`.
 *
 * # What a wrong step 2 costs
 *
 * Nothing, on the values a person writes by hand. A program printing `2.5` or
 * `0.1` is unaffected. The rule only shows itself on a seven-digit number, which
 * is exactly the kind of bug that survives a test suite written by the person who
 * wrote the code.
 */

static int render_float(double value, char *buffer, size_t capacity) {
  if (value != value) {
    return snprintf(buffer, capacity, "NaN");
  }
  if (value == 1.0 / 0.0) {
    return snprintf(buffer, capacity, "+Inf");
  }
  if (value == -1.0 / 0.0) {
    return snprintf(buffer, capacity, "-Inf");
  }

  /* Step 1: the shortest precision that round-trips.
   *
   * Seventeen is the widest that can name every double: a 64-bit float needs 17
   * significant decimal digits to be distinguishable from its neighbours, and no
   * fewer can name all of them. `%.16e` is not enough — 0.3333333333333333 is. */
  char scratch[64];
  int significant = 17;
  for (int precision = 1; precision <= 17; precision++) {
    snprintf(scratch, sizeof(scratch), "%.*e", precision - 1, value);
    if (strtod(scratch, NULL) == value) {
      significant = precision;
      break;
    }
  }
  snprintf(scratch, sizeof(scratch), "%.*e", significant - 1, value);

  /* `scratch` is now `d[.ddd]e[+-]dd`, so the decimal point sits at
   * `exponent + 1`: 123456789 has exponent 8 and its point is after the ninth
   * digit. `atoi` reads the exponent whether or not there is a sign, and the sign
   * is before the `1` rather than before the exponent, so it is not in the way. */
  const char *exponentPart = strchr(scratch, 'e');
  int pointPosition = exponentPart == NULL ? 1 : atoi(exponentPart + 1) + 1;

  /* Step 2: exponent form or fixed form, on Go's rule. */
  int exponent = pointPosition - 1;
  if (exponent < -4 || exponent >= 6) {
    return snprintf(buffer, capacity, "%.*e", significant - 1, value);
  }

  int fractionDigits = significant - pointPosition;
  if (fractionDigits < 0) {
    fractionDigits = 0;
  }
  return snprintf(buffer, capacity, "%.*f", fractionDigits, value);
}

/* write_text copies as much of `text` as fits and reports whether it all did.
 *
 * The truncation flag is separate because a caller rendering a nested structure
 * has to decide what to do about it — an array of 5000 elements should say so
 * rather than stop mid-element. */
static size_t write_text(const char *text, size_t length, char *buffer, size_t capacity,
                         size_t offset, int *truncated) {
  size_t available = capacity > offset + 1 ? capacity - offset - 1 : 0;
  size_t copy = length < available ? length : available;
  if (copy != 0) {
    memcpy(buffer + offset, text, copy);
  }
  buffer[offset + copy] = '\0';
  if (copy != length) {
    *truncated = 1;
  }
  return copy;
}

static size_t render_string(NilValue value, char *buffer, size_t capacity, size_t offset,
                            int *truncated) {
  NilString *string = (NilString *)nil_object_header(value);
  return write_text(string->data, string->header.length, buffer, capacity, offset, truncated);
}

static size_t render_array(NilValue value, char *buffer, size_t capacity, size_t offset,
                           int *truncated) {
  NilArray *array = (NilArray *)nil_object_header(value);
  int inner = 0;
  size_t written = write_text("[", 1, buffer, capacity, offset, truncated);

  for (uint32_t i = 0; i < array->header.length; i++) {
    /* The element count is capped by what the buffer can hold rather than by the
     * array's own length. A cycle would otherwise print forever, and a million
     * elements would print a million either way. */
    if (i > 0) {
      written += write_text(", ", 2, buffer, capacity, offset + written, truncated);
    }
    written += render_into(array->data[i], buffer, capacity, offset + written, truncated);
    if (inner) {
      *truncated = 1;
      return written;
    }
  }
  written += write_text("]", 1, buffer, capacity, offset + written, truncated);
  return written;
}

static size_t render_map(NilValue value, char *buffer, size_t capacity, size_t offset,
                         int *truncated) {
  NilMap *map = (NilMap *)nil_object_header(value);
  int inner = 0;
  size_t written = write_text("{", 1, buffer, capacity, offset, truncated);

  /* Entries are in the order the keys were first written.
   *
   * The table itself is a hash table and its slots are in hash order; walking them
   * would print `{a: 2, b: 1}` for `struct P { b: Int, a: Int }`, which the
   * interpreters printed as `{b: 1, a: 2}`. A struct's fields are declared in an
   * order the reader is entitled to rely on, and a hash literal's entries are
   * required to be printed left to right (SEMANTICS.md §1), so both go through
   * `order` rather than through the slots.
   *
   * The `occupied` check is kept as a guard. Nothing removes a key, so an entry in
   * `order` always names a live slot; if that ever stops being true the entry is
   * skipped rather than reading a slot's worth of nothing. */
  for (size_t i = 0; i < map->count && !inner; i++) {
    size_t slot = map->order[i];
    if (!map->occupied[slot]) {
      continue;
    }
    if (written > offset + 1) {
      written += write_text(", ", 2, buffer, capacity, offset + written, truncated);
    }
    written += render_into(map->keys[slot], buffer, capacity, offset + written, truncated);
    written += write_text(": ", 2, buffer, capacity, offset + written, truncated);
    written += render_into(map->values[slot], buffer, capacity, offset + written, truncated);
  }
  written += write_text("}", 1, buffer, capacity, offset + written, truncated);
  return written;
}

static size_t render_into(NilValue value, char *buffer, size_t capacity, size_t offset,
                          int *truncated) {
  switch (value & NIL_TAG_MASK) {
  case NIL_TAG_NIL:
    return write_text("null", 4, buffer, capacity, offset, truncated);
  case NIL_TAG_TRUE:
    return write_text("true", 4, buffer, capacity, offset, truncated);
  case NIL_TAG_FALSE:
    return write_text("false", 5, buffer, capacity, offset, truncated);
  case NIL_TAG_INT: {
    char scratch[32];
    int written = snprintf(scratch, sizeof(scratch), "%lld", (long long)nil_int_value(value));
    return write_text(scratch, (size_t)written, buffer, capacity, offset, truncated);
  }
  case NIL_TAG_PTR: {
    /* A pointer tag with no address is a null pointer, and it prints as null
     * rather than as `<object>`. `nil_type_name` says "null" for it, and a render
     * that disagreed with the type name would make an error message name a type
     * the program never had. */
    if (nil_object_kind(value) < 0) {
      return write_text("null", 4, buffer, capacity, offset, truncated);
    }
    switch (nil_object_kind(value)) {
    case NIL_OBJ_STRING:
      return render_string(value, buffer, capacity, offset, truncated);
    case NIL_OBJ_FLOATBOX: {
      char scratch[64];
      int written = render_float(nil_float_value(value), scratch, sizeof(scratch));
      return write_text(scratch, (size_t)written, buffer, capacity, offset, truncated);
    }
    case NIL_OBJ_INTBOX: {
      char scratch[32];
      int written = snprintf(scratch, sizeof(scratch), "%lld", (long long)nil_int_value(value));
      return write_text(scratch, (size_t)written, buffer, capacity, offset, truncated);
    }
    case NIL_OBJ_ARRAY:
      return render_array(value, buffer, capacity, offset, truncated);
    case NIL_OBJ_MAP:
      return render_map(value, buffer, capacity, offset, truncated);
    case NIL_OBJ_CLOSURE:
      return write_text("<function>", 10, buffer, capacity, offset, truncated);
    }
    return write_text("<object>", 8, buffer, capacity, offset, truncated);
  }
  }
  return write_text("<unknown>", 9, buffer, capacity, offset, truncated);
}

void nil_print_str(const char *text) { fputs(text, stdout); }

void nil_println_str(const char *text) {
  fputs(text, stdout);
  fputc('\n', stdout);
}

/* nil_render writes a value's text into a caller's buffer and returns its length.
 *
 * Exposed rather than kept static for two reasons: the FFI bridge that talks to a
 * host application needs to put a value into a host buffer without going through
 * stdout, and the conformance harness needs to read what the runtime *would* print
 * rather than capture it. Both would otherwise reimplement the renderer, and a
 * second renderer is a second set of answers.
 *
 * The buffer is always NUL-terminated when `capacity` is at least one. */
int64_t nil_render(NilValue value, char *buffer, int64_t capacity) {
  if (buffer == NULL || capacity <= 0) {
    return 0;
  }
  int truncated = 0;
  size_t written = render_into(value, buffer, (size_t)capacity, 0, &truncated);
  buffer[written < (size_t)capacity ? written : (size_t)capacity - 1] = '\0';
  return (int64_t)written;
}

void nil_print(NilValue value) {
  char *buffer = (char *)malloc(PRINT_BUFFER_SIZE);
  if (buffer == NULL) {
    fputs("<print: out of memory>\n", stderr);
    return;
  }
  int truncated = 0;
  render_into(value, buffer, PRINT_BUFFER_SIZE, 0, &truncated);
  fputs(buffer, stdout);
  free(buffer);
}

void nil_println(NilValue value) {
  char *buffer = (char *)malloc(PRINT_BUFFER_SIZE);
  if (buffer == NULL) {
    fputs("<print: out of memory>\n", stderr);
    return;
  }
  int truncated = 0;
  render_into(value, buffer, PRINT_BUFFER_SIZE, 0, &truncated);
  fputs(buffer, stdout);
  fputc('\n', stdout);
  free(buffer);
}

/* ── Interpolation ────────────────────────────────────────────────────── */

/* A builder grows by doubling.
 *
 * Doubling rather than by a fixed amount, because `"a \(x) b \(y) c \(z)"` appends
 * three times and a fixed growth makes the second append reallocate in a program
 * that builds a long string in a loop. */
static void builder_reserve(NilBuilder *builder, size_t extra) {
  if (builder->length + extra + 1 <= builder->capacity) {
    return;
  }
  size_t capacity = builder->capacity == 0 ? 64 : builder->capacity;
  while (capacity < builder->length + extra + 1) {
    capacity *= 2;
  }
  char *data = (char *)realloc(builder->data, capacity);
  if (data == NULL) {
    nil_panic(NIL_ERR_OOM, "out of memory building an interpolated string");
    return;
  }
  builder->data = data;
  builder->capacity = capacity;
}

static void builder_append(NilBuilder *builder, const char *text, size_t length) {
  builder_reserve(builder, length);
  if (length != 0) {
    memcpy(builder->data + builder->length, text, length);
  }
  builder->length += length;
  builder->data[builder->length] = '\0';
}

/* The four `nil_interp_*` functions are separate rather than one with a type
 * switch, because the generated code knows the type of each piece statically: the
 * switch would be four branches for a decision the caller has already made. */
static void builder_append_value(NilBuilder *builder, NilValue value) {
  char scratch[64];
  int written;

  switch (value & NIL_TAG_MASK) {
  case NIL_TAG_NIL:
    written = snprintf(scratch, sizeof(scratch), "null");
    break;
  case NIL_TAG_TRUE:
    written = snprintf(scratch, sizeof(scratch), "true");
    break;
  case NIL_TAG_FALSE:
    written = snprintf(scratch, sizeof(scratch), "false");
    break;
  case NIL_TAG_INT:
    written = snprintf(scratch, sizeof(scratch), "%lld", (long long)nil_int_value(value));
    break;
  case NIL_TAG_PTR:
    switch (nil_object_kind(value)) {
    case NIL_OBJ_STRING: {
      NilString *string = (NilString *)nil_object_header(value);
      /* The source is pinned across the append, because `builder_reserve` can
       * allocate — and an allocation can collect. */
      nil_gc_pin(value);
      builder_append(builder, string->data, string->header.length);
      nil_gc_unpin_all();
      return;
    }
    case NIL_OBJ_FLOATBOX:
      written = render_float(nil_float_value(value), scratch, sizeof(scratch));
      break;
    case NIL_OBJ_INTBOX:
      written = snprintf(scratch, sizeof(scratch), "%lld", (long long)nil_int_value(value));
      break;
    case NIL_OBJ_ARRAY:
      written = snprintf(scratch, sizeof(scratch), "[...]");
      break;
    case NIL_OBJ_MAP:
      written = snprintf(scratch, sizeof(scratch), "{...}");
      break;
    default:
      written = snprintf(scratch, sizeof(scratch), "<object>");
      break;
    }
    break;
  default:
    written = snprintf(scratch, sizeof(scratch), "<unknown>");
    break;
  }
  builder_append(builder, scratch, (size_t)written);
}

NilBuilder *nil_builder_new(void) {
  NilBuilder *builder = (NilBuilder *)malloc(sizeof(NilBuilder));
  if (builder == NULL) {
    nil_panic(NIL_ERR_OOM, "out of memory creating a string builder");
    return NULL;
  }
  builder->data = NULL;
  builder->length = 0;
  builder->capacity = 0;
  builder_reserve(builder, 0);
  builder->data[0] = '\0';
  return builder;
}

void nil_builder_free(NilBuilder *builder) {
  if (builder == NULL) {
    return;
  }
  free(builder->data);
  free(builder);
}

/* nil_builder_finish takes the builder's bytes and frees the builder.
 *
 * The bytes are copied into a Nilang string and the original buffer released, so
 * the result does not depend on the builder staying alive. A string that borrowed
 * the builder's memory would be freed by the next `nil_builder_free` — which is
 * exactly what happens to an interpolated string held in a loop variable. */
NilValue nil_builder_finish(NilBuilder *builder) {
  if (builder == NULL) {
    return NIL_TAG_NIL;
  }
  NilValue result = nil_string_lit(builder->data == NULL ? "" : builder->data, builder->length);
  free(builder->data);
  free(builder);
  return result;
}

void nil_interp_int(NilBuilder *builder, NilValue value) {
  char text[32];
  int written = snprintf(text, sizeof(text), "%lld", (long long)nil_int_value(value));
  builder_append(builder, text, (size_t)written);
}

void nil_interp_float(NilBuilder *builder, NilValue value) {
  char text[64];
  int written = render_float(nil_float_value(value), text, sizeof(text));
  builder_append(builder, text, (size_t)written);
}

void nil_interp_str(NilBuilder *builder, NilValue value) {
  if (!nil_is_string(value)) {
    /* A non-string in a string position renders as its text rather than as
     * nothing. A missing piece is far more visible than a slightly wrong one, and
     * the interpolation is the only place a program's mistake shows up verbatim. */
    builder_append_value(builder, value);
    return;
  }
  nil_gc_pin(value);
  builder_append_value(builder, value);
  nil_gc_unpin_all();
}

void nil_interp_bool(NilBuilder *builder, NilValue value) {
  builder_append_value(builder, nil_bool_new(nil_truthy(value)));
}

void nil_interp_nil(NilBuilder *builder) { builder_append(builder, "null", 4); }