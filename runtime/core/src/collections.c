/* collections.c — arrays and maps.
 *
 * # Why arrays grow
 *
 * Nilang's `push` returns a *new* array rather than mutating the one it was called
 * on, so the language guarantees no aliasing surprises. The C representation grows
 * in place anyway, because the value is behind a pointer and a program holding two
 * references to the same array has already been given the same aliasing — which is
 * why the language's copy-on-push is implemented by copying, not by sharing.
 *
 * # Why maps are open-addressed
 *
 * A Nilang map is usually small. A chain of buckets answers in one and a half
 * probes on average; an open-addressed table with the load factor capped at one
 * half answers in one, and a map with ten keys is the common case in a program that
 * reads configuration or a record.
 *
 * No tombstones: a key is never removed, only added or replaced, so an empty slot
 * is always empty. That is what lets the probe sequence stop at the first empty
 * slot — a tombstone would have to be skipped, and a deletion that did not leave
 * one would break every later lookup for every key that probed past it.
 *
 * # The rule that governs hashing
 *
 * Two values that compare equal must hash equally. Everything else about this file
 * follows from that, and violating it produces a map that stores a value somewhere
 * its own lookup will never reach.
 */

#include "nil.h"
#include "nil_internal.h"

#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#define NIL_MAP_LOAD_NUMERATOR 1
#define NIL_MAP_LOAD_DENOMINATOR 2

/* ── Indexing ─────────────────────────────────────────────────────────── */

/* nil_index_string reads one character of a string.
 *
 * Out of range is nil rather than an error, which is the one place indexing
 * behaves differently on strings than on lists. A string's length is a property of
 * its contents, so a program walking one cannot know its bounds; a list's length is
 * a property of the program, so an out-of-range index there is a bug worth stopping
 * for.
 *
 * The result is a one-character string rather than a code point, because that is
 * what a program concatenating pieces expects: `"abc"[0] + "!"` is `"a!"`.
 */
static NilValue nil_index_string(NilValue container, NilValue key) {
  NilString *string = (NilString *)nil_object_header(container);
  int64_t index = nil_int_value(key);
  if (index < 0) {
    return NIL_TAG_NIL;
  }
  size_t offset = nil_string_byte_offset(string, index);
  if (offset >= string->header.length) {
    return NIL_TAG_NIL;
  }
  size_t width = utf8_sequence_length(string->data + offset, string->header.length - offset);
  return nil_string_lit(string->data + offset, width);
}

/* The two narrowers below are defined next to the types they narrow.
 *
 * `nil_index_get` needs both of them before either exists, because it has to
 * inspect a container to decide which of the two it is holding — and a function
 * that could not look would have to call the general `nil_object_kind` and read
 * the fields itself, which is the same knowledge in a second place. */
static NilArray *array_of(NilValue value);
static NilMap *map_of(NilValue value);

/* nil_index_get reads `container[key]`, whatever the container is.
 *
 * # Why the runtime dispatches rather than the compiler choosing
 *
 * The compiler does not know whether `x[k]` means "element k of a list" or "the
 * value under key k in a map" — at MIR level `m["k"]` and `xs[1]` are the same
 * instruction. Emitting a call to `nil_array_get` therefore made a map lookup fail
 * with "cannot index a value that is not an array", and emitting one that guessed
 * would have meant the guess living in the compiler.
 *
 * This is the same reasoning as arithmetic being a runtime call (ADR-0037): the
 * representation carries the type and only something that understands it can answer.
 * See ADR-0043.
 *
 * The semantics per container:
 *
 *   - a list: `key` is a position, out of range is nil;
 *   - a map: `key` is any value, a missing key is nil;
 *   - a string: `key` is a character position, out of range is nil;
 *   - anything else: an error, because no later check can make it work.
 *
 * # Why reading past the end is nil and writing past it is not
 *
 * `nil_index_set` on an out-of-range position stops the program; this returns nil.
 * The asymmetry is deliberate and it is the interpreter's rule, not a convenience.
 *
 * A read is a question — "what is at position 10?" has an answer, and for a
 * three-element list that answer is "nothing there", which is what nil already
 * means. Nil is also what a *missing map key* answers, so a lookup that may miss
 * needs no check either way: `h[k]` and `xs[i]` can both be answered with the same
 * question of the value.
 *
 * A write is different in kind. Writing to position 10 of a three-element list
 * cannot be carried out — there is no slot — so the only two honest outcomes are
 * refusing or growing the list. Growing silently would change the length of
 * something the program can also read, and refusing says plainly that the program
 * is wrong. It is wrong, and stopping is how it finds out.
 */
NilValue nil_index_get(NilValue container, NilValue key) {
  switch (nil_object_kind(container)) {
  case NIL_OBJ_ARRAY:
    /* A list is indexed by position, and the key has to be a number.
     *
     * Without this check `xs["k"]` reads element zero: `nil_int_value` on a string
     * answers 0 rather than refusing, so a program indexing a list by name would
     * quietly get its first element forever. That is the worst shape of bug — no
     * error, no crash, and an answer that looks plausible.
     */
    if (!nil_is_number(key)) {
      nil_panic(NIL_ERR_TYPE, "cannot index a list with something that is not a position");
    }
    {
      /* The bound is checked here rather than left to `nil_array_get`, which
       * stops the program. `nil_array_get` is the primitive: a caller that asks it
       * for element 10 of 3 has asked for something impossible and should be told
       * so. `nil_index_get` is the language's subscript operator, and the language
       * says a read past the end is nil. Both answers are right; they answer
       * different questions. */
      NilArray *array = array_of(container);
      int64_t position = nil_int_value(key);
      if (position < 0 || position >= (int64_t)array->header.length) {
        return NIL_TAG_NIL;
      }
    }
    return nil_array_get(container, key);

  case NIL_OBJ_MAP:
    return nil_map_get(container, key);

  case NIL_OBJ_STRING:
    return nil_index_string(container, key);

  default:
    nil_panic(NIL_ERR_TYPE, "cannot index a value that is not a list, a map or a string");
    break;
  }
  return NIL_TAG_NIL;
}

/* nil_index_set writes `container[key] = value`.
 *
 * A string is not writable, and saying so is better than silently discarding the
 * assignment: a program that assigns into a string has a bug, and a program that
 * carries on with the string unchanged has the same bug and no clue.
 */
NilValue nil_index_set(NilValue container, NilValue key, NilValue value) {
  switch (nil_object_kind(container)) {
  case NIL_OBJ_ARRAY:
    return nil_array_set(container, key, value);
  case NIL_OBJ_MAP:
    return nil_map_set(container, key, value);
  case NIL_OBJ_STRING:
    nil_panic(NIL_ERR_TYPE, "cannot assign into a string");
    break;
  default:
    nil_panic(NIL_ERR_TYPE, "cannot assign into a value that is not a list or a map");
  }
  return NIL_TAG_NIL;
}

/* ── Arrays ───────────────────────────────────────────────────────────── */

NilValue nil_array_new(int64_t length) {
  if (length < 0) {
    length = 0;
  }
  NilArray *array = (NilArray *)nil_object_new(NIL_OBJ_ARRAY, sizeof(NilArray));
  if (array == NULL) {
    return NIL_TAG_NIL;
  }
  array->header.length = (uint32_t)length;
  array->capacity = (size_t)length;
  array->data = length == 0 ? NULL : (NilValue *)calloc((size_t)length, sizeof(NilValue));
  if (length != 0 && array->data == NULL) {
    nil_panic(NIL_ERR_OOM, "out of memory allocating an array");
  }
  return nil_object_pointer(array);
}

static NilArray *array_of(NilValue value) {
  if (nil_object_kind(value) != NIL_OBJ_ARRAY) {
    return NULL;
  }
  return (NilArray *)nil_object_header(value);
}

/* ── List operations ──────────────────────────────────────────────────── */

/* nil_first and nil_last are `xs.first` and `xs.last`.
 *
 * They return nil on an empty list rather than an error, because "the first element
 * of nothing" has no answer and every language with these operations answers nil.
 *
 * Separate functions rather than a special index, because `xs[0]` on an empty list
 * *is* an error. The difference between asking for the first element and asking for
 * element zero is the difference between a question with no answer and a mistake.
 */
NilValue nil_first(NilValue list) {
  NilArray *array = array_of(list);
  if (array == NULL) {
    nil_panic(NIL_ERR_TYPE, "cannot take the first element of something that is not a list");
  }
  if (array->header.length == 0) {
    return NIL_TAG_NIL;
  }
  return array->data[0];
}

NilValue nil_last(NilValue list) {
  NilArray *array = array_of(list);
  if (array == NULL) {
    nil_panic(NIL_ERR_TYPE, "cannot take the last element of something that is not a list");
  }
  if (array->header.length == 0) {
    return NIL_TAG_NIL;
  }
  return array->data[array->header.length - 1];
}

/* nil_rest is `rest(xs)` — everything after the first element.
 *
 * Nil on an empty list, for the same reason `first` and `last` are: there is no
 * answer, and nil is what "no answer" already means.
 *
 * A *new* list, because Nilang lists are values and `push` returns a new list
 * rather than changing the one it was given. Sharing the storage would give
 * `rest(xs)[0] = 9` a way to reach into `xs`, which is exactly the aliasing the
 * language does not have.
 */
NilValue nil_rest(NilValue list) {
  NilArray *array = array_of(list);
  if (array == NULL) {
    nil_panic(NIL_ERR_TYPE, "cannot take the rest of something that is not a list");
  }
  if (array->header.length == 0) {
    return NIL_TAG_NIL;
  }

  NilArray *result =
      (NilArray *)nil_object_new(NIL_OBJ_ARRAY, sizeof(NilArray));
  if (result == NULL) {
    return NIL_TAG_NIL;
  }
  size_t kept = array->header.length - 1;
  result->header.length = (uint32_t)kept;
  result->capacity = kept;
  result->data = kept == 0 ? NULL : (NilValue *)calloc(kept, sizeof(NilValue));
  if (kept != 0 && result->data == NULL) {
    nil_panic(NIL_ERR_OOM, "out of memory allocating a list for rest");
  }

  /* Both lists are pinned across the copy. Nothing here allocates after the two
   * `calloc`s, so the pin list is cleared rather than left around. */
  nil_gc_pin(list);
  memcpy(result->data, array->data + 1, kept * sizeof(NilValue));
  nil_gc_unpin_all();
  return nil_object_pointer(result);
}

int64_t nil_array_len(NilValue value) {
  NilArray *array = array_of(value);
  return array == NULL ? 0 : (int64_t)array->header.length;
}

/* nil_bounds_error reports an out-of-range index and does not return.
 *
 * One function for arrays and strings, so the message is identical for both: a
 * program that indexed past the end of a list and past the end of a string gets the
 * same shape of answer and the same `NIL_ERR_INDEX` code. */
void nil_bounds_error(const char *kind, int64_t index, int64_t length) {
  char message[160];
  snprintf(message, sizeof(message), "%s index %lld is out of range, length is %lld", kind,
           (long long)index, (long long)length);
  nil_panic(NIL_ERR_INDEX, message);
}

NilValue nil_array_get(NilValue value, NilValue index) {
  NilArray *array = array_of(value);
  if (array == NULL) {
    nil_panic(NIL_ERR_TYPE, "cannot index a value that is not an array");
  }
  int64_t position = nil_int_value(index);
  if (position < 0 || position >= (int64_t)array->header.length) {
    nil_bounds_error("array", position, (int64_t)array->header.length);
  }
  return array->data[position];
}

NilValue nil_array_set(NilValue value, NilValue index, NilValue element) {
  NilArray *array = array_of(value);
  if (array == NULL) {
    nil_panic(NIL_ERR_TYPE, "cannot index a value that is not an array");
  }
  int64_t position = nil_int_value(index);
  if (position < 0 || position >= (int64_t)array->header.length) {
    nil_bounds_error("array", position, (int64_t)array->header.length);
  }
  /* No pin: nothing here allocates, and a pin whose only purpose is to survive an
   * allocation that cannot happen reads as though one could. */
  array->data[position] = element;
  return element;
}

void nil_array_push(NilValue value, NilValue element) {
  NilArray *array = array_of(value);
  if (array == NULL) {
    nil_panic(NIL_ERR_TYPE, "cannot push to a value that is not an array");
  }

  if (array->header.length == array->capacity) {
    size_t capacity = array->capacity == 0 ? 4 : array->capacity * 2;
    NilValue *data = (NilValue *)realloc(array->data, capacity * sizeof(NilValue));
    if (data == NULL) {
      nil_panic(NIL_ERR_OOM, "out of memory growing an array");
    }
    /* The new space is zeroed. A hole read as a null tag would be a value the
     * program never stored, and the language has no `undefined` — every read has
     * to have an answer, and "nothing is there yet" has to be one of them. */
    memset(data + array->capacity, 0, (capacity - array->capacity) * sizeof(NilValue));
    array->data = data;
    array->capacity = capacity;
  }
  array->data[array->header.length++] = element;
}

/* ── Maps ─────────────────────────────────────────────────────────────── */

NilValue nil_map_new(int64_t capacity) {
  NilMap *map = (NilMap *)nil_object_new(NIL_OBJ_MAP, sizeof(NilMap));
  if (map == NULL) {
    return NIL_TAG_NIL;
  }
  /* A power of two, because the probe sequence masks with `capacity - 1`. A
   * non-power-of-two capacity would need a modulo per probe, and a modulo is the
   * difference between a lookup being a few instructions and being a division. */
  size_t wanted = 4;
  size_t asked = capacity <= 0 ? 0 : (size_t)capacity;
  while (wanted < asked * NIL_MAP_LOAD_DENOMINATOR) {
    wanted *= 2;
  }

  map->capacity = wanted;
  map->count = 0;
  map->keys = (NilValue *)calloc(wanted, sizeof(NilValue));
  map->values = (NilValue *)calloc(wanted, sizeof(NilValue));
  map->occupied = (uint8_t *)calloc(wanted, sizeof(uint8_t));
  /* Slot-sized like the others rather than count-sized, because growing rehashes
   * and every slot index in here then moves. One array sized to the table is one
   * realloc to get wrong instead of two. */
  map->order = (size_t *)calloc(wanted, sizeof(size_t));
  if (map->keys == NULL || map->values == NULL || map->occupied == NULL ||
      map->order == NULL) {
    nil_panic(NIL_ERR_OOM, "out of memory allocating a map");
  }
  return nil_object_pointer(map);
}

static NilMap *map_of(NilValue value) {
  if (nil_object_kind(value) != NIL_OBJ_MAP) {
    return NULL;
  }
  return (NilMap *)nil_object_header(value);
}

/* hash_word mixes eight bytes into a running FNV-1a hash.
 *
 * One function rather than an inline loop at each of the three call sites, because
 * a hash written three times is a hash written three different ways — and the
 * property that matters here holds only if the bytes are mixed in the same order
 * every time.
 *
 * The byte order is fixed little-endian rather than left to the machine, so a value
 * hashes the same on a big-endian target. The table is never persisted, so this is
 * about two machines probing in the same order rather than about reading a table
 * off disk. */
static uint64_t hash_word(uint64_t hash, uint64_t word) {
  for (size_t i = 0; i < sizeof(uint64_t); i++) {
    hash ^= (unsigned char)(word >> (i * 8));
    hash *= 1099511628211ULL;
  }
  return hash;
}

/* map_hash is FNV-1a over the value's identity, and "identity" is what `map_equal`
 * compares.
 *
 * # Why this cannot hash the pointer
 *
 * It did, and the map broke in a way that looked like a growth bug: `m["k0"] = 1`
 * followed by `m["k0"]` missed, because the two `"k0"` literals are two different
 * string objects at two different addresses. Growth made it worse and made it look
 * intermittent, since a table that had not yet rehashed could keep hitting the same
 * slot by accident.
 *
 * # What is hashed
 *
 * A string by its bytes, a number by its value, everything else by its
 * representation. The tag is part of an inline value's identity — `null`, the two
 * booleans and an inline integer must hash differently because `map_equal` treats
 * them differently. For a pointer the tag is *not* hashed: it says what the object
 * is, not which object it is. */
static uint64_t map_hash(NilValue value) {
  uint64_t hash = 1469598103934665603ULL;

  if ((value & NIL_TAG_MASK) == NIL_TAG_PTR) {
    switch (nil_object_kind(value)) {
    case NIL_OBJ_STRING: {
      NilString *string = (NilString *)nil_object_header(value);
      const unsigned char *bytes = (const unsigned char *)string->data;
      for (uint32_t i = 0; i < string->header.length; i++) {
        hash ^= bytes[i];
        hash *= 1099511628211ULL;
      }
      /* The length is mixed in so that two strings sharing a prefix do not sit on
       * the same chain for their whole common length. */
      hash ^= (uint64_t)string->header.length;
      hash *= 1099511628211ULL;
      return hash;
    }

    case NIL_OBJ_INTBOX: {
      /* The number, not the box. A program that puts 5 into a map and looks up 5
       * has to find it whether 5 was inlined or boxed, and the two are different
       * addresses. */
      return hash_word(hash, (uint64_t)((NilBox *)nil_object_header(value))->value);
    }

    case NIL_OBJ_FLOATBOX: {
      /* The bits, not the value converted to a number: two floats that differ only
       * below the print precision are still different values, and rounding them
       * together would make one key stand for two. */
      double number = 0;
      memcpy(&number, &((NilBox *)nil_object_header(value))->value, sizeof(double));
      uint64_t bits = 0;
      memcpy(&bits, &number, sizeof(bits));
      return hash_word(hash, bits);
    }

    default:
      break;
    }
  }

  /* An inline value: null, a boolean, an inline integer, or a tagged null. Its
   * whole word is its identity. */
  return hash_word(hash, (uint64_t)value);
}

/* map_equal is value equality, not address equality.
 *
 * It agrees with `map_hash` about what makes two values the same. Two strings with
 * the same characters are the same key even when they are different objects; two
 * numbers with the same value are the same key whether one is boxed and one is not;
 * and an integer and a float holding the same number are the same key, because
 * `5 == 5.0` is true in every language that has both and a map that disagreed would
 * make `m[5] = x; m[5.0]` miss.
 *
 * Comparing addresses would make a map's behaviour depend on whether the collector
 * happened to move something — which it does not today, and which it must not be
 * allowed to, because compiled code holds raw pointers in registers where a
 * collector cannot rewrite them. */
static int map_equal(NilValue a, NilValue b) {
  if (a == b) {
    return 1;
  }
  if (nil_is_string(a) && nil_is_string(b)) {
    NilString *left = (NilString *)nil_object_header(a);
    NilString *right = (NilString *)nil_object_header(b);
    if (left->header.length != right->header.length) {
      return 0;
    }
    return memcmp(left->data, right->data, left->header.length) == 0;
  }
  if (nil_is_number(a) && nil_is_number(b)) {
    if (nil_is_float(a) || nil_is_float(b)) {
      /* Across the two, ask the comparison that already knows the answer rather
       * than restating it here — the two would drift, and the drift would only
       * show up in a program that mixed the types in one map. */
      return nil_int_cmp(a, b, 0) == 1;
    }
    return nil_int_value(a) == nil_int_value(b);
  }
  return 0;
}

/* map_find returns the slot holding a key, or the slot it should occupy.
 *
 * One function for both, because the probe sequence is the same and the only
 * difference is whether the probe ended on a match or on an empty slot. A separate
 * lookup and insert would be two copies of a sequence that has to agree. */
static size_t map_find(NilMap *map, NilValue key, int *found) {
  size_t mask = map->capacity - 1;
  size_t slot = (size_t)(map_hash(key) & mask);

  for (size_t probe = 0; probe < map->capacity; probe++) {
    if (!map->occupied[slot]) {
      *found = 0;
      return slot;
    }
    if (map_equal(map->keys[slot], key)) {
      *found = 1;
      return slot;
    }
    slot = (slot + 1) & mask;
  }

  /* A full table means the load factor was violated, which `nil_map_set` prevents
   * by growing first. Returning slot 0 without saying so would write the key into
   * whatever is there and corrupt the map silently. */
  nil_set_error(NIL_ERR_INTERNAL, "map table is full");
  *found = 0;
  return 0;
}

static void map_grow(NilMap *map) {
  size_t capacity = map->capacity * 2;

  NilValue *keys = (NilValue *)calloc(capacity, sizeof(NilValue));
  NilValue *values = (NilValue *)calloc(capacity, sizeof(NilValue));
  uint8_t *occupied = (uint8_t *)calloc(capacity, sizeof(uint8_t));
  size_t *order = (size_t *)calloc(capacity, sizeof(size_t));
  if (keys == NULL || values == NULL || occupied == NULL || order == NULL) {
    nil_panic(NIL_ERR_OOM, "out of memory growing a map");
  }

  NilValue *oldKeys = map->keys;
  NilValue *oldValues = map->values;
  uint8_t *oldOccupied = map->occupied;
  size_t *oldOrder = map->order;
  size_t oldCapacity = map->capacity;
  /* Taken before `count` is reset, because it is how many entries have a place in
   * the order to be carried across. */
  size_t oldCount = map->count;

  map->keys = keys;
  map->values = values;
  map->occupied = occupied;
  map->order = order;
  map->capacity = capacity;
  map->count = 0;

  for (size_t i = 0; i < oldCapacity; i++) {
    if (!oldOccupied[i]) {
      continue;
    }
    int found = 0;
    size_t slot = map_find(map, oldKeys[i], &found);
    map->keys[slot] = oldKeys[i];
    map->values[slot] = oldValues[i];
    map->occupied[slot] = 1;
    map->count++;
  }

  /* The order is carried across *by key*, not by old slot index.
   *
   * Rehash moves every key, so an index into the old table says nothing about where
   * the key went. Walking the old order and asking the new table where each key
   * landed reproduces the written order exactly; walking the old *slots* would give
   * the new table's hash order, which is the thing this array exists to not be. */
  for (size_t i = 0; i < oldCount; i++) {
    int found = 0;
    order[i] = map_find(map, oldKeys[oldOrder[i]], &found);
  }

  free(oldKeys);
  free(oldValues);
  free(oldOccupied);
  free(oldOrder);
}

NilValue nil_map_get(NilValue value, NilValue key) {
  NilMap *map = map_of(value);
  if (map == NULL) {
    nil_panic(NIL_ERR_TYPE, "cannot look up a key in a value that is not a map");
  }
  if (map->count == 0) {
    return NIL_TAG_NIL;
  }
  int found = 0;
  size_t slot = map_find(map, key, &found);
  return found ? map->values[slot] : NIL_TAG_NIL;
}

NilValue nil_map_set(NilValue value, NilValue key, NilValue element) {
  NilMap *map = map_of(value);
  if (map == NULL) {
    nil_panic(NIL_ERR_TYPE, "cannot assign a key on a value that is not a map");
  }

  /* Grow before probing, so the insertion that triggers the growth cannot find the
   * table already full. */
  if ((map->count + 1) * NIL_MAP_LOAD_DENOMINATOR >= map->capacity * NIL_MAP_LOAD_NUMERATOR) {
    map_grow(map);
  }

  /* Pinned across the insertion. `map_grow` allocates, and so does the key's own
   * construction if the caller built one; a collection in between would free the
   * map being written to. */
  nil_gc_pin(value);
  nil_gc_pin(key);
  nil_gc_pin(element);

  int found = 0;
  size_t slot = map_find(map, key, &found);
  map->keys[slot] = key;
  map->values[slot] = element;
  if (!found) {
    map->occupied[slot] = 1;
    /* Recorded *before* the count is bumped, because the slot is the new entry's
     * and `order[count]` is where the next one goes.
     *
     * Only for a key that was not there. Replacing a value keeps the key's place in
     * the written order — `m["a"] = 1; m["b"] = 2; m["a"] = 3` is still `a, b`,
     * because what was written second is `b`. */
    map->order[map->count] = slot;
    map->count++;
  }
  map->header.length = (uint32_t)map->count;

  nil_gc_unpin_all();
  return element;
}

int64_t nil_map_len(NilValue value) {
  NilMap *map = map_of(value);
  return map == NULL ? 0 : (int64_t)map->count;
}

/* ── Members ──────────────────────────────────────────────────────────── */

NilValue nil_get_field(NilValue value, NilValue name) {
  NilMap *map = map_of(value);
  if (map == NULL) {
    /* Dot access is not indexing. A member that is not there is a mistake, and
     * answering nil would be indistinguishable from a member that holds nil — so
     * the program would carry on producing wrong answers instead of stopping. */
    nil_panic(NIL_ERR_TYPE, "cannot read a member of a value that is not a map");
  }
  return nil_map_get(value, name);
}


/* ── Calls ────────────────────────────────────────────────────────────── */

NilValue nil_closure_new(NilValue (*function)(NilValue *args, int64_t count)) {
  NilClosure *closure = (NilClosure *)nil_object_new(NIL_OBJ_CLOSURE, sizeof(NilClosure));
  if (closure == NULL) {
    return NIL_TAG_NIL;
  }
  closure->function = function;
  closure->captures = NULL;
  closure->captureCount = 0;
  return nil_object_pointer(closure);
}

/* nil_call invokes a callable with an argument array.
 *
 * The argument array is a Nilang array, so the callee sees the same representation
 * it would see if the call had been compiled directly. A `void*` would mean the
 * callee and the runtime had to agree on a second calling convention, and the two
 * would drift the first time either changed. */
NilValue nil_call(NilValue callee, NilValue args) {
  if (nil_object_kind(callee) != NIL_OBJ_CLOSURE) {
    return NIL_TAG_NIL;
  }

  NilClosure *closure = (NilClosure *)nil_object_header(callee);
  if (closure->function == NULL) {
    return NIL_TAG_NIL;
  }

  NilArray *arguments = array_of(args);
  size_t count = arguments == NULL ? 0 : arguments->header.length;

  /* The callee and its arguments are pinned across the call: the callee can
   * allocate, and a collection in the middle of it would otherwise be free to free
   * the arguments it is reading. */
  nil_gc_pin(callee);
  nil_gc_pin(args);
  NilValue result = closure->function(arguments == NULL ? NULL : arguments->data,
                                      (int64_t)count);
  nil_gc_unpin_all();
  return result;
}

/* ── Raw allocation ───────────────────────────────────────────────────── */

NilValue nil_alloc(int64_t size, int64_t zeroed) {
  (void)zeroed; /* always zeroed; see nil.h */
  return nil_array_new(size);
}
