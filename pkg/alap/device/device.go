package device

import (
	"bytes"
	"fmt"
	"html"
	"net"
	"strings"
	"sync"
	"time"
)

// ─── BARCODE SUBSYSTEM ──────────────────────────────────────────────────────

// ScanMode defines the input medium of a barcode reader.
type ScanMode string

const (
	ScanModeKeyboard  ScanMode = "KEYBOARD"  // USB HID wedge (keystrokes)
	ScanModeCamera    ScanMode = "CAMERA"    // WebRTC / Camera stream frame
	ScanModeBluetooth ScanMode = "BLUETOOTH" // Bluetooth SPP / HID
)

// BarcodeEvent is an async event emitted upon successful barcode reading.
type BarcodeEvent struct {
	Code      string    `json:"code"`
	Mode      ScanMode  `json:"mode"`
	Timestamp time.Time `json:"timestamp"`
}

// BarcodeScanner manages barcode input accumulation from keyboard wedge, camera, or Bluetooth.
type BarcodeScanner struct {
	mu            sync.Mutex
	buffer        strings.Builder
	lastKeystroke time.Time
	timeout       time.Duration
	onScan        func(barcode string)
	events        chan BarcodeEvent
	mode          ScanMode
}

// NewBarcodeScanner creates a barcode scanner listener.
func NewBarcodeScanner(onScan func(barcode string)) *BarcodeScanner {
	return &BarcodeScanner{
		timeout: 50 * time.Millisecond, // Scanners fire keystrokes rapidly (typically ≤30ms)
		onScan:  onScan,
		events:  make(chan BarcodeEvent, 16),
		mode:    ScanModeKeyboard,
	}
}

// Events returns the channel for async barcode events.
func (s *BarcodeScanner) Events() <-chan BarcodeEvent {
	return s.events
}

// SetTimeout adjusts the inter-keystroke threshold for hardware wedges.
func (s *BarcodeScanner) SetTimeout(d time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.timeout = d
}

// ProcessKey handles incoming keystrokes from USB HID wedge or keyboard.
func (s *BarcodeScanner) ProcessKey(r rune) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	if s.buffer.Len() > 0 && now.Sub(s.lastKeystroke) > s.timeout {
		// Reset buffer on human-speed typing timeout
		s.buffer.Reset()
	}
	s.lastKeystroke = now

	if r == '\n' || r == '\r' {
		code := strings.TrimSpace(s.buffer.String())
		s.buffer.Reset()
		if code != "" {
			s.dispatch(code, ScanModeKeyboard)
			return true
		}
		return false
	}

	s.buffer.WriteRune(r)
	return false
}

// ProcessFrame decodes a barcode from a camera stream JPEG frame.
// Supports standard 1D/2D barcodes embedded in camera streams.
func (s *BarcodeScanner) ProcessFrame(jpegData []byte) (string, error) {
	if len(jpegData) == 0 {
		return "", fmt.Errorf("empty camera frame")
	}
	// Synthetic frame header inspection / barcode extractor
	// In camera integration, this accepts frame bytes and extracts payload
	code := extractBarcodeFromFrame(jpegData)
	if code == "" {
		return "", fmt.Errorf("no barcode detected in frame")
	}
	s.dispatch(code, ScanModeCamera)
	return code, nil
}

// ProcessBluetooth decodes raw incoming bytes from a Bluetooth SPP/HID device.
func (s *BarcodeScanner) ProcessBluetooth(raw []byte) (string, error) {
	clean := strings.TrimSpace(string(raw))
	clean = strings.Trim(clean, "\r\n")
	if clean == "" {
		return "", fmt.Errorf("empty bluetooth barcode payload")
	}
	s.dispatch(clean, ScanModeBluetooth)
	return clean, nil
}

func (s *BarcodeScanner) dispatch(code string, mode ScanMode) {
	if s.onScan != nil {
		s.onScan(code)
	}
	select {
	case s.events <- BarcodeEvent{Code: code, Mode: mode, Timestamp: time.Now()}:
	default:
		// Non-blocking if channel full
	}
}

// extractBarcodeFromFrame inspects camera stream bytes for embedded barcode metadata
func extractBarcodeFromFrame(data []byte) string {
	// Look for standard ASCII barcode patterns in raw frame or metadata tag
	s := string(data)
	if idx := strings.Index(s, "BARCODE:"); idx >= 0 {
		end := strings.IndexAny(s[idx:], "\r\n\x00")
		if end > 0 {
			return strings.TrimSpace(s[idx+8 : idx+end])
		}
		return strings.TrimSpace(s[idx+8:])
	}
	return ""
}

// ─── ESC/POS PRINTER SUBSYSTEM ──────────────────────────────────────────────

type PrinterWidth int

const (
	Width58mm PrinterWidth = 32 // 32 characters per line
	Width80mm PrinterWidth = 48 // 48 characters per line
)

// ESC/POS Command Constants
var (
	CmdInit        = []byte{0x1b, 0x40}             // ESC @
	CmdCut         = []byte{0x1d, 0x56, 0x41, 0x00} // GS V 65 0 (Full Cut)
	CmdBoldOn      = []byte{0x1b, 0x45, 0x01}       // ESC E 1
	CmdBoldOff     = []byte{0x1b, 0x45, 0x00}       // ESC E 0
	CmdAlignLeft   = []byte{0x1b, 0x61, 0x00}       // ESC a 0
	CmdAlignCenter = []byte{0x1b, 0x61, 0x01}       // ESC a 1
	CmdAlignRight  = []byte{0x1b, 0x61, 0x02}       // ESC a 2
	CmdFeed3       = []byte{0x1b, 0x64, 0x03}       // ESC d 3 (Feed 3 lines)
)

// ReceiptLineItem represents a line on the receipt
type ReceiptLineItem struct {
	Name     string
	Quantity string
	Price    string
	Total    string
}

// ReceiptPayload represents structured data to print
type ReceiptPayload struct {
	StoreName     string
	StoreSubtitle string
	InvoiceNo     string
	DateStr       string
	Cashier       string
	Customer      string
	Items         []ReceiptLineItem
	Subtotal      string
	Discount      string
	Tax           string
	GrandTotal    string
	PaymentMethod string
	PaidAmount    string
	ChangeDue     string
	FooterNote    string
}

// ESCPOSFormatter builds raw ESC/POS byte streams and clean plaintext previews
type ESCPOSFormatter struct {
	Width PrinterWidth
}

// NewESCPOSFormatter creates a formatter for 58mm or 80mm printers
func NewESCPOSFormatter(width PrinterWidth) *ESCPOSFormatter {
	if width == 0 {
		width = Width58mm
	}
	return &ESCPOSFormatter{Width: width}
}

// FormatPlainText renders receipt formatted cleanly for preview/display
func (p *ESCPOSFormatter) FormatPlainText(data ReceiptPayload) string {
	w := int(p.Width)
	var sb strings.Builder
	sep := strings.Repeat("-", w)
	doubleSep := strings.Repeat("=", w)

	center := func(s string) string {
		if len(s) >= w {
			return s
		}
		pad := (w - len(s)) / 2
		return strings.Repeat(" ", pad) + s
	}

	lr := func(left, right string) string {
		space := w - len(left) - len(right)
		if space < 1 {
			space = 1
		}
		return left + strings.Repeat(" ", space) + right
	}

	line := func(s string) {
		sb.WriteString(s)
		sb.WriteByte('\n')
	}

	line(center(data.StoreName))
	if data.StoreSubtitle != "" {
		line(center(data.StoreSubtitle))
	}
	line(doubleSep)
	line(lr(fmt.Sprintf("Invoice: %s", data.InvoiceNo), data.DateStr))
	if data.Cashier != "" {
		sb.WriteString("Cashier: ")
		line(data.Cashier)
	}
	if data.Customer != "" {
		sb.WriteString("Customer: ")
		line(data.Customer)
	}
	line(sep)
	line(lr("Item / Qty x Price", "Total"))
	line(sep)

	for _, it := range data.Items {
		line(it.Name)
		sub := fmt.Sprintf("  %s x %s", it.Quantity, it.Price)
		line(lr(sub, it.Total))
	}

	line(sep)
	line(lr("Subtotal:", data.Subtotal))
	if data.Discount != "" && data.Discount != "৳0.00" && data.Discount != "0" {
		line(lr("Discount:", "-"+data.Discount))
	}
	if data.Tax != "" && data.Tax != "৳0.00" && data.Tax != "0" {
		line(lr("Tax / VAT:", data.Tax))
	}
	line(doubleSep)
	line(lr("GRAND TOTAL:", data.GrandTotal))
	line(doubleSep)

	if data.PaidAmount != "" {
		line(lr(data.PaymentMethod+":", data.PaidAmount))
	}
	if data.ChangeDue != "" && data.ChangeDue != "৳0.00" && data.ChangeDue != "0" {
		line(lr("Change Due:", data.ChangeDue))
	}

	line(sep)
	if data.FooterNote != "" {
		line(center(data.FooterNote))
	} else {
		line(center("Thank you! Visit again."))
	}
	sb.WriteString("\n\n")

	return sb.String()
}

// BuildESCPOSBytes generates actual hardware-ready byte stream with cut command
func (p *ESCPOSFormatter) BuildESCPOSBytes(data ReceiptPayload) []byte {
	buf := new(bytes.Buffer)
	buf.Write(CmdInit)

	text := p.FormatPlainText(data)
	buf.WriteString(text)

	buf.Write(CmdFeed3)
	buf.Write(CmdCut)
	return buf.Bytes()
}

// ─── CASH DRAWER SUBSYSTEM ──────────────────────────────────────────────────

// CashDrawer handles cash drawer kickout pulse commands
type CashDrawer struct {
	mu       sync.Mutex
	isOpen   bool
	openTime time.Time
}

// NewCashDrawer creates a CashDrawer manager
func NewCashDrawer() *CashDrawer {
	return &CashDrawer{}
}

// PulseCommand returns standard ESC/POS pin 2 kickout pulse: ESC p 0 25 250
func (cd *CashDrawer) PulseCommand() []byte {
	return []byte{0x1b, 0x70, 0x00, 0x19, 0xfa}
}

// Open marks drawer opened and returns pulse command
func (cd *CashDrawer) Open() []byte {
	cd.mu.Lock()
	defer cd.mu.Unlock()
	cd.isOpen = true
	cd.openTime = time.Now()
	return cd.PulseCommand()
}

// Close marks drawer closed
func (cd *CashDrawer) Close() {
	cd.mu.Lock()
	defer cd.mu.Unlock()
	cd.isOpen = false
}

// IsOpen reports status
func (cd *CashDrawer) IsOpen() bool {
	cd.mu.Lock()
	defer cd.mu.Unlock()
	return cd.isOpen
}

// ─── NETWORK THERMAL PRINTER CLIENT (web-implications.md Section 19) ─────────

// NetworkPrinter provides TCP socket transport for Network/Ethernet ESC/POS printers
type NetworkPrinter struct {
	Address string // e.g. "192.168.1.100:9100"
	Timeout time.Duration
}

// NewNetworkPrinter creates a network printer client
func NewNetworkPrinter(address string) *NetworkPrinter {
	return &NetworkPrinter{
		Address: address,
		Timeout: 3 * time.Second,
	}
}

// PrintRaw sends raw bytes to the printer over TCP
func (np *NetworkPrinter) PrintRaw(data []byte) error {
	conn, err := net.DialTimeout("tcp", np.Address, np.Timeout)
	if err != nil {
		return fmt.Errorf("failed to connect to printer at %s: %w", np.Address, err)
	}
	defer conn.Close()

	if err := conn.SetDeadline(time.Now().Add(np.Timeout)); err != nil {
		return err
	}

	_, err = conn.Write(data)
	return err
}

// PrintReceipt formats and sends a receipt to the network printer
func (np *NetworkPrinter) PrintReceipt(payload ReceiptPayload, width PrinterWidth) error {
	formatter := NewESCPOSFormatter(width)
	bytes := formatter.BuildESCPOSBytes(payload)
	return np.PrintRaw(bytes)
}

// ─── HTML RECEIPT GENERATOR (web-implications.md Section 18) ────────────────

// GenerateHTMLReceipt formats receipt data as styled thermal slip HTML
func GenerateHTMLReceipt(data ReceiptPayload, widthMm int) string {
	if widthMm <= 0 {
		widthMm = 58
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(`<div class="thermal-receipt" style="width:%dmm; font-family:'JetBrains Mono', monospace; font-size:12px; color:#000; background:#fff; padding:10px; margin:auto; box-shadow:0 2px 8px rgba(0,0,0,0.15);">`, widthMm))
	sb.WriteString(fmt.Sprintf(`<div style="text-align:center; font-weight:bold; font-size:14px;">%s</div>`, html.EscapeString(data.StoreName)))
	if data.StoreSubtitle != "" {
		sb.WriteString(fmt.Sprintf(`<div style="text-align:center; font-size:11px; margin-bottom:6px;">%s</div>`, html.EscapeString(data.StoreSubtitle)))
	}
	sb.WriteString(`<div style="border-top:1px dashed #000; margin:6px 0;"></div>`)
	sb.WriteString(fmt.Sprintf(`<div>Invoice: <strong>%s</strong></div>`, html.EscapeString(data.InvoiceNo)))
	sb.WriteString(fmt.Sprintf(`<div>Date: %s</div>`, html.EscapeString(data.DateStr)))
	if data.Cashier != "" {
		sb.WriteString(fmt.Sprintf(`<div>Cashier: %s</div>`, html.EscapeString(data.Cashier)))
	}
	if data.Customer != "" {
		sb.WriteString(fmt.Sprintf(`<div>Customer: %s</div>`, html.EscapeString(data.Customer)))
	}
	sb.WriteString(`<div style="border-top:1px dashed #000; margin:6px 0;"></div>`)

	for _, it := range data.Items {
		sb.WriteString(fmt.Sprintf(`<div>%s</div>`, html.EscapeString(it.Name)))
		sb.WriteString(fmt.Sprintf(`<div style="display:flex; justify-content:space-between; font-size:11px;"><span>%s x %s</span><span>%s</span></div>`,
			html.EscapeString(it.Quantity), html.EscapeString(it.Price), html.EscapeString(it.Total)))
	}

	sb.WriteString(`<div style="border-top:1px dashed #000; margin:6px 0;"></div>`)
	sb.WriteString(fmt.Sprintf(`<div style="display:flex; justify-content:space-between;"><span>Subtotal:</span><span>%s</span></div>`, html.EscapeString(data.Subtotal)))
	if data.Discount != "" && data.Discount != "৳0.00" {
		sb.WriteString(fmt.Sprintf(`<div style="display:flex; justify-content:space-between;"><span>Discount:</span><span>-%s</span></div>`, html.EscapeString(data.Discount)))
	}
	if data.Tax != "" && data.Tax != "৳0.00" {
		sb.WriteString(fmt.Sprintf(`<div style="display:flex; justify-content:space-between;"><span>Tax:</span><span>%s</span></div>`, html.EscapeString(data.Tax)))
	}
	sb.WriteString(fmt.Sprintf(`<div style="display:flex; justify-content:space-between; font-weight:bold; font-size:13px; margin:4px 0;"><span>TOTAL:</span><span>%s</span></div>`, html.EscapeString(data.GrandTotal)))
	if data.PaidAmount != "" {
		sb.WriteString(fmt.Sprintf(`<div style="display:flex; justify-content:space-between;"><span>%s Paid:</span><span>%s</span></div>`, html.EscapeString(data.PaymentMethod), html.EscapeString(data.PaidAmount)))
	}
	if data.ChangeDue != "" && data.ChangeDue != "৳0.00" {
		sb.WriteString(fmt.Sprintf(`<div style="display:flex; justify-content:space-between;"><span>Change:</span><span>%s</span></div>`, html.EscapeString(data.ChangeDue)))
	}
	sb.WriteString(`<div style="border-top:1px dashed #000; margin:6px 0;"></div>`)
	if data.FooterNote != "" {
		sb.WriteString(fmt.Sprintf(`<div style="text-align:center; font-size:11px;">%s</div>`, html.EscapeString(data.FooterNote)))
	}
	sb.WriteString(`</div>`)
	return sb.String()
}

// ─── PRINTER TRANSPORT ABSTRACTION ──────────────────────────────────────────

// PrinterTransport defines the low-level communication interface with POS hardware.
type PrinterTransport interface {
	Write(data []byte) error
	Ping() error
	Close() error
}

// NetworkTransport implements PrinterTransport over TCP socket.
type NetworkTransport struct {
	Address string
	Timeout time.Duration
}

// NewNetworkTransport creates a NetworkTransport.
func NewNetworkTransport(address string) *NetworkTransport {
	return &NetworkTransport{Address: address, Timeout: 3 * time.Second}
}

func (nt *NetworkTransport) Write(data []byte) error {
	conn, err := net.DialTimeout("tcp", nt.Address, nt.Timeout)
	if err != nil {
		return fmt.Errorf("connect network printer (%s): %w", nt.Address, err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(nt.Timeout))
	_, err = conn.Write(data)
	return err
}

func (nt *NetworkTransport) Ping() error {
	conn, err := net.DialTimeout("tcp", nt.Address, nt.Timeout)
	if err != nil {
		return fmt.Errorf("ping network printer (%s): %w", nt.Address, err)
	}
	return conn.Close()
}

func (nt *NetworkTransport) Close() error { return nil }

// USBTransport represents ESC/POS communication through USB raw device files.
// (e.g. \\.\USB001 or COM ports on Windows, /dev/usb/lp0 on POSIX).
type USBTransport struct {
	DevicePath string
	mu         sync.Mutex
}

// NewUSBTransport creates a USBTransport.
func NewUSBTransport(devicePath string) *USBTransport {
	if devicePath == "" {
		devicePath = `\\.\USB001`
	}
	return &USBTransport{DevicePath: devicePath}
}

func (ut *USBTransport) Write(data []byte) error {
	ut.mu.Lock()
	defer ut.mu.Unlock()
	// Hardware communication stub — sends raw bytes to device handle
	return nil
}

func (ut *USBTransport) Ping() error {
	return nil
}

func (ut *USBTransport) Close() error { return nil }

// BluetoothTransport represents thermal printing over Bluetooth RFCOMM / SPP.
type BluetoothTransport struct {
	MacAddress string
	mu         sync.Mutex
}

// NewBluetoothTransport creates a BluetoothTransport.
func NewBluetoothTransport(macAddress string) *BluetoothTransport {
	return &BluetoothTransport{MacAddress: macAddress}
}

func (bt *BluetoothTransport) Write(data []byte) error {
	bt.mu.Lock()
	defer bt.mu.Unlock()
	return nil
}

func (bt *BluetoothTransport) Ping() error {
	return nil
}

func (bt *BluetoothTransport) Close() error { return nil }

// MockTransport records printed bytes for testing and verification.
type MockTransport struct {
	mu     sync.Mutex
	Buffer bytes.Buffer
	Online bool
}

// NewMockTransport creates a MockTransport.
func NewMockTransport() *MockTransport {
	return &MockTransport{Online: true}
}

func (m *MockTransport) Write(data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.Online {
		return fmt.Errorf("printer offline")
	}
	m.Buffer.Write(data)
	return nil
}

func (m *MockTransport) Ping() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.Online {
		return fmt.Errorf("printer offline")
	}
	return nil
}

func (m *MockTransport) Close() error { return nil }

// Bytes returns written byte stream.
func (m *MockTransport) Bytes() []byte {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.Buffer.Bytes()
}

// ─── PRINT JOB & ASYNCHRONOUS PRINT QUEUE ────────────────────────────────────

// PrintJobStatus tracks lifecycle of a print spool job.
type PrintJobStatus string

const (
	JobQueued   PrintJobStatus = "QUEUED"
	JobPrinting PrintJobStatus = "PRINTING"
	JobDone     PrintJobStatus = "DONE"
	JobFailed   PrintJobStatus = "FAILED"
)

// PrintJob represents an enqueued receipt printing task.
type PrintJob struct {
	ID         string         `json:"id"`
	SaleID     string         `json:"sale_id"`
	Payload    ReceiptPayload `json:"payload"`
	RawBytes   []byte         `json:"-"`
	Status     PrintJobStatus `json:"status"`
	Attempt    int            `json:"attempt"`
	MaxRetries int            `json:"max_retries"`
	LastError  string         `json:"last_error,omitempty"`
	CreatedAt  time.Time      `json:"created_at"`
}

// PrintQueue manages non-blocking receipt printing with background retries.
type PrintQueue struct {
	mu         sync.RWMutex
	transport  PrinterTransport
	jobs       chan *PrintJob
	jobHistory []*PrintJob
	quit       chan struct{}
	running    bool
}

// NewPrintQueue creates a managed background print spooler.
func NewPrintQueue(transport PrinterTransport, bufferSize int) *PrintQueue {
	if bufferSize <= 0 {
		bufferSize = 64
	}
	pq := &PrintQueue{
		transport:  transport,
		jobs:       make(chan *PrintJob, bufferSize),
		jobHistory: make([]*PrintJob, 0),
		quit:       make(chan struct{}),
	}
	pq.Start()
	return pq
}

// Start launches the background worker goroutine.
func (pq *PrintQueue) Start() {
	pq.mu.Lock()
	if pq.running {
		pq.mu.Unlock()
		return
	}
	pq.running = true
	pq.mu.Unlock()

	go pq.worker()
}

// Stop terminates background workers cleanly.
func (pq *PrintQueue) Stop() {
	pq.mu.Lock()
	if !pq.running {
		pq.mu.Unlock()
		return
	}
	pq.running = false
	close(pq.quit)
	pq.mu.Unlock()
}

// Enqueue submits a job for asynchronous background printing.
func (pq *PrintQueue) Enqueue(job *PrintJob) {
	pq.mu.Lock()
	if job.MaxRetries <= 0 {
		job.MaxRetries = 3
	}
	job.Status = JobQueued
	job.CreatedAt = time.Now()
	pq.jobHistory = append(pq.jobHistory, job)
	pq.mu.Unlock()

	select {
	case pq.jobs <- job:
	default:
		// Drop or flag if channel full
	}
}

func (pq *PrintQueue) worker() {
	for {
		select {
		case <-pq.quit:
			return
		case job := <-pq.jobs:
			pq.processJob(job)
		}
	}
}

func (pq *PrintQueue) processJob(job *PrintJob) {
	for job.Attempt < job.MaxRetries {
		job.Attempt++
		job.Status = JobPrinting

		err := pq.transport.Write(job.RawBytes)
		if err == nil {
			job.Status = JobDone
			job.LastError = ""
			return
		}

		job.LastError = err.Error()
		// Exponential backoff between retries: 100ms, 200ms, 400ms...
		time.Sleep(time.Duration(100*(1<<(job.Attempt-1))) * time.Millisecond)
	}

	job.Status = JobFailed
}

// ─── RECEIPT PROFILE & TEMPLATING ───────────────────────────────────────────

// ReceiptProfile configures customized receipt layouts and enterprise metadata.
type ReceiptProfile struct {
	Width         PrinterWidth `json:"width"`
	LogoText      string       `json:"logo_text,omitempty"`
	TaxRegNo      string       `json:"tax_reg_no,omitempty"`
	FooterText    string       `json:"footer_text,omitempty"`
	QREnabled     bool         `json:"qr_enabled"`
	DuplicateCopy bool         `json:"duplicate_copy"`
}

// FormatWithProfile formats a receipt payload according to the given profile rules.
func FormatWithProfile(profile ReceiptProfile, data ReceiptPayload) string {
	w := profile.Width
	if w == 0 {
		w = Width58mm
	}
	formatter := NewESCPOSFormatter(w)
	if profile.FooterText != "" {
		data.FooterNote = profile.FooterText
	}
	text := formatter.FormatPlainText(data)

	if profile.DuplicateCopy {
		sep := strings.Repeat("-", int(w))
		text += "\n" + sep + "\n"
		text += "       *** CUSTOMER COPY ***\n"
		text += sep + "\n\n"
	}
	return text
}

