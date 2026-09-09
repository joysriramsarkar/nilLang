# Nil self-hosting bootstrap

This directory contains compiler phases written in Nil and hosted by the trusted Go implementation.

## Self-hosted compiler image

The checked-in Nil sources now form a complete hosted compiler pipeline:

1. The trusted Go parser and evaluator run the Nil-written compiler once to create stage B.
2. Stage B embeds the Nil compiler modules and their deterministic portable IR.
3. Stage B rebuilds stage C from its embedded Nil sources.
4. Bootstrap succeeds only when the stage B and stage C images are byte-identical.

Build or verify the compiler image from the repository root:

```powershell
go run ./cmd/nil-bootstrap build
go run ./cmd/nil-bootstrap verify
```

The default image is written to `build/nil-compiler.json`. To rebuild only from that image:

```powershell
go run ./cmd/nil-bootstrap rebuild build/nil-compiler.json build/nil-compiler.next.json
```

The Go implementation remains the stage-0 runtime and object-system host. Compiler language logic (lexing, parsing, semantic analysis, and portable IR emission) is implemented in Nil and reaches a reproducible fixed point.

## Compiler stages

Stage 0 starts with `lexer.nil`. It tokenizes the implemented ASCII language core, including keywords, identifiers, numbers, strings, comments, operators, delimiters, and source positions. `lexer_test.go` checks its output against the Go lexer.

Stage 1 begins with `parser.nil`. It is a Nil-written Pratt parser for declarations, returns, literal and identifier expressions, prefix expressions, grouped expressions, and precedence-aware binary expressions. Its hash-backed AST is serializable without depending on Go AST objects. `parser_test.go` compares that AST with a normalized Go parser AST and checks structured syntax diagnostics.

Run the acceptance check from the repository root:

```powershell
go test ./bootstrap
```

The independently tested stages are:

1. Nil lexer for the compiler's implemented ASCII bootstrap core.
2. Nil parser producing a serializable AST for the bootstrap grammar.
3. Nil semantic analysis with lexical scopes and binding diagnostics.
4. Deterministic portable bootstrap IR generation.
5. Reproducible bootstrap where compiler A builds B, B builds C, and B/C outputs match.