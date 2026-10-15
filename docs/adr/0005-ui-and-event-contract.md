# ADR-0005: UI Lifecycle and Event Contract

## Status

**Proposed** — owned by `alap-framework`, recorded here because it spans Alap,
OnuronOS and the emulator.

## Context

A UI in this ecosystem can be rendered by three implementations:

- the Go UI renderer in NilLang (`pkg/alap/ui`, `pkg/nilui`);
- the Rust shell and `alap/` renderer in OnuronOS;
- the server-side HTML/SSR path in Alap, which the web app store uses.

They do not need to be pixel-identical — Go, Rust and an HTML serialiser will
never agree on that, and requiring it would buy nothing. What must agree is
*behaviour*: which events fire, what state transitions they cause, what a
component's props mean, when a widget is disposed, and what an unsupported
widget does.

Nothing states that today. Each implementation decided, and an app written
against one of them can behave differently on a device using another in ways no
test catches.

The parts that need to be pinned:

- component type, name, props and state;
- rendering output schema;
- event names, payload types and serialisation;
- state update and reconciliation semantics;
- navigation, lifecycle and disposal;
- asynchronous events and cancellation;
- focus, keyboard and accessibility;
- font shaping, Unicode grapheme handling and Bengali text metrics;
- layout units, scaling, DPI and display orientation;
- the behaviour of an unsupported widget.

Bengali text is not a footnote here. Grapheme clustering and conjunct formation
differ between renderers, and a component that measures text differently
produces different layout on a phone than in a browser.

## Decision

**A versioned, functional golden contract, not a pixel contract.** Alap owns it.

1. The contract specifies component semantics, event names and payload types,
   state transitions, lifecycle and disposal, accessibility tree shape, and text
   and layout measurement rules.
2. Cross-renderer parity is tested against a shared corpus of components and
   interactions. Each renderer produces the same functional trace — events
   fired in order, resulting state, accessibility tree, layout constraints
   resolved — for the same input.
3. Where a renderer genuinely differs, the difference is recorded as a **named
   parity gap** in the contract, with the renderer's name. A known gap is
   acceptable; an undocumented one is a defect.
4. An unsupported widget is an explicit, named refusal. It does not silently
   render as an empty box.

Because the contract is owned by Alap, it lives in Alap. It is **not** added to
NilLang's contract bundle until Alap has it in a state that can be pinned.

## Alternatives rejected

**Pixel-identical rendering as the bar.** Rejected: it is unreachable across
three toolchains and a browser, and meeting it partially produces a contract
nobody satisfies, which is worse than one everybody meets.

**Let each renderer be authoritative on its own platform.** This is the status
quo and it is rejected because it makes app behaviour a property of the device,
which is the opposite of what a package store needs to promise.

**One renderer everywhere, including the web.** Rejected: SSR and client
rendering are genuinely different environments, and a phone-only UI framework for
the store site would not survive contact with a browser tab.

## Migration plan

1. Alap writes the component and event contract, reusing the existing `UI_SPEC.md`
   as its starting point and reconciling it with what the three renderers
   actually do.
2. Alap publishes a conformance corpus: components, interactions, and the
   expected functional trace.
3. Each renderer runs the corpus. Mismatches become named parity gaps, with an
   owner and a date.
4. Only then is the contract considered stable enough to pin in the shared
   bundle.

## Evidence

Today: `docs/spec/UI_SPEC.md`, `pkg/nilui/parity/` and its `DEVICE_MATRIX.md`
and `FLAKINESS.md`, and the Bengali text tests in `pkg/nilui/textbn`. These are
local artefacts, not a cross-repository contract.

## Reversal condition

If a functional golden contract proves too brittle — every renderer change
requires a contract change — the contract narrows to event and state semantics
only, and layout and text measurement become explicitly per-renderer with
documented limits. The parts that must never be dropped are event names, state
transitions and disposal: those are what an app's correctness rests on.