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

## Phase 0 (roadmap §6 — নির্ভরযোগ্য ভিত্তি)
বিস্তারিত: `../onuron_ecosystem_plan/PHASE0_STATUS_BN.md`

1. **Exit condition release blocker** — `tests/p0_exit_test.go`; উপরের প্রতিটি item-এর evidence থাকতে হবে এবং CI-তে চালু থাকতে হবে।
2. **একই gate সব path-এ** — `pkg/compiler/entry_rejection_test.go`-এ ৮টি entry point × ৮টি invalid program, diagnostic **code** সমানতাসহ; `compiler/frontend/modules.go` দিয়ে import-এর module-ও check হয় (আগে হতো না)।
3. **Capability matrix** — `tests/capability_matrix_test.go`; requirement table থেকে generated, তাই পিছিয়ে থাকতে পারে না।
4. **Task/GC স্থিতিশীলতা** — `tests/concurrency_spec_test.go`; `CONCURRENCY.md`-এর প্রতিটি ধারার বিপরীতে পরীক্ষা + GC stress। Process-global task scope সরানো হয়েছে (`evalTaskRoot` → environment)।
5. **Toolchain pin ও reproducible build** — `../onuron_ecosystem_plan/TOOLCHAIN_PINS.md`; CI-তে `reproducible-build` job।

## P1
- Typed HIR-ই একমাত্র ইনপুট; SSA/CFG opt (fold/prop/DCE) + parity। (`compiler/hir|mir|lower`)
- Runtime hardening: typed VM error, GC root, task cancel/timeout/panic spec, capability proof, import cycle/init-order। (`compiler/vm/`, `runtime/gc/`, `docs/spec/CONCURRENCY|MEMORY_MODEL|CAPABILITIES|EFFECTS|MODULE_SYSTEM`)
- VM/WASM parity matrix, `nil fmt --check` + LSP, `nil.lock` + signing policy, stdlib stabilize, bootstrap reproducibility।

## Phase 1 — চুক্তি (cross-repo contract)
- `contracts/` — machine-readable contract bundle: capability vocabulary + requirement table, `.nilax` manifest schema (v1 stable, v2 proposed), runtime ABI, signing, compatibility matrix. Checksummed in `contracts/bundle.json`।
- `pkg/contract/` — Go reader + ৭ ধাপের default-deny validator (archive → identity → integrity → trust → compatibility → capability → install)। `pkg/nilpkg` verifier এখন এটি চালায়।
- `docs/adr/0001..0008` — পাঁচ রিপোর ownership/contract সিদ্ধান্ত, বিকল্প, migration, evidence ও reversal condition সহ।
- `cmd/nil-contract` — `generate` (requirement table + signing vectors + index) / `verify` (checksum + drift) / `validate` (.nilax)।
- বাকি: Contract E (UI/event) Alap-এ, Contract F (POS) Alap-এ, Contract H (protocol) emulator/OS-এ — owner repo-তেই থাকবে।

## DoD
- `go test ./...` সবুজ (Linux/macOS/Windows); fuzz/watch-তালিকার টেস্ট স্থিতিশীল; স্পেক-বদলে conformance ফিক্সচার একসাথে।

## এই সেশনে
- [x] 13 Oct: `go test ./compiler/... ./pkg/...` সবুজ (Phase 0)।
- [x] 13 Oct: P0.1 grammar table (`compiler/parser/grammar_table_test.go`, commit `5afc0ce`)।
- [x] 13 Oct: P0.2 entry rejection (`pkg/compiler/entry_rejection_test.go`, commit `164dc02`)।
- [x] 13 Oct: P0.3 sequence narrowing + Any audit (commit `bed3844`)।
- [x] 13 Oct: P0.4 scope freeze — block shadowing (checker+evaluator+VM), app transparency, per-pass loop bindings (fresh cells), unified scope corpus — commit `8d9eef3`।
- [x] 13 Oct: P0.6 never-panics fuzz (commit `8c60dba`) + P1 scoped MIR lowering (commit `1e4a633`)।
- [x] 14 Oct: P0.7 diagnostics standardize (commit `65331d8`) — নিচে।
- [x] 14 Oct: P0.8 CI gate — conformance matrix + dx-gate (fmt/rejection) + fuzz-smoke (commit `ffe94d3`)।
- [x] 14 Oct: P1.4 typed VM errors — নিচে।
- [x] 14 Oct: Phase 1 contract bundle — `contracts/`, `pkg/contract`, `cmd/nil-contract`, `docs/adr/0001..0008`। `pkg/nilpkg` verifier এখন contract চালায়; CI-তে `contracts` job।
- [x] 15 Oct: Phase 0 — P0 exit gate, entry-point gate matrix (import module check সহ), capability matrix, concurrency spec conformance + stress, toolchain pin ও reproducible build।
