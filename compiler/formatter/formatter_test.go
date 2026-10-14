package formatter

import (
	"strings"
	"testing"
)

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
