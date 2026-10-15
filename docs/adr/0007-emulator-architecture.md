# ADR-0007: Emulator Architecture

## Status

Accepted — the OS-side implementation of this decision is OnuronOS ADR-0009 and
the `nilsim` role split recorded in that repository's plan.

## Context

The emulator was carrying more than one thing:

- QEMU orchestration, device profiles and virtual hardware;
- a language VM of its own;
- a desktop simulator/front-end;
- `nilctl` and `nilosd` control tooling.

Four of those are the emulator's job. The language VM is not. A second VM inside
the emulator means emulator behaviour and device behaviour can diverge on
anything the two implementations disagree about — and neither divergence nor the
disagreement is visible from inside either repository.

The desktop simulator is a similar case: if the OS already ships a shell and
compositor, a second interactive front-end on the host is a second implementation
of the same behaviour with no second conformance requirement.

## Decision

**QEMU is the authoritative execution backend. There is exactly one desktop
simulator. The emulator does not re-derive language semantics.**

- QEMU/VM lifecycle, device profiles, virtual hardware, snapshots and logs are
  the emulator's.
- A `.nil` artifact the emulator installs and runs is produced by the canonical
  compiler (ADR-0001). The emulator's own VM is not a language implementation and
  does not claim conformance.
- Exactly one interactive simulator is active. The OS-side `nilsim` stays, per
  OnuronOS's own record; any second front-end is either merged into it or
  retired.
- `nilctl` and `nilosd` own the emulator control protocol. That protocol, the
  SoftBus protocol and the store's REST API are three separate contracts with
  three separate versioning policies, and they are not interchangeable.
- The virtual HAL is where ESC/POS and hardware behaviour are observed. A claim
  about device behaviour needs output from it, not a mock.

## Alternatives rejected

**Keep the emulator's VM as a fast path.** Rejected: a fast path that implements
the language differently is where conformance bugs hide. The optimisation is to
avoid booting QEMU where QEMU is not needed, not to run a different program.

**Keep both simulators, one for development and one for CI.** Rejected: the two
diverge, and the one that diverges is whichever nobody is watching. If two are
truly needed, one is generated from the other or both run the same conformance
corpus.

**Let the emulator define device behaviour and have the OS conform to it.**
Rejected: the device is the thing that ships. The emulator tests the OS; it does
not define it.

## Migration plan

1. The role split is recorded in OnuronOS (`nilsim` stays; application runtime
   execution lives in the emulator). **Done.**
2. The emulator's language VM is marked `prototype/subset` in its README and
   removed from any compatibility claim.
3. Duplicate front-ends are inventoried, and each is either merged or retired —
   with a fallback tag, never in a single sweeping removal.
4. The emulator's CI runs the same conformance corpus the OS does, against
   QEMU, so "the emulator agrees" becomes a test result rather than an assertion.

## Evidence

- `contracts/compatibility/matrix.json`, `cross_repo_claims` — the emulator's
  claim is recorded as partial and says why.
- OnuronOS ADR-0009 and its `docs/plan` entry recording the nilsim outcome.
- The deterministic replay gate in the emulator (`test(runtime)`: instant
  `.nilrec` replay reproduces a live patch stream), which is the evidence that
  emulator runs are reproducible enough to compare against the OS.

## Reversal condition

If QEMU cannot boot in an environment the ecosystem must support, the emulator
gains a native execution backend — as a second *device* backend, not a second
*language* implementation. The language boundary in ADR-0001 does not relax.