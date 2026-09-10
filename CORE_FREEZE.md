# Nilang 0.1 Core Freeze

**Status:** Active
**Scope:** Nilang core language, compiler semantics, and runtime contracts
**Exit criterion:** The Nilang 0.1 Core Language Stable definition of done is met

The following compatibility surfaces are frozen:

- Language syntax frozen
- Type semantics frozen
- Scope semantics frozen
- Assignment semantics frozen
- Function semantics frozen
- Module semantics frozen
- Error semantics frozen
- Concurrency semantics frozen
- Memory semantics frozen
- Effect semantics frozen
- Capability semantics frozen

During the freeze, changes may correct an implementation/specification mismatch,
close a safety hole, improve diagnostics without changing their stable code, or add
tests and tooling. New syntax, types, effects, capabilities, and runtime semantics
must wait until the freeze exits.

Any intentional compatibility change requires all of the following in the same
change:

1. A specification update under `docs/spec/`.
2. Evaluator and VM conformance coverage when the behavior is executable.
3. A stable diagnostic and regression test when the behavior is rejected.
4. A migration note in the release notes.
5. Explicit approval to amend this freeze contract.

The implementation is not allowed to claim an unimplemented draft feature as a
Nilang 0.1 guarantee. Draft and experimental behavior must be labelled as such in
the specification.