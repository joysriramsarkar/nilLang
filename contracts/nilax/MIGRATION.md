# `.nilax` manifest: v1 → v2 migration and compatibility policy

**Contract:** B (`.nilax` package format)
**Status:** proposed — v2 is not implemented anywhere yet
**Owner:** `nilLang`

## Why v2 exists

`manifest-v1.schema.json` documents the manifest a `nil build` produces today. It
is honest about what it is — a working format with five named gaps:

1. no `source_revision`, so a published package does not identify its source;
2. no `runtime_abi`, so a package cannot state which runtime it needs;
3. no `payload_checksums`, so per-file integrity depends entirely on a verifier
   that already holds the expected whole-archive digest;
4. `signature` is a bare string here and a structured object in
   `pkg/signing`, which is two representations of one fact;
5. `app_name` doubles as an application id.

Gap 3 is the security one. Today the only thing that ties a `.nilax` file to the
bytes someone approved is the whole-file checksum in the signature. A store that
records a size and a digest can detect substitution; a store that records neither
cannot detect anything. v2 makes per-file digests part of the signed manifest, so
tampering with one payload entry is visible without re-signing the package.

## Compatibility policy

A verifier built against this bundle must behave like this for any `format_version`:

| manifest `format_version` | Behaviour |
|---|---|
| 1 | Validate against `manifest-v1.schema.json`. Accepted when every stable rule passes. |
| 2 | Validate against `manifest-v2.schema.json`. |
| anything else, including a future 3 | **Refuse.** Not a warning, not a "try it anyway". |

A verifier that does not understand a version says so. It does not fall back to
reading the fields it recognises, because a partial read of a manifest is
indistinguishable from a complete one at the point where the decision is made.

Within a major version, **adding an optional field is not breaking** and
`additionalProperties` stays `false` only because an unknown field in a manifest
we sign is a mistake we want to hear about. Before a field ships it must appear
in `properties`, in the schema, and in the `signed_fields` list of any signature
format that covers it.

## v1 → v2 field mapping

| v1 | v2 | Note |
|---|---|---|
| `format_version: 1` | `format_version: 2` | |
| `app_name` | `app_id` | `app_name` was already a reverse-DNS id in the store; it becomes typed rather than renamed in practice. |
| `app_version` | `version` | |
| `entry_bytecode` | `payload.entrypoint` | |
| `targets` (array) | `target` (single) | A v2 package is single-target. A multi-target v1 package produces one v2 package per target. |
| `permissions` | `capabilities` | Same values, renamed so the field stops claiming to be a permission when it is a capability. |
| `dependencies` (map) | `dependencies` (array) | A map cannot carry a per-dependency checksum. |
| — | `source_revision` | New. Builders that cannot determine a revision must emit `sha256:<digest>` over the source tree, or the package is not publishable. |
| — | `runtime_abi` | New, from `abi/runtime-abi.json`. |
| — | `payload.type`, `payload.backends` | New. |
| — | `payload_checksums` | New. |
| `signature` (string) | `signature` (object) | New structure. See below. |
| `metadata` | `metadata` | Unchanged. |
| `author`, `description` | — | Dropped. Identity belongs in the store's own metadata, not in the signed artifact, so that a signature does not need re-issuing when a description is corrected. |
| `compiler_version` | `compiler.name` + `compiler.version` + `compiler.revision` | The revision is what actually distinguishes two builds of the same version. |
| `created_at` | — | Dropped from the signed manifest. It stays in the store record. A timestamp inside the signed bytes is a reproducibility hazard and buys nothing a verifier uses. |

## Signature migration

v1 signatures sign a `SigningPayload` of `{checksum, timestamp, key_id}` where
`checksum` is the SHA-256 of the whole `.nilax` file. That payload is specified
in `signing/signing.json` and is unchanged — an existing v1 signature keeps
verifying, because v1 was never wrong, only under-specified.

v2 signs a canonicalised manifest with `signature` itself excluded, plus the
payload checksums. Two implementations agree when they produce byte-identical
canonical bytes; `signing/signing.json` states how.

## Sequencing

v2 is not shipped in one step. The order below matters, because each step is
independently useful and each step that flips a default needs its consumers
ready first.

1. **Publish the schema.** Done by this change. No validator enforces it yet.
2. **Build v2 packages alongside v1.** `nil build --format 2` writes both; the
   v1 output stays the default.
3. **Teach verifiers to accept both**, still defaulting to v1, and record which
   they saw. OnuronOS, the emulator and the store each do this in their own
   release.
4. **Flip the default** to v2 once every verifier in §11 of the contracts
   document accepts it.
5. **Retire v1** only after a published support window, and only with an ADR.
   v1 packages already signed must keep installing; a retired format that refuses
   old packages breaks every device that has one.

## What is deliberately not decided here

- Whether `payload_checksums` is a flat map or per-directory. Flat is simpler and
  the archive is flat; it stays flat unless an archive needs one.
- Whether `source_revision` is a VCS sha or a digest in general. The schema
  accepts either and the store records which.
- Whether `metadata` is ever signed. Today it is not, and a consumer must not
  treat metadata as trusted.