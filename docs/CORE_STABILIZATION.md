# NilLang Core Stabilization Backlog

This document is the execution plan for the stabilization period. New language
features remain out of scope until the P0 exit gate is green. Each item should
land as an independently tested commit in the listed order.

## Release gates

| Level | Exit condition |
| --- | --- |
| P0: Core semantics | Specification and implementation agree; every execution path type-checks; evaluator and VM pass conformance; malformed source never panics the compiler. |
| P1: Canonical compiler | AST -> typed HIR -> MIR is authoritative; basic optimization is semantics-preserving; effects, capabilities, modules, and runtime ownership are enforced. |
| P2: Ecosystem | Standard library, formatter, LSP, lockfile/signing policy, WASM parity, and self-hosting bootstrap are usable. |
| P3: Platform | Alap is stable above the language boundary; Android packaging and the Onuron runtime consume capability-checked artifacts. |

## P0: Core semantics

1. **Freeze grammar and binding rules**
   Owner: `compiler/lexer`, `compiler/parser`, `compiler/ast`, `docs/spec`.
   Reconcile every EBNF production with parser tests. Annotated `let` and strict
   assignment are implemented; continue with optional initializers, compound
   assignment, and the remaining productions. Assignment must update an
   existing lexical binding and report `E0102` otherwise.
   Test: grammar table tests plus parser golden AST fixtures.

2. **Make verification the only front end**
   Owner: `pkg/compiler`, `cmd/nil`, `cmd/nilc`.
   Extract one parse-and-check API returning a typed program and structured
   diagnostics. Route evaluator, VM, HIR/MIR, WASM, REPL imports, and modules
   through it. No backend may receive a program after a failed check.
   Test: each public entry point rejects the same invalid corpus.

3. **Eliminate implicit `Any`**
   Owner: `compiler/types`, `compiler/typecheck`.
   Classify every current `types.Any` use as explicit dynamic intent,
   unresolved inference, or missing generic support. Add `Array<T>`,
   `Optional<T>`, and tensor element/shape contracts before tightening
   `first`, `last`, `push`, and tensor builtins.
   Test: a table records every permitted `Any` origin; unclassified fallback
   fails CI.

4. **Freeze scope and closure semantics**
   Owner: typechecker, evaluator, compiler symbol table, HIR lowering.
   Cover global, module, function, block, loop, component, handler, and closure
   scopes, including shadowing, capture, mutation, and lifetime.
   Test: the same scope corpus runs through checker, evaluator, and VM.

5. **Add language conformance harness**
   Owner: `tests/conformance`.
   Store source plus expected value, stdout, diagnostics, and runtime error.
   Compare evaluator and VM first, then add optimized VM and WASM. Backend
   disagreement is a test failure, not an allowed target difference.

6. **Enforce compiler-never-panics**
   Owner: lexer, parser, typechecker, HIR/MIR lowerers.
   Add seeded fuzz targets and regression corpora. Any source input must return
   an AST or diagnostics. Panics caused by internal invariants remain bugs.

7. **Standardize diagnostics**
   Owner: `compiler/diagnostics` and all producers.
   Every diagnostic carries code, severity, primary span, message, optional
   labels, and help. Remove plain backend error strings at user-facing edges.

8. **P0 CI gate**
   Extend Linux, macOS, and Windows CI with conformance, fuzz seed corpus,
   formatter check, and explicit CLI rejection smoke tests. Publish the first
   conformance matrix as NilLang 0.1.

## P1: Canonical compiler and runtime

1. Make typed HIR the only semantic input and attach resolved bindings, types,
   effects, capabilities, and source spans to calls and values.
2. Lower HIR to CFG-based MIR, introduce SSA deliberately, then implement
   constant folding, propagation, dead-code removal, unreachable-block
   removal, and block simplification with optimized/unoptimized equivalence
   tests.
3. Specify tracing GC as the initial ownership model; add roots for globals,
   stacks, closures, futures, components, and native handles.
4. Convert VM bounds, invalid opcodes/constants/closures, call arity, division,
   modulo, and allocation failures into typed runtime errors.
5. Specify task/future success, failure, cancellation, timeout, panic, and
   parent-child lifetime; implement structured concurrency after cancellation
   is observable.
6. Require compile-time effect/capability proofs and runtime host checks for
   every native call. Oracle output remains advisory and outside deterministic
   verification.
7. Finish import/export/alias/visibility, cycle detection, initialization
   order, and package boundaries.
8. Keep entity schema generation, typed queries, prepared statements,
   migrations, transactions, and offline sync in Alap packages, driven by core
   types rather than new language syntax.

## P2: Ecosystem and self-hosting

1. Stabilize Nilang-written `std` modules for collections, strings, math, IO,
   filesystem, network, time, JSON, crypto, concurrency, and testing.
2. Make `nil fmt` deterministic with `--check`; expose the parser, checker,
   symbols, references, rename, hover, diagnostics, formatting, and semantic
   tokens through `nil-lsp`.
3. Add `nil.lock` with source, version, checksum, and resolved graph; make
   `nil build --frozen` reject drift. Apply configurable allow/warn/deny signing
   policy during install and build.
4. Maintain a VM/WASM feature matrix and require parity fixtures before a
   feature is marked supported.
5. Complete bootstrap lexer, parser, analyzer, and emitter only after the P0
   grammar and type contracts are frozen. Reproducibly compare stage-1 and
   stage-2 compiler output.
6. Add official benchmarks for arithmetic, recursion, closures, collections,
   JSON, async, database, and web. Optimize only from profiles measuring time,
   allocation, GC, opcode frequency, and native calls.

## P3: Alap and Onuron

Alap owns reactive UI, data, database, offline sync, POS, and application
services. POS remains a flagship Alap package. Android first targets reliable
source-to-APK packaging and capability adapters. Onuron progresses through
runtime, then shell, then OS; it does not fork NilLang semantics.

## Version milestones

| Version | Required evidence |
| --- | --- |
| 0.1 | Frozen grammar/core semantics, conformance harness, panic-free fuzz corpus, structured diagnostics. |
| 0.2 | Type system, VM, closures, and modules stable. |
| 0.3 | Typed HIR, MIR, optimizer, and WASM parity. |
| 0.4 | Package lock/signing, formatter, LSP, and standard library. |
| 0.5 | Reproducible self-hosting stages. |
| 1.0 | Stable specification, ABI, package format, standard library, and VM/WASM behavior. |