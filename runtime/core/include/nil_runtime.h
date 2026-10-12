/* nil_runtime.h — Nilang mobile embedder C ABI (L8.1).
 *
 * What this header promises:
 *
 * A mobile host (Android JNI, iOS ObjC, HarmonyOS NAPI) embeds the Nilang
 * core as a small shared library (`libnilang_core.so` / `.dylib`) and talks
 * to it only through this header. The five lifecycle entry points below are
 * the whole contract; everything else is a plain C struct so that no host
 * needs to understand the tagged `NilValue` representation.
 *
 * String safety:
 *
 * Strings always cross as an explicit (pointer, byte-length) pair
 * (`NilRuntimeString`). Nothing here is NUL-terminated on the way in, so a
 * Bengali string with embedded NUL bytes survives the crossing intact.
 *
 * Versioning:
 *
 * `nil_ffi_abi_v1` is a link guard (same rule as the FFI ABI): the symbol's
 * *name* carries the version so that a v1 object never links cleanly against
 * a v2-only runtime. Generated/host code that embeds a bundle must call it
 * (or reference it) once per module.
 */

#ifndef NIL_RUNTIME_H
#define NIL_RUNTIME_H

#include <stddef.h>
#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

#define NIL_RUNTIME_ABI_VERSION 1

/* A string crossing the ABI: pointer plus byte length (UTF-8, not NUL
 * terminated). `len` is in bytes, never characters. */
typedef struct NilRuntimeString {
  const char *ptr;
  int64_t len;
} NilRuntimeString;

/* A raw bundle crossing the ABI: pointer plus byte length. */
typedef struct NilRuntimeBundle {
  const uint8_t *data;
  int64_t len;
} NilRuntimeBundle;

/* Event forwarded from the host UI thread. `kind` names the event
 * ("click", "resume", ...); `payload` is an opaque UTF-8 JSON document. */
typedef struct NilRuntimeEvent {
  NilRuntimeString kind;
  NilRuntimeString payload;
} NilRuntimeEvent;

/* Callback the runtime invokes on the host (e.g. "render", "log").
 * `ctx` is the host's opaque pointer, handed back on every call. */
typedef void (*NilRuntimeCallback)(void *ctx, NilRuntimeEvent event);

/* Lifecycle entry points. Return 0 on success, nonzero nil exit code else. */
int32_t nil_init(void);
int32_t nil_run_bundle(NilRuntimeBundle bundle);
int32_t nil_dispatch_event(NilRuntimeEvent event);
void nil_set_callback(NilRuntimeCallback cb, void *ctx);
void nil_quit(void);

/* String-safe helpers. All take explicit lengths; none read past `len`. */
int64_t nil_runtime_string_len(NilRuntimeString s);
int32_t nil_runtime_string_equal(NilRuntimeString a, NilRuntimeString b);

/* Link guard: reference once per embedding module. Empty body, forever. */
void nil_ffi_abi_v1(void);

#ifdef __cplusplus
}
#endif

#endif /* NIL_RUNTIME_H */
