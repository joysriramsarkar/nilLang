package lexer

import (
	"testing"

	"github.com/joysriramsarkar/nilLang/compiler/token"
)

// FuzzLexer pins the never-panics gate at the token level: arbitrary bytes —
// unterminated strings and comments, lone surrogates, NULs, deep nesting —
// must tokenize or record an error, never panic.
func FuzzLexer(f *testing.F) {
	seeds := []string{
		"let x = 10;",
		"\"unterminated string",
		"/* unterminated comment",
		"// line comment without newline",
		"'a'",
		"'ab'",
		"``",
		"\"\\x\"",
		"\"\\u12\"",
		"\x00\x01\x02\xff\xfe",
		"\xed\xa0\x80",
		"((((((((((()))))))))))",
		"{{{{{{{{{{}}}}}}}}}}",
		"[[[[[[[[[[[]]]]]]]]]]",
		"let সংখ্যা = ৪২;",
		"let ক্ষ = 1; ক্ষ;",
		"123abc",
		"1.2.3",
		"1e",
		"1e+",
		"0b102",
		"0o89",
		"0x",
		"1_000_000",
		"...",
		"=>",
		"?.",
		"??",
		"..",
		"..=",
		"@",
		"#",
		"$",
		"`",
		"let let let",
		"fn fn fn",
		"\"\"\"",
		"''",
		"/* /* nested */ still open",
		"\t\r\n ",
		"let x = \"a\" + \"b\";",
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, input string) {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("Lexer panicked on input %q: %v", input, r)
			}
		}()
		l := New(input)
		// The token stream must terminate: cap the walk well above any
		// honest output so a non-terminating lexer fails loudly instead of
		// hanging the fuzzer.
		for i := 0; i < len(input)*4+16; i++ {
			if l.NextToken().Type == token.EOF {
				return
			}
		}
		t.Fatalf("Lexer did not terminate on input %q", input)
	})
}
