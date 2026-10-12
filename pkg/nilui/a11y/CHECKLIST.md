# TalkBack / VoiceOver Checklist (U11.8)

Mandatory before any Alap UI release. Each row must be verified on a real
device with the screen reader on — the emulator alone is not enough.

## Per-component (all 54)

- [ ] Role announced correctly (button/link/header/image/checkbox/…).
- [ ] Label in Bangla and English (locale switch covered by U12 tests).
- [ ] Hint describes the *outcome*, not the gesture ("saves draft").
- [ ] Value/state announced (checked/on/off, slider value, progress %).
- [ ] Disabled state announced and not focusable.
- [ ] Touch target ≥ 48×48dp (gate: `TestA11yGate_AllComponents`, `TargetOK`).

## Order & navigation

- [ ] Screen-reader order matches visual/logical order (`Tree.Order`).
- [ ] No hidden nodes leak into the order.
- [ ] FocusScope tab order == reader order for forms.
- [ ] Dialog/sheet traps focus until dismissed; focus returns to invoker.

## Display

- [ ] Dynamic type 100%→200%: no clipped Bangla matras, no overlap.
- [ ] Contrast AA (4.5:1) on light AND dark themes (`theme.AA`).
- [ ] Reduced transparency honored (`OpaqueColor`).
- [ ] Reduced motion: animations snap to end state (`ReducedMotion`).

## Gestures with reader on

- [ ] Tap/Drag/LongPress all reachable via reader actions.
- [ ] Scroll containers announce boundaries.
- [ ] IME composing (Avro/Gboard) announces committed clusters.

## Sign-off

Device / OS / reader version / tester / date per row in the release issue.
