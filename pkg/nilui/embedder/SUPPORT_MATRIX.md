# Feature Support Matrix (U13.7)

Declared capabilities per host. Anything not listed here is NOT supported —
check this file, not tribal knowledge.

| Feature | Android (Compose) | iOS (SwiftUI) | HarmonyOS (XComponent) | Web (SSR) |
|---|---|---|---|---|
| View→Element→Render trees | ✅ | ✅ | ✅ | ✅ |
| Keyed patch apply (MOVE) | ✅ | ✅ | ✅ | ✅ |
| Software rasterizer / preview | ✅ | ✅ | ✅ | ✅ (golden) |
| 54-component catalog | ✅ | ✅ | ✅ | ✅ |
| Bengali shaping (clusters) | ✅ | ✅ | ✅ | ✅ |
| IME composing (Avro/Gboard) | ✅ | ⚠️ Gboard only | ⚠️ Celia only | ✅ |
| Haptics | ✅ | ✅ | ✅ | ❌ (stub) |
| Hero transitions | ✅ | ✅ | ⚠️ partial | ✅ |
| Dynamic color (Material You) | ✅ A12+ | ❌ | ❌ | ❌ |
| TalkBack / VoiceOver | ✅ TalkBack | ✅ VoiceOver | ⚠️ checklist only | ❌ |
| Deep links | ✅ intent | ✅ universal | ✅ app-link | ✅ URL |
| State restoration | ✅ | ✅ | ✅ | ✅ (session) |
| Native escape hatch | ✅ | ✅ | ✅ | ❌ |
| Hot reload (dev) | ✅ | ✅ | ✅ | ✅ |

Legend: ✅ supported · ⚠️ partial · ❌ not supported.
