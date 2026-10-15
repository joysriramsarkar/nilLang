# Onuron ecosystem ADRs

These records cover decisions that span more than one of the five repositories:
NilLang, Alap Framework, OnuronOS, the NilOS emulator and the Onuron Project
store. They live in NilLang because NilLang owns the language, the capability
vocabulary and the package format, so the reader who has to understand a
cross-repo contract most often starts there.

Each record states its status, the decision, the alternatives that were rejected
and why, the migration path, the evidence that the decision is being followed,
and the condition that would reverse it.

| ADR | Subject | Owner of the decision | Status |
|---|---|---|---|
| [0001](0001-canonical-nilang-implementation.md) | One canonical language implementation | nilLang | Accepted |
| [0002](0002-compiler-and-cli-ownership.md) | Compiler, CLI and language tooling ownership | nilLang | Accepted |
| [0003](0003-nilax-package-and-abi-versioning.md) | `.nilax` package format and ABI versioning | nilLang | Accepted |
| [0004](0004-capability-enforcement.md) | Capability enforcement and its known holes | nilLang + onuronOS | Accepted |
| [0005](0005-ui-and-event-contract.md) | UI lifecycle and event contract | alap-framework | Proposed |
| [0006](0006-pos-source-of-truth.md) | POS source of truth and generated store artifacts | alap-framework | Proposed |
| [0007](0007-emulator-architecture.md) | Emulator architecture and the single simulator | nilos-emulator | Accepted (see onuronOS ADR-0009) |
| [0008](0008-docs-and-maturity-generation.md) | Documentation and maturity data generation | nilLang + onuron_project | Accepted |

## Status values

- **Proposed** — agreed in principle, not yet implemented. A repository may not
  claim support for something whose ADR is still proposed.
- **Accepted** — implemented, with the evidence named in the record.
- **Superseded** — replaced by a later ADR, which is named.

## Relationship to the per-repository ADRs

OnuronOS keeps its own records in `docs/adr/` for decisions that are internal to
the OS. Where the two overlap — the emulator's role, Alap's boundaries — the
cross-repo record here is authoritative and points at the OS record for the
implementation detail. Two ADRs that contradict each other are a defect in one of
them; the cross-repo one wins, and the other is amended.