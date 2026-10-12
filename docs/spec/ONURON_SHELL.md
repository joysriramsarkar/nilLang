# Onuron Shell Note (U16.6, with language plan L8)

The Onuron OS shell hosts Alap UI through the same embedder ABI as Android
(`pkg/nilui/embedder`: `alap_node_create/measure/arrange/paint/destroy` +
event callback). Shell specifics:

- **Compositor:** Onuron compositor owns final pixels; Alap submits the
  retained display list per frame (see `pkg/nilui/render`).
- **Backend:** Vulkan preferred (`pkg/gpu` detection via `NIL_VULKAN=1`);
  software rasterizer is the fallback for headless/test shells.
- **Insets:** status bar + gesture bar arrive as `Insets{SafeTop,
  SafeBottom}`; the on-screen keyboard adds `Keyboard` (see embedder test).
- **Theme:** Onuron dark roles are the system default
  (`theme.DarkRoles`); apps override locally per UI_SPEC §8.
- **Language side:** shell services bind through the L8 native adapters;
  UI keeps no OS-specific branches — platform differences live in adapters
  and `SUPPORT_MATRIX.md`, never in component code.
