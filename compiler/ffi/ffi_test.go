package ffi

import (
	"strings"
	"testing"

	"github.com/joysriramsarkar/nilLang/compiler/mir"
)

// TestATypeWithNoCrossingIsRefusedByName is the rule that makes an unfinished FFI
// safe to ship.
//
// A type with no defined crossing is **refused by name** rather than converted to
// something plausible.
//
// A plausible conversion is the worst outcome available: the module is well-formed,
// the assembler accepts it, the linker resolves it, and the program reads a length
// out of a pointer. It fails on the *data*, not on the build, which is the hardest
// kind of failure to find and the easiest to ship by accident.
func TestATypeWithNoCrossingIsRefusedByName(t *testing.T) {
	cases := []struct {
		name    string
		fn      *mir.Function
		wantMsg string
	}{
		{
			name: "an unannotated parameter",
			fn: &mir.Function{
				Name: "mystery", Foreign: true, Abi: "C",
				Params: []string{"x"}, ParamTypes: []string{""},
				ReturnType: "Int",
			},
			wantMsg: "unannotated",
		},
		{
			name: "a list, which has no crossing at all",
			fn: &mir.Function{
				Name: "sort", Foreign: true, Abi: "C",
				Params: []string{"xs"}, ParamTypes: []string{"Array<Int>"},
				ReturnType: "Void",
			},
			wantMsg: "Array<Int>",
		},
		{
			name: "a record",
			fn: &mir.Function{
				Name: "render", Foreign: true, Abi: "C",
				Params: []string{"r"}, ParamTypes: []string{"Record"},
				ReturnType: "Void",
			},
			wantMsg: "Record",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := SignatureFor(mir.NewProgram(), tc.fn)
			if err == nil {
				t.Fatalf("the crossing table accepted a type that does not have one for %s", tc.fn.Name)
			}
			if !strings.Contains(err.Error(), tc.wantMsg) {
				t.Errorf("the refusal does not name %q:\n%v", tc.wantMsg, err)
			}
			// A refusal that says only "no" is barely better than a wrong answer: the
			// reader still has to work out which of the declaration's types was the
			// problem, and the declaration may be in another file. So it has to say
			// what it looked for *and* what was available.
			if !strings.Contains(err.Error(), "defined crossings:") {
				t.Errorf("the refusal does not say what is available:\n%v", err)
			}
		})
	}
}

// TestARefusalSaysWhatIsAvailable is the other half.
//
// A diagnostic that only says no leaves the reader with nothing to do except read
// the FFI's source. Naming the crossings that do exist turns it into the next line
// they could write instead.
func TestARefusalSaysWhatIsAvailable(t *testing.T) {
	_, err := SignatureFor(mir.NewProgram(), &mir.Function{
		Name: "sort", Foreign: true, Abi: "C",
		Params: []string{"xs"}, ParamTypes: []string{"Array<Int>"},
		ReturnType: "Void",
	})
	if err == nil {
		t.Fatal("the crossing table accepted a parameter type with no crossing")
	}
	if !strings.Contains(err.Error(), Supported) {
		t.Errorf("the refusal does not list what is available:\n%v", err)
	}
	// And it names `String`, which is the answer to the question this test used to
	// ask — the crossing that was missing when this file was written and is not now.
	if !strings.Contains(Supported, "String") {
		t.Errorf("the list of available crossings does not mention String: %q", Supported)
	}
}

// TestTheCrossingIsCaseAndNamespaceInsensitive says two names for one type cross
// the same way.
//
// `std.Int` and `Int` are one type, and the crossing depends on the type rather
// than on how somebody qualified or capitalised it. Getting this wrong produces a
// module where the same function is declared two different ways depending on which
// import brought the name in — which is a bug that appears only in the file that
// imports something.
func TestTheCrossingIsCaseAndNamespaceInsensitive(t *testing.T) {
	want, ok := For("Int")
	if !ok {
		t.Fatal("Int has no crossing")
	}

	for _, spelling := range []string{"Int", "int", "INT", "std.Int", "core.Int", "std.int"} {
		got, found := For(spelling)
		if !found {
			t.Errorf("%q has no crossing", spelling)
			continue
		}
		if got.LLVM != want.LLVM || got.Unbox != want.Unbox {
			t.Errorf("%q crosses as %s/%s but Int crosses as %s/%s",
				spelling, got.LLVM, got.Unbox, want.LLVM, want.Unbox)
		}
	}
}

// TestEveryCrossingNamesBothDirections is what stops a type crossing one way.
//
// A type with an unbox but no box works perfectly for a `-> Void` function, so the
// mistake is invisible until somebody uses it as a return — and then the module
// declares nothing and the linker reports an undefined symbol for a function the
// reader can see named in the header.
func TestEveryCrossingNamesBothDirections(t *testing.T) {
	for name, crossing := range Types {
		if crossing.C == "" {
			t.Errorf("%s crosses as an LLVM type but has no C spelling; a generated header "+
				"would print an empty type and a C compiler would reject it", name)
		}
		if crossing.LLVM == "" {
			t.Errorf("%s has a C spelling but no LLVM one", name)
		}
		// `void` crosses unchanged in both directions, so it is the one entry with
		// no conversion on either side. Everything else needs both.
		if name == "void" {
			continue
		}
		if crossing.Unbox == "" {
			t.Errorf("%s has no unbox; a program passing it to C would hand over a "+
				"tagged word the callee would read as a number", name)
		}
		if crossing.Box == "" {
			t.Errorf("%s has no box; a program receiving it would hold whatever the "+
				"callee returned, unboxed, and read a number's low bits as a tag", name)
		}
	}
}

// TestACrossingSaysHowManyArgumentsItUses is the field that keeps the emitter and
// the generated header from disagreeing.
//
// The emitter writes the call's argument list and the header generator writes the
// prototype's. Both count the same way, and if they counted differently the header
// would compile and link and be wrong.
func TestACrossingSaysHowManyArgumentsItUses(t *testing.T) {
	for name, crossing := range Types {
		want := 1
		if name == "void" {
			want = 0
		}
		if crossing.Splat {
			want = 2
		}
		if crossing.Params != want {
			t.Errorf("%s says it uses %d arguments; %d is what it actually takes",
				name, crossing.Params, want)
		}
	}
}

// TestAFunctionTypeIsRecognisedByItsShapeAndNothingElse guards the one heuristic in
// the table.
//
// `fn` at the start plus a parameter list is a function type. The obvious looser
// version — `fn` at the start plus a bracket somewhere — accepts `fn`, `fnord` and
// `FnPtr`, and each of those would be handed a callback's crossing when it is not a
// function type at all. That does not produce a refusal; it produces a module that
// calls C with a closure where a number belongs.
func TestAFunctionTypeIsRecognisedByItsShapeAndNothingElse(t *testing.T) {
	functionTypes := []string{
		"fn(Int) -> Int",
		"fn() -> Void",
		"fn(Int, String) -> String",
	}
	for _, annotation := range functionTypes {
		crossing, ok := For(annotation)
		if !ok {
			t.Errorf("%q is a function type and has no crossing", annotation)
			continue
		}
		if crossing.C != Callback.C {
			t.Errorf("%q crosses as %q rather than as a callback", annotation, crossing.C)
		}
	}

	// And the near-misses, which must fall through to the refusal.
	for _, annotation := range []string{"fn", "fnord", "FnPtr", "Function", "Fixnum"} {
		if _, ok := For(annotation); ok {
			t.Errorf("%q was read as a function type", annotation)
		}
	}
}
