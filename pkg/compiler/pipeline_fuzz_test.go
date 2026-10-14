package compiler_test

import (
	"testing"

	pkgcompiler "github.com/joysriramsarkar/nilLang/pkg/compiler"
)

// FuzzPipelineCompile pins never-panics through the whole compile path —
// frontend gate plus direct and MIR bytecode backends. A source either
// compiles or comes back with an error; it must never panic the compiler.
//
// The evaluator and the VM are deliberately NOT fuzzed here: arbitrary
// programs can loop forever (`while (true) {}`), and a fuzzer cannot tell a
// hang from a slow case. Never-panics for execution is covered by the
// regression corpus in tests/ instead.
func FuzzPipelineCompile(f *testing.F) {
	seeds := []string{
		`let x = 40; x + 2;`,
		`fn f(a: Int) -> Int { return a + 1; }; f(1);`,
		`let x = (1 + 2;`,
		`y = 1;`,
		`break;`,
		`for i in [1, 2] { x; }`,
		`match x { 1 => 2, _ => 3 };`,
		`component C { state s: Int = 0; render { return s; } }`,
		`{ x; }`,
		`import "web";`,
		`extern "C" fn sin(x: Float) -> Float;`,
		`enum S { A, B(Int) }`,
		`struct U { id: Int, name: String }`,
		`trait T { fn m(); }`,
		`impl P for U { fn m() { x; } }`,
		`let a = b ?? c;`,
		`while let m = opt { x; }`,
		"\x00\xff\xfe",
		`let ক্ষ = 1; ক্ষ;`,
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, input string) {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("pipeline panicked on input %q: %v", input, r)
			}
		}()
		_ = pkgcompiler.NewPipeline(input, "fuzz.nil").Compile()
		_ = pkgcompiler.NewPipeline(input, "fuzz.nil").WithBackend(pkgcompiler.BackendMir).Compile()
	})
}

// FuzzDecodeBytecode pins never-panics for hostile bytecode images: random
// bytes must decode-error, never panic. A .nilax arrives over the network,
// so this is the sandbox boundary, not just robustness.
func FuzzDecodeBytecode(f *testing.F) {
	seeds := [][]byte{
		{},
		{0x00},
		[]byte("NABC"),
		[]byte("NABC\x00\x01"),
		{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff},
		[]byte("nilax-malicious-payload-that-keeps-going-forever-and-ever"),
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, input []byte) {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("DecodeBytecode panicked on %d bytes: %v", len(input), r)
			}
		}()
		_, _ = pkgcompiler.DecodeBytecode(input)
	})
}
