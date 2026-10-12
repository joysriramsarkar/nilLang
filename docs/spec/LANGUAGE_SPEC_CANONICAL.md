# NilLang Canonical Language Specification

**Version:** 1.0.1-draft
**Status:** Compatibility Matrix Reference
**Purpose:** Authoritative cross-implementation compatibility reference

---

## 1. Introduction

This document is the canonical compatibility matrix for NilLang. It records which
language features each implementation in the Onuron ecosystem actually supports,
with the canonical Go implementation as the reference.

### Implementations compared

| Implementation | Location | Maturity |
|----------------|----------|----------|
| **nilLang (Go)** — canonical | `nilLang/` | Production: 3 engines (evaluator, VM, AOT/LLVM), full type checker, conformance suite under `nilLang/tests/crossruntime/` |
| **Rust runtime** | `onuronOS/runtime/nillang/` | Prototype: parser + JSON bytecode + UI-oriented VM |
| **alap-framework (Go)** | `alap-framework/` | Early-alpha: separate lexer/parser/NABC VM, non-fatal type checker |

### Legend

| Symbol | Meaning |
|--------|---------|
| ✅ | Implemented and executable |
| ⚠️ | Partial, syntax-only, or a non-functional stub (see Notes) |
| ❌ | Absent / not found in that implementation |

Every value below was checked against the implementation's source. Where a cell is
⚠️ or ❌ the Notes column names the reason. See `alap-framework/RELATIONSHIP.md`
and `onuronOS/runtime/nillang/README.md` for the maturity discussion.

---

## 2. Types

| Feature | Go | Rust | alap | Notes |
|---------|-----|------|------|-------|
| `Int` (64-bit signed) | ✅ | ⚠️ | ✅ | |
| `Float` (64-bit IEEE) | ✅ | ⚠️ | ✅ | |
| `String` (UTF-8) | ✅ | ⚠️ | ✅ | |
| `Bool` | ✅ | ⚠️ | ✅ | |
| `Byte` | ✅ | ❌ | ❌ | alap: `ValBytes` defined but never constructed |
| `Null` | ✅ | ⚠️ | ✅ | |
| `Void` | ✅ | ❌ | ⚠️ | alap: type-level only, no void value |
| `Any` | ✅ | ❌ | ⚠️ | alap: checker fallback type |
| `?T` (Optional) | ✅ | ❌ | ⚠️ | alap: checker only, runtime unenforced |
| `Result<T,E>` | ✅ | ❌ | ❌ | alap: type kind only; no constructors |
| `A \| B` (Union) | ✅ | ❌ | ⚠️ | alap: parsed + checker, no runtime |
| `Array<T>` | ✅ | ❌ | ✅ | |
| `Map<K,V>` | ✅ | ❌ | ⚠️ | alap: object-backed; no map literal/type |
| Numeric widths (`i8`..`u64`, `f32`/`f64`) | ✅ | ❌ | ⚠️ | alap: checker only; int64/float64 at runtime |

---

## 3. Operators

| Feature | Go | Rust | alap | Notes |
|---------|-----|------|------|-------|
| Arithmetic `+` `-` `*` `/` `%` | ✅ | ⚠️ | ✅ | |
| Comparison `==` `!=` `<` `<=` `>` `>=` | ✅ | ⚠️ | ✅ | |
| Logical `&&` `\|\|` `!` | ✅ | ⚠️ | ✅ | |
| Bitwise `&` `\|` `^` `<<` `>>` | ✅ | ❌ | ❌ | alap: lexed but parser has no bitwise level |
| Unary `-` `!` | ✅ | ⚠️ | ✅ | alap: unary `+`/`~` not parsed |
| Range `..` `..=` | ✅ | ❌ | ❌ | alap: `..` lexed, never parsed |
| Coalesce `??` | ✅ | ❌ | ⚠️ | alap: compiles to truthy OR, so coalesces `0`/`""` |
| Safe access `?.` | ✅ | ❌ | ⚠️ | alap: parsed, codegen ignores the optional flag |
| String concatenation `+` | ✅ | ⚠️ | ✅ | |
| String interpolation `\(expr)` | ✅ | ❌ | ❌ | alap: template strings treated as plain text |
| Assignment `=` | ✅ | ⚠️ | ✅ | |
| Compound `+=` `-=` `*=` `/=` `%=` | ✅ | ❌ | ✅ | alap: bitwise compound ops are broken |

---

## 4. Control Flow

| Feature | Go | Rust | alap | Notes |
|---------|-----|------|------|-------|
| `if` / `else if` / `else` | ✅ | ⚠️ | ✅ | |
| `while` loop | ✅ | ⚠️ | ✅ | |
| `while let` | ✅ | ❌ | ❌ | |
| `for` loop (range) | ✅ | ❌ | ❌ | alap: no range syntax |
| `for` loop (array) | ✅ | ❌ | ✅ | |
| `for` loop (string) | ✅ | ❌ | ✅ | |
| C-style `for (;;)` | ✅ | ❌ | ✅ | |
| `break` | ✅ | ⚠️ | ✅ | |
| `continue` | ✅ | ⚠️ | ✅ | |
| `match` expression | ✅ | ❌ | ⚠️ | alap: statement, literal-equality only, no exhaustiveness |
| Match guards | ✅ | ❌ | ❌ | |
| Match or-patterns `\|` | ✅ | ❌ | ❌ | |
| Match exhaustiveness | ✅ | ❌ | ❌ | |
| Ternary `?:` | ✅ | ❌ | ✅ | |

---

## 5. Functions

| Feature | Go | Rust | alap | Notes |
|---------|-----|------|------|-------|
| `fn` declaration | ✅ | ⚠️ | ✅ | alap keyword is `function` |
| Anonymous functions | ✅ | ❌ | ✅ | |
| Closures | ✅ | ❌ | ❌ | alap: capture set never populated |
| Recursion | ✅ | ⚠️ | ✅ | |
| Higher-order functions | ✅ | ❌ | ✅ | |
| Default parameters | ✅ | ❌ | ⚠️ | alap: parsed, ignored by codegen |
| Variadic parameters | ✅ | ❌ | ❌ | alap: `Rest` parsed, ignored |
| `return` statement | ✅ | ⚠️ | ✅ | |
| `pure fn` modifier | ✅ | ❌ | ❌ | |
| `unsafe fn` modifier | ✅ | ❌ | ❌ | |
| `requires` clause | ✅ | ❌ | ❌ | |
| Generic functions `fn f<T>` | ✅ | ❌ | ⚠️ | alap: syntactic only, ignored by checker/codegen |
| Function types `fn(A) -> B` | ✅ | ❌ | ⚠️ | alap: parsed + checked, no runtime |

---

## 6. Structs

| Feature | Go | Rust | alap | Notes |
|---------|-----|------|------|-------|
| `struct` declaration | ✅ | ⚠️ | ⚠️ | alap: parsed; codegen emits nothing |
| Struct literal `T { field: val }` | ✅ | ⚠️ | ✅ | alap: object literal |
| Field access `.field` | ✅ | ⚠️ | ✅ | |
| Field mutation | ✅ | ❌ | ✅ | |
| Nested structs | ✅ | ⚠️ | ✅ | |
| Struct methods | ✅ | ❌ | ❌ | alap: members ignored by codegen |
| Static methods `T::new()` | ✅ | ❌ | ❌ | |

---

## 7. Enums

| Feature | Go | Rust | alap | Notes |
|---------|-----|------|------|-------|
| `enum` declaration | ✅ | ❌ | ⚠️ | alap: parsed; codegen emits nothing |
| Unit variants | ✅ | ❌ | ❌ | alap: members not registered as globals |
| Payload variants `A(T)` | ✅ | ❌ | ❌ | |
| Enum pattern matching | ✅ | ❌ | ❌ | |

---

## 8. Traits

| Feature | Go | Rust | alap | Notes |
|---------|-----|------|------|-------|
| `trait` declaration | ✅ | ❌ | ❌ | alap: no `trait` keyword (has `interface`) |
| `impl Trait for Type` | ✅ | ❌ | ❌ | alap: no `impl` keyword |
| Method calls on traits | ✅ | ❌ | ❌ | |
| Generic traits | ✅ | ❌ | ❌ | |

---

## 9. Modules

| Feature | Go | Rust | alap | Notes |
|---------|-----|------|------|-------|
| `import` statement | ✅ | ⚠️ | ⚠️ | alap: parsed; no linking emitted |
| `export` visibility | ✅ | ❌ | ⚠️ | alap: parsed only |
| `private` visibility | ✅ | ❌ | ⚠️ | alap: token exists, not enforced |
| Module re-export | ✅ | ❌ | ⚠️ | alap: parsed only |

---

## 10. Error Handling

| Feature | Go | Rust | alap | Notes |
|---------|-----|------|------|-------|
| `Result<T,E>` type | ✅ | ❌ | ❌ | |
| `Ok()` / `Err()` constructors | ✅ | ❌ | ❌ | |
| `unwrap()` | ✅ | ❌ | ❌ | |
| `unwrapOr()` | ✅ | ❌ | ❌ | |
| `isOk()` / `isErr()` | ✅ | ❌ | ❌ | |
| `?T` optional type | ✅ | ❌ | ⚠️ | alap: checker only |
| `Some()` / `None()` | ✅ | ❌ | ❌ | |
| `isSome()` / `isNone()` | ✅ | ❌ | ❌ | |
| Runtime error categories | ✅ | ❌ | ❌ | |
| `try` / `catch` | ❌ | ❌ | ⚠️ | alap: opcodes emitted, VM treats them as no-ops; not in nilLang yet |

---

## 11. Concurrency

| Feature | Go | Rust | alap | Notes |
|---------|-----|------|------|-------|
| `async` / `await` | ✅ | ❌ | ⚠️ | alap: `OP_AWAIT` is a synchronous no-op |
| `task` spawning | ✅ | ❌ | ⚠️ | alap: emits a Go goroutine |
| Channels | ✅ | ❌ | ⚠️ | alap: send/recv exist but no channel constructor |
| Select statement | ✅ | ❌ | ❌ | |
| Structured concurrency | ✅ | ❌ | ❌ | |

---

## 12. Capabilities

| Feature | Go | Rust | alap | Notes |
|---------|-----|------|------|-------|
| `requires` clause | ✅ | ❌ | ❌ | |
| Capability enforcement | ✅ | ❌ | ❌ | alap: Go-side permission map only, not the language |
| `pure` modifier | ✅ | ❌ | ❌ | |
| `unsafe` modifier | ✅ | ❌ | ❌ | |

---

## 13. Standard Library

| Feature | Go | Rust | alap | Notes |
|---------|-----|------|------|-------|
| `println` / `print` | ✅ | ❌ | ⚠️ | alap: `print` only |
| `len` | ✅ | ❌ | ⚠️ | alap: `.length` property instead |
| `first` / `last` | ✅ | ❌ | ❌ | |
| `push` / `rest` | ✅ | ❌ | ⚠️ | alap: `push` only |
| `str` | ✅ | ❌ | ❌ | alap: `.toString()` |
| `isNull` | ✅ | ❌ | ❌ | |
| Math module | ✅ | ❌ | ⚠️ | alap: partial (`abs/sqrt/floor/ceil/round/random/min/max`) |
| String module | ✅ | ❌ | ✅ | |
| Array module | ✅ | ❌ | ✅ | |
| Map module | ✅ | ❌ | ⚠️ | alap: object-backed |
| JSON module | ✅ | ❌ | ⚠️ | alap: parse real, stringify stubbed |
| HTTP module | ✅ | ❌ | ⚠️ | alap: in VM, missing from type checker |
| File system | ✅ | ❌ | ✅ | |
| Time module | ✅ | ❌ | ✅ | |
| Crypto module | ✅ | ❌ | ✅ | |

---

## 14. UI / Alap Framework

| Feature | Go | Rust | alap | Notes |
|---------|-----|------|------|-------|
| `component` declaration | ✅ | ⚠️ | ✅ | |
| `state` declaration | ✅ | ⚠️ | ⚠️ | alap: fields parsed, no reactive wiring |
| `render` method | ✅ | ⚠️ | ❌ | alap: uses `build()` |
| `build` method | ✅ | ❌ | ✅ | |
| `on` event handler | ✅ | ❌ | ⚠️ | alap: works, but handlers cannot capture state |
| `emit` event | ✅ | ❌ | ❌ | |
| `app` declaration | ✅ | ⚠️ | ❌ | alap: root is a `@Entry` component |
| Declarative UI tree | ✅ | ⚠️ | ✅ | |
| Reactive signals | ✅ | ❌ | ⚠️ | alap: Go-only library, not wired into the VM |

---

## 15. Summary Matrix

| Category | Go | Rust | alap |
|----------|-----|------|------|
| **Types** | ✅ Full | ⚠️ Partial | ⚠️ Partial |
| **Operators** | ✅ Full | ⚠️ Partial | ⚠️ Partial |
| **Control Flow** | ✅ Full | ⚠️ Partial | ⚠️ Partial |
| **Functions** | ✅ Full | ⚠️ Partial | ⚠️ Partial |
| **Structs** | ✅ Full | ⚠️ Partial | ⚠️ Partial |
| **Enums** | ✅ Full | ❌ None | ❌ None |
| **Traits** | ✅ Full | ❌ None | ❌ None |
| **Modules** | ✅ Full | ⚠️ Partial | ⚠️ Partial |
| **Error Handling** | ✅ Full | ❌ None | ❌ None |
| **Concurrency** | ✅ Full | ❌ None | ⚠️ Stub |
| **Capabilities** | ✅ Full | ❌ None | ❌ None |
| **Standard Library** | ✅ Full | ❌ None | ⚠️ Partial |
| **UI Framework** | ✅ Full | ⚠️ Partial | ⚠️ Partial |

---

## 16. Rust Runtime Limitations

The Rust runtime (`onuronOS/runtime/nillang`) is a **prototype** that supports a
subset of NilLang syntax focused on UI app definitions:

- ✅ Parser for basic syntax
- ✅ JSON bytecode serialization (`NILB`/v1)
- ✅ Basic VM for app/state management
- ✅ Struct and app definitions
- ⚠️ Event dispatch — hardcoded patterns (`btn_launch`, `set_*`)
- ⚠️ Scene rendering — XML strings only, no render pipeline
- ❌ No expression evaluation
- ❌ No control flow execution
- ❌ No function calls
- ❌ No error handling
- ❌ No standard library

**This is NOT a drop-in replacement for nilLang.**

---

## 17. Alap-Framework Relationship

The alap-framework is an **early-alpha prototype** of the Alap UI framework. It
has a separate lexer/parser/NABC VM and a non-fatal type checker (type errors are
collected but do not block compilation).

- It is **NOT** the canonical implementation
- nilLang is the canonical implementation
- alap-framework's value: platform adapters (Android/iOS/Linux generators), web server, ORM
- Future direction: alap-framework should either import nilLang packages or be merged

See `alap-framework/RELATIONSHIP.md` for details.

---

## 18. Version History

| Version | Date | Changes |
|---------|------|---------|
| 1.0.0-draft | 2026-10-12 | Initial canonical specification |
| 1.0.1-draft | 2026-10-12 | Corrected the alap-framework column to source-verified support; legacy all-✅ column conflated nilLang's internal `pkg/alap` layer with the separate alap-framework repo |
