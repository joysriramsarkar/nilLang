/* channel.c — a message queue between a host and a guest, and the encoding its
 * messages are written in.
 *
 * # What this is for
 *
 * The rest of the FFI is one direction at a time: Nilang calls C, C answers in a
 * register, and the call ends. A platform channel is the other shape. A host — a JVM,
 * an Android activity, anything with a C entry point — starts the exchange, and it
 * does so **later**: while the guest is doing something else, from another thread,
 * with no call in flight to answer through.
 *
 * So a message cannot come back in a register. It is queued, and the guest collects
 * it when it is next ready.
 *
 * # Why the service loop is here and not in Nilang
 *
 * Because the loop has to *block*, and blocking means waiting for something another
 * thread will signal. A Nilang `while` loop calling a C function that waits would do
 * the same thing, but then every program that wants to serve a channel writes the
 * loop, gets the wakeup right, and decides what happens when the host closes it. One
 * implementation, in the runtime, is one answer.
 *
 * What the loop must **not** do is run guest code on a host's thread. It does not: it
 * decodes, invokes the handler through a callback on the thread that called
 * `nil_channel_serve`, and encodes the answer. A posting thread touches only the
 * queue — a lock and two linked lists.
 *
 * # Why an encoding at all
 *
 * Because the two sides share no types and, more to the point, no *knowledge* of each
 * other's types. A call crossing knows its signature at compile time. A message
 * arriving from a host does not: it carries whatever the host chose to send, and the
 * guest has to read it without having been told what it is.
 *
 * So a message is self-describing — one tag byte, then a payload — and the definitions
 * live here, because the runtime is the only thing that knows what a `NilValue` is.
 *
 * # Why little-endian, written out by hand
 *
 * Because a platform channel is the feature most likely to cross a machine, and
 * because writing the bytes out costs ten lines and removes a class of bug that only
 * appears on the machine nobody tested on. A `memcpy` of a `double` into a message
 * would be correct everywhere except the one place it matters.
 */

#include "nilffi.h"
#include "nil_internal.h"

#include <stdio.h>
#include <stdlib.h>
#include <string.h>

/* ── Locks ────────────────────────────────────────────────────────────── */

/* One shape, two spellings.
 *
 * The alternative is a lock type in `nilffi.h`, which would put `<windows.h>` or
 * `<pthread.h>` into every generated C file that includes the header. The header is
 * read by people; the implementation is not. */
#if defined(_WIN32)
#include <windows.h>
typedef CRITICAL_SECTION NilLock;
typedef CONDITION_VARIABLE NilSignal;
#define nil_lock_init(lock) InitializeCriticalSection(lock)
#define nil_lock_acquire(lock) EnterCriticalSection(lock)
#define nil_lock_release(lock) LeaveCriticalSection(lock)
#define nil_lock_destroy(lock) DeleteCriticalSection(lock)
#define nil_signal_init(signal) InitializeConditionVariable(signal)
#define nil_signal_wait(signal, lock) SleepConditionVariableCS(signal, lock, INFINITE)
#define nil_signal_wake(signal) WakeAllConditionVariable(signal)
#define nil_signal_destroy(signal) ((void)(signal))
#else
#include <pthread.h>
typedef pthread_mutex_t NilLock;
typedef pthread_cond_t NilSignal;
#define nil_lock_init(lock) pthread_mutex_init(lock, NULL)
#define nil_lock_acquire(lock) pthread_mutex_lock(lock)
#define nil_lock_release(lock) pthread_mutex_unlock(lock)
#define nil_lock_destroy(lock) pthread_mutex_destroy(lock)
#define nil_signal_init(signal) pthread_cond_init(signal, NULL)
#define nil_signal_wait(signal, lock) pthread_cond_wait(signal, lock)
#define nil_signal_wake(signal) pthread_cond_broadcast(signal)
#define nil_signal_destroy(signal) pthread_cond_destroy(signal)
#endif

/* ── The encoding ─────────────────────────────────────────────────────── */

/* One tag byte, then a payload whose shape the tag says.
 *
 * Fixed-width fields are little-endian and written byte by byte, so the bytes are the
 * same on a machine whose `double` is big-endian. The tags are frozen: a host and a
 * guest built from different versions must still read each other, and that only holds
 * if a tag never changes meaning. `NIL_CHANNEL_ABI_VERSION` is what to raise when one
 * has to.
 *
 * A byte has no tag of its own. `Byte` is an `Int` in Nilang and crosses as `uint8_t`
 * (see `nilffi.h`), so a byte arrives here already widened; a separate tag would be a
 * second spelling of one value, and the decoder would have to choose between them. */
enum NilChannelTag {
  NIL_CHANNEL_NULL = 0,
  NIL_CHANNEL_FALSE = 1,
  NIL_CHANNEL_TRUE = 2,
  NIL_CHANNEL_INT = 3,
  NIL_CHANNEL_FLOAT = 4,
  NIL_CHANNEL_STRING = 5,
  NIL_CHANNEL_LIST = 6,
  NIL_CHANNEL_MAP = 7
};

static void put_u64(char *out, uint64_t value) {
  for (int i = 0; i < 8; i++) {
    out[i] = (char)((value >> (i * 8)) & 0xFFu);
  }
}

static uint64_t get_u64(const char *bytes) {
  uint64_t value = 0;
  for (int i = 0; i < 8; i++) {
    value |= (uint64_t)(unsigned char)bytes[i] << (i * 8);
  }
  return value;
}

static void put_f64(char *out, double value) {
  /* Through the bit pattern rather than a cast: a double's bytes are only defined by
   * its representation, and this is the one place that has to be right about it. */
  uint64_t bits = 0;
  memcpy(&bits, &value, sizeof(bits));
  put_u64(out, bits);
}

/* size_of is `nil_encoded_size` without building anything.
 *
 * Two passes over the value — the first counting, the second writing — because the
 * caller needs a buffer before there is anything to put in it. */
static int64_t size_of(NilValue value) {
  switch (nil_object_kind(value)) {
    case -1:
      return nil_is_inline_int(value) ? 9 : 1;
    case NIL_OBJ_INTBOX:
      return 9;
    case NIL_OBJ_FLOATBOX:
      return 9;
    case NIL_OBJ_STRING: {
      NilString *string = (NilString *)nil_object_header(value);
      return 1 + 8 + (int64_t)string->header.length;
    }
    case NIL_OBJ_ARRAY: {
      NilArray *array = (NilArray *)nil_object_header(value);
      int64_t total = 9;
      for (size_t i = 0; i < array->header.length; i++) {
        int64_t item = size_of(array->data[i]);
        if (item < 0) {
          return -1;
        }
        total += item;
      }
      return total;
    }
    case NIL_OBJ_MAP: {
      /* Walked in the map's written order, not its slots.
       *
       * Two reasons, and the second is the one that matters: the same map encoded
       * twice produces the same bytes, so a host and a guest cannot disagree about
       * what a value is; and the order a program wrote is the order a host reading a
       * message sees, which is the order the message is *for*. */
      NilMap *map = (NilMap *)nil_object_header(value);
      int64_t total = 9;
      for (size_t i = 0; i < map->count; i++) {
        size_t slot = map->order[i];
        int64_t key = size_of(map->keys[slot]);
        int64_t item = size_of(map->values[slot]);
        if (key < 0 || item < 0) {
          return -1;
        }
        total += key + item;
      }
      return total;
    }
    default:
      /* A closure has no meaning outside the program that made it, and a value with
       * no tag would be indistinguishable from a number. Refused by kind rather than
       * written as something the other side would misread. */
      return -1;
  }
}

int64_t nil_encoded_size(NilValue value) { return size_of(value); }

static int64_t write_value(char *out, int64_t at, int64_t capacity, NilValue value);

/* write_count writes a count, refusing a count the buffer cannot hold.
 *
 * Two bytes an element is the floor — a list element has a tag, a map pair has two —
 * so this rejects an impossible count before writing any of it. That is the write
 * side of the rule `read_list` and `read_map` enforce on the other side. */
static int64_t write_count(char *out, int64_t at, int64_t capacity, uint64_t count) {
  if (at + 8 > capacity || count > (uint64_t)(capacity - at - 8) / 2) {
    return -1;
  }
  put_u64(out + at, count);
  return at + 8;
}

static int64_t write_value(char *out, int64_t at, int64_t capacity, NilValue value) {
  if (at < 0 || at >= capacity) {
    return -1;
  }
  switch (nil_object_kind(value)) {
    case -1:
      if (!nil_is_inline_int(value)) {
        if (value == NIL_TAG_TRUE) {
          out[at++] = (char)NIL_CHANNEL_TRUE;
        } else if (value == NIL_TAG_FALSE) {
          out[at++] = (char)NIL_CHANNEL_FALSE;
        } else {
          out[at++] = (char)NIL_CHANNEL_NULL;
        }
        return at;
      }
      out[at++] = (char)NIL_CHANNEL_INT;
      if (at + 8 > capacity) {
        return -1;
      }
      put_u64(out + at, (uint64_t)(int64_t)nil_int_unbox(value));
      return at + 8;
    case NIL_OBJ_INTBOX:
      out[at++] = (char)NIL_CHANNEL_INT;
      if (at + 8 > capacity) {
        return -1;
      }
      put_u64(out + at, (uint64_t)nil_int_unbox(value));
      return at + 8;
    case NIL_OBJ_FLOATBOX:
      out[at++] = (char)NIL_CHANNEL_FLOAT;
      if (at + 8 > capacity) {
        return -1;
      }
      put_f64(out + at, nil_float_unbox(value));
      return at + 8;
    case NIL_OBJ_STRING: {
      NilString *string = (NilString *)nil_object_header(value);
      size_t length = (size_t)string->header.length;
      out[at++] = (char)NIL_CHANNEL_STRING;
      if (at + 8 > capacity) {
        return -1;
      }
      put_u64(out + at, (uint64_t)length);
      at += 8;
      if ((int64_t)length > capacity - at) {
        return -1;
      }
      memcpy(out + at, string->data, length);
      return at + (int64_t)length;
    }
    case NIL_OBJ_ARRAY: {
      NilArray *array = (NilArray *)nil_object_header(value);
      size_t length = (size_t)array->header.length;
      out[at++] = (char)NIL_CHANNEL_LIST;
      int64_t after = write_count(out, at, capacity, (uint64_t)length);
      if (after < 0) {
        return -1;
      }
      at = after;
      for (size_t i = 0; i < length; i++) {
        int64_t end = write_value(out, at, capacity, array->data[i]);
        if (end < 0) {
          return -1;
        }
        at = end;
      }
      return at;
    }
    case NIL_OBJ_MAP: {
      NilMap *map = (NilMap *)nil_object_header(value);
      size_t count = map->count;
      out[at++] = (char)NIL_CHANNEL_MAP;
      /* The count is of **pairs**, so a reader that sees one count knows to read two
       * values per step. Written in the map's own order — the order `size_of` walked,
       * because the two have to agree or the size and the bytes disagree. */
      int64_t after = write_count(out, at, capacity, (uint64_t)count);
      if (after < 0) {
        return -1;
      }
      at = after;
      for (size_t i = 0; i < count; i++) {
        size_t slot = map->order[i];
        int64_t end = write_value(out, at, capacity, map->keys[slot]);
        if (end < 0) {
          return -1;
        }
        at = end;
        end = write_value(out, at, capacity, map->values[slot]);
        if (end < 0) {
          return -1;
        }
        at = end;
      }
      return at;
    }
    default:
      return -1;
  }
}

int64_t nil_encode(NilValue value, char *out, int64_t capacity) {
  int64_t size = size_of(value);
  if (size < 0 || out == NULL || capacity < size) {
    return -1;
  }
  int64_t end = write_value(out, 0, capacity, value);
  /* The count and the write disagreeing is a bug in this file, not a fact about the
   * caller's value — and saying so is the difference between a message that was
   * refused and one that was silently truncated. */
  if (end != size) {
    nil_set_error(NIL_ERR_INTERNAL, "a message's size and its encoding disagree");
    return -1;
  }
  return end;
}

/* ── Decoding ─────────────────────────────────────────────────────────── */

/* A cursor over a message, and the reason a decode cannot be tricked.
 *
 * Every read checks the remaining length first. A host is not obliged to send a
 * well-formed message — it may be another version, or not a host at all — and reading
 * past the end would return whatever the allocator left there, which is to say
 * whatever is cheapest to guess.
 *
 * `failed` exists because `null` is a legal value and `NIL_TAG_NIL` is its answer, so
 * the return value alone cannot say "this decode did not work". */
typedef struct {
  const char *bytes;
  int64_t length;
  int64_t at;
  int failed;
} NilReader;

static void reader_fail(NilReader *reader, const char *why) {
  if (!reader->failed) {
    reader->failed = 1;
    nil_set_error(NIL_ERR_INTERNAL, why);
  }
}

static int reader_has(NilReader *reader, int64_t n) {
  return !reader->failed && n >= 0 && reader->at >= 0 && reader->at <= reader->length - n;
}

static int reader_u8(NilReader *reader, uint8_t *out) {
  if (!reader_has(reader, 1)) {
    reader_fail(reader, "a message ended in the middle of a tag");
    return 0;
  }
  *out = (uint8_t)reader->bytes[reader->at++];
  return 1;
}

static int reader_u64(NilReader *reader, uint64_t *out) {
  if (!reader_has(reader, 8)) {
    reader_fail(reader, "a message ended in the middle of a number");
    return 0;
  }
  *out = get_u64(reader->bytes + reader->at);
  reader->at += 8;
  return 1;
}

static NilValue read_value(NilReader *reader);

static NilValue read_list(NilReader *reader) {
  uint64_t count = 0;
  if (!reader_u64(reader, &count)) {
    return NIL_TAG_NIL;
  }
  /* Bounded before anything is allocated.
   *
   * A count is the one number in a message a two-byte edit can make enormous, and
   * trusting it is how a malformed message becomes an allocation the size of the
   * machine. One byte per element is the floor: every element carries a tag. */
  if (count > (uint64_t)(reader->length - reader->at)) {
    reader_fail(reader, "a message claims more elements than it holds");
    return NIL_TAG_NIL;
  }

  NilValue out = nil_array_new((int64_t)count);
  if (out == NIL_TAG_NIL) {
    return NIL_TAG_NIL;
  }
  /* Pinned across the build.
   *
   * The collector has an explicit root set and no stack scan (see `gc.c`), so a value
   * held only in a C local is unreachable and the next allocation would free it — this
   * one, halfway through the list it is being filled with. */
  nil_gc_pin(out);
  for (uint64_t i = 0; i < count && !reader->failed; i++) {
    NilValue item = read_value(reader);
    if (reader->failed) {
      break;
    }
    nil_array_set(out, nil_int_new((int64_t)i), item);
  }
  nil_gc_unpin_all();
  return reader->failed ? NIL_TAG_NIL : out;
}

static NilValue read_map(NilReader *reader) {
  uint64_t count = 0;
  if (!reader_u64(reader, &count)) {
    return NIL_TAG_NIL;
  }
  /* Two bytes per pair, for the reason a list gets one per element: two tags. */
  if (count > (uint64_t)(reader->length - reader->at) / 2) {
    reader_fail(reader, "a message claims more pairs than it holds");
    return NIL_TAG_NIL;
  }

  NilValue out = nil_map_new((int64_t)count);
  if (out == NIL_TAG_NIL) {
    return NIL_TAG_NIL;
  }
  nil_gc_pin(out);
  for (uint64_t i = 0; i < count && !reader->failed; i++) {
    NilValue key = read_value(reader);
    if (reader->failed) {
      break;
    }
    NilValue item = read_value(reader);
    if (reader->failed) {
      break;
    }
    nil_map_set(out, key, item);
  }
  nil_gc_unpin_all();
  return reader->failed ? NIL_TAG_NIL : out;
}

static NilValue read_value(NilReader *reader) {
  uint8_t tag = 0;
  if (!reader_u8(reader, &tag)) {
    return NIL_TAG_NIL;
  }
  switch (tag) {
    case NIL_CHANNEL_NULL:
      return NIL_TAG_NIL;
    case NIL_CHANNEL_FALSE:
      return nil_bool_new(0);
    case NIL_CHANNEL_TRUE:
      return nil_bool_new(1);
    case NIL_CHANNEL_INT: {
      uint64_t bits = 0;
      if (!reader_u64(reader, &bits)) {
        return NIL_TAG_NIL;
      }
      return nil_int_new((int64_t)bits);
    }
    case NIL_CHANNEL_FLOAT: {
      uint64_t bits = 0;
      if (!reader_u64(reader, &bits)) {
        return NIL_TAG_NIL;
      }
      double value = 0.0;
      memcpy(&value, &bits, sizeof(value));
      return nil_float_new(value);
    }
    case NIL_CHANNEL_STRING: {
      uint64_t length = 0;
      if (!reader_u64(reader, &length)) {
        return NIL_TAG_NIL;
      }
      if (!reader_has(reader, (int64_t)length)) {
        reader_fail(reader, "a message claims a string longer than it holds");
        return NIL_TAG_NIL;
      }
      NilValue out = nil_string_new(reader->bytes + reader->at, (size_t)length);
      reader->at += (int64_t)length;
      return out;
    }
    case NIL_CHANNEL_LIST:
      return read_list(reader);
    case NIL_CHANNEL_MAP:
      return read_map(reader);
    default:
      reader_fail(reader, "a message contains a tag this runtime does not know");
      return NIL_TAG_NIL;
  }
}

NilValue nil_decode(const char *bytes, int64_t length) {
  if (bytes == NULL || length <= 0) {
    nil_set_error(NIL_ERR_INTERNAL, "a message needs at least one byte");
    return NIL_TAG_NIL;
  }
  NilReader reader = {bytes, length, 0, 0};
  NilValue value = read_value(&reader);
  if (reader.failed) {
    return NIL_TAG_NIL;
  }
  /* Trailing bytes are a refusal rather than a shrug.
   *
   * They mean the two sides disagree about what this message is, and reading the part
   * both understand would hide exactly that disagreement. */
  if (reader.at != reader.length) {
    nil_set_error(NIL_ERR_INTERNAL, "a message has bytes left over after decoding");
    return NIL_TAG_NIL;
  }
  return value;
}

/* ── Channels ─────────────────────────────────────────────────────────── */

/* One queued message, and its ownership.
 *
 * The bytes are `malloc`ed and freed by the queue, which the collector knows nothing
 * about — the same arrangement `nil_ffi_buffer` uses. A message therefore belongs to
 * whichever queue holds it and to nothing else, until it is popped. */
typedef struct NilFFIMessage {
  int64_t token;
  char *bytes;
  int64_t length;
  struct NilFFIMessage *next;
} NilFFIMessage;

typedef struct NilFFIChannel {
  int64_t generation;
  int64_t slot;
  int64_t refs;
  NilLock lock;
  NilSignal arrived;
  NilSignal answered;
  NilFFIMessage *in_head, *in_tail;
  int64_t in_count;
  NilFFIMessage *out_head, *out_tail;
  int64_t out_count;
  int closed;
} NilFFIChannel;

/* The slot table.
 *
 * A fixed array with reuse, rather than a map keyed by handle: the number of channels
 * a program holds open at once is small and bounded by the program, and an array makes
 * "is this handle real" an arithmetic check rather than a lookup that can fail in a new
 * way.
 *
 * # Why a handle carries a generation
 *
 * Because a slot is reused, and a bare index would make a host's stale handle name
 * somebody else's channel. That is the classic handle-reuse bug and it is silent: the
 * host posts a message that lands in an unrelated conversation.
 *
 * A handle is `generation * SLOTS + slot + 1`. Zero is never a handle, so a caller
 * that never checked an answer and passed 0 on gets "not open" rather than slot one;
 * and a handle from a closed channel names a slot whose generation has moved on, which
 * is detected rather than obeyed.
 *
 * With a 64-bit generation a stale handle would have to survive 2^64 reuses of its slot
 * before it could name a live channel again. It is written down rather than reasoned
 * about because the reasoning is exactly the kind that is right until the one case it
 * was not right about. */
#define NIL_CHANNEL_SLOTS 64
static NilFFIChannel *channel_slots[NIL_CHANNEL_SLOTS];
static int64_t channel_generations[NIL_CHANNEL_SLOTS];
static int64_t channel_live = 0;
static int64_t channel_queued = 0;
static NilLock channel_table_lock;
static int channel_table_ready = 0;

static void channel_table_init(void) {
  if (channel_table_ready) {
    return;
  }
  nil_lock_init(&channel_table_lock);
  channel_table_ready = 1;
}

/* # Lock order
 *
 * The table lock, then a channel's own lock, and never the other way round. A channel
 * lock is taken while the table lock is held and is *kept* until `channel_release`,
 * which releases it before taking the table lock again — so the two are never held at
 * once by one thread, and a thread waiting on a condition variable (which releases its
 * channel lock) can still take the table lock. */
static NilFFIChannel *channel_enter(int64_t handle) {
  channel_table_init();
  if (handle <= 0) {
    return NULL;
  }
  int64_t slot = (handle - 1) % NIL_CHANNEL_SLOTS;
  int64_t generation = (handle - 1) / NIL_CHANNEL_SLOTS;

  nil_lock_acquire(&channel_table_lock);
  NilFFIChannel *channel = channel_slots[slot];
  if (channel == NULL || channel->generation != generation) {
    nil_lock_release(&channel_table_lock);
    return NULL;
  }
  channel->refs++;
  nil_lock_acquire(&channel->lock);
  nil_lock_release(&channel_table_lock);
  return channel;
}

/* channel_release drops a reference and frees the channel when the last one goes while
 * it is closed and empty.
 *
 * # Why a count and not just the queues being empty
 *
 * Because a thread waiting inside a condition variable has *released* its channel lock
 * to wait. A close can therefore acquire it, mark the channel, and — with nothing but
 * the queue lengths to stop it — destroy the very lock the sleeper is queued on, which
 * is an access violation rather than a tidy failure. It happened.
 *
 * A sleeper holds a reference, so the count is one, so nothing is destroyed until it
 * wakes, sees the channel closed, and leaves. */
static void channel_release(NilFFIChannel *channel) {
  int64_t slot = channel->slot;
  nil_lock_release(&channel->lock);

  int destroy = 0;
  nil_lock_acquire(&channel_table_lock);
  channel->refs--;
  if (channel->refs == 0 && channel->closed && channel->in_head == NULL &&
      channel->out_head == NULL) {
    /* Cleared before the channel is freed, so a thread that enters between the two
     * finds no slot rather than a freed one. */
    channel_slots[slot] = NULL;
    channel_live--;
    destroy = 1;
  }
  if (destroy) {
    channel_queued -= channel->in_count + channel->out_count;
  }
  nil_lock_release(&channel_table_lock);

  if (destroy) {
    nil_lock_destroy(&channel->lock);
    nil_signal_destroy(&channel->arrived);
    nil_signal_destroy(&channel->answered);
    free(channel);
  }
}

static void channel_drain(NilFFIMessage **head, int64_t *count) {
  while (*head != NULL) {
    NilFFIMessage *message = *head;
    *head = message->next;
    free(message->bytes);
    free(message);
    (*count)--;
    channel_queued--;
  }
  *head = NULL;
}

int64_t nil_channel_new(void) {
  channel_table_init();
  nil_lock_acquire(&channel_table_lock);
  int64_t handle = 0;
  for (int64_t slot = 0; slot < NIL_CHANNEL_SLOTS; slot++) {
    if (channel_slots[slot] != NULL) {
      continue;
    }
    NilFFIChannel *channel = (NilFFIChannel *)calloc(1, sizeof(NilFFIChannel));
    if (channel == NULL) {
      break;
    }
    channel_generations[slot]++;
    channel->generation = channel_generations[slot];
    channel->slot = slot;
    nil_lock_init(&channel->lock);
    nil_signal_init(&channel->arrived);
    nil_signal_init(&channel->answered);
    channel_slots[slot] = channel;
    channel_live++;
    handle = channel->generation * NIL_CHANNEL_SLOTS + slot + 1;
    break;
  }
  nil_lock_release(&channel_table_lock);

  if (handle == 0) {
    /* Said with the number, because "too many channels" is useless and "more than 64"
     * is a fact a reader can act on. */
    char message[96];
    snprintf(message, sizeof(message),
             "a program may hold at most %d channels open at once; close one first",
             NIL_CHANNEL_SLOTS);
    nil_set_error(NIL_ERR_INTERNAL, message);
  }
  return handle;
}

static void queue_push(NilFFIMessage **head, NilFFIMessage **tail, NilFFIMessage *message) {
  message->next = NULL;
  if (*tail == NULL) {
    *head = message;
  } else {
    (*tail)->next = message;
  }
  *tail = message;
}

static NilFFIMessage *queue_pop(NilFFIMessage **head, NilFFIMessage **tail) {
  NilFFIMessage *message = *head;
  if (message == NULL) {
    return NULL;
  }
  *head = message->next;
  if (*head == NULL) {
    *tail = NULL;
  }
  message->next = NULL;
  return message;
}

static void message_free(NilFFIMessage *message) {
  if (message != NULL) {
    free(message->bytes);
    free(message);
  }
}

int64_t nil_channel_post(int64_t handle, int64_t token, const char *method, int64_t method_length,
                         const char *argument, int64_t argument_length) {
  NilFFIChannel *channel = channel_enter(handle);
  if (channel == NULL) {
    nil_set_error(NIL_ERR_INTERNAL, "a message was posted to a channel that is not open");
    return -1;
  }
  if (method == NULL || method_length < 0 || argument == NULL || argument_length < 0) {
    channel_release(channel);
    nil_set_error(NIL_ERR_INTERNAL, "a message needs a method and an argument");
    return -1;
  }

  /* The wire form is a four-byte method length, the method, and the argument's own
   * encoded bytes.
   *
   * A length rather than a separator, because a method may contain any byte at all and
   * a host writing a separator would eventually put one inside one. The argument is
   * *not* length-prefixed because it is self-delimiting: an encoded value says how long
   * it is, which is the whole reason the codec is written the way it is.
   *
   * Nothing here builds a Nilang value, so a host thread never touches the collector —
   * and the collector's pin list and root set are process-wide, not per-thread, so two
   * threads using them corrupt each other. That was an access violation, not a
   * subtlety. */
  int64_t size = 4 + method_length + argument_length;
  NilFFIMessage *message = (NilFFIMessage *)calloc(1, sizeof(NilFFIMessage));
  if (message == NULL) {
    channel_release(channel);
    nil_set_error(NIL_ERR_INTERNAL, "out of memory queueing a message");
    return -1;
  }
  message->bytes = (char *)malloc((size_t)size);
  if (message->bytes == NULL) {
    free(message);
    channel_release(channel);
    nil_set_error(NIL_ERR_INTERNAL, "out of memory queueing a message");
    return -1;
  }
  message->bytes[0] = (char)((method_length >> 24) & 0xFF);
  message->bytes[1] = (char)((method_length >> 16) & 0xFF);
  message->bytes[2] = (char)((method_length >> 8) & 0xFF);
  message->bytes[3] = (char)(method_length & 0xFF);
  memcpy(message->bytes + 4, method, (size_t)method_length);
  memcpy(message->bytes + 4 + method_length, argument, (size_t)argument_length);
  message->token = token;
  message->length = size;

  if (channel->closed) {
    /* Refused rather than dropped silently. A host that posts into a channel that has
     * gone away has a bug, and the bug should say so where the host can see it. */
    message_free(message);
    channel_release(channel);
    nil_set_error(NIL_ERR_INTERNAL, "a message was posted to a closed channel");
    return -1;
  }
  queue_push(&channel->in_head, &channel->in_tail, message);
  channel->in_count++;
  channel_queued++;
  nil_signal_wake(&channel->arrived);
  channel_release(channel);
  return size;
}

/* nil_channel_take_reply is how a host waits for an answer.
 *
 * Blocks until one is queued or the channel is closed, and copies it into the caller's
 * scope along with the token it was posted under. The copy is not optional: the queue
 * frees a message the moment it is popped — the queue is its only owner and it has just
 * given it up — so handing over a pointer would hand over memory that is already gone.
 *
 * The answer comes back as **bytes**, not as a value. A host that received a `NilValue`
 * would have to be built against the runtime's representation to read it, which is the
 * one thing a host is not; and building one here would mean touching the collector from
 * a thread the guest may be collecting on. `nil_decode` is how a host that wants a
 * value gets one, on its own thread, where the collector's state belongs to it. */
int64_t nil_channel_take_reply(int64_t handle, int64_t *token, NilFFIString *out) {
  NilFFIChannel *channel = channel_enter(handle);
  if (channel == NULL) {
    nil_set_error(NIL_ERR_INTERNAL, "a reply was taken from a channel that is not open");
    return -1;
  }
  if (out == NULL) {
    channel_release(channel);
    nil_set_error(NIL_ERR_INTERNAL, "a reply needs somewhere to be copied to");
    return -1;
  }

  while (channel->out_head == NULL && !channel->closed) {
    nil_signal_wait(&channel->answered, &channel->lock);
  }
  NilFFIMessage *message = queue_pop(&channel->out_head, &channel->out_tail);
  if (message == NULL) {
    channel_release(channel);
    return -1;
  }
  channel->out_count--;
  channel_queued--;

  size_t written = 0;
  char *bytes = nil_ffi_scope_alloc(message->length, &written);
  if (bytes == NULL) {
    message_free(message);
    channel_release(channel);
    return -1;
  }
  memcpy(bytes, message->bytes, (size_t)message->length);
  out->bytes = bytes;
  out->length = written;
  if (token != NULL) {
    *token = message->token;
  }
  message_free(message);
  channel_release(channel);
  return (int64_t)written;
}

/* nil_channel_outstanding is what a leak test reads.
 *
 * Two counts in one number, channels times a thousand plus queued messages, because
 * "a program did not leak" is one claim and a program that leaks a channel and a
 * program that leaks a message have different bugs. A thousand is larger than any
 * plausible queue depth, so the two never overlap. */
int64_t nil_channel_outstanding(void) {
  channel_table_init();
  nil_lock_acquire(&channel_table_lock);
  int64_t live = channel_live;
  int64_t queued = channel_queued;
  nil_lock_release(&channel_table_lock);
  return live * 1000 + queued;
}

/* nil_channel_close ends a channel.
 *
 * # What happens to messages still in the queues
 *
 * They are discarded, and that is the contract rather than an accident. A close says
 * "this conversation is over"; a host that has not taken its replies has its own bug,
 * and dropping them is how the host finds out. The alternative — holding the channel
 * open until the queues drain — means a guest that exits without being closed leaves
 * its channel open for ever, which is worse than a lost message.
 *
 * # Why the answer is about being open and not about being freed
 *
 * Because those are different questions and a host asks the first. A channel closed
 * with work in flight is still closed, and a host that reads the answer as "did you
 * free it" learns nothing useful. It answers 1 when the channel was open, which is also
 * what makes closing twice ordinary rather than an error. */
int64_t nil_channel_close(int64_t handle) {
  NilFFIChannel *channel = channel_enter(handle);
  if (channel == NULL) {
    return 0;
  }
  channel->closed = 1;
  /* Both, and broadcast: a waiter on either side has to be told, and a program may
   * legitimately have a host thread waiting for a reply while the guest thread waits
   * for a message. */
  nil_signal_wake(&channel->arrived);
  nil_signal_wake(&channel->answered);
  channel_drain(&channel->in_head, &channel->in_count);
  channel_drain(&channel->out_head, &channel->out_count);
  channel_release(channel);
  return 1;
}

/* serve_one takes one message, runs the handler, and queues the answer.
 *
 * Returns 1 for a message served, 0 when the channel closed with none left, and -1 for
 * a message that could not be handled. The caller holds the channel's lock and a
 * reference; the wait inside releases the lock but not the reference, which is what
 * keeps a close from destroying the lock this thread is queued on. */
static int serve_one(NilFFIChannel *channel, NilValue handler) {
  /* A reference is held by anyone who has called `channel_enter` and not yet left.
   * The serve loop enters once (before the loop) and releases once (after), not per
   * message, because a message could be queued but the loop never sees it — the close
   * could come from another thread between two messages — and the count must stay
   * non-zero until the loop is finished. */

  while (channel->in_head == NULL && !channel->closed) {
    nil_signal_wait(&channel->arrived, &channel->lock);
  }
  NilFFIMessage *message = queue_pop(&channel->in_head, &channel->in_tail);
  if (message == NULL) {
    return 0;
  }
  channel->in_count--;
  channel_queued--;

  if (message->length < 4) {
    message_free(message);
    nil_set_error(NIL_ERR_INTERNAL, "a message is too short to hold a method");
    return -1;
  }
  uint32_t method_length = (uint32_t)((unsigned char)message->bytes[0] << 24) |
                           (uint32_t)((unsigned char)message->bytes[1] << 16) |
                           (uint32_t)((unsigned char)message->bytes[2] << 8) |
                           (uint32_t)((unsigned char)message->bytes[3]);
  if ((int64_t)method_length + 4 > message->length) {
    message_free(message);
    nil_set_error(NIL_ERR_INTERNAL, "a message claims a method longer than it holds");
    return -1;
  }

  /* The error is cleared first, so that a `null` argument — a legal message — is not
   * mistaken for a decode that failed. The two are told apart by the error and not by
   * the value, because `null` is what a failed decode answers. */
  nil_clear_error();
  NilValue method = nil_string_new(message->bytes + 4, method_length);
  NilValue argument = nil_decode(message->bytes + 4 + method_length,
                                  message->length - 4 - (int64_t)method_length);
  if (nil_last_error_code() != NIL_OK) {
    message_free(message);
    return -1;
  }

  /* Handler, arguments and answer pinned across the call.
   *
   * The handler is a whole Nilang program: it allocates, and a collection in the middle
   * of it would otherwise be free to collect the arguments it was just called with, or
   * the answer it has just produced. */
  NilValue args[2];
  args[0] = method;
  args[1] = argument;
  nil_gc_pin(handler);
  nil_gc_pin(method);
  nil_gc_pin(argument);
  NilValue answer = nil_callback_invoke(handler, args, 2);
  nil_gc_pin(answer);

  int64_t size = nil_encoded_size(answer);
  if (size < 0) {
    /* Said here rather than answered with nothing, because a guest that returns the
     * wrong thing and a host that receives an empty answer is the worst of the three
     * possible outcomes: it looks like it worked. */
    nil_gc_unpin_all();
    message_free(message);
    nil_set_error(NIL_ERR_INTERNAL, "a handler answered with a value that cannot cross");
    return -1;
  }

  NilFFIMessage *reply = (NilFFIMessage *)calloc(1, sizeof(NilFFIMessage));
  if (reply == NULL) {
    nil_gc_unpin_all();
    message_free(message);
    nil_set_error(NIL_ERR_INTERNAL, "out of memory queueing a reply");
    return -1;
  }
  reply->bytes = (char *)malloc((size_t)size);
  if (reply->bytes == NULL) {
    free(reply);
    nil_gc_unpin_all();
    message_free(message);
    nil_set_error(NIL_ERR_INTERNAL, "out of memory queueing a reply");
    return -1;
  }
  if (nil_encode(answer, reply->bytes, size) != size) {
    message_free(reply);
    nil_gc_unpin_all();
    message_free(message);
    return -1;
  }
  reply->token = message->token;
  reply->length = size;
  nil_gc_unpin_all();

  queue_push(&channel->out_head, &channel->out_tail, reply);
  channel->out_count++;
  channel_queued++;
  nil_signal_wake(&channel->answered);

  message_free(message);
  return 1;
}

int64_t nil_channel_serve(int64_t handle, NilValue handler) {
  NilFFIChannel *channel = channel_enter(handle);
  if (channel == NULL) {
    nil_set_error(NIL_ERR_INTERNAL, "a channel that is not open was served");
    return -1;
  }

  /* Arity checked once, before the first message.
   *
   * A handler is registered once and called many times, so this is the one moment
   * where the count can be checked against a signature — the same reason
   * `nil_callback_new` asks. `nil_callback_invoke` checks on every call as well,
   * because a callback can also arrive from C, where there is no such moment. */
  if (nil_callback_arity(handler) != 2) {
    channel_release(channel);
    nil_set_error(NIL_ERR_INTERNAL, "a channel handler takes a method and an argument");
    return -1;
  }

  for (;;) {
    int served = serve_one(channel, handler);
    if (served < 0) {
      channel_release(channel);
      return -1;
    }
    if (served == 0) {
      /* Nothing left and closed: the loop ends between messages, not only when one
       * happens to arrive. */
      break;
    }
  }

  channel_release(channel);
  return 0;
}
