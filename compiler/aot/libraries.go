package aot

import (
	"fmt"
	"strings"

	"github.com/joysriramsarkar/nilLang/compiler/llvm"
	"github.com/joysriramsarkar/nilLang/compiler/nilcore"
)

// libraryFlags turns library names into linker arguments.
//
// # Why the name is not quoted or path-joined here
//
// A name is a name. `m` is a name on every platform that has it and a file on
// none; a path is a path and belongs in `nil.json` written as one. Deciding which
// a string is would mean guessing, and guessing wrong produces a linker error
// about a file nobody wrote — so the split is made where the two are written
// differently instead.
//
// # Why `-l` and not a bare name
//
// The `-l` prefix is what tells the driver "look this up in the library search
// path" rather than "this is a file". Passing the bare name works on some drivers
// and not others, and the ones where it works are the ones where it happens to
// find the same file.
//
// # Why duplicates are dropped
//
// Two `-l` flags for one library are harmless on every linker tried and wasteful
// on all of them, and they arrive easily: a program can name a library and so can
// `nil.json`, and the two are meant to be combined. Sorting makes the command the
// same for the same set of libraries, which is what lets a build be compared to
// the last one.
func libraryFlags(libraries []string) []string {
	seen := map[string]bool{}
	unique := make([]string, 0, len(libraries))
	for _, name := range libraries {
		name = trimLibraryName(name)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		unique = append(unique, name)
	}
	sortStrings(unique)

	flags := make([]string, 0, len(unique))
	for _, name := range unique {
		flags = append(flags, "-l"+name)
	}
	return flags
}

// trimLibraryName removes the decorations a person writes out of habit.
//
// `-lm` and `libm.so` and `m` all name the same library, and which spelling a
// developer reaches for depends on the platform they last worked on. Normalising
// them here means `nil.json` accepts what a person would write, and the linker
// still receives one spelling.
func trimLibraryName(name string) string {
	for {
		// `-l` and `-L` are consumed *together* rather than one dash at a time.
		// Stripping only the dash turned `-lm` into `lm`, and the caller then wrote
		// `-llm` — a library that does not exist, reported by the linker as an
		// undefined symbol in a file nobody named.
		if strings.HasPrefix(name, "-l") || strings.HasPrefix(name, "-L") {
			name = name[2:]
			continue
		}
		name = strings.TrimLeft(name, "- \t")
		if name == "" {
			return ""
		}

		// The platform's file spellings. Each suffix is trimmed by its own length
		// rather than by a count written next to it: `.a` is two characters and
		// `.dylib` is six, and a literal that disagrees with its own constant drops
		// a character of the library's name without saying so.
		for _, suffix := range []string{".dylib", ".so", ".a", ".lib"} {
			if base, found := strings.CutSuffix(name, suffix); found {
				name = strings.TrimPrefix(base, "lib")
				break
			}
		}
		return name
	}
}

// sortStrings sorts in place. A tiny insertion sort rather than a `sort` import,
// because the lists here are the length of a `nil.json` libraries list — usually
// zero, occasionally two — and a function that imports nothing for the common case
// is a function that costs nothing in it.
func sortStrings(values []string) {
	for i := 1; i < len(values); i++ {
		value := values[i]
		j := i - 1
		for j >= 0 && values[j] > value {
			values[j+1] = values[j]
			j--
		}
		values[j+1] = value
	}
}

// runtimeLibraryNames names the system libraries the runtime itself needs
// linked on the target, before any project library. It delegates to the
// runtime's own package, because the need (`sqrt`, `round`, `pow` live in
// `libm` on unix and in the C runtime on Windows) is a property of the
// runtime's sources, and a second list here would agree with it until the day
// the runtime called something new.
//
// A link that omits it fails on Linux with `undefined reference to 'sqrt'`;
// a link that keeps it on Windows fails on MSVC with `LNK1181: cannot open
// input file 'm.lib'`, because no such library exists there. Both failures
// were found by running the same suite on both.
func runtimeLibraryNames(targetOS llvm.OS) []string {
	return nilcore.RuntimeLibraries(string(targetOS))
}

// RuntimeLinkFlags are the linker flags the runtime itself needs on the
// target, ready to append to a link line that already holds the runtime's
// objects. Callers that link `nilcore` objects without going through
// `Build` — the FFI self-test is one — need exactly these and nothing else.
func RuntimeLinkFlags(targetOS llvm.OS) []string {
	return nilcore.RuntimeLinkFlags(string(targetOS))
}

// effectiveLibraries merges what the project asked for with what the runtime
// needs, dropping what the platform must not see.
//
// A project that declares `"libraries": ["m"]` (the documented spelling for
// `sin`) keeps working everywhere: unix links it once — the deduplication in
// `libraryFlags` collapses the project's `m` and the runtime's — and Windows
// drops it, because MSVC has no `m.lib` (its math is in the C runtime) and
// mingw links its math implicitly. Dropping rather than translating, because
// there is nothing to translate to: any spelling of `m` on Windows names a
// file that does not exist.
func effectiveLibraries(targetOS llvm.OS, requested []string) []string {
	merged := append(append([]string(nil), requested...), runtimeLibraryNames(targetOS)...)
	if targetOS != llvm.OSWindows {
		return merged
	}
	kept := merged[:0]
	for _, name := range merged {
		if trimLibraryName(name) == "m" {
			continue
		}
		kept = append(kept, name)
	}
	return kept
}

// LibraryAdvice is what the build says about the libraries it was given.
//
// It is a warning and not an error because a project may legitimately link a
// library no `extern` mentions — one reached through a C constructor, or through
// a library that pulls in another. What it does catch is the common failure: a
// library named in `nil.json` that nothing uses is usually a typo, and a typo in a
// link line surfaces much later as "undefined reference to" a symbol nobody can
// connect back to the name that was wrong.
type LibraryAdvice struct {
	// Linked is every library passed to the linker.
	Linked []string

	// Unused is a library no `extern` declaration refers to.
	Unused []string
}

// DescribeLibraries compares the libraries asked for against the ones declared.
//
// `declared` is the set of library names the program's `extern "C"` declarations
// could plausibly need. A declaration names a *symbol*, not a library — `sin` lives
// in `m` on Linux and in the C runtime everywhere else — so a program cannot say
// which library it wants, and only `nil.json` can. The comparison is therefore
// between the libraries asked for and the ones the project declared needing, and
// the honest result is usually "no advice either way".
func DescribeLibraries(linked, declared []string) LibraryAdvice {
	advice := LibraryAdvice{Linked: append([]string(nil), linked...)}

	wanted := map[string]bool{}
	for _, name := range declared {
		if trimmed := trimLibraryName(name); trimmed != "" {
			wanted[trimmed] = true
		}
	}
	// With no declarations there is nothing to compare against, and every library
	// would look unused. Saying so is better than reporting a list the reader
	// cannot act on.
	if len(wanted) == 0 {
		return advice
	}

	for _, name := range linked {
		if trimmed := trimLibraryName(name); trimmed != "" && !wanted[trimmed] {
			advice.Unused = append(advice.Unused, trimmed)
		}
	}
	sortStrings(advice.Unused)
	return advice
}

// String renders the advice as a single warning line, or "" when there is none.
func (a LibraryAdvice) String() string {
	if len(a.Unused) == 0 {
		return ""
	}
	names := ""
	for i, name := range a.Unused {
		if i > 0 {
			names += ", "
		}
		names += name
	}
	return fmt.Sprintf("%s: linked but no extern declaration refers to it; "+
		"if a symbol is undefined, this name is the first thing to check", names)
}
