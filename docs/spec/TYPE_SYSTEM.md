# NilLang Type System Specification
**Version:** 1.0.0-draft  
**Status:** Draft Normative Specification
**Conformance Level:** Nilang 0.1 Core Freeze (implemented subset only)

---

## 0. Implementation Boundary

Nilang 0.1 implements inference and checking for core literals, annotated and
inferred bindings, constants, assignment, functions, arrays, hashes, entities,
and components. Generic syntax, algebraic data types, exhaustive matching,
function effect types, and domain scalar representations remain planned.

`Any` is still inferred at untyped function parameters, component boundaries,
and built-ins whose generic contracts have not been implemented. Explicit-only
`Any` is the frozen target rule, not yet a conformance claim. Every new fallback
to implicit `Any` is prohibited during the core freeze; existing origins are
tracked by the P0 stabilization backlog.

## 1. Design Principles & Soundness Guarantee

NilLang enforces a sound, static, and structural type system. The compiler type checker acts as the mandatory verification gate for all execution pipelines (Tree-Walking Evaluator, Stack VM Bytecode, WASM, Native FFI).

### Cardinal Rules:
1. **Verification-First Gate**: Compilation immediately aborts if any expression fails static verification. Downstream representations (HIR, MIR, Bytecode, WASM) are never constructed for ill-typed programs.
2. **Explicit Intent for `Any` (target rule)**: The `Any` type is an explicit top type indicating deliberate untyped/dynamic intent by the programmer (`let x: Any = ...`). Existing implicit fallback sites are compatibility debt and must be removed before this rule becomes an implemented guarantee.
3. **No Unsound Numeric Coercion**: Implicit promotion between integer sizes or between integers and floating-point values is prohibited. Conversions must be explicit via casting functions (`int(f)`, `float(i)`).
4. **Definite Assignment**: Variables must be initialized upon declaration (`let x: Int = 10;`). Uninitialized variable declarations are rejected with `E0103`.

---

## 2. Type Classification & Taxonomy

```text
                                Type (Interface)
                                       │
        ┌──────────────┬───────────────┼───────────────┬──────────────┐
        ▼              ▼               ▼               ▼              ▼
  PrimitiveType    CompoundType    AlgebraicType    FunctionType    DomainType
    • Int (i64)     • Struct        • Enum           • fn(T) -> U    • Entity
    • Float (f64)   • Array<T>      • Union (A | B)  • Effects: []   • Money
    • String        • Hash<K, V>    • Optional (?T)                  • UUID
    • Bool                          • Result<T, E>                   • Date
    • Byte (u8)
    • Null / Void
    • Any (explicit)
```

---

## 3. Primitives & Built-in Scalar Types

| Type | Bit Width | Literal Examples | Default Value | Notes |
|---|---|---|---|---|
| `Int` | 64-bit signed (two's complement) | `0`, `42`, `-10_000` | `0` | Canonical integer type. Aliases: `i64`, `int`. |
| `Float` | 64-bit IEEE 754 float | `0.0`, `3.14159`, `-0.5` | `0.0` | Canonical floating point. Aliases: `f64`, `float`. |
| `String` | UTF-8 encoded byte slice | `"hello"`, `"বাংলা"` | `""` | Immutable string. Supports interpolation `\(expr)`. |
| `Bool` | Boolean truth value | `true`, `false` | `false` | Non-numeric boolean. |
| `Byte` | 8-bit unsigned integer | `byte(255)`, `0xFF` | `0` | Unsigned byte for binary data. Alias: `u8`. |
| `Null` | Unit null reference | `null` | `null` | Only assignable to `Null`, `?T`, or `Any`. |
| `Void` | Unit type (no value) | - | - | Return type for side-effect-only procedures. |
| `Any` | Dynamic escape hatch | any literal | `null` | Explicit dynamic opt-in. Disables static check for binding. |

### Extended Domain Primitives
- `Money`: Exact fixed-point currency representation preventing IEEE-754 precision loss.
- `UUID`: 128-bit Universally Unique Identifier.
- `Email`, `Date`, `Quantity`: Validated business domain scalar types.

---

## 4. Compound & Generic Types (Partially Implemented)

### 4.1 Arrays (`Array<T>` / `List<T>`)
An ordered, homogeneous sequence of elements:
```nil
let numbers: Array<Int> = [1, 2, 3];
```
- Inferred as `Array<T>` where `T` is the common supertype of all elements.
- Empty array literal `[]` without annotation requires explicit type parameter (`let empty: Array<String> = [];`).

### 4.2 Hashes (`Hash<K, V>`)
An associative map of keys to values:
```nil
let scores: Hash<String, Int> = {"Alice": 95, "Bob": 88};
```
- Keys must implement equality comparison.

### 4.3 Structs
Named records with typed fields:
```nil
struct User {
    id: Int,
    name: String,
    active: Bool
}
```
- Structural field lookup: `user.name`.
- Structural equality requires identical field names and recursively equal types.

### 4.4 Algebraic Data Types (Enums & Sum Types)
```nil
enum Status {
    Pending,
    Active(Int),
    Failed(String)
}
```
- Pattern matching via `match` exhaustively validates variants.

### 4.5 Optional Types (`?T`)
Syntactic sugar for `T | Null`:
```nil
let username: ?String = null;
```
- Safe navigation operator `?.` short-circuits to `null` if the receiver is `null`.
- Null coalescing `??` unwraps an optional with a fallback value.

```nil
username ?? "guest"          // "guest" when null, otherwise the name
user?.address.city           // null when user is null — the whole chain
user?.name ?? "unknown"      // the two compose
```

**Implementation status:** implemented in both engines. The `?T` annotation, the
ternary `cond ? a : b`, `?.`, and `??` all parse, check, and run. Semantics are
specified in SEMANTICS §4D; the two rules that are easy to get wrong — the
receiver is evaluated once, and `?.` guards the whole chain rather than the single
step it appears on — are stated there and covered by tests.

The lexer produces `??` and `?.` as single tokens rather than a `?` followed by
another character. `a ?? b ? c : d` has both readings otherwise, and settling it
in the lexer is cheaper than carrying the ambiguity into every consumer.

**Precedence:** `??` sits between `||` and `&&`; `?.` sits with member access and
binds tighter than every binary operator. `a ?? b ?? c` groups left to right, so
a chain does not need parentheses, and `a ?? b ? c : d` reads as `a ?? (b ? c : d)`.

### 4.6 Result Types (`Result<T, E>`)
Container representing either success (`Ok(T)`) or failure (`Err(E)`).

---

## 5. Type Assignability & Subtyping Rules

A type `S` is assignable to `T` (written `S <: T`) if and only if:
1. **Identity**: `S` is identical to `T`.
2. **Top Type**: `T` is `Any`.
3. **Optional Wrapping**: `T` is `?U` and `S` is assignable to `U`, or `S` is `Null`.
4. **Union Inclusion**: `T` is `Union(T_1 | ... | T_n)` and `S` is assignable to at least one `T_i`.
5. **Generic Covariance**: `Array<S> <: Array<T>` if and only if `S <: T` (immutable read contexts).
6. **Function Subtyping (Contravariant Parameters, Covariant Return)**:
   `fn(P_s) -> R_s <: fn(P_t) -> R_t` if `P_t <: P_s` and `R_s <: R_t`.

### 5.4 The two directions are opposite, and both are enforced

A parameter is **contravariant**: `fn(Any) -> R` may stand where `fn(String) -> R`
is expected. The reverse is refused.

```nil
fn takes(f: fn(String) -> Int) -> Int { ... }
fn wide(x: Any) -> Int { ... }
takes(wide)      // accepted — a function that accepts anything can be used
                 // where one that accepts a String is wanted
```

A return type is **covariant**: `fn(A) -> Any` may stand where `fn(A) -> Int` is
expected, because anything reading the result is prepared for the wider type. The
reverse is refused.

```nil
fn takes(f: fn(Any) -> Int) -> Int { ... }
fn narrow(x: String) -> String { ... }
takes(narrow)    // refused — this function cannot be called with an Int
```

Arity is part of the rule, not a separate check: a function of a different arity
is a different function, whatever its types say.

**Why this is tested in both directions.** An identity check instead of variance
would accept and reject almost the same programs, and the difference only appears
where a type sits between `Any` and something concrete. A suite that asserted
only the accepting cases would pass just as well with `Equals`, and only the
refusing cases would notice. `TestFunctionVarianceIsNotPlainEquality` is the pair
that tells them apart.

### 5.1 Rules 3 and 4 are checked in both directions

Rules 3 and 4 say a *source* belongs to a *composite target*. That is the direction
every concrete type has to implement, and it is the direction that was missing:
`PrimitiveType` handled it and no other type did.

The consequences were that `?Address` was a type nothing could be put into, and
`P | Q` was a type no `P` could satisfy — so `let x: ?String = "s";` and
`let x: P | Q = P { .. };` were both rejected while `let x: Int | String = 5;`
was accepted. A union annotation was written down and then ignored.

The rule now lives in one function every concrete type calls. Leaving it to each
type is how a rule ends up half-implemented in the first place.

### 5.2 A union's members are resolved before they are compared

`types.Parse` is deliberately permissive: an unrecognised name becomes an empty
struct rather than an error, so a type from a module the checker has not read still
parses. The cost is that `Int | Nope` resolved to a union containing a placeholder,
and a placeholder compares equal to nothing — so the annotation accepted every
value the one *real* member accepted and rejected the rest for the wrong reason.

Every member is therefore checked on its own, and the whole annotation is
reported as unknown (`E0502`) when any member is. The same applies to an optional
base: `?Address` resolves to the declared struct, not to a name with no fields.

### 5.3 Numeric widths are real types

`i8`, `i16`, `i32`, `i64`, `u8`, `u16`, `u32`, `u64`, `f32`, and `f64` are each a
distinct type, not a spelling of `Int`. Both spellings are accepted: `u8` in
source and `Uint8` after resolution name the same width.

A width that changes nothing is worse than no width: a developer who writes it
reasonably believes a range is being checked. Aliasing them to `Int` is what made
`let x: i8 = 300;` compile.

**The one concession: an integer literal.**

```nil
let x: u8 = 5;      // accepted — the literal had no width of its own
let x: u8 = 300;    // E0110 — it does not fit
let y: Int = x;     // E0101 — a width is not Int, and nothing converts it
```

An integer literal takes the width of its context when it fits, and none whose
range it does not. That is context sensitivity, not coercion: the literal has no
width of its own until the declaration gives it one. A *variable* of type `Int`
cannot go into a `u8` — there is no implicit step anywhere, which is what "no
implicit numeric coercion" means in practice.

**Arithmetic keeps a width.** `u8 + u8` is `u8`. A bare literal beside a width
adopts that width, so `x + 1` on a `u8` stays `u8`. Anything else widens to `Int`,
which is wider than every width in the language and therefore cannot be too
narrow.

**`+` over untyped values is `Any`, not `Int`.** Two values the checker cannot type
may well be strings at run time — `payload["name"] + payload["tag"]` in the UI
examples is exactly that — and answering `Int` would break a program that has
always worked.

#### What is not enforced

The run-time representation is still a 64-bit integer. `let a: u8 = 200;
let b: u8 = a + a;` is 400 and stays 400. Catching that needs a narrowing
instruction in both engines, and a check that fires for some values and not others
is worse than one documented as compile-time.

### 5.6 Narrowing

`Ok`, `Err`, `Some`, `unwrap`, and `unwrapOr` each *compute* the type they produce
and used not to say it — every one of them was registered as `Any` in and `Any` out.
That is not a small imprecision: `unwrap(r)` had no type, so every value it
produced was unchecked from there on, and a program could read an error payload as
a success value without the checker noticing.

```nil
unwrap(Ok(5))                  // Int, not Any
let r: ?Int = Some(5);         // ?Int, not ?Any
let r: Result<Int, String> = Ok(5);
unwrap(r)                      // Int
```

The error side of a `Result` stays `Any`, because a checker that has not read
every `Err` in a program cannot know the narrowest error type, and guessing one
would reject correct programs.

`unwrap` and `unwrapOr` applied to something that is neither is `E0112`, rather
than quietly producing `Any` for the rest of the program. The one exception is
`Any` itself: an unannotated function — including every standard-library helper
— returns `Any`, so refusing `unwrap` on it made `unwrap(money.add(a, b))` a
compile error and the language's own documented patterns unusable under
`nil run`. `Any` may hold a `Result` at run time, and the run-time unwrap
already fails loudly on anything else, so the static gate stands aside for
`Any` and lets the dynamic one decide. Only a type that is *definitely* not a
`Result` or an Optional (an `Int` literal, a `String`) is still `E0112`.

A `Result<T, E>` and an Optional each exist as a dedicated type *and* as a generic
spelled `Result<...>` / `Optional<...>`. An annotation resolves to one and a value
produces the other, so the two are bridged positionally; without that, a `let r:
Result<Int, String> = Ok(5)` compares two things that look identical in the
message and are different in the tree.

---

## 6. Built-in Function Signatures & Generic Contracts

### 6.1 User-defined generics

A function may declare type parameters between its name and its parameter list:

```nil
fn id<T>(x: T) -> T { return x; }
fn size<T>(items: Array<T>) -> Int { return len(items); }
fn pair<A, B>(a: A, b: B) -> String { return "\(a)-\(b)"; }
```

Inside the declaration, every mention of a type parameter is a real type — a
`TypeVar` — so the body is checked rather than skipped. A type parameter resolves
*before* the declared-type lookups, because a name the checker has deliberately
not fixed is the opposite of one it has failed to recognise: resolving `T` to the
placeholder struct `Parse` produces for unknown names would make it equal to
nothing and report `E0502` for every generic function.

**The type parameters are in force before the parameters are resolved**, not just
for the body. A parameter's own annotation is where `T` is written most often, and
resolving `x: T` before pushing `T` would report the parameter's type as unknown.
They are *restored* rather than cleared when a body ends, because generics nest
through ordinary calls.

**`Array` and `List` are one type.** Every document says `Array<T>` and the
checker infers `List<T>` for an array literal. Leaving them apart would make
`fn size<T>(items: Array<T>)` reject `size([1, 2])` — a generic function that
cannot be called on the only kind of array the language produces.

#### Inference at a call site

A call site matches each parameter type against the argument and binds the
variables it can, then checks the argument against the **instantiated** signature:

- `Array<T>` against `List<Int>` gives `T = Int`.
- `fn id<T>(x: T) -> T` called as `id(5)` gives `T = Int` and returns `Int`.
- `size(5)` binds nothing and is rejected (`E0204`) — an integer is not an array,
  and no substitution would make it one.

Checking an argument against the *unbound* signature is the mistake this ordering
exists to prevent: `Int` against a bare `T` would either match or not for a reason
that says nothing about the call.

#### The boundary

Inference is **structural, not unification**. It looks through a generic type, an
optional, and a union. It does not descend into a function type's parameter list,
and it does not reason about the body.

The consequence is that a variable appearing in no parameter cannot be pinned
down, so it becomes `Any`:

```nil
fn empty<T>() -> Array<T> { return []; }
let xs: Array<Int> = empty();   // accepted — the limitation, asserted in
                                // TestGenericBoundaryIsWhereItIs
```

A suite that only checked what works would let that boundary move unnoticed, so
the test above pins it. Narrowing it needs the return type to be inferred from the
body, which is a much larger piece of work.

#### It is erased

There is no monomorphisation. `id` is compiled once and a call site passes
ordinary values. Both engines therefore reach it through the plain call path and
cannot disagree about it, and there is no run-time cost — the same argument as
ADR-0007 and ADR-0013, one level up.

### 6.2 Built-in signatures

Built-in operations carry strict generic parametric contracts:

```nil
// Collections
fn len<T>(container: Array<T> | Hash<Any, Any> | String) -> Int;
fn push<T>(items: Array<T>, item: T) -> Array<T>;
fn first<T>(items: Array<T>) -> ?T;
fn last<T>(items: Array<T>) -> ?T;
fn rest<T>(items: Array<T>) -> Array<T>;

// Type Reflection
fn type(value: Any) -> String;

// Mathematical / Numerical
fn abs(x: Int) -> Int;
fn abs(x: Float) -> Float;
```

---

## 7. Diagnostic Codes for Type Checking

| Code | Name | Description |
|---|---|---|
| `E0101` | `TypeMismatch` | Attempted to assign, initialize, or return a value incompatible with target type. |
| `E0102` | `UndefinedIdentifier` | Variable or symbol referenced before declaration. |
| `E0103` | `MissingInitializer` | Variable declared without mandatory initial value. |
| `E0104` | `ConstMutation` | Attempted reassignment to an immutable `const` binding. |
| `E0105` | `ArityMismatch` | Function called with incorrect number of arguments. |
| `E0106` | `NonCallableType` | Attempted to call an expression whose type is not a function. |
| `E0107` | `InvalidMemberAccess` | Accessing non-existent field or member on struct or object. |
| `E0108` | `IncompatibleBinaryOp` | Binary operator applied to incompatible operand types (e.g. `Int + String`). |
| `E0109` | `UncheckedNullAccess` | Member access on nullable type without `?.` or explicit check. |