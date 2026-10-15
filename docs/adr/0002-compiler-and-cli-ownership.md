# ADR-0002: Compiler, CLI and Language Tooling Ownership

## Status

Accepted

## Context

Language-level tooling exists in more than one place:

- `nil` (build, run, fmt, profile, render), `nilc`, `nil-lsp`, `nil-bootstrap`
  and `nil-runner` live in NilLang.
- `pkg/dap` implements a language debugger adapter.
- `pkg/signing` and `cmd/nilkey` implement the build/signing key tools.
- The emulator contains a language VM and, historically, its own language
  tooling.
- Alap has its own parser and CLI surface.

When two tools answer the same question about a program — "is this valid", "what
does this symbol mean" — and disagree, the user has no way to tell which one is
right. The P0.2 work in NilLang exists because `pkg/compiler`, `cmd/nil`,
`cmd/nilc` and the REPL could each accept a program another rejected.

## Decision

**All language-level tooling is owned by NilLang and nothing else builds its own.**

| Tool | Owner | Notes |
|---|---|---|
| `nil`, `nilc` | nilLang | One parse-and-check gate behind both. |
| `nil-lsp` | nilLang | Language server. |
| Language DAP adapter (`pkg/dap`) | nilLang | Not a general debugger. |
| `nilkey`, build signing tools | nilLang | Keys live with the tooling that needs them, never with a consumer. |
| `.nilax` bundling and format | nilLang | See ADR-0003. |
| Formatter (`nil fmt`) | nilLang | |
| Language VM / compiler | nilLang | See ADR-0001. |

Consumers get three ways in, in preference order:

1. **Import the Go API.** `pkg/compiler`, `pkg/bundle`, `pkg/contract`,
   `pkg/signing`.
2. **Run the pinned `nilc` binary.** For a consumer in another language that
   wants the canonical frontend without linking Go.
3. **Read the contract bundle.** For a consumer that only validates artifacts
   and never compiles.

A consumer that implements a fourth option — its own frontend — may do so for
its own reasons, but it may not call the result canonical, and any disagreement
is resolved in NilLang's favour.

## Alternatives rejected

**Ship the compiler as a library plus a stable C ABI and let consumers pick.**
This is the eventual shape, and it is closer to right than what exists. It is
rejected *now* because publishing a C ABI pins a representation that has not
settled, and the plan already records the runtime ABI surface as
`partial`/unspecified. Publishing it early would freeze today's uncertainty
permanently.

**Let the strongest consumer own the CLI.** Rejected: that inverts the
dependency. The emulator does not define the language because it has the most
realistic test harness; it defines how a language program behaves on a
particular device profile.

**Keep both and require users to pick.** Rejected: it makes the disagreement the
user's problem, which is the outcome the whole contract effort exists to
prevent.

## Migration plan

1. `pkg/compiler.Compile` is the single parse-and-check entry point; the CLI and
   the library both go through it.
2. `pkg/compiler/entry_rejection_test.go` pins the rejection corpus across every
   public entry point.
3. Each duplicate tool in another repository is either deleted or renamed so that
   it cannot be mistaken for the canonical one. Renaming is the interim step
   that lets a removal be reviewed as a rename first.

## Evidence

- `pkg/compiler/entry_rejection_test.go`
- `tests/frontend_gate_test.go`
- CI job `dx-gate`, which runs the CLI rejection smoke on every OS.

## Reversal condition

If the language acquires a target that Go cannot compile for, and the compiler is
therefore blocked, the compiler moves to whatever language reaches that target —
carrying the conformance corpus as the arbitration mechanism, exactly as in
ADR-0001. The tooling does not fragment before the compiler does.