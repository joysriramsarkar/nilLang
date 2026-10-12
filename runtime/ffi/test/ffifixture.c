/* ffifixture.c — a C library for the FFI tests to call.
 *
 * # Why a fixture rather than a real library
 *
 * The point of these tests is the *boundary*, not any particular C function. Every
 * one of them takes and returns `int64_t` so that the calling convention on this
 * side is the one the backend emits: a Nilang value is a tagged word in an `i64`,
 * and a C `int64_t` is an `i64`.
 *
 * Using a real library instead — `sin`, `strlen`, `abs` — would test two things at
 * once and fail in one of them for an unrelated reason. `int abs(int)` is declared
 * `i32` in C and `i64` here; a test that declared it faithfully would fail on the
 * signature rather than on anything about the FFI, and a test that declared it the
 * convenient way would pass while lying about the ABI.
 *
 * So these functions say exactly what the boundary is, and the tests that use them
 * say the same thing: a Nilang `Int` crosses as a tagged word, and a C `int64_t`
 * crosses as a plain number. Reconciling those two is L5.3's work, and this file is
 * the fixed point it works against.
 */

#include <stdint.h>

/* The identity, so a test can prove the value arrived rather than that the answer
 * happened to be right. */
int64_t nilffi_double(int64_t value) { return value * 2; }

/* Addition across two arguments, so a test can prove both arrived and in order. */
int64_t nilffi_add(int64_t a, int64_t b) { return a + b; }

/* A fixed answer, so a test can prove the call happened at all when the argument
 * path is under suspicion. */
int64_t nilffi_answer(void) { return 42; }

/* A large value, well past what 32 bits can hold. This is the test that would fail
 * if the boundary truncated rather than converted. */
int64_t nilffi_large(void) { return 8589934592LL; } /* 2^33 */

/* Negative, so sign extension is checked rather than assumed. */
int64_t nilffi_negative(void) { return -7; }
