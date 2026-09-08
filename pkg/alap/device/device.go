package device

import (
	"bytes"
	"fmt"
	"strings"
	"sync"
	"time"
)

// ─── BARCODE SUBSYSTEM ──────────────────────────────────────────────────────

// BarcodeScanner manages barcode input accumulation from keyboard wedge or serial/USB devices
type BarcodeScanner struct {
	mu            sync.Mutex
	buffer        strings.Builder
	lastKeystroke time.Time
	timeout       time.Duration
	onScan        func(barcode string)
}

// NewBarcodeScanner creates a barcode scanner listener
func NewBarcodeScanner(onScan func(barcode string)) *BarcodeScanner {
	return &BarcodeScanner{
		timeout: 100 * time.Millisecond, // Scanners fire keystrokes within milliseconds
		onScan:  onScan,
	}
}

// ProcessKey handles incoming keystrokes
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
		if code != "" && s.onScan != nil {
			s.onScan(code)
			return true
		}
		return false
	}

	s.buffer.WriteRune(r)
	return false
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
