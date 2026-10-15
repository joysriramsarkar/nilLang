# ADR-0003: `.nilax` Package Format and ABI Versioning

## Status

Accepted

## Context

A `.nilax` is a zip carrying `manifest.json`, compiled bytecode, and whatever
resources the app needs. The manifest written by `pkg/bundle` records a format
version, an app name and version, the entry bytecode path, targets, a build
timestamp, a compiler version, a dependency map, a capability list, free-form
metadata and — if it was signed — a signature string.

Five consumers read that manifest: the NilLang installer, OnuronOS at install
time, the emulator in CI, the store's API and any third-party package manager.

The format has five gaps that matter once more than one consumer exists:

1. no source revision, so a published package does not identify its source;
2. no runtime ABI field, so a package cannot state what it needs from a runtime;
3. no per-file digests, so payload integrity rests entirely on a whole-archive
   signature that only helps a verifier who already holds the expected digest;
4. `signature` is a bare string here and a structured object in `pkg/signing`,
   which is two representations of one fact;
5. `app_name` doubles as an application identifier.

There was also no written compatibility policy, which meant each consumer
decided on its own what to do with a manifest format it did not recognise.

## Decision

**NilLang owns the package format. The contract is published, checksummed, and
enforced default-deny.**

- The machine-readable contract is `contracts/nilax/manifest-v1.schema.json`
  (as implemented) and `contracts/nilax/manifest-v2.schema.json` (proposed),
  with the migration and compatibility policy in
  `contracts/nilax/MIGRATION.md`.
- `contracts/abi/runtime-abi.json` defines the language spec version, the runtime
  ABI version and its compatible range, and the target list.
- **An unknown `format_version` is refused.** A verifier does not read the fields
  it recognises out of a future format; a partial read of a manifest is
  indistinguishable from a complete one at the point the decision is made.
- **An unknown capability, target or backend is refused.** Never defaulted,
  never ignored.
- **Validation runs in a fixed order** — archive safety, identity, integrity,
  trust, compatibility, capability, install — and reports every failure rather
  than the first.
- v1 is frozen for the duration of the core freeze. v2 is `proposed` and no
  validator enforces it.

## Alternatives rejected

**Validate on demand, without a published schema.** Rejected: the schema is what
a Rust or JavaScript consumer implements against. A Go struct is not a contract,
it is one implementation's opinion of one.

**Make every unknown value a warning.** Rejected outright. This is the decision
the whole record exists to prevent: a future target name that silently installs
and then fails to launch is worse than a refusal the user can act on.

**Go straight to v2 and break v1.** Rejected. Six packages are published in the
store today and devices have them installed. Replacing the format without a
window strands every one of them, which is why the migration is staged and v1
is retired only after a window.

**Sign the archive only, forever.** Rejected for v2. A whole-archive signature
means a store that records a size and a digest detects substitution and a store
that records neither detects nothing. Per-file digests inside the signed
manifest make the second case detectable.

## Migration plan

1. Publish both schemas and the policy. **Done.**
2. Enforce the contract on the existing install path. **Done** —
   `pkg/nilpkg.Verifier.VerifyBundle` runs the seven stages; `nil-contract
   validate` is the standalone form.
3. `nil build --format 2` writes both; v1 stays the default.
4. Each verifier learns to accept both and records which it saw.
5. Flip the default once every verifier accepts v2.
6. Retire v1 only after a published window, with a dedicated ADR.

## Evidence

- `pkg/contract/contract_test.go` — schema, policy and drift gates.
- `pkg/nilpkg/contract_test.go` — enforcement against real archives.
- `contracts/nilax/MIGRATION.md` — the field-by-field migration.
- All six published store packages pass `nil-contract validate`; none passes
  `nil-contract validate -trust`, because none is currently signed. That is the
  recorded state, not a passing result.

## Reversal condition

If v2's per-file digest model turns out to cost more than it buys — for
instance because package sizes make manifest signatures meaningfully slower to
verify — the digest list moves out of the manifest and into a detached sidecar.
The rest of this record, and in particular the default-deny policy, does not
change.