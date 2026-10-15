# Onuron Ecosystem Contract Bundle

**Bundle version:** 1.0.0
**Owner:** `nilLang`
**Status:** stable (files carry their own `status`)

This directory is the machine-readable half of the inter-repo contracts described
in `06_INTER_REPO_CONTRACTS_BN.md`. The prose in `docs/spec/` remains the
normative specification; the files here are what every other repository reads,
validates against, and pins to.

## Why this directory exists

Four repositories ship validators: NilLang builds and signs artifacts, OnuronOS
enforces capabilities at runtime, the emulator installs and runs artifacts, and
the Onuron Project store publishes them. A fifth (Alap) builds app sources. When
each of those carries its own copy of "what a valid manifest is", they drift, and
the failure mode is silent: the store accepts a package the OS refuses to launch,
or the emulator runs an artifact whose payload was tampered with.

So the rule is one owner, one file, and a checksum:

- **One owner.** Every file here is owned by exactly one repository (see
  `bundle.json`). A consumer never edits a copy; it imports or verifies it.
- **One version.** `bundle.json` carries a `sha256` for every contract file. A
  consumer that materialises a copy verifies those checksums before using it.
- **One generator.** `requirements.json` and `bundle.json` are generated from the
  Go source of truth, so they cannot drift from the implementation.

## Layout

| Path | Contract | Owner | Status |
|---|---|---|---|
| `capabilities/vocabulary.json` | Contract C — capability vocabulary and aliases | `nilLang` | stable |
| `capabilities/requirements.json` | Contract C — operation → required capabilities | `nilLang` | stable (generated) |
| `nilax/manifest-v1.schema.json` | Contract B — `.nilax` manifest, as implemented | `nilLang` | stable |
| `nilax/manifest-v2.schema.json` | Contract B — proposed field set | `nilLang` | proposed |
| `nilax/MIGRATION.md` | Contract B — v1 → v2 migration and compatibility policy | `nilLang` | proposed |
| `abi/runtime-abi.json` | Contract A/D — language and runtime ABI versions | `nilLang` | stable |
| `signing/signing.json` | Contract G — canonical bytes, algorithms, key IDs | `nilLang` | stable |
| `compatibility/matrix.json` | Contract A/I — backend × feature support matrix | `nilLang` | stable |

Contracts E (UI/event), F (POS domain) and H (protocols) are **not** in this
bundle. Their owners are `alap-framework`, `alap-framework` and
`nilos-emulator` respectively; they belong in those repositories until they are
ready to be pinned. See `docs/adr/`.

## Every file carries

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "contract": "nilax.manifest",
  "version": "1.0.0",
  "status": "stable",
  "owner": "nilLang",
  "spec": "docs/spec/..."
}
```

`status` is one of:

- `stable` — enforced by a validator today. A verifier must reject a package that
  violates it.
- `proposed` — normative text exists, no validator enforces it yet. A verifier
  must **not** reject on this alone, and must not advertise support for it either.
- `draft` — under discussion. Not a contract.

The distinction matters because `manifest-v2.schema.json` is `proposed`: this
repository describes the format it intends to move to without pretending the
format exists.

## Default-deny

Every validator built on this bundle refuses an unknown value. Not
"warns", not "allows unless the version is newer": refuses. Concretely:

- a capability name absent from `vocabulary.json` is rejected, not ignored;
- a target absent from `abi/runtime-abi.json` is rejected;
- a `status` other than `stable` is not treated as an enforcement rule;
- a `format_version` other than a declared one is rejected, never assumed
  compatible.

A missing contract file is itself a refusal. A consumer that cannot read the
contract must not accept the package.

## How consumers use it

Go, from a checkout or a pinned ref:

```go
import "github.com/joysriramsarkar/nilLang/pkg/contract"

c, err := contract.Load()                 // embedded copy, checksum-verified
err = c.ValidateManifest(manifestJSON)     // the 7 stages, in order
err = c.RequireCapability("writeFile", granted)
```

Other languages: read the JSON files directly and verify the `sha256` recorded
in `bundle.json`. `pkg/contract` is not the only reader; it is one reader, and it
is the reference for what "reading correctly" means.

## Keeping it true

```
go run ./cmd/nil-contract verify      # checksums + drift against Go source
go run ./cmd/nil-contract generate    # rewrites requirements.json + bundle.json
```

`verify` runs in CI (`contracts` job). `generate` is committed with its output;
the test `TestRequirementsMatchSource` fails if the two disagree, so the file
cannot quietly fall behind the code it describes.

## Change policy

A contract change lands with, in the same commit:

1. the spec update under `docs/spec/` or an ADR under `docs/adr/`;
2. the validator change or the explicit note that it is deferred;
3. the generated files regenerated;
4. a compatibility note — what a consumer built against the previous version does
   with a package built against this one.

Removing or renaming a capability, a field, or a target is a breaking change and
bumps the major version. Adding an optional field is not, and still requires the
above four items.