# MOBILE_RUNTIME — Nilang mobile native core (L8.2/L8.3)

> Status: scaffold + ABI frozen. `.so` artefacts are built by CI/NDK, never
> checked in.

## Architecture (ADR-0003)

Two tiers. The full `nil-runner` (~25MB with the Go runtime) stays on
desktop/CI. Phones ship `libnilang_core.so` (~2–4MB target): GC + strings +
bytecode VM behind the `runtime/core/include/nil_runtime.h` C ABI:

```
nil_init / nil_run_bundle / nil_dispatch_event / nil_set_callback / nil_quit
+ NilRuntimeString (ptr,len) table + nil_ffi_abi_v1 link guard
```

Hosts (Android JNI `pkg/mobile/android`, iOS ObjC bridge `pkg/mobile/ios`,
HarmonyOS NAPI `pkg/mobile/harmonyos`) call only that header.

## Building libnilang_core.so

Prerequisites: Rust toolchain + Android NDK (for Android) or Xcode (for iOS).

```sh
# Host check (Linux/macOS)
cargo build --release -p nilang_native

# Android arm64 (primary) and x86_64 (emulator)
rustup target add aarch64-linux-android x86_64-linux-android
cargo ndk --target aarch64-linux-android --platform 24 build --release
cargo ndk --target x86_64-linux-android  --platform 24 build --release
# Output: target/<triple>/release/libnilang_core.so

# iOS (requires macOS + Xcode): see pkg/mobile/ios (XCFramework stub)
rustup target add aarch64-apple-ios
cargo build --release --target aarch64-apple-ios
```

Copy the results into the embedder:

```
pkg/mobile/android template: lib/<abi>/libnilang_core.so   # arm64-v8a, x86_64
pkg/mobile/ios scaffold:     NilangCore.xcframework/ios-arm64/libnilang_core.a
```

## Size budget (M4)

APK target: **<10MB**. Measure with:

```sh
nil build android --size
```

Breakdown is printed by `pkg/mobile/android.SizeReport`: `native-libs`,
`nilax-bundle`, `dex`, `resources`, `other`. If over budget: strip debug
symbols (`-s -w`), ship per-ABI APKs, or compress the `.nilax` bundle.

## Go fallback

When cgo is off (all mobile cross-builds), `runtime/vm/bridge_nocgo.go`
answers time/sleep/math/device in pure Go with the same `Call` names as the
cgo bridge. The phone artefact never uses either bridge directly — it uses
the C ABI above.
