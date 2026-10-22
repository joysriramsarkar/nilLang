# NilLang Alap — Canonical Implementation

This package (`nilLang/pkg/alap`) is the **canonical** Alap implementation for the
Onuron ecosystem. All new features, bug fixes, and API changes land here first.

Other implementations:
- `alap-framework/` (Go) — legacy standalone framework; kept for backward
  compatibility but should not gain new features. See its README for migration.
- `onuronOS/alap/` (Rust) — OS-side runtime; mirrors the canonical API but
  may lag. Track drift in `onuronOS/docs/LANGUAGE_AND_RUNTIME_CONTRACT.md`.

When in doubt, treat this package as the source of truth for the Alap API.
