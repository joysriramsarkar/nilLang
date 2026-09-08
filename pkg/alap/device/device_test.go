package device

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/joysriramsarkar/nilLang/pkg/alap/data"
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

func TestBilingualBengaliReceiptFormatting(t *testing.T) {
	profile := ReceiptProfile{
		Width:         Width58mm,
		Locale:        "bn-BD",
		DuplicateCopy: true,
	}

	payload := ReceiptPayload{
		StoreName:  "লাখন ভান্ডার",
		InvoiceNo:  "INV-2026-99",
		GrandTotal: "৳1,500.00",
	}

	receipt := FormatWithProfile(profile, payload)
	if !strings.Contains(receipt, "লাখন ভান্ডার") {
		t.Fatal("expected receipt to contain Bengali store name")
	}
	if !strings.Contains(receipt, "৳১,৫০০.০০") {
		t.Fatalf("expected Bengali numerals '৳১,৫০০.০০' in receipt, got:\n%s", receipt)
	}
	if !strings.Contains(receipt, "গ্রাহক কপি") {
		t.Fatal("expected Bengali customer copy footer")
	}
}

func TestDurablePrintQueueCrashRecovery(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "print_durable_test.db")
	pool, err := data.OpenSQLite(dbPath)
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	defer pool.Close()

	runner := data.NewRealMigrationRunner(pool)
	data.RegisterPOSMigrations(runner)
	if err := runner.Up(); err != nil {
		t.Fatalf("migrations up: %v", err)
	}

	mockTransport := NewMockTransport()
	dpq := NewDurablePrintQueue(pool, mockTransport, 16)
	defer dpq.Stop()

	payload := ReceiptPayload{
		StoreName:  "Durable Test Store",
		InvoiceNo:  "INV-DUR-01",
		GrandTotal: "৳2,500.00",
	}

	formatter := NewESCPOSFormatter(Width58mm)
	raw := formatter.BuildESCPOSBytes(payload)

	job := &PrintJob{
		ID:       "pj-crash-01",
		Payload:  payload,
		RawBytes: raw,
	}

	if err := dpq.Enqueue(job); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	// Wait for worker completion
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if job.GetStatus() == JobDone {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if job.GetStatus() != JobDone {
		t.Fatalf("expected job status DONE, got %s", job.GetStatus())
	}

	// Verify DB record status is COMPLETED
	var status string
	_ = pool.QueryRow("SELECT status FROM job_records WHERE id = 'pj-crash-01'").Scan(&status)
	if status != "COMPLETED" {
		t.Fatalf("expected DB status COMPLETED, got %s", status)
	}

	// Now simulate application crash with an interrupted job in DB
	dpq.Stop()

	nowStr := time.Now().UTC().Format(time.RFC3339)
	payloadJSON := `{"StoreName":"Durable Test Store","InvoiceNo":"INV-CRASH-RECOVERED","GrandTotal":"৳9,000.00"}`
	_, err = pool.Exec(
		`INSERT INTO job_records (id, job_type, status, payload, attempt, max_attempts, created_at)
		 VALUES ('pj-interrupted-99', 'PRINT_RECEIPT', 'PENDING', ?, 0, 3, ?)`,
		payloadJSON, nowStr,
	)
	if err != nil {
		t.Fatalf("insert crash job: %v", err)
	}

	// Start a brand new DurablePrintQueue (simulating application restart)
	mockTransport2 := NewMockTransport()
	dpq2 := NewDurablePrintQueue(pool, mockTransport2, 16)
	defer dpq2.Stop()

	// Wait for recovery worker to process the interrupted job
	deadline2 := time.Now().Add(2 * time.Second)
	var recStatus string
	for time.Now().Before(deadline2) {
		_ = pool.QueryRow("SELECT status FROM job_records WHERE id = 'pj-interrupted-99'").Scan(&recStatus)
		if recStatus == "COMPLETED" {
			break
		}
		time.Sleep(30 * time.Millisecond)
	}

	if recStatus != "COMPLETED" {
		t.Fatalf("expected recovered job to transition to COMPLETED, got %s", recStatus)
	}

	if !strings.Contains(string(mockTransport2.Bytes()), "INV-CRASH-RECOVERED") {
		t.Fatalf("expected recovered print transport to receive INV-CRASH-RECOVERED receipt data")
	}
}
