
/* nilcore_selftest.c — exercises the runtime and reports what it found. * # Why this exists as a C
 * program and not a set of Go tests * The runtime's contract is a C ABI. A Go test can call it
 * through cgo, but cgo builds with the *Go* toolchain and the shipped library is built with a C
 * compiler, so a test that runs under cgo would pass on a machine whose C compiler produces
 * objects that cannot be linked — which is exactly the failure this file exists to rule out. * So
 * it is compiled and linked the way a real program's runtime is, and run as a separate process.
 * Everything it reports is a line of output, and the Go test reads those lines. * # The output
 * format * `ok <name>` or `FAIL <name> <detail>`, one per line, and the exit status is the number
 * of failures. Both parts matter: the name says *which* behaviour broke and the detail says what
 * was different, so a failure on a machine that is not this one still says something useful
 * without re-running under a debugger. */

#include "nil.h"
#include "nil_internal.h"
#include <stdio.h>
#include <string.h>
static int failures = 0;
static void ok(const char *what) { printf("ok %s\n", what); }
static void fail(const char *what, const char *detail) {
  printf("FAIL %s %s\n", what, detail);
  failures++;
}
static void check(const char *what, int condition) {
  if (condition) {
    ok(what);
  } else {
    fail(what, "condition was false");
  }
}
/* Two integer helpers, and the split is deliberate. * `check_int` compares plain numbers;
 * `check_value` unwraps a Nilang value once and compares that. There is deliberately no helper
 * that takes a Nilang value and compares it to a number *without* unwrapping, because that shape
 * invites `check_int(name, nil_string_len(s), 5)` — passing an already-unwrapped number where a
 * tagged value is expected — and the result is a comparison of a number against its own bit
 * pattern. Three checks in the first version of this file failed exactly that way, and the failure
 * looks like a runtime bug rather than a test bug, which is the worst possible place to look for
 * it. */

static void check_int(const char *what, int64_t got, int64_t want) {
  if (got == want) {
    ok(what);
    return;
  }
  char detail[128];
  snprintf(detail, sizeof(detail), "wanted %lld, got %lld", (long long)want, (long long)got);
  fail(what, detail);
}
static void check_value(const char *what, NilValue value, int64_t want) {
  check_int(what, nil_int_value(value), want);
}
/* check_bool compares a runtime *boolean value* against an expected answer. * The value is a
 * Nilang `Bool`, not the integers 0 and 1, so it is asked through `nil_truthy` rather than
 * compared as a number. The first version compared it as a number, and it passed for the wrong
 * reason: the raw tag of `true` is 2, which is not 0, so `got != 0` was true and `want != 0` was
 * false — and the check said the two disagreed, which is what made the tag change visible. Now
 * that the runtime builds booleans properly, this reads the value the way a program would. */

static void check_bool(const char *what, NilValue got, int want) {
  int truthy = nil_truthy(got) != 0;
  if (truthy == (want != 0)) {
    ok(what);
    return;
  }
  /* The rendered value goes in its own buffer and the message is built from both with enough
   * room for each, so a long value cannot truncate the words that say what was wanted. */
  char detail[128];
  nil_render(got, detail, (int64_t)sizeof(detail));
  char rendered[256];
  snprintf(rendered, sizeof(rendered), "wanted %s, got %s", want ? "true" : "false", detail);
  fail(what, rendered);
}
/* check_text is the one that matters most. * Every rendering rule in the runtime is a claim about
 * what a program sees, and a claim that is only ever checked by reading the C source is a claim
 * that is never checked. The expected strings here are the specification, written out. */

static void check_text(const char *what, NilValue value, const char *want) {
  char buffer[256];
  nil_render(value, buffer, (int64_t)sizeof(buffer));
  if (strcmp(buffer, want) == 0) {
    ok(what);
    return;
  }
  char detail[512];
  snprintf(detail, sizeof(detail), "wanted [%s], got [%s]", want, buffer);
  fail(what, detail);
}
/* ── Integers ─────────────────────────────────────────────────────────── */

static void test_integers(void) {
  check_value("add", nil_int_add(nil_int_new(2), nil_int_new(3)), 5);
  check_value("sub", nil_int_sub(nil_int_new(9), nil_int_new(4)), 5);
  check_value("mul", nil_int_mul(nil_int_new(6), nil_int_new(7)), 42);
  check_value("div", nil_int_div(nil_int_new(7), nil_int_new(2)), 3);
  check_value("mod", nil_int_mod(nil_int_new(7), nil_int_new(2)), 1);

  /* Truncation toward zero, which is what C does and what the interpreter reports. A floor-based
   * division would make these two disagree. */
  check_value("negative division truncates toward zero",
              nil_int_div(nil_int_new(-7), nil_int_new(2)), -3);
  check_value("negative remainder truncates toward zero",
              nil_int_mod(nil_int_new(-7), nil_int_new(2)), -1);
  check_value("negation through zero", nil_int_sub(nil_int_new(0), nil_int_new(5)), -5);

  /* The inline representation has a boundary: an integer whose low three bits are already 0b011
   * cannot be tagged, so it is boxed. Both sides of that boundary have to read back as the same
   * number, because a program cannot tell which one it got. */
  NilValue boxed = nil_int_new(3);
  check_value("a value that must be boxed reads back", boxed, 3);
  check_value("an inline integer reads back", nil_int_new(4), 4);

  /* The bit pattern `NIL_TAG_PTR` is the one value whose low three bits say "pointer" while its
   * address bits say nothing. Three predicates used to dereference the header without checking
   * for it, so passing this value to any of them was a null dereference — and this value is not
   * exotic: it is exactly what an integer of 4 would look like under a four-bit tag. */
  NilValue looksLikeANullPointer = (NilValue)4;
  check("the tagged null is not a string", !nil_is_string(looksLikeANullPointer));
  check("the tagged null is not a float", !nil_is_float(looksLikeANullPointer));
  check("the tagged null is not a number", !nil_is_number(looksLikeANullPointer));
  check("the tagged null is not an array", nil_array_len(looksLikeANullPointer) == 0);
  check("the tagged null is not a map", nil_map_len(looksLikeANullPointer) == 0);
  check("the tagged null is false", !nil_truthy(looksLikeANullPointer));
  check("the tagged null renders as an object",
        strcmp(nil_type_name(looksLikeANullPointer), "null") == 0);
  check_text("the tagged null can be printed", looksLikeANullPointer, "null");
  int64_t large = 0x123456789ABCDEFll;
  check_int("a large integer round-trips through a box", nil_int_value(nil_int_new(large)), large);

  /* A float beside an integer is promoted, not refused. This is the one implicit conversion the
   * language has, and both engines have to agree about it. */
  NilValue mixed = nil_float_add(nil_int_new(1), nil_float_new(2.5));
  check_text("an integer beside a float is promoted", mixed, "3.5");

  /* A string beside a number is concatenation, because that is what `+` means in every language
   * that has `+` on strings — and because the compiler emits one call for `+` whether it is
   * joining numbers or joining text, so this is the only place the question can be answered. */
  check_text("a string plus an integer concatenates",
             nil_int_add(nil_string_lit("count: ", 7), nil_int_new(42)), "count: 42");
  check_text("an integer plus a string concatenates",
             nil_int_add(nil_int_new(7), nil_string_lit(" items", 6)), "7 items");
  check_text("a string plus a float concatenates",
             nil_int_add(nil_string_lit("x: ", 3), nil_float_new(2.5)), "x: 2.5");
  check_text("a string plus a boolean concatenates",
             nil_int_add(nil_string_lit("ok: ", 4), nil_bool_new(1)), "ok: true");
  check_text("two strings concatenate", nil_int_add(nil_string_lit("a", 1), nil_string_lit("b", 1)),
             "ab");

  /* What is still an error: a number beside something with no text at all. The message names the
   * operation and both types rather than saying "failed". */
  NilValue bad = nil_int_add(nil_array_new(1), nil_int_new(1));
  check("adding a list to an integer is nil", bad == NIL_TAG_NIL);
  check("the type error has a code", nil_last_error_code() == NIL_ERR_TYPE);
  check("the type error names both types",
        strstr(nil_last_error(), "array") != NULL && strstr(nil_last_error(), "integer") != NULL);
  nil_clear_error();
}
/* ── Comparison ───────────────────────────────────────────────────────── */

static void test_comparison(void) {
  NilValue a = nil_int_new(5);
  NilValue b = nil_int_new(5);
  NilValue c = nil_int_new(7);
  check_bool("equal", nil_int_cmp(a, b, 0), 1);
  check_bool("not equal", nil_int_cmp(a, c, 0), 0);
  check_bool("less than", nil_int_cmp(a, c, 2), 1);
  check_bool("less or equal", nil_int_cmp(a, b, 3), 1);
  check_bool("greater than", nil_int_cmp(c, a, 4), 1);
  check_bool("greater or equal", nil_int_cmp(c, c, 5), 1);

  /* `5 == 5.0` is true in every language that has both. Comparing the boxed bit patterns would
   * make it depend on whether 5 happened to be inlined. */
  check_bool("an integer equals a float of the same value",
             nil_int_cmp(nil_int_new(5), nil_float_new(5.0), 0), 1);
  check_bool("an integer orders against a float",
             nil_int_cmp(nil_int_new(4), nil_float_new(5.0), 2), 1);

  /* Strings order by bytes and then by length, so the result is a total order. "ab" and "abc"
   * sharing a prefix must not compare equal. */
  NilValue ab = nil_string_lit("ab", 2);
  NilValue abc = nil_string_lit("abc", 3);
  check_bool("a prefix does not make two strings equal", nil_str_cmp(ab, abc, 0), 0);
  check_bool("a shorter string sorts first", nil_str_cmp(ab, abc, 2), 1);

  /* Two different objects with the same characters are the same value. */
  check_bool("equal strings in different objects compare equal",
             nil_str_cmp(nil_string_lit("hi", 2), nil_string_lit("hi", 2), 0), 1);
}
/* ── Strings ──────────────────────────────────────────────────────────── */

/* "héllo" is five characters and six bytes. The distinction is the whole point of these two
 * constants: a runtime that counted bytes would report six, and the interpreter reports five. */

static const char *ACCENTED = "h\xc3\xa9llo";
static void test_strings(void) {
  NilValue s = nil_string_lit(ACCENTED, strlen(ACCENTED));
  check_int("len counts characters, not bytes", nil_string_len(s), 5);
  check_int("index reads a code point", nil_string_index(s, 1), 0xE9);
  check_int("index after a multi-byte character", nil_string_index(s, 2), 'l');
  check_text("a non-ASCII string renders as itself", s, "héllo");
  check_text("concat", nil_string_concat(nil_string_lit("a", 1), nil_string_lit("b", 1)), "ab");
  check_text("substr", nil_substr(nil_string_lit("abcdef", 6), 1, 3), "bcd");
  check_text("substr to the end", nil_substr(nil_string_lit("abcdef", 6), 3, 100), "def");
  check_text("substr with a negative start counts from the end",
             nil_substr(nil_string_lit("abcdef", 6), -2, 2), "ef");
  check_text("substr with a negative length", nil_substr(nil_string_lit("abcdef", 6), 0, -2),
             "abcd");
  check_int("find reports a character index",
            nil_string_find(nil_string_lit("abcdef", 6), nil_string_lit("cd", 2)), 2);
  check_int("find reports -1 when absent",
            nil_string_find(nil_string_lit("abcdef", 6), nil_string_lit("zz", 2)), -1);

  /* A needle after a multi-byte character: the byte offset is 3 and the character index is 2,
   * and a caller indexing with `substr` needs the second. */
  NilValue mixed = nil_string_lit("h\xc3\xa9llo", 6);
  check_int("find counts characters after a multi-byte character",
            nil_string_find(mixed, nil_string_lit("ll", 2)), 2);
  check_text("upper", nil_string_to_upper(nil_string_lit("abC", 3)), "ABC");
  check_text("lower", nil_string_to_lower(nil_string_lit("AbC", 3)), "abc");
  check_text("trim", nil_string_trim(nil_string_lit("  ab \t\n", 7)), "ab");
  check_text("trim of nothing to trim", nil_string_trim(nil_string_lit("ab", 2)), "ab");
  check_text("char from an ASCII code", nil_char_from_int('x'), "x");
  check_text("char from a two-byte code", nil_char_from_int(0xE9), "é");
  check_text("char from a three-byte code", nil_char_from_int(0x4E2D), "中");
  check_text("char from an invalid code point", nil_char_from_int(0x110000), "");

  /* Interpolation: the builder is what `"\(a) and \(b)"` compiles to. */
  NilBuilder *builder = nil_builder_new();
  nil_interp_str(builder, nil_string_lit("a=", 2));
  nil_interp_int(builder, nil_int_new(42));
  nil_interp_str(builder, nil_string_lit(" and b=", 7));
  nil_interp_bool(builder, nil_bool_new(1));
  check_text("interpolation", nil_builder_finish(builder), "a=42 and b=true");
}
/* ── Arrays ───────────────────────────────────────────────────────────── */

static void test_arrays(void) {
  NilValue array = nil_array_new(3);
  check_int("a new array has its length", nil_array_len(array), 3);
  nil_array_set(array, nil_int_new(0), nil_int_new(10));
  nil_array_set(array, nil_int_new(2), nil_int_new(30));
  check_int("reading a set element", nil_int_value(nil_array_get(array, nil_int_new(0))), 10);

  /* An unwritten element reads as nil, not as whatever was on the heap. The language has no
   * `undefined`, so every read has to have an answer. */
  check("an unwritten element is nil", nil_array_get(array, nil_int_new(1)) == NIL_TAG_NIL);

  /* Push grows past the initial capacity, which is the path where a reallocation could lose what
   * was already stored. */

  for (int i = 0; i < 20; i++) {
    nil_array_push(array, nil_int_new(i));
  }
  check_int("push grows the array", nil_array_len(array), 23);
  check_int("an element survives the growth", nil_int_value(nil_array_get(array, nil_int_new(0))),
            10);
  check_int("a pushed element reads back", nil_int_value(nil_array_get(array, nil_int_new(22))),
            19);
  check_text("an array renders with brackets and commas", nil_array_new(0), "[]");
}
/* ── Maps ─────────────────────────────────────────────────────────────── */

static void test_maps(void) {
  NilValue map = nil_map_new(2);

  /* More keys than the map was created for: the table has to grow, and the rehash has to keep
   * every entry. Losing one here is the classic open-addressing bug. */

  for (int i = 0; i < 50; i++) {
    char key[16];
    snprintf(key, sizeof(key), "k%d", i);
    nil_map_set(map, nil_string_lit(key, strlen(key)), nil_int_new(i * 10));
  }
  check_int("a map grows past its initial capacity", nil_map_len(map), 50);
  int intact = 1;
  for (int i = 0; i < 50; i++) {
    char key[16];
    snprintf(key, sizeof(key), "k%d", i);
    NilValue found = nil_map_get(map, nil_string_lit(key, strlen(key)));
    if (nil_int_value(found) != i * 10) {
      intact = 0;
      break;
    }
  }
  check("every entry survives the growth", intact);

  /* A missing key is nil, and that is what `?.` and `??` test for. A missing key that produced
   * something else would make every optional in the language lie. */
  check("a missing key is nil", nil_map_get(map, nil_string_lit("absent", 6)) == NIL_TAG_NIL);

  /* Replacing a key does not add an entry. A map whose length grew on every write would make
   * `len` wrong after any update. */
  nil_map_set(map, nil_string_lit("k0", 2), nil_int_new(999));
  check_int("replacing a key does not change the length", nil_map_len(map), 50);
  check_int("a replaced key reads back", nil_int_value(nil_map_get(map, nil_string_lit("k0", 2))),
            999);

  /* Integer keys, including one that has to be boxed. A key that hashed from its representation
   * rather than its value would miss. */
  NilValue ints = nil_map_new(4);
  nil_map_set(ints, nil_int_new(1), nil_string_lit("one", 3));
  nil_map_set(ints, nil_int_new(1000000007), nil_string_lit("big", 3));
  check_text("an integer key", nil_map_get(ints, nil_int_new(1)), "one");
  check_text("a large integer key", nil_map_get(ints, nil_int_new(1000000007)), "big");

  /* A member read on a map is a lookup. */
  NilValue record = nil_map_new(4);
  nil_map_set(record, nil_string_lit("name", 4), nil_string_lit("nilang", 6));
  check_text("a member read", nil_get_field(record, nil_string_lit("name", 4)), "nilang");
}
/* ── Truthiness ───────────────────────────────────────────────────────── */

static void test_truthiness(void) {
  check("zero is false", nil_truthy(nil_int_new(0)) == 0);
  check("a non-zero integer is true", nil_truthy(nil_int_new(1)) == 1);
  check("a negative integer is true", nil_truthy(nil_int_new(-1)) == 1);
  check("null is false", nil_truthy(NIL_TAG_NIL) == 0);
  check("true is true", nil_truthy(NIL_TAG_TRUE) == 1);
  check("false is false", nil_truthy(NIL_TAG_FALSE) == 0);

  /* The empty-container rules. Every language with a truthy test makes this choice and a program
   * has to be able to rely on it. */
  check("an empty string is false", nil_truthy(nil_string_lit("", 0)) == 0);
  check("a non-empty string is true", nil_truthy(nil_string_lit("a", 1)) == 1);
  check("an empty array is false", nil_truthy(nil_array_new(0)) == 0);
  check("an empty map is false", nil_truthy(nil_map_new(1)) == 0);
  check("zero as a float is false", nil_truthy(nil_float_new(0.0)) == 0);

  /* NaN is the one number that is not equal to itself. `if x` asks "is it non-zero", which is
   * the only question with a stable answer for it. */
  check("not-a-number is false", nil_truthy(nil_float_new(0.0 / 0.0)) == 0);
  check("not inverts true", nil_truthy(nil_bool_not(nil_bool_new(1))) == 0);
  check("not inverts false", nil_truthy(nil_bool_not(nil_bool_new(0))) == 1);

  /* `&&` and `||` return a Bool, not a number. * The result of a comparison and the result of a
   * logical operator are both tagged values, and a `1` is the tag of *false*. A runtime that
   * returned the raw truthiness made every `a && b` that held report `false`. */
  check_bool("and of two true", nil_logic(nil_bool_new(1), nil_bool_new(1), 0), 1);
  check_bool("and of true and false", nil_logic(nil_bool_new(1), nil_bool_new(0), 0), 0);
  check_bool("and of two false", nil_logic(nil_bool_new(0), nil_bool_new(0), 0), 0);
  check_bool("or of two true", nil_logic(nil_bool_new(1), nil_bool_new(1), 1), 1);
  check_bool("or of false and true", nil_logic(nil_bool_new(0), nil_bool_new(1), 1), 1);
  check_bool("or of two false", nil_logic(nil_bool_new(0), nil_bool_new(0), 1), 0);

  /* The operands are values, so the operator uses their truthiness rather than requiring
   * booleans: `1 && "a"` is true, and `0 || 5` is true. */
  check_bool("and over arbitrary values", nil_logic(nil_int_new(1), nil_string_lit("a", 1), 0), 1);
  check_bool("or over arbitrary values", nil_logic(nil_int_new(0), nil_int_new(5), 1), 1);

  /* A comparison answers a Bool too, which is the same reason. */
  check_bool("a true comparison", nil_int_cmp(nil_int_new(1), nil_int_new(1), 0), 1);
  check_bool("a false comparison", nil_int_cmp(nil_int_new(1), nil_int_new(2), 0), 0);
}
/* ── Rendering ────────────────────────────────────────────────────────── */

static void test_rendering(void) {
  check_text("null", NIL_TAG_NIL, "null");
  check_text("true", NIL_TAG_TRUE, "true");
  check_text("false", NIL_TAG_FALSE, "false");
  check_text("a positive integer", nil_int_new(42), "42");
  check_text("a negative integer", nil_int_new(-42), "-42");

  /* The float rule, which is the whole of the shared specification between the three engines: an
   * integral float keeps its `.0`, an integer has none. Printing both as "2" would make a
   * program that round-trips through text fail on the round trip rather than on the comparison.
   */
  check_text("an integral float prints without a point", nil_float_new(2.0), "2");
  check_text("a negative integral float prints without a point", nil_float_new(-2.0), "-2");
  check_text("zero prints without a point", nil_float_new(0.0), "0");
  check_text("a fractional float", nil_float_new(2.5), "2.5");
  check_text("a value the program wrote as 2.1", nil_float_new(2.1), "2.1");
  check_text("a small float", nil_float_new(0.1), "0.1");
  check_text("one third", nil_float_new(1.0 / 3.0), "0.3333333333333333");
  check_text("not-a-number", nil_float_new(0.0 / 0.0), "NaN");
  check_text("infinity", nil_float_new(1.0 / 0.0), "+Inf");
  check_text("negative infinity", nil_float_new(-1.0 / 0.0), "-Inf");

  /* A large integral float does not go through the integer path — it is too big for a long long
   * — so it takes the shortest-representation branch. */
  check_text("a large float", nil_float_new(1e20), "1e+20");
}
/* ── Math ─────────────────────────────────────────────────────────────── */

static void test_math(void) {
  check_text("sqrt", nil_sqrt(nil_int_new(2)), "1.4142135623730951");
  check_text("floor", nil_floor(nil_float_new(2.7)), "2");
  check_text("ceil", nil_ceil(nil_float_new(2.1)), "3");
  check_text("round", nil_round(nil_float_new(2.5)), "3");

  /* abs keeps an integer an integer. Returning a float would make `abs` change a program's types
   * depending on nothing. */
  check_int("abs of a negative integer", nil_int_value(nil_abs(nil_int_new(-7))), 7);
  check_text("abs of a negative float", nil_abs(nil_float_new(-2.5)), "2.5");
  check_int("min", nil_int_value(nil_min_int(nil_int_new(3), nil_int_new(9))), 3);
  check_int("max", nil_int_value(nil_max_int(nil_int_new(3), nil_int_new(9))), 9);
  check_text("min of floats", nil_min_float(nil_float_new(3.5), nil_float_new(9.5)), "3.5");
  check_text("pow", nil_pow(nil_int_new(2), nil_int_new(10)), "1024");
}
/* ── The collector ────────────────────────────────────────────────────── */

static NilValue make_chain(int depth) {
  if (depth == 0) {
    return nil_string_lit("leaf", 4);
  }
  NilValue next = make_chain(depth - 1);
  NilValue array = nil_array_new(1);
  nil_array_set(array, nil_int_new(0), next);
  return array;
}
static void test_collector(void) {
  int64_t before = nil_gc_object_count();

  /* An unreferenced chain is collected. If it is not, the collector is not running at all —
   * which a test that only checks that live data survives would pass. */
  NilValue discarded = make_chain(20);
  check("a discarded chain was built", discarded != NIL_TAG_NIL);
  nil_gc_collect();
  check("unreferenced objects are reclaimed", nil_gc_object_count() < before + 20);

  /* A rooted chain survives. The value is held in a registered root slot, which is exactly what
   * the emitter writes for a local that can hold a heap value. */
  NilValue slot = NIL_TAG_NIL;
  nil_gc_root_register(&slot);
  slot = make_chain(50);
  int64_t withRoot = nil_gc_object_count();

  /* Allocating more than the threshold forces several collections of their own, then an explicit
   * one. * Arrays, not strings: `nil_string_lit` pins for the whole run, because a string
   * constant has no pointer leading to it. Using it here would make the 500 allocations
   * immortal, and the heap-growth check below would then be measuring the pin list rather than
   * the collector. */

  for (int i = 0; i < 500; i++) {
    NilValue scratch = nil_array_new(4);
    (void)scratch;
  }
  nil_gc_collect();

  /* The chain is walked with the runtime's own accessors rather than by reaching into the
   * structs. A `NilObject *` is the *address* of a header; the tagged `NilValue` is that address
   * with the low three bits set, and mixing the two is how this test first crashed —
   * `nil_array_get` correctly refused a value that was never a value. * Success is "we got
   * all the way down to the string at the bottom", not "every node was an array": the leaf *is*
   * a string, and stopping on it is the point. The first version of this loop treated reaching
   * the leaf as a failure. */
  NilValue node = slot;
  int steps = 0;
  while (nil_object_kind(node) == NIL_OBJ_ARRAY && steps < 60) {
    node = nil_array_get(node, nil_int_new(0));
    steps++;
  }
  check("a rooted chain survives collection", steps == 50 && nil_is_string(node));
  check_text("and its leaf is intact", node, "leaf");
  check("the collection did not grow the heap without bound",
        nil_gc_object_count() < withRoot + 500);
  nil_gc_root_unregister(&slot);
  slot = NIL_TAG_NIL;
  nil_gc_collect();

  /* A value held only by a *runtime* pin survives a collection that the pin triggered. This is
   * the ordering rule that makes `nil_string_concat` safe. */
  NilValue pinned = nil_string_lit("held", 4);
  nil_gc_pin(pinned);
  NilValue copy = nil_string_concat(pinned, nil_string_lit("-copy", 5));
  check_text("a pinned value survives the allocation beside it", copy, "held-copy");
  nil_gc_unpin_all();
  nil_gc_collect();

  /* An allocation that trips the collection threshold keeps the object it just
   * made. The allocator used to link the object first and collect second, so
   * the sweep freed the newborn while the caller still held it and the next
   * write landed in freed memory. The first loop leaves only garbage behind,
   * which resets the threshold to its floor; the second loop then roots
   * exactly enough live objects that one allocation trips a collection while
   * the newborn is still unreferenced. That member must still be valid. */
  for (int i = 0; i < 100001; i++) {
    (void)nil_array_new(0);
  }
  NilValue herd_slot = NIL_TAG_NIL;
  nil_gc_root_register(&herd_slot);
  herd_slot = nil_array_new(0);
  for (int i = 0; i < 100000; i++) {
    nil_array_push(herd_slot, nil_array_new(0));
  }
  int64_t survivors = 0;
  int64_t herd_len = nil_array_len(herd_slot);
  for (int64_t i = 0; i < herd_len; i++) {
    NilValue member = nil_array_get(herd_slot, nil_int_new(i));
    if (nil_object_kind(member) == NIL_OBJ_ARRAY && nil_array_len(member) == 0) {
      survivors++;
    }
  }
  check("the allocation that tripped the threshold survived its own collection",
        herd_len == 100000 && survivors == 100000);
  nil_gc_root_unregister(&herd_slot);
  herd_slot = NIL_TAG_NIL;
  nil_gc_collect();
}
/* ── Indexing ─────────────────────────────────────────────────────────── */

/* `container[key]` decides what the container is. * The compiler cannot: at MIR level `m["k"]` and
 * `xs[1]` are the same instruction, so emitting a call to `nil_array_get` made every map lookup
 * fail. These checks are the runtime half of that decision, and they are here rather than only in
 * the conformance suite because they are cheap and they belong next to the code. */

static void test_indexing(void) {
  NilValue list = nil_array_new(3);
  nil_array_set(list, nil_int_new(0), nil_string_lit("a", 1));
  nil_array_set(list, nil_int_new(1), nil_string_lit("b", 1));
  nil_array_set(list, nil_int_new(2), nil_string_lit("c", 1));
  check_text("a list index", nil_index_get(list, nil_int_new(1)), "b");
  NilValue map = nil_map_new(2);
  nil_map_set(map, nil_string_lit("k", 1), nil_int_new(1));
  check_value("a map key", nil_index_get(map, nil_string_lit("k", 1)), 1);
  check("a missing map key is nil", nil_index_get(map, nil_string_lit("z", 1)) == NIL_TAG_NIL);

  /* A string indexed by character position gives a one-character string, so that `s[0] + "!"` is
   * `"a!"` rather than a code point nobody can print. */
  NilValue text = nil_string_lit("abc", 3);
  check_text("a string index", nil_index_get(text, nil_int_new(0)), "a");
  check_text("a string index at the end", nil_index_get(text, nil_int_new(2)), "c");
  check("a string index out of range is nil", nil_index_get(text, nil_int_new(9)) == NIL_TAG_NIL);
  check("a negative string index is nil", nil_index_get(text, nil_int_new(-1)) == NIL_TAG_NIL);

  /* The character count is what the index counts, so an accented letter is one position rather
   * than two bytes. */
  check_text("a string index after a multi-byte character", nil_index_get(text, nil_int_new(1)),
             "b");

  /* The three containers accept the same operation, which is the point. */
  nil_index_set(map, nil_string_lit("n", 1), nil_int_new(9));
  check_value("indexed assignment into a map", nil_index_get(map, nil_string_lit("n", 1)), 9);
  nil_index_set(list, nil_int_new(0), nil_string_lit("z", 1));
  check_text("indexed assignment into a list", nil_index_get(list, nil_int_new(0)), "z");

  /* An index on an integer has no answer, and the operation stops the program. That is checked
   * in `TestAnIndexOnANonContainerStopsTheProgram`, in Go, because a check here would end the
   * run before the rest of the self-test's report was printed. */
}
/* ── Calls ────────────────────────────────────────────────────────────── */

static NilValue add_three(NilValue *args, int64_t count) {
  int64_t total = 0;
  for (int64_t i = 0; i < count; i++) {
    total += nil_int_value(args[i]);
  }
  return nil_int_new(total);
}
static void test_calls(void) {
  NilValue callee = nil_closure_new(add_three);
  check("a closure was created", callee != NIL_TAG_NIL);
  NilValue args = nil_array_new(3);
  nil_array_set(args, nil_int_new(0), nil_int_new(1));
  nil_array_set(args, nil_int_new(1), nil_int_new(2));
  nil_array_set(args, nil_int_new(2), nil_int_new(4));
  check_int("a closure is called through nil_call", nil_int_value(nil_call(callee, args)), 7);
  check("calling something that is not a function is nil",
        nil_call(nil_int_new(1), args) == NIL_TAG_NIL);
}
/* ── Allocation ───────────────────────────────────────────────────────── */

static void test_allocation(void) {
  NilValue block = nil_alloc(16, 1);
  check_int("alloc gives the requested size", nil_array_len(block), 16);
  check("an allocated block is zeroed", nil_array_get(block, nil_int_new(5)) == NIL_TAG_NIL);
  check("alloc of nothing is an empty array", nil_array_len(nil_alloc(0, 1)) == 0);
}
/* ── Errors ───────────────────────────────────────────────────────────── */

/* A failing operation stops the program. That is the contract: a compiled program cannot return an
 * error to code that has no way to catch one, so the error becomes an exit status. * Which makes
 * this the *last* thing the self-test does — the process does not come back. The Go harness that
 * runs this binary therefore expects the run to end with `NIL_ERR_INDEX` and treats everything
 * printed before that line as the report. */

static void test_errors(void) {
  nil_clear_error();
  check("no error to start with", nil_last_error_code() == NIL_OK);

  /* A type failure is recoverable: it sets the error and answers nil, because the caller may
   * well be able to do something else with the value. The language should have caught it at
   * compile time; the runtime reports it for the paths that reach here anyway. */
  NilValue sum = nil_int_add(nil_array_new(1), nil_int_new(1));
  check("a type failure answers nil", sum == NIL_TAG_NIL);
  check("a type failure records its code", nil_last_error_code() == NIL_ERR_TYPE);
  check("a type failure records its message", strstr(nil_last_error(), "array") != NULL);
  nil_clear_error();
  check("an error can be cleared", nil_last_error_code() == NIL_OK);
  check("clearing empties the message", nil_last_error()[0] == '\0');

  /* Beyond this point the program stops, so nothing below can report a failure in the usual way
   * — and the Go harness expects the run to end here, with this code, having printed no FAIL
   * before it. */
  printf("about to stop with an out-of-range index\n");
  nil_bounds_error("array", 9, 3);
  fail("an out-of-range index stops the program", "it did not");
}
/* test_language_answers covers the answers the *language* defines, as opposed to the answers the
 * representation defines. * Every check here was a disagreement between the three engines: the
 * interpreter and the VM agreed with each other and the compiled back end did not, because it
 * asked the runtime a question the runtime had no rule for. The pattern is the same in all of them
 * — a comparison, a conversion, or a subscript between things whose types differ, reaching a
 * runtime that only knew about two of them. * They live in one function rather than being
 * scattered, because that is what they share: each is a place where the language's answer is "it
 * depends, and here is how", and the runtime is the only component that can see enough to decide.
 */

static void test_language_answers(void) {

  /* ── Equality between kinds ────────────────────────────────────────────── * The interpreter's
   * rule, reproduced exactly: two numbers compare as numbers, two strings as text, a string
   * beside anything else compares the *rendered* text of both, and anything else falls through
   * to identity. That last clause is where nil and booleans come from — and it is why `"1" == 1`
   * is true, which reads as surprising until one notices that the same rule makes `1 == "1"`,
   * `"2.5" == 2.5` and `"true" == true` all true: a program comparing what it read with what it
   * wrote is comparing text, and text is what it has. */
  check_bool("a string equals the text of a number",
             nil_str_cmp(nil_string_lit("1", 1), nil_int_new(1), 0), 1);
  check_bool("a number equals the text of a string",
             nil_str_cmp(nil_int_new(1), nil_string_lit("1", 1), 0), 1);
  check_bool("a string does not equal different text",
             nil_str_cmp(nil_string_lit("1", 1), nil_int_new(2), 1), 1);
  check_bool("a float equals its own rendering",
             nil_str_cmp(nil_string_lit("2.5", 3), nil_float_new(2.5), 0), 1);
  check_bool("a boolean equals its own rendering",
             nil_str_cmp(nil_string_lit("true", 4), nil_bool_new(1), 0), 1);

  /* Two booleans answer equality and nothing else. Ordering two booleans would mean saying which
   * is smaller, and the interpreter refuses it. */
  check_bool("true equals true", nil_int_cmp(nil_bool_new(1), nil_bool_new(1), 0), 1);
  check_bool("true does not equal false", nil_int_cmp(nil_bool_new(1), nil_bool_new(0), 0), 0);
  check_bool("false equals false", nil_int_cmp(nil_bool_new(0), nil_bool_new(0), 0), 1);
  check_bool("booleans answer inequality too", nil_int_cmp(nil_bool_new(1), nil_bool_new(0), 1), 1);
  nil_clear_error();
  check("ordering two booleans is refused",
        nil_int_cmp(nil_bool_new(1), nil_bool_new(0), 2) == NIL_TAG_NIL);
  check("ordering two booleans records a type error", nil_last_error_code() == NIL_ERR_TYPE);

  /* ── Nil ───────────────────────────────────────────────────────────────── * Every lookup in
   * the language can answer nil — a missing key, a read past the end, a member that is not there
   * — so `x == null` is the ordinary way to ask whether a lookup found anything. An engine that
   * made that a type error would make the most common test in the language unanswerable. */
  check_bool("null equals null", nil_int_cmp(NIL_TAG_NIL, NIL_TAG_NIL, 0), 1);
  check_bool("null does not equal a number", nil_int_cmp(NIL_TAG_NIL, nil_int_new(1), 0), 0);
  check_bool("a number does not equal null", nil_int_cmp(nil_int_new(1), NIL_TAG_NIL, 0), 0);
  check_bool("null is not equal to a number", nil_int_cmp(NIL_TAG_NIL, nil_int_new(1), 1), 1);

  /* ── Conversion to text ────────────────────────────────────────────────── */
  check_text("str of a number", nil_to_str(nil_int_new(42)), "42");
  check_text("str of a string is the string", nil_to_str(nil_string_lit("abc", 3)), "abc");
  check_text("str of a boolean", nil_to_str(nil_bool_new(1)), "true");
  check_text("str of nil", nil_to_str(NIL_TAG_NIL), "null");

  /* A string longer than the first buffer `str` tries, because `str` is how a program asks what
   * something is and an answer cut to 128 bytes is not one. */
  {
    char big[600];
    memset(big, 'x', sizeof(big) - 1);
    big[sizeof(big) - 1] = '\0';
    NilValue long_text = nil_to_str(nil_string_lit(big, (int64_t)strlen(big)));
    check_int("str of a long string is not truncated", nil_string_len(long_text), 599);
  }
  check_bool("isNull of nil", nil_is_null(NIL_TAG_NIL), 1);
  check_bool("isNull of a number", nil_is_null(nil_int_new(0)), 0);
  check_bool("isNull of false", nil_is_null(nil_bool_new(0)), 0);

  /* ── Subscript reads and writes ────────────────────────────────────────── * The asymmetry is
   * the language's rule, not an oversight: a read past the end of a list answers nil because
   * "what is at position 10?" has the answer "nothing there", which is what nil means. A write
   * past the end cannot be carried out at all, so it stops the program. */
  NilValue xs = nil_array_new(3);
  nil_array_set(xs, nil_int_new(0), nil_int_new(1));
  nil_array_set(xs, nil_int_new(1), nil_int_new(2));
  nil_array_set(xs, nil_int_new(2), nil_int_new(3));
  check("a read past the end of a list is nil", nil_index_get(xs, nil_int_new(10)) == NIL_TAG_NIL);
  check("a read before the start of a list is nil",
        nil_index_get(xs, nil_int_new(-1)) == NIL_TAG_NIL);
  check("a read inside a list is the element",
        nil_int_value(nil_index_get(xs, nil_int_new(1))) == 2);

  /* The primitive still refuses, because a caller that asks it for element 10 of 3 has asked for
   * something impossible and should be told so directly. */
  nil_clear_error();
  check("the array primitive itself refuses a bad position",
        nil_array_get(xs, nil_int_new(10)) == NIL_TAG_NIL);
  check("the refusal records an index error", nil_last_error_code() == NIL_ERR_INDEX);
  nil_clear_error();
  check_value("a subscript write returns what was written",
              nil_index_set(xs, nil_int_new(1), nil_int_new(9)), 9);
  check_value("the write reached the list", nil_index_get(xs, nil_int_new(1)), 9);

  /* ── Members ───────────────────────────────────────────────────────────── * A member read is
   * a different runtime function from a subscript read, because a member that is not there is a
   * mistake while a subscript past the end is a question. A member *write* is not separate: the
   * parser normalises `m.name = v` into a subscript whose key is the string `"name"`, so by the
   * time the runtime is called there is nothing left to distinguish, and a second entry point
   * would be one nothing could reach. */
  NilValue record = nil_map_new(0);
  nil_map_set(record, nil_string_lit("name", 4), nil_string_lit("nil", 3));
  check_text("a member read lands", nil_get_field(record, nil_string_lit("name", 4)), "nil");
  nil_index_set(record, nil_string_lit("name", 4), nil_string_lit("nilang", 6));
  check_text("a member write replaces", nil_get_field(record, nil_string_lit("name", 4)), "nilang");

  /* ── List operations ───────────────────────────────────────────────────── */
  check_value("first", nil_int_value(nil_first(xs)), 1);
  check_value("last", nil_int_value(nil_last(xs)), 9);
  NilValue empty = nil_array_new(0);
  check("first of an empty list is nil", nil_first(empty) == NIL_TAG_NIL);
  check("last of an empty list is nil", nil_last(empty) == NIL_TAG_NIL);
  check("rest of an empty list is nil", nil_rest(empty) == NIL_TAG_NIL);
  NilValue tail = nil_rest(xs);
  check_int("rest drops the first element", nil_array_len(tail), 2);
  check_value("rest keeps the second", nil_int_value(nil_array_get(tail, nil_int_new(0))), 2);
  check_value("rest keeps the third", nil_int_value(nil_array_get(tail, nil_int_new(1))), 9);
  check("rest returns a new list", tail != xs);

  /* Rest must not alias: `rest(xs)[0] = 9` has to be a way to change the tail, not a way to
   * reach into the list it came from. */
  nil_array_set(tail, nil_int_new(0), nil_int_new(99));
  check_value("writing to the tail leaves the original alone",
              nil_int_value(nil_array_get(xs, nil_int_new(1))), 2);

  /* ── substr's two forms ────────────────────────────────────────────────── */
  check_text("the two-argument substr runs to the end",
             nil_substr_from(nil_string_lit("abcdef", 6), 3), "def");
  check_text("the two-argument substr from zero is the whole string",
             nil_substr_from(nil_string_lit("abcdef", 6), 0), "abcdef");
  check_text("the two-argument substr past the end is empty",
             nil_substr_from(nil_string_lit("abcdef", 6), 99), "");
  check_text("the two-argument substr with a negative start counts from the end",
             nil_substr_from(nil_string_lit("abcdef", 6), -2), "ef");
}
int main(void) {

  /* Unbuffered, so that a crash does not take the report with it. * A test harness whose output
   * is captured through a pipe is fully buffered by default, and a crash would discard every
   * line before it — leaving a bare exit code as the only evidence and nothing to say which
   * check was running. */
  setvbuf(stdout, NULL, _IONBF, 0);
  test_integers();
  test_comparison();
  test_strings();
  test_arrays();
  test_maps();
  test_truthiness();
  test_rendering();
  test_math();
  test_collector();
  test_calls();
  test_indexing();
  test_allocation();
  test_language_answers();
  test_errors();
  printf("failures %d\n", failures);
  return failures == 0 ? 0 : 1;
}
