# NilLang Type System Specification
**Version:** 1.0.0-draft  
**Status:** Authoritative Normative Specification  
**Conformance Level:** Nilang 0.1 Core Freeze

---

## 1. Design Principles & Soundness Guarantee

NilLang enforces a sound, static, and structural type system. The compiler type checker acts as the mandatory verification gate for all execution pipelines (Tree-Walking Evaluator, Stack VM Bytecode, WASM, Native FFI).

### Cardinal Rules:
1. **Verification-First Gate**: Compilation immediately aborts if any expression fails static verification. Downstream representations (HIR, MIR, Bytecode, WASM) are never constructed for ill-typed programs.
2. **Explicit Intent for `Any`**: The `Any` type is an explicit top type indicating deliberate untyped/dynamic intent by the programmer (`let x: Any = ...`). Silent implicit fallback to `Any` during type inference is strictly prohibited. Uninferable expressions result in diagnostic `E0101`.
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

## 4. Compound & Generic Types

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

---

## 6. Built-in Function Signatures & Generic Contracts

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