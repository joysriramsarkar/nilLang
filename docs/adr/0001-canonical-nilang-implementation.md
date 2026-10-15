# ADR-0001: One Canonical NilLang Implementation

## Status

Accepted

## Context

NilLang exists in three places that all claim to run it:

- `nilLang` — the Go compiler, the tree-walking evaluator and the bytecode VM.
- `alap-framework` — a Go parser, compiler, VM and stdlib of its own.
- `onuronOS/runtime/nillang` and the emulator — small VMs written in Rust, plus
  a second language VM inside the emulator.

The five repositories were audited separately and each reported its
implementation as working. None of the audits established that the four
implementations agree on what a program means.

That gap is not academic. "The POS app runs" is a claim about five codebases,
and with no conformance suite binding them the claim cannot be checked. The
alternative — every runtime re-derives the language — means a semantic fix in
NilLang has to be reimplemented four times, correctly, and the ecosystem's
behaviour depends on which one a device happens to be running.

## Decision

**The Go implementation in NilLang is the canonical NilLang.** Its compiler,
evaluator and bytecode VM define the language.

Alternatives and their roles:

| Implementation | Role | May claim |
|---|---|---|
| `nilLang` Go compiler + VM | canonical | Nilang 0.1, subject to the support matrix |
| `nilLang` evaluator | REPL and development path | The subset the conformance corpus covers, with its weaker capability gate stated |
| Alap's Go parser/compiler/VM | **not** a conformance reference | Nothing. It delegates to, or tracks, the canonical compiler. |
| OnuronOS Rust mini-VM | subset runtime | Only the subset its conformance run covers. Not Nilang 0.1 compatibility. |
| Emulator language VM | **not** a language implementation | Nothing. It runs artifacts the canonical compiler produced. |

A non-canonical runtime earns a wider claim by passing the conformance suite in
`tests/` on the features it supports. Until it does, its README states the
subset. This is a claim about evidence, not about permission: nothing here stops
a runtime from running Nilang programs, and everything here stops it from
calling the result "Nilang 0.1 compatible".

## Alternatives rejected

**Rust rewrite of the core now.** Rejected by the ecosystem plan, and the reason
holds: a rewrite is a second implementation built before the first is stable, so
it doubles the conformance problem instead of solving it. If the Go
implementation is going to be replaced, replacing it after it is specified and
tested is strictly better than replacing it before.

**Treat the four implementations as equally authoritative.** Rejected because
there is no tie-breaker. Four parsers disagreeing about a grammar production
produces a bug report that nobody can assign.

**Keep them and add a compatibility-mode flag per runtime.** Rejected: the flag
would encode "this subset works" in prose, and prose does not fail CI.

## Migration plan

1. NilLang's conformance corpus is the reference. It exists and runs on three
   operating systems.
2. The support matrix (`contracts/compatibility/matrix.json`) states each
   backend's status with an evidence path.
3. Non-canonical runtimes add conformance runs for the subset they execute and
   downgrade anything they cannot cover.
4. Duplicate toolchains are removed one at a time, each removal keeping a
   dependency map, a parity test and a fallback tag — never as a single sweeping
   delete.

## Evidence

- `tests/cross_engine_values_test.go` — evaluator and VM agree on values.
- `tests/scope_corpus_test.go` — the same scope corpus across checker,
  evaluator and VM.
- `tests/frontend_gate_test.go` — one parse-and-check gate for every public entry
  point.
- CI job `conformance` on ubuntu, macOS and Windows.
- `contracts/compatibility/matrix.json` — where a backend's claim is recorded.

## Reversal condition

If the Go implementation cannot be maintained — it cannot reach a target the
ecosystem needs, or a language change cannot be made in it without unacceptable
effort — then the canonical role moves. It does not move because a Rust
implementation exists and is faster; it moves when the canonical implementation
is the thing standing in the way, and only with the conformance corpus already
able to arbitrate between the two.