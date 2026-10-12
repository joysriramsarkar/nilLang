# Syntactic Diagnostics and Numeric Literals

**Status:** Implemented
**Scope:** `compiler/parser` diagnostic channel, `compiler/lexer` numeric lexing
**Relates to:** `LANGUAGE_SPEC.md`, `ERROR_MODEL.md`, `TYPE_SYSTEM.md`

---

## 1. Why this document exists

Before this change, a parse failure was a bare Go string of the form
`line 12:5: expected next token to be ILPAREN, got SEMICOLON instead`.

That shape is fine for a terminal but unusable for everything else in the
toolchain. An editor cannot highlight a range it does not have coordinates for.
The LSP cannot decide which problems to surface as diagnostics versus log lines.
A language server cannot deduplicate or reorder errors by position. No
downstream tool can react to "expected ILPAREN" other than by matching English
prose.

So syntax errors now carry the same structured shape as type errors: a stable
code from `ERROR_MODEL.md`, a span, the quoted source line, a suggestion, and
an AI-readable explanation.

---

## 2. The dual channel, and why both are kept

`parser.Parser` exposes two views of the same failure:

| Method | Type | Audience |
| --- | --- | --- |
| `Errors() []string` | legacy text | CLI output, existing tests |
| `Diagnostics() []*diagnostics.Diagnostic` | structured | LSP, editors, `nil check`, tooling |

Both are populated from a single call site (`Parser.recordError`), so they can
never disagree. The string form keeps its historic `line L:C: ` prefix, which
is why no existing test or user-visible message changed shape.

`frontend.Result` surfaces both:

- `ParseErrors` — the strings.
- `Diagnostics` — the structured form, carrying `Span`, `Suggestion`, and
  `AIExplanation`.

---

## 3. Syntactic diagnostic codes

Codes come from the `Lexical & Syntactic` band of `ERROR_MODEL.md`:

| Code | Meaning | Typical cause |
| --- | --- | --- |
| `E0001` | Unexpected character or invalid token | a stray `}`; a malformed numeric literal |
| `E0002` | Unterminated string literal | missing closing quote |
| `E0003` | Unexpected end of file | a statement or expression left open |
| `E0004` | Expected token X, got Y | a missing delimiter or operator |

Two codes from later bands are also raised at parse time, because the parser is
where the necessary context lives:

| Code | Meaning |
| --- | --- |
| `E0403` | Duplicate entity field |
| `E0406` | Component declares a second `render` or `build` block |

### EOF is distinguished from a mismatch

An unclosed construct produces `E0003`, not `E0004`. "expected `)`" at end of
file is a different mistake from "expected `)` but found `;`", and a fix for one
does not fix the other.

### Spans

`recordError` derives a span from the offending token:

- `StartLine` / `StartCol` — the token position.
- `EndCol` — start plus the token's display width, so a multi-character literal
  is underlined across its full width.
- `ContextLine` — the source line, when the parser was built with
  `NewWithSource`.

`EndCol` is clamped to at least `StartCol + 1` so that even an EOF token, which
carries no literal text, produces a visible caret.

### Interpolation re-anchoring

A string template like `"মোট \(a + )"` is parsed by a sub-parser over the
extracted fragment. The sub-parser's line and column numbers refer to that
fragment, not to the real file, so its diagnostics are re-anchored onto the
template's opening token before being merged into the parent. A developer sees
an error pointing at the template, not at an impossible line 1 column 4.

---

## 4. Numeric literals

The lexer accepted Bengali digits (`unicode.IsDigit` returns true for `০`-`৯`)
but the parser handed the raw text to `strconv.ParseInt`, which understands
only ASCII. The result: `let সংখ্যা = ৪২;` was a hard parse error in a language
whose entire premise is that Bengali reads naturally. This is now fixed, and
the specification's other numeric promises are implemented alongside it.

### Accepted forms

| Form | Example | Value |
| --- | --- | --- |
| Decimal | `1234` | 1234 |
| Bengali digits | `৪২` | 42 |
| Fullwidth digits | `１２３` | 123 |
| Digit separators | `1_000_000` | 1000000 |
| Mixed script | `১_০০০` | 1000 |
| Hexadecimal | `0xFF`, `0xdeadBEEF` | 255, 3735928559 |
| Octal | `0o777` | 511 |
| Binary | `0b1010`, `0b1111_1111` | 10, 255 |
| Float | `1.5` | 1.5 |
| Bengali float | `১.৫` | 1.5 |

### Design notes

**Folding is scoped to numeric literals.** `compiler/parser/numerals.go` rewrites
non-ASCII decimal digits to ASCII and drops `_` separators. It is applied only
on the path from a `INT` or `FLOAT` token to its value, so identifier names and
string contents are never touched. `let ১২৩` and `let "১২৩"` are unrelated
concerns and stay that way.

**Radix prefixes are lexed, not parsed.** `lexer.readNumber` recognises `0x`,
`0o`, and `0b` and consumes the appropriate digit run. This keeps the token a
single unit, so `0xFF` is one `INT` token rather than three. A prefix is only
taken when the leading `0` is immediately followed by a radix marker, which
leaves `0`, `0.5`, and `0` followed by an identifier on the decimal path.

**Negative literals are not literals.** A leading `-` is produced by the unary
minus operator, never by the number itself. `parseIntLiteral` rejects a leading
sign so that the token grammar stays unambiguous and `-9223372036854775808`
(where the magnitude does not fit in `int64`) can never be mis-lexed.

**Fast path.** `foldDigits` returns the input unchanged when no digit needs
folding and no separator is present, so ASCII-only literals pay nothing.

---

## 5. The single frontend gate

Every compilation path now goes through `frontend.Check`. Before this, four
paths hand-rolled their own lex → parse → check sequence, which is how the
native backend came to skip type checking entirely and hand ill-typed programs
to `cc`, producing a C compiler error in place of a Nilang diagnostic.

Exemptions are enumerated, justified, and enforced by
`TestFrontendGateIsSingle` in `tests/frontend_gate_test.go`. Adding a package to
that list requires writing a reason, so the exception set cannot grow silently.

---

## 6. Test obligations

| Test | Guards |
| --- | --- |
| `TestFrontendGateIsSingle` | no unexempted package bypasses the gate |
| `TestFrontendGateExemptionsAreJustified` | every exemption carries a reason |
| `TestParserProducesStableDiagnosticCodes` | codes are stable and expected |
| `TestParserDiagnosticSpansArePopulated` | spans are navigable |
| `TestFrontendLegacyAPIStaysCompatible` | the string channel still works |
| `TestWellTypedSourcePassesGate` | valid sources, including Bengali, pass |
| `compiler/parser/numerals_test.go` | Bengali digits, separators, radix |
