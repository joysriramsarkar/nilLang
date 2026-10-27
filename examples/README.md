# 📚 Nilang Examples & Reference Implementations

Welcome to the canonical collection of examples, reference projects, and code patterns for the **Nilang Programming Language** (`.nil`), built for the **Alap Framework** and the **Onuron OS** ecosystem.

---

## 📂 Directory Structure

```
examples/
├── projects/                       # 🏛️ Full Multi-File Reference Systems
│   ├── nil-bank/                   # Double-Entry Core Banking & Ledger (9/9 tests pass)
│   ├── nil-quest/                  # Turn-Based RPG Combat Engine & FSM (9/9 tests pass)
│   ├── nil-db/                     # In-Memory Relational Database Engine (10/10 tests pass)
│   ├── nil-doc/                    # Markdown AST Lexer, Parser & Renderer (10/10 tests pass)
│   ├── nil-analytics/              # Matrix, Vector & Linear Regression ML (13/13 tests pass)
│   ├── README.md                   # Detailed Projects Catalog
│   └── PROJECTS_MASTER_CATALOG.md  # Comprehensive Architectural Reference
│
├── snippets/                       # 🧪 150 Conformance Patterns (15 Domains)
│   ├── 01_arithmetic.nil ... 150_levenshtein_distance.nil
│   ├── nil.json                    # Package manifest with capabilities
│   ├── test_all_150_snippets.ps1   # Automated runner (150/150 PASS)
│   ├── README.md                   # Full Snippets Catalog with Explanations
│   └── SNIPPETS_CATALOG.md         # Reference Index
│
├── hello-onuron/                   # 📱 First Alap Mobile App
├── todo-alap/                      # 📝 Reactive State & Component Lifecycle
├── animation-demo/                 # 🎬 60 FPS Animation & Easing
├── game-starter/                   # 🎮 Portable Game Loop & Engine Scaffolds
├── compute-platform/               # ⚡ High-Throughput Matrix & Distributed Compute
└── alap-*/                         # 🎨 Alap Framework Domain Templates (music, pos, settings, etc.)
```

---

## 🚀 Quick Start

### 1. Running a Showcase Project
Navigate to any project directory and run its entry point or test suite:

```bash
# NilAnalytics (Data Science & ML)
cd examples/projects/nil-analytics
nil run src/main.nil
nil run test/test_analytics.nil

# NilBank (Banking & Ledger)
cd ../nil-bank
nil run src/main.nil
nil run test/test_bank.nil

# NilDB (Relational In-Memory Database)
cd ../nil-db
nil run src/main.nil
nil run test/test_db.nil
```

### 2. Running the 150 Language Snippets
Test all 150 core language and standard library patterns:

```powershell
cd examples/snippets
powershell -ExecutionPolicy Bypass -File .\test_all_150_snippets.ps1
```

---

## 🏛️ System Showcase Projects Summary

| Project | Responsibility / Domain | Files | Tests | Status |
|---|---|:---:|:---:|:---:|
| [NilBank](projects/nil-bank/) | Double-entry bookkeeping, multi-currency accounts, APY interest & loans | 7 | 9 / 9 | 100% PASS |
| [NilQuest](projects/nil-quest/) | Turn-based RPG combat, inventory, stat scaling, state machine | 7 | 9 / 9 | 100% PASS |
| [NilDB](projects/nil-db/) | In-memory relational tables, schema, queries, filtering, aggregation | 6 | 10 / 10 | 100% PASS |
| [NilDoc](projects/nil-doc/) | Markdown tokenization, block/inline AST parsing, HTML generation | 6 | 10 / 10 | 100% PASS |
| [NilAnalytics](projects/nil-analytics/) | Vector algebra, matrix multiplication, statistics, linear regression | 6 | 13 / 13 | 100% PASS |

---

## 🛡️ Capability Security Model
Nilang requires explicit capabilities declared in `nil.json` for accessing host primitives (`stdout`, `stdin`, `clock`, `random`, `filesystem`, `network`):

```json
{
  "name": "my-project",
  "version": "1.0.0",
  "entry": "src/main.nil",
  "capabilities": ["stdout", "clock", "filesystem"]
}
```
Projects in this directory demonstrate proper capability scoping under Onuron OS and `nilrt`.
