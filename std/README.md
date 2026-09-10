# Nilang stdlib integration patch

This bundle is the implementation package for the next stdlib step.

## What it adds

1. Embedded `std/<module>` source loading.
2. A stable host-native call namespace.
3. Correct JSON conversion for Null/Bool/Int/Float/String/Array/Hash.
4. Base64 encode/decode.
5. SHA-256/SHA-512.
6. HMAC-SHA256.
7. CSPRNG bytes using `crypto/rand`.
8. HTTP GET/POST with a bounded response and timeout.
9. Standard-library bundle tests.
10. Pure Nilang task/channel/url wrappers.

## Required wiring

The current repository already has a `NativeCallHandler` and
`CapabilityChecker` on the VM. Wire the stdlib host dispatcher into that
handler, and map the constants in `pkg/stdlib/native.go` to the capability
names in the VM policy.

The source wrappers use a `native(...)` primitive. If the current parser/
compiler does not yet expose `native` as a language-level call, wire these
modules directly as registered native modules instead; do not add arbitrary
host calls to ordinary user programs.

## Security

- HTTP is capability-gated as `std.net`.
- Crypto and randomness are capability-gated as `std.crypto`.
- JSON itself can be deterministic and need not require a network/filesystem
  capability.
- Never expose a general shell/FFI escape through `native`.
- Enforce a response-size limit and request timeout.
- Keep capability checking before the host call.

## Important

This is intentionally a patch bundle rather than an automatic push. The
GitHub connection available to this session denied repository write access
(HTTP 403), so no claim is made that these changes have been committed to
the user's repository.
