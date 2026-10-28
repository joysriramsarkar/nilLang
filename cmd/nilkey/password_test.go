package main

import "testing"

// Password resolution order: an explicit flag value wins (and warns on
// stderr), then the NILKEY_PASSWORD environment variable. The interactive
// prompt is the last resort and is not exercised here — it reads a
// terminal.
func TestResolvePasswordOrder(t *testing.T) {
	t.Setenv(PasswordEnvVar, "env-password")

	if got := resolvePassword("flag-password", "prompt: "); got != "flag-password" {
		t.Errorf("flag value: got %q, want flag-password", got)
	}
	if got := resolvePassword("", "prompt: "); got != "env-password" {
		t.Errorf("env value: got %q, want env-password", got)
	}
}
