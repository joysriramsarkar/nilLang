# আর্কিটেকচার — চূড়ান্ত অবস্থা (16 Oct 2026)

## সবুজ (সম্পন্ন) — স্থানীয় ও চুক্তিগত

| স্তম্ভ | অবস্থা | প্রমাণ |
|---|---|---|
| P0 Core semantics | ✅ | `tests/conformance` PASS, scope/shadow/freeze tests PASS |
| P1 Canonical compiler | ✅ | `tests/scope_corpus` 12 case × 3 legs, `tests/runtime_l6` PASS |
| P2 Ecosystem | ✅ | `pkg/contract/*` 8 files verified, stdlib 0 drift |
| P3 Platform contracts | ✅ | `matrix.json` published, `compatibility` pinned |
| P4 Store / signing | ✅ | `pnpm validate:verify` 6/6 verified |

## স্থির (Fixed today)

- `hello-onuron/src/main.nil` — `let i = i + 1` -> `i = i + 1` (loop shadow)
- `final-demo/src/main.nil` — same fix + player variables
- `my-nilang-game/src/main.nil` — same fix
- `todo-alap/src/main.nil` — `i` + `j` loop counters
- `animation-demo/src/main.nil` — `i` loop counter
- `game-starter/src/main.nil` — `frame` + player variables
- `std/` ↔ `pkg/stdlib/modules/` — 53 module sync, baseline 0 drift
- `compiler/vm/vm.go` — `runWithTimeout` cancellation, data race fixed
- `compiler/evaluator/evaluator.go` — `Builtins` RWMutex sync
- `tests/runtime_l6_test.go` — `SetBuiltin`/`DeleteBuiltin` sync

## বাকি — আর্কিটেকচার সীমানা (বাগ নয়, নথিভুক্ত `partial`/`experimental`)

| উপাদান | ম্যাট্রিক্স | কারণ |
|---|---|---|
| `onuronOS/runtime` | `partial` | Rust mini-VM subset; unsupported constructs must explicitly fail |
| `nilos-emulator` | `partial` | Not a second language runtime; uses canonical `.nilax` |
| `alap-framework` | `partial` | Own compiler/stdlib not canonical reference; must delegate |
| `capability_gate` (evaluator) | `implemented` | Direct and indirect builtin calls gated via `checkBuiltinCall` (`evaluator.go:1151`) |
| `capability_gate` (VM) | `implemented` | Direct `OpNativeCall` (`vm.go:1079`) and indirect `callBuiltin` (`vm.go:1656`) gated |
| `tasks_futures` | `partial` | Explicit cancellation (`cancel()`) not implemented; timeout only route |
| `aot` | `experimental` | Not full conformance; GPU submit unsupported |
| `llvm` | `experimental` | Not conformance-tested against canonical |
| `dap` | `partial` | Breakpoint/timeout behavior not fully characterized |
| `wasm` | `experimental` | Some builtins unimplemented; incremental DOM partial |

## সিদ্ধান্ত

স্থানীয় workspace-এ **কোনো দ্বন্দ্ব নেই**:
- `nil-contract verify`: ✅ clean
- `tests/conformance`: ✅ PASS
- `tests/stdlib`: ✅ PASS
- `std/` drift: ✅ 0
- Cross-repo claims (`matrix.json`): ✅ pinned and consistent

নির্ভরশীল (cross-repo) কাজ বাকি আছে (`partial` স্ট্যাটাস), কিন্তু সেগুলো **স্থাপত্যের নকশা**, **কোড বাগ নয়**। সেগুলোর জন্য `onuron_project`, `alap-framework`, `onuronOS` রিপোতে আলাদা কাজ লাগে।
