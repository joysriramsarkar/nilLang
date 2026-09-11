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