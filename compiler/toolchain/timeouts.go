package toolchain

import "time"

// Timeouts for the external tools.
//
// They exist because a build that shells out can hang: a compiler waiting on a
// lock file, a linker waiting on a core dump, an antivirus scanner on Windows
// holding a file open for a minute. A build that hangs holds a machine and reports
// nothing; one that gives up says so.
const (
	// versionTimeout is generous for reading `--version`, because on Windows the
	// first run of a freshly-installed compiler may be scanning itself.
	versionTimeout = 30 * time.Second

	// execTimeout bounds a compilation.
	//
	// It is long — LLVM optimising at `-O3` is slow — because a timeout that
	// fires during a legitimate large build is worse than a slow one: the work is
	// lost either way, but a timeout reports as a failure rather than as slowness.
	execTimeout = 10 * time.Minute
)
