# ADR-0004: Capability Enforcement and Its Known Holes

## Status

Accepted

## Context

Capabilities are the ecosystem's security boundary between a Nilang program and
the host. The design is least privilege: a program holds no ambient authority,
and every privileged operation needs an explicit, auditable grant from
`nil.json`.

The implementation is real but partial, and the partiality is asymmetric:

- **Direct, statically named calls are gated.** The compiler emits `OpNativeCall`
  for a call to an operation in the requirement table, and the VM consults the
  grant before dispatch, refusing with a typed `*DeniedError` (E0201).
- **Indirect calls are not gated.** `let f = writeFile; f("a.txt", "b");` is not
  recognised at emission time, because at that point the target is an ordinary
  local. Nothing carries the requirement along with the value.
- **The evaluator path is weaker than the VM.** It scans the program before
  evaluation and refuses a directly called gated operation with no grant. It
  sees direct calls only and cannot report a refusal site. The VM path is the
  enforced one.
- **The `requires` clause and E0201-at-compile-time do not exist** in the
  grammar or the type checker. Refusal is at run time, on the first gated call.
- **Pure-context effect checking is inert.** The checker's pure-context flag is
  never enabled, so E0202 cannot fire.
- **An operation absent from the requirement table is refused.** A false refusal
  is a visible bug; a false grant is a vulnerability.

The consequence is that the language gate is a real boundary for a program that
calls things directly and a partial one otherwise. Anything that treats the
manifest as the only security boundary is relying on the parts that are missing.

## Decision

**The language gate is a defence in depth layer, not the boundary. OnuronOS
enforcement is the boundary.** The gaps are named in the normative spec, in the
published contract, and in every artifact a consumer can see.

- The requirement table is exhaustive and explicit, generated into
  `contracts/capabilities/requirements.json` so that a verifier in another
  language can consult it without linking Go.
- The capability vocabulary is published in
  `contracts/capabilities/vocabulary.json`, with aliases and with the planned
  sub-capabilities listed as planned — so `filesystem.read` is refused with an
  explanation rather than a shrug.
- Every validator is default-deny: an unknown capability name is refused.
- Alap carries capabilities as app, route and module metadata and grants no
  authority of its own.
- OnuronOS maps a declared capability onto real SELinux, service, file and device
  permissions and enforces them.
- The emulator runs least-privilege negative tests.
- **No feature is described as supported without the coverage named.** The
  support matrix downgrades a cell with no evidence to experimental.

The indirect-call gap is recorded as a known limitation rather than closed by a
new syntax. Closing it properly needs either the `requires` clause or a
first-class function value that carries its requirement — both are language
changes, and both need explicit approval to amend the core freeze.

## Alternatives rejected

**Close the indirect-call gap now with a `requires` clause.** Rejected for now:
it is new grammar during an active freeze, and CORE_FREEZE.md requires a spec
update, conformance coverage, a stable diagnostic, a migration note and
explicit approval to amend the freeze in the same change. It is not refused
forever; it is sequenced behind the freeze.

**Treat the manifest as the only security boundary.** Rejected: it is the part
with a known hole. A manifest is a claim made by the package about itself.

**Infer requirements from naming conventions.** Rejected, and it was rejected in
the implementation too. An inferred rule either misses a dangerous operation or
denies a harmless one, and both failure modes are security-relevant.

**Remove the gated operations from the language instead.** Rejected: the
operations are the reason a Nilang program is useful. The answer is enforcement,
not amputation.

## Migration plan

1. Publish the requirement table and vocabulary as part of the contract bundle.
   **Done.**
2. Each consumer's validator adopts the published table rather than its own.
3. OS enforcement coverage is measured against the vocabulary: for each
   capability, an install-time test that the declared permission is granted and a
   negative test that it is not.
4. After the freeze exits, revisit the indirect-call design with the spec change
   the freeze requires.

## Evidence

- `tests/capability_enforcement_test.go`
- `compiler/capability/manifest_test.go`
- `contracts/capabilities/vocabulary.json`, whose `known_limitations` field is the
  machine-readable form of this record's context.
- `pkg/contract/contract_test.go` — `TestRequireCapabilityRefusesUnknownOperations`
  and `TestCapabilityVocabularyIsDefaultDeny`.

## Reversal condition

If OnuronOS enforcement is not delivered, or proves weaker than the language
gate in practice, this record is wrong and the decision must be revisited. The
language gate would then need to become load-bearing, which means the indirect
call gap has to be closed first — and the freeze amended to allow it. The
alternative, shipping with the OS layer unwritten and the manifest as the only
boundary, is not an option the ecosystem has.