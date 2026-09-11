# NilLang Capability Security Specification
**Version:** 1.0.0-draft  
**Status:** Authoritative Normative Specification  
**Conformance Level:** Nilang 0.1 Core Freeze

---

## 0. Implementation Boundary

Implemented today: `nil.json` capability declaration, parsing and validation of
those declarations (`ProjectConfig.ValidateCapabilities`), and the profile
reporting in `nil profile` (`capability.CapabilityMatrix`).

Not implemented, and therefore **not** a Nilang 0.1 guarantee:

- The `requires` clause in §4.1 and diagnostic `E0201` do not exist in the
  grammar or the type checker.
- The runtime gate in §4.2 is inert: the compiler never emits `OpNativeCall`,
  and `vm.CapabilityChecker` is only ever assigned in tests. A `native("...")`
  call is dispatched through the ordinary `native` built-in, so host operations
  such as `std.db.exec`, `std.http.get`, `exec`, `writeFile`, and `removeFile`
  currently run without any grant check.
- Effect checking in pure contexts is inert as well: the checker's pure-context
  flag is never enabled, so `E0202` cannot fire.

Everything in §4 is a draft design target. Per `CORE_FREEZE.md` it must not be
presented as an enforced guarantee until it is wired into both engines with
conformance coverage.

---

## 1. Overview & Threat Model

Capabilities form the security boundary between NilLang application code and host operating system resources (especially within Onuron OS and Alap mobile/web runtime).

### Principle of Least Privilege:
An application or package has zero ambient authority to access system resources. Every privileged operation requires an explicit, statically auditable capability grant.

---

## 2. Standard Capability Namespaces

```text
filesystem.read      filesystem.write      network
camera               microphone            location
process              database              sensors
bluetooth            crypto                gpu
```

---

## 3. Project Manifest Declarations (`nil.json`)

Applications explicitly declare their capability requirements in `nil.json`:

```json
{
  "name": "pos-terminal",
  "version": "1.0.0",
  "capabilities": [
    "network",
    "database",
    "filesystem.read",
    "filesystem.write"
  ]
}
```

---

## 4. Compile-Time & Runtime Dual Enforcement

### 4.1 Compile-Time Gate
Functions declaring capability requirements:
```nil
fn syncDatabase() requires network, database {
    // ...
}
```
If `pos-terminal` does not declare `"network"` in `nil.json`, compilation terminates immediately with diagnostic:
```text
error[E0201]: capability 'network' required by 'syncDatabase' is not granted in nil.json
```

### 4.2 Runtime Sandboxing Gate
When executing compiled bytecode:
1. `OpNativeCall` verifies the target host operation against the authorized capability token table.
2. If authorization is absent or revoked by the user, the runtime terminates the call with `CapabilityDeniedError`.
3. Bytecode cannot forge or bypass capability tokens.