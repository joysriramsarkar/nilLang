# NilLang Error Model Specification
**Version:** 1.0.0-draft  
**Status:** Authoritative Normative Specification  
**Conformance Level:** Nilang 0.1 Core Freeze

---

## 1. Cardinal Invariants

1. **Compiler Never-Panic Guarantee**:
   The compiler must **never panic** on any user-provided source code, regardless of syntax errors, cyclic structures, deep recursion, or malformed tokens. Any deviation constitutes a compiler bug. All compilation issues are communicated via structured diagnostics.
2. **Deterministic Diagnosability**:
   Every error carries a stable error code (`E0xxx`), exact source location coordinates (line, column, span), offending source context, and actionable correction guidance.
3. **Structured Runtime Errors**:
   Runtime exceptions are typed objects (`RuntimeError`) rather than arbitrary host process crashes or unbounded panics.

---

## 2. Compile-Time Diagnostic Architecture

### 2.1 Diagnostic Data Model
```text
Diagnostic
├── Code: String (e.g. "E0101")
├── Severity: { ERROR, WARNING, INFO, HINT }
├── Message: String
├── Span: { Filename, StartLine, StartCol, EndLine, EndCol }
├── ContextLine: String (source snippet)
├── Suggestion: String (actionable "help: ...")
└── AIExplanation: String (optional advisory rationale)
```

### 2.2 Standard Error Code Registry

#### Lexical & Syntactic (`E0001` - `E0099`)
- `E0001`: Unexpected character or invalid token.
- `E0002`: Unterminated string literal.
- `E0003`: Unexpected end of file.
- `E0004`: Syntax error (expected token X, got Y).

#### Typing & Binding (`E0101` - `E0199`)
- `E0101`: Type mismatch.
- `E0102`: Undefined identifier in read or assignment.
- `E0103`: Missing variable initializer.
- `E0104`: Attempted assignment to constant.
- `E0105`: Function call arity mismatch.
- `E0106`: Invocation of non-callable expression.
- `E0107`: Unknown struct or object member.
- `E0108`: Incompatible binary operator types.
- `E0111`: A member chain names something the receiver's declared type does not
  have. It is checked only where the receiver's type is known: a hash has
  whatever keys were put in it, and a receiver whose type could not be inferred is
  not evidence of a mistake.

- `E0110`: An integer literal does not fit the width it was written against.
- `E0112`: `unwrap` or `unwrapOr` applied to a value that is neither a `Result`
  nor an `?T`. `Any` is exempt: it may hold either at run time, and the
  run-time unwrap fails loudly on anything else.
- `E0113`: A numeric width used where a different width or `Int` is expected,
  with no implicit conversion between them.

#### Capabilities & Security (`E0201` - `E0299`)
- `E0201`: Missing required capability in project manifest.
- `E0202`: Pure function violates purity constraint (attempts I/O or mutation).
- `E0203`: Unsafe operation invoked without `unsafe` block.

#### Modules & Packages (`E0301` - `E0399`)
- `E0301`: Circular module dependency detected.
- `E0302`: Module not found at specified path.
- `E0303`: Symbol not exported by module.

#### Entities & Components (`E0401` - `E0499`)
- `E0401`: Entity declaration without a valid identifier name.
- `E0402`: Duplicate entity declaration in the same scope.
- `E0403`: Duplicate field name inside one entity.
- `E0404`: Entity field type could not be resolved.
- `E0405`: More than one primary key declared on an entity.
- `E0406`: Component declaration without a name.

#### Algebraic Data Types (`E0501` - `E0599`)
- `E0501`: Duplicate declaration — a type name, an enum variant, a trait, or a
  trait method redeclared.
- `E0502`: Unknown type name in a parameter, return, field, or alias annotation.
- `E0503`: Unknown field on a struct.
- `E0504`: Duplicate field in a struct literal.
- `E0505`: Unknown enum variant in a `match` or a call.
- `E0506`: Missing field in a struct literal.
- `E0507`: Non-exhaustive `match` over an enum, `bool`, `Result`, or `Optional`.
- `E0508`: Duplicate `match` arm.
- `E0509`: Wrong number of arguments for an enum variant.
- `E0510`: Wrong arity for a `match` pattern.
- `E0511`: Variable not bound in the arm that would read it.

- `E0204`: A call cannot satisfy a generic signature's structure — an integer
  where `Array<T>` is expected, most often.

It is separate from `E0101` because the two are reached by different reasoning:
`E0101` compares two concrete types, while this compares an argument against a
signature the checker has just instantiated. Sharing a code would hide which of
the two a developer is looking at.

#### Traits & Implementations (`E0601` - `E0699`)
- `E0601`: An `impl` does not provide a method its trait declares, or provides it
  with a different arity. Also used for a trait that declares one method twice.
- `E0602`: An `impl` provides a method its trait does not declare; or a type
  already has an `impl`.
- `E0603`: A method call cannot be resolved — the receiver's type has no `impl`,
  or the `impl` does not provide that method; or an `impl` names a trait that is
  not declared.

#### Warnings (`W0001` - `W0299`)

No warning is currently defined.

A warning never makes a program invalid: the checker's verdict is a count of
*errors*, so one is reported, printed on the way through, and the program
continues. Counting every diagnostic as a failure would erase that distinction.

`W0201` was the warning that stood in while numeric widths were aliases of `Int`.
Widths are real types now, so it is retired — a diagnostic that fires on correct
code is worse than no diagnostic at all.

---

## 3. Formatting Standard (Rustc-Style)

All compiler error output adheres to this standard:

```text
error[E0101]: type mismatch

  12 | let price: Money = "100"
                         ^^^^^

expected:
    Money

found:
    String

help:
    use money(100)
```

---

## 4. Runtime Error Taxonomy

When an unrecoverable operational condition occurs during execution, the runtime generates a `RuntimeError`:

| Runtime Error Category | Condition | Evaluator / VM Behavior |
|---|---|---|
| `DivisionByZero` | Integer division or modulo by zero (`/ 0`, `% 0`) | Terminate with `RuntimeError: division by zero`. |
| `StackOverflow` | VM evaluation stack exceeds maximum allocated depth | Terminate with `RuntimeError: stack overflow`. |
| `FrameOverflow` | Call frame depth exceeds `MaxFrames` | Terminate with `RuntimeError: call frame overflow`. |
| `IndexOutOfBounds` | Array index $< 0$ or $\ge \text{length}$ | Returns `Null` or raises `IndexOutOfBounds`. |
| `InvalidBytecode` | Unknown opcode or malformed instruction sequence | Terminate with `RuntimeError: invalid opcode 0x..`. |
| `CapabilityDenied` | `OpNativeCall` lacks host authorization | Terminate with `RuntimeError: capability <name> denied`. |
| `TaskCancelled` | Task execution aborted via cancellation signal | Returns `TaskCancelledError`. |

All backends (Evaluator, Stack VM, WASM) must report identical runtime error categories for equivalent trigger programs.

### 4.1 The category is the contract, not the message

Message text is for humans and may change without breaking conformance. The
**category** is the machine-readable contract, and it is the only thing a host
can branch on.

`object.RuntimeError` carries both. It is deliberately usable as two interfaces
at once:

- as an `object.Object`, which is how the tree-walking evaluator unwinds by
  returning values;
- as a `Go error`, which is how the stack VM unwinds by returning from `Run`.

`object.CategoryOf` reads the category from either shape, unwrapping when
necessary. A bare `*object.Error` reports *no* category rather than being given
one, so "categorised" and "legacy" stay distinguishable.

### 4.2 Extended categories

Beyond the table above, the closed set also includes:

| Category | Condition |
|---|---|
| `TaskFaulted` | A task terminated with an error |
| `TypeError` | An operation applied to a value of the wrong type |
| `UnknownOperation` | A native operation with no implementation |
| `Unsupported` | A valid construct the current backend cannot execute |

`object.IsValidCategory` reports membership. The set is closed so that a
consumer can switch exhaustively over it; a typo should fail loudly rather than
create an unmatchable value.

### 4.3 Which limit binds

`StackOverflow` and `FrameOverflow` describe different failures, so which one a
deeply recursive program reports must be decided deliberately rather than
emerging from constant sizes.

**Frame depth is the binding constraint in both engines.** A program that
recurses without bound is told it ran out of frames:

- the VM checks `MaxFrames` in `callClosure` *before* stack capacity, and
  `StackSize` is sized at `MaxFrames * 8` so the stack cannot trip first;
- the tree-walking evaluator bounds call depth at the same 1024 in
  `applyFunction`, counting frames rather than AST nodes.

This previously disagreed: the VM's smaller operand stack reported
`StackOverflow` while the evaluator recursed in Go until the process died. The
evaluator had no bound at all before this revision.

`StackOverflow` therefore means a single frame needing more slots than the whole
stack, which is a genuine capacity failure rather than recursion depth.

### 4.4 Wrapping preserves structure

A capability refusal reaches the VM as a `*capability.DeniedError`, which
carries the operation name and the missing capabilities. The VM wraps it in a
`RuntimeError` so that the failure carries a category like every other, and
`RuntimeError.Unwrap` keeps the cause reachable. `capability.AsDenied` unwraps,
so a host gets both the category and the structure.

### 4.5 A failing builtin stops the program

A builtin reports failure by **returning an error object**, not by raising
anything. The evaluator stops there because every one of its `Eval` calls
propagates the value.

The VM must therefore check what a builtin returned. Pushing the error and
carrying on made `assert(false, "x")` a no-op under `--vm`: the program reached
the statement after it and the assertion never happened. The check belongs in
`callBuiltin`, where the value is produced, rather than after every instruction,
because a per-instruction check costs on every step to catch a case that arises
only at a call.

A `RuntimeError` keeps its category, as §4.1 requires. A bare `Error` carries
none, so it is reported with the message it already had rather than being given a
category in the VM alone — inventing one would make the two engines print
different text for the same fault.

### 4.6 Values crossing an engine boundary

A value produced by a Go builtin is an ordinary `*object.Boolean`, not the
receiving package's `True` or `False` singleton. Truthiness, `!`, and `==` must
therefore compare by **type and value**, never by pointer identity.

With identity comparison, `isOk(x)` returned a boolean the VM did not recognise
as false, so `if (isOk(x)) { ... } else { ... }` always took the first branch
under `--vm` while the evaluator was correct. The same mistake in `!` made
`!isOk(x)` always true, and in `==` made `isOk(x) == false` always false.

`object.TRUE`, `object.FALSE`, and `object.NULL` exist so that packages share one
instance, but the comparisons do not rely on it: correctness must not depend on
which allocation path produced the value.

### 4.7 Conformance evidence

| Test | Guards |
| --- | --- |
| `TestBothEnginesReportTheSameErrorCategory` | both engines, six fault kinds |
| `TestDeepRecursionReportsFrameOverflowNotStackOverflow` | §4.3 |
| `TestBoundedRecursionStillSucceeds` | the bound rejects nothing legitimate |
| `TestRuntimeErrorCategorySetIsClosed` | §4.2 |
| `TestCategoryOfDistinguishesLegacyErrors` | §4.1 |
| `TestCapabilityDenialKeepsItsStructureThroughTheCategoryWrapper` | §4.4 |
| `TestDualEngineConformance` `ExpectCategory` field | category equality per case |
| `TestFailingBuiltinStopsTheProgram` | §4.5, both engines |
| `TestFailingBuiltinInsideALoopStops` | §4.5 away from the top level |
| `TestBuiltinBooleanIsFalsyInCondition` | §4.6, six builtins |
| `TestBangOnBuiltinBoolean` | §4.6 for `!` |
| `TestEqualityOfBuiltinBoolean` | §4.6 for `==` and `!=` |

The WASM backend is **not yet covered**: it cannot execute loops or conditionals
at all (see `LANGUAGE_SPEC.md` §8), so no trigger program exists for it. It is
not claimed as conforming.
