package main

import (
	"fmt"
	"os"

	"github.com/joysriramsarkar/nilLang/compiler/diagnostics"
)

// printWarnings writes the non-error diagnostics to stderr.
//
// # Why this is a function rather than a block at each call site
//
// Warnings are printed on the way *through*, not on failure, so every entry point
// that runs the frontend gate has to remember to do it. Four of them forgot, and a
// warning that is stored but never shown is indistinguishable from one that was
// never produced — which is the whole difference between saying something and
// not saying it.
//
// It goes to stderr so it does not mix into a program's own output. A developer
// piping `nil run` into a file should get the file, not a remark about a type.
//
// It returns nothing and changes nothing: the program continues either way. The
// severity is the whole meaning of the word.
func printWarnings(diags []*diagnostics.Diagnostic) {
	for _, d := range diags {
		if d == nil || d.Severity == diagnostics.SeverityError {
			continue
		}
		fmt.Fprintf(os.Stderr, "⚠️  %s\n", d.String())
	}
}
