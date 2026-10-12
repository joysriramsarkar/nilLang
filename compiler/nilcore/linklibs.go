package nilcore

// RuntimeLibraries names the system libraries the runtime itself needs linked
// for targetOS ("linux", "windows", "macos", ...), before any project library.
//
// The runtime calls `sqrt`, `round` and `pow`, which live in `libm` on unix
// and in the C runtime on Windows — so unix links `m` and Windows links
// nothing extra. A link that omits it fails on Linux with `undefined
// reference to 'sqrt'`; a link that keeps it on Windows fails on MSVC with
// `LNK1181: cannot open input file 'm.lib'`, because no such library exists
// there (and mingw links its math implicitly, so it needs nothing either).
// Both failures were found by running the same suite on both.
//
// It lives here rather than beside the link line so there is exactly one list:
// the AOT builder merges it with the project's libraries, and the tests that
// link the runtime's objects directly use the flags below. A second list
// would agree with this one until the day the runtime called something new.
func RuntimeLibraries(targetOS string) []string {
	if targetOS == "windows" {
		return nil
	}
	return []string{"m"}
}

// RuntimeLinkFlags are the linker flags for RuntimeLibraries, ready to append
// to a link line that already holds the runtime's objects.
func RuntimeLinkFlags(targetOS string) []string {
	libraries := RuntimeLibraries(targetOS)
	flags := make([]string, 0, len(libraries))
	for _, name := range libraries {
		flags = append(flags, "-l"+name)
	}
	return flags
}
