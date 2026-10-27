/* gc.c — the collector.
 *
 * # The algorithm, and why this one
 *
 * Mark and sweep with an explicit root set. Not reference counting (a cycle would
 * leak forever, and Nilang builds cyclic data through maps and lists), and not a
 * conservative stack scan (it cannot see a value held only in a callee, and it
 * cannot tell a stale pointer from a live one).
 *
 * # What is collected
 *
 * Anything the roots do not reach. The roots are:
 *
 *   - every registered root slot, which the emitter writes for a local that can
 *     hold a heap value;
 *   - the objects the runtime itself holds while an operation is in flight, through
 *     the pin list;
 *   - the interned string constants, which have no pointer to them but are
 *     referenced by generated code.
 *
 * # What it does not do
 *
 * It does not run in parallel and it does not move objects. A moving collector
 * would need every raw pointer rewritten, and compiled code holds raw pointers in
 * registers where a collector cannot see them. Non-moving keeps the compiled code's
 * pointers valid for the whole collection — which is a hard requirement, not a
 * simplification: see ADR-0041.
 *
 * # The one ordering rule
 *
 * An object is pinned *before* the allocation that could collect, and unpinned
 * after. Getting this backwards is the single most damaging mistake available here:
 * a collection between the allocation and the pin frees an object that is about to
 * be used, and the program reads freed memory with no error — it just answers
 * differently than it did last run.
 */

#include "nil.h"
#include "nil_internal.h"

#include <stdio.h>
#include <stdlib.h>
#include <string.h>

/* One slot in the chain of every allocated object.
 *
 * The chain is what makes a sweep possible: walking it visits every object without
 * needing per-type destructors, and it is why allocation is a handful of
 * instructions rather than a free-list search. */
typedef struct GcBlock {
  struct GcBlock *next;
  NilObject *object;
} GcBlock;

static GcBlock *all_blocks = NULL;
static size_t live_objects = 0;

/* The root set is an array of *pointers to slots*, because a root is a location the
 * collector reads — not a value. Registering the value instead would make a
 * reassignment of the local invisible to the collector, which is the common case in
 * a loop. */
static NilValue **root_slots = NULL;
static size_t root_count = 0;
static size_t root_capacity = 0;

/* Pinned for the whole run: the string constants.
 *
 * Values, not slots. A pin names an object the runtime holds itself; a root names
 * a location the compiled code holds it in — and the compiled code reassigns that
 * location, which is why `root_slots` above is a different type. */
static NilValue *pinned = NULL;
static size_t pinned_count = 0;
static size_t pinned_capacity = 0;

/* Pinned for the duration of one operation, cleared by `nil_gc_unpin_all`. */
static NilValue *temporary = NULL;
static size_t temporary_count = 0;
static size_t temporary_capacity = 0;

static int gc_enabled = 1;
static int64_t gc_threshold = 100000;

/* Set while a collection is running.
 *
 * An allocation *inside* a sweep would splice the chain being walked into itself and
 * the traversal would loop. Rather than check for re-entry at every allocation, the
 * check is one flag read on the hot path. */
static int collecting = 0;

static void mark_object(NilObject *object);

static int is_object_pointer(NilValue value) {
  if ((value & NIL_TAG_MASK) != NIL_TAG_PTR) {
    return 0;
  }
  /* A null pointer tagged as an object would be indistinguishable from nil, and
   * dereferencing it is the crash every collector implementation has had at least
   * once. */
  return value != (NIL_TAG_PTR);
}

NilObject *nil_object_header(NilValue value) {
  if (!is_object_pointer(value)) {
    return NULL;
  }
  return (NilObject *)(value & ~((NilValue)NIL_TAG_MASK));
}

/* nil_object_kind is `header(value)->kind`, or -1 when there is no header.
 *
 * # Why this exists rather than a NULL check at each predicate
 *
 * Because every NULL check written by hand is a chance to forget one, and the
 * consequence of forgetting is a null dereference on a value the language
 * considers ordinary. `nil_int_new(4)` produces a value whose bit pattern is
 * `NIL_TAG_PTR` under a four-bit tagging scheme, and three predicates dereferenced
 * the header without checking before this function existed.
 */
int nil_object_kind(NilValue value) {
  NilObject *object = nil_object_header(value);
  return object == NULL ? -1 : (int)object->kind;
}

static NilValue value_of(NilObject *object) {
  return ((NilValue)object) | NIL_TAG_PTR;
}

/* nil_track links a new object into the collection chain.
 *
 * The block is allocated first and the object linked second, so an allocation
 * failure leaves the chain untouched. Linking an object the collector cannot find
 * would be worse than refusing to make it: the object would be freed at the next
 * collection while the program still holds it. */
static void track(NilObject *object) {
  GcBlock *block = (GcBlock *)malloc(sizeof(GcBlock));
  if (block == NULL) {
    nil_panic(NIL_ERR_OOM, "out of memory while tracking an object");
    return;
  }
  block->object = object;
  block->next = all_blocks;
  all_blocks = block;
  live_objects++;
}

/* ── Roots ────────────────────────────────────────────────────────────── */

/* next_capacity is how big an array of `needed` entries should become.
 *
 * Doubling rather than a fixed increment: the root set and the pin lists grow once
 * per value in some loops, and a fixed increment would make every one of those
 * reallocations a copy of the whole array. The floor of 64 means the first few
 * never allocate twice for a handful of entries.
 */
static size_t next_capacity(size_t needed, size_t current) {
  size_t size = current == 0 ? 64 : current;
  while (size < needed) {
    size *= 2;
  }
  return size;
}

/* Two typed growers rather than one that takes `void **`.
 *
 * A single `void **` version would be four characters shorter at each call site and
 * would rely on reading one pointer type through another, which C does not define.
 * This is a garbage collector: a bug that appears as a stray write in a release
 * build is precisely the bug this library exists to prevent, so the two functions
 * stay typed.
 */
static void grow_slots(NilValue ***array, size_t *capacity, size_t needed) {
  if (needed <= *capacity) {
    return;
  }
  size_t size = next_capacity(needed, *capacity);
  NilValue **grown = (NilValue **)realloc(*array, size * sizeof(NilValue *));
  if (grown == NULL) {
    nil_panic(NIL_ERR_OOM, "out of memory while growing the root set");
    return;
  }
  *array = grown;
  *capacity = size;
}

static void grow_values(NilValue **array, size_t *capacity, size_t needed) {
  if (needed <= *capacity) {
    return;
  }
  size_t size = next_capacity(needed, *capacity);
  NilValue *grown = (NilValue *)realloc(*array, size * sizeof(NilValue));
  if (grown == NULL) {
    nil_panic(NIL_ERR_OOM, "out of memory while growing a pin list");
    return;
  }
  *array = grown;
  *capacity = size;
}

void nil_gc_root_register(NilValue *slot) {
  if (slot == NULL) {
    return;
  }
  grow_slots(&root_slots, &root_capacity, root_count + 1);
  root_slots[root_count++] = slot;
}

void nil_gc_root_unregister(NilValue *slot) {
  for (size_t i = 0; i < root_count; i++) {
    if (root_slots[i] != slot) {
      continue;
    }
    /* A swap rather than a shift: the collector does not care about the order, and
     * a root registered and unregistered in a loop is the common case — shifting
     * every later root to close the gap is O(n) per removal. */
    root_slots[i] = root_slots[--root_count];
    return;
  }
}

/* ── Pins ─────────────────────────────────────────────────────────────── */

void nil_gc_pin(NilValue value) {
  if (!is_object_pointer(value)) {
    return;
  }
  grow_values(&temporary, &temporary_capacity, temporary_count + 1);
  temporary[temporary_count++] = value;
}

void nil_gc_unpin_all(void) { temporary_count = 0; }

/* nil_gc_unpin removes one value from the permanent pin list.
 *
 * # Why there is exactly one of these
 *
 * Everything else pinned permanently is pinned forever: a string constant lives
 * for the whole run, and there is nothing to release. A callback is different — C
 * holds one, calls it, and finishes with it — so its pin has an end, and without
 * this the only way to end one is to stop pinning, which would also release every
 * string literal in the program.
 *
 * # Why the *first* match rather than the last
 *
 * The same value may be pinned twice, when two things each asked to keep it. The
 * two pins are the same promise, so removing one of them is right: the other still
 * holds. Removing the last would unpin a value something else still holds, which is
 * the failure this function could cause and the one it is written to avoid.
 *
 * # Why a value that is not pinned is not an error
 *
 * Because `nil_callback_release` is something C code calls, possibly twice, on a
 * pointer it may or may not own. A double release is a bug in the caller, but
 * stopping the program because of one leaves the process in a worse state than
 * ignoring it.
 */
void nil_gc_unpin(NilValue value) {
  for (size_t i = 0; i < pinned_count; i++) {
    if (pinned[i] != value) {
      continue;
    }
    for (size_t j = i + 1; j < pinned_count; j++) {
      pinned[j - 1] = pinned[j];
    }
    pinned_count--;
    return;
  }
}

void nil_gc_pin_string(NilValue value) {
  if (!is_object_pointer(value)) {
    return;
  }
  grow_values(&pinned, &pinned_capacity, pinned_count + 1);
  pinned[pinned_count++] = value;
}

/* ── Mark ─────────────────────────────────────────────────────────────── */

static void mark_object(NilObject *object) {
  if (object == NULL || object->marked) {
    return;
  }
  object->marked = 1;

  switch ((int)object->kind) {
  case NIL_OBJ_STRING:
    /* The bytes are not traced: a string owns them and they own nothing. */
    break;

  case NIL_OBJ_ARRAY: {
    NilArray *array = (NilArray *)object;
    for (size_t i = 0; i < array->header.length; i++) {
      NilValue element = array->data[i];
      if (is_object_pointer(element)) {
        mark_object(nil_object_header(element));
      }
    }
    break;
  }

  case NIL_OBJ_MAP: {
    NilMap *map = (NilMap *)object;
    for (size_t i = 0; i < map->capacity; i++) {
      if (!map->occupied[i]) {
        continue;
      }
      /* Both halves are marked. A map whose values were left unmarked would free
       * exactly the values a program is about to read, while its keys survived. */
      if (is_object_pointer(map->keys[i])) {
        mark_object(nil_object_header(map->keys[i]));
      }
      if (is_object_pointer(map->values[i])) {
        mark_object(nil_object_header(map->values[i]));
      }
    }
    break;
  }

  case NIL_OBJ_CLOSURE: {
    NilClosure *closure = (NilClosure *)object;
    for (size_t i = 0; i < closure->captureCount; i++) {
      if (is_object_pointer(closure->captures[i])) {
        mark_object(nil_object_header(closure->captures[i]));
      }
    }
    break;
  }

  case NIL_OBJ_INTBOX:
  case NIL_OBJ_FLOATBOX:
  default:
    break;
  }
}

/* ── Sweep ────────────────────────────────────────────────────────────── */

/* nil_destroy frees what an object owns beyond its header.
 *
 * It exists so the sweep does not have to know every type: a new object kind needs
 * its case added here and nowhere else. Forgetting one is not a leak of the object
 * itself — its memory is still freed — it is a leak of the buffer it points at. */
void nil_destroy(NilObject *object) {
  switch ((int)object->kind) {
  case NIL_OBJ_STRING:
    free(((NilString *)object)->data);
    break;
  case NIL_OBJ_ARRAY:
    free(((NilArray *)object)->data);
    break;
  case NIL_OBJ_MAP:
    free(((NilMap *)object)->keys);
    free(((NilMap *)object)->values);
    free(((NilMap *)object)->occupied);
    free(((NilMap *)object)->order);
    break;
  case NIL_OBJ_CLOSURE:
    free(((NilClosure *)object)->captures);
    break;
  default:
    break;
  }
  free(object);
}

void nil_gc_collect(void) {
  if (collecting) {
    return;
  }
  collecting = 1;

  /* Step 1: clear every mark.
   *
   * Walking the chain rather than keeping a bitmap is deliberate: a bitmap has to
   * be indexed by an address, which means the address has to be stable, which means
   * the heap cannot be a moving allocator. The chain makes no such demand. */
  for (GcBlock *block = all_blocks; block != NULL; block = block->next) {
    block->object->marked = 0;
  }

  /* Step 2: mark from the roots. */
  for (size_t i = 0; i < root_count; i++) {
    NilValue value = *root_slots[i];
    if (is_object_pointer(value)) {
      mark_object(nil_object_header(value));
    }
  }
  for (size_t i = 0; i < temporary_count; i++) {
    mark_object(nil_object_header(temporary[i]));
  }
  for (size_t i = 0; i < pinned_count; i++) {
    mark_object(nil_object_header(pinned[i]));
  }

  /* Step 3: sweep.
   *
   * The freed block is unlinked *before* it is freed, and `next` is read before the
   * unlink. Unlinking after the free reads freed memory to find out where to
   * unlink, which turns every collection into a second chance at the same bug. */
  GcBlock *block = all_blocks;
  GcBlock *previous = NULL;

  while (block != NULL) {
    GcBlock *next = block->next;

    if (block->object->marked) {
      previous = block;
    } else {
      if (previous == NULL) {
        all_blocks = next;
      } else {
        previous->next = next;
      }
      nil_destroy(block->object);
      free(block);
      live_objects--;
    }
    block = next;
  }

  collecting = 0;
}

static void maybe_collect(void) {
  if (!gc_enabled || collecting) {
    return;
  }
  if ((int64_t)live_objects < gc_threshold) {
    return;
  }
  nil_gc_collect();

  /* The threshold is raised above the live set rather than reset. Resetting means a
   * program whose working set is larger than the threshold collects on every
   * allocation and never finishes. Raising it to one and a half times what survived
   * means the next collection comes after that much has been allocated again. */
  int64_t next = (int64_t)live_objects * 3 / 2;
  gc_threshold = next > 100000 ? next : 100000;
}

void *nil_object_new(uint8_t kind, size_t bytes) {
  /* calloc, not malloc: the object is linked into the collection chain before its
   * fields are written, and a collection in that window would read a `next` that is
   * whatever the allocator returned. */
  NilObject *object = (NilObject *)calloc(1, bytes);
  if (object == NULL) {
    nil_panic(NIL_ERR_OOM, "out of memory");
    return NULL;
  }
  object->kind = kind;
  object->marked = 0;
  object->next = NULL;
  /* Collect before linking: the new object is not yet in the chain, so the
   * sweep cannot see it. Linking first and collecting second frees the very
   * object just allocated whenever the threshold is reached, and the caller
   * then writes into freed memory. */
  maybe_collect();
  track(object);
  return object;
}

NilValue nil_object_pointer(void *object) {
  return object == NULL ? NIL_TAG_NIL : value_of((NilObject *)object);
}

void nil_gc_set_threshold(int64_t objects) { gc_threshold = objects > 0 ? objects : 100000; }
void nil_gc_disable(void) { gc_enabled = 0; }
void nil_gc_enable(void) { gc_enabled = 1; }
int64_t nil_gc_object_count(void) { return (int64_t)live_objects; }