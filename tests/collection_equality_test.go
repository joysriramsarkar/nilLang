package tests

import (
	"testing"
)

// TestCollectionEqualityIsIdentityInBothEngines pins SEMANTICS.md §5.3:
// two collections compare by identity, so two separately built arrays
// with equal contents are NOT equal, in either engine. The VM used to
// compare Inspect() text (structural), so `[1, 2] == [1, 2]` was false
// in the evaluator and true in the VM — the same program answered
// differently depending on how it was run.
func TestCollectionEqualityIsIdentityInBothEngines(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{
			name:   "two array literals are not equal",
			source: `let result = [1, 2] == [1, 2];`,
			want:   "false",
		},
		{
			name:   "two array literals are unequal",
			source: `let result = [1, 2] != [1, 2];`,
			want:   "true",
		},
		{
			name:   "an array equals itself",
			source: `let a = [1, 2]; let result = a == a;`,
			want:   "true",
		},
		{
			name:   "two hash literals are not equal",
			source: `let result = {"a": 1} == {"a": 1};`,
			want:   "false",
		},
		{
			name:   "a hash equals itself",
			source: `let h = {"a": 1}; let result = h == h;`,
			want:   "true",
		},
		{
			name:   "two Ok values are not equal",
			source: `let result = Ok(1) == Ok(1);`,
			want:   "false",
		},
		{
			name:   "an alias compares equal",
			source: `let a = [1, 2]; let b = a; let result = a == b;`,
			want:   "true",
		},
		{
			name:   "booleans still compare by value",
			source: `let result = isOk(Ok(1)) == true;`,
			want:   "true",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := evalSource(t, tc.source)
			if !res.OK {
				t.Fatalf("source was rejected: %s", res.ErrSummary())
			}
			if got, err := evaluatorEval(res.Program, "result"); err != nil {
				t.Fatalf("evaluator: %v", err)
			} else if got != tc.want {
				t.Errorf("evaluator: got %s, want %s", got, tc.want)
			}
			if got, err := vmEval(res.Program, "result"); err != nil {
				t.Fatalf("VM: %v", err)
			} else if got != tc.want {
				t.Errorf("VM: got %s, want %s", got, tc.want)
			}
		})
	}
}
