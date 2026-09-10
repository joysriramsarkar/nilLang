package formatter

import (
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

	// Deterministic Hash key ordering
	hashSrc := `let m = { "z": 100, "a": 200, "m": 300 };`
	expectedHash := `let m = {"a": 200, "m": 300, "z": 100};
`
	fHash, err := Format(hashSrc)
	if err != nil {
		t.Fatalf("failed to format hash: %v", err)
	}
	if fHash != expectedHash {
		t.Fatalf("hash mismatch:\nGot:\n%s\nExpected:\n%s", fHash, expectedHash)
	}
}

