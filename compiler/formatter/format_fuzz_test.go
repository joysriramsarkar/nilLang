package formatter

import (
	"testing"
)

// FuzzFormat pins the never-panics gate for the formatter. The formatter must
// tolerate source that does not parse or check — formatting broken code is its
// job (see the frontend exemptions) — so arbitrary input must return text or
// an error, never panic.
func FuzzFormat(f *testing.F) {
	seeds := []string{
		"let x = 10;",
		"fn f() { return 1; }",
		"let x = ;",
		"if (x) { y; }",
		"{ x; }",
		"(((((",
		"\"unterminated",
		"/* unterminated",
		"let = 1;",
		"fn { return }",
		"\x00\xff\xfe",
		"let ক্ষ = 1;",
		"match x { 1 => 2, _ => 3 };",
		"component C { state s: Int = 0; }",
		"}}}",
		"{{{\n}}}",
		"\t  let   x=1  ;  \n",
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, input string) {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("Format panicked on input %q: %v", input, r)
			}
		}()
		_, _ = Format(input)
	})
}
