# Nil Ecosystem Documentation

Welcome to the technical documentation repository for **NilLang**, the **Alap Framework**, and **Onuron OS**.

---

## 📑 Specifications & Architecture

- **[Ecosystem Architecture](spec/ARCHITECTURE.md)**:
  - System division and hierarchy (Human -> AI Agent -> NilLang / Alap -> Compiler & Oracle -> MIR -> WASM / Native / Bytecode).
  - NilLang Runtime Profiles vs. Alap Application Modules.
  - Package Management (`nilpkg`), Cryptographic Signing (`nilkey`), and `.nilax` Application Bundles.
  - Onuron SoftBus peer-to-peer distributed protocol.
  - AI-Compiler Integration and Hallucination Resistance architecture.

- **[Formal Language Specification](spec/LANGUAGE_SPEC.md)**:
  - Language design philosophy ("One Way To Do It").
  - Lexical structure, tokens, Unicode & Bengali native identifiers.
  - Complete grammar (EBNF) for statements and expressions.
  - Multi-tiered type system (Primitives, ADTs, Generics, Unions, Capabilities, and Effects).
  - Execution models: Tree-Walking Interpreter, High-Level IR (HIR), Mid-Level IR (MIR / CFG), WebAssembly, and Stack VM Bytecode.

---

## 🛠️ CLI Quick Reference

| Command | Description |
|---|---|
| `nil init <name> --profile <p>` | Initialize a new NilLang project with specified profile |
| `nil build [target]` | Compile standalone executable or `.nilax` package |
| `nil build wasm` | Generate WebAssembly binary (`.wasm`) and browser harness |
| `nil run [file.nil]` | Execute file via AST interpreter or `-vm` bytecode engine |
| `nil hir [file.nil]` | Inspect High-Level Intermediate Representation & constant folding |
| `nil mir [file.nil]` | Inspect Mid-Level Intermediate Representation & Control-Flow Graph |
| `nil oracle [subcommand]` | Query compiler oracle for types, symbols, signatures, and static safety |
| `nil check [path]` | Verify profile capability boundaries & AI guard contracts |
| `nil verify [component]` | Run formal 6-stage Verified Novelty pipeline |
| `nil render [file.nil]` | Render declarative Alap UI components to ANSI / HTML |
