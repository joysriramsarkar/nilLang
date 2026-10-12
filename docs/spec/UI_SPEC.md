# UI_SPEC.md — Alap UI Contract (Single Source of Truth)

> **Status:** normative for `pkg/alap/ui` + `pkg/nilui/*`.
> **ADRs:** ADR-0001 (native adapter + shared logic; software rasterizer secondary),
> ADR-0005 (single keyed-patch runtime), ADR-0006 (`pkg/alap/ui` is the only model).
> Master plan: `পরিকল্পনা/মাস্টার-প্ল্যান-২-আলাপ-ইউআই.md` (U0–U16).

## 1. Architecture

```
Nilang source → evaluator → component Hash → convertHashToComponent (cmd/nil)
  → ui.Page (pkg/alap/ui, the ONLY model)
  → RenderSSR (HTML + state hydration) → browser
  → events POST /__alap/event → Dispatch → re-render → PATCH JSON or HTML
  → AlapRuntime.applyPatches (pkg/alap/runtime/alap-runtime.js, canonical)
```

- `pkg/alap/component.go` + `renderer.go` are **deprecated shims** delegating to `ui`.
- `pkg/alap/state` `DOMPatch` vocabulary is retired; `ui.Patch` is canonical.
- `pkg/nilui` holds the typed IR, layout, rasterizer, text, input, theme,
  components, router, a11y, embedder, devtools subpackages.

## 2. VDOM & Patch Contract (U0.2/U0.3)

- `VNode{Type, Tag, Key, ID, Attrs, Events, Children, Text}`.
- Patch types: `REPLACE, PROPS, TEXT, INSERT_CHILD, REMOVE_CHILD, MOVE`.
- `Diff` is keyed when **all** children on both sides carry keys: LIS-based
  reorder in O(n); pure shuffle emits only `MOVE`, never `REPLACE`.
- JS `applyPatches(root, patches)` mirrors Go semantics 1:1, including `MOVE`
  (remove-then-insert at index) and indexed `INSERT_CHILD`.

## 3. UI IR (U1) — `pkg/nilui/ir`

`Node{Type, Key, Props, Slots, Children, Events, Modifiers, Span, Version}`.
Validation codes `UI0101–UI0150` (see §3.4). Stable keys required in lists;
duplicate key → `UI0103`. Serialization `.nilax` (JSON) must round-trip.

### 3.4 Diagnostic registry (U1.6)

| Code | Meaning |
|---|---|
| UI0101 | unknown component type |
| UI0102 | missing required prop |
| UI0103 | duplicate key in list |
| UI0104 | wrong child type for slot |
| UI0105 | unknown prop |
| UI0106 | bad prop type |
| UI0107 | missing key in list child |
| UI0108 | bad event binding |
| UI0109 | version mismatch |
| UI0110 | span/source lost |
| UI0140 | bridge: unbound Nilang component type (cmd/nil) |

## 4. Reactivity (U2)

Nilang builtins: `signal(v)`, `computed(fn)`, `effect(fn)`,
`reactiveArray(...)`, `store({...})`. Semantics: glitch-free push-pull with
batching, priority `high/normal/low`, `dispose()` teardown. Go core:
`pkg/nilui/reactive` + `pkg/alap/state` (Effect.stop honored).

## 5. Layout (U3) — `pkg/nilui/layout`

`RenderBox{Measure(Constraints)→Size, Arrange(Rect)}`. Constraints
min/max, tight/loose, unbounded. Units dp/sp/pt/%/intrinsic. Box model
(margin/padding/border). Column/Row(flex), Stack, Grid, Scroll, intrinsic,
overflow/clip rules, RTL logical directions (`start/end`).

## 6. Render pipeline (U4) — `pkg/nilui/render`

View→Element→Render trees, dirty marking with relayout boundaries, retained
display list, software rasterizer (real RGBA pixel buffer; golden tests
font-independent), backend interface Vulkan/Metal/GLES/Software, frame budget
16.7ms@60Hz / 8.3ms@120Hz measured per frame.

## 7. Components (U7) — 54 catalog

Foundation(6): Page, Text, RichText, Box, Spacer, Divider.
Layout(11): Column, Row, Stack, Grid, ScrollView, SafeArea, Padding, Align, SizedBox, Flex, Layout.
Input(11): Button, IconButton, TextField, SearchField, PasswordField, Checkbox, CheckboxGroup, RadioGroup, Switch, Slider, Stepper.
Display(9): Image, Icon, List, LazyColumn, LazyRow, DataGrid, ProgressIndicator, ActivityIndicator, Badge.
Feedback(6): Dialog, Modal, BottomSheet, Snackbar, Tooltip, Popover.
Navigation(6): NavigationStack, AppBar, BottomNavigation, TabBar, PageView, Route.
Alap-special(5): Canvas, Chart, SemanticGroup, FocusScope, AnimatedContainer.

Each: contract (props/slots/modifiers/events/states), theme-aware, lifecycle
(mount/unmount/dispose), a11y label, test.

## 8. Theme (U8)

Token layers primitive→semantic→component; light/dark roles; sp type scale;
4/8/12/16/24/32 spacing; shape/elevation; resolution order
element→parent→app→system; dynamic-color interface; WCAG AA 4.5:1 enforced;
adaptive Cupertino/Material/Harmony variants.

## 9. Animation (U9)

`timeline.go` values are **applied** to bound targets every tick. Tick-based
cancellable scheduler (no blocking Sleep on caller), Sequence/Parallel
composition, implicit, transition, spring (critically damped), keyframe,
stagger, hero/shared, reduced-motion kill-switch.

## 10. a11y gates (U11)

Every component exposes role/label/hint/value/state in the semantic tree;
minimum touch target 48×48dp; contrast AA; reduced-motion honored. Gate test
`TestA11yGate_AllComponents` must pass.

## 11. i18n (U12)

`pkg/alap/i18n` catalog + `LocalizeTree` bridge into IR trees; bn-BD locale
(date/number/currency via `textbn`), RTL flag, platform-locale interface.

## 12. Embedder (U13) — C ABI

```c
alap_node_t* alap_node_create(const char* type, const char* key);
void alap_node_measure(alap_node_t*, float min_w, float min_h, float max_w, float max_h);
void alap_node_arrange(alap_node_t*, float x, float y, float w, float h);
void alap_node_paint(alap_node_t*, alap_paint_ctx_t*);
void alap_node_destroy(alap_node_t*);
typedef void (*alap_event_cb)(const char* node_id, const char* event, const char* payload);
```

Android (Compose host) / iOS (SwiftUI host) / HarmonyOS (XComponent host)
adapters; insets; native escape hatch; support matrix in
`pkg/nilui/embedder/SUPPORT_MATRIX.md`.

## 13. Tests (U14)

Unit per component · layout ≥100 · golden ≥200 raster-deterministic ·
Bengali golden ≥50 · semantic/a11y per component · lifecycle · parity suite
(core × 4 backends) · device matrix doc · flakiness policy (no time-based
asserts, fixed seeds, tolerance-based pixel compare) · perf budget CI test.

## 14. Devtools (U15)

Hot reload preserving state (SourceWatcher + replaceAllApps), inspector
(tree/layout/signals), formatter+lint (`nilfmt` UI rules), `@preview`
annotation, browser preview, Studio link.

## 15. Examples (U16)

POS, music DB, typing practice, settings, search/loading/retry,
master-detail, counter→production, foldable adaptive — all runnable under
`examples/`.
