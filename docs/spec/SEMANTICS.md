# NilLang Execution Semantics Specification
**Version:** 1.0.0-draft  
**Status:** Draft Normative Specification
**Conformance Level:** Nilang 0.1 Core Freeze (implemented subset only)

---

## 0. Normative Scope

The evaluator/VM conformance suite is the executable authority for Nilang 0.1
runtime semantics. A rule in this document is frozen only when both engines
implement it and a conformance case covers it. Optimized VM and WASM parity are
future conformance levels.

## 1. Evaluation Order & Determinism

NilLang evaluation is strictly deterministic and left-to-right:
1. **Expressions**: Sub-expressions are evaluated in left-to-right lexical order.
   In `f(a(), b())`, `a()` is completely evaluated before `b()`, and both are evaluated before `f` is invoked.
2. **Binary Operations**: For binary expression `E_left op E_right`, `E_left` is evaluated first.
3. **Short-Circuiting**:
   - `A && B`: If `A` evaluates to `false`, `B` is not evaluated.
   - `A || B`: If `A` evaluates to `true`, `B` is not evaluated.

---

## 2. Binding, Mutation & Assignment Semantics

### 2.1 Lexical Bindings
Bindings are introduced into the current lexical scope exclusively via explicit declarations:
- `let x = expr;` introduces a mutable lexical binding initialized to the result of `expr`.
- `let x: Type = expr;` introduces a typed mutable binding.
- `const c = expr;` introduces an immutable lexical binding.
- `fn name(params) { ... }` introduces a function binding.
- Function formal parameters introduce bindings in the function's local scope upon call.

### 2.2 Strict Assignment Semantics (Option A)
An assignment statement updates an existing binding:
```nil
x = 20;
x += 5;
x -= 2;
```
**Rules:**
1. **No Auto-Declaration**: The assignment target must resolve to a valid lexical binding in the current or an enclosing ancestor scope.
2. **Compile-Time Rejection**: If the identifier does not resolve to an active declaration, compilation aborts with diagnostic:
   ```text
   error[E0102]: undefined identifier 'x' in assignment
   ```
3. **Const Immutability**: If the identifier resolves to a `const` declaration, compilation aborts with:
   ```text
   error[E0104]: cannot assign to constant 'c'
   ```
4. **Compound Assignment**:
   `x += expr` is semantically equivalent to `x = x + expr`, with the target evaluated exactly once.

---

## 3. Lexical Scoping & Closure Semantics

### 3.1 Scope Hierarchy
Lexical scopes form a parent-child tree:
```text
Global Scope
  └── Module Scope
        └── Function Scope
              └── Block Scope (if, while, for)
                    └── Inner Block Scope
```

**Lookup Rules:**
- Symbol lookup begins in the innermost active scope and traverses outward towards the global scope.
- Outer bindings are shadowed when an inner scope declares a binding with the identical name:
  ```nil
  let x = 10;
  {
      let x = 20; // Shadows outer x within this block
      x + 5;      // 25
  }
  x;              // 10 (outer binding unchanged)
  ```

### 3.2 Closure Capture
Functions capture identifiers referenced from enclosing scopes (free variables).
Read-only capture and nearest-scope resolution are implemented by both engines.
Mutable by-reference capture is implemented by the evaluator but remains
planned for the Stack VM; it is therefore not a frozen Nilang 0.1 guarantee.
The target semantics are:
```nil
fn makeCounter() {
    let count = 0;
    return fn() {
        count = count + 1;
        return count;
    };
}

let counter = makeCounter();
counter(); // 1
counter(); // 2
```
- Captured bindings survive the lifetime of the stack frame in which they were originally declared.
- Mutations made to a captured binding from within a closure are visible to all closures sharing that binding context.

---

## 4. Control Flow Semantics

### 4.1 Conditional Execution (`if` / `else`)
- The condition expression is evaluated.
- Truthiness: `true` is truthy; `false` and `null` are falsy. Integers other than 0 and non-empty strings are considered truthy in dynamic contexts, but static type checking requires condition to evaluate to `Bool`.
- Only the selected branch executes; the alternative is never evaluated.

### 4.2 Loops (`while` and `for`)
- `while (cond) { body }`: Evaluates `cond`. If truthy, executes `body` and repeats.
- `for (item in collection) { body }`:
  - Iterates over arrays in 0-indexed sequential order.
  - Iterates over hash entries deterministically.
- `break` exits the innermost enclosing loop immediately.
- `continue` jumps to the next iteration of the innermost enclosing loop.

---

## 5. Arithmetic & Runtime Exception Semantics

### 5.1 Division & Modulo
- **Integer Division by Zero**:
  `x / 0` and `x % 0` where `x: Int` raises a runtime exception:
  ```text
  RuntimeError: division by zero
  ```
  Both Evaluator and VM halt execution and unwind to the nearest error handler or terminate with exit code 1.
- **Floating-Point Division by Zero**:
  `f / 0.0` where `f: Float` conforms to IEEE-754:
  - `> 0.0 / 0.0` -> `+Infinity`
  - `< 0.0 / 0.0` -> `-Infinity`
  - `0.0 / 0.0` -> `NaN`

### 5.2 Numeric Overflow
- `Int` operations wrap on two's complement 64-bit boundaries in performance mode, or raise `IntegerOverflow` in debug mode.

---

## 6. Conformance Obligations

The Evaluator and Stack VM must produce equivalent typed values or runtime
error categories for:
1. All arithmetic operations and division exceptions.
2. Short-circuit side effects.
3. Closure variable capture and mutation.
4. Scope shadowing boundaries.
5. Exit codes and standard output.

Optimized VM and WASM become subject to this obligation only when they are
added to the executable conformance harness.