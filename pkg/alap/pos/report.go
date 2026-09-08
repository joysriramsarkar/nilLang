package pos

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"sort"
	"time"

	"github.com/joysriramsarkar/nilLang/pkg/alap/data"
)

// TopProductStat aggregates top selling products
type TopProductStat struct {
	ProductID    string       `json:"product_id"`
	SKU          string       `json:"sku"`
	Name         string       `json:"name"`
	QuantitySold data.Decimal `json:"quantity_sold"`
	RevenueMinor int64        `json:"revenue_minor"`
}

// DailySalesReport aggregates key retail performance indicators
type DailySalesReport struct {
	Date                string           `json:"date"`
	TotalOrders         int64            `json:"total_orders"`
	TotalGrossSales     int64            `json:"total_gross_sales_minor"`
	TotalDiscounts      int64            `json:"total_discounts_minor"`
	TotalTax            int64            `json:"total_tax_minor"`
	TotalNetSales       int64            `json:"total_net_sales_minor"`
	TotalProfitMinor    int64            `json:"total_profit_minor"`
	ProfitMarginPercent float64          `json:"profit_margin_percent"`
	PaymentBreakdown    map[string]int64 `json:"payment_breakdown"`
	TopProducts         []TopProductStat `json:"top_products"`
}

// ReportingEngine produces analytical reports and exports
type ReportingEngine struct {
	checkout  *CheckoutService
	inventory *InventoryLedger
	catalog   *CatalogRepository
}

// NewReportingEngine creates a ReportingEngine
func NewReportingEngine(checkout *CheckoutService, inventory *InventoryLedger, catalog *CatalogRepository) *ReportingEngine {
	return &ReportingEngine{
		checkout:  checkout,
		inventory: inventory,
		catalog:   catalog,
	}
}

// GenerateDailyReport compiles daily retail performance
func (re *ReportingEngine) GenerateDailyReport(targetDate time.Time) *DailySalesReport {
	sales := re.checkout.AllSales()

	rep := &DailySalesReport{
		Date:             targetDate.Format("2006-01-02"),
		PaymentBreakdown: make(map[string]int64),
		TopProducts:      make([]TopProductStat, 0),
	}

	prodMap := make(map[string]*TopProductStat)

	for _, s := range sales {
		if s.Status == StatusCancelled {
			continue
		}

		rep.TotalOrders++
		rep.TotalGrossSales += s.SubtotalMinor
		rep.TotalDiscounts += s.DiscountMinor
		rep.TotalTax += s.TaxMinor
		rep.TotalNetSales += s.TotalMinor
		rep.TotalProfitMinor += s.GrossProfitMinor()

		for _, p := range s.Payments {
			rep.PaymentBreakdown[string(p.Method)] += p.AmountMinor
		}

		for _, it := range s.Items {
			stat, ok := prodMap[it.ProductID]
			if !ok {
				stat = &TopProductStat{
					ProductID: it.ProductID,
					SKU:       it.SKU,
					Name:      it.Name,
				}
				prodMap[it.ProductID] = stat
			}
			stat.QuantitySold = stat.QuantitySold.Add(it.Quantity)
			stat.RevenueMinor += it.SubtotalMinor
		}
	}

	if rep.TotalNetSales > 0 {
		rep.ProfitMarginPercent = (float64(rep.TotalProfitMinor) / float64(rep.TotalNetSales)) * 100.0
	}

	topList := make([]TopProductStat, 0, len(prodMap))
	for _, v := range prodMap {
		topList = append(topList, *v)
	}

	sort.Slice(topList, func(i, j int) bool {
		return topList[i].RevenueMinor > topList[j].RevenueMinor
	})

	if len(topList) > 5 {
		topList = topList[:5]
	}
	rep.TopProducts = topList

	return rep
}

// ExportSalesCSV exports sales transactions as standard CSV
func (re *ReportingEngine) ExportSalesCSV() (string, error) {
	sales := re.checkout.AllSales()

	var buf bytes.Buffer
	w := csv.NewWriter(&buf)

	// Header
	_ = w.Write([]string{
		"Invoice Number", "Date", "Customer ID", "Items Count",
		"Subtotal (BDT)", "Discount (BDT)", "Tax (BDT)", "Total (BDT)", "Status",
	})

	for _, s := range sales {
		row := []string{
			s.InvoiceNumber,
			s.CreatedAt.Format("2006-01-02 15:04:05"),
			s.CustomerID,
			fmt.Sprintf("%d", len(s.Items)),
			data.NewMoney(s.SubtotalMinor, "BDT").Format(),
			data.NewMoney(s.DiscountMinor, "BDT").Format(),
			data.NewMoney(s.TaxMinor, "BDT").Format(),
			data.NewMoney(s.TotalMinor, "BDT").Format(),
			string(s.Status),
		}
		_ = w.Write(row)
	}

	w.Flush()
	return buf.String(), w.Error()
}

// ─── STOCK VALUATION & PROFITABILITY ────────────────────────────────────────

// StockValuationReport analyzes current inventory cost vs retail value
type StockValuationReport struct {
	TotalSKUs              int          `json:"total_skus"`
	TotalUnits             data.Decimal `json:"total_units"`
	TotalCostValueMinor    int64        `json:"total_cost_value_minor"`
	TotalRetailValueMinor  int64        `json:"total_retail_value_minor"`
	UnrealizedProfitMinor  int64        `json:"unrealized_profit_minor"`
	ProjectedMarginPercent float64      `json:"projected_margin_percent"`
}

// GenerateStockValuationReport compiles real-time inventory assets and valuation
func (re *ReportingEngine) GenerateStockValuationReport() *StockValuationReport {
	rep := &StockValuationReport{}
	products := re.catalog.AllProducts()

	for _, p := range products {
		if !p.Active || p.Stock.Value <= 0 {
			continue
		}
		rep.TotalSKUs++
		rep.TotalUnits = rep.TotalUnits.Add(p.Stock)

		costVal := p.Cost.MulDecimal(p.Stock).Minor
		retailVal := p.Price.MulDecimal(p.Stock).Minor

		rep.TotalCostValueMinor += costVal
		rep.TotalRetailValueMinor += retailVal
	}

	rep.UnrealizedProfitMinor = rep.TotalRetailValueMinor - rep.TotalCostValueMinor
	if rep.TotalRetailValueMinor > 0 {
		rep.ProjectedMarginPercent = (float64(rep.UnrealizedProfitMinor) / float64(rep.TotalRetailValueMinor)) * 100.0
	}

	return rep
}

// ─── CASHIER PERFORMANCE REPORT ──────────────────────────────────────────────

// CashierPerformanceReport aggregates orders and sales volume by cashier
type CashierPerformanceReport struct {
	CashierID            string  `json:"cashier_id"`
	TotalOrders          int64   `json:"total_orders"`
	TotalRevenueMinor    int64   `json:"total_revenue_minor"`
	AverageOrderValMinor int64   `json:"avg_order_value_minor"`
	CashTenderMinor      int64   `json:"cash_tender_minor"`
	CardTenderMinor      int64   `json:"card_tender_minor"`
	MFSTenderMinor       int64   `json:"mfs_tender_minor"`
}

// GenerateCashierPerformanceReport produces cashier productivity breakdown
func (re *ReportingEngine) GenerateCashierPerformanceReport() []*CashierPerformanceReport {
	cashierMap := make(map[string]*CashierPerformanceReport)

	for _, s := range re.checkout.AllSales() {
		if s.Status == StatusCancelled {
			continue
		}
		cashier := s.CashierID
		if cashier == "" {
			cashier = "unassigned"
		}
		rep, ok := cashierMap[cashier]
		if !ok {
			rep = &CashierPerformanceReport{CashierID: cashier}
			cashierMap[cashier] = rep
		}

		rep.TotalOrders++
		rep.TotalRevenueMinor += s.TotalMinor

		for _, p := range s.Payments {
			switch p.Method {
			case MethodCash:
				rep.CashTenderMinor += p.AmountMinor
			case MethodCard:
				rep.CardTenderMinor += p.AmountMinor
			case MethodBKash, MethodNagad, MethodUPI:
				rep.MFSTenderMinor += p.AmountMinor
			}
		}
	}

	res := make([]*CashierPerformanceReport, 0, len(cashierMap))
	for _, rep := range cashierMap {
		if rep.TotalOrders > 0 {
			rep.AverageOrderValMinor = rep.TotalRevenueMinor / rep.TotalOrders
		}
		res = append(res, rep)
	}

	sort.Slice(res, func(i, j int) bool {
		return res[i].TotalRevenueMinor > res[j].TotalRevenueMinor
	})
	return res
}

// ─── TAX COMPLIANCE REPORT ──────────────────────────────────────────────────

// TaxReport aggregates tax collected across orders
type TaxReport struct {
	TotalOrders       int64 `json:"total_orders"`
	TotalSalesMinor   int64 `json:"total_sales_minor"`
	TotalTaxMinor     int64 `json:"total_tax_minor"`
	TaxableSalesMinor int64 `json:"taxable_sales_minor"`
}

// GenerateTaxReport produces VAT and tax compliance summary
func (re *ReportingEngine) GenerateTaxReport() *TaxReport {
	rep := &TaxReport{}
	for _, s := range re.checkout.AllSales() {
		if s.Status == StatusCancelled {
			continue
		}
		rep.TotalOrders++
		rep.TotalSalesMinor += s.TotalMinor
		rep.TotalTaxMinor += s.TaxMinor
		if s.TaxMinor > 0 {
			rep.TaxableSalesMinor += s.SubtotalMinor
		}
	}
	return rep
}

