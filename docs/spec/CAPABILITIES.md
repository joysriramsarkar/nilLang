# NilLang Capability Security Specification
**Version:** 1.0.0-draft  
**Status:** Authoritative Normative Specification  
**Conformance Level:** Nilang 0.1 Core Freeze

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