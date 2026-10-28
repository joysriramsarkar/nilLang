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

4. **Hash Literals**: Entries of `{ ... }` are evaluated strictly left to right in
   the order written, key then value, for each entry in turn. The AST records
   that order in `HashLiteral.OrderedPairs`, and every consumer — the
   tree-walking evaluator, the bytecode compiler, the HIR lowerer, and the
   formatter — iterates it rather than the underlying map. Where a key repeats,
   the last written entry wins. A hash node assembled programmatically without
   an order slice is evaluated in sorted printed-key order so that behaviour
   stays deterministic.

   This rule was previously unsatisfied: entries lived only in a Go map, so
   evaluation order followed Go's randomised per-map iteration seed and two runs
   of the same program could differ. `TestHashLiteralEvaluatesInSourceOrder` and
   `TestHashLiteralOrderIsStableAcrossRuns` in `tests/determinism_test.go` are the
   conformance evidence.

5. **Observed order is insertion order.** Wherever a hash is observed rather than
   only looked up — printed, iterated with `for k, v in h`, or serialised — the
   entries appear in the order they were written.

   Printing once sorted the *rendered text*, which is a different thing: sorting
   the strings reorders keys and values independently, so `{ b: 1, a: 2 }`
   printed as `{ a: 2, b: 1 }`. A struct's printed form therefore disagreed with
   the order its fields were declared, which is the one order a reader has to be
   able to rely on. The VM built its hashes by writing to the map directly and so
   had no order to preserve at all; both now go through `SetPair`, and `Inspect`
   iterates `OrderedPairs`.

   **A hash written after the fact is the same rule.** `m["l"] = 1; m["k"] = 2;`
   writes two keys, so `m` observes as `l, k`. This was not true of the
   interpreters, which bypassed `SetPair` on the assignment path and so had no
   order to preserve; and it was not true of the compiled engine at all, because
   the runtime printed a map in its table's hash order. Insertion order is used
   everywhere now, for a hash literal and for an assignment alike — both are
   deterministic, so the sorted fallback a hash *literal* still has for nodes
   built programmatically is no longer needed on either path.

   Conformance evidence: `TestStructDeclarationAndFieldAccess/field_order_follows_the_declaration`
   and `TestHashLiteralOrderIsStableAcrossRuns`, both across engines, plus
   `ThreeEnginesAgree/Struct Keeps Declaration Order`,
   `.../Map Keeps Written Order Through A Rehash` and
   `.../Rewriting A Key Keeps Its Place`.

---

## 2. Binding, Mutation & Assignment Semantics

### 2.1 Lexical Bindings
Bindings are introduced into the current lexical scope exclusively via explicit declarations:
- `let x = expr;` introduces a mutable lexical binding initialized to the result of `expr`.
- `let x: Type = expr;` introduces a typed mutable binding.
- `const c = expr;` introduces an immutable lexical binding.
- `fn name(params) { ... }` introduces a function binding.
- Function formal parameters introduce bindings in the function's local scope upon call.
- A `for` loop introduces its loop variables in the loop's own scope (SEMANTICS §4.2).

**A declaration may shadow a builtin.** `let len = 5;` introduces an ordinary
binding; it does not refer to the `len` builtin, and the builtin remains reachable
from every scope that has not shadowed it.

This is worth stating because the natural implementation — a symbol table that
returns an already-present entry on redefinition — makes shadowing silently fail:
the declaration writes to one slot while every read goes to the builtin. The
symbol table therefore creates a fresh binding when the existing one is a
builtin, and reuses it otherwise.

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


### 2.3 Assigning into a container

Assignment is not only to a binding. `a[1] = 9` and `m.name = "nilang"` are assignments, and both write **into the container** rather than rebinding it:

```nil
let a = [1, 2, 3];
a[1] = 9;
a;              // [1, 9, 3] - a is still the list, and 9 is inside it
```

**Rules:**

1. The container expression is evaluated **once**, before the value.
2. The write mutates the container the expression named. It does not replace a binding, so a program holding two references to one list sees the change through both.
3. The assignment is an **expression**: it evaluates to the value written. `y = (a[0] = 5);` is legal and binds `y` to `5`.
4. The parser normalises `m.name = v` into a subscript whose key is the string `"name"`. The two spellings are therefore the same operation by the time any backend sees them.

**Reading past the end is `null`; writing past the end is an error.**

```nil
let a = [1, 2, 3];
a[10];          // null - "what is at position 10?" has an answer
a[10] = 9;      // error: index out of bounds - there is no slot to write
```

The asymmetry is deliberate. A *read* is a question, and for a three-element list the honest answer is "nothing there" - which is what `null` already means. It is also what a missing map key answers, so a lookup that may miss needs no check either way:

```nil
let m = {"a": 1};
m["a"] == null;    // false
m["zz"] == null;   // true  - the ordinary way to ask whether a key was present
```

A *write* is different in kind: there is no slot, so the only two honest outcomes are refusing or growing the list. Growing silently would change the length of something the program can also read, and refusing says plainly that the program is wrong - which it is.

A missing map key reads as `null` for the same reason. A member that does not exist is a **type error** rather than `null`, because `null` is indistinguishable from a member that *holds* `null`, so answering it would let a program carry on producing wrong answers instead of stopping.

### 2.4 What a block evaluates to

A block's value is **its last statement's value, whatever kind that statement is**:

```nil
fn pick(c: Bool) -> Int { if c { 1 } else { 2 } }   // 1 or 2
let x = { 5 + 5 };                                  // 10
```

A `let` produces `null`, so a block ending in a declaration evaluates to `null` rather than to whatever it was written after. An `if` whose arms both `return` has no value at all, because no execution reaches past it.

A program behaves the same way: its value is the value of its last top-level expression statement.
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
  if (true) {
      let x = 20; // Shadows outer x within this block
      x + 5;      // 25
  }
  x;              // 10 (outer binding unchanged)
  ```
  (A bare `{ ... }` block is not a statement; the example uses `if` so it runs
  as written. All three legs — checker, evaluator, VM — agree: conformance
  evidence is `TestScopeCorpusAgreesAcrossEngines` in `tests/scope_corpus_test.go`.)

- An `app` body is deliberately transparent rather than scoped: its bindings
  are program-level (`app H { let ax = 9; } ax;` is 9 on every engine).

### 3.2 Closure Capture
Functions capture identifiers referenced from enclosing scopes (free variables).
Both engines implement read-only capture, nearest-scope resolution, and
**mutable by-reference capture**.

A captured variable is a shared cell, not a copy. The enclosing frame and every
closure that captures it read and write the same cell, so a mutation made inside
a closure is visible to the parent and to sibling closures.

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

Each call to `makeCounter` creates a fresh `count` cell, so two counters
constructed separately do not share state.

**VM implementation.** The stack VM boxes a captured local *in place*: the local
slot stops holding the value and holds an `OpCaptureCell`, and `OpGetLocal` /
`OpSetLocal` read and write through it automatically. Boxing is idempotent, so
capturing the same local from several nested closures does not split the cell.
Globals need no boxing, since one shared array holds them for every frame.

This previously diverged: the VM wrapped a captured value in a cell that only the
closure could see, so the counter example returned `0` from the VM and `1` from
the evaluator. Conformance evidence is
`TestMutableCaptureAgreesAcrossEngines` in `tests/closure_capture_test.go`.
- Captured bindings survive the lifetime of the stack frame in which they were originally declared.
- Mutations made to a captured binding from within a closure are visible to all closures sharing that binding context.

---

## 4. Control Flow Semantics

### 4.1 Conditional Execution (`if` / `else`)
- The condition expression is evaluated.
- Truthiness: `true` is truthy; `false` and `null` are falsy. Integers other than 0 and non-empty strings are considered truthy in dynamic contexts, but static type checking requires condition to evaluate to `Bool`.
- Only the selected branch executes; the alternative is never evaluated.

### 4.2 Loops (`while` and `for`)

**`while (cond) { body }`** — evaluates `cond`; while truthy, executes `body` and
repeats. `body` must be a block.

**`for <var> in <iterable> { body }`** — iterates in one of three forms:

```nil
for i in 0..10        // exclusive range: i from 0 to 9
for i in 0..=10       // inclusive range: i from 0 to 10
for item in items     // each element, in index order
for idx, item in items // index and element together
```

Rules:

1. **The iterable is materialised once**, before the loop begins. Mutating the
   collection inside the body cannot change the iteration, which is what stops a
   `for` from skipping or repeating elements when the body appends to the array
   being walked.
2. **Loop variables are scoped to the body.** They are declared by the loop, are
   invisible after it, and get a fresh binding on every pass. A closure created
   inside the body therefore captures that iteration's value.
3. **Array and string iteration is by index, ascending**, 0-based.
4. **String iteration yields characters**, counted in runes rather than bytes, so
   Bengali text advances one character at a time.
5. **Hash iteration follows insertion order**, so `for k, v in h` is reproducible
   across runs. See §1 rule 4.
6. **A range costs nothing regardless of its size.** `for i in 0..1000000000`
   allocates no array; a cursor walks between the bounds.
7. **A descending range counts down.** `for i in 10..0` runs 10, 9, …, 1, because
   that is what a developer means, and a `while` with a negated step is strictly
   more work to read.
8. **Iterating an empty collection runs zero times.** Iterating `null` also runs
   zero times rather than failing: an absent optional is an empty walk.

**`break`** exits the innermost enclosing loop immediately.

**`continue`** starts the next iteration of the innermost enclosing loop, skipping
the rest of the body.

A `break` or `continue` with no enclosing loop is a compile-time error
(`E0005`). A function boundary resets the loop scope, so `break` inside a function
declared within a loop is also an error: a call is a jump the loop cannot see
across.

**Nested loops are independent.** A `break` in an inner loop leaves the outer one
running.

#### Implementation

`for` and ranges have no bytecode of their own. A lowering pass
(`compiler/lower`) rewrites them into `while` loops before any backend sees the
program, which is what makes the two engines agree structurally rather than by
duplicated work. Two details of that lowering are load bearing:

- **The cursor advances before the body.** `continue` unwinds by stopping the
  enclosing block, so anything after it in that block would never run. Advancing
  first means a `continue` skips only the rest of the body, which is its meaning.
  Placing the increment last, where it reads more naturally, would make
  `continue` an infinite loop.
- **`break` and `continue` are not lowered away.** They become jumps in the VM and
  signal values in the evaluator. The only way to lower them would be a flag plus
  conditional jumps, which is more code for identical behaviour.
- **Each pass declares its loop bindings fresh.** The evaluator gives the body a
  new environment per pass; the VM stores `let`/`state` declarations in a loop
  body with fresh-cell opcodes (`OpSetLocalFresh` inside functions,
  `OpSetGlobalFresh` at top level) and shares the current pass's cell at
  closure creation (`OpBoxGlobal` for top-level bindings, `OpBoxLocal` for
  locals). Passes without any capture pay one type test and no allocation.

### 4.3 `while let`

```nil
while let Some(x) = items[i] { total = total + x; i = i + 1; }
```

1. **The pattern is the `match` grammar.** `while let` accepts exactly what a
   `match` arm accepts, so a pattern that compiles in one compiles in the other.
   There is no second pattern language to keep in step.
2. **The loop repeats while the pattern matches.** A non-matching value ends it,
   with no error: an exhausted optional or a failed `Result` is the ordinary way
   out, not a fault.
3. **Bound names are scoped to one pass.** They are introduced as ordinary `let`
   bindings at the top of the body, so they are fresh each iteration and a closure
   created in the body captures them the way a developer expects.
4. **`break` and `continue` work as they do in any other loop.**
5. **The value is re-evaluated on every pass.** This is what makes the form
   useful: `while let Some(x) = items[i]` walks a collection because the body
   advances `i` and the next pass therefore reads a different element. Binding the
   value once before the loop would read `items[0]` forever and never terminate.
   Rust's `while let` behaves the same way.

   The cost is that a value with side effects is evaluated once for the test and
   again for each binding the pattern introduces — two or three times per pass
   rather than once. That is the price of not needing a separate pop-style
   operation to advance a cursor, which the language does not have. A value whose
   evaluation is expensive or observable should be bound to a variable first.

**`while let` is lowering, not a new instruction.** It becomes a `while` loop
whose test is the pattern match and whose body starts with the bindings, using the
same pattern builder a `match` arm uses. One implementation means the engines
cannot disagree about it.

---

## 4A. Algebraic Data Types

### 4A.1 Structs

```nil
struct User { name: String, age: Int }
let u = User { name: "Ali", age: 30 };
u.age          // 30
```

1. **A struct is a type, not a value.** The declaration has no run-time effect.
2. **Fields are checked statically.** An unknown field is `E0503`, a missing one is
   `E0506`, and a field given the wrong type is `E0101`. A struct value always has
   every field: defaulting one would invent data the developer did not write, and
   leaving it unset would fail later, further from the mistake.
3. **Field order is declaration order**, and it is preserved wherever the value is
   observed — printing, iteration, serialisation.
4. **Field access is `u.name`, and `u["name"]` also works.** A struct value is a
   keyed collection at run time, so the index form is available. This is a
   documented permissiveness, not an oversight: a tagged runtime representation
   would need a distinct object type, a new opcode, and special cases in every
   operation that walks a collection, to prevent a form of access that costs
   nothing and cannot fail. Struct identity — "is this value a `User`?" — is
   enforced statically, where it is unambiguous.
5. **A field may hold a struct**, including one of its own type only through an
   array or hash, since the language has no recursion in value position.

### 4A.2 Enums

```nil
enum Shape { Circle(Int), Square }
let a = Circle(3);      // a variant with a payload
let b = Square;         // a variant without one
```

1. **A variant carries at most one payload.** `Circle(Int)` is enough for the
   shapes a result type actually takes; a general algebraic data type is not
   offered.
2. **Variant names share one namespace across all enums**, so `match` can say
   `Circle(r)` without naming the enum. Declaring the same name twice is `E0501`.
3. **A payload-free variant is written bare**: `Square`, not `Square()`. The
   parentheses would carry nothing.
4. **A variant name binds as a value.** `let s = Square;` produces the variant.
5. **A declared enum shares the value namespace with functions.** `Circle(7)` is
   the variant, because `Circle` is declared. A function cannot be given a name an
   enum already uses.

### 4A.3 `match`

```nil
let label = match value {
  0 => "zero",
  1 | 2 | 3 => "small",
  n if n < 0 => "negative",
  Circle(r) => "circle of \(r)",
  Square => "square",
  other => "other",
};
```

`match` is an **expression**: every arm produces a value, and the whole produces
the arm that applied.

**Patterns**

| Form | Meaning |
| --- | --- |
| `_` | matches anything |
| `name` | matches anything and binds it to `name` |
| `name` where `name` is a declared variant | matches that variant |
| `Variant(binding)` | matches the variant and binds its payload |
| `Variant(_)` | matches the variant and discards its payload |
| `Name { field: pattern, ... }` | a struct pattern; listed fields must match |
| `literal` | matches an equal integer, float, string, bool, or `null` |
| `p1 \| p2` | matches either alternative |

A bare identifier is a binding **unless** it names a declared variant, in which
case it is that variant. The alternative would leave `Circle => …` binding a value
to a variable named `Circle` while the only way to name a variant in a pattern is
`Circle()`. Rust, Swift, and Kotlin read it the same way.

**Guards** — `pattern if condition => body` — may use the pattern's bindings. A
guard's failure falls through to the next arm, and an arm whose guard fails is
the same as an arm that did not match.

**Or-patterns** are expanded into separate arms, because the alternatives can
bind different names and one arm cannot bind what only some of them introduce.

**Struct patterns list the fields they care about.** Fields not listed are
ignored; the type checker is what notices a pattern that forgot a field it should
have constrained.

**Built-in cases.** `Ok`, `Err`, `Some`, and `None` are matched as variants,
because the values they produce already have cases:

```nil
match result { Ok(v) => use(v), Err(e) => report(e) }
```

`Err(e)` binds the error through `errValue`, which exists for the same reason:
`Ok`, `Err`, `isOk`, and `isErr` were all present, and nothing could get the error
out.

**Exhaustiveness.** When the subject's type is known and its cases are closed — an
enum, `Bool`, `Result`, or `Optional` — every case must be covered, or the program
is rejected with `E0507`. A wildcard or catch-all binding arm satisfies this
without listing the cases. Exhaustiveness is not decidable for any other type, so
no check is made; a non-exhaustive match over `Any` stops at run time through the
`unreachable` call the lowering emits.

**Bindings are scoped to their arm**, are invisible afterwards, and are fresh per
arm. A closure inside an arm captures the binding normally.

### 4A.4 Implementation

None of these constructs has a bytecode of its own. A lowering pass
(`compiler/lower`) rewrites them before any backend sees the program:

- A struct literal becomes a hash literal, so field access costs nothing extra.
- An enum value becomes a hash with a `variant` key and, when present, a
  `payload` key.
- A `match` becomes a let-bound subject followed by a nested `if`/`else` chain
  whose innermost `else` is `unreachable(...)`.

The chain needs no "already matched" flag, because a nested `if`/`else` stops at
the first branch taken, which is precisely match's semantics. Desugaring rather
than adding opcodes keeps the two engines in agreement structurally: there is one
implementation of each construct, so there is nothing to keep in step.

The subject is bound to a hidden local rather than inlined into every arm's test,
so a subject with a side effect is evaluated exactly once.

`while let` is the one exception: its value is inlined, because re-evaluating it
each pass is what lets the body advance a cursor (§4.3 rule 5).

---

## 4B. Module Visibility

```nil
// math.nil
export fn area(r: Int) -> Int { return r * r * 3; }   // public
private fn validate(r: Int) -> Int { return r; }      // internal
fn helper() -> Int { return 1; }                      // internal
```

```nil
import { area } from "./math.nil";     // works
import { helper } from "./math.nil";    // refused
```

1. **A module's interface is what it exports.** Everything else is internal:
   reachable from inside the module, unreachable from outside it.
2. **Importing a name the module does not export is an error at the import**,
   not a null bound silently and failed later. The distinction a developer needs
   is "this module does not have that", not "this module has that and it is null".
   The evaluator reports it at run time and the bytecode compiler reports it at
   compile time (`E0303`); both refuse, which is the obligation.
3. **Every missing name is reported at once.** Failing on the first turns one edit
   into two round trips for a mistake the whole list already answers.
4. **`private` is the explicit form of leaving a name out.** It says the same thing
   as omitting `export` in a file that uses it, and exists so the intent is visible
   at the declaration rather than inferred from an absence.
5. **A visibility marker changes what leaves the module, not what the declaration
   does inside it.** Marking a declaration is transparent to evaluation,
   compilation, lowering, and type checking.
6. **Both forms of import honour the rule.** `import { a } from "./m.nil"` binds
   named exports; `import "./m.nil" as m` builds a value containing exactly the
   exports.

### 4B.1 The migration rule

**A file that contains no `export` at all exports everything.** A file that does
use `export` exports exactly what it marked.

This is not the rule a language would choose from the start, and it is worth being
explicit about why it is the one chosen here. The alternative — an unmarked
declaration is private — breaks every module that has ever been written, all of
which predate the keyword. A module opts into strict visibility by writing a
single `export`; from that point on, anything unmarked is private.

The rule is observable in both directions and both are tested, so it cannot drift
into something subtler by accident.

### 4B.2 Imported modules are lowered

A module is executed, so it is lowered exactly as a top-level program is: `for`,
`match`, `struct`, `enum`, and `while let` are rewritten before any backend sees
it.

This was previously unsatisfied for both engines. Each loaded the module's *source*
program, so a module using any lowered construct reached the evaluator or the
bytecode compiler as a node they refuse — reported as an internal error. The
importing program was lowered and the module was not, which is a difference with
no defensible reason behind it.

Conformance evidence: `TestExportIsTransparentAtTheDeclaration/struct` and
`TestWholeModuleImportExposesExportedNames`, both across engines, and
`TestFunction Modifier Keywords Are Soft` for the modifiers.

---

## 4C. Traits & Implementations

```nil
trait Greeter {
    fn greet(self, loud: String) -> String
}

struct Dog { name: String }
struct Cat { name: String }

impl Greeter for Dog {
    fn greet(self, loud: String) -> String { return loud + " Woof"; }
}

impl Greeter for Cat {
    fn greet(self, loud: String) -> String { return loud + " Meow"; }
}

Dog { name: "Rex" }.greet("Hi")   // "Hi Woof"
```

1. **A trait declares what an implementation owes.** It holds signatures and no
   bodies: a body in the declaration would be a second place to read and no way to
   know which one an implementation actually ran.
2. **An impl must provide every method its trait declares, with the same
   arity** (`E0601`), and may not add one the trait does not declare (`E0602`). An
   impl that can do more than the trait promises is not an implementation of that
   trait, and the extra method would be unreachable through it.
3. **A type has at most one impl** (`E0602`). Two would make "which
   implementation runs" unanswerable except by declaration order, which makes the
   answer depend on where a line was written.
4. **A method call resolves against the receiver's static type.** If that type has
   no impl, or an impl without that method, the call is `E0603` — a diagnostic at
   the call, not a failure further on.
5. **The receiver is an ordinary first parameter**, written `self` by convention.
   It is not removed and passed some other way, so the generated function is
   callable and checkable like any other. `self` is a soft keyword: it is a name,
   not a reserved word.
6. **A declaration may precede its use.** A trait may be written after the impl
   that implements it, matching the forward-reference rule the rest of the type
   system already follows.
7. **A method body is checked like a function body.** Its parameters, return type,
   and everything it touches go through the same checks — including method calls
   on `self`, which resolve because `self` is annotated with the implemented type.
   A method must not end up with weaker checking than a free function.

### 4C.1 There is no method table

An `impl` introduces no dispatch table, no vtable, and no per-object type tag. Each
method becomes an ordinary top-level function under a name a developer cannot
write, and a method call is rewritten to call it with the receiver as the first
argument.

That is a lowering, and for the same reason `for` and `match` are (ADR-0007): one
implementation means the two engines run the same code and cannot disagree about
what a method does. It also means there is no run-time cost.

The rewrite needs to know the receiver's type, which only the checker knows. So the
checker records the resolution on the side and the lowering pass reads it. Writing
the answer into the tree instead would leave a formatter rendering the generated
name, since a formatter reads the same program.

### 4C.2 What is not here

Static methods (`Type.new()`) and generic traits. A static method needs a
namespace form the language has no syntax for, and generics need a type system
that can constrain them; both are L2 work.

Conformance evidence: `tests/traits_test.go` and the `Trait Method Call`,
`Two Types One Trait`, `Method Uses Self`, `Method Calls Method`,
`Method Over A Loop Variable`, and `Self Is An Ordinary Name` cases in
`tests/conformance`, all across engines.

### 4C.3 Loop bodies are checked

A `for` body is type-checked like a `while` body: the iterable, the loop variable,
and every statement inside.

This was previously unsatisfied for `for` alone. `for` has no bytecode of its own
— it is lowered into a `while` — and the checker ran before lowering, so it saw a
statement type it had no case for and fell through to the default. Every statement
inside a `for` body went unchecked: an undefined identifier, a call with the wrong
arity, a method on a value that has no impl. All of it reached the backend and
failed there, or worse, did not fail.

The loop variable's type comes from the iterable's element type, which is what
makes `for box in boxes { box.area() }` resolve to the impl for `Box`. Where the
element type is not known the variable is `Any`, as everywhere else the checker has
nothing to go on — and `Any` still catches an undefined name or the wrong number of
arguments, which is most of what a body can get wrong.

---

## 4D. Optional Access & Null Coalescing

```nil
let name = user?.profile.name ?? "anonymous"
```

1. **`a ?? b` is `a` unless `a` is null, then `b`.** A present but falsy value is
   present: `0 ?? 9` is `0` and `"" ?? "d"` is `""`. Only null takes the fallback.
2. **`a?.b` is `b` on `a`, or null when `a` is null.** It is not a test that can
   fail; the null is the answer.
3. **The receiver is evaluated once.** Both operators bind it to a hidden local
   before branching. A naive `if (a == null) { null } else { a.b }` evaluates `a`
   twice, which is wrong for a receiver with a side effect and wasteful for an
   expensive one — and it is the same reason a `match` subject is bound.
4. **`?.` guards the whole chain, not the step it appears on.** In `o?.i.v`, a null
   `o` yields null for the entire expression. Guarding only `o.i` would read `.v`
   off a null, which is the failure this rule exists to prevent.
5. **The fallback is evaluated only when it is needed.** `findUser(id) ??
   makeDefault()` must not build a default user when there is already one.
6. **`a ?? b ?? c` groups left to right.** The fallback of the first is the rest of
   the chain, so no parentheses are needed.
7. **`??` does not swallow the ternary.** `a ?? b ? c : d` reads as
   `a ?? (b ? c : d)`. The lexer produces `??` as one token precisely so this
   ambiguity does not reach the parser.
8. **`d?.greet()` resolves a trait method** exactly as `d.greet()` does. What
   changes when the receiver is null is a run-time question the lowering answers;
   it is not a reason to skip the check.

### 4D.1 There is no new opcode

Both operators lower to `if`/`else` with a bound receiver:

```
a ?? b        →  let r = a;      if (isNull(r)) { b }   else { r }
a?.b          →  let r = a;      if (isNull(r)) { null } else { r.b }
a?.b(args)    →  let r = a;      if (isNull(r)) { null } else { r.b(args) }
o?.i.v        →  let r = o;      if (isNull(r)) { null } else { r.i.v }
```

For the same reason `for` and `match` are (ADR-0007): one implementation means the
two engines run the same code and cannot disagree about what an operator means.

`a?.b(args)` is lowered as one construct rather than a callee followed by a call.
Lowering the callee would produce `null` for the empty case and then call it —
which is the opposite of what the operator is for.

`isNull` is therefore a builtin both engines need, and it is exported for the same
reason `isSome` is: a developer writing `x ?? d` should be able to write
`isNull(x)` when they want the question on its own.

Conformance evidence: `tests/optionals_test.go` and the ten `Coalesce`/`Safe
Access`/`IsNull` cases in `tests/conformance`, all across engines.

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
- `Int` operations wrap on two's complement 64-bit boundaries. Both the Evaluator and the VM wrap identically; there is currently no separate debug mode and no `IntegerOverflow` error. (A checked-arithmetic mode may be added in a future version; until then, wrapping is the defined behaviour in all modes.)


### 5.3 Equality between different kinds

`==` and `!=` are defined for every pair of values. The rule depends on what is on each side, and the order below is the order of the cases:

| Operands | Answer |
|---|---|
| two numbers | compared numerically, a float on either side promoting both |
| two strings | compared as bytes |
| **a string and anything else** | **both rendered to text, and the text compared** |
| two booleans | equality only |
| **anything and `null`** | **`==` is true only when both are `null`** |
| two collections | identity |
| anything else | type error |

The two bold rows are the ones that read as surprising, and both are the same decision.

**A string beside a non-string compares rendered text.** So `"1" == 1` is true, and so is `1 == "1"`, `"2.5" == 2.5` and `"true" == true`. This is consistent and checkable once stated: a program that reads a number out of a record and compares it with a literal it wrote is comparing text, and text is what it has. Requiring the types to match instead would make every such program need a conversion it does not have.

**`null` answers equality against anything.** `null == null` is true; `null == 1` is false; `null != 1` is true.

This is not a special case invented for convenience — it is what falls out of the rule that a pair matching no other case is compared by identity, and two nulls are the same pointer. It has to hold, because **every lookup in the language can answer `null`**: a missing map key, a read past the end of a list, a function returning nothing. `x == null` is therefore the ordinary way to ask whether a lookup found anything, and an engine that made it a type error would make the most common test in the language unanswerable.

**Two booleans answer equality and nothing else.** `true < false` is not an ordering of anything: the language would have to say which of the two booleans is smaller, and it declines to.

**Comparison answers a Boolean, and a Boolean is not `0` or `1`.** In the runtime's representation the tag of `false` is `1` and the tag of `true` is `2`, so a raw `1` reaching a program from a comparison is not `true` — it is `false`. Every comparison therefore returns a properly built Boolean rather than an integer, and the runtime functions that answer one are declared to return `int64_t` rather than `int` so that passing the answer on as a value is a type error rather than a wrong result.

### 5.4 Where these rules are decided

The rules above live in **one** place per layer, and that placement is deliberate:

- the **interpreter** defines them, because the interpreter is the language's specification;
- the **VM** implements the same rules, and the three-engine gate checks it did;
- a **compiled** program does not implement them at all — it emits a call, and the **runtime** answers.

The runtime is the only component that can see what it is holding. At MIR level `m["k"]` and `xs[1]` are one instruction, so a compiler that chose between `nil_map_get` and `nil_array_get` would be guessing; the same holds for `+` on two integers versus two strings. See LANGUAGE_SPEC §6.6.

---

## 6. Conformance Obligations

The Evaluator and Stack VM must produce equivalent typed values or runtime
error categories for:
1. All arithmetic operations and division exceptions.
2. Short-circuit side effects.
3. Closure variable capture and mutation.
4. Scope shadowing boundaries.
5. Exit codes and standard output.
6. Hash literal evaluation order, including duplicate keys, and observed order (§1 rule 4).
7. `for` loops in all three forms, ranges, `break`, and `continue` (§4.2).
8. `while let` over every pattern form, with `break` and `continue` (§4.3).
9. `struct` construction and field access, `enum` variants, and every `match`
   pattern form, including guards and or-patterns (§4A).
10. Truthiness, `!`, and `==` over values produced by a builtin, in both engines
    (ERROR_MODEL §4.6).
11. Stopping on a failing builtin, at the top level and inside a loop
    (ERROR_MODEL §4.5).
12. Which names a module exports, and the refusal to import one it does not
    (§4B). A visibility rule enforced by one engine is not enforced.
13. Trait method calls, in every receiver shape: a value, a literal, a field, and
    a loop variable (§4C). A method reaches the engines through the ordinary
    call path, so a divergence here means the lowering does not do what it claims.
14. `??` and `?.` in every shape: chain, field, call, and whole-chain guard (§4D).
    Both lower to `if`/`else`, so a divergence here would mean the two engines
    disagree about what an operator means — which is the failure mode this
    architecture exists to prevent.

Determinism is a separate obligation that applies within a single engine as
well as across engines: evaluating the same program twice must produce the same
result. Conformance evidence lives in `tests/conformance` (cross-engine
equivalence) and `tests/determinism_test.go` (repeat-run stability).

Optimized VM and WASM become subject to this obligation only when they are
added to the executable conformance harness.
