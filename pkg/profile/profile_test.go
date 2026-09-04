package profile

import (
	"testing"
)

func TestProfileRegistry(t *testing.T) {
	profiles := []string{"web", "mobile", "server", "data", "os", "embedded", "core"}

	for _, id := range profiles {
		p, err := Get(id)
		if err != nil {
			t.Fatalf("failed to get profile %s: %v", id, err)
		}
		if string(p.ID) != id {
			t.Errorf("expected ID %s, got %s", id, p.ID)
		}
		if len(p.AllowedCaps) == 0 {
			t.Errorf("profile %s has no allowed caps", id)
		}
	}
}

func TestValidateProfileCaps(t *testing.T) {
	// Web profile requesting Process should fail
	res, err := ValidateProfileCaps("web", []string{"Network", "Process"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Valid {
		t.Errorf("expected validation failure for Process on web")
	}

	// Server profile requesting Network and Database should succeed
	resServer, err := ValidateProfileCaps("server", []string{"Network", "Database", "Filesystem"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resServer.Valid {
		t.Errorf("expected server validation to pass, got: %v", resServer.Violations)
	}
}
