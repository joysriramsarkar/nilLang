/* stringfixture.c — C that takes Nilang strings and answers with them.
 *
 * # Why this exists
 *
 * A string crosses as two values and comes back as two. Every way of getting that
 * wrong is silent: a length in characters where C wanted bytes gives a different
 * answer for any string outside ASCII, and a pointer with no length makes the empty
 * string and the end of a buffer the same value.
 *
 * So the fixture takes a string, measures it, and builds one — and the tests use a
 * string whose length in characters and length in bytes differ. A crossing that
 * confuses them answers the character count, and the test says which one it wanted.
 *
 * Every function here that *answers* with a string takes a `NilFFIString *out`,
 * because that is the one convention and a second one would be a second thing to
 * get wrong. See the ownership note in `nilffi.h` for why the buffers are `static`
 * rather than freed.
 */

#include <stdlib.h>
#include <string.h>

#include "nilffi.h"

/* Declared before it is defined because `nilffi_upper` and `nilffi_take` both fall
 * back to it, and a definition after its first use means an implicit declaration
 * that a compiler is right to object to — it returns `void` and the compiler has no
 * way to know that. */
void nilffi_nothing(NilFFIString *out);

/* `snprintf` is what a C author reaches for when the answer's length is only known
 * once it has been produced. It is here rather than written out again so that the
 * one thing `nil_ffi_buffer` has to get right — a buffer of exactly the length asked
 * for, with a terminator when there is room — is checked against the real thing. */
#include <stdio.h>

/* The byte length C sees for a string Nilang passed.
 *
 * `strlen` is deliberately not used: it stops at the first NUL, and a Nilang string
 * may contain one. Counting what was actually passed is what a C API that takes a
 * length does, and it is the answer the crossing is supposed to give. */
int64_t nilffi_byte_length(const char *bytes, size_t length) {
  (void)bytes;
  return (int64_t)length;
}

/* How many characters a UTF-8 string has.
 *
 * The counterpart to `nilffi_byte_length`, and the number Nilang's own `len`
 * answers. Counting the *starts* of UTF-8 sequences rather than the bytes that
 * are ASCII is what makes it a character count: "héllo" has five of them and six
 * bytes, and a fixture that counted ASCII bytes would answer four — a third
 * number, which is worse than no number at all because it looks plausible.
 *
 * Hand-written rather than borrowed from the runtime, so that the test's
 * expectation is not the implementation's own arithmetic agreeing with itself. */
int64_t nilffi_char_length(const char *bytes, size_t length) {
  int64_t count = 0;
  size_t index = 0;
  while (index < length) {
    unsigned char byte = (unsigned char)bytes[index];
    size_t width = 1;
    if ((byte & 0xE0) == 0xC0) {
      width = 2;
    } else if ((byte & 0xF0) == 0xE0) {
      width = 3;
    } else if ((byte & 0xF8) == 0xF0) {
      width = 4;
    }
    count++;
    index += width;
  }
  return count;
}

/* Answers a string whose length is only known while it is being built.
 *
 * This is the case a `static` buffer cannot serve and `malloc` serves badly: a
 * function that formats a number, or joins a variable number of pieces, has no
 * answer size until it has produced it. `malloc` would leak — the crossing copies
 * the bytes and nobody is left to free them — so the answer is built in a runtime
 * buffer that the crossing reclaims when the call returns.
 *
 * It also asks for a buffer it never uses, to show that a scope frees *every*
 * allocation in it and not just the one that became the answer. */
void nilffi_repeat(const char *bytes, size_t length, int64_t times, NilFFIString *out) {
  nilffi_nothing(out);
  if (bytes == NULL || length == 0 || times <= 0) {
    return;
  }

  /* A second buffer, deliberately discarded. If the scope reclaimed only what the
   * callee handed over, this would leak one block per call and the outstanding
   * count in the test would climb with every iteration. */
  char *scratch = nil_ffi_buffer((int64_t)length);
  if (scratch == NULL) {
    return;
  }
  memcpy(scratch, bytes, length);

  size_t total = length * (size_t)times;
  char *answer = nil_ffi_buffer((int64_t)total);
  if (answer == NULL) {
    return;
  }
  for (int64_t i = 0; i < times; i++) {
    memcpy(answer + (size_t)i * length, bytes, length);
  }

  out->bytes = answer;
  out->length = total;
}

/* Answers a string, having first let a whole Nilang program run.
 *
 * The nesting case, and the one that decides whether a buffer scope is a stack or
 * a free list. This function allocates its answer, calls a callback — which is a
 * complete Nilang program, free to make its own string-returning calls — and only
 * then reads its own buffer back.
 *
 * The inner program's string call opens a scope and closes it. If closing freed
 * "whatever is outstanding" rather than "whatever was allocated since my own open",
 * that inner close would take *this* function's buffer, and the `memcpy` below
 * would read freed memory — in a program that did everything correctly, which is
 * the only kind of memory bug worth designing against.
 *
 * So the answer is written into the outer buffer *after* the callback returns, and
 * its length is the length the outer buffer was allocated for. Both facts are
 * checked: a wrong length means the buffer was replaced, and wrong bytes mean it
 * was freed.
 *
 * The callback arrives as a `NilValue` handle rather than a `NilFFICallback`
 * pointer, because a bare function pointer carries no context and a callback has to
 * know which function it is. See `nilffi.h`. */
void nilffi_nested(NilValue callback, int64_t argument, NilFFIString *out) {
  const char *answer = "nested";
  size_t answerLength = 6;

  char *outer = nil_ffi_buffer((int64_t)answerLength);
  if (outer == NULL) {
    nilffi_nothing(out);
    return;
  }

  NilValue boxed = nil_int_box(argument);
  nil_callback_invoke(callback, &boxed, 1);

  /* Read the buffer the callback could not have taken. */
  memcpy(outer, answer, answerLength);
  out->bytes = outer;
  out->length = answerLength;
}

/* Answers a number formatted as text — the case the buffer exists for.
 *
 * The size is not known before `snprintf` runs, which is the whole reason a C
 * author would reach for `malloc` here and leak. The buffer is sized by asking once
 * with a null destination, which `snprintf` answers with the length it would have
 * written, and then again for real. */
void nilffi_format(int64_t number, NilFFIString *out) {
  char probe[32];
  int needed = snprintf(probe, sizeof(probe), "<%lld>", (long long)number);
  if (needed < 0) {
    nilffi_nothing(out);
    return;
  }

  char *answer = nil_ffi_buffer(needed);
  if (answer == NULL) {
    nilffi_nothing(out);
    return;
  }
  snprintf(answer, (size_t)needed + 1, "<%lld>", (long long)number);

  out->bytes = answer;
  out->length = (size_t)needed;
}

/* Answers a string the crossing must not touch.
 *
 * A pointer the runtime never allocated, from a table the callee owns for the life
 * of the process. The scope closes around this call and must leave it alone — a
 * scope that freed "whatever `out->bytes` is" rather than "whatever came out of
 * `nil_ffi_buffer`" would free a `static` and take the program down on the first
 * iteration of a loop. */
void nilffi_static(NilFFIString *out) {
  static const char answer[] = "constant";
  out->bytes = answer;
  out->length = sizeof(answer) - 1;
}

/* Answers with the very pointer it was handed.
 *
 * The other half of `nilffi_static`: a callee that passes through what it received
 * allocates nothing at all. The crossing copies it, the scope frees nothing, and
 * the program is right — which is the whole reason the scope tracks what it handed
 * out rather than what it was given. */
void nilffi_identity(const char *bytes, size_t length, NilFFIString *out) {
  out->bytes = bytes;
  out->length = length;
}

/* Answers the bytes it was given, uppercased.
 *
 * A *new* buffer rather than the one it was handed: the string Nilang passed stays
 * valid as long as the Nilang program does, and writing into the callee's own
 * buffer is what proves the crossing copies rather than borrows — a borrow would
 * have to be forbidden here, and the whole point is that it is not.
 *
 * Before `nil_ffi_buffer` existed this used a `static char[256]` and clipped
 * anything longer, because the only alternative was a `malloc` nobody would free.
 * The size is now whatever the input's is — the first thing a fixed buffer gets
 * wrong, and the only thing it can get wrong. */
void nilffi_upper(const char *bytes, size_t length, NilFFIString *out) {
  if (bytes == NULL || length == 0) {
    nilffi_nothing(out);
    return;
  }
  char *scratch = nil_ffi_buffer((int64_t)length);
  if (scratch == NULL) {
    nilffi_nothing(out);
    return;
  }
  for (size_t i = 0; i < length; i++) {
    unsigned char byte = (unsigned char)bytes[i];
    scratch[i] = (char)(byte >= 'a' && byte <= 'z' ? byte - 32 : byte);
  }
  out->bytes = scratch;
  out->length = length;
}

/* Answers the first `count` bytes.
 *
 * A `count` past the end is clamped rather than refused, because this is the shape
 * of C function that slices and the interesting question is what it does at the
 * edge — not whether it can be made to fail.
 *
 * The bytes are the ones that were handed in, which is the case the ownership rule
 * most easily gets wrong: a crossing that *copied* the string on the way out and
 * then copied it again on the way back would be right, and a crossing that copied
 * only on the way back would be reading the caller's buffer rather than a fresh
 * one — which is fine, and the reason this function cannot tell the two apart. */
void nilffi_take(const char *bytes, size_t length, int64_t count, NilFFIString *out) {
  if (bytes == NULL || length == 0 || count <= 0) {
    nilffi_nothing(out);
    return;
  }
  if ((size_t)count > length) {
    count = (int64_t)length;
  }
  out->bytes = bytes;
  out->length = (size_t)count;
}

/* An empty string, built on the C side and written through the caller's pointer.
 *
 * The other half of `nilffi_take` at zero, and the case where a pointer and a
 * length disagree most sharply: there is nothing to point at, and the answer has to
 * still be a string rather than a null. */
void nilffi_nothing(NilFFIString *out) {
  out->bytes = NULL;
  out->length = 0;
}

/* Answers a string built here rather than passed in.
 *
 * The prefix is fixed and the name is appended, so the answer is longer than
 * anything that went in — which is what catches a crossing that reuses the input's
 * length for the output's.
 *
 * The buffer is the runtime's, and the size is computed rather than capped. This is
 * the function a fixed `static` array could not serve: a long name used to be
 * silently truncated, and truncation is a wrong answer that looks like a right one
 * until someone checks the length. */
void nilffi_greeting(const char *name, size_t length, NilFFIString *out) {
  const char *prefix = "hello, ";
  const size_t prefixLength = 7;
  size_t total = prefixLength + length;

  char *buffer = nil_ffi_buffer((int64_t)total);
  if (buffer == NULL) {
    nilffi_nothing(out);
    return;
  }
  memcpy(buffer, prefix, prefixLength);
  if (length > 0) {
    memcpy(buffer + prefixLength, name, length);
  }

  out->bytes = buffer;
  out->length = total;
}
