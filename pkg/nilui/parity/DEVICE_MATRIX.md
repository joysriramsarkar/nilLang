# Device Matrix (U14.8)

Parity + golden suites run in CI (software raster). Device runs are manual
per release until device-farm CI lands.

| Device | OS | Backend | Suites | Owner |
|---|---|---|---|---|
| Pixel 6a (mid-range) | Android 14 | Vulkan/GLES | parity, layout, a11y gate, perf 60Hz | release captain |
| Moto G (low-end) | Android 12 | GLES | perf 60Hz, 48dp, TalkBack checklist | release captain |
| iPhone SE | iOS 17 | Metal | parity, VoiceOver checklist | release captain |
| MatePad | HarmonyOS 4 | XComponent | parity subset, checklist | release captain |
| Linux CI | — | Software | FULL (goldens ≥200, bn ≥50, unit, perf) | CI |

Frame budget gate: mid-range Android p95 ≤ 16.7ms (60Hz); 120Hz panels ≤
8.3ms where claimed. Numbers recorded in the release issue.
