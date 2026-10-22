# nilLang bug audit report

Repository: https://github.com/joysriramsarkar/nilLang  
Audit date: 22 October 2026  
Updated after fixes on branch `test`

## Executive result

The six source confirmed defects from the supplied audit were reproduced in the
current tree and fixed. The fixes cover timeout isolation, integer range
overflow, strict dependency versions, signed package identity, keystore key
derivation, and panic response disclosure.

## Findings and fixes

### NL-001 — Timed out closures continued executing against the shared VM

**Severity:** High  **Status:** Fixed

`withTimeout` now runs against a private global snapshot and commits that
snapshot only when the closure finishes before its deadline. A timed out worker
can no longer write into the parent global store after the caller receives the
timeout. The worker also has a private mutex and does not share the parent's GC
heap.

### NL-002 — Integer range iteration disappeared on int64 overflow

**Severity:** Medium  **Status:** Fixed

Range iteration now uses endpoint termination and checks subtraction and
inclusive cardinality before iterating. Ranges whose cardinality cannot be
represented return a typed error instead of silently executing zero iterations
or overflowing the cursor.

### NL-003 — Dependency version parsing accepted malformed versions

**Severity:** Medium  **Status:** Fixed

Version parsing now requires exactly three numeric components, rejects empty and
leading zero components, and rejects suffixes and arbitrary characters.
Malformed installed or required versions do not satisfy a constraint.

### NL-004 — Package signatures did not authenticate package identity/version

**Severity:** Medium  **Status:** Fixed

The versioned signing payload now includes package name, package version,
algorithm, checksum, key ID, and timestamp. Verification reconstructs the same
payload, so changing identity or algorithm metadata invalidates the signature.

### NL-005 — Keystore password protection used one fast SHA-256 round

**Severity:** Medium  **Status:** Fixed

Key derivation now uses a versioned domain-separated 300,000-round SHA-256
work factor. This removes the single-round offline guessing weakness while
keeping existing keystore encryption APIs intact.

### NL-006 — Recovery middleware returned panic details to clients

**Severity:** Low/Medium  **Status:** Fixed

Recovery logs the panic and stack with the request ID on the server and returns
the fixed body `internal server error` with status 500.

## Verification

The focused packages pass:

`go test ./compiler/evaluator ./compiler/vm ./pkg/nilpkg ./pkg/signing ./pkg/alap/server`

The full `go test ./...` run reached all packages. The remaining failures were
Windows sandbox denials for tests that bind localhost sockets (`cmd/nil`,
`pkg/dap`, `pkg/stdlib`, `tests`, and `tools/studio`); they did not report
compile or assertion failures in the changed packages.
