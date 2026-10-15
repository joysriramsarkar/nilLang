# NilLang Capability Security Specification
**Version:** 1.0.0-draft  
**Status:** Authoritative Normative Specification  
**Conformance Level:** Nilang 0.1 Core Freeze

---

## 0. Implementation Boundary

Implemented today:

- `nil.json` capability declaration, parsing and validation of those
  declarations (`ProjectConfig.ValidateCapabilities`), and the profile reporting
  in `nil profile` (`capability.CapabilityMatrix`).
- **The capability requirement table** (`compiler/capability/manifest.go`): an
  explicit declaration of what each builtin and `std.*` operation requires.
- **Compile-time instrumentation** (§4.2): `compiler/compiler` emits
  `OpNativeCall` for a direct, statically named call to a gated operation. Pure
  builtins keep the ordinary `OpCall`.
- **The runtime gate** (§4.2): the VM consults `CapabilityChecker` before every
  gated dispatch and refuses with a typed `*DeniedError` naming the operation,
  what it required, and what the program held.
- **A conservative refusal scan on the evaluator path**: `nil run` inspects the
  program before evaluation and refuses if a directly called gated operation
  lacks a grant.

Not implemented, and therefore **not** a Nilang 0.1 guarantee:

- The `requires` clause in §4.1 and compile-time diagnostic `E0201` do not exist
  in the grammar or the type checker. Refusals happen at run time, on the first
  gated call, rather than before the program starts.
- **Indirect calls are not gated.** `let f = writeFile; f("a.txt", "b");` is not
  recognised, because at the point of emission the call target is an ordinary
  local. Closing this needs either a `requires` clause or a first-class function
  value that carries its requirement with it; the latter is the design this
  specification prefers.
- The evaluator path (§4.4) sees direct calls only and cannot report a refusal
  site the way the VM does. **The VM path is the enforced one.**
- Effect checking in pure contexts is inert as well: the checker's pure-context
  flag is never enabled, so `E0202` cannot fire.

An operation absent from the requirement table is **refused** rather than assumed
safe. A false refusal is a visible bug; a false grant is a vulnerability.

Conformance evidence is listed in `tests/capability_enforcement_test.go` and
`compiler/capability/manifest_test.go`. Per `CORE_FREEZE.md` nothing here may be
presented as an enforced guarantee without that coverage.

### Machine-readable form

This document is the normative specification. Its machine-readable counterpart
is the contract bundle in `contracts/`:

- `contracts/capabilities/vocabulary.json` — the canonical names, the aliases,
  the recognised-but-ungated names, and the limitations in §0 as structured
  data;
- `contracts/capabilities/requirements.json` — **generated** from
  `compiler/capability/manifest.go`, so a verifier in another language consults
  the same table rather than transcribing it.

Both are checksummed in `contracts/bundle.json`. `pkg/contract` reads them and
enforces them; `go run ./cmd/nil-contract verify` fails if either drifts from the
code. When this document and that table disagree, the table is generated from
the code and this document is what needs correcting — which is a deliberate
asymmetry, because the table can be checked automatically and this cannot.

---

## 1. Overview & Threat Model

Capabilities form the security boundary between NilLang application code and host operating system resources (especially within Onuron OS and Alap mobile/web runtime).

### Principle of Least Privilege:
An application or package has zero ambient authority to access system resources. Every privileged operation requires an explicit, statically auditable capability grant.

---

## 2. Standard Capability Namespaces

The canonical names are lowercase and operation-oriented. They appear inline in
a manifest next to the code that needs them:

```text
core    stdout    stderr    stdin    clock    environment
filesystem       process        network       database
crypto           random         clipboard
device           sensors        camera        location
notifications    ui             async
```

Fine-grained sub-capabilities (`filesystem.read`, `filesystem.write`) remain a
Planned refinement. The requirement table declares whole-resource grants today,
so a program requesting filesystem access receives read and write together. That
is coarser than least privilege and is recorded here rather than glossed over:
sub-capability splitting is what the table would need to change first.

---

## 3. Project Manifest Declarations (`nil.json`)

Applications explicitly declare their capability requirements in `nil.json`:

```json
{
  "name": "pos-terminal",
  "version": "1.0.0",
  "capabilities": ["network", "database", "filesystem", "stdout"]
}
```

A missing manifest is not an error. A standalone script runs with no grants,
which under the VM path means its first native call is refused. That is the
correct default for an undeclared program rather than granting everything.

`nil run` resolves the manifest by searching upward from the file being run, so
a script inside a project inherits the project's grants regardless of the
working directory.

---

## 4. Compile-Time & Runtime Dual Enforcement

The grant rule both gates implement:

```
granted ⊇ required   →  run
otherwise            →  refuse, naming the operation and the missing capability
```

Holding extra capabilities is harmless; holding fewer is a refusal. An operation
requiring nothing is always allowed, which is what keeps pure computation usable
under a manifest that declares nothing.

### 4.1 Compile-Time Gate (Planned)

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

This clause does not exist in the grammar yet. `ast.Inspect` now exists to
support the static analysis that would back it.

### 4.2 Runtime Sandboxing Gate (Implemented)

**Instrumentation.** `compiler/compiler` emits `OpNativeCall` rather than
`OpCall` for a direct, statically named call to a gated operation. The operation
name travels in the constant pool, so a disassembly shows which call is made
(`vm.DisassembleWithConstants` renders those operands):

```
writeFile("a.txt", "b");   →  OpConstant, OpConstant, OpNativeCall writeFile 2
len([1, 2, 3]);            →  OpConstant, OpGetBuiltin, OpCall 1
```

**Dispatch.** On `OpNativeCall` the VM:
1. Consults `CapabilityChecker`; a refusal returns a typed `*DeniedError`.
2. Dispatches to an embedder's `NativeCallHandler` if one is installed,
   otherwise to the language's own builtin.
3. Treats an operation with no implementation as an error rather than a silent
   null.

Step 2's fallback is what makes gating transparent: when instrumentation was
introduced, the missing builtin fallback turned every gated builtin into a no-op
returning null.

A refusal renders as:

```text
❌ E0201: capability denied: writeFile requires [filesystem]: capability not granted
   = help: add the filesystem capability to the manifest, for example {"capabilities": ["filesystem"]}
```

Bytecode cannot forge or bypass capability tokens: the operation name is a
constant, not a computed value, so a program cannot ask for a different
operation at run time.

### 4.3 Manifest Vocabulary

Two spellings are accepted and normalised to a canonical lowercase form.
Operation-oriented names (`filesystem`, `stdout`) appear inline in manifests;
resource-oriented names (`Filesystem`, `GPS`) come from `pkg/capability`. Matching
is case insensitive and ignores `-`, `_`, and spaces, so `"Filesystem"`,
`"file-system"`, and `"file_system"` are the same capability, and `gps` aliases
onto `location`.

### 4.4 The Evaluator Path

The tree-walking evaluator has no call-site gate: builtins are Go function values
dispatched inside a switch, with nothing to intercept. Rather than leave that
path ungated, `nil run` inspects the program before evaluation and refuses if a
directly called gated operation lacks a declared capability.

This scan is weaker than §4.2 and documented as such: it sees direct calls only,
not calls through a function value, and it runs before evaluation so it cannot
report a first refusal site. **The VM path is the enforced one**, and the refusal
message tells the user to use `nil run --vm`.