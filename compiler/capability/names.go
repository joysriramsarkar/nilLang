package capability

import "strings"

// Two vocabularies describe capabilities in this codebase and they must
// interoperate.
//
//   - This package uses lowercase, operation-oriented names such as
//     "filesystem" and "stdout", because they appear inline in a nil.json
//     manifest next to the code that needs them.
//   - pkg/capability uses capitalised resource names such as "Filesystem" and
//     "Network", because they describe profile level resources.
//
// A manifest written by hand, or by an older tool, may use either spelling. The
// canonical form is lowercase, and normalisation maps the capitalised forms
// onto it so that both are accepted at the boundary.

// resourceAliases maps the capitalised pkg/capability names onto the canonical
// lowercase capability names.
var resourceAliases = map[string]string{
	"filesystem": Filesystem,
	"network":    Network,
	"database":   Database,
	"process":    Process,
	"crypto":     Crypto,
	"random":     Random,
	"camera":     Camera,
	"sensors":    Sensors,
	"device":     Device,
	"gpu":        "gpu",
	"audio":      "audio",
	"printer":    "printer",
	"scanner":    "scanner",
	"bluetooth":  "bluetooth",
	"ai":         "ai",
	// pkg/capability calls location "GPS".
	"gps": Location,
	// Terminal streams are one resource in pkg/capability.
	"stdout":        Stdout,
	"stderr":        Stderr,
	"stdin":         Stdin,
	"clipboard":     Clipboard,
	"notifications": Notifications,
}

// NormalizeName returns the canonical lowercase spelling of a capability name.
//
// Matching is case insensitive and treats '-' and '_' as equivalent, so
// "File-System", "file_system", and "Filesystem" all normalise to "filesystem".
//
// The alias table is consulted afterwards, which is what lets "GPS" resolve to
// the canonical "location".
func NormalizeName(name string) string {
	trimmed := strings.TrimSpace(name)
	lowered := strings.ToLower(trimmed)
	lowered = strings.ReplaceAll(lowered, "-", "")
	lowered = strings.ReplaceAll(lowered, "_", "")
	lowered = strings.ReplaceAll(lowered, " ", "")
	if canonical, ok := resourceAliases[lowered]; ok {
		return canonical
	}
	return lowered
}

// NormalizeList canonicalises every entry of a granted capability list.
func NormalizeList(names []string) []string {
	out := make([]string, 0, len(names))
	for _, name := range names {
		if normalized := NormalizeName(name); normalized != "" {
			out = append(out, normalized)
		}
	}
	return out
}
