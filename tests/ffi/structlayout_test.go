package ffi

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/joysriramsarkar/nilLang/compiler/ffi"
	"github.com/joysriramsarkar/nilLang/compiler/frontend"
	"github.com/joysriramsarkar/nilLang/compiler/hir"
	"github.com/joysriramsarkar/nilLang/compiler/llvm"
	"github.com/joysriramsarkar/nilLang/compiler/mir"
	"github.com/joysriramsarkar/nilLang/compiler/toolchain"
)

// L5.7: a struct crosses the boundary, and the offsets have to be C's.
//
// Every other layout check in the repository compares the compiler against itself:
// the numbers in `compiler/ffi/layout_test.go` are written by hand and the algorithm
// is asked to match them. That catches the algorithm disagreeing with a person, and
// it does not catch the person.
//
// This one asks a C compiler. It writes the struct as a C author would write it,
// asks both compilers where they put each field, and compares that against what the
// FFI's layout says. A disagreement is a buffer C reads at the wrong offsets, and
// both sides compiled.

// cShapes are the structs, written the way somebody writes a C struct.
//
// Written by hand rather than generated from the crossing table on purpose. A
// generated struct would be built from the same numbers the algorithm produced, so
// it would agree by construction and the test would prove nothing. These are what a
// C author expects, and the FFI has to meet them.
var cShapes = []struct {
	name     string
	nilTypes []string
	cBody    []string
}{
	{
		name:     "Point",
		nilTypes: []string{"Int", "Int"},
		cBody:    []string{"int64_t x;", "int64_t y;"},
	},
	{
		name:     "Flagged",
		nilTypes: []string{"Int", "Byte"},
		cBody:    []string{"int64_t x;", "uint8_t y;"},
	},
	{
		name:     "Sparse",
		nilTypes: []string{"Byte", "Int", "Byte"},
		cBody:    []string{"uint8_t a;", "int64_t x;", "uint8_t b;"},
	},
	{
		name:     "Labelled",
		nilTypes: []string{"Int", "String"},
		cBody:    []string{"int64_t x;", "const char *label;", "size_t label_len;"},
	},
	{
		name:     "LabelOnly",
		nilTypes: []string{"String"},
		cBody:    []string{"const char *bytes;", "size_t length;"},
	},
	{
		name:     "Pack",
		nilTypes: []string{"Byte", "Byte", "Byte"},
		cBody:    []string{"uint8_t a;", "uint8_t b;", "uint8_t c;"},
	},
	{
		name:     "Three",
		nilTypes: []string{"Float", "Float", "Float"},
		cBody:    []string{"double a;", "double b;", "double c;"},
	},
	{
		name:     "Mixed",
		nilTypes: []string{"Byte", "Bool", "Int", "String"},
		cBody:    []string{"uint8_t a;", "int b;", "int64_t x;", "const char *s;", "size_t s_len;"},
	},
}

// TestBothCompilersPutEveryFieldWhereTheFFIPutsIt is the gate on the layout.
//
// Both compilers, in one link, answering the same question from the same header —
// the same shape as the runtime-layout test, because the reason is the same: two
// compilers in one link must agree, and a disagreement is not a style problem.
func TestBothCompilersPutEveryFieldWhereTheFFIPutsIt(t *testing.T) {
	tc := toolchain.DiscoverToolchain(llvm.TargetHost)
	cc, hasC := tc.CCompiler()
	if !hasC {
		t.Skip("no C compiler on this machine; the layout was not compared")
	}

	dir := t.TempDir()
	includes := []string{
		"-I", filepath.Join("..", "..", "runtime", "core", "include"),
		"-I", filepath.Join("..", "..", "runtime", "ffi", "include"),
	}

	probes := []struct {
		prefix   string
		compiler toolchain.Tool
	}{{"c", cc}}
	if tc.Clang.Available() {
		probes = append(probes, struct {
			prefix   string
			compiler toolchain.Tool
		}{"clang", tc.Clang})
	}

	objects := make([]string, 0, len(probes)+1)
	for _, probe := range probes {
		source := filepath.Join(dir, probe.prefix+".c")
		if err := os.WriteFile(source, []byte(offsetProbe(probe.prefix)), 0o644); err != nil {
			t.Fatal(err)
		}
		object := filepath.Join(dir, probe.prefix+".o")
		args := append([]string{"-c", source, "-o", object, "-O0", "-std=c11"}, includes...)
		if out, err := toolchain.RunTool(probe.compiler, args...); err != nil {
			t.Fatalf("the %s probe did not compile:\n%s", probe.prefix, strings.TrimSpace(out))
		}
		objects = append(objects, object)
	}

	driver := filepath.Join(dir, "driver.c")
	if err := os.WriteFile(driver, []byte(offsetDriver(probes)), 0o644); err != nil {
		t.Fatal(err)
	}
	driverObject := filepath.Join(dir, "driver.o")
	args := append([]string{"-c", driver, "-o", driverObject, "-O0", "-std=c11"}, includes...)
	if out, err := toolchain.RunTool(cc, args...); err != nil {
		t.Fatalf("the driver did not compile:\n%s", strings.TrimSpace(out))
	}
	objects = append(objects, driverObject)

	executable := filepath.Join(dir, "offsets")
	if runtime.GOOS == "windows" {
		executable += ".exe"
	}
	linkArgs := append(append([]string{}, objects...), "-o", executable)
	if out, err := toolchain.RunTool(cc, linkArgs...); err != nil {
		t.Fatalf("the probes did not link:\n%s", strings.TrimSpace(out))
	}

	result, err := exec.Command(executable).CombinedOutput()
	if err != nil {
		t.Fatalf("running the offset comparison failed: %v\n%s", err, result)
	}
	lines := strings.Split(strings.TrimSpace(string(result)), "\n")
	if len(lines) < 2 {
		t.Skipf("only one compiler answered; the comparison needs two:\n%s", result)
	}

	cLine, clangLine := strings.TrimSpace(lines[0]), strings.TrimSpace(lines[1])
	if cLine != clangLine {
		t.Errorf("the two compilers disagree about the structs:\n  c:     %s\n  clang: %s", cLine, clangLine)
	}

	// And now the comparison that matters: ours against theirs.
	ours := ourLayouts(t)
	got := strings.Fields(cLine)
	want := strings.Fields(ours)
	if len(got) != len(want) {
		t.Fatalf("the probe answered %d numbers and the FFI has %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("number %d: a C compiler says %s and the FFI says %s\n"+
				"  all C:  %s\n  all FFI: %s", i, got[i], want[i], cLine, ours)
		}
	}
}

// offsetProbe asks one compiler where each field of each struct sits.
func offsetProbe(prefix string) string {
	var b strings.Builder
	b.WriteString("#include <stddef.h>\n#include <stdint.h>\n\n")
	for _, shape := range cShapes {
		fmt.Fprintf(&b, "typedef struct {\n")
		for _, field := range shape.cBody {
			fmt.Fprintf(&b, "  %s\n", field)
		}
		fmt.Fprintf(&b, "} %s_t;\n\n", shape.name)
	}
	for _, shape := range cShapes {
		fmt.Fprintf(&b, "long %s_sizeof_%s(void) { return (long)sizeof(%s_t); }\n", prefix, shape.name, shape.name)
		fmt.Fprintf(&b, "long %s_alignof_%s(void) { return (long)_Alignof(%s_t); }\n", prefix, shape.name, shape.name)
		for i, field := range shape.cBody {
			fmt.Fprintf(&b, "long %s_offset_%s_%d(void) { return (long)offsetof(%s_t, %s); }\n",
				prefix, shape.name, i, shape.name, cFieldName(field))
		}
	}
	return b.String()
}

// offsetDriver prints every compiler's answers in one order.
func offsetDriver(probes []struct {
	prefix   string
	compiler toolchain.Tool
}) string {
	var b strings.Builder
	b.WriteString("#include <stdio.h>\n")
	for _, probe := range probes {
		fmt.Fprintf(&b, "extern long %s_sizeof_%s(void);\n", probe.prefix, "Point")
	}
	for _, shape := range cShapes {
		for _, probe := range probes {
			fmt.Fprintf(&b, "extern long %s_sizeof_%s(void);\n", probe.prefix, shape.name)
			fmt.Fprintf(&b, "extern long %s_alignof_%s(void);\n", probe.prefix, shape.name)
			for i := range shape.cBody {
				fmt.Fprintf(&b, "extern long %s_offset_%s_%d(void);\n", probe.prefix, shape.name, i)
			}
		}
	}
	b.WriteString("int main(void) {\n")
	for _, probe := range probes {
		for _, shape := range cShapes {
			fmt.Fprintf(&b, "  printf(\"%%ld \", %s_sizeof_%s());\n", probe.prefix, shape.name)
			fmt.Fprintf(&b, "  printf(\"%%ld \", %s_alignof_%s());\n", probe.prefix, shape.name)
			for i := range shape.cBody {
				fmt.Fprintf(&b, "  printf(\"%%ld \", %s_offset_%s_%d());\n", probe.prefix, shape.name, i)
			}
		}
		b.WriteString("  printf(\"\\n\");\n")
	}
	b.WriteString("  return 0;\n}\n")
	return b.String()
}

// ourLayouts is the same answers, from the FFI's own layout, in the same order.
func ourLayouts(t *testing.T) string {
	t.Helper()

	var numbers []string
	for _, shape := range cShapes {
		declaration := "struct " + shape.name + " { "
		for i, nilType := range shape.nilTypes {
			if i > 0 {
				declaration += ", "
			}
			// The field name has to match the C one, so the two agree about which
			// field is which.
			declaration += fieldNameFor(shape, i) + ": " + nilType
		}
		declaration += " }\n"

		program := lowerTypes(t, declaration)

		layout, err := ffi.LayoutOf(program, shape.name)
		if err != nil {
			t.Fatalf("laying out %s out failed: %v", shape.name, err)
		}
		numbers = append(numbers, strconv.FormatInt(layout.Size, 10))
		numbers = append(numbers, strconv.FormatInt(layout.Alignment, 10))
		for _, field := range layout.Fields {
			numbers = append(numbers, strconv.FormatInt(field.Offset, 10))
			// A string field is *two* C members — a pointer and a length — and the C
			// probe reports one offset per member. Emitting only the first would
			// compare the first three offsets of a four-offset struct and pass while
			// the length landed in the wrong place, which is the mistake this test
			// exists to catch.
			//
			// The second word is eight bytes on: both are eight-aligned and the
			// pointer comes first, which is the order `nil_string_unbox` writes them
			// in and the order the C struct declares them in.
			if field.Crossing.Splat {
				numbers = append(numbers, strconv.FormatInt(field.Offset+8, 10))
			}
		}
	}
	return strings.Join(numbers, " ") + " "
}

// cFieldName is the name of a C struct member.
//
// Read backwards from the end of the declaration rather than by splitting on
// whitespace, because `const char *label;` has no space to split on — the name is
// glued to the star — and a splitter returns `*label;`, which is neither a name nor
// a field `offsetof` will accept. The error that comes back says `expected ')'`,
// which names neither the struct nor the mistake.
func cFieldName(declaration string) string {
	trimmed := strings.TrimRight(strings.TrimSpace(declaration), ";")
	end := len(trimmed)
	start := end
	// Backwards over identifier characters only. A `*` is not one, so the walk stops
	// at it and the name is what lies between — which is the whole trick, and the
	// thing the first version got wrong by stepping *onto* the star and handing
	// `offsetof` an `*label` it answers with "expected identifier before '*'".
	for start > 0 && isIdentifierByte(trimmed[start-1]) {
		start--
	}
	return trimmed[start:end]
}

func isIdentifierByte(b byte) bool {
	return b == '_' || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
}

// fieldNameFor picks the name the C struct gave field `index`.
//
// Read out of the C body rather than invented, because the C probe says
// `offsetof(T, name)` and a name of ours that does not match would ask about a
// field that does not exist — which compiles on some compilers and answers 0.
func fieldNameFor(shape struct {
	name     string
	nilTypes []string
	cBody    []string
}, index int,
) string {
	return cFieldName(shape.cBody[index])
}

// lowerTypes lowers a program that declares one struct.
func lowerTypes(t *testing.T, declaration string) *mir.Program {
	t.Helper()

	// The struct is declared and never constructed: what is under test is the
	// declaration, and constructing it would only add a way for this test to fail
	// for an unrelated reason.
	source := declaration + "\nlet unused = 1;"
	checked := frontend.Check(source, frontend.Options{})
	if !checked.OK {
		t.Fatalf("the program was rejected: %s\n%s", checked.ErrSummary(), source)
	}
	optimized := hir.NewOptimizer().Optimize(hir.NewLowerer().LowerProgram(checked.Lowered))
	return mir.NewLowerer().LowerHIR(optimized)
}
