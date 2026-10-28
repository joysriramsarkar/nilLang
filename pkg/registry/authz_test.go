package registry

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
)

// publishWithChecksum posts name@version with explicit file bytes and an
// explicit declared checksum (empty means "none declared").
func publishWithChecksum(t *testing.T, s *Server, key, name, version string, content []byte, checksum string) int {
	t.Helper()
	var form bytes.Buffer
	w := multipart.NewWriter(&form)
	meta, _ := json.Marshal(PublishRequest{Name: name, Version: version, Checksum: checksum})
	_ = w.WriteField("metadata", string(meta))
	fw, _ := w.CreateFormFile("package", "pkg.nilax")
	_, _ = fw.Write(content)
	_ = w.Close()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/packages/publish", &form)
	req.Header.Set("Content-Type", w.FormDataContentType())
	if key != "" {
		req.Header.Set("X-API-Key", key)
	}
	rec := httptest.NewRecorder()
	s.handlePublish(rec, req)
	return rec.Code
}

func deletePackage(t *testing.T, s *Server, key, name string) int {
	t.Helper()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/packages/"+name, nil)
	if key != "" {
		req.Header.Set("X-API-Key", key)
	}
	rec := httptest.NewRecorder()
	s.handlePackageByID(rec, req)
	return rec.Code
}

// A package name belongs to its first publisher: another registered user's
// valid key must not let them overwrite or delete it, while the owner and
// an admin still can.
func TestPackageOwnershipEnforced(t *testing.T) {
	s := newTestServer(t)
	adminKey := registerUser(t, s, "admin", "admin-password") // first user = admin
	aliceKey := registerUser(t, s, "alice", "alice-password")
	bobKey := registerUser(t, s, "bob", "bob-password")

	if code := publishPackage(t, s, aliceKey, "widget", "1.0.0"); code != http.StatusCreated {
		t.Fatalf("alice first publish: got %d, want 201", code)
	}

	// Bob cannot publish over Alice's name, even with his own valid key.
	if code := publishPackage(t, s, bobKey, "widget", "2.0.0"); code != http.StatusForbidden {
		t.Fatalf("bob overwrite of alice's package: got %d, want 403", code)
	}
	// Bob cannot delete it either.
	if code := deletePackage(t, s, bobKey, "widget"); code != http.StatusForbidden {
		t.Fatalf("bob delete of alice's package: got %d, want 403", code)
	}
	// Bob's own name is fine.
	if code := publishPackage(t, s, bobKey, "bobthing", "1.0.0"); code != http.StatusCreated {
		t.Fatalf("bob own publish: got %d, want 201", code)
	}
	// Alice can publish a new version of her own package.
	if code := publishPackage(t, s, aliceKey, "widget", "1.1.0"); code != http.StatusCreated {
		t.Fatalf("alice own new version: got %d, want 201", code)
	}
	// The recorded owner is Alice.
	pkg, err := s.storage.GetPackageByName("widget")
	if err != nil {
		t.Fatalf("GetPackageByName: %v", err)
	}
	if pkg.Owner != "alice" {
		t.Fatalf("widget owner = %q, want alice", pkg.Owner)
	}
	// An admin may delete another user's package.
	if code := deletePackage(t, s, adminKey, "widget"); code != http.StatusOK {
		t.Fatalf("admin delete: got %d, want 200", code)
	}
}

// The server verifies the declared checksum against the bytes it actually
// received, instead of copying the claim into the index.
func TestPublishChecksumVerifiedServerSide(t *testing.T) {
	s := newTestServer(t)
	key := registerUser(t, s, "erin", "erin-password")

	content := []byte("the real package bytes")
	sum := sha256.Sum256(content)
	good := hex.EncodeToString(sum[:])
	bad := hex.EncodeToString(sha256.New().Sum(nil))

	if code := publishWithChecksum(t, s, key, "checksummed", "1.0.0", content, bad); code != http.StatusBadRequest {
		t.Fatalf("mismatched checksum publish: got %d, want 400", code)
	}
	// The rejected publish must not be registered.
	if _, err := s.storage.GetPackageByName("checksummed"); err == nil {
		t.Fatal("checksum-mismatched package was registered")
	}
	if code := publishWithChecksum(t, s, key, "checksummed", "1.0.0", content, good); code != http.StatusCreated {
		t.Fatalf("matching checksum publish: got %d, want 201", code)
	}

	// With no checksum declared, the server-computed sum lands in the index.
	if code := publishWithChecksum(t, s, key, "undeclared", "1.0.0", content, ""); code != http.StatusCreated {
		t.Fatalf("no-checksum publish: got %d, want 201", code)
	}
	pkg, err := s.storage.GetPackageByName("undeclared")
	if err != nil {
		t.Fatalf("GetPackageByName: %v", err)
	}
	if pkg.Checksum != good {
		t.Fatalf("index checksum = %q, want server-computed %q", pkg.Checksum, good)
	}
}

// Users (and ownership) persist in the data directory: after a restart the
// account still logs in, its key still works, anonymous publish is still
// refused, and the no-users bootstrap state does not come back.
func TestUsersAndOwnershipSurviveRestart(t *testing.T) {
	dir := t.TempDir()
	cfg := &ServerConfig{Host: "127.0.0.1", Port: 0, DataDir: dir, MaxUploadMB: 5, EnableAuth: false}

	s1, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	key := registerUser(t, s1, "frank", "frank-password")
	if code := publishPackage(t, s1, key, "durable", "1.0.0"); code != http.StatusCreated {
		t.Fatalf("pre-restart publish: got %d, want 201", code)
	}

	// "Restart": a fresh server over the same data directory.
	s2, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("NewServer after restart: %v", err)
	}

	// Login still works — password hash/salt were persisted.
	body, _ := json.Marshal(map[string]string{"username": "frank", "password": "frank-password"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/login", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	s2.handleLogin(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("login after restart: got %d, want 200 (body %s)", rec.Code, rec.Body.String())
	}

	// Anonymous publish is refused: bootstrap did not come back.
	if code := publishPackage(t, s2, "", "sneaky", "1.0.0"); code != http.StatusUnauthorized {
		t.Fatalf("anonymous publish after restart: got %d, want 401", code)
	}
	// The original API key still works, and ownership survived with it.
	if code := publishPackage(t, s2, key, "durable", "1.1.0"); code != http.StatusCreated {
		t.Fatalf("owner publish after restart: got %d, want 201", code)
	}
	otherKey := registerUser(t, s2, "mallory", "mallory-password")
	if code := publishPackage(t, s2, otherKey, "durable", "9.9.9"); code != http.StatusForbidden {
		t.Fatalf("non-owner publish after restart: got %d, want 403", code)
	}
}
