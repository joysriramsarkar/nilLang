# nilLang — প্ল্যান (ক্যানোনিকাল ভাষা)

> মালিক: lexer/parser/types/typecheck/HIR/MIR/VM/stdlib/LSP/DAP/nil CLI/`.nilax`+signing লাইব্রেরি।
> সীমা: `CORE_FREEZE.md` সক্রিয় — শুধু mismatch/safety/diagnostic ফিক্স, নতুন ফিচার নয়।

## বর্তমান অবস্থা
- Dual engine: evaluator (REPL/dev) + bytecode compiler+VM (`nil run -- -vm`)। WASM/AOT/GPU-sim/SoftBus/nilpkg/nilkey আছে।
- ঝুঁকি-এলাকা: capability, task/future concurrency, VM native-op stub, FFI/AOT লিংকিং, DAP timeout (সাম্প্রতিক git log অনুযায়ী)।
- অসম্পূর্ণ (stub): `compiler/wasm` কিছু builtin, `compiler/vm` native-op, GPU real submit, incremental DOM, `runtime/vm/native` HAL।

## P0 (freeze-compliant, `docs/CORE_STABILIZATION.md` ক্রমে)
1. **P0.1 Grammar freeze** — EBNF বনাম lexer/parser/AST মিলানো; golden AST ফিক্সচার। (`docs/spec/`, `compiler/lexer|parser|ast`)
2. **P0.2 Single parse-and-check frontend** — `pkg/compiler` + `cmd/nil|nilc` + REPL এক পথে; failed-check-এ reject corpus। (`pkg/compiler`, `compiler/frontend|toolchain`)
3. **P0.3 Implicit Any দূর** — `Array<T>/Optional<T>` + tensor contract, `first/last/push` টাইট করা। (`compiler/types|typecheck`, `tests/typesystem|generics|functiontypes`)
4. **P0.4 Mutable closure capture** — `OpGetFree` read-only কপি → shared cell; checker/evaluator/VM unified corpus। (`compiler/compiler/`, `compiler/vm/vm.go`)
5. **P0.5–8 Conformance + never-panic + diagnostics + CI gate** — `tests/conformance|frontend_gate|architecture`, seeded fuzz (`parser|typecheck` fuzz), structured error code (`compiler/diagnostics`), Linux/macOS/Windows matrix।

## P1
- Typed HIR-ই একমাত্র ইনপুট; SSA/CFG opt (fold/prop/DCE) + parity। (`compiler/hir|mir|lower`)
- Runtime hardening: typed VM error, GC root, task cancel/timeout/panic spec, capability proof, import cycle/init-order। (`compiler/vm/`, `runtime/gc/`, `docs/spec/CONCURRENCY|MEMORY_MODEL|CAPABILITIES|EFFECTS|MODULE_SYSTEM`)
- VM/WASM parity matrix, `nil fmt --check` + LSP, `nil.lock` + signing policy, stdlib stabilize, bootstrap reproducibility।

## DoD
- `go test ./...` সবুজ (Linux/macOS/Windows); fuzz/watch-তালিকার টেস্ট স্থিতিশীল; স্পেক-বদলে conformance ফিক্সচার একসাথে।

## এই সেশনে
- [x] 13 Oct: `go test ./compiler/... ./pkg/...` সবুজ (Phase 0)।
- [x] 13 Oct: P0.1 grammar table (`compiler/parser/grammar_table_test.go`, commit `5afc0ce`) — EBNF §3 productions pinned; E0102 যাচাই করে দেখা গেছে আগে থেকেই implemented+tested (`typecheck.go:541`); বাকি P0.1 (E0102-এর বাইরে) সম্পূর্ণ।
- [ ] P0.2 single parse-and-check frontend (পরের আইটেম)।
