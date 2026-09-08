package pos_test

import (
	"strings"
	"testing"
	"time"

	"github.com/joysriramsarkar/nilLang/pkg/alap/pos"
)

func TestAuthPINLifecycle(t *testing.T) {
	auth := pos.NewAuthService()

	cashier := &pos.UserAccount{
		ID:    "u-cashier-01",
		OrgID: "org-01",
		Name:  "Robiul Alam",
		Phone: "01755123456",
		Role:  pos.RoleCashier,
	}

	if err := auth.RegisterUser(cashier, "1234"); err != nil {
		t.Fatalf("RegisterUser: %v", err)
	}

	// 1. Wrong PIN attempt
	_, err := auth.LoginPIN(cashier.Phone, "9999", "reg-01")
	if err == nil {
		t.Fatalf("expected error for wrong PIN, got nil")
	}

	// 2. Correct PIN login
	session, err := auth.LoginPIN(cashier.Phone, "1234", "reg-01")
	if err != nil {
		t.Fatalf("LoginPIN: %v", err)
	}
	if session.Token == "" {
		t.Fatalf("expected session token, got empty")
	}
	if session.Role != pos.RoleCashier {
		t.Fatalf("expected role cashier, got %s", session.Role)
	}

	// 3. Validate session
	valSession, err := auth.ValidateSession(session.Token)
	if err != nil {
		t.Fatalf("ValidateSession: %v", err)
	}
	if valSession.UserID != cashier.ID {
		t.Fatalf("expected user ID %s, got %s", cashier.ID, valSession.UserID)
	}

	// 4. Authorize permitted action
	_, err = auth.AuthorizeSession(session.Token, pos.PermSaleCreate)
	if err != nil {
		t.Fatalf("expected PermSaleCreate to be authorized: %v", err)
	}

	// 5. Authorize sensitive action (Forbidden for cashier)
	_, err = auth.AuthorizeSession(session.Token, pos.PermPriceOverride)
	if err == nil {
		t.Fatalf("expected PermPriceOverride to be forbidden for cashier, got nil")
	}
	if !strings.Contains(err.Error(), "forbidden") {
		t.Fatalf("expected forbidden error, got %v", err)
	}

	// 6. Logout
	auth.Logout(session.Token)
	_, err = auth.ValidateSession(session.Token)
	if err == nil {
		t.Fatalf("expected error validating logged-out session, got nil")
	}
}

func TestAuthAccountLockout(t *testing.T) {
	auth := pos.NewAuthService()
	auth.MaxAttempts = 5
	auth.LockDuration = 10 * time.Minute

	user := &pos.UserAccount{
		ID:    "u-lock-01",
		OrgID: "org-01",
		Name:  "Test User",
		Phone: "01799887766",
		Role:  pos.RoleCashier,
	}
	if err := auth.RegisterUser(user, "4321"); err != nil {
		t.Fatalf("RegisterUser: %v", err)
	}

	// Fail 4 times
	for i := 1; i <= 4; i++ {
		_, err := auth.LoginPIN(user.Phone, "0000", "reg-01")
		if err == nil {
			t.Fatalf("expected failure on attempt %d", i)
		}
		expectedRemaining := 5 - i
		if !strings.Contains(err.Error(), "remaining") {
			t.Fatalf("expected 'remaining' warning on attempt %d, got %v", i, err)
		}
		_ = expectedRemaining
	}

	// 5th failure: triggers lockout
	_, err := auth.LoginPIN(user.Phone, "0000", "reg-01")
	if err == nil {
		t.Fatalf("expected lockout error on 5th attempt")
	}
	if !strings.Contains(err.Error(), "locked") {
		t.Fatalf("expected 'locked' error, got %v", err)
	}

	// 6th attempt with CORRECT PIN: Must STILL be rejected because account is locked!
	_, err = auth.LoginPIN(user.Phone, "4321", "reg-01")
	if err == nil {
		t.Fatalf("expected locked account to reject even correct PIN")
	}
	if !strings.Contains(err.Error(), "locked") {
		t.Fatalf("expected locked account message, got %v", err)
	}
}

func TestAuthManagerRolePermissions(t *testing.T) {
	auth := pos.NewAuthService()

	manager := &pos.UserAccount{
		ID:    "u-mgr-01",
		OrgID: "org-01",
		Name:  "Manager Farhan",
		Phone: "01822334455",
		Role:  pos.RoleManager,
	}
	_ = auth.RegisterUser(manager, "8888")

	session, err := auth.LoginPIN(manager.Phone, "8888", "reg-01")
	if err != nil {
		t.Fatalf("login manager: %v", err)
	}

	// Verify all sensitive operations permitted for manager
	sensitiveOps := []string{
		pos.PermSaleVoid,
		pos.PermPriceOverride,
		pos.PermDiscountOverride,
		pos.PermInventoryAdjust,
		pos.PermShiftClose,
		pos.PermReportView,
		pos.PermRefundManager,
	}

	for _, op := range sensitiveOps {
		_, err := auth.AuthorizeSession(session.Token, op)
		if err != nil {
			t.Errorf("expected manager to be authorized for %s, got error: %v", op, err)
		}
	}
}
