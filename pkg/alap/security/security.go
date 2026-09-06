package security

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/joysriramsarkar/nilLang/pkg/alap/routing"
)

// JWTHeader represents a JSON Web Token header
type JWTHeader struct {
	Alg string `json:"alg"`
	Typ string `json:"typ"`
}

// JWTClaims represents standard and custom JWT claims
type JWTClaims struct {
	Subject   string                 `json:"sub,omitempty"`
	Issuer    string                 `json:"iss,omitempty"`
	Audience  string                 `json:"aud,omitempty"`
	ExpiresAt int64                  `json:"exp"`
	IssuedAt  int64                  `json:"iat"`
	Roles     []string               `json:"roles,omitempty"`
	Custom    map[string]interface{} `json:"custom,omitempty"`
}

// GenerateJWT creates a signed HS256 JWT token
func GenerateJWT(claims JWTClaims, secret string) (string, error) {
	header := JWTHeader{Alg: "HS256", Typ: "JWT"}
	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", err
	}

	if claims.IssuedAt == 0 {
		claims.IssuedAt = time.Now().Unix()
	}

	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}

	h64 := base64.RawURLEncoding.EncodeToString(headerJSON)
	c64 := base64.RawURLEncoding.EncodeToString(claimsJSON)
	payload := h64 + "." + c64

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	sig64 := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	return payload + "." + sig64, nil
}

// VerifyJWT validates a signed JWT token and returns its claims
func VerifyJWT(token string, secret string) (*JWTClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, errors.New("invalid token format")
	}

	payload := parts[0] + "." + parts[1]
	providedSig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, errors.New("invalid signature encoding")
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	expectedSig := mac.Sum(nil)

	if !hmac.Equal(providedSig, expectedSig) {
		return nil, errors.New("invalid token signature")
	}

	claimsJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, errors.New("invalid claims encoding")
	}

	var claims JWTClaims
	if err := json.Unmarshal(claimsJSON, &claims); err != nil {
		return nil, err
	}

	if claims.ExpiresAt > 0 && time.Now().Unix() > claims.ExpiresAt {
		return nil, errors.New("token expired")
	}

	return &claims, nil
}

// JWTMiddleware validates Bearer token and stores claims in context
func JWTMiddleware(secret string) routing.Middleware {
	return func(next routing.HandlerFunc) routing.HandlerFunc {
		return func(ctx *routing.Context) (interface{}, error) {
			authHeader := ctx.Headers["Authorization"]
			if !strings.HasPrefix(authHeader, "Bearer ") {
				return nil, errors.New("401: missing or invalid authorization header")
			}

			token := strings.TrimPrefix(authHeader, "Bearer ")
			claims, err := VerifyJWT(token, secret)
			if err != nil {
				return nil, fmt.Errorf("401: %v", err)
			}

			ctx.UserID = claims.Subject
			ctx.Store["claims"] = claims
			ctx.Store["roles"] = claims.Roles

			return next(ctx)
		}
	}
}

// ─── PASSWORD HASHING ───────────────────────────────────────────────────────

// HashPassword hashes a password with a random 16-byte salt using HMAC-SHA256
func HashPassword(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}

	mac := hmac.New(sha256.New, salt)
	mac.Write([]byte(password))
	hash := mac.Sum(nil)

	return hex.EncodeToString(salt) + ":" + hex.EncodeToString(hash), nil
}

// VerifyPassword checks if a plain password matches the stored salt:hash
func VerifyPassword(password, stored string) bool {
	parts := strings.Split(stored, ":")
	if len(parts) != 2 {
		return false
	}

	salt, err := hex.DecodeString(parts[0])
	if err != nil {
		return false
	}

	expectedHash, err := hex.DecodeString(parts[1])
	if err != nil {
		return false
	}

	mac := hmac.New(sha256.New, salt)
	mac.Write([]byte(password))
	actualHash := mac.Sum(nil)

	return hmac.Equal(actualHash, expectedHash)
}

// ─── SECURE COOKIES ─────────────────────────────────────────────────────────

// SignCookie signs a cookie value with secret key
func SignCookie(val string, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(val))
	sig := hex.EncodeToString(mac.Sum(nil))
	return val + "." + sig
}

// VerifySignedCookie validates signed cookie value
func VerifySignedCookie(signedVal string, secret string) (string, bool) {
	idx := strings.LastIndex(signedVal, ".")
	if idx == -1 {
		return "", false
	}

	val := signedVal[:idx]
	expectedSig := signedVal[idx+1:]

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(val))
	actualSig := hex.EncodeToString(mac.Sum(nil))

	if hmac.Equal([]byte(actualSig), []byte(expectedSig)) {
		return val, true
	}
	return "", false
}

// ─── CSRF PROTECTION ────────────────────────────────────────────────────────

// GenerateCSRFToken creates a cryptographically secure token
func GenerateCSRFToken() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// CSRFMiddleware validates CSRF tokens for mutating HTTP requests
func CSRFMiddleware(cookieName string) routing.Middleware {
	return func(next routing.HandlerFunc) routing.HandlerFunc {
		return func(ctx *routing.Context) (interface{}, error) {
			// Safe methods bypass CSRF check
			if ctx.Method == "GET" || ctx.Method == "HEAD" || ctx.Method == "OPTIONS" {
				return next(ctx)
			}

			cookieToken := ctx.Cookies[cookieName]
			headerToken := ctx.Headers["X-CSRF-Token"]

			if cookieToken == "" || headerToken == "" || cookieToken != headerToken {
				return nil, errors.New("403: CSRF token mismatch or missing")
			}

			return next(ctx)
		}
	}
}

// ─── RBAC AUTHORIZATION ─────────────────────────────────────────────────────

// RequireRole ensures context has the required role
func RequireRole(requiredRole string) routing.Middleware {
	return func(next routing.HandlerFunc) routing.HandlerFunc {
		return func(ctx *routing.Context) (interface{}, error) {
			rolesRaw, ok := ctx.Store["roles"]
			if !ok {
				return nil, errors.New("403: access denied, no roles assigned")
			}

			roles, ok := rolesRaw.([]string)
			if !ok {
				return nil, errors.New("403: invalid roles data")
			}

			for _, r := range roles {
				if r == requiredRole || r == "admin" {
					return next(ctx)
				}
			}

			return nil, fmt.Errorf("403: access denied, role %q required", requiredRole)
		}
	}
}
