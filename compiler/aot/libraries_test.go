package aot

import (
	"strings"
	"testing"
)

// TestALibraryNameBecomesALinkFlag is the whole of what `nil.json`'s libraries
// list does.
//
// A program declares `extern "C" fn sin(x: Float) -> Float;` and says *what* it
// needs — a symbol. `nil.json` says *where it lives* — `-lm`. Neither is enough
// alone, and a build that inferred one from the other would either link every
// library a program mentions or fail to link one it needs.
func TestALibraryNameBecomesALinkFlag(t *testing.T) {
	cases := []struct {
		name  string
		given []string
		want  []string
	}{
		{"one library", []string{"m"}, []string{"-lm"}},
		{"several", []string{"pthread", "m"}, []string{"-lm", "-lpthread"}},
		{"none", nil, nil},

		// The spellings a developer reaches for out of habit. All three name the
		// same library, and which one someone writes depends on the platform they
		// last worked on — so accepting all three means `nil.json` takes what a
		// person would write and the linker still receives one spelling.
		{"already prefixed", []string{"-lm"}, []string{"-lm"}},
		{"a shared object", []string{"libm.so"}, []string{"-lm"}},
		{"a macOS library", []string{"libSystem.B.dylib"}, []string{"-lSystem.B"}},
		{"a static archive", []string{"libz.a"}, []string{"-lz"}},

		// Two spellings of one library, from a program and from nil.json combined.
		// Harmless on every linker tried and wasteful on all of them.
		{"the same library twice", []string{"m", "libm.so"}, []string{"-lm"}},
		{"blank entries", []string{"", "  ", "m"}, []string{"-lm"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := libraryFlags(tc.given)
			if strings.Join(got, " ") != strings.Join(tc.want, " ") {
				t.Errorf("libraryFlags(%q) = %q, wanted %q", tc.given, got, tc.want)
			}
		})
	}
}

// TestTheLinkOrderIsFixed says what the flags are for.
//
// Sorted so that two builds of the same project produce the same link line, which
// is what lets a build be compared with the last one and a cache be keyed on it.
func TestTheLinkOrderIsFixed(t *testing.T) {
	got := libraryFlags([]string{"z", "m", "pthread"})
	want := []string{"-lm", "-lpthread", "-lz"}

	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("the order is %q, wanted %q", got, want)
	}
}

// TestAdviceNamesALibraryNothingAskedFor is the warning's whole job.
//
// A library in `nil.json` that no `extern` mentions is usually a typo, and a typo
// in a link line surfaces much later as "undefined reference to" a symbol nobody
// can connect back to the name that was wrong.
func TestAdviceNamesALibraryNothingAskedFor(t *testing.T) {
	advice := DescribeLibraries([]string{"m", "typo"}, []string{"m"})
	if len(advice.Unused) != 1 || advice.Unused[0] != "typo" {
		t.Errorf("the advice names %q, wanted [typo]", advice.Unused)
	}
	if !strings.Contains(advice.String(), "typo") {
		t.Errorf("the warning does not name the library: %q", advice.String())
	}
}

// TestAdviceIsSilentWhenEveryLibraryIsWanted keeps the warning from becoming
// noise, which is how warnings get ignored.
func TestAdviceIsSilentWhenEveryLibraryIsWanted(t *testing.T) {
	advice := DescribeLibraries([]string{"m", "pthread"}, []string{"m", "pthread"})
	if len(advice.Unused) != 0 {
		t.Errorf("the advice names %q when nothing is unused", advice.Unused)
	}
	if advice.String() != "" {
		t.Errorf("a clean build produced a warning: %q", advice.String())
	}
}

// TestAdviceSaysNothingWithoutDeclarations is the case a naive check gets wrong.
//
// With no `extern` declarations, every library looks unused. Reporting all of them
// would be a warning on every project that links the runtime and declares nothing,
// which is a warning nobody reads.
func TestAdviceSaysNothingWithoutDeclarations(t *testing.T) {
	advice := DescribeLibraries([]string{"m", "pthread"}, nil)
	if len(advice.Unused) != 0 {
		t.Errorf("the advice names %q with nothing declared", advice.Unused)
	}
	if advice.String() != "" {
		t.Errorf("a build with no externs produced a warning: %q", advice.String())
	}
}
