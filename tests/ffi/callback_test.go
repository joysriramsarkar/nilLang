package ffi

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joysriramsarkar/nilLang/compiler/frontend"
	"github.com/joysriramsarkar/nilLang/compiler/hir"
	"github.com/joysriramsarkar/nilLang/compiler/llvm"
	"github.com/joysriramsarkar/nilLang/compiler/mir"
	"github.com/joysriramsarkar/nilLang/compiler/toolchain"
)

// TestACallbackCrossesIntoCAndIsCalledFromThere is the gate on L5.5.
//
// Every part of a callback is invisible until C actually calls it. The declaration
// compiles, the closure is built, the pointer is stored — and the program is
// correct right up to the moment C decides to use it. A callback that is never
// invoked passes every test that does not invoke one, and fails on the user's
// machine at a moment they did not choose.
//
// So the test invokes it, and checks the value that comes back.
func TestACallbackCrossesIntoCAndIsCalledFromThere(t *testing.T) {
	source := `
		extern "C" fn nilffi_register(callback: fn(Int) -> Int) -> Void;
		extern "C" fn nilffi_fire(value: Int) -> Int;

		fn double_it(n: Int) -> Int {
			return n + n;
		}

		nilffi_register(double_it);
		nilffi_fire(21);
	`

	got := runProgram(t, source, buildCallbackFixture(t, toolchain.DiscoverToolchain(llvm.TargetHost)))
	if !strings.Contains(got, "42") {
		t.Errorf("the callback did not answer correctly: printed %q, wanted 42", got)
	}
}

// TestACallbackSeesTheValueCArgumentsWith is the other half.
//
// A callback that always answers the same thing would pass the test above even if
// C passed nothing at all. This one takes a different value each time, so a
// callback ignoring its argument fails.
func TestACallbackSeesTheValueCArgumentsWith(t *testing.T) {
	source := `
		extern "C" fn nilffi_register(callback: fn(Int) -> Int) -> Void;
		extern "C" fn nilffi_fire(value: Int) -> Int;

		fn add_seven(n: Int) -> Int {
			return n + 7;
		}

		nilffi_register(add_seven);
		nilffi_fire(1) + nilffi_fire(10);
	`

	got := runProgram(t, source, buildCallbackFixture(t, toolchain.DiscoverToolchain(llvm.TargetHost)))
	// 8 and 17, added by the program.
	if !strings.Contains(got, "25") {
		t.Errorf("the callback did not see what C passed: printed %q, wanted 25", got)
	}
}

// TestACallbackOutlivesTheCallThatMadeIt is the lifetime question.
//
// A callback is pinned for as long as C holds it, not for the duration of the call
// that handed it over. This checks it by registering once, allocating heavily to
// force a real collection, and *then* firing — a pin scoped to the registration
// would be gone, and the closure reclaimable, before the second call.
func TestACallbackOutlivesTheCallThatMadeIt(t *testing.T) {
	source := `
		extern "C" fn nilffi_register(callback: fn(Int) -> Int) -> Void;
		extern "C" fn nilffi_fire(value: Int) -> Int;
		extern "C" fn nilffi_churn(rounds: Int) -> Int;

		fn triple(n: Int) -> Int {
			return n * 3;
		}

		nilffi_register(triple);
		nilffi_churn(5000);
		nilffi_fire(1) + nilffi_fire(2) + nilffi_fire(3);
	`

	got := runProgram(t, source, buildCallbackFixture(t, toolchain.DiscoverToolchain(llvm.TargetHost)))
	// 3 + 6 + 9. A callback that had been collected would answer anything.
	if !strings.Contains(got, "18") {
		t.Errorf("the callback did not survive the allocations: printed %q, wanted 18", got)
	}
}

// TestACallbackIsCalledMoreThanOnceWithoutBeingReRegistered is the same property
// from C's side, and closer to what C really does.
//
// `qsort` keeps a comparator for the length of a sort. Two calls with no
// registration between them is the ordinary shape of a callback, and a runtime that
// only supports one call per registration has not implemented callbacks.
func TestACallbackIsCalledMoreThanOnceWithoutBeingReRegistered(t *testing.T) {
	source := `
		extern "C" fn nilffi_register(callback: fn(Int) -> Int) -> Void;
		extern "C" fn nilffi_fire_twice(a: Int, b: Int) -> Int;
		extern "C" fn nilffi_call_count() -> Int;

		fn increment(n: Int) -> Int {
			return n + 1;
		}

		nilffi_register(increment);
		nilffi_fire_twice(10, 100) + nilffi_call_count();
	`

	got := runProgram(t, source, buildCallbackFixture(t, toolchain.DiscoverToolchain(llvm.TargetHost)))
	// 11 + 101 = 112, plus 2 for the two calls: 114.
	if !strings.Contains(got, "114") {
		t.Errorf("the callback was not called twice: printed %q, wanted 114", got)
	}
}

// TestTheWrongArityIsAnErrorRatherThanACrash is why the arity travels with the
// callback.
//
// The function pointer says nothing about its own signature, so the arity is the
// only record of how many arguments C will pass. Calling with the wrong count must
// be a message — not a read past the end of C's one-element argument array, which
// on most machines is a silently wrong answer rather than a crash.
//
// The C side answers -1 for a refused call, so the program sees a number it could
// not otherwise have got. A program that survived by luck and answered 6 would
// print 6; this prints -1.
func TestTheWrongArityIsAnErrorRatherThanACrash(t *testing.T) {
	source := `
		extern "C" fn nilffi_register(callback: fn(Int) -> Int) -> Void;
		extern "C" fn nilffi_fire_wrong_arity(value: Int) -> Int;

		fn add_one(n: Int) -> Int {
			return n + 1;
		}

		nilffi_register(add_one);
		nilffi_fire_wrong_arity(5);
	`

	got := runProgram(t, source, buildCallbackFixture(t, toolchain.DiscoverToolchain(llvm.TargetHost)))
	if !strings.Contains(got, "-1") {
		t.Errorf("a callback called with the wrong arity did not answer -1: printed %q", got)
	}
}

// TestTheArityTheCallbackReportsIsTheFunctionsOwn checks the count is derived, not
// declared twice.
//
// It comes from the function's parameter list at the point the closure is built.
// Anything else is a second place to update when a signature changes, and the two
// would disagree exactly when it mattered.
func TestTheArityTheCallbackReportsIsTheFunctionsOwn(t *testing.T) {
	source := `
		extern "C" fn nilffi_register(callback: fn(Int) -> Int) -> Void;
		extern "C" fn nilffi_register_pair(callback: fn(Int, Int) -> Int) -> Void;
		extern "C" fn nilffi_reported_arity() -> Int;
		extern "C" fn nilffi_reported_pair_arity() -> Int;
		extern "C" fn nilffi_fire_pair(a: Int, b: Int) -> Int;

		fn single(a: Int) -> Int { return a; }
		fn paired(a: Int, b: Int) -> Int { return a + b; }

		nilffi_register(single);
		nilffi_register_pair(paired);
		nilffi_reported_arity() + nilffi_reported_pair_arity() + nilffi_fire_pair(3, 4);
	`

	got := runProgram(t, source, buildCallbackFixture(t, toolchain.DiscoverToolchain(llvm.TargetHost)))
	// 1 + 2 + 7
	if !strings.Contains(got, "10") {
		t.Errorf("the reported arities are wrong: printed %q, wanted 10 (one plus two, plus seven)", got)
	}
}

// TestReleasingACallbackIsSafe is the other end of the lifetime.
//
// A pin with no release is a leak; a release with no pin is nothing. Both must work,
// and both must be safe to do twice — C code holding a pointer is allowed to be
// unsure, and stopping a program over a double release leaves it worse off than
// continuing.
func TestReleasingACallbackIsSafe(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{
			name: "a release is seen",
			source: `
				extern "C" fn nilffi_register(callback: fn(Int) -> Int) -> Void;
				extern "C" fn nilffi_release() -> Int;
				fn noop(n: Int) -> Int { return n; }
				nilffi_register(noop);
				nilffi_release();
			`,
			want: "1",
		},
		{
			name: "releasing twice is survivable",
			source: `
				extern "C" fn nilffi_register(callback: fn(Int) -> Int) -> Void;
				extern "C" fn nilffi_release_twice() -> Int;
				fn noop(n: Int) -> Int { return n; }
				nilffi_register(noop);
				nilffi_release_twice();
			`,
			want: "1",
		},
		{
			name: "releasing something that is not a callback is survivable",
			source: `
				extern "C" fn nilffi_release_not_a_callback() -> Int;
				nilffi_release_not_a_callback();
			`,
			want: "1",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := runProgram(t, tc.source, buildCallbackFixture(t, toolchain.DiscoverToolchain(llvm.TargetHost)))
			if !strings.Contains(got, tc.want) {
				t.Errorf("printed %q, wanted it to contain %q", got, tc.want)
			}
		})
	}
}

// TestAFunctionUsedAsAValueIsAClosure pins the compiler side.
//
// `let apply = twice;` used to load a variable that did not exist — the emitter
// failed with `loading "twice", which has no slot`, because a name that is a
// function was read as a name that is a variable. The fix belongs in MIR, and this
// says the value is built rather than read.
func TestAFunctionUsedAsAValueIsAClosure(t *testing.T) {
	ir, err := compileModule(`
		fn twice(n: Int) -> Int { return n + n; }
		let apply = twice;
		apply(21);
	`)
	if err != nil {
		t.Fatalf("emitting failed: %v", err)
	}

	// The address is taken, not called: `i64 @nil.twice` would run the function
	// while building the value that names it.
	if !strings.Contains(ir, "ptr @nil.twice.cb") {
		t.Errorf("a function used as a value did not take its address:\n%s", ir)
	}
	if !strings.Contains(ir, "@nil_callback_new") {
		t.Errorf("a function used as a value did not become a closure:\n%s", ir)
	}
	// And it is not read out of a slot, which is the bug this replaces.
	if strings.Contains(ir, `load i64, ptr %twice`) {
		t.Errorf("a function name was still read as a variable slot:\n%s", ir)
	}
}

// TestTheArityIsWhatTheFunctionTakes is what makes a miscounted call survivable.
//
// A callback whose arity does not match what C passes reads past the end of C's
// argument array. The count travels with the callback for exactly that reason, and
// this says where it comes from: the function's own parameter list, read at the
// point the closure is built.
func TestTheArityIsWhatTheFunctionTakes(t *testing.T) {
	cases := map[string]string{
		`fn one(a: Int) -> Int { return a; } let f = one; f(1);`:           "@nil_callback_new(ptr @nil.one.cb, i64 1)",
		`fn two(a: Int, b: Int) -> Int { return a; } let f = two; f(1,2);`: "@nil_callback_new(ptr @nil.two.cb, i64 2)",
		`fn none() -> Int { return 1; } let f = none; f();`:                "@nil_callback_new(ptr @nil.none.cb, i64 0)",
	}

	for source, want := range cases {
		ir, err := compileModule(source)
		if err != nil {
			t.Fatalf("emitting failed for %q: %v", source, err)
		}
		if !strings.Contains(ir, want) {
			t.Errorf("the closure does not carry the right arity; wanted %q:\n%s", want, ir)
		}
	}
}

// TestAFunctionReturningNothingIsNotACallback keeps the two apart.
//
// A callback is a function C calls and gets a value from. A procedure has no
// value to cross, and treating the two the same would build a closure for something
// that cannot be called back into.
//
// The shape check lives in the backend and is tested there; what this pins is that
// a program writing `fn(Int) -> Int` in an extern signature actually gets a callback
// rather than a refusal.
func TestAFunctionReturningNothingIsNotACallback(t *testing.T) {
	ir, err := compileModule(`
		extern "C" fn takes_callback(c: fn(Int) -> Int) -> Void;
		fn handler(n: Int) -> Int { return n; }
		takes_callback(handler);
	`)
	if err != nil {
		t.Fatalf("a function type in an extern signature was refused: %v", err)
	}
	if !strings.Contains(ir, "declare void @takes_callback(i64)") {
		t.Errorf("a callback parameter is not declared as one word:\n%s", ir)
	}
}

// compileModule emits a program and returns the IR, without building it.
func compileModule(source string) (string, error) {
	checked := frontend.Check(source, frontend.Options{})
	if !checked.OK {
		return "", fmt.Errorf("the program was rejected: %s", checked.ErrSummary())
	}
	optimized := hir.NewOptimizer().Optimize(hir.NewLowerer().LowerProgram(checked.Lowered))
	return llvm.NewEmitter(llvm.TargetHost).Emit(mir.NewLowerer().LowerHIR(optimized))
}

// buildCallbackFixture compiles the C that registers and fires callbacks.
func buildCallbackFixture(t *testing.T, tc toolchain.Toolchain) string {
	t.Helper()

	source := filepath.Join("..", "..", "runtime", "ffi", "test", "callbackfixture.c")
	if _, err := os.Stat(source); err != nil {
		t.Skipf("the C fixture is not present: %v", err)
	}
	compiler, hasCompiler := tc.CCompiler()
	if !hasCompiler {
		t.Skip("no C compiler on this machine; callbacks were checked by emitted IR only")
	}

	object := filepath.Join(t.TempDir(), "callbackfixture.o")
	// One `-I` per directory. A single `-I` followed by two paths gives the second
	// one no include path at all, and the error is "nilffi.h: No such file or
	// directory" from a file the reader can see.
	flags := []string{"-c", source, "-o", object, "-O0",
		"-I", filepath.Join("..", "..", "runtime", "core", "include"),
		"-I", filepath.Join("..", "..", "runtime", "ffi", "include")}
	if _, err := toolchain.RunTool(compiler, flags...); err != nil {
		t.Fatalf("compiling the C fixture failed: %v", err)
	}
	return object
}
