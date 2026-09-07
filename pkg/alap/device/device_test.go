package device

import (
	"strings"
	"testing"
)

func TestBarcodeScannerBuffer(t *testing.T) {
	scannedCode := ""
	scanner := NewBarcodeScanner(func(barcode string) {
		scannedCode = barcode
	})

	input := "8901030012345\n"
	for _, r := range input {
		scanner.ProcessKey(r)
	}

	if scannedCode != "8901030012345" {
		t.Fatalf("Expected scanned code '8901030012345', got '%s'", scannedCode)
	}
}

func TestESCPOSReceiptFormatting(t *testing.T) {
	formatter := NewESCPOSFormatter(Width58mm)
	payload := ReceiptPayload{
		StoreName:     "LAKHAN BHANDAR",
		StoreSubtitle: "Wholesale & Retail",
		InvoiceNo:     "INV-2026-001",
		DateStr:       "07/09/2026",
		Cashier:       "Joy Sarkar",
		Items: []ReceiptLineItem{
			{Name: "Miniket Rice (50kg)", Quantity: "1", Price: "৳3,400.00", Total: "৳3,400.00"},
			{Name: "Teer Soybean Oil (5L)", Quantity: "2", Price: "৳890.00", Total: "৳1,780.00"},
		},
		Subtotal:      "৳5,180.00",
		Discount:      "৳180.00",
		Tax:           "৳0.00",
		GrandTotal:    "৳5,000.00",
		PaymentMethod: "Cash",
		PaidAmount:    "৳5,000.00",
		ChangeDue:     "৳0.00",
	}

	text := formatter.FormatPlainText(payload)
	if !strings.Contains(text, "LAKHAN BHANDAR") {
		t.Fatalf("Expected receipt to contain store name")
	}
	if !strings.Contains(text, "INV-2026-001") {
		t.Fatalf("Expected receipt to contain invoice number")
	}
	if !strings.Contains(text, "GRAND TOTAL") {
		t.Fatalf("Expected receipt to contain grand total")
	}

	bytes := formatter.BuildESCPOSBytes(payload)
	if len(bytes) == 0 {
		t.Fatalf("Expected non-empty ESC/POS byte sequence")
	}
}

func TestCashDrawer(t *testing.T) {
	cd := NewCashDrawer()
	pulse := cd.Open()
	if !cd.IsOpen() {
		t.Fatalf("Expected drawer to be marked open")
	}
	if len(pulse) != 5 || pulse[0] != 0x1b || pulse[1] != 0x70 {
		t.Fatalf("Invalid cash drawer pulse: %v", pulse)
	}
	cd.Close()
	if cd.IsOpen() {
		t.Fatalf("Expected drawer to be marked closed")
	}
}
