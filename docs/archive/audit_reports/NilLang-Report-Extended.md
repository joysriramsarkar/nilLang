# NilLang – Extended Technical Report (Volume II)

This volume complements `NilLang-Report.md` with the following additional
detail: formal grammar, operator table, complete standard-library API,
compiler internals, memory model, tutorial examples, and deployment guides.
Sections are numbered continuously with Volume I for cross-reference.

---

## 15. Formal Grammar (EBNF)

```
program        ::= decl (NEWLINE decl)*
decl           ::= func_decl | struct_decl | trait_decl | const_decl | import_decl
func_decl      ::= 'func' IDENT '(' params? ')' ('->' type)? block
params         ::= param (',' param)*
param          ::= IDENT ':' type
struct_decl    ::= 'struct' IDENT (':' trait_list)? '{' field* '}'
field          ::= IDENT ':' type
trait_decl     ::= 'trait' IDENT '{' trait_item* '}'
const_decl     ::= 'const' IDENT ':' type '=' expr
import_decl    ::= 'import' path ('as' IDENT)?
block          ::= '{' stmt* '}'
stmt           ::= let_stmt | assign | if_stmt | for_stmt | while_stmt | return_stmt | expr
let_stmt       ::= 'let' IDENT (':' type)? '=' expr
assign         ::= expr '=' expr
if_stmt        ::= 'if' expr block ('elif' expr block)* ('else' block)?
for_stmt       ::= 'for' IDENT 'in' expr '..' expr block
while_stmt     ::= 'while' expr block
return_stmt    ::= 'return' expr?
type           ::= primitive | IDENT | type '[' INT ']' | 'List' '<' type '>'
primitive      ::= 'int' | 'float' | 'bool' | 'str' | 'char' | 'nil'
expr           ::= ...
```

The definitive, Internet‑draft‑style grammar file is `spec/grammar.ebnf`; it is
used both by the `nlc` bootstrap tokenizer and by the regression tests in
`tests/grammar/*`.

---

## 16. Operator Precedence and Associativity

| Precedence | Operator(s) | Associativity | Meaning |
|-----------|------------|---------------|---------|
| 1 | `?` | Postfix | Nil-safe dereference |
| 2 | `* / %` | Left | Multiplicative |
| 3 | `+ -` | Left | Additive |
| 4 | `<< >>` | Left | Bitwise shift |
| 5 | `&` | Left | Bitwise AND |
| 6 | `^` | Left | Bitwise XOR |
| 7 | `\|` | Left | Bitwise OR |
| 8 | `..` | Left | Range constructor |
| 9 | `< <= > >=` | Left | Relational |
| 10 | `== !=` | Left | Equality |
| 11 | `\tand` (`&&`) | Left | Logical AND |
| 12 | `\tor` (`\|\|`) | Left | Logical OR |
| 13 | `=` | Right | Assignment |

All binary operators can be overloaded via the `Ops` trait, with the exception
of the short‑circuiting logical operators and assignment.

---

## 17. Complete Standard Library API

### 17.1 Module `io`

* `print(...args: Any)` – formatted output to stdout.
* `input() -> str` – reads a line from stdin.
* `open(path: str) -> Result<File, IoError>` – opens a file handle.
* `File.read_to_end() -> Result<str, IoError>`
* `File.write(data: &str) -> Result<(), IoError>`

### 17.2 Module `net`

* `TcpListener.bind(addr: &str) -> Result<TcpListener, NetError>`
* `TcpListener.accept() -> Result<(TcpStream, SocketAddr), NetError>`
* `TcpStream.read(buf: &mut [byte]) -> Result<int, NetError>`
* `HttpClient.get(url: &str) -> Result<Response, HttpError>`

### 17.3 Module `crypto`

* `sha256(data: &[byte]) -> [byte; 32]`
* `aes_gcm_encrypt(key: &[byte], nonce: &[byte], plaintext: &[byte]) -> Vec<byte>`
* `aes_gcm_decrypt(key, nonce, ciphertext) -> Result<Vec<byte>, CryptoError>`
* `rand_bytes(dest: &mut [byte])` – cryptographically secure output.

### 17.4 Module `unicode`

* `split_graphemes(s: &str) -> Vec<str>` – grapheme‑cluster aware splitting.
* `normalize_nfc(s: &str) -> str` – composed normalization.
* `bengali_consonants() -> &'static [char]` – table of Bengali consonants.

### 17.5 Module `json`

* `encode<T: Serialize>(value: &T) -> str`
* `decode<T: Deserialize>(s: &str) -> Result<T, JsonError>`

### 17.6 Module `sqlite`

* `Database::open(path: &str) -> Result<Database, SqlError>`
* `Database.execute(sql: &str, params: &Params) -> Result<(), SqlError>`
* `Database.query(sql: &str, params: &Params) -> Result<Rows, SqlError>`

### 17.7 Module `http`

* `serve(addr: &str, router: &Router) -> Result<(), HttpServerError>`
* `Router.new() -> Router`
* `Router.get(path: &str, handler: Fn(Request) -> Response)`
* `Response.html(body: &str) -> Response`
* `Response.json<T: Serialize>(value: &T) -> Response`

Full hyperlinks to API docs: `https://docs.nilLang.dev/api/*`.

---

## 18. Compiler Internals – Deep Dive

### 18.1 Lexer (Finite‑State Machine)

The lexer is implemented as a *Mealy* machine that emits `Token` objects defined
in `src/token.nls`.

Token categories:
* `Ident`
* `Literal` (Int, Float, Str, Char)
* `Operator`
* `Delimiter`
* `Comment`
* `Whitespace`

Graph transition table is built at compile time using a generated `.json` file
(`build/token_table.json`) that can also be inspected via `nlc --dump-lexer`.

### 18.2 AST Nodes

```nls
enum Expr {
    Literal(Literal),
    Var(Ident),
    Binary { lhs: Box<Expr>, op: BinOp, rhs: Box<Expr> },
    Call { callee: Box<Expr>, args: Vec<Expr> },
    Lambda { params: Vec<Param>, body: Block },
    NilCheck { inner: Box<Expr> }
}
```

Each AST is **imm portable**; e.g., `Binary` contains its span information
(`Span { start, end }`).

### 18.3 Type Inference Algorithm (Herbrand/Most General Unifier)

1. **Assign type variables** to each sub‑expression (e.g., `?1`, `?2`).
2. **Collect constraints**:
   * Operand types must unify.
   * Function parameter types must match
   * Return types must unify.
   * Generic constraints become explicit trait bounds.
3. **Solve** via a recursive unification routine (`unify(T1, T2)`).
4. **Generalize** and finalize the principal type.

For trait resolution, we use a **generic dictionary passing** technique at the
MIR layer; a small `trait_dispatch` table is generated at compile‑time.

### 18.4 Optimiser

* **Function inlining**: threshold by IR size (default 256 nodes).
* **Tail‑call optimisation** added in v0.2.2 (production readiness*com).
* **Loop vectorisation**: basic SIMD (AVX2) lowering via LLVM `@llvm.vector.reduce`.

### 18.5 Back‑End

* **LLVM**: generated IR is piped to `llc` and `lld` unless `--compiler=internal` is selected.
* **WebAssembly**: uses a lightweight custom runtime (`runtime/wasm/runtime.rs`) that exposes i/O helpers.

---

## 19. Memory Model – Detailed

### 19.1 Runtime Representations

* **Primitive types** (u32, f64) happen on the **stack** directly.
* **Aggregates <= 64 bytes** use **Small‑Value Optimization** (SVO): stored on
the stack and only heap when they escape.
* **Heap objects**: 16‑byte aligned word; header contains:
  * **Reference count** (u32, saturating)
  * **Type tag** (u16)
  * **Reserved padding** (u16)

### 19.2 Garbage Collection

* **Default**: Reference counting + lazy cycle detector (mark‑and‑sweep).
* **Tracing mode**: Boehm–Weiser mark‑and‑sweep (requires `--gc=tracing`).
* **Tiny mode** (`--tiny`): disable GC entirely; work is purely deterministic
resource management via `drop`.

### 19.3 Escape Analysis

The escape pass decides:
* stack‑allocated object? (uses the classic stack‑based pointer analysis).
* object allocated with a call to `heap_new` plus note.

A quick heuristic: `mov` + `ret` local alias exported via channels always
forces heap allocation.

### 19.4 Ownership Semantics

* **Move by default** for non‑`Clone` types.
* **`Clone` trait** for types requiring deep copy.
* **Arc<T> semantics** built‑in and appear under the hood as atomic RC.

---

## 20. Deployment & Packaging

### 20.1 Publishing a Crate

```bash
nlpm pack                 # validate, compile & sign
nlpm publish              # uploading to nilpkg.dev
nlpm unpublish --force    # remove
```

Authentication uses **Ed25519** key pairs (`~/.nilLang/keys`) and is cached.

### 20.2 Cross‑Compiling

```bash
nlc build app.nls --target aarch64-unknown-linux-gnu \
  --cc /usr/bin/clang --linker gold
```

### 20.3 Containers / Docker

```Dockerfile
FROM ghcr.io/nilLang/base:latest
COPY . /app
RUN nlc build main.nls -o /app/app
CMD ["/app/app"]
```

### 20.4 WebAssembly

```bash
nlc wasm main.nls -o dist
nlpm add @nilLang/wasm-runtime
npm i @nilLang/wasm-polyfill
```

Wasm host import table defines 14 syscalls (console.write, timers, fetch).

---

## 21. Security Model

* **Default denial**: network calls require `#[network]` annotation and manifest entry.
* **Sandbox**: `nlc run --sandbox` uses seccomp on Linux / Pledge on OpenBSD.
* **Thread safety**: `Arc<X>` plus channels; data race detected at compile time.
* **Memory safety**: no arbitrary pointer arithmetic unless `#[unsafe]` block.
* **Cryptography**: standard lib uses Ring/LibreSSL.

---

## 22. Performance Tuning Guide

1. **Use iterators** not `append` in loops (`for el in list { ... }`).
2. **Structure padding**: keep **structs ≤ 64 bytes** to enable SVO.
3. **Avoid ` clones`**, use `Borrow` instead of move if possible.
4. **Profile** via `nlc --profile` then `nil-perf` sample tool.

---

## 23. Small Tutorial – Building a Todo List with NilLang

### 23.1 Scaffold

```bash
nlpm init todo
nlpm add io sqlite http
```

### 23.2 Model (`models/todo.nls`)

```nls
struct Todo {
    id: int
    title: str
    done: bool
}
```

### 23.3 Service (`services/db.nls`)

```nls
import sqlite as sql

func init_db() -> Result<(), sql::SqlError> {
    let db = sql::Database::open("todo.db")?
    db.execute("CREATE TABLE IF NOT EXISTS todos (
        id INTEGER PRIMARY KEY,
        title TEXT NOT NULL,
        done INTEGER NOT NULL
    )", ())?
    Ok(())
}
```

### 23.4 HTTP Handler

```nls
import http as http
import json as json

func handle_add(req: http::Request) -> http::Response {
    let body = json::decode::<Todo>(req.body)?;
    if body.title.is_empty() { return http::Response::bad_request() }
    // insert into db...
    http::Response::ok(json::encode(body))
}
```

### 23.5 Main server

```nls
func main() -> int {
    init_db().unwrap()
    let router = http::Router::new()
    router.post("/todos", handle_add)
    http::serve("0.0.0.0:3000", &router).unwrap()
    0
}
```

### 23.6 Build & Run

```bash
nlc build main.nls -o todo
./todo
```

---

## 24. Automated Testing

```nls
// tests/todo_test.nls
import { assert_eq } from "test"

@test
func test_add() {
    let t = Todo { id: 0, title: "Learn NilLang", done: false }
    assert_eq!(t.title, "Learn NilLang")
}
```

```bash
nlc test  # discovers *.test files
```

---

## 25. Continuous Integration (CI)

* `.github/workflows/ci.yml`: checkout, install `nlc` from release, `nlc test`.
* `release-please.yml` produces release binaries/tags once a week.
* Coverage reported to `codecov` using LCOV export.

---

## 26. Glossary

* **AST** – Abstract Syntax Tree; an intermediary representation.
* **MIR** – Mid‑level Intermediate Representation used for optimisation.
* **SVO** – Small Value Optimization.
* **Trait** – Interface‑like abstraction providing polymorphic methods.
* **Coroutine** – User‑level lightweight thread.

---

## 27. Bibliography & References

1. Sarkar, J. – *NilLang Design Draft* (2021) – personal blog.
2. Wikimedia – *Bengali consonant tables* (Wikidata P1084 etc.).
3. LLVM Project – *LLVM Language Reference Manual*.
4. GNU – *GCC docs* (C/C++ semantics reference).
5. Info‑GmbH – *WebAssembly Core Specification* 2.0.

---

## 28. Further Work & Collaboration

We invite:
* **Documentation patches** – `docs/*` Markdown only.
* **Standard library crates** – new packages submitted to `nilpkg.dev`.
* **Language proposals** – via GitHub Issues labelled `proposal`.

Contact: `joysriram.com` (email given) and Discord channel once joined.

---

*Report Volume II generated 2026‑10‑21 by OpenCode.*
