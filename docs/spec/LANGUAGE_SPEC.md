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

Declarative UI / Framework extension keywords (supported in AST/parser):
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

IfStatement    ::= "if" "(" Expression ")" BlockStatement ( "else" ( IfStatement | BlockStatement ) )?
WhileStatement ::= "while" "(" Expression ")" BlockStatement
ForStatement   ::= "for" "(" Identifier "in" Expression ")" BlockStatement

BlockStatement ::= "{" Statement* "}"

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
MemberExpr     ::= "." Identifier

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
