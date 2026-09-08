package posui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/joysriramsarkar/nilLang/pkg/alap/data"
	"github.com/joysriramsarkar/nilLang/pkg/alap/pos"
)

// POSUIServer serves the production POS web UI and REST API.
type POSUIServer struct {
	engine *pos.POSEngine
	mux    *http.ServeMux
}

// NewPOSUIServer creates the POS HTTP server with all routes mounted.
func NewPOSUIServer(engine *pos.POSEngine) *POSUIServer {
	s := &POSUIServer{engine: engine, mux: http.NewServeMux()}
	s.registerRoutes()
	return s
}

// ServeHTTP implements http.Handler.
func (s *POSUIServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

// ListenAndServe starts the HTTP server.
func (s *POSUIServer) ListenAndServe(addr string) error {
	return http.ListenAndServe(addr, s.mux)
}

func (s *POSUIServer) registerRoutes() {
	s.mux.HandleFunc("/", s.handleIndex)
	s.mux.HandleFunc("/api/products", s.handleProducts)
	s.mux.HandleFunc("/api/products/search", s.handleProductSearch)
	s.mux.HandleFunc("/api/products/barcode/", s.handleBarcodeSearch)
	s.mux.HandleFunc("/api/categories", s.handleCategories)
	s.mux.HandleFunc("/api/customers", s.handleCustomers)
	s.mux.HandleFunc("/api/cart/checkout", s.handleCheckout)
	s.mux.HandleFunc("/api/shift/current", s.handleCurrentShift)
	s.mux.HandleFunc("/api/shift/open", s.handleOpenShift)
	s.mux.HandleFunc("/api/shift/close", s.handleCloseShift)
	s.mux.HandleFunc("/api/shift/cash-movement", s.handleCashMovement)
	s.mux.HandleFunc("/api/sales/recent", s.handleRecentSales)
	s.mux.HandleFunc("/api/inventory/movements", s.handleInventoryMovements)
	s.mux.HandleFunc("/api/health", s.handleHealth)
}

// ─── PAGE ──────────────────────────────────────────────────────────────────────

func (s *POSUIServer) handleIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, posUIHTML)
}

// ─── API HANDLERS ─────────────────────────────────────────────────────────────

func (s *POSUIServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "version": "1.0.0"})
}

func (s *POSUIServer) handleProducts(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.engine.Catalog.AllProducts())
}

func (s *POSUIServer) handleProductSearch(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	writeJSON(w, http.StatusOK, s.engine.Catalog.Search(q))
}

func (s *POSUIServer) handleBarcodeSearch(w http.ResponseWriter, r *http.Request) {
	barcode := strings.TrimPrefix(r.URL.Path, "/api/products/barcode/")
	if p, found := s.engine.Catalog.FindByBarcode(barcode); found {
		writeJSON(w, http.StatusOK, p)
		return
	}
	writeJSON(w, http.StatusNotFound, map[string]string{"error": "product not found: " + barcode})
}

func (s *POSUIServer) handleCategories(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.engine.Catalog.AllCategories())
}

func (s *POSUIServer) handleCustomers(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	all := s.engine.Customers.AllCustomers()
	if q == "" {
		writeJSON(w, http.StatusOK, all)
		return
	}
	ql := strings.ToLower(q)
	var results []*pos.Customer
	for _, c := range all {
		if strings.Contains(strings.ToLower(c.Name), ql) || strings.Contains(c.Phone, q) {
			results = append(results, c)
		}
	}
	if results == nil {
		results = []*pos.Customer{}
	}
	writeJSON(w, http.StatusOK, results)
}

// ─── CHECKOUT ─────────────────────────────────────────────────────────────────

type checkoutRequest struct {
	Items      []cartItemReq  `json:"items"`
	Payments   []paymentReq   `json:"payments"`
	CustomerID string         `json:"customer_id"`
	CashierID  string         `json:"cashier_id"`
	RegisterID string         `json:"register_id"`
	CouponCode string         `json:"coupon_code"`
}
type cartItemReq struct {
	ProductID string `json:"product_id"`
	Quantity  string `json:"quantity"` // decimal string e.g. "2.5"
}
type paymentReq struct {
	Method string `json:"method"`
	Amount int64  `json:"amount_minor"`
}

func (s *POSUIServer) handleCheckout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "POST required"})
		return
	}
	var req checkoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}

	cart := pos.NewCart(fmt.Sprintf("web-%s", req.CashierID), req.CustomerID)
	for _, item := range req.Items {
		p, found := s.engine.Catalog.FindByID(item.ProductID)
		if !found {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "product not found: " + item.ProductID})
			return
		}
		qty, err := data.ParseDecimal(item.Quantity)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid quantity for " + item.ProductID + ": " + err.Error()})
			return
		}
		cart.AddProduct(p, qty)
	}
	if req.CouponCode != "" {
		cart.ApplyCoupon(req.CouponCode)
	}
	cart.Recalculate()

	payments := make([]pos.PaymentRecord, len(req.Payments))
	for i, p := range req.Payments {
		payments[i] = pos.PaymentRecord{
			Method:      pos.PaymentMethod(p.Method),
			AmountMinor: p.Amount,
		}
	}

	cashierID := req.CashierID
	if cashierID == "" {
		cashierID = "cashier-01"
	}
	registerID := req.RegisterID
	if registerID == "" {
		registerID = "reg-01"
	}

	result, err := s.engine.Checkout.Execute(cart, payments, cashierID, registerID, req.CustomerID)
	if err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"sale":           result.Sale,
		"tender":         result.Tender,
		"receipt_text":   result.ReceiptText,
		"trigger_drawer": result.TriggerDrawer,
	})
}

// ─── SHIFT ────────────────────────────────────────────────────────────────────

func (s *POSUIServer) handleCurrentShift(w http.ResponseWriter, r *http.Request) {
	shift, err := s.engine.Shifts.CurrentShift()
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, shift)
}

type openShiftReq struct {
	RegisterID        string `json:"register_id"`
	CashierID         string `json:"cashier_id"`
	CashierName       string `json:"cashier_name"`
	StartingCashMinor int64  `json:"starting_cash_minor"`
}

func (s *POSUIServer) handleOpenShift(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "POST required"})
		return
	}
	var req openShiftReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	shift, err := s.engine.Shifts.OpenShift(req.RegisterID, req.CashierID, req.CashierName, req.StartingCashMinor)
	if err != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, shift)
}

type closeShiftReq struct {
	ActualCashMinor int64  `json:"actual_cash_minor"`
	Notes           string `json:"notes"`
}

func (s *POSUIServer) handleCloseShift(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "POST required"})
		return
	}
	var req closeShiftReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	shift, err := s.engine.Shifts.CloseShift(req.ActualCashMinor, req.Notes)
	if err != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, shift)
}

type cashMovementReq struct {
	Type        string `json:"type"` // "CASH_IN" or "CASH_OUT"
	AmountMinor int64  `json:"amount_minor"`
	Reason      string `json:"reason"`
}

func (s *POSUIServer) handleCashMovement(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "POST required"})
		return
	}
	var req cashMovementReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	movType := pos.CashMovementIn
	if req.Type == "CASH_OUT" {
		movType = pos.CashMovementOut
	}
	mov, err := s.engine.Shifts.RecordCashMovement(movType, req.AmountMinor, req.Reason)
	if err != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, mov)
}

// ─── REPORTING ────────────────────────────────────────────────────────────────

func (s *POSUIServer) handleRecentSales(w http.ResponseWriter, r *http.Request) {
	all := s.engine.Checkout.AllSales()
	limit := 50
	if len(all) < limit {
		limit = len(all)
	}
	writeJSON(w, http.StatusOK, all[:limit])
}

func (s *POSUIServer) handleInventoryMovements(w http.ResponseWriter, r *http.Request) {
	productID := r.URL.Query().Get("product_id")
	if productID != "" {
		writeJSON(w, http.StatusOK, s.engine.Inventory.MovementsForProduct(productID))
		return
	}
	writeJSON(w, http.StatusOK, s.engine.Inventory.RecentMovements(100))
}

// ─── HELPER ───────────────────────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
