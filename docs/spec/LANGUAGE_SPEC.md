# NilLang (নীলাং) Formal Language Specification
**Version:** 1.0.0-draft  
**Status:** Authoritative Normative Specification  
**Ecosystem:** NilLang Core Language, Alap Framework, Onuron OS  

---

## 1. Introduction & Design Philosophy

NilLang (নীলাং) is a strict, compact, general-purpose system and application programming language engineered for both human developers and Artificial Intelligence code generation engines.

### Key Tenets
1. **Single Canonical Syntax ("One Way To Do It")**: Eliminates syntactic ambiguity, making the language predictable, readable, and maximally resilient against AI hallucinations.
2. **First-Class Intermediate Representations (HIR & MIR)**: Target-independent compilation pipelines bridging high-level declarative logic down to Stack VM bytecode, WebAssembly (WASM), and native system bridges.
3. **Sound, Multi-Tiered Type System**: Complete static and structural typing incorporating algebraic data types (ADTs), generics, union types, optional/result types, traits, capabilities, and operational effect annotations.
4. **Verified Novelty & AI-Compiler Oracle**: The compiler functions as an active truth oracle. AI code generation models query compiler-verified symbols, capabilities, and contracts before proposing changes, while new extensions pass through a formal six-stage verification sandbox.
5. **Clear Architectural Separation**:
   - **NilLang**: The strict core programming language (types, grammar, memory, execution, IR, bytecode, VM).
   - **Alap**: Cross-platform application architecture framework (UI, Web, Mobile, Server, Data, SoftBus).
   - **Onuron**: Operating system platform and kernel runtime.

---

## 2. Lexical Structure & Tokens

### 2.1 Source Character Set
Source files are encoded in UTF-8. Identifiers and string literals natively support Unicode characters, including standard Bengali glyphs.

### 2.2 Comments
- Line comment: `// Single line comment`
- Block comment: `/* Multi-line comment */`

### 2.3 Keywords
```text
let       const     fn        return    if        else
while     for       import    export    type      struct
enum      trait     impl      async     await     match
pure      unsafe    requires  ensures   invariant
```

Declarative UI / Framework extension keywords:
```text
component state     render    emit      on        build     style
```

### 2.4 Literals
- **Integers**: Decimal integers (`0`, `42`, `-10`).
- **Floats**: IEEE-754 64-bit floating point (`3.14159`, `-0.001`).
- **Booleans**: `true`, `false`.
- **Null**: `null`.
- **Strings**: Enclosed in double quotes `"..."`. Supports escape sequences (`\n`, `\t`, `\"`, `\\`) and string interpolation `\(expression)`.

---

## 3. Grammar (EBNF Summary)

```ebnf
Program        ::= Statement* EOF

Statement      ::= LetStatement
	             | AppStatement
                | ComponentDeclaration
                | StateDeclaration
                 | ConstStatement
                 | AssignStatement
                 | ReturnStatement
                 | IfStatement
                 | WhileStatement
                 | ForStatement
                 | BlockStatement
                 | StructDecl
                 | EnumDecl
                 | TraitDecl
                 | ExpressionStatement

LetStatement   ::= "let" Identifier ( ":" Type )? ( "=" Expression )? ";"
ConstStatement ::= "const" Identifier ( ":" Type )? "=" Expression ";"
AssignStatement::= Identifier ( "=" | "+=" | "-=" ) Expression ";"
ReturnStatement::= "return" Expression? ";"

An assignment updates an existing binding. It never declares a binding. The
target is resolved lexically from the innermost scope outward; assigning to an
unresolved identifier is a compile-time error (`E0102`). Bindings are introduced
by declarations such as `let`, `const`, `state`, parameters, and named language
constructs only.

IfStatement    ::= "if" "(" Expression ")" BlockStatement ( "else" ( IfStatement | BlockStatement ) )?
WhileStatement ::= "while" "(" Expression ")" BlockStatement
ForStatement   ::= "for" "(" Identifier "in" Expression ")" BlockStatement

BlockStatement ::= "{" Statement* "}"
AppStatement   ::= "app" Identifier? BlockStatement
StateDeclaration ::= "state" Identifier ( ":" Identifier )? ( "=" Expression )? ";"?

ComponentDeclaration ::= "component" Identifier "{" ComponentMember* "}"
ComponentMember ::= StateDeclaration
                  | "render" BlockStatement
                  | "build" BlockStatement
                  | "on" Identifier ( "(" Identifier? ")" )? BlockStatement
                  | Statement

Expression     ::= LogicalOr
LogicalOr      ::= LogicalAnd ( "||" LogicalAnd )*
LogicalAnd     ::= Equality ( "&&" Equality )*
Equality       ::= Relational ( ( "==" | "!=" ) Relational )*
Relational     ::= Additive ( ( "<" | "<=" | ">" | ">=" ) Additive )*
Additive       ::= Multiplicative ( ( "+" | "-" ) Multiplicative )*
Multiplicative ::= Unary ( ( "*" | "/" | "%" ) Unary )*
Unary          ::= ( "-" | "!" | "await" ) Unary | Postfix
Postfix        ::= Primary ( CallExpr | IndexExpr | MemberExpr )*

CallExpr       ::= "(" ( Expression ( "," Expression )* )? ")"
IndexExpr      ::= "[" Expression "]"
MemberExpr     ::= "." ( Identifier | "state" | "render" | "emit" | "on" | "build" )

Primary        ::= Identifier
                 | IntegerLiteral
                 | FloatLiteral
                 | StringLiteral
                 | BooleanLiteral
                 | "null"
                 | "(" Expression ")"
                 | ListLiteral
                 | HashLiteral
                 | FunctionLiteral

`emit(name, payload?)` is a callable expression. Within an evaluator component it records the
most recent event as `component.lastEvent`; in compiled bytecode it returns the payload without
host delivery. Host event delivery is performed through component event handlers. `render` is the
preferred Page-tree producer; when it is absent, `build` is used as the component's Page-tree
producer by the CLI renderer.
```

---

## 4. Formal Type System

### 4.1 Primitive Types
| Type | Representation | Default | Description |
|---|---|---|---|
| `Int` | 64-bit signed integer | `0` | Standard integer value |
| `Float` | 64-bit IEEE float | `0.0` | Standard floating-point number |
| `String`| UTF-8 string | `""` | Immutable string slice |
| `Bool` | Boolean | `false` | `true` or `false` |
| `Byte` | 8-bit unsigned integer | `0` | Raw byte |
| `Null` | Nil reference | `null` | Absence of value |
| `Void` | Empty return | - | Unit type for side-effect operations |
| `Any` | Dynamic object | `null` | Unchecked top type |

### 4.2 Compound & Algebraic Types
1. **Struct**: Named collection of typed fields.
   ```nil
   struct User {
       id: Int,
       name: String,
       active: Bool
   }
   ```
2. **Enum**: Tagged union with optional payload.
   ```nil
   enum Status {
       Pending,
       Active(Int),
       Failed(String)
   }
   ```
3. **Generics**: Parameterized types: `List<T>`, `Hash<K, V>`.
4. **Union Types**: Disjunction of multiple types: `Int | String`.
5. **Optional Types**: Shorthand `?T` equivalent to `T | Null`.
6. **Result Types**: `Result<T, E>` with `Ok(T)` or `Err(E)`.
7. **Trait / Interface**: Contract of method signatures implemented by structs.
   ```nil
   trait Serializable {
       fn serialize() -> String;
   }
   ```
8. **Function Types**: First-class function signature `fn(Arg1, Arg2) -> ReturnType [Effects]`.

---

## 5. Effect & Capability System

NilLang incorporates security and side-effect guarantees directly into the compiler type checker.

### 5.1 Operational Effects
Every expression and function possesses an effect signature:
- `pure`: Mathematically pure. No external I/O, no mutation outside local stack. Deterministic.
- `read`: Reads memory, state, or environment.
- `write`: Mutates shared state or local fields.
- `network`: Performs network transmission or socket communication.
- `spawn`: Spawns asynchronous tasks, threads, or actors.
- `unsafe`: Calls low-level C/Rust FFI or direct memory pointers.

### 5.2 System Capabilities
Hardware and platform access requires explicit capability declarations:
```text
Filesystem   Network     Camera      GPS         Bluetooth
GPU          Database    Process     Crypto      AI
Sensors      Audio
```

Example project declaration in `nil.json`:
```json
{
  "name": "vision-app",
  "capabilities": ["Camera", "GPU", "Network"]
}
```
If a function invokes a Camera API when `"Camera"` is missing from the capability matrix, compilation halts immediately with diagnostic `E0201: CapabilityViolation`.

---

## 6. Intermediate Representations (HIR & MIR)

### 6.1 Compiler Pipeline Architecture
```text
           Source Code (.nil)
                   │
                   ▼
                 Lexer
                   │
                   ▼
                 Parser
                   │
                   ▼
         Abstract Syntax Tree (AST)
                   │
                   ▼
         Type & Capability Checker
                   │
                   ▼
      High-Level Intermediate Rep (HIR)
        • Scope & Symbol Resolution
        • Desugaring
        • Constant Folding
                   │
                   ▼
       Mid-Level Intermediate Rep (MIR)
        • Control Flow Graphs (CFG)
        • Three-Address / Basic Blocks
        • Dead Code Elimination
                   │
         ┌─────────┼─────────┐
         ▼         ▼         ▼
      NABC VM     WASM     Native / FFI
      Bytecode   Binary    (Rust/C Bridge)
```

### 6.2 High-Level IR (HIR)
HIR represents desugared, type-annotated syntax where syntactic sugars (e.g. `+=`, string interpolation `\(x)`, loops) are standardized into normalized semantic nodes. HIR performs early constant folding (e.g. `2 + 3 * 4` $\to$ `14`).

### 6.3 Mid-Level IR (MIR)
MIR breaks procedural execution into a Control Flow Graph (CFG) comprised of `BasicBlock` structures terminating in unconditional jumps, conditional branches (`BranchIf`), or `Return`. Instructions are linearized three-address statements operating on constants, stack variables, and compiler-generated temporaries (`_t0`, `_t1`).

### 6.4 Application Entry Block
`app { ... }` and the named form `app Name { ... }` are executable top-level statements. The body executes exactly once in module scope, and declarations made in the body remain visible to following module statements. `app` is contextual: outside an application entry form, it remains a legal identifier.

Both the tree-walking evaluator and the bytecode VM implement the same execution and visibility semantics.

### 6.5 Tasks and Channels
`task { ... }` starts a zero-argument asynchronous computation and returns a future. `await future` blocks the current computation until the task finishes and returns its result. Awaiting the same future more than once returns the same result.

Tasks receive a snapshot of visible lexical bindings. Mutable runtime objects in that snapshot, including channels, retain their identity and can be used for explicit communication. `Channel(capacity)` creates a buffered channel; `send(channel, value)` blocks until the value can be sent, and `receive(channel)` blocks until a value is available. The tree-walking evaluator additionally accepts `channel.send(value)` and `channel.receive()`.

### 6.6 NABC Bytecode Image
A `.nabc` artifact is a versioned binary image beginning with the `NABC` magic header. Version 1 stores the instruction stream and the complete constant pool, including integer, float, string, and nested compiled-function constants with local and parameter metadata.

Bundle runners decode and validate the image before VM execution. The bytecode entry can therefore execute without bundled NilLang source. Invalid headers, unsupported versions or constants, excessive lengths, truncated data, and trailing bytes are rejected.

### 6.7 Declarative Components
A component declaration creates one module-scoped component value. Each `state` member creates a
mutable binding captured by `render`, `build`, and `on` blocks. `render` and `build` are zero-argument
functions. Event blocks accept zero or one payload parameter, are exposed by name, and execute in
the same captured state scope. A declared payload parameter is bound to `null` when omitted.

```nil
component Counter {
   state count: i32 = 0;
   render {
      return {
         "type": "Page",
         "title": "Counter",
         "content": [{"type": "Button", "event": "increment", "payload": {"step": 1}}]
      };
   }
   on increment(payload) { count = count + payload["step"]; emit("changed", count); }
}
```

The evaluator exposes `component.dispatch("increment", payload)`; the payload argument is optional.
Compiled bytecode exposes the equivalent handler as `component.events.increment(payload)`. Calling
`render()` again observes the updated state.
Render output is an ordinary hash tree. `nil render file.nil` discovers a declared component,
invokes `render` (or `build` when no `render` member exists), and converts a Page-shaped tree to
the existing ANSI and HTML Alap renderers. `nil render file.nil --event increment` dispatches one
named event before rendering, so the generated preview contains the updated tree and serialized
component state for browser hydration. This is a deterministic single-event preview contract, not
a persistent browser event loop. `nil dev file.nil` provides the persistent development contract:
`data-alap-click` actions and optional JSON `data-alap-payload` metadata are posted to
`POST /__alap/event`. The server serially dispatches each session's handler and returns a newly
rendered Alap root. The browser replaces that root and hydrates the returned state before binding
the next action. Scripts can invoke the same contract through `__ALAP__.dispatch(name, payload)`.

Component declarations, state, render/build blocks, and named events are preserved through AST,
type checking, HIR, MIR, evaluator, and direct bytecode compilation. Current components are
module-scoped singletons. Automatic dependency tracking, DOM patch scheduling, native windows,
and GPU command submission are not language-runtime guarantees in this version.

The development event loop uses one mutex-protected component instance per browser cookie session.
Sessions are isolated from one another and expire after 30 minutes of inactivity. Unknown or expired
session cookies create a fresh component instance. Incremental DOM reconciliation and source hot
reload with state preservation are not part of this contract.

---

## 7. AI-Compiler Oracle & Verified Novelty

### 7.1 Compiler-as-an-Oracle API
AI models interact with the compiler through deterministic introspection endpoints:
- `list_types()`: Returns all known primitive and user-defined types.
- `list_functions()`: Returns all callable signatures and effect sets.
- `find_symbol(name)`: Exact symbol metadata and contract specification.
- `check_expression(expr)`: Performs instantaneous type-check on an expression without evaluating it.
- `explain_error(err)`: Provides actionable, structured diagnostic explanations and suggested corrections.

### 7.2 Novelty Verification Lifecycle
When an AI proposes novel components, functions, or packages, the proposal undergoes a 6-stage verification pipeline:
```text
Proposal (EXPERIMENTAL)
   │
   ├─► Stage 1: Lexical & AST Parse Check
   ├─► Stage 2: Type & Capability Safety Validation
   ├─► Stage 3: HIR/MIR Compilation & Optimization
   ├─► Stage 4: Unit Assertion & Property Test Suite
   ├─► Stage 5: Sandbox Capability & Security Audit
   └─► Stage 6: Certification & Ed25519 Signing ──► Status: VERIFIED / STABLE
```

---

## 8. WebAssembly (WASM) Target

NilLang compiles directly from MIR to standard WebAssembly (WASM) modules (`.wasm`) and WebAssembly Text (`.wat`):
- Generates standard WASM sections: Types, Functions, Exports, Memory, and Code.
- Provides a unified Browser Runtime (`nil_runtime.js`) for DOM bindings, canvas drawing, timer scheduling, and console I/O.
- Powers the `alap/web` profile, transforming declarative Alap components into high-performance web applications.
