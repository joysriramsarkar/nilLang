package frontend

import (
	"runtime/debug"
	"testing"
)

// FuzzCheck pins the P0.6 never-panics gate end to end: any source input must
// come back as a Result with diagnostics, never a panic — through lexing,
// parsing, type checking and lowering alike.
func FuzzCheck(f *testing.F) {
	seeds := []string{
		`let x = 1;`,
		`fn f() -> Int { return 1; }`,
		`let x = (1 + 2;`,
		`y = 1;`,
		`break;`,
		`while (x) { break; }`,
		`for i in [1, 2] { fns = push(fns, i); }`,
		`match x { 1 => 2, _ => 3 };`,
		`{ x; }`,
		`return 1;`,
		`component C { state s: Int = 0; render { return s; } }`,
		`let f = fn() { count = count + 1; return count; };`,
		`import "web";`,
		`extern "C" fn sin(x: Float) -> Float;`,
		`trait T { fn m(); }`,
		`impl P for U { fn m() { x; } }`,
		`export fn f() { x; }`,
		`type ID = Int;`,
		`let a = b ?? c;`,
		`let a = b?.c;`,
		`let u = User { id: 1 };`,
		`enum S { A, B(Int) }`,
		`struct U { id: Int, name: String }`,
		`while let m = opt { x; }`,
		`"str" + 1;`,
		`((((((((((1))))))))));`,
		"\x00\xff\xfe\xed\xa0\x80",
		`let ক্ষ = 1; ক্ষ;`,
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, input string) {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("Check panicked on input %q: %v\n%s", input, r, debug.Stack())
			}
		}()
		res := Check(input, Options{Filename: "fuzz.nil"})
		_ = res.ErrSummary()
	})
}
