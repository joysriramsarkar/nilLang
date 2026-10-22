# AOT / LLVM Lowering — Known Limitations (L-8)

The conformance suite (`tests/conformance`) skips a number of cases because the
AOT (LLVM) backend does not yet lower the following constructs. These are
**real compiler gaps**, not design skips, and are tracked here until implemented:

- `Ok` / `Err` / `Some` / `None` constructors — no LLVM lowering exists
  (`llvm: no lowering for a call to Ok/Err/Some/None`).
- Slot allocation for certain top-level constants — `loading "MAX", which has no slot`.
- AOT output is therefore **not** claimed as production-ready; JIT/VM path is
  the supported execution target.

Until these are implemented, conformance results must be read with the skip count
in mind (see `tests/conformance` output).
