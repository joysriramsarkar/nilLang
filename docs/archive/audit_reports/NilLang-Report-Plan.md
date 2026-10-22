# NilLang – 150‑Part Long‑Report Plan

The report is divided into **150 compositional parts** (each ≈ 2,500–4,000
Bangla words).  After this outline is read, each part is appended to the
master file `NilLang-Report-Full.md` by running `touch`/`append` commands or
utilising `edit` with a unique marker block.

| # | Title |
|---|-------|
| 1 | Preface – Motivation & Context |
| 2 | Historical Timeline (2021‑2026) |
| 3 | Language Design Philosophy |
| 4 | Influence Matrix (Rust, Go, MLIR, Mojo) |
| 5 | Roadmap v1.0 → v2.0 |
| 6 | Lexical Structure & Tokens |
| 7 | Unicode Support & Bengali Text |
| 8 | Identifiers and Keywords |
| 9 | Numeric Literals (Int, Float, Hex, Bin, Oct) |
| 10 | String Literals & Escapes |
| 11 | Char vs Rune Model |
| 12 | Comments & Documentation Comments |
| 13 | Expression Grammar Full |
| 14 | Statement Grammar Full |
| 15 | Type Grammar & Precedence |
| 16 | Type Inference Rules (HM + Constraints) |
| 17 | Trait Resolution Algorithm |
| 18 | Variance & Subtyping |
| 19 | Slices & Range Semantics |
| 20 | Match / Pattern Matching Design |
| 21 | Block Syntax & Indentation Rules |
| 22 | Error Handling (`Result`, `?`, `panic`) |
| 23 | `Option<T>` & Nil‑Safety Flow |
| 24 | Generics: Declarations & Bounds |
| 25 | Higher‑Rank & Existential Types |
| 26 | Closures & Capturing Environment |
| 27 | Function Pointers vs Closures |
| 28 | Modules & Import System |
| 29 | Visibility (`pub` / `priv`) |
| 30 | Packages, Manifest & Registry |
| 31 | Toolchain Bootstrapping (`nlc self`) |
| 32 | Cross‑Compiler Backends (LLVM/WASM) |
| 33 | Standard Library – `io` Deep |
| 34 | Standard Library – `net` Deep |
| 35 | Standard Library – `crypto` Deep |
| 36 | Standard Library – `unicode` Deep |
| 37 | Standard Library – `json` Deep |
| 38 | Standard Library – `sqlite` Deep |
| 39 | Standard Library – `http` Deep |
| 40 | Standard Library – `test` Deep |
| 41 | Lexer – DFA Construction |
| 42 | Lexer – Unicode Grapheme Handling |
| 43 | Parser – PEG Rules |
| 44 | Parser – Error Recovery |
| 45 | AST Node Reference |
| 46 | Span/Location Tracking |
| 47 | Type Inference Pass |
| 48 | Trait Dictionary Passing |
| 49 | MIR – Lowering |
| 50 | MIR – Optimisations |
| 51 |MIR – Escape Analysis |
| 52 | Backend – LLVM IR Generation |
| 53 | Backend – WebAssembly Lowering |
| 54 | Runtime – Startup & Threads |
| 55 | Runtime – Channels & Select |
| 56 | Runtime – Scheduler |
| 57 | Memory – Stack/Heap Split |
| 58 | Memory – Reference Counting |
| 59 | Memory – Cycle Collector |
| 60 | Memory – Tracing GC Mode |
| 61 | Ownership & Borrow Checker – Hard Cases |
| 62 | Concurrency – Locks vs Channels |
| 63 | Concurrency – Fairness & Deadlock Prevention |
| 64 | Networking – HTTP/2 & WebSockets |
| 65 | HTTP Server – Middleware Model |
| 66 | Asynchronous I/O & async/await |
| 67 | Database – SQLite WAL & Indexing |
| 68 | Database – ORM/Entity Design |
| 69 | Persistance – JSON vs CBOR |
| 70 | Web APIs – REST/GraphQL |
| 71 | Authentication – JWT & OAuth |
| 72 | Security – Sandboxing |
| 73 | Sandboxing – seccomp/Pledge |
| 74 | Sandboxing – Resource Limits |
| 75 | Cryptography – AEAD & Hashes |
| 76 | Randomness (CSPRNG) |
| 77 | Cross‑Platform Path & Filesystem |
| 78 | File I/O Streams & Buffers |
| 79 | Text Encoding – UTF‑8/UTF‑16 |
| 80 | Grapheme‑Cluster – Bengali Compound |
| 81 | Unicode Normalization & ISCII |
| 82 | Performance – Optimisation Guide |
| 83 | Performance – Profiling (`nlc --profile`) |
| 84 | Performance – Benchmark Suite |
| 85 | Benchmarks – CPU (FizzBuzz, CSV) |
| 86 | Benchmarks – Memory (Sieve, Graph) |
| 87 | Benchmarks – Concurrency (Fib) |
| 88 | Benchmarks – Wasm Startup |
| 89 | Testing – Unit Tests |
| 90 | Testing – Property‑Based |
| 91 | Testing – Fuzzing |
| 92 | Testing – Coverage |
| 93 | CI/CD – GitHub Actions |
| 94 | CI – Cache & Artifacts |
| 95 | Release Management |
| 96 | Publishing to Registry |
| 97 | Dependency Resolution (`nlpm`) |
| 98 | Semantic Versioning & Lockfile |
| 99 | Interoperability – C FFI |
| 100 | Interoperability – Rust Bindings |
| 101 | Interoperability – Go Bindings |
| 102 | Interoperability – JS/Wasm |
| 103 | Interoperability – Kotlin/JNI |
| 104 | FFI – C Headers & ctypes |
| 105 | FFI – Callbacks & Trampolines |
| 106 | Package – `pos-app` Deep Architecture |
| 107 | Package – `gandatabase` Deep Architecture |
| 108 | Package – `banglagan` Deep Architecture |
| 109 | Package – `banglagan_android_app` Deep Architecture |
| 110 | Package – `notes-app-nilLang` Deep Architecture |
| 111 | Package – `onuron_project` Deep Architecture |
| 112 | Tutorial – Hello World to CLI |
| 113 | Tutorial – CLI Todo App |
| 114 | Tutorial – HTTP Web Server |
| 115 | Tutorial – PWA (Proof of Concept) |
| 116 | Tutorial – Wasm Game (Snake) |
| 117 | Tutorial – Mobile Calculator |
| 118 | Tutorial – Database CRUD |
| 119 | Tutorial – REST API |
| 120 | Tutorial – GraphQL Gateway |
| 121 | Best Practices – Code Style |
| 122 | Best Practices – Project Layout |
| 123 | Best Practices – Error Propagation |
| 124 | Best Practices – Logging & Tracing |
| 125 | Best Practices – Unicode & I18n |
| 126 | Anti‑Patterns & How to Avoid |
| 127 | Migration: From C to NilLang |
| 128 | Migration: From Rust to NilLang |
| 129 | Migration: From Python to NilLang |
| 130 | Ecosystem – Community & Governance |
| 131 | Ecosystem – Contribution Process |
| 132 | Ecosystem – Roadmap Governance |
| 133 | Ecosystem – Funding & Sustainability |
| 134 | Future – Formal Verification |
| 135 | Future – Distributed Systems |
| 136 | Future – Embedded & IoT |
| 137 | Future – AI‑Native Features |
| 138 | Future – AR/VR & Graphics |
| 139 | Case Study – 01: E‑Commerce POS |
| 140 | Case Study – 02: Music DB |
| 141 | Case Study – 03: Android Cal |
| 142 | Case Study – 04: Note App |
| 143 | Case Study – 05: Web Snakes |
| 144 | Case Study – 06: Onuron Ecosystem |
| 145 | Academic References |
| 146 | Appendix A – Grammar (Formal) |
| 147 | Appendix B – API Cheat Sheet |
| 148 | Appendix C – Unicode Tables |
| 149 | Appendix D – Error Codes |
| 150 | Final Epilogue |

*Format*: Each part ends with the marker `<!-- END PART n -->`.  Continue until all 150 are produced.
