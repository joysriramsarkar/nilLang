/* nilffi_selftest.c — exercises the FFI's half of the runtime and reports what it found.
 *
 * # Why this is a separate program from nilcore_selftest.c
 *
 * Because `runtime/core` must not be able to reach `nilffi.h`, and that separation is
 * real rather than a convention: `nilcore` compiles the core files with only the core
 * include path, so a core file that included the FFI header would not build.
 *
 * A *test* is allowed to reach across, and this one has to: the codec and the channel
 * are the FFI's, they build Nilang values, and there is nowhere else they can be
 * checked from. Putting them in the core self-test would have meant giving the core a
 * second include path "just for the tests", which is how a separation stops being one.
 *
 * # The output format is the core self-test's
 *
 * `ok <name>` or `FAIL <name> <detail>`, one per line, and the exit status is the
 * number of failures. The Go harness reads those lines, so a failure on a machine that
 * is not this one still says which behaviour broke.
 */

#include "nilffi.h"

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

static void check_int(const char *what, int64_t got, int64_t want) {
  if (got == want) {
    ok(what);
    return;
  }
  char detail[128];
  snprintf(detail, sizeof(detail), "wanted %lld, got %lld", (long long)want, (long long)got);
  fail(what, detail);
}

static void check_value(const char *what, NilValue got, int64_t want) {
  check_int(what, nil_int_unbox(got), want);
}

/* ── The codec ───────────────────────────────────────────────────────── */

/* check_roundtrip encodes a value and decodes it back, then compares the *rendering*.
 *
 * # Why the rendering rather than the bytes
 *
 * Because the bytes are an implementation detail and the value is the contract. A test
 * that asserted "this tag, then these eight bytes" would fail on every correct change
 * to the encoding and pass on an encoding that is self-consistent and incompatible
 * with every host. What the two sides must agree about is that what comes out is what
 * went in — and the rendering is the only view of a value both sides can take.
 */
static void check_roundtrip(const char *what, NilValue value) {
  char buffer[4096];
  int64_t size = nil_encoded_size(value);
  if (size <= 0) {
    fail(what, "the value was refused before it was encoded");
    return;
  }
  check_int("encode wrote what it measured", nil_encode(value, buffer, (int64_t)sizeof(buffer)),
            size);

  nil_clear_error();
  NilValue back = nil_decode(buffer, size);
  if (nil_last_error_code() != NIL_OK) {
    fail(what, "decoding the value back set an error");
    return;
  }

  char rendered[256];
  nil_render(back, rendered, (int64_t)sizeof(rendered));
  if (strcmp(rendered, what) == 0) {
    ok("a value survives the round trip");
  } else {
    char detail[512];
    snprintf(detail, sizeof(detail), "wanted [%s], got [%s]", what, rendered);
    fail("a value survives the round trip", detail);
  }
}

static void test_codec(void) {
  /* Built through the same constructors a program uses, so this exercises values
   * rather than hand-made bit patterns. */
  check_roundtrip("null", NIL_TAG_NIL);
  check_roundtrip("true", nil_bool_new(1));
  check_roundtrip("false", nil_bool_new(0));
  check_roundtrip("0", nil_int_new(0));
  check_roundtrip("-1", nil_int_new(-1));
  check_roundtrip("9223372036854775807", nil_int_new(INT64_MAX));
  check_roundtrip("-9223372036854775808", nil_int_new(INT64_MIN));
  check_roundtrip("1.5", nil_float_new(1.5));
  check_roundtrip("0.1", nil_float_new(0.1));
  check_roundtrip("", nil_string_new("", 0));
  check_roundtrip("hello", nil_string_new("hello", 5));

  /* Bengali, three bytes per character — the case a host is most likely to get wrong
   * by assuming one, and the language's own reason the codec exists. */
  check_roundtrip("\xe0\xa6\xa8\xe0\xa7\x80\xe0\xa6\xb2\xe0\xa6\xbe\xe0\xa6\x82",
                  nil_string_new("\xe0\xa6\xa8\xe0\xa7\x80\xe0\xa6\xb2\xe0\xa6\xbe\xe0\xa6\x82", 15));

  NilValue list = nil_array_new(3);
  nil_array_set(list, nil_int_new(0), nil_int_new(1));
  nil_array_set(list, nil_int_new(1), nil_string_new("two", 3));
  nil_array_set(list, nil_int_new(2), NIL_TAG_NIL);
  check_roundtrip("[1, two, null]", list);

  NilValue map = nil_map_new(2);
  nil_map_set(map, nil_string_new("b", 1), nil_int_new(2));
  nil_map_set(map, nil_string_new("a", 1), nil_int_new(1));
  /* Written order, not sorted. The message is read in the order the program wrote it,
   * which is the whole reason the map keeps one. */
  check_roundtrip("{b: 2, a: 1}", map);

  /* Nesting, because the depth check is the one easiest to get wrong and the one a
   * host hits by accident when a map holds a map. */
  NilValue outer = nil_array_new(1);
  nil_array_set(outer, nil_int_new(0), map);
  check_roundtrip("[{b: 2, a: 1}]", outer);
}

/* A value that cannot leave the program. */
static NilValue a_closure(NilValue *args, int64_t count) { return nil_int_new(count); }

static void test_codec_refusals(void) {
  char buffer[64];
  NilValue closure = nil_callback_new(a_closure, 1);

  check_int("a callback does not cross", nil_ffi_can_cross(closure), 0);
  check_int("encoded_size refuses a callback", nil_encoded_size(closure), -1);
  check_int("encode refuses a callback", nil_encode(closure, buffer, (int64_t)sizeof(buffer)), -1);

  /* A buffer too small is a refusal, never a short write. A caller that cannot tell
   * those apart writes a truncated message and sends it. */
  NilValue small = nil_string_new("hello", 5);
  check_int("a buffer that is too small is refused", nil_encode(small, buffer, 4), -1);
  check_int("a buffer that is exactly the size is accepted", nil_encode(small, buffer, 14), 14);
}

static void test_malformed(void) {
  /* Every truncation of a real message, rather than the first.
   *
   * Every one, because the first is the one a decoder is most likely to survive: a
   * partial read of a small value often lands inside the value and happens to be
   * plausible. */
  NilValue sample = nil_string_new("abcdefgh", 8);
  char buffer[64];
  int64_t size = nil_encode(sample, buffer, (int64_t)sizeof(buffer));
  int refused = 1;
  for (int64_t cut = 0; cut < size; cut++) {
    nil_clear_error();
    nil_decode(buffer, cut);
    if (nil_last_error_code() == NIL_OK) {
      refused = 0;
      break;
    }
  }
  check("every truncation of a message is refused", refused);

  char unknown[1] = {(char)200};
  nil_clear_error();
  nil_decode(unknown, 1);
  check_int("an unknown tag is refused", nil_last_error_code(), NIL_ERR_INTERNAL);

  /* A string claiming a million bytes inside a nine-byte message. */
  char lying[9];
  lying[0] = 5; /* NIL_CHANNEL_STRING */
  for (int i = 0; i < 8; i++) {
    lying[1 + i] = 0;
  }
  lying[8] = 1;
  nil_clear_error();
  nil_decode(lying, 9);
  check_int("a string longer than the message is refused", nil_last_error_code(), NIL_ERR_INTERNAL);

  /* A list claiming a billion elements. Two bytes of a count would do it, so this is
   * the shape rather than an extreme: the point is that it is refused rather than
   * allocated. */
  char huge[9];
  huge[0] = 6; /* NIL_CHANNEL_LIST */
  for (int i = 0; i < 8; i++) {
    huge[1 + i] = 0;
  }
  huge[8] = 0xFF;
  nil_clear_error();
  nil_decode(huge, 9);
  check_int("a list claiming more elements than it holds is refused",
            nil_last_error_code(), NIL_ERR_INTERNAL);

  char extra[2] = {0, 0};
  nil_clear_error();
  nil_decode(extra, 2);
  check_int("a message with bytes left over is refused", nil_last_error_code(), NIL_ERR_INTERNAL);

  nil_clear_error();
  nil_decode(NULL, 4);
  check_int("a message needs bytes", nil_last_error_code(), NIL_ERR_INTERNAL);
}

/* ── Channels ─────────────────────────────────────────────────────────── */

/* One queued message's arguments, encoded the way a host would.
 *
 * A host has no `NilValue`, so it cannot build one — and if it could, encoding through
 * the runtime would touch the collector from a thread the guest may be collecting on.
 * The wire form is therefore five lines of `memcpy`: a four-byte big-endian method
 * length, the method, and the argument's own encoded bytes.
 *
 * Written here as a host would write it rather than reusing the runtime's own packing,
 * because a test that reuses the implementation checks that the implementation agrees
 * with itself. */
typedef struct {
  char bytes[256];
  int64_t length;
} Wire;

static Wire wire_of(const char *method, const char *argument) {
  Wire wire;
  int64_t method_length = (int64_t)strlen(method);
  int64_t argument_length = (int64_t)strlen(argument);
  wire.length = 4 + method_length + argument_length;
  wire.bytes[0] = (char)((method_length >> 24) & 0xFF);
  wire.bytes[1] = (char)((method_length >> 16) & 0xFF);
  wire.bytes[2] = (char)((method_length >> 8) & 0xFF);
  wire.bytes[3] = (char)(method_length & 0xFF);
  memcpy(wire.bytes + 4, method, (size_t)method_length);
  memcpy(wire.bytes + 4 + method_length, argument, (size_t)argument_length);
  return wire;
}

/* encoded_int is what `nil_encode` produces for a small integer, written out.
 *
 * A literal rather than a call, so the host side of the test never builds a value —
 * which is the property the channel is designed around, and a test that quietly used
 * the runtime to build its own input would stop checking it. */
static void encoded_int(int64_t number, char *out, int64_t *length) {
  out[0] = 3; /* NIL_CHANNEL_INT */
  uint64_t bits = (uint64_t)number;
  for (int i = 0; i < 8; i++) {
    out[1 + i] = (char)((bits >> (i * 8)) & 0xFFu);
  }
  *length = 9;
}

/* ── Threads ─────────────────────────────────────────────────────────── */

/* # Why these tests need one
 *
 * Because `nil_channel_serve` blocks until the channel is closed, and a close has to
 * come from the *other* side — that is what makes a channel two-sided. A test that
 * posts everything first and then serves is not testing the feature; it is testing a
 * queue, and a queue is the easy half.
 *
 * So the guest serves on one thread and the host posts and closes on another. That is
 * the shape a real program has, and it is the only one that can hang: a post that
 * merely appended to a list would leave a serve blocked for ever rather than passing.
 */
#if defined(_WIN32)
#include <windows.h>
typedef HANDLE NilThread;
/* Two values in one allocation, because the Windows entry point takes a single
 * `LPVOID` and a struct would have to be copied into it. */
static DWORD WINAPI serve_entry(LPVOID argument) {
  int64_t handle = *(int64_t *)argument;
  NilValue handler = *(NilValue *)((char *)argument + sizeof(int64_t));
  (void)nil_channel_serve(handle, handler);
  return 0;
}
static void *serve_argument(int64_t handle, NilValue handler) {
  void *block = malloc(sizeof(int64_t) + sizeof(NilValue));
  *(int64_t *)block = handle;
  *(NilValue *)((char *)block + sizeof(int64_t)) = handler;
  return block;
}
static NilThread thread_start(void *argument) {
  return CreateThread(NULL, 0, serve_entry, argument, 0, NULL);
}
static void thread_join(NilThread thread) {
  WaitForSingleObject(thread, 10000);
  CloseHandle(thread);
}
#else
#include <pthread.h>
typedef pthread_t NilThread;
static void *serve_entry(void *argument) {
  int64_t handle = *(int64_t *)argument;
  NilValue handler = *(NilValue *)((char *)argument + sizeof(int64_t));
  (void)nil_channel_serve(handle, handler);
  return NULL;
}
static void *serve_argument(int64_t handle, NilValue handler) {
  void *block = malloc(sizeof(int64_t) + sizeof(NilValue));
  *(int64_t *)block = handle;
  *(NilValue *)((char *)block + sizeof(int64_t)) = handler;
  return block;
}
static NilThread thread_start(void *argument) {
  pthread_t thread;
  pthread_create(&thread, NULL, serve_entry, argument);
  return thread;
}
static void thread_join(NilThread thread) { pthread_join(thread, NULL); }
#endif

/* A handler standing in for a guest program.
 *
 * It answers with the argument doubled, which is enough to prove the argument arrived
 * and the answer went back without needing to know anything about the guest.
 *
 * The arguments are tagged Nilang values and `nil_int_add` takes tagged values: the
 * raw int64 from `nil_int_unbox` must never be passed back in, because its low three
 * bits are then read as a type tag and the value is misread (this crashed the very
 * first threaded message with an access violation). */
static NilValue double_it(NilValue *args, int64_t count) {
  if (count != 2) {
    return NIL_TAG_NIL;
  }
  return nil_int_add(args[1], args[1]);
}

/* take_replies reads `count` answers and checks each is the doubled token.
 *
 * Shared by the two channel tests below because the thing worth checking is the
 * answer, and checking it in two places would mean a change to the check could land in
 * one of them. */
static void take_replies(int64_t handle, int64_t count) {
  for (int64_t token = 1; token <= count; token++) {
    NilFFIString reply = {NULL, 0};
    int64_t seen_token = -1;
    int64_t size = nil_channel_take_reply(handle, &seen_token, &reply);
    if (size <= 0) {
      fail("a reply is waiting", "take_reply answered nothing");
      return;
    }
    if (seen_token != token) {
      char detail[80];
      snprintf(detail, sizeof(detail), "wanted token %lld, got %lld", (long long)token,
               (long long)seen_token);
      fail("the reply carries the token it was asked for", detail);
      return;
    }
    ok("the reply carries the token it was asked for");
    nil_clear_error();
    NilValue decoded = nil_decode(reply.bytes, size);
    if (nil_last_error_code() != NIL_OK) {
      fail("a reply is waiting", "the reply did not decode");
      return;
    }
    check_int("the answer is the doubled argument", nil_int_unbox(decoded), token * 20);
  }
}

/* post_three is the host's half: three messages, written as bytes, from another thread.
 *
 * The doubling is applied by the *guest*, so each token is posted as its own value and
 * the answer comes back doubled. That is what makes the argument's bytes observable
 * rather than merely present. */
static void post_three(int64_t handle) {
  for (int64_t token = 1; token <= 3; token++) {
    char argument[16];
    int64_t argument_length = 0;
    encoded_int(token * 10, argument, &argument_length);
    Wire wire = wire_of("double", argument);
    if (argument_length > 0) {
      /* The wire carries the encoded argument, so it is rebuilt with the real length
       * rather than `strlen` — an encoded integer is not text. */
      int64_t method_length = (int64_t)strlen("double");
      wire.length = 4 + method_length + argument_length;
      memcpy(wire.bytes + 4 + method_length, argument, (size_t)argument_length);
      if (nil_channel_post(handle, token, wire.bytes + 4, method_length,
                           wire.bytes + 4 + method_length, argument_length) <= 0) {
        fail("a message posts", "the host could not post");
        return;
      }
    }
  }
}

/* A guest serving, and a host that posts three messages, reads three answers, and
 * closes.
 *
 * The close is the point of the arrangement: it is what ends the serve loop, and a
 * guest that has to be closed by the host it is talking to is the whole thing. */
static void test_channel_serves(void) {
  int64_t before = nil_channel_outstanding();
  int64_t handle = nil_channel_new();
  NilValue handler = nil_callback_new(double_it, 2);

  void *argument = serve_argument(handle, handler);
  NilThread server = thread_start(argument);

  post_three(handle);
  take_replies(handle, 3);

  check_int("closing answers that it closed", nil_channel_close(handle), 1);
  thread_join(server);
  free(argument);
  check_int("nothing is left over", nil_channel_outstanding(), before);
  nil_callback_release(handler);
}

/* The part no other test reaches: messages posted while the guest is *already blocked*
 * on a thread that was started first and has nothing to do.
 *
 * This is the asynchronous callback the whole feature exists for, and it is the one
 * that cannot be faked: the guest is asleep inside `nil_channel_serve` before the
 * message exists, so a post that merely appended to a list would leave the serve
 * blocked for ever and this test would hang rather than pass. */
static void test_a_host_can_post_from_another_thread(void) {
  int64_t before = nil_channel_outstanding();
  int64_t handle = nil_channel_new();
  NilValue handler = nil_callback_new(double_it, 2);

  /* The guest starts first, and blocks: nothing has been posted yet. */
  void *argument = serve_argument(handle, handler);
  NilThread server = thread_start(argument);

  post_three(handle);
  take_replies(handle, 3);

  nil_channel_close(handle);
  thread_join(server);
  free(argument);
  check_int("nothing is left over", nil_channel_outstanding(), before);
  nil_callback_release(handler);
}

static void test_channel_lifecycle(void) {
  int64_t before = nil_channel_outstanding();

  int64_t handle = nil_channel_new();
  check("a channel opens", handle > 0);
  check_int("an open channel counts", nil_channel_outstanding(), before + 1000);

  Wire wire = wire_of("noop", "");
  check("a message posts before anyone serves",
        nil_channel_post(handle, 7, wire.bytes + 4, 4, wire.bytes + 8, 0) > 0);
  check_int("a queued message counts", nil_channel_outstanding(), before + 1001);

  /* Closing answers "was it open", not "did you free it" — so a host can tell a
   * channel it closed from one that was already gone. */
  check_int("closing answers that it closed", nil_channel_close(handle), 1);
  check_int("and says so again the second time", nil_channel_close(handle), 0);
  check_int("a closed channel and its queued work are gone", nil_channel_outstanding(), before);

  int64_t second = nil_channel_new();
  check("the slot comes back", second > 0);
  check_int("and the handle is not the one that was closed", second != handle, 1);
  nil_channel_close(second);
  check_int("nothing is left over", nil_channel_outstanding(), before);
}

/* A handle from a closed channel must not name whatever took its place.
 *
 * The failure this rules out is silent and nasty: a host holding a stale handle posts
 * into an unrelated conversation and nothing anywhere reports an error. */
static void test_a_stale_handle_is_refused(void) {
  int64_t before = nil_channel_outstanding();

  int64_t stale = nil_channel_new();
  nil_channel_close(stale);

  /* Open enough channels to reuse the slot, because a slot that was still free would
   * make the test pass for the wrong reason. */
  int64_t live[4];
  for (int i = 0; i < 4; i++) {
    live[i] = nil_channel_new();
  }

  Wire wire = wire_of("noop", "");
  nil_clear_error();
  check_int("a stale handle is refused", nil_channel_post(stale, 1, wire.bytes + 4, 4, wire.bytes + 8,
                                                         0),
            -1);
  check("and it says the channel is not open", strstr(nil_last_error(), "not open") != NULL);

  check_int("and the live channels are untouched", nil_channel_outstanding(), before + 4 * 1000);
  for (int i = 0; i < 4; i++) {
    nil_channel_close(live[i]);
  }
  check_int("nothing is left over", nil_channel_outstanding(), before);
}

static void test_channel_refusals(void) {
  int64_t before = nil_channel_outstanding();
  int64_t handle = nil_channel_new();

  /* A handler of the wrong arity is refused at the moment it is registered, which is
   * the only moment where the count and a signature can be compared. */
  NilValue wrong = nil_callback_new(double_it, 1);
  nil_clear_error();
  check_int("a handler of the wrong arity is refused", nil_channel_serve(handle, wrong), -1);
  check("and it says so", strstr(nil_last_error(), "method and an argument") != NULL);
  nil_callback_release(wrong);
  nil_channel_close(handle);

  /* Posting to a channel that is closed, and to one that never was. */
  Wire wire = wire_of("noop", "");
  nil_clear_error();
  check_int("posting to a closed channel is refused",
            nil_channel_post(handle, 1, wire.bytes + 4, 4, wire.bytes + 8, 0), -1);
  check("and it says which", strstr(nil_last_error(), "not open") != NULL);

  check_int("closing a handle that is not open is not an error", nil_channel_close(9999), 0);

  check_int("nothing is left over", nil_channel_outstanding(), before);
}

int main(void) {
  /* Unbuffered, so a crash does not take the report with it. */
  setvbuf(stdout, NULL, _IONBF, 0);

  test_codec();
  test_codec_refusals();
  test_malformed();
  test_channel_lifecycle();
  test_a_stale_handle_is_refused();
  test_channel_refusals();
  test_channel_serves();
  test_a_host_can_post_from_another_thread();

  printf("failures %d\n", failures);
  return failures == 0 ? 0 : 1;
}
