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
