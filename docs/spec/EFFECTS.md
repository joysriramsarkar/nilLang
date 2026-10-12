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

**What is actually enforced.** Constraint 1 is implemented: a call inside a
`pure fn` to a function that is not itself `pure` reports `E0202` at the call.
Constraints 2 and 3 are not, and this document does not claim them. A `pure fn`
is a claim the developer makes and the checker verifies as far as it can — it
cannot prove the claim, which is why the modifier is written rather than inferred.

---

## 4. Capability Clauses: `requires`

A function that cannot do its job without a system capability declares it:

```nil
fn readConfig(path: String) -> String requires "filesystem" {
    return readFile(path);
}

fn fetch(url: String) -> String requires "network", "timeout" { ... }
```

1. **The clause is checked statically, at every call site.** A function whose
   capability is missing fails the moment it is called, and that is knowable
   before the program starts. The diagnostic is `E0201`, the same code the run-time
   capability denial uses: both say the same thing — an operation needs a
   capability the project does not have — at two different times.
2. **The declaration is checked as well as each call.** A function nobody calls
   still declares an intention, and a typo in a capability name would otherwise sit
   unnoticed until the day something called it.
3. **Capability names are normalised on both sides.** Nilang has two vocabularies
   for the same capability: a lowercase operation-oriented `filesystem` and a
   capitalised resource-oriented `Filesystem` from the host side. A project that
   granted one satisfies a clause naming the other. Normalising only on the way in
   would leave the two failing to match, which is a bug that looks like a missing
   grant.
4. **`pure` and `requires` are contradictory.** A function that requires a
   capability is performing a side effect, so declaring both is reported rather
   than silently resolved.

A project with no `nil.json` has no capabilities, so any `requires` clause in one
fails. That is the intended reading: the capability genuinely is not granted, and
adding it to the manifest is how you say it is.

---

## 5. `unsafe fn`

```nil
unsafe fn readRaw(ptr: Int, len: Int) -> Array<Byte> { ... }
```

1. **`unsafe fn` says the function performs capability-gated operations and takes
   responsibility for them.** Inside it, a gated operation does not need its
   capability granted to the project. The exemption is per function and has to be
   written down, so a reader can see where it applies.
2. **The project must hold the `unsafe` capability itself**, so the exemption
   cannot be had by accident. Without it, the declaration and every call to it
   report `E0201`.
3. **An `unsafe fn`'s own `requires` clause is not checked against the project**,
   because the function has declared that it copes with the capabilities it uses.
   That is what the modifier is for.

---

## 6. Effect Composition & Polymorphism

For higher-order functions:
```nil
fn map<T, U>(items: Array<T>, transform: fn(T) -> U [Effects]) -> Array<U> [Effects]
```
The higher-order function inherits the effect set of the callback argument, propagating purity guarantees through functional abstractions.