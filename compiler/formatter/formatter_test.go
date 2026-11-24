package formatter

import (
	"strings"
	"testing"
)

func TestFormatterPreservesComments(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want []string // substrings that must all appear, in order
		drop []string // substrings that must not appear
	}{
		{
			name: "own-line comments at every level",
			src: `// Program banner
let x = 1;
let y = fn(a) {
    // inner note
    if (a > 0) {
        return a;
    }
    return 0 - a;
};
`,
			want: []string{"// Program banner\nlet x = 1;", "// inner note\n    if (a > 0) {"},
		},
		{
			name: "multi-line block comment keeps its body",
			src: `/* one
   two */
let z = 1;
`,
			want: []string{"/* one\n   two */\nlet z = 1;"},
		},
		{
			name: "comment at end of file is kept",
			src: `let a = 1;
// trailing banner
`,
			want: []string{"let a = 1;\n// trailing banner\n"},
		},
		{
			name: "same-line trailing comment moves to its own line",
			src: `let a = 1; // note
let b = a;
`,
			want: []string{"let a = 1;\n// note\nlet b = a;"},
			drop: []string{"let a = 1; // note"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Format(tc.src)
			if err != nil {
				t.Fatalf("Format failed: %v", err)
			}
			for _, w := range tc.want {
				if !strings.Contains(got, w) {
					t.Fatalf("formatted output lost %q:\n%s", w, got)
				}
			}
			for _, d := range tc.drop {
				if strings.Contains(got, d) {
					t.Fatalf("formatted output unexpectedly kept %q:\n%s", d, got)
				}
			}
			// Preservation must be stable: formatting the formatted text
			// again cannot move or drop a comment.
			again, err := Format(got)
			if err != nil {
				t.Fatalf("re-format failed: %v", err)
			}
			if again != got {
				t.Fatalf("comment preservation is not idempotent:\n--- Pass 1 ---\n%s\n--- Pass 2 ---\n%s", got, again)
			}
		})
	}
}

func TestFormatterCommentOnlyFile(t *testing.T) {
	got, err := Format("// just a note\n")
	if err != nil {
		t.Fatalf("Format failed on comment-only file: %v", err)
	}
	if got != "// just a note\n" {
		t.Fatalf("comment-only file must round-trip, got %q", got)
	}
}

func TestFormatterIdempotency(t *testing.T) {
	sources := []string{
		`let x = 10;
let y = 20;
let z = x + y;
`,
		`fn add(a, b) {
    return a + b;
}

let result = add(5, 10);
`,
		`while (count < 10) {
    count = count + 1;
}
`,
		`entity User {
    id: UUID primary;
    name: String required;
    email: String unique;
}
`,
	}

	for i, src := range sources {
		formatted1, err := Format(src)
		if err != nil {
			t.Fatalf("case %d failed first format: %v", i, err)
		}

		formatted2, err := Format(formatted1)
		if err != nil {
			t.Fatalf("case %d failed second format: %v", i, err)
		}

		if formatted1 != formatted2 {
			t.Fatalf("case %d is not idempotent!\n--- Pass 1 ---\n%s\n--- Pass 2 ---\n%s", i, formatted1, formatted2)
		}
	}
}

func TestFormatterNormalizesMessyCode(t *testing.T) {
	messy := `let  x=10 ;let   y =  20 ; return  x+y ;`
	expected := `let x = 10;
let y = 20;
return x + y;
`

	formatted, err := Format(messy)
	if err != nil {
		t.Fatalf("failed to format messy code: %v", err)
	}

	if formatted != expected {
		t.Fatalf("formatting mismatch:\nGot:\n%s\nExpected:\n%s", formatted, expected)
	}
}

func TestFormatterIfElseAndHash(t *testing.T) {
	src := `if (x > 10) { return 1; } else { return 0; }`
	expected := `if (x > 10) {
    return 1;
} else {
    return 0;
}
`
	formatted, err := Format(src)
	if err != nil {
		t.Fatalf("failed to format if/else: %v", err)
	}
	if formatted != expected {
		t.Fatalf("if/else mismatch:\nGot:\n%s\nExpected:\n%s", formatted, expected)
	}

	hashSrc := `let m = { "z": 100, "a": 200, "m": 300 };`

	// A hash literal keeps the order it was written in. The formatter used to
	// sort keys alphabetically to paper over Go map iteration order, which made
	// output deterministic but silently reordered the developer's code. Order
	// preservation is the real requirement (SEMANTICS.md §1), so the assertion
	// is that formatting is a no-op on an already-formatted literal.
	expectedHash := `let m = {"z": 100, "a": 200, "m": 300};
`
	fHash, err := Format(hashSrc)
	if err != nil {
		t.Fatalf("failed to format hash: %v", err)
	}
	if fHash != expectedHash {
		t.Fatalf("hash order was not preserved:\nGot:\n%s\nExpected:\n%s", fHash, expectedHash)
	}
}

func TestFormatterIndexAssignAndAppStatement(t *testing.T) {
	src := `let a = [1, 2];
a[0] = 42;
`
	formatted, err := Format(src)
	if err != nil {
		t.Fatalf("failed to format index assignment: %v", err)
	}
	expected := `let a = [1, 2];
a[0] = 42;
`
	if formatted != expected {
		t.Fatalf("index assignment was altered or deleted!\nGot:\n%s\nExpected:\n%s", formatted, expected)
	}
}

// TestFormatterKeepsGroupingParens pins the paren rule: the formatter may
// drop parens only when re-parsing regroups identically. `!(a && b)` printed
// as `!a && b` flips the meaning — the bootstrap lexer parity failure that
// found this — and `(a + b) * c` printed flat re-binds the multiplication.
func TestFormatterKeepsGroupingParens(t *testing.T) {
	cases := []struct{ src, want string }{
		{`let x = !(a && b);`, `!(a && b)`},
		{`let x = !(a || b);`, `!(a || b)`},
		{`let x = -(a + b);`, `-(a + b)`},
		{`let x = (a + b) * c;`, `(a + b) * c`},
		{`let x = a * (b + c);`, `a * (b + c)`},
		{`let x = (a == b) == c;`, `a == b == c`},
		{`let x = a == (b == c);`, `a == (b == c)`},
		{`let x = -a + b;`, `-a + b`},
		{`let x = a + b + c;`, `a + b + c`},
		{`let x = f((a + b));`, `f(a + b)`},
	}
	for _, tc := range cases {
		formatted, err := Format(tc.src + "\n")
		if err != nil {
			t.Fatalf("Format(%q) failed: %v", tc.src, err)
		}
		if !strings.Contains(formatted, tc.want) {
			t.Errorf("Format(%q) lost grouping:\n got: %q\nwant substring: %q", tc.src, formatted, tc.want)
		}
		again, err := Format(formatted)
		if err != nil {
			t.Fatalf("reformat failed for %q: %v", tc.src, err)
		}
		if again != formatted {
			t.Errorf("not idempotent for %q:\nfirst:\n%ssecond:\n%s", tc.src, formatted, again)
		}
	}
}
