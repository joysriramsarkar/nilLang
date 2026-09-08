package device

import (
	"strings"
	"testing"
	"time"
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

func TestBarcodeScannerModes(t *testing.T) {
	scanner := NewBarcodeScanner(nil)

	// Test Camera frame extraction
	frameData := []byte("HEADER...BARCODE:8901030012345\r\nFOOTER")
	code, err := scanner.ProcessFrame(frameData)
	if err != nil || code != "8901030012345" {
		t.Fatalf("Camera frame scan failed: %v, code=%s", err, code)
	}

	// Test Bluetooth raw packet
	btData := []byte("  8901030077778\r\n")
	code2, err := scanner.ProcessBluetooth(btData)
	if err != nil || code2 != "8901030077778" {
		t.Fatalf("Bluetooth scan failed: %v, code=%s", err, code2)
	}

	// Test Async Event Channel receives both
	ev1 := <-scanner.Events()
	if ev1.Code != "8901030012345" || ev1.Mode != ScanModeCamera {
		t.Fatalf("Unexpected event 1: %+v", ev1)
	}

	ev2 := <-scanner.Events()
	if ev2.Code != "8901030077778" || ev2.Mode != ScanModeBluetooth {
		t.Fatalf("Unexpected event 2: %+v", ev2)
	}
}

func TestPrintQueueAndMockTransport(t *testing.T) {
	transport := NewMockTransport()
	queue := NewPrintQueue(transport, 10)
	defer queue.Stop()

	job := &PrintJob{
		ID:       "job-01",
		SaleID:   "sale-100",
		RawBytes: []byte("TEST ESC/POS RECEIPT DATA\n"),
	}

	queue.Enqueue(job)

	// Wait for background worker
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if job.GetStatus() == JobDone {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	if job.GetStatus() != JobDone {
		t.Fatalf("Expected job status DONE, got %s (err: %s)", job.GetStatus(), job.GetLastError())
	}

	if string(transport.Bytes()) != "TEST ESC/POS RECEIPT DATA\n" {
		t.Fatalf("Transport buffer mismatch: got %q", string(transport.Bytes()))
	}
}

func TestReceiptProfileFormatting(t *testing.T) {
	profile := ReceiptProfile{
		Width:         Width58mm,
		FooterText:    "Visit us online at example.com",
		DuplicateCopy: true,
	}

	payload := ReceiptPayload{
		StoreName:  "Test Shop",
		InvoiceNo:  "INV-001",
		DateStr:    "08/09/2026",
		GrandTotal: "৳1,500.00",
	}

	receipt := FormatWithProfile(profile, payload)
	if !strings.Contains(receipt, "Test Shop") {
		t.Fatal("Expected receipt to contain store name")
	}
	if !strings.Contains(receipt, "Visit us online at example.com") {
		t.Fatal("Expected custom footer note from profile")
	}
	if !strings.Contains(receipt, "*** CUSTOMER COPY ***") {
		t.Fatal("Expected duplicate customer copy block")
	}
}
