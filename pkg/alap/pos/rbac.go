package pos

import (
	"fmt"
	"strings"
)

// Standard POS Roles
const (
	RoleCashier = "cashier"
	RoleManager = "manager"
	RoleAdmin   = "admin"
)

// Standard Domain Permissions (web-implications.md & Roadmap Item 18)
const (
	PermSaleCreate       = "sale.create"
	PermSaleVoid         = "sale.void"
	PermSaleRefund       = "sale.refund"
	PermDiscountOverride = "discount.override"
	PermPriceOverride    = "price.override"
	PermInventoryAdjust  = "inventory.adjust"
	PermCashOpen         = "cash.open"
	PermShiftClose       = "shift.close"
	PermCreditSale       = "credit.sale"
	PermReportView       = "report.view"
	PermUserManage       = "user.manage"
	PermRefundStandard   = "refund.standard"
	PermRefundManager    = "refund.manager" // Required for refunds > ৳5,000
	PermShiftOpen        = "register.open"
	PermDrawerOpen       = "register.drawer"
	PermAuditView        = "audit.view"
)

var rolePermissions = map[string][]string{
	RoleCashier: {
		PermSaleCreate,
		PermSaleRefund,
		PermRefundStandard,
		PermShiftOpen,
		PermCashOpen,
		PermDrawerOpen,
	},
	RoleManager: {
		PermSaleCreate,
		PermSaleVoid,
		PermSaleRefund,
		PermDiscountOverride,
		PermPriceOverride,
		PermInventoryAdjust,
		PermCashOpen,
		PermShiftOpen,
		PermShiftClose,
		PermCreditSale,
		PermReportView,
		PermRefundStandard,
		PermRefundManager,
		PermDrawerOpen,
		PermAuditView,
	},
	RoleAdmin: {
		PermSaleCreate,
		PermSaleVoid,
		PermSaleRefund,
		PermDiscountOverride,
		PermPriceOverride,
		PermInventoryAdjust,
		PermCashOpen,
		PermShiftOpen,
		PermShiftClose,
		PermCreditSale,
		PermReportView,
		PermRefundStandard,
		PermRefundManager,
		PermDrawerOpen,
		PermAuditView,
		PermUserManage,
	},
}

// Authorize verifies that a role holds the required permission, returning an error if denied.
func Authorize(role, permission string) error {
	if !HasPermission(role, permission) {
		return fmt.Errorf("forbidden: role %q lacks required permission %q", role, permission)
	}
	return nil
}

// HasPermission checks if a given role possesses a requested permission.
func HasPermission(role, permission string) bool {
	perms, ok := rolePermissions[strings.ToLower(strings.TrimSpace(role))]
	if !ok {
		return false
	}
	for _, p := range perms {
		if p == permission {
			return true
		}
	}
	return false
}

// ManagerRefundThresholdMinor is ৳5,000.00 (500,000 minor)
const ManagerRefundThresholdMinor int64 = 500000

// ErrManagerApprovalRequired is returned when an operation exceeds threshold without manager role.
var ErrManagerApprovalRequired = fmt.Errorf("manager authorization required for refunds exceeding ৳5,000.00")

// CheckRefundAuthorization verifies if the actor role is authorized for the given refund sum.
func CheckRefundAuthorization(role string, refundMinor int64) error {
	if refundMinor > ManagerRefundThresholdMinor {
		if !HasPermission(role, PermRefundManager) {
			return ErrManagerApprovalRequired
		}
	}
	if !HasPermission(role, PermRefundStandard) {
		return fmt.Errorf("role %q lacks refund permission", role)
	}
	return nil
}
