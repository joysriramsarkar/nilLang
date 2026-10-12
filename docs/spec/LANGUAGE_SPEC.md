# NilLang (নীলাং) Formal Language Specification
**Version:** 1.0.0-draft  
**Status:** Draft Normative Specification
**Ecosystem:** NilLang Core Language, Alap Framework, Onuron OS  

---

## 0. Conformance Status

This document defines the target language and labels the Nilang 0.1 contract.
Only behavior marked **Implemented** is a frozen compatibility guarantee.
**Experimental** behavior may be exercised but is not stable. **Planned**
behavior is design material and must not be accepted as an implementation
claim.

| Surface | Nilang 0.1 status |
| --- | --- |
| Literals, expressions, `let`, `const`, assignment, functions, closures, arrays, hashes, and control flow | Implemented |
| Components, state, events, entities, `task`, `await`, and imports | Experimental |
| Numeric literals: Bengali digits, `_` separators, `0x`/`0o`/`0b` radix | Implemented |
| Structured syntactic diagnostics (`E0001`-`E0005`) with spans | Implemented |
| Single frontend gate (`frontend.Check`) for every compilation path | Implemented |
| `for` loops, ranges, `break`, and `continue` | Implemented |
| `struct` declarations, struct literals, and field access | Implemented |
| `enum` declarations and variant construction | Implemented |
| `match` with literals, bindings, guards, or-patterns, enum, struct, and `Ok`/`Err`/`Some`/`None` patterns | Implemented |
| Exhaustiveness checking (`E0507`) for enums, bool, `Result`, and `Optional` | Implemented |
| `while let` with any `match` pattern | Implemented |
| `trait` declarations and `impl Trait for Type` blocks | Implemented |
| Optional access (`a?.b`) and null coalescing (`a ?? b`) | Implemented |
| `isNull` builtin | Implemented |
| Method calls (`x.m(args)`) resolved against the receiver's static type | Implemented |
| `for` loop bodies type-checked | Implemented |
| `type` aliases | Implemented |
| Parameter and return type annotations (`fn f(a: Int) -> Int`) | Implemented |
| `pure fn`, `unsafe fn`, and a `requires` capability clause | Implemented |
| `export` / `private` module visibility, enforced by both engines | Implemented |
| Shadowing a builtin with `let` | Implemented |
| Mutable by-reference closure capture in both engines | Implemented |
| Deterministic hash literal evaluation order | Implemented |
| Lowering applied to imported modules as well as the importing program | Implemented |
| Categorised runtime errors (`DivisionByZero`, `FrameOverflow`, ...) with cross-engine agreement | Implemented |
| Capability gate on the VM path | Implemented |
| Garbage collector wired to the VM's allocation and roots | Implemented |
| Traits, `impl`, and generics | Implemented (traits and impl); generics Planned |
| Static methods (`Type.new()`) | Planned |
| Compile-time effect checking beyond the `pure` modifier | Planned |
| Union types beyond `?T`, and generic traits | Implemented (unions `A \| B`); generic traits Planned |
| Generic functions (`fn map<T, U>(...)`) with structural inference at call sites | Implemented |
| Member chains (`a.b.c.d`) resolved against the declared type, `E0111` | Implemented |
| Function types with contravariant parameters and covariant returns | Implemented |
| Higher-order functions, including generic functions as arguments | Implemented |
| Numeric width annotations (`i8`..`u64`, `f32`/`f64`) as distinct types | Implemented — literal ranges checked; no implicit coercion |
| `Result<T,E>` / `?T` narrowing through `unwrap`, `unwrapOr`, `Ok`, `Err`, `Some` | Implemented |
| Struct/Enum/Trait checking: field existence, mismatch, assignability, exhaustiveness | Implemented |
| Union type checking (`A \| B`) and optional assignability in both directions | Implemented |
| Generics (`fn map<T, U>`, inference) | Implemented — structural inference; see TYPE_SYSTEM §6.1 for the boundary |
| AST -> typed HIR -> MIR as the only backend path | Planned |
| Stack VM bytecode image | Implemented |
| WASM target | Experimental |

The repository tests, not an unchecked example in this draft, are the
conformance evidence for an Implemented row.

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
- **Integers**: 64-bit signed integers in decimal (`0`, `42`), hexadecimal (`0xFF`),
  octal (`0o777`), or binary (`0b1010`). `_` may be used as a readability separator
  (`1_000_000`). Bengali (`৪২`) and fullwidth (`１２３`) decimal digits denote the same
  values as their ASCII equivalents, so a program may be written entirely in
  Bengali script. A negative literal is written with a leading `-` operator, never
  as part of the literal token.
- **Floats**: IEEE-754 64-bit floating point (`3.14159`, `-0.001`, `১.৫`). The decimal
  point is always `.`; digits may be Bengali or ASCII.
- **Booleans**: `true`, `false`.
- **Null**: `null`.
- **Strings**: Enclosed in double quotes `"..."`. Supports escape sequences (`\n`, `\t`, `\"`, `\\`) and string interpolation `\(expression)`.

  *Known limitation:* a string literal cannot appear inside an interpolation,
  because the lexer terminates a string at the first unescaped quote. Write
  `let parts = ["a", "b"]; "\(join(parts, "-"))"` rather than
  `"\(join(["a", "b"], "-"))"`.

See `SYNTACTIC_DIAGNOSTICS.md` for the folding rules and their scope.

### 2.5 Diagnostics

Syntactic errors are reported as structured diagnostics with a stable code from
`ERROR_MODEL.md`, a span, the quoted source line, and a suggestion. The codes are
`E0001` (unexpected token), `E0002` (unterminated string), `E0003` (unexpected end
of file), and `E0004` (expected token X, got Y). Every compilation path obtains
them through `frontend.Check`. See `SYNTACTIC_DIAGNOSTICS.md`.

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
                 | WhileLetStatement
                 | ForStatement
                 | BlockStatement
                 | StructDecl
                 | EnumDecl
                 | TraitDecl
                 | ImplDecl
                 | VisibilityStatement
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
WhileLetStatement ::= "while" "let" MatchPattern "=" Expression BlockStatement
ForStatement   ::= "for" Identifier ( "," Identifier )? "in" Expression BlockStatement
BreakStatement ::= "break" ";"?
ContinueStatement ::= "continue" ";"?

FunctionDecl   ::= ( "pure" | "unsafe" )? "fn" Identifier TypeParams? "(" Parameter ( "," Parameter )* ")" ( "->" TypeName )? RequiresClause? BlockStatement
TypeParams     ::= "<" Identifier ( "," Identifier )* ">"
Parameter      ::= Identifier ( ":" TypeName )?
RequiresClause ::= "requires" STRING ( "," STRING )*

TraitDecl      ::= "trait" Identifier "{" TraitMethod* "}"
TraitMethod    ::= "fn" Identifier "(" Parameter ( "," Parameter )* ")" ( "->" TypeName )?
ImplDecl       ::= "impl" Identifier "for" Identifier "{" ImplMethod* "}"
ImplMethod     ::= "fn" Identifier "(" Parameter ( "," Parameter )* ")" ( "->" TypeName )? BlockStatement

// A method's first parameter is the receiver. It is written as an ordinary
// parameter, named `self` by convention, and `self` is a soft keyword: a method
// that does not name it, and any use of the name outside a method, both work.
MethodDecl     ::= ImplMethod

StructDecl     ::= "struct" Identifier "{" StructField ( "," StructField )* ","? "}"
StructField    ::= Identifier ":" TypeName
StructLiteral  ::= Identifier "{" ( Identifier ":" Expression ) ( "," Identifier ":" Expression )* ","? "}"

EnumDecl       ::= "enum" Identifier "{" EnumVariant ( "," EnumVariant )* ","? "}"
EnumVariant    ::= Identifier ( "(" TypeName ")" )?

TypeAliasDecl  ::= "type" Identifier "=" TypeName

CoalesceExpression ::= Expression "??" Expression
SafeAccessExpression ::= Expression "?." Identifier

VisibilityStatement ::= ( "export" | "private" ) DeclaredStatement
DeclaredStatement   ::= LetStatement
                     | ConstStatement
                     | FunctionDecl
                     | StructDecl
                     | EnumDecl
                     | TypeAliasDecl

MatchExpression::= "match" Expression "{" MatchArm ( "," MatchArm )* ","? "}"
MatchArm       ::= MatchPattern ( "if" Expression )? "=>" Expression
MatchPattern   ::= "_"
                | MatchPattern "|" MatchPattern
                | Identifier "{" MatchField ( "," MatchField )* "}"
                | Identifier ( Identifier | "_" )?
                | Literal

The keywords `type`, `pure`, `unsafe`, `requires`, `export`, `private`, and
`self` are **soft keywords**: a declaration introduced by one is recognised only
when the rest of the construct follows. `let type = "widget"` and `pure = 1` are
ordinary bindings, and a bare `pure` in expression position is a variable read,
not a modifier. A hard keyword would take the name away for no benefit, and
`unsafe` in particular is a plausible identifier in a systems-adjacent language.

`self` was a hard keyword in anticipation of a receiver syntax the language does
not have. A method's receiver is an ordinary first parameter, so there is nothing
left for `self` to disambiguate, and making it soft keeps the name available.

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
// A range binds to the expression on its left, so `0..n - 1` means
// `0..(n - 1)` rather than `(0..n) - 1`. It is only meaningful in a
// `for` header; anywhere else it is rejected (E0005).
Range          ::= Additive ( ".." Additive | "..=" Additive )
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

### 4.2 Compound & Algebraic Types (Planned Unless Marked Otherwise)
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

## 5. Effect & Capability System (Planned)

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

### 6.1 Target Compiler Pipeline Architecture
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

**Every node must have a translation, or say that it does not.** An unrecognised node used to lower to `nil` — indistinguishable from a legitimately empty translation — so a construct that had silently stopped working looked exactly like one that had never existed. The lowering now records every node type it could not translate, and `Lowerer.UnhandledNodes()` reports the tally.

**Backends compile the lowered program, never the parsed one.** `Result.Lowered` carries this contract in its own documentation, and three call sites read `Result.Program` instead. Nothing failed when they did; the programs simply behaved differently. That was the whole of `L3.2`'s cost: handed the unlowered tree, the HIR path met `for`, `match`, `struct`, `trait`, `?.` and `??` — constructs the lowering pass desugars into `while` loops, `if` chains and calls — and had no case for any of them, so each would have needed its own implementation *in HIR*. That is the duplication the contract exists to prevent: two places turning a `for` into something, free to disagree about what. `cmd/nil/backend_program.go` now holds the contract in one function, so the mistake is visible rather than repeatable.

**A declaration is data, not nothing.** `struct P { x: Int }` emits no instruction, and dropping it is tempting — but a dropped node cannot be reported, printed, or answered for, and "what does this program declare?" is the first question a language server asks. `TypeDeclStmt` keeps the name, fields and variants; backends ignore it, which is what they should do, and a pass that needs it has something to read.

**A pass that rebuilds a node copies every field.** The HIR optimiser rebuilds each node it visits rather than mutating it, which is what makes constant folding possible. A `case` that forgets a field produces no failure — it produces a node that is 99% right, and the missing 1% surfaces somewhere else entirely: a named function printed as `fn()` only after optimisation, so the fault appeared to belong to the folding logic rather than to the line that dropped the field. `TestTheOptimizerPreservesNodeIdentity` asserts on optimised output, because that is where the omission is visible.

**`let x = v; body` lowers to a block, and that is the whole trick.** `?.`, `??` and `match` all desugar into this node, because binding the receiver once is what stops a chain from reading the same value twice. It becomes `BlockExpr` — a block used where a value is expected — rather than a node with its own body field, which MIR would then have to learn for every construct that produces one.

### 6.3 Mid-Level IR (MIR)

MIR breaks procedural execution into a Control Flow Graph (CFG) comprised of `BasicBlock` structures terminating in unconditional jumps, conditional branches (`BranchIf`), or `Return`. Instructions are linearized three-address statements operating on constants, stack variables, and compiler-generated temporaries (`_t0`, `_t1`).

**A temporary is a name with a slot, not a position on the operand stack.** The VM's operand stack is a stack; an SSA-style name outlives the expression that pushed it. `_t0 = load n; _t1 = _t0 > 3` pushes two values, and reading `_t0` back has to mean the first one. A temporary that means "whatever is on top" is right once — for the instruction immediately after — and wrong for every other use.

**A two-way choice names both arms.** `cond ? a : b` lowers to a `SelectInst` carrying `Cond`, `True` and `False`, which the emitter expands into a jump over the arm not taken. Both arms must push exactly one value and neither may run, which is why they are expressions rather than blocks: a block could leave the stack unbalanced, and the arm that did not run would leave it wrong for everything after.

A select that names only one arm cannot be printed correctly, which is the point. The lowering once answered a ternary with a binary operator whose right operand was the consequence alone — the alternative block was built and then discarded — and the emitted IR read `_t2 = _t1 ? "big"` for both `n > 3` and `n <= 3`.

### 6.3.1 Backends and the canonical path

Every backend consumes the lowered program, and the MIR path is `AST → HIR → MIR → target`. `nil run --vm --backend=mir` selects it; the default is the older AST-to-bytecode compiler.

`TestThreeEnginesAgree` is what holds the two to the same behaviour: twenty-two programs run through the evaluator, the direct compiler and the MIR emitter, requiring all three to answer identically. Four faults were found this way that no amount of reading MIR would have shown:

- a list literal compiled to `call array(...)` and a map to `call map(...)`, names the VM does not have — a literal is not a call, and spelling it as one invented a function every backend would have had to be taught. They are `AggregateInst`, emitting `OpArray` / `OpHash`;
- every named function became `fn_anon_N`, so no call site could find it;
- subscript and field access emitted no opcode at all, leaving both operands on the stack, so `xs[1]` produced `1` and `m["k"]` produced `"k"` — the right *shape* of answer, answering the wrong question;
- a builtin call pushed its arguments before its callee, so every builtin failed with a message naming the argument rather than the ordering.

**The MIR path is selectable, not the default.** Those four are the language's core. Modules, imports, `task`/`await`, components and capabilities have not been through the comparison, and swapping a shipped path for a partly-verified one is a worse trade than the duplication `L3.4` set out to remove. Promoting it is gated on conformance over the whole suite.

**What the MIR backend does not carry yet.** Source positions: MIR instructions have no token, so a breakpoint cannot be placed by line. Inventing positions would put a debugger on a plausible-looking wrong line, which is worse than admitting the step is unlocated. Global names *are* emitted, because a backend that omits them silently reduces a debugger to a slot viewer.

### 6.3.2 Optimisation passes

MIR runs, to a fixed point: constant folding, branch simplification and copy propagation, redundant-computation elimination, block merging, dead code elimination. HIR runs constant folding, constant conditions, and loop-invariant hoisting.

**A temporary is only rewritten with what was known *before* it.** An instruction joins the copy map after its operands are rewritten, never before. Recording first lets an instruction rewrite its own destination to a value it is about to produce: `_t1 = load a` reads `a`, and a map containing `_t1 = a` makes that same instruction read the uninitialised slot it was about to write. The MIR is then self-consistent and answers wrongly.

**Writing to a variable invalidates every fact known about it.** Without this, `store a, 2` after `_t0 = load a` leaves `_t0 = a` in the map, and a later use of `_t0` reads the *new* `a`.

**Value numbering runs after copy propagation, not before.** It compares instructions by their operands, so a `load` propagation has already replaced with the variable it read is no longer shaped like the load beside it — and the two would not match though they compute the same value. A pass must also refuse to replace a computation with *its own* result: `_t2 = _t2` computes the same value as itself.

**Hoisting moves only leading bindings, and only past nothing that branches.** A binding after a `return` does not run on the iteration that returns. A `const` is a promise made where it is written, and moving the promise changes where it was made.

**A binding whose name a nested loop walks is never hoisted.** This is the one that is silent. `let j = 0` before `while j < 3 { … j = j + 1 }` hoists above the *outer* loop, so the reset happens once instead of once per outer iteration: the inner loop runs once instead of N times, and a program counting N² counts N. The name is written by no statement *at the point of the binding*, so only a look at what follows finds it.

**Measured, not asserted.** `TestMeasureOptimisationGain` prints block count, instruction count and run time for four programs, taking the fastest of seven runs — a single run of a program this short is dominated by whatever else the machine was doing, and one-shot measurements of this optimiser reported both a 30% slowdown and a 30% speed-up on consecutive runs.

### 6.6 The LLVM backend

MIR lowers to **textual** LLVM IR, and the system toolchain turns that into a binary.

**Text, not a binding.** A binding to LLVM's C++ API would pin the compiler to one LLVM version, one C++ toolchain, and one platform's library conventions. A language that builds for eight targets does not want those constraints on the *compiler*; it wants them on the *machine doing the build*. Textual IR is a documented interchange format that anything — `clang`, `llc`, `lli`, a future LLVM — can read.

**`clang` alone is enough; `llc` is optional.** `clang` reads `.ll` and links in one step. `llc` stops at an object file and needs a second invocation plus a hand-written list of the C runtime's objects. Some distributions ship `clang` without `llc` — the current official Windows installer does — so requiring `llc` would report "no toolchain" on machines that have a perfectly good compiler.

**Arithmetic is a runtime call, not an LLVM instruction.** `add i64` on two tagged values produces a number, which is not what the language's `+` means: the language's `+` on two strings concatenates, and on a string and an integer is an error rather than an address. A direct `add` would have to understand the runtime's tagging, which means a second implementation of "what does `+` mean" alongside the interpreter's and the VM's.

**Every variable is an `alloca`.** The simplest correct memory model, and deliberately not the fast one: each slot has a name and an address, so a value is read and written by name rather than tracked as an SSA value a branch must reconcile. `mem2reg` promotes these in one pass, so this is the cost of being obviously correct, paid once at a known optimisation step.

**A select evaluates both arms before the branch.** LLVM requires every phi at the top of its block, so an arm's value has to exist before the label that introduces it. Evaluating an arm that is not taken costs a discarded call, which `instcombine` removes — cheaper than IR LLVM will not accept.

**A temporary written in two blocks is joined by a `phi`.** The two arms of an `if` assign one MIR temporary, because the `if` is *for* a value. Emitting that name once per arm defines it twice, which LLVM rejects as *multiple definition of local value* — over a line that reads entirely ordinarily. Each arm therefore writes a block-local name and the merge block joins the copies.

A MIR *variable* is not treated this way. A variable is a slot with one `alloca`, and every block loads and stores that same address; the `alloca` already joins the blocks, so a `phi` on top would be joining what is already joined.

**Every function returns an `i64`, `main` aside.** Only the entry *has* to — it is the one that hands a value back to the process. But a `void` function cannot be called for a result, and emitting `void` for a function the program calls for a value produces a module whose result is undefined: LLVM accepts it, the linker accepts it, and the program reads whatever was in the register. One signature for all of them costs a value a caller may ignore; it buys the guarantee that no caller can be handed an undefined one.

**Parameters are written unnamed and referenced positionally** — `%0`, `%1`. LLVM names an unnamed value by position, so a reference to any other spelling is a reference to a name the function does not have. Naming parameters after their Nilang names would read better and is deliberately not done: a program may name a parameter `t0`, which is the form this backend gives a temporary, and the collision would be silent in the module and fatal in the assembler.

**A call's arguments are written as `i64 <value>`.** LLVM's textual parser reads an argument list as `type value` pairs and has no way to infer the type. `call i64 @f(%x)` is rejected with *invalid type for function argument* and the caret points at `%x`, which reads as though `%x` were malformed when it is perfectly ordinary. Every value in this language is an `i64` — a tagged word — so the type is a constant rather than something to look up.

**A call to a name this module does not define and the runtime does not own is refused, not emitted.** The alternative produces a module referring to a function nobody defined, and the failure arrives from the assembler pointing at an argument. Refusing at the point of lowering names the construct instead, which is the difference between a gap somebody can close and a symptom somebody has to diagnose from scratch. See §6.8.

**A string constant is written with LLVM's escaping, not Go's.** A byte-string literal `c"..."` understands exactly two escapes — `\\` and `\XX` — while Go's `%q` writes a NUL as `\x00` and a newline as `\n`. The two agree on `\"` and disagree on nearly everything else, so `%q` turns `"a\0b"` into a five-byte constant declared as three:

```
error: constant expression type mismatch: got type '[6 x i8]' but expected '[3 x i8]'
```

The declared length and the written bytes came from two different encoders, and the error names a *length* rather than a string — so every control character in a program is one rejected module away. The bytes are written directly instead, so `[N x i8]` always counts what was actually written.

Verified against **clang 22.1.8** on this machine. Two classes of check cover the emitter: an assertion on the emitted text, which is the part reviewable without a toolchain, and asking `clang` itself, which is the only thing that knows the textual format's rules. Neither is sufficient alone.

**Why phones need AOT (L4.14).** The desktop build cross-“compiles” by
rebuilding the Go runner and appending the `.nilax` bundle — no Nilang code
generator is involved. That is correct and fast, but every app then carries
the Go runtime (~15–25MB installed), which breaks store-size budgets and
cold-start times on Android/iOS. The LLVM AOT path instead compiles MIR to
native code linked against the small `libnilcore`/`libnilang_core` runtime
(GC + strings + bytecode VM, ≈2–4MB), entered only through the
`nil_runtime.h` C ABI. So: desktop ships the runner; phones ship AOT native
code plus the narrow core. `nil build android --size` reports the APK
against the <10MB budget (M4); see `docs/spec/MOBILE_RUNTIME.md`.

### 6.8 Conformance: three engines, one answer

The interpreter, the bytecode VM and a compiled program must answer a program **identically**. A backend cannot claim to be correct without it, and two engines agreeing does not establish it — two engines can share a mistake, and when they do it is shared invisibly.

The gate runs the same corpus through all three and compares. Its rule for a skip is that **an unrecorded reason is a failure**: a skip whose cause the project has not written down is a gap nobody is tracking, and the next reader will believe the list is complete.

Current state: **104 of 245 cases compared across all three engines, 0 disagreements, 0 unrecorded skips.** The remainder are constructs no backend implements yet — loops, `match`, struct and enum values, `?.` and `??`, and the `Result`/`Optional` constructors — each refused by name.

Three kinds of check exist because three kinds of bug exist, and no one of them catches another's:

| Kind of bug | What it looks like | What catches it |
|---|---|---|
| Silent — nothing fails | All three engines run; answers differ | The three-engine gate |
| Wrong IR — text reads correctly | The assembler rejects it | Asking `clang` |
| A real gap | The backend says what it cannot do | The skip's reason must be written down |

A unit test, a text scan, or a single engine would each have passed on every bug in that table. That is why all three are required.

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

## 8. WebAssembly (Experimental Target)

NilLang compiles directly from MIR to standard WebAssembly (WASM) modules (`.wasm`) and WebAssembly Text (`.wat`):
- Generates standard WASM sections: Types, Functions, Exports, Memory, and Code.
- Provides a unified Browser Runtime (`nil_runtime.js`) for DOM bindings, canvas drawing, timer scheduling, and console I/O.
- Powers the `alap/web` profile, transforming declarative Alap components into high-performance web applications.


## 9. FFI: Calling C

An `extern` declaration names a function this program does not define. The
declaration is a *signature*: the linker resolves it, and the language knows
nothing about the body.

### 9.1 The declaration

```nil
extern "C" fn sin(x: Float) -> Float;
extern "C" fn atan2(y: Float, x: Float) -> Float;
```

**Rules:**

1. The ABI is written as a string. Only `"C"` exists today, and it is written out
   rather than implied so that a second ABI would be a different declaration rather
   than a misspelling.
2. A declaration has **no body**. `extern "C" fn f() -> Int { return 1; }` is
   rejected: the declaration says the function is defined elsewhere, and a body
   here would be a definition the linker then finds a duplicate of.
3. A declaration must be **named**. There is nothing for the linker to resolve
   otherwise.
4. `extern` is a **soft keyword**. `extern "C" fn ...` is a declaration;
   `let extern = 1` is an ordinary binding. Requiring a string after the keyword is
   what separates them, so reserving the word would take a usable name away and
   disambiguate nothing.
5. The parameters and return type are **checked at every call site**, exactly as a
   function's are. The signature is the only thing the language knows about the
   function, so it is the only thing a call can be checked against.
6. The arity is **exact**. A C function takes a fixed number of arguments; a call
   with the wrong count is a compile-time error at the call rather than a call
   reading a register the callee never expected to be set.

### 9.2 The crossing

A Nilang value is a **tagged word**: three bits say what it is and the rest is
the payload. C has no such convention. A C `int64_t` is 64 bits of number and
nothing else.

So nothing crosses as it is. Every value is converted on the way out and converted
back on the way in, with no exception:

```nil
extern "C" fn triple(n: Int) -> Int;
triple(14);        // C receives 14, answers 42, the program holds 42
```

Without the conversion, `triple(14)` would pass the *tagged word* for 14 — which
is `171` — and read 42 back as a tagged word whose low three bits are `2`, the tag
of `true`. The program would print `true`.

**This is defined for these types and no others:**

| Nilang | crosses as | notes |
|---|---|---|
| `Int` | `int64_t` | exact for the 61 bits an inline integer has |
| `Float` | `double` | IEEE-754, copied rather than converted |
| `Bool` | `int` | 0 or 1 out; any non-zero in becomes `true` |
| `Byte` | `uint8_t` | |
| `String` | `const char *, size_t` | two arguments; see §9.5 |
| a `struct` the program declared | `const <Name> *` | one pointer to a filled buffer; see §9.16 |
| any function type | a callback handle | not a function pointer; see §9.11 |

**Anything else is refused by name.** The message names the type, the function, the
parameter, and what *is* available:

```
error: no crossing for "Array<Int>": it is neither a built-in type nor a struct this
       program declared (defined crossings: Int, Float, Bool, Byte, String, and any
       function type as a callback): extern "sort" takes it at parameter 1
```

The refusal happens **at the declaration**, not at the call. An annotation the
programmer wrote that the FFI does not support is a gap in the FFI, and it should read
like one: "there is no crossing for `Array<Int>` yet" is a fact, and "expected i64" at a
call two hundred lines later is not.

A refusal here is deliberate and it is the only safe answer. A *plausible*
conversion is the worst outcome available: the module assembles, the linker
resolves it, and the program reads a length out of a pointer. That fails on the
data rather than on the build, which is the hardest kind of failure to find and the
easiest to ship by accident.

### 9.3 Where the conversion lives

In the runtime, not in the compiler.

The representation is a fact about the runtime, and a conversion written in a back
end would be a second implementation of "what is an integer". The two would drift,
and they would drift quietly, the first time the representation changed.

Every crossing names two runtime functions — one each way — and a module declares
the ones its own calls reference:

```c
int64_t nil_int_unbox(NilValue value);   /* Nilang -> C  */
NilValue nil_int_box(int64_t value);      /* C -> Nilang  */
```

A module that uses no `extern` declares none of them, so its IR is unchanged by the
FFI's existence and a diff of two modules shows only what actually changed.

Both directions of every crossing are checked against `nil.h` and `nilffi.h` by a
test that derives its expectations from the crossing table — so the declarations
the emitter writes are compared against the headers that define them, rather than
against a second copy of the same list.

The check is only as good as the parser that reads the headers, and **a parser that
skips a construct is silent** — indistinguishable from "there are none". So every
header read also asserts that each line which *looks* like a declaration is a name
the parser found. Two forms had been invisible to all of it: a pointer return
(`NilFFIString *f(void)`) and a function-pointer parameter
(`nil_closure_new(NilValue (*function)(...))`), neither of which any declaration
used until the FFI's buffer scope added one.

### 9.4 Where the crossing table lives

In `compiler/ffi`, not in a back end. What a type becomes at the boundary is a fact
about the *pair* of languages — LLVM is not what makes `Int` an `int64_t` — and two
things now write it down: the `declare` the LLVM back end emits and the prototype
the generated header carries (§9.9). While the table lived inside the LLVM package
the header generator had either to import that package or copy it, and a copy is a
second implementation of "what is a string at the boundary" — free to disagree with
the first in a way that compiles, links, and is wrong.

### 9.5 Strings
1. A string crosses as a **`(ptr, len)` pair** — two arguments, never one. A C
   string has no length, and a length is the only thing that distinguishes an empty
   string from the end of one.
2. The length is in **bytes**, not characters. A C function counting bytes and a
   Nilang program counting characters disagree about any string outside ASCII, and
   the C side must be right: it is handing over a buffer, and a buffer has a size in
   bytes.
3. An empty string crosses as **a null pointer and a zero length**. Every C API
   written to take a length handles that, and so does `memcpy(dst, NULL, 0)`.
   Reporting a non-null pointer to nothing would be worse: a C function that checks
   for null before reading would treat it as a real string and read address zero.
4. Crossing **out** borrows. The pointer is to the string's own bytes and stays
   valid as long as the string does.
5. Crossing **in copies**. `nil_string_copy` is the only way bytes become a Nilang
   string, so the C side may free its buffer as soon as it has handed it over. This
   is the price of not having to specify when a borrow ends, which every caller
   would otherwise have to know.

**Nothing returns a struct.** A string comes back through a pointer the *caller*
allocated:

```c
typedef struct NilFFIString {
  const char *bytes;
  size_t length;
} NilFFIString;

void nil_string_unbox(NilValue value, NilFFIString *out);
void nilffi_greeting(const char *name, size_t length, NilFFIString *out);
```

This is not caution about alignment or padding. Whether a 16-byte struct comes
back in registers or through a hidden pointer **is not a convention two compilers
share**:

| compiler | triple | `{ptr, i64}` returned |
|---|---|---|
| clang | `x86_64-pc-windows-msvc` | in `RAX:RDX` |
| gcc | `x86_64-w64-mingw32` | through a pointer in `RCX` |

Both are correct implementations of their own ABI, both appear in the same link,
and the call site cannot tell which one it received. A struct return whose meaning
depends on which compiler produced the other half is not a contract.

The same applies to `sizeof` and `alignof` (L5.9): until they are agreed by both
compilers, nothing in the FFI may return an aggregate by value.

A function that answers with a string therefore has the signature a C author would
not choose — `void`, with a trailing out-parameter — and that is the point. The
callee fills, and neither side has to know a register convention at all.

### 9.6 Memory: who owns what

Four rules, in one place, because the alternative is a C author reading five
functions and assembling them in their head — and getting one wrong in a way that
surfaces as a corrupted heap three calls later.

| you have | it is owned by | it dies when | you must |
|---|---|---|---|
| a string out | the Nilang string | with the Nilang string | nothing |
| a buffer from `nil_ffi_buffer` | the runtime | at `nil_ffi_scope_close` | nothing |
| the struct from `nil_ffi_scope_open` | the runtime | at `nil_ffi_scope_close` | nothing |
| a callback from `nil_callback_new` | the runtime | at `nil_callback_release` | release it |
| a `NilValue` of any kind | the collector | at the next collection | nothing |

**Nothing a C function receives needs freeing.** There is no `nil_free`, and that is
deliberate: the collector owns every `NilValue` and reclaims it at the next sweep,
so freeing one by hand leaves a pointer in the allocation chain that the *next*
sweep follows — a use-after-free that happens later and names nothing. Handing out
a `free` would also mean a second implementation of "what is reachable", free to
disagree with the collector's.

**Nothing a C function returns needs freeing either**, provided it built the answer
with `nil_ffi_buffer`. The asymmetry is the point: a *caller* cannot tell whether it
got a runtime buffer or a `static` one, so it must not free either, while a *callee*
knows exactly what it built and so is the one the rules are written for.

#### Building an answer

```c
void greet(const char *name, size_t length, NilFFIString *out) {
  char *answer = nil_ffi_buffer(7 + length);
  memcpy(answer, "hello, ", 7);
  memcpy(answer + 7, name, length);
  out->bytes = answer;
  out->length = 7 + length;
}
```

`nil_ffi_buffer` exists because the two alternatives are both wrong. `malloc` leaks
— the crossing copies the bytes and nobody is left to free them. A `static` array
can only hold an answer whose size is known when the program was written, and clips
the rest; a clipping answer is the worst kind of wrong, because the program holds a
string, the string is shorter than it should be, and nothing says so.

The bytes are valid until the call returns, which is exactly as long as the crossing
needs them — it copies immediately, before anything else can run.

#### The scope owns the crossing struct too

`nil_ffi_scope_open` returns the `NilFFIString` the call crosses through. The
caller does not allocate it, because **an `alloca` written at a call site lands in
whatever block that call is in** — so a call inside a loop body allocated once per
iteration and the stack grew until it did not. On Windows the linker noticed, by
clang emitting a `__chkstk` probe against an MSVC triple that a mingw linker had no
symbol for; elsewhere it is a stack overflow at whatever iteration exhausts the
stack, on the one input large enough to reach it.

One struct serves the whole call. Arguments are unboxed and read out into SSA values
*before* the call, so by the time the callee fills it for its answer nothing is
still pointing into it.

#### Scopes nest

A C function may call a callback, that callback may be a whole Nilang program, and
that program may make its own string-returning call. Each opens its own scope and
each closes its own. A **stack with a mark**, not a free list: the inner close would
take the outer function's buffer, and the outer function would return a pointer into
freed memory having done everything correctly.

### 9.7 Stability

The surface above is a contract. Anything a C author writes against is frozen, and a
program compiled against one version keeps working against it.

#### What counts as a change

| changes | version |
|---|---|
| adding a function | **yes** |
| changing a signature | **yes** |
| changing what a type crosses as | **yes** |
| changing who owns what | **yes** |
| adding a comment, an error message, or an implementation of an existing promise | no |

The version is a whole number, not a date. A date changes on every addition, which
would make every addition break every compiled object — including the ones that
changed nothing at the boundary.

#### The guard

```c
#define NIL_FFI_ABI_VERSION 1
void nil_ffi_abi_v1(void);     /* link-time guard */
int64_t nil_ffi_version(void); /* what is actually loaded */
```

Generated code calls the guard in **every module that declares an `extern`**, before
anything else runs, and in no module that does not.

The version is in the **symbol's name** so that the linker does the checking.
`nil_ffi_abi_v1` and `nil_ffi_abi_v2` are different symbols, so a v1 object against
a v2-only runtime fails to resolve — there is no "compatible" case for the linker to
guess at. A single symbol both versions define would defeat the whole arrangement.

A guard in every module would guard nothing: a program with no `extern` has no
version to disagree with, and a module linked against a symbol it never calls can
have it satisfied by anything that happens to export it.

#### Asking the layout rather than assuming it

```c
typedef enum NilFFILayout {
  NIL_FFI_LAYOUT_VALUE = 0,    /* NilValue — the tagged word that crosses   */
  NIL_FFI_LAYOUT_OBJECT = 1,   /* NilObject — the header every object has  */
  NIL_FFI_LAYOUT_STRING = 2,   /* NilString                               */
  NIL_FFI_LAYOUT_CLOSURE = 3,  /* NilClosure                              */
  NIL_FFI_LAYOUT_CALLBACK = 4, /* NilFFICallback — what a C side stores    */
  NIL_FFI_LAYOUT_CROSSING = 5  /* NilFFIString — the struct a string uses  */
} NilFFILayout;

int64_t nil_ffi_sizeof(NilFFILayout what);
int64_t nil_ffi_alignof(NilFFILayout what);
```

Both answer **-1** for a selector they do not know. An enum in C is whatever integer
somebody passes, and a default of 0 would be a plausible-looking number for a program
about to allocate with.

These are the *runtime's* types, and the runtime is a shared library the program does
not include the source of. A program that writes `sizeof(NilString)` has assumed the
compiler it is using and the runtime it linked agree — which is a fact to be tested,
not one to be assumed.

**That they agree is a test, not a claim.** Both compilers are asked, from the same
header, in the same link:

```
runtime: 8 8 24 8 32 8 56 8 8 8 16 8
clang:   8 8 24 8 32 8 56 8 8 8 16 8
```

compared as text, because what matters is that they produced *the same numbers*; a
parser would only give a worse message about the same fact. And the numbers are
checked against a written expectation, because a checked constant that silently
changes is worse than none.

Alignment, not size, is the interesting half: a type of the right size and the wrong
alignment still crashes, and only on whichever code happened to be unaligned.

#### Aggregates do not cross by value

A struct or union is never an argument and never a return, in either direction.

A struct's **size and alignment** are agreed — every compiler targeting an ABI
computes them the same way, which is what the table above exists to check. What is
**not** agreed is how a struct return is classified:

| compiler | triple | `{ptr, i64}` returned |
|---|---|---|
| clang | `x86_64-pc-windows-msvc` | in `RAX:RDX` |
| gcc | `x86_64-w64-mingw32` | through a pointer in `RCX` |

So a struct crosses through **memory the caller owns**, which has one meaning
everywhere. That is why `nil_string_unbox` writes through a pointer: the first
version returned a struct, linked cleanly on a machine holding both compilers, and
answered every string with length zero.

### 9.8 Linking

A declaration names a **symbol**; a library says **where it lives**. Neither is
enough alone.

```nil
extern "C" fn sin(x: Float) -> Float;   // says: I need `sin`
```

`sin` is in `libm` on Linux, in the C runtime on Windows, and in `libSystem` on
macOS — three answers from one declaration, and the linker is what knows which. So
the program says what it uses and `nil.json` says where it lives:

```json
{
  "build": {
    "c_sources": ["c"],
    "libraries": ["m"]
  }
}
```

- **`libraries`** are library *names*, to be searched for. `m`, `-lm` and `libm.so`
  all name the same one and all three are accepted, because which spelling somebody
  reaches for depends on the platform they last worked on.
- **`c_sources`** are `.c` files or directories, compiled and linked into the
  program. Most FFI work is neither an existing library nor a runtime call: it is C
  a project writes alongside the Nilang that calls it, and there is no library for
  it until the build makes one.
- **`c_flags`** go to the C compiler and the linker: `-I`, `-D`, `-std=`.

A library that no declaration refers to is **reported**, because a typo in a link
line surfaces much later as "undefined reference to" a symbol nobody can connect
back to the name that was wrong. It is a warning and not an error: a project may
legitimately link something reached through a constructor or another library.

### 9.9 The generated header

```sh
nil header                     # build/<name>.h
nil header --out=bridge.h      # somewhere else
```

A program that calls C has written declarations for functions somebody else
implements. The C side then has to guess: what the parameters really are, whether a
string is one pointer or two, where the answer goes, and who may free what. This
writes those answers down, from the same crossing table the compiler emits its calls
from.

```c
/* greeting.h — generated by `nil header`. Do not edit. */
#ifndef NILANG_GREETING_H
#define NILANG_GREETING_H

#include <stddef.h>
#include <stdint.h>

#include "nilffi.h"

int64_t add(int64_t a, int64_t b);

void greet(const char *name, size_t name_len, NilFFIString *out);
/* A String argument is a (pointer, length) pair. The length is in BYTES, */
/* not characters, and the bytes are NOT NUL-terminated — a string may */
/* contain a NUL, so strlen() would answer the wrong thing. */
/* The answer goes in *out. Fill it with nil_ffi_buffer(n): the runtime */
/* reclaims that when the call returns, so you do not free it and must not. */
/* An answer you already own — a static string, a table entry, or the */
/* pointer you were handed — is handed over unchanged and left alone. */

void register_handler(NilFFICallback handler);

#endif /* NILANG_GREETING_H */
```

**The rules are written next to the prototypes they apply to.** Every one of them is
enforced at run time and produces *no diagnostic* when broken — a string that is not
NUL-terminated, a buffer freed too early, an answer built with `malloc` that nobody
will ever free. A header that omitted them would be a header whose reader writes a
bug and never finds out.

**It is discardable.** Nothing reads it that could not regenerate it, and editing it
is wasted work the next build silently undoes. It is derived, not owned: a header
that had to be kept in step by hand would be a second source of truth, which is the
thing it exists to avoid.

**Every parameter is named**, because a parameter with no name is not a parameter in
C — `void register(NilFFICallback);` declares a *variable* called `NilFFICallback` and
the compiler rejects the whole header with "variable or field 'NilFFICallback'
declared void". A function taking nothing is declared `nothing(void)`, because
`nothing()` in C means "any arguments" and promises something the module does not
deliver.

**A name C could not declare is refused where it is written.** `extern "C" fn
register(handler: fn(Int) -> Int) -> Void;` is a legal Nilang program and not a legal
C function, and the name is the symbol the function is called by, so it cannot be
mangled either. The refusal names the function and says why it cannot be changed —
because the obvious advice, "pick a different name", breaks the link.

### 9.10 Building

```sh
nil build --aot                    # a native executable
nil build --aot --keep-module      # and keep the .ll beside it
```

The `.ll` is the only part of a native build a person can read without a
disassembler, so keeping it is how somebody investigates a wrong answer.

### 9.11 Callbacks

A Nilang function can be handed to C, which stores it and calls it later.

```nil
extern "C" fn register_handler(h: fn(Int) -> Int) -> Void;

fn scale(n: Int) -> Int { return n * 10; }

register_handler(scale);   // C now holds a callback
```

**A callback crosses as a closure object, not a bare pointer.**

Because a function is not all a closure is. `let factor = 3; fn scaled(n) { n * factor }` captures `factor`, and `factor` is a heap value the collector may reclaim. A bare pointer carries nothing about what it needs, so nothing can keep the captures alive and the callback reads freed memory on a call that may not happen for hours.

The closure holds the function pointer and is **pinned** for as long as C holds it. The collector does not move objects (ADR-0041), so the pointer C stores stays valid for as long as the object does — C keeps one word and never tells the collector it is there. What it must not do is let the object be *collected*, and that is what the pin is for.

**The pin ends at `nil_callback_release`, and not before.**

- Releasing early is a use-after-free waiting for a call that has not happened yet.
- Forgetting to release is a leak: it costs memory and the program keeps working.
- Releasing twice, or releasing something that is not a callback, is **ignored rather
  than fatal**. C code holding a pointer may legitimately be unsure, and stopping a
  program over a double release leaves it worse off than continuing.

**A callback's body is ordinary Nilang code and never learns a boundary exists.**

The conversion happens at the C function's own edges, not inside the callback:

```c
int64_t fire(int64_t argument) {
  NilValue boxed = nil_int_box(argument);                        /* C -> Nilang */
  return nil_int_unbox(nil_callback_invoke(h, &boxed, 1));        /* Nilang -> C */
}
```

Both edges convert because a callback invocation is *inside* Nilang: its result is
already a Nilang value, and handing that to a caller expecting an `int64_t` is a
crossing like any other. Passing the argument straight through would give a Nilang
function the number 21 rather than the *tagged value* for 21 — two different words,
and the function would read the low three bits of a number nobody tagged.

### 9.12 How a callback is reached

A compiled Nilang function is `i64 @nil.f(i64, i64)` — arguments in registers, in
order. A closure is invoked as `i64 (*)(NilValue *args, int64_t count)` — an array
and a count. **The two signatures do not match.**

So each function used as a value gets an **adapter** of the closure shape:

```llvm
define i64 @nil.scale.cb(ptr %args, i64 %count) {
entry:
  %a0 = getelementptr i64, ptr %args, i64 0
  %v0 = load i64, ptr %a0, align 8
  %r = call i64 @nil.scale(i64 %v0)
  ret i64 %r
}
```

One per function rather than one shared, because C cannot forward a variable number
of arguments and the only portable way is to have the right number of moves compiled
in. The adapter converts nothing — see §9.11 for why it does not need to.

Only functions actually used as a value get one, so a module with no callbacks is
byte-identical to one written before callbacks existed.

### 9.13 Arity

The function pointer says nothing about its own signature, so **the arity travels
with the callback**.

```c
int64_t arity = nil_callback_arity(callback);   /* or -1, if not a callback */
```

A callback invoked with the wrong number of arguments answers nil and records an
error rather than reading past the end of the caller's argument array — which on most
machines is a silently wrong answer rather than a crash.

The count comes from the function's own parameter list, read where the closure is
built. Anything else would be a second place to update when a signature changes, and
the two would disagree exactly when it mattered.

### 9.14 Using a callback from C

Two forms, and the choice between them is not a matter of taste.

**`nil_callback_invoke(cb, args, count)`** — the portable form. No function-pointer
round trip is involved, so nothing depends on a function pointer being convertible
to an integer, which is not guaranteed by C and is not true on every platform.

**`nil_callback_trampoline`** — the same call as a function pointer C may store and
call later:

```c
NilValue (*trampoline)(NilValue, NilValue *, int64_t) = nil_callback_trampoline;
trampoline(callback, args, count);
```

**A callback crosses as two values, a function pointer and a context, and never as
one.** A C function pointer carries no context and a callback must know *which*
Nilang function it is. Giving it context means either passing it alongside the
pointer — which is this — or generating a fresh piece of machine code per callback so
the pointer alone carries it.

The first is one function and a documented convention. The second is an
executable-memory allocator plus a different one per architecture, and a generated
trampoline that reads a stack slot the compiler got wrong is a crash with no stack
trace pointing at anything anybody wrote.

The trampoline's address is a **compile-time constant**, so C can write
`nil_callback_trampoline` by name and never call anything to ask for it. There is no
function-pointer value anywhere in the ABI, and nothing depends on one surviving a
round trip through an integer.

### 9.15 Calling a function value from Nilang

```nil
fn twice(n: Int) -> Int { return n + n; }
let apply = twice;
apply(21);          // 42
```

A name that is a function is a **value referring to that function**, not a slot to
read. It used to load unconditionally, so this became `load twice` and the emitter
failed with `loading "twice", which has no slot` — a name that is not a variable
being read as one.

Calling such a value goes through `nil_call` with the arguments in a Nilang array.
That costs an allocation and a store per argument on every indirect call, and it buys
**one** calling convention rather than two: a C-side variadic call would be a second
convention for the runtime and the compiler to agree on, and the two would drift the
first time either changed. When indirect calls turn out to matter, `nil_call` is the
one place to make faster, because it is the only place the array is built.

### 9.16 Structs

A `struct` a program declared crosses **as a pointer to a buffer the caller fills**.
One direction only, for a reason given below.

```nil
struct Point { x: Int, y: Int, z: Int }
extern "C" fn distance(p: Point) -> Int;

distance(Point { x: 100, y: 20, z: 3 });   // C receives a const Point_t *
```

The C side sees:

```c
typedef struct Point_t { int64_t x; int64_t y; int64_t z; } Point_t;
int64_t distance(const Point_t *p);
```

**The layout is C's, not ours.** Fields are laid out by C's rules — each field at the
next offset that satisfies its own alignment, and the whole struct's size rounded up to
its own alignment. A `Bool` is an `int`, so a `Bool` after an `Int` leaves seven bytes
of padding:

| struct | size | alignment | offsets |
|---|---|---|---|
| `P { x: Int, y: Int }` | 16 | 8 | `x@0 y@8` |
| `Sparse { a: Bool, x: Int, b: Bool, y: Int }` | 32 | 8 | `a@0 x@8 b@16 y@24` |
| `Named { label: String, count: Int }` | 24 | 8 | `label@0 count@16` |
| `T { a: Byte, b: Byte, c: Byte }` | 3 | 1 | `a@0 b@1 c@2` |

`Sparse`'s fields add up to eighteen bytes and it occupies thirty-two. `T` is three
bytes wide and nothing rounds it up, because no field in it asks for eight-byte
alignment — which is why a layout that assumed every struct ended at an eight-byte
boundary would put `T` inside a containing struct eight bytes late.

The compiler computes the same layout and stores each field at the offset it computed:

```llvm
%buf = call ptr @nil_ffi_buffer(i64 32)
%at  = getelementptr inbounds i8, ptr %buf, i64 8
%v   = call i64 @nil_int_unbox(i64 %field)
store i64 %v, ptr %at, align 8
```

Two things about that are deliberate.

**The offsets are written as bytes, not as a declared LLVM struct type.** A
`%nil.struct.Point = type { i64, i64 }` would have to repeat the padding rule inside
LLVM's type syntax to agree with C, and a struct type whose declared layout disagrees
with the stores into it is a disagreement nobody would ever see. `getelementptr i8`
with a literal offset needs no type at all and records the offset actually used.

**The buffer belongs to the call's scope.** It comes from `nil_ffi_buffer`, the same
scope a string answer is built in, and it is released when the call returns. An
`alloca` at the call site would land in whatever block the call is in, so a call inside
a loop body allocated once per iteration and the stack grew until it did not. A string
*field* unboxes through that same scope, so there is one lifetime and one place that
releases.

**A `String` field is two C members.** The pointer and the length, eight bytes apart,
which is what `nil_string_unbox` writes:

```c
typedef struct Named_t { const char *label; size_t label_len; } Named_t;
```

The `_len` suffix is a name the reader would otherwise have to guess, and a prototype
whose struct has a member the header never mentions is a header C cannot compile.

**The generated header declares the struct.** From the same layout the module's stores
were written from, so a C file that includes the header gets the struct the program
actually has. Writing it by hand gets the right thing by luck when the fields happen to
be in order, and a buffer C reads at the wrong offsets — silently, for as long as the
program runs — when they are not. The name is `Point_t` with a `_t` suffix so that
including the header cannot collide with a type the reader already had.

**A struct with a field that has no crossing is refused**, by the field:

```
struct Point { x: Int, items: List<Int> }
```

Laying out the fields that do cross would leave a hole where the list belongs, and C
would read the following field's bytes as its length — the same shape of mistake as a
struct returned by value, reached by a different road.

### 9.17 A struct cannot come back

A struct *return* is refused, and the refusal says so by name:

```
extern "origin" returns Point, and a struct cannot cross back yet.
A struct goes *into* C as a pointer to a buffer the caller fills, and that is one
direction only. Nothing builds a Nilang struct from C-owned bytes, so answering with
a pointer would hand the program a tagged word pointing at memory it does not own.
```

That is the whole of the reasoning, and it is an ownership argument rather than a
difficulty one. A struct going *in* is a buffer the compiler can fill from fields it
can read. A struct coming *back* would need the runtime to build a Nilang struct out of
C's bytes, and nothing does that. Answering with a pointer would return a tagged word
pointing at C-owned memory — a value the program's GC does not own and cannot trace,
which is the exact failure this whole boundary exists to prevent.

Both halves of the message are load-bearing. The first says what is missing; the second
says what works, because "structs do not cross" would send a reader looking for a
restriction that does not exist.

### 9.18 Platform channels

A `Channel` crosses as a message queue, not as a value. The host writes messages, the
guest serves them, and neither touches the other's memory.

```nil
struct Named { label: String, count: Int }
extern "C" fn total(n: Named) -> Int;
```

That is the FFI's contract (§9.1). The platform channel (§8.4, `docs/spec/MODULE_SYSTEM.md` §3.5) is the *other* direction: a host sends a message (`nil_channel_post`), a guest receives (`nil_channel_serve`), and the message carries whatever the host chose — a tag byte, then the payload's own bytes (see `runtime/ffi/src/channel.c`). A host does not build a `NilValue`; the encoding builds itself from whatever the host holds. A guest that wants a value calls `nil_decode`, and a value that is not a type the channel understands is refused rather than misread.

The service loop (`nil_channel_serve`) holds one reference, and the close (`nil_channel_close`) frees the queue. A stale handle — one from a closed slot whose generation changed — is refused by arithmetic, which is the same design as a struct parameter: the shape is in the data, not in an assumption about it.
