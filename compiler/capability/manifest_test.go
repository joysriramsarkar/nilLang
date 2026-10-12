package capability

import (
	"strings"
	"testing"
)

// TestRequirementSubsetSemantics verifies the core grant rule: holding extra
// capabilities is fine, holding fewer is not.
func TestRequirementSubsetSemantics(t *testing.T) {
	cases := []struct {
		name    string
		req     Requirement
		granted map[string]bool
		want    bool
	}{
		{"empty requirement always granted", Requirement{}, map[string]bool{}, true},
		{"exact match", Requirement{"filesystem"}, map[string]bool{"filesystem": true}, true},
		{"superset granted", Requirement{"filesystem"}, map[string]bool{"filesystem": true, "network": true}, true},
		{"missing member", Requirement{"filesystem", "network"}, map[string]bool{"filesystem": true}, false},
		{"nothing granted", Requirement{"stdout"}, map[string]bool{}, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.req.Grants(tc.granted); got != tc.want {
				t.Errorf("Grants = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestMissingListsUnmetInOrder checks that the diagnostic names every missing
// capability, in declaration order, so the message is stable.
func TestMissingListsUnmetInOrder(t *testing.T) {
	req := Requirement{"filesystem", "network", "clock"}
	missing := req.Missing(map[string]bool{"network": true})
	if len(missing) != 2 {
		t.Fatalf("Missing returned %v, want two entries", missing)
	}
	if missing[0] != "filesystem" || missing[1] != "clock" {
		t.Errorf("Missing = %v, want [filesystem clock]", missing)
	}
}

// TestKnownOperationsHaveRequirements guards against a new builtin being added
// without a capability declaration. A missing entry would silently fall through
// to "unknown" and be refused at runtime, which is safe but wrong; this test
// makes the failure visible instead.
//
// The list is the set of operations the conformance suite and examples
// exercise. Adding one here means adding it to the requirement table too.
func TestKnownOperationsHaveRequirements(t *testing.T) {
	operations := []string{
		"print", "println", "puts", "input",
		"exec", "exit", "cwd", "chdir",
		"readFile", "writeFile", "fileExists", "isDir",
		"listDir", "makeDir", "removeFile",
		"time", "random",
		"std.time.sleep",
		"std.db.query", "std.db.exec",
		"std.http.get", "std.http.post",
	}
	for _, op := range operations {
		if !IsKnown(op) {
			t.Errorf("operation %q has no declared capability requirement", op)
		}
	}
}

// TestUnknownOperationsAreRefused confirms the conservative default. An
// operation nobody declared is refused rather than assumed harmless.
func TestUnknownOperationsAreRefused(t *testing.T) {
	// Even with every known capability granted, an undeclared operation fails.
	granted := []string{Stdout, Stderr, Stdin, Clock, Environment, Filesystem,
		Process, Network, Database, Crypto, Random, Clipboard, Device, Sensors,
		Camera, Location, Notifications, UI, Async}
	check := GrantChecker(granted)

	err := check("totally.unknown.operation")
	if err == nil {
		t.Fatal("an unknown operation must be refused, even with all capabilities granted")
	}
	denied, ok := AsDenied(err)
	if !ok {
		t.Fatalf("error should be a *DeniedError, got %T: %v", err, err)
	}
	if denied.Reason != "unknown capability requirement" {
		t.Errorf("Reason = %q, want %q", denied.Reason, "unknown capability requirement")
	}
}

// TestGrantCheckerRefusesUngrantedCapability is the core enforcement test.
func TestGrantCheckerRefusesUngrantedCapability(t *testing.T) {
	check := GrantChecker([]string{"stdout"})

	if err := check("print"); err != nil {
		t.Errorf("print should be granted: %v", err)
	}
	if err := check("writeFile"); err == nil {
		t.Error("writeFile must be refused when only stdout is granted")
	}

	denied, ok := AsDenied(check("writeFile"))
	if !ok {
		t.Fatal("refusal should be a *DeniedError")
	}
	if len(denied.Required) != 1 || denied.Required[0] != Filesystem {
		t.Errorf("Required = %v, want [%s]", denied.Required, Filesystem)
	}
	if !strings.Contains(denied.Error(), "writeFile") {
		t.Errorf("message should name the operation: %s", denied.Error())
	}
	if !strings.Contains(denied.Suggestion(), "filesystem") {
		t.Errorf("suggestion should name the capability: %s", denied.Suggestion())
	}
}

// TestGrantCheckerAcceptsAlternateSpellings covers the two vocabularies that
// coexist in the codebase.
func TestGrantCheckerAcceptsAlternateSpellings(t *testing.T) {
	spellings := []string{"filesystem", "Filesystem", "FILE_SYSTEM", "file-system", " FileSystem "}
	for _, spelling := range spellings {
		check := GrantChecker([]string{spelling})
		if err := check("writeFile"); err != nil {
			t.Errorf("grant %q should authorise writeFile: %v", spelling, err)
		}
	}
}

// TestNormalizeMapsResourceNames checks the alias table against the
// capitalised resource vocabulary used by pkg/capability.
func TestNormalizeMapsResourceNames(t *testing.T) {
	cases := map[string]string{
		"Filesystem":  Filesystem,
		"Network":     Network,
		"Database":    Database,
		"Process":     Process,
		"GPS":         Location,
		"Camera":      Camera,
		"Sensors":     Sensors,
		"stdout":      Stdout,
		"crypto":      Crypto,
		"FileSystem":  Filesystem,
		"file_system": Filesystem,
	}
	for input, want := range cases {
		if got := NormalizeName(input); got != want {
			t.Errorf("NormalizeName(%q) = %q, want %q", input, got, want)
		}
	}
}

// TestDeniedErrorCodeMatchesSpecification ties the runtime refusal to the
// documented diagnostic code.
func TestDeniedErrorCodeMatchesSpecification(t *testing.T) {
	if Code != "E0201" {
		t.Errorf("capability denial code = %q, want E0201 per ERROR_MODEL.md", Code)
	}
}
