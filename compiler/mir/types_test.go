package mir

import (
	"testing"

	"github.com/joysriramsarkar/nilLang/compiler/frontend"
	"github.com/joysriramsarkar/nilLang/compiler/hir"
)

// A struct's *value* is a hash. Its *declaration* is not in the value, and until
// this map existed the declaration stopped meaning anything at MIR — which is the
// level every back end reads.
//
// That was invisible for a long time because nothing downstream needed to know. Then
// three things needed to at once: the FFI cannot lay out a struct it cannot see, the
// generated C header cannot declare one, and anything reasoning about field access
// has to know a field exists. All three read this map.

// TestAStructDeclarationSurvivesIntoMIR is the foundation L5.7 stands on.
//
// Without it there is no way to ask "what are Point's fields" of a lowered program
// — the question is not merely inconvenient, it is unanswerable, because the
// information was discarded two stages earlier.
func TestAStructDeclarationSurvivesIntoMIR(t *testing.T) {
	program := lowerForTest(t, `
		struct Point { x: Int, y: Int }
		let p = Point { x: 1, y: 2 };
		let result = p.x + p.y;
	`)

	point, declared := program.Types["Point"]
	if !declared {
		t.Fatalf("a struct declaration did not survive into MIR; the program knows %d types: %v",
			len(program.Types), typeNames(program))
	}
	if point.Kind != KindStruct {
		t.Errorf("Point is a %q, wanted a struct", point.Kind)
	}
	if len(point.Fields) != 2 {
		t.Fatalf("Point has %d fields, wanted 2", len(point.Fields))
	}
}

// TestFieldsKeepTheirOrder is the part that is easy to get wrong.
//
// A struct crossing a boundary is laid out in declaration order, so two runs of the
// same program must agree about where `y` lives. A map would make that depend on
// iteration order, which is not the same twice.
func TestFieldsKeepTheirOrder(t *testing.T) {
	program := lowerForTest(t, `
		struct Point { x: Int, y: Int, z: Int }
		let p = Point { x: 1, y: 2, z: 3 };
		let result = p.x + p.y + p.z;
	`)

	point := program.Types["Point"]
	if point == nil {
		t.Fatal("Point did not survive")
	}
	want := []string{"x", "y", "z"}
	for i, name := range want {
		if point.Fields[i].Name != name {
			t.Errorf("field %d is %q, wanted %q", i, point.Fields[i].Name, name)
		}
	}
}

// TestAFieldCarriesTheTypeItResolvedToAndTheNameItWasWritten answers two different
// questions at once.
//
// Something laying a struct out needs to know `std.Int` is eight bytes; a
// diagnostic needs to quote what the programmer wrote. Keeping only one of them is
// enough to be wrong somewhere.
func TestAFieldCarriesTheTypeItResolvedToAndTheNameItWasWritten(t *testing.T) {
	program := lowerForTest(t, `
		struct Sample { count: Int, ratio: Float, label: String }
		let s = Sample { count: 1, ratio: 2.0, label: "x" };
		let result = s.count;
	`)

	sample := program.Types["Sample"]
	if sample == nil {
		t.Fatal("Sample did not survive")
	}
	for _, field := range sample.Fields {
		if field.TypeString == "" {
			t.Errorf("field %q has no annotation as written", field.Name)
		}
		if field.Type == nil {
			t.Errorf("field %q (%s) did not resolve to a type", field.Name, field.TypeString)
		}
	}
}

// TestEveryKindOfTypeDeclarationSurvives, because an enum reaching MIR is a
// different question from a struct's and a struct-only test would miss its absence.
func TestEveryKindOfTypeDeclarationSurvives(t *testing.T) {
	program := lowerForTest(t, `
		struct Point { x: Int }
		enum Colour { Red, Green, Blue }
		type Meters = Int

		let p = Point { x: 1 };
		let c = Red;
		let d: Meters = 5;
		let result = p.x + d;
	`)

	for name, want := range map[string]TypeKind{
		"Point":  KindStruct,
		"Colour": KindEnum,
		"Meters": KindAlias,
	} {
		declaration, declared := program.Types[name]
		if !declared {
			t.Errorf("%s did not survive into MIR; the program knows %v", name, typeNames(program))
			continue
		}
		if declaration.Kind != want {
			t.Errorf("%s is a %q, wanted %q", name, declaration.Kind, want)
		}
	}
}

// TestAProgramWithNoDeclarationsHasAnEmptyMapAndNotAMissingOne.
//
// A nil map and an empty one mean the same thing and behave differently: reading a
// nil map works, *writing* to one panics. A program that declared no types and
// arrived with a nil map would crash the moment a declaration reached it.
func TestAProgramWithNoDeclarationsHasAnEmptyMapAndNotAMissingOne(t *testing.T) {
	program := lowerForTest(t, `let a = 1; a + 1;`)

	if program.Types == nil {
		t.Fatal("a program with no declarations has no Types map at all, rather than an empty one")
	}
	if len(program.Types) != 0 {
		t.Errorf("a program with no declarations has %d: %v", len(program.Types), typeNames(program))
	}
	// And it is writable, which is the difference that matters.
	program.Types["Later"] = &TypeDef{Name: "Later", Kind: KindStruct}
}

// TestTheLowererNeverPanicsOnADeclarationInsideAFunction keeps the new arm honest.
//
// `lowerStatement` is called for function bodies as well as the top level, and a
// declaration nested in a body has no `currentProg` to record into. Dropping it is
// right — a local type is not a program's type — but panicking is not.
func TestTheLowererNeverPanicsOnADeclarationInsideAFunction(t *testing.T) {
	lowerForTest(t, `
		struct Outer { x: Int }

		fn make(v: Int) -> Int {
			let inner = v + 1;
			return inner;
		}

		let p = Outer { x: make(1) };
		let result = p.x;
	`)
}

func lowerForTest(t *testing.T, source string) *Program {
	t.Helper()

	checked := frontend.Check(source, frontend.Options{})
	if !checked.OK {
		t.Fatalf("the program was rejected: %s", checked.ErrSummary())
	}
	optimized := hir.NewOptimizer().Optimize(hir.NewLowerer().LowerProgram(checked.Lowered))
	return NewLowerer().LowerHIR(optimized)
}

func typeNames(program *Program) []string {
	names := make([]string, 0, len(program.Types))
	for name := range program.Types {
		names = append(names, name)
	}
	return names
}
