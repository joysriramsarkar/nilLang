package registry

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newTestServer(t *testing.T) *Server {
	t.Helper()
	s, err := NewServer(&ServerConfig{
		Host:        "127.0.0.1",
		Port:        0,
		DataDir:     t.TempDir(),
		MaxUploadMB: 5,
		EnableAuth:  false,
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	return s
}

func registerUser(t *testing.T, s *Server, username, password string) string {
	t.Helper()
	body, _ := json.Marshal(map[string]string{
		"username": username, "email": username + "@x.test", "password": password,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/register", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	s.handleRegister(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("register %s: status=%d body=%s", username, rec.Code, rec.Body.String())
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode register response: %v", err)
	}
	key, _ := resp["api_key"].(string)
	if key == "" {
		t.Fatal("register response carries no api_key")
	}
	// The user object must not leak the key.
	if user, ok := resp["user"].(map[string]interface{}); ok {
		if _, leaked := user["api_key"]; leaked {
			t.Fatal("register response leaks api_key inside user object")
		}
	}
	return key
}

func publishPackage(t *testing.T, s *Server, key, name, version string) int {
	t.Helper()
	var form bytes.Buffer
	w := multipart.NewWriter(&form)
	meta, _ := json.Marshal(PublishRequest{Name: name, Version: version})
	_ = w.WriteField("metadata", string(meta))
	fw, _ := w.CreateFormFile("package", "pkg.nilax")
	_, _ = fw.Write([]byte("fake-package-bytes"))
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

func TestPublishRequiresKeyOnceUsersExist(t *testing.T) {
	s := newTestServer(t)

	// Bootstrap publish on a fresh server succeeds without a key.
	if code := publishPackage(t, s, "", "alpha", "1.0.0"); code != http.StatusCreated {
		t.Fatalf("bootstrap publish: got %d, want 201", code)
	}

	key := registerUser(t, s, "alice", "s3cret-password")

	// Anonymous publish is now refused.
	if code := publishPackage(t, s, "", "beta", "1.0.0"); code != http.StatusUnauthorized {
		t.Fatalf("anonymous publish after registration: got %d, want 401", code)
	}
	// Wrong key is refused.
	if code := publishPackage(t, s, "nil_deadbeef", "beta", "1.0.0"); code != http.StatusUnauthorized {
		t.Fatalf("bad-key publish: got %d, want 401", code)
	}
	// Correct key succeeds.
	if code := publishPackage(t, s, key, "beta", "1.0.0"); code != http.StatusCreated {
		t.Fatalf("authed publish: got %d, want 201", code)
	}
}

func TestPublishRejectsTraversalFilenames(t *testing.T) {
	s := newTestServer(t)
	key := registerUser(t, s, "bob", "another-s3cret")

	for _, tc := range [][2]string{
		{"../../evil", "1.0.0"},
		{"ok", "../evil"},
		{"..", "1.0.0"},
	} {
		if code := publishPackage(t, s, key, tc[0], tc[1]); code != http.StatusBadRequest {
			t.Fatalf("publish %q %q: got %d, want 400", tc[0], tc[1], code)
		}
	}
	uploads, _ := filepath.Glob(filepath.Join(s.config.DataDir, "uploads", "*.nilax"))
	for _, f := range uploads {
		if strings.Contains(filepath.Base(f), "evil") && strings.Contains(f, "..") {
			t.Fatalf("traversal file escaped uploads: %s", f)
		}
	}
	_ = os.RemoveAll(s.config.DataDir)
}

func TestLoginVerifiesPassword(t *testing.T) {
	s := newTestServer(t)
	key := registerUser(t, s, "carol", "right-password")
	if key == "" {
		t.Fatal("no key issued")
	}

	login := func(password string) int {
		body, _ := json.Marshal(map[string]string{"username": "carol", "password": password})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/users/login", bytes.NewReader(body))
		rec := httptest.NewRecorder()
		s.handleLogin(rec, req)
		return rec.Code
	}
	if code := login("right-password"); code != http.StatusOK {
		t.Fatalf("correct password: got %d, want 200", code)
	}
	if code := login("wrong-password"); code != http.StatusUnauthorized {
		t.Fatalf("wrong password: got %d, want 401", code)
	}
	if code := login(""); code != http.StatusUnauthorized {
		t.Fatalf("empty password: got %d, want 401", code)
	}
}

func TestDeleteRequiresKey(t *testing.T) {
	s := newTestServer(t)
	key := registerUser(t, s, "dave", "dave-password")
	if code := publishPackage(t, s, key, "gone", "1.0.0"); code != http.StatusCreated {
		t.Fatalf("setup publish: got %d", code)
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/packages/gone", nil)
	rec := httptest.NewRecorder()
	s.handlePackageByID(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous delete: got %d, want 401", rec.Code)
	}

	req = httptest.NewRequest(http.MethodDelete, "/api/v1/packages/gone", nil)
	req.Header.Set("X-API-Key", key)
	rec = httptest.NewRecorder()
	s.handlePackageByID(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("authed delete: got %d, want 200", rec.Code)
	}
}
