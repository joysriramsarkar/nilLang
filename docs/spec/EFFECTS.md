# NilLang Effect System Specification
**Version:** 1.0.0-draft  
**Status:** Authoritative Normative Specification  
**Conformance Level:** Nilang 0.1 Core Freeze

---

## 1. Overview & Soundness Goals

The NilLang effect system tracks observable side effects at compile time, providing mathematical guarantees about purity, determinism, and safety.

### Cardinal Invariant:
A function cannot exhibit any operational effect that is not explicitly declared in its signature or permitted by its caller's context.

---

## 2. Closed Set of Operational Effects

| Effect | Description | Permitted Operations |
|---|---|---|
| `pure` | Mathematically pure and deterministic | Local stack computation, immutable reads. No I/O, no mutation outside call frame. |
| `read` | State or memory reading | Reads global, module, or component state. |
| `write` | State mutation | Mutates shared variables, component state, or global registers. |
| `io` | General I/O | Standard input/output, logging, console printing. |
| `network` | Socket & HTTP | Outbound or inbound network communication. |
| `storage` | Persistent storage | File system or SQLite local access. |
| `ui` | Reactive UI | Modifies DOM, native UI widget tree, or dispatches visual events. |
| `async` | Concurrency | Spawns tasks, awaits futures, pauses execution frame. |
| `unsafe` | Low-level FFI | Raw pointer arithmetic, C/Rust FFI calls. |

---

## 3. Purity Proofs & Compile-Time Enforcement

A function declared `pure`:
```nil
pure fn add(a: Int, b: Int) -> Int {
    return a + b;
}
```
**Constraints Enforced by Compiler:**
1. Cannot invoke any function that has effects other than `pure`.
2. Cannot read or write mutable global variables.
3. Cannot initiate network requests, file system operations, or hardware access.
4. Calling an impure function from a `pure` function triggers diagnostic `E0202: PurityViolation`.

---

## 4. Effect Composition & Polymorphism

For higher-order functions:
```nil
fn map<T, U>(items: Array<T>, transform: fn(T) -> U [Effects]) -> Array<U> [Effects]
```
The higher-order function inherits the effect set of the callback argument, propagating purity guarantees through functional abstractions.