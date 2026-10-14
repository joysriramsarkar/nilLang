// Package capability declares which system capabilities Nilang operations
// require, and provides the metadata the compiler and runtime need to enforce
// those requirements.
//
// The compiler cannot import pkg/capability: that package depends on
// application level concerns, and tests/architecture_test.go forbids the
// dependency. The canonical declarations therefore live here, in the compiler
// tree, and pkg/capability consumes them.
//
// See docs/spec/CAPABILITIES.md for the normative model.
package capability

import "sort"

// Requirement is the set of capabilities an operation needs in order to run.
//
// A Requirement with no entries needs nothing and is always granted.
type Requirement []string

// Grants reports whether granted satisfies every entry of r.
//
// The comparison is a subset test: holding extra capabilities is fine, holding
// fewer is not. An empty requirement is trivially satisfied, so pure operations
// never need a capability grant.
func (r Requirement) Grants(granted map[string]bool) bool {
	for _, name := range r {
		if !granted[name] {
			return false
		}
	}
	return true
}

// Missing returns the entries of r that granted does not satisfy, in order.
func (r Requirement) Missing(granted map[string]bool) []string {
	var missing []string
	for _, name := range r {
		if !granted[name] {
			missing = append(missing, name)
		}
	}
	return missing
}

// String renders the requirement as a comma separated list, or "none".
func (r Requirement) String() string {
	if len(r) == 0 {
		return "none"
	}
	out := ""
	for i, name := range r {
		if i > 0 {
			out += ", "
		}
		out += name
	}
	return out
}

// Capability names.
//
// These strings appear in nil.json manifests, in `requires` clauses, and in
// diagnostics, so they are part of the toolchain's public surface.
const (
	Core          = "core"
	Stdout        = "stdout"
	Stderr        = "stderr"
	Stdin         = "stdin"
	Clock         = "clock"
	Environment   = "environment"
	Filesystem    = "filesystem"
	Process       = "process"
	Network       = "network"
	Database      = "database"
	Crypto        = "crypto"
	Random        = "random"
	Clipboard     = "clipboard"
	Device        = "device"
	Sensors       = "sensors"
	Camera        = "camera"
	Location      = "location"
	Notifications = "notifications"
	UI            = "ui"
	Async         = "async"
)

// builtinRequirements maps a builtin or std module function name to the
// capabilities it needs.
//
// Only operations that reach outside the language runtime appear here. Anything
// absent requires nothing, which is the correct default for pure computation
// and data manipulation.
//
// The list is deliberately explicit rather than inferred from a naming
// convention: an inferred rule would either miss a dangerous operation or deny
// a harmless one, and both failure modes are security relevant.
var builtinRequirements = map[string]Requirement{
	// Terminal output and input.
	"print":   {Stdout},
	"println": {Stdout},
	"puts":    {Stdout},
	"input":   {Stdin},
	// clear emits ANSI escape codes to the terminal — no capability needed,
	// but the name must be KNOWN so the compiler emits OpNativeCall instead
	// of a closure call the VM cannot resolve.
	"clear": {},

	// Process and environment.
	"exec":  {Process},
	"exit":  {Process},
	"cwd":   {Environment},
	"chdir": {Environment},

	// Filesystem.
	"readFile":   {Filesystem},
	"writeFile":  {Filesystem},
	"fileExists": {Filesystem},
	"isDir":      {Filesystem},
	"listDir":    {Filesystem},
	"makeDir":    {Filesystem},
	"removeFile": {Filesystem},

	// Time and randomness.
	"time":   {Clock},
	"sleep":  {Clock},
	"random": {Random},

	// std modules.
	"std.time.sleep":          {Clock},
	"std.http.get":            {Network},
	"std.http.post":           {Network},
	"std.http.request":        {Network},
	"std.net.dial":            {Network},
	"std.net.listen":          {Network},
	"std.db.query":            {Database},
	"std.db.exec":             {Database},
	"std.crypto.randomBytes":  {Random},
	"std.clipboard.read":      {Clipboard},
	"std.clipboard.write":     {Clipboard},
	"std.device.batteryLevel": {Device},
	"std.device.model":        {Device},
	"std.sensors.read":        {Sensors},
	"std.camera.capture":      {Camera},
	"std.location.current":    {Location},
	"std.notify.send":         {Notifications},
}

// RequirementsFor returns the capabilities an operation needs.
//
// The second result reports whether the operation is known at all. An unknown
// operation is not silently treated as pure: callers decide, and the compiler
// treats unknown operations as requiring Core so that a typo in a manifest
// cannot accidentally widen access.
func RequirementsFor(name string) (Requirement, bool) {
	req, ok := builtinRequirements[name]
	return req, ok
}

// IsKnown reports whether the operation appears in the requirement table.
func IsKnown(name string) bool {
	_, ok := builtinRequirements[name]
	return ok
}

// KnownOperations lists every operation with a declared requirement, sorted so
// that generated listings and diagnostics are stable.
func KnownOperations() []string {
	out := make([]string, 0, len(builtinRequirements))
	for name := range builtinRequirements {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// GrantChecker builds the runtime gate the VM consults before running a native
// operation.
//
// It is constructed from the capabilities a program was granted, which come
// from the nil.json manifest at build time and the profile at run time. A
// program granted nothing can still run pure code; the first native call it
// makes is what gets refused.
//
// Names are normalised on the way in, so a manifest may spell a capability
// "Filesystem", "file-system", or "filesystem".
func GrantChecker(granted []string) func(string) error {
	set := make(map[string]bool, len(granted))
	for _, name := range NormalizeList(granted) {
		set[name] = true
	}

	return func(operation string) error {
		req, known := RequirementsFor(operation)
		if !known {
			// Unknown operations are refused rather than assumed safe. This is
			// the conservative direction: a false refusal is a visible bug, a
			// false grant is a vulnerability.
			return &DeniedError{
				Operation: operation,
				Required:  Requirement{Core},
				Reason:    "unknown capability requirement",
			}
		}
		if req.Grants(set) {
			return nil
		}
		return &DeniedError{
			Operation: operation,
			Required:  req,
			Granted:   NormalizeList(granted),
			Reason:    "capability not granted",
		}
	}
}
