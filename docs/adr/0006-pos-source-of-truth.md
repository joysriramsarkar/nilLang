# ADR-0006: POS Source of Truth and Generated Store Artifacts

## Status

**Proposed** — owned by `alap-framework`.

## Context

The POS application exists in more than one form: a `.nil` source in the store's
`registry-src/`, a Go domain implementation under `pkg/alap/pos/`, and a published
`.nilax` in the store's package directory and on disk in `onuron_project`.

There is no single place to edit. When a business rule changes, somebody has to
know which copies to change and which artifact to rebuild and re-sign — and
getting that wrong produces a store entry whose behaviour does not match its
source, which is the failure the ecosystem's whole contract effort is meant to
prevent.

POS is where this bites hardest, because POS has real invariants: stock, cart
totals, tax and discount, purchase inward, refunds and returns, payment status,
shift open and close, cash reconciliation, RBAC and audit, and concurrent sales.
An invariant that holds in the Go tests and not in the shipped app is not tested.

## Decision

**One source per artifact, and every store artifact generated from it.**

1. **One canonical POS source.** The `.nil` application in the store's
   `registry-src/` is the single editable source. The Go domain implementation
   under `pkg/alap/pos/` is a test oracle and a library, not a second copy of the
   application; if it needs a behaviour the app does not have, that is a defect
   in the oracle.
2. **One business invariant suite, one fixture set.** Scenarios and their
   expected results live in one canonical fixture. The Go implementation and the
   `.nil` application are both tested against it.
3. **Store artifacts are generated.** A `.nilax` in the store is built from the
   canonical source by a pipeline, and the store record carries the source
   revision, the compiler revision, the package digest and the signature key id
   (ADR-0003).
4. **The same invariant suite runs in the emulator**, including the ESC/POS
   golden output for a virtual printer. Receipt bytes are part of the
   observable behaviour of a POS system, so they are asserted, not eyeballed.
5. **A published `.nilax` whose source revision does not resolve is refused by
   the store.** "It works on the device" is not a substitute for knowing what it
   was built from.

## Alternatives rejected

**Keep the Go implementation as the source and generate the `.nil` app from
it.** Rejected: it inverts the language's role. NilLang is the canonical
implementation (ADR-0001), so the app is the artefact and the Go code is the
oracle.

**Keep both and require a human to keep them in step.** This is the status quo.
It is rejected for the ordinary reason: it works until the first release where
nobody remembered.

**Test the shipped `.nilax` only.** Rejected: a binary that fails a test tells
you the release is broken, not which rule diverged. Both the source and the
artifact are tested, against the same fixtures.

## Migration plan

1. Name the canonical POS source explicitly and record it in the store schema.
2. Extract the existing Go POS tests into a shared invariant fixture file.
3. Run that fixture against the `.nil` application in the emulator, including
   ESC/POS golden output.
4. Add source revision and compiler revision to the store record, and refuse a
   package whose source revision does not resolve.
5. Re-sign and republish the current POS package so the published artifact and
   the canonical source agree.

## Evidence

Today: `pkg/alap/pos/conformance_test.go`, `e2e_test.go`, `failure_test.go` and
`concurrent_test.go` exercise the Go implementation. These are the oracle the
fixture extraction starts from.

Not yet: the same fixtures run against the `.nil` application. Until they do,
the POS package's store entry must not claim end-to-end conformance.

## Reversal condition

If generating the store artifact from the canonical source proves to make
release slower than the ecosystem can accept, the store moves to accepting a
hand-built artifact **with** a mandatory source revision and a reproducible-build
check, so the artifact is still verifiably derived from a source. The invariant
fixture requirement does not relax in either case: an untested POS release is
the thing this record exists to prevent.