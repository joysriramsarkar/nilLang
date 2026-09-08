package pos_test

import (
	"strings"
	"testing"

	"github.com/joysriramsarkar/nilLang/pkg/alap/pos"
)

func TestPaymentTerminalHappyPath(t *testing.T) {
	gw := pos.NewMockTerminalGateway()
	service := pos.NewTerminalService(gw)

	// 1. Create intent with idempotency key
	const idempKey = "idemp-sale-001"
	intent1, err := service.CreateIntent("sale-101", pos.MethodCard, 250000, "term-pos-01", idempKey)
	if err != nil {
		t.Fatalf("CreateIntent: %v", err)
	}
	if intent1.Status != pos.IntentCreated {
		t.Fatalf("expected CREATED status, got %s", intent1.Status)
	}

	// 2. Idempotency test: second call with same key returns same intent
	intent2, err := service.CreateIntent("sale-101", pos.MethodCard, 250000, "term-pos-01", idempKey)
	if err != nil {
		t.Fatalf("second CreateIntent failed: %v", err)
	}
	if intent1.ID != intent2.ID {
		t.Fatalf("idempotency violation: expected same intent ID %s, got %s", intent1.ID, intent2.ID)
	}

	// 3. Execute intent
	executed, err := service.ExecuteIntent(intent1.ID)
	if err != nil {
		t.Fatalf("ExecuteIntent: %v", err)
	}
	if executed.Status != pos.IntentCaptured {
		t.Fatalf("expected CAPTURED status, got %s", executed.Status)
	}
	if executed.ProviderRef == "" {
		t.Fatalf("expected non-empty provider ref")
	}
	if executed.AuthCode == "" {
		t.Fatalf("expected non-empty auth code")
	}
}

func TestPaymentTerminalTimeoutReconciliation(t *testing.T) {
	gw := pos.NewMockTerminalGateway()
	gw.ShouldTimeOut = true // Hardware communication times out

	service := pos.NewTerminalService(gw)
	intent, err := service.CreateIntent("sale-102", pos.MethodCard, 100000, "term-02", "idemp-timeout-1")
	if err != nil {
		t.Fatalf("CreateIntent: %v", err)
	}

	// Execute intent: Gateway times out on send, but ReconcileTimeout succeeds and resolves to CAPTURED
	res, err := service.ExecuteIntent(intent.ID)
	if err != nil {
		t.Fatalf("expected reconciliation to recover transaction, got err: %v", err)
	}
	if res.Status != pos.IntentCaptured {
		t.Fatalf("expected reconciled status CAPTURED, got %s", res.Status)
	}
	if !strings.HasPrefix(res.ProviderRef, "REC-") {
		t.Fatalf("expected reconciled provider ref starting with REC-, got %s", res.ProviderRef)
	}
}

func TestPaymentTerminalDeclined(t *testing.T) {
	gw := pos.NewMockTerminalGateway()
	gw.FailOnCapture = true // Card declined by issuing bank

	service := pos.NewTerminalService(gw)
	intent, _ := service.CreateIntent("sale-103", pos.MethodCard, 500000, "term-03", "idemp-declined-1")

	_, err := service.ExecuteIntent(intent.ID)
	if err == nil {
		t.Fatalf("expected capture error for declined card")
	}
	saved, _ := service.GetIntent(intent.ID)
	if saved.Status != pos.IntentFailed {
		t.Fatalf("expected FAILED status, got %s", saved.Status)
	}
	if !strings.Contains(saved.LastError, "declined") {
		t.Fatalf("expected declined in last error, got %s", saved.LastError)
	}
}
