package nilcore

import (
	"strings"
	"testing"
)

// The runtime's own link needs, by target: libm everywhere but Windows, where
// the math lives in the C runtime. Found by linking the same objects on both.
func TestTheRuntimeLinksItsOwnMathLibrary(t *testing.T) {
	for _, targetOS := range []string{"linux", "macos", "android", "onuron"} {
		if got := RuntimeLinkFlags(targetOS); strings.Join(got, " ") != "-lm" {
			t.Errorf("RuntimeLinkFlags(%s) = %q, wanted [-lm]", targetOS, got)
		}
	}
	if got := RuntimeLinkFlags("windows"); len(got) != 0 {
		t.Errorf("RuntimeLinkFlags(windows) = %q, wanted nothing", got)
	}
}
