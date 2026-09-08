package pos

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/joysriramsarkar/nilLang/pkg/alap/data"
)

// UserAccount represents a POS user or cashier
type UserAccount struct {
	ID             string     `json:"id"`
	OrgID          string     `json:"org_id"`
	Name           string     `json:"name"`
	Phone          string     `json:"phone"`
	Email          string     `json:"email,omitempty"`
	Role           string     `json:"role"` // RoleCashier, RoleManager, RoleAdmin
	PINHash        string     `json:"pin_hash"`
	Salt           string     `json:"salt"`
	FailedAttempts int        `json:"failed_attempts"`
	LockedUntil    *time.Time `json:"locked_until,omitempty"`
	Active         bool       `json:"active"`
}

// Session represents an authenticated cashier or manager device session
type Session struct {
	Token      string    `json:"token"`
	UserID     string    `json:"user_id"`
	UserName   string    `json:"user_name"`
	Role       string    `json:"role"`
	RegisterID string    `json:"register_id"`
	CreatedAt  time.Time `json:"created_at"`
	ExpiresAt  time.Time `json:"expires_at"`
}

// IsExpired checks whether the session has lapsed
func (s *Session) IsExpired() bool {
	return time.Now().After(s.ExpiresAt)
}

// AuthService manages secure PIN hashing, cashier login, lockout policy, and sessions
type AuthService struct {
	mu           sync.RWMutex
	users        map[string]*UserAccount // keyed by ID and Phone
	sessions     map[string]*Session     // keyed by Token
	db           *data.RealDBPool
	audit        *AuditTrail
	SessionTTL   time.Duration
	LockDuration time.Duration
	MaxAttempts  int
}

// NewAuthService constructs a production AuthService
func NewAuthService() *AuthService {
	return &AuthService{
		users:        make(map[string]*UserAccount),
		sessions:     make(map[string]*Session),
		SessionTTL:   8 * time.Hour,
		LockDuration: 15 * time.Minute,
		MaxAttempts:  5,
	}
}

// SetDB configures the real database pool
func (as *AuthService) SetDB(db *data.RealDBPool) {
	as.mu.Lock()
	defer as.mu.Unlock()
	as.db = db
}

// SetAudit configures audit trail logging
func (as *AuthService) SetAudit(audit *AuditTrail) {
	as.mu.Lock()
	defer as.mu.Unlock()
	as.audit = audit
}

// HashPIN produces a salted SHA-256 digest of a cashier PIN
func HashPIN(pin, salt string) string {
	h := sha256.New()
	h.Write([]byte(salt))
	h.Write([]byte(":"))
	h.Write([]byte(pin))
	return hex.EncodeToString(h.Sum(nil))
}

func generateRandomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// RegisterUser registers a user with a secure PIN
func (as *AuthService) RegisterUser(u *UserAccount, rawPIN string) error {
	as.mu.Lock()
	defer as.mu.Unlock()

	if u.ID == "" {
		return fmt.Errorf("user ID cannot be empty")
	}
	if len(rawPIN) < 4 {
		return fmt.Errorf("PIN must be at least 4 digits")
	}

	u.Salt = generateRandomHex(16)
	u.PINHash = HashPIN(rawPIN, u.Salt)
	u.Active = true

	as.users[u.ID] = u
	if u.Phone != "" {
		as.users[u.Phone] = u
	}

	if as.db != nil {
		nowStr := time.Now().UTC().Format(time.RFC3339)
		combinedHash := fmt.Sprintf("%s:%s", u.Salt, u.PINHash)
		_, err := as.db.Exec(
			`INSERT INTO users (id, org_id, name, phone, email, pin_hash, role_id, active, created_at, updated_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, 1, ?, ?)`,
			u.ID, u.OrgID, u.Name, u.Phone, u.Email, combinedHash, u.Role, nowStr, nowStr,
		)
		if err != nil {
			return fmt.Errorf("persist user: %w", err)
		}
	}

	return nil
}

// LoginPIN authenticates a cashier/manager via PIN, enforcing lockout on repeated failures
func (as *AuthService) LoginPIN(userIDOrPhone, pin, registerID string) (*Session, error) {
	as.mu.Lock()
	defer as.mu.Unlock()

	user, ok := as.users[userIDOrPhone]
	if !ok {
		return nil, fmt.Errorf("invalid user credentials")
	}

	now := time.Now()

	// 1. Lockout check
	if user.LockedUntil != nil && now.Before(*user.LockedUntil) {
		remaining := time.Until(*user.LockedUntil).Round(time.Second)
		return nil, fmt.Errorf("account locked due to multiple failed attempts. Try again in %v", remaining)
	}

	// 2. PIN verification
	expectedHash := HashPIN(pin, user.Salt)
	if expectedHash != user.PINHash {
		user.FailedAttempts++
		remaining := as.MaxAttempts - user.FailedAttempts
		if user.FailedAttempts >= as.MaxAttempts {
			lockUntil := now.Add(as.LockDuration)
			user.LockedUntil = &lockUntil
			if as.audit != nil {
				as.audit.Record(ActionCustomerDuePaid, user.ID, "security", nil, nil,
					fmt.Sprintf("Account locked for %s after %d failed PIN attempts", user.Name, user.FailedAttempts))
			}
			return nil, fmt.Errorf("maximum attempts reached. Account locked for %v", as.LockDuration)
		}
		return nil, fmt.Errorf("incorrect PIN. %d attempts remaining before account lockout", remaining)
	}

	// 3. Success: reset counters
	user.FailedAttempts = 0
	user.LockedUntil = nil

	// 4. Issue session
	token := generateRandomHex(32)
	session := &Session{
		Token:      token,
		UserID:     user.ID,
		UserName:   user.Name,
		Role:       user.Role,
		RegisterID: registerID,
		CreatedAt:  now,
		ExpiresAt:  now.Add(as.SessionTTL),
	}
	as.sessions[token] = session

	return session, nil
}

// ValidateSession verifies if a session token is valid and active
func (as *AuthService) ValidateSession(token string) (*Session, error) {
	as.mu.RLock()
	defer as.mu.RUnlock()

	session, ok := as.sessions[token]
	if !ok {
		return nil, fmt.Errorf("unauthorized: invalid session token")
	}
	if session.IsExpired() {
		return nil, fmt.Errorf("unauthorized: session expired")
	}
	return session, nil
}

// Logout terminates an active session
func (as *AuthService) Logout(token string) {
	as.mu.Lock()
	defer as.mu.Unlock()
	delete(as.sessions, token)
}

// AuthorizeSession verifies the session and checks if the authenticated role holds the required permission
func (as *AuthService) AuthorizeSession(token string, permission string) (*Session, error) {
	session, err := as.ValidateSession(token)
	if err != nil {
		return nil, err
	}
	if err := Authorize(session.Role, permission); err != nil {
		return nil, err
	}
	return session, nil
}
