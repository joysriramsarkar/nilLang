# Nilang Standard Library — initial tree

This directory is intentionally split into:

## Implemented now

These modules only depend on currently available Nilang primitives/builtins:

- `core.nil`
- `math.nil`
- `strings.nil`
- `collections.nil`
- `testing.nil`
- `io.nil`
- `fs.nil`
- `process.nil`
- `time/instant.nil`
- `time/duration.nil`
- `time/date.nil`
- `net/url.nil`
- `data/csv.nil`
- `text/encoding.nil`
- `text/unicode.nil`
- `concurrency/task.nil`
- `concurrency/channel.nil`

## Host-backed contracts

These files deliberately do not pretend that the current runtime already
provides secure/native support:

- `data/json.nil`
- `data/base64.nil`
- `net/http.nil`
- `net/socket.nil`
- `crypto/hash.nil`
- `crypto/hmac.nil`
- `crypto/random.nil`
- `concurrency/sync.nil`

Those APIs need native runtime primitives and, eventually, Nilang capability
checks.

## Important integration rule

The standard library should be resolved as `std/*` by the module resolver.
Do not add new language syntax just for standard library functionality.

The Nilang language specification currently treats generics, Optional, Result,
effects/capabilities, and the canonical AST→typed HIR→MIR backend as planned
rather than frozen 0.1 guarantees. Standard-library APIs therefore avoid
depending on those features.
