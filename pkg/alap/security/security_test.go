package security

import (
	"testing"
	"time"

	"github.com/joysriramsarkar/nilLang/pkg/alap/routing"
)

func TestJWTLifecycle(t *testing.T) {
	secret := "super-secret-key-1234"

	claims := JWTClaims{
		Subject:   "user-42",
		Roles:     []string{"editor", "author"},
		ExpiresAt: time.Now().Add(1 * time.Hour).Unix(),
	}

	token, err := GenerateJWT(claims, secret)
	if err != nil {
		t.Fatalf("failed to generate JWT: %v", err)
	}

	// Verify valid token
	verified, err := VerifyJWT(token, secret)
	if err != nil {
		t.Fatalf("failed to verify valid JWT: %v", err)
	}
	if verified.Subject != "user-42" {
		t.Errorf("expected subject user-42, got %s", verified.Subject)
	}

	// Verify tampering fails
	tampered := token + "tamper"
	_, err = VerifyJWT(tampered, secret)
	if err == nil {
		t.Errorf("expected tampered token verification to fail")
	}

	// Verify expired token
	expiredClaims := JWTClaims{
		Subject:   "user-99",
		ExpiresAt: time.Now().Add(-1 * time.Hour).Unix(),
	}
	expiredToken, _ := GenerateJWT(expiredClaims, secret)
	_, err = VerifyJWT(expiredToken, secret)
	if err == nil {
		t.Errorf("expected expired token to fail verification")
	}
}

func TestPasswordHashing(t *testing.T) {
	pwd := "MySecurePass!2026"
	hash, err := HashPassword(pwd)
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}

	if !VerifyPassword(pwd, hash) {
		t.Errorf("expected password verification to succeed")
	}

	if VerifyPassword("WrongPassword", hash) {
		t.Errorf("expected wrong password to fail verification")
	}
}

func TestSignedCookies(t *testing.T) {
	secret := "cookie-secret"
	sessionID := "sess_xyz_123"

	signed := SignCookie(sessionID, secret)
	val, ok := VerifySignedCookie(signed, secret)
	if !ok || val != sessionID {
		t.Errorf("cookie verification failed: ok=%v val=%s", ok, val)
	}

	// Tampered cookie
	_, ok = VerifySignedCookie(signed+"bad", secret)
	if ok {
		t.Errorf("expected tampered cookie to fail")
	}
}

func TestRBACMiddleware(t *testing.T) {
	r := routing.NewRouter()

	adminOnly := RequireRole("admin")
	r.GET("/admin", func(ctx *routing.Context) (interface{}, error) {
		return "admin area", nil
	}, adminOnly)

	// User without admin role
	ctx1 := routing.NewContext("GET", "/admin")
	ctx1.Store["roles"] = []string{"user"}
	_, err := r.Dispatch(ctx1)
	if err == nil {
		t.Errorf("expected access denied for non-admin")
	}

	// User with admin role
	ctx2 := routing.NewContext("GET", "/admin")
	ctx2.Store["roles"] = []string{"admin"}
	res, err := r.Dispatch(ctx2)
	if err != nil || res != "admin area" {
		t.Errorf("expected success for admin: res=%v err=%v", res, err)
	}
}
