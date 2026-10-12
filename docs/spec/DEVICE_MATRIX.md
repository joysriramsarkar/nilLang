# DEVICE_MATRIX — on-device test plan (L8.10)

> Status: emulator job defined; physical-device runs are manual until CI
> runners with hardware exist.

## Emulator CI job

```yaml
# .github/workflows/device.yml (to be added with real runners)
- Android emulator (API 29, 31, 34) × ABIs (arm64-v8a, x86_64):
    1. `nil build android`
    2. `adb install app.apk` (assert <10MB via --size)
    3. launch `NilangActivity`, assert `nativeInit == 0`
    4. push `app.nilax`, assert bundle runs to `nil_quit`
    5. lifecycle: pause/resume/destroy, assert state machine log matches
- iOS simulator (macOS runner only): install `.ipa`, same assertions
- HarmonyOS emulator (DevEco image): install `.hap`, same assertions
```

## Matrix

| Platform   | Target            | Min OS   | ABIs              | Install artefact | Size gate |
|------------|-------------------|----------|-------------------|------------------|-----------|
| Android    | emulator + device | API 24+  | arm64-v8a, x86_64 | `.apk`           | <10MB     |
| iOS        | simulator+device  | iOS 17+  | arm64             | `.ipa`           | <25MB     |
| HarmonyOS  | emulator          | API 9+   | arm64             | `.hap`           | <10MB     |

## What is asserted on-device

1. Install + cold start under 2s (emulator) / 1s (device).
2. Bundle executes and renders first frame.
3. Lifecycle transitions (L8.6) survive pause/resume without state loss.
4. Permission denial (L8.7) degrades to stub data, never crashes.
5. Reproducible bundle (L8.9): hash on device equals CI hash.
