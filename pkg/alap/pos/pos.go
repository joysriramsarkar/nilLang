package pos

import (
	"github.com/joysriramsarkar/nilLang/pkg/alap/data"
	"github.com/joysriramsarkar/nilLang/pkg/alap/sync"
)

// POSEngine represents the unified enterprise retail engine specified in web-implications.md
type POSEngine struct {
	Catalog   *CatalogRepository
	Inventory *InventoryLedger
	Carts     *CartManager
	Shifts    *ShiftManager
	Customers *CustomerRepository
	Audit     *AuditTrail
	Checkout  *CheckoutService
	Refund    *RefundService
	Reports   *ReportingEngine
	SyncQueue *sync.SyncQueue
	Sync      *sync.SyncEngine
}

// NewPOSEngine constructs a fully wired POS application engine
func NewPOSEngine() *POSEngine {
	catalog := NewCatalogRepository()
	inventory := NewInventoryLedger(catalog)
	shifts := NewShiftManager()
	customers := NewCustomerRepository()
	audit := NewAuditTrail()
	checkout := NewCheckoutService(catalog, inventory, shifts, customers, audit)
	refund := NewRefundService(checkout, inventory, audit)
	reports := NewReportingEngine(checkout, inventory, catalog)
	carts := NewCartManager()
	syncQueue := sync.NewSyncQueue()
	syncEngine := sync.NewSyncEngine()

	engine := &POSEngine{
		Catalog:   catalog,
		Inventory: inventory,
		Carts:     carts,
		Shifts:    shifts,
		Customers: customers,
		Audit:     audit,
		Checkout:  checkout,
		Refund:    refund,
		Reports:   reports,
		SyncQueue: syncQueue,
		Sync:      syncEngine,
	}

	// Register sync handler for offline transactions
	syncEngine.RegisterHandler("sale", func(op *sync.MutationOperation) error {
		// Idempotently apply synced sale
		return nil
	})

	return engine
}

// SeedDefaultEnterpriseData seeds baseline categories, products, and customers
func (pe *POSEngine) SeedDefaultEnterpriseData() {
	// Categories
	pe.Catalog.AddCategory(&Category{ID: "cat-1", Name: "Rice & Pulses", NameBn: "চাল ও ডাল", Icon: "🌾"})
	pe.Catalog.AddCategory(&Category{ID: "cat-2", Name: "Oil & Ghee", NameBn: "তেল ও ঘি", Icon: "🛢️"})
	pe.Catalog.AddCategory(&Category{ID: "cat-3", Name: "Flour & Sugar", NameBn: "আটা ও চিনি", Icon: "🥣"})
	pe.Catalog.AddCategory(&Category{ID: "cat-4", Name: "Spices & Salt", NameBn: "মসলা ও লবণ", Icon: "🧂"})
	pe.Catalog.AddCategory(&Category{ID: "cat-5", Name: "Tea & Beverages", NameBn: "চা ও পানীয়", Icon: "☕"})

	// Baseline Products
	pe.Catalog.AddProduct(&Product{
		ID:          "p-01",
		SKU:         "RICE-MIN-50",
		Barcode:     "8901030012345",
		Name:        "Miniket Rice (50kg Bag)",
		NameBn:      "মিনিকেট চাল (৫০ কেজি বস্তা)",
		CategoryID:  "cat-1",
		Unit:        "বস্তা",
		Price:       data.NewMoney(340000, "BDT"), // ৳3,400.00
		Cost:        data.NewMoney(305000, "BDT"), // ৳3,050.00 WAC
		Stock:       data.NewDecimalFromInt(45),
		LowStockMin: data.NewDecimalFromInt(10),
		Active:      true,
	})

	pe.Catalog.AddProduct(&Product{
		ID:          "p-02",
		SKU:         "RICE-NAZ-25",
		Barcode:     "8901030012346",
		Name:        "Nazirshail Premium Rice (25kg)",
		NameBn:      "নাজিরশাইল প্রিমিয়াম চাল (২৫ কেজি)",
		CategoryID:  "cat-1",
		Unit:        "বস্তা",
		Price:       data.NewMoney(215000, "BDT"),
		Cost:        data.NewMoney(192000, "BDT"),
		Stock:       data.NewDecimalFromInt(30),
		LowStockMin: data.NewDecimalFromInt(8),
		Active:      true,
	})

	pe.Catalog.AddProduct(&Product{
		ID:          "p-03",
		SKU:         "OIL-TEER-5L",
		Barcode:     "8901030098765",
		Name:        "Teer Fortified Soybean Oil (5L)",
		NameBn:      "তীর ফর্টিফাইড সয়াবিন তেল (৫ লিটার)",
		CategoryID:  "cat-2",
		Unit:        "বোতল",
		Price:       data.NewMoney(89000, "BDT"),
		Cost:        data.NewMoney(81000, "BDT"),
		Stock:       data.NewDecimalFromInt(65),
		LowStockMin: data.NewDecimalFromInt(15),
		Active:      true,
	})

	pe.Catalog.AddProduct(&Product{
		ID:          "p-04",
		SKU:         "OIL-RUP-2L",
		Barcode:     "8901030098766",
		Name:        "Rupchanda Soybean Oil (2L)",
		NameBn:      "রূপচাঁদা সয়াবিন তেল (২ লিটার)",
		CategoryID:  "cat-2",
		Unit:        "বোতল",
		Price:       data.NewMoney(37000, "BDT"),
		Cost:        data.NewMoney(33500, "BDT"),
		Stock:       data.NewDecimalFromInt(80),
		LowStockMin: data.NewDecimalFromInt(20),
		Active:      true,
	})

	pe.Catalog.AddProduct(&Product{
		ID:          "p-05",
		SKU:         "ATTA-FRSH-2K",
		Barcode:     "8901030055555",
		Name:        "Fresh Chakki Atta (2kg)",
		NameBn:      "ফ্রেশ চাক্কি আটা (২ কেজি)",
		CategoryID:  "cat-3",
		Unit:        "প্যাকেট",
		Price:       data.NewMoney(12500, "BDT"),
		Cost:        data.NewMoney(10500, "BDT"),
		Stock:       data.NewDecimalFromInt(120),
		LowStockMin: data.NewDecimalFromInt(25),
		Active:      true,
	})

	pe.Catalog.AddProduct(&Product{
		ID:          "p-06",
		SKU:         "SUG-DESH-1K",
		Barcode:     "8901030055556",
		Name:        "Deshi White Sugar (1kg)",
		NameBn:      "দেশি সাদা চিনি (১ কেজি)",
		CategoryID:  "cat-3",
		Unit:        "কেজি",
		Price:       data.NewMoney(14000, "BDT"),
		Cost:        data.NewMoney(12200, "BDT"),
		Stock:       data.NewDecimalFromInt(95),
		LowStockMin: data.NewDecimalFromInt(20),
		Active:      true,
	})

	pe.Catalog.AddProduct(&Product{
		ID:          "p-07",
		SKU:         "TEA-ISPA-400",
		Barcode:     "8901030077777",
		Name:        "Ispahani Mirzapore Tea (400g)",
		NameBn:      "ইস্পাহানি মির্জাপুর চা (৪০০ গ্রাম)",
		CategoryID:  "cat-5",
		Unit:        "প্যাকেট",
		Price:       data.NewMoney(23000, "BDT"),
		Cost:        data.NewMoney(19800, "BDT"),
		Stock:       data.NewDecimalFromInt(50),
		LowStockMin: data.NewDecimalFromInt(12),
		Active:      true,
	})

	pe.Catalog.AddProduct(&Product{
		ID:          "p-08",
		SKU:         "SALT-MOL-1K",
		Barcode:     "8901030077778",
		Name:        "Molla Super Salt (1kg)",
		NameBn:      "মোল্লা সুপার সল্ট (১ কেজি)",
		CategoryID:  "cat-4",
		Unit:        "প্যাকেট",
		Price:       data.NewMoney(4200, "BDT"),
		Cost:        data.NewMoney(3400, "BDT"),
		Stock:       data.NewDecimalFromInt(150),
		LowStockMin: data.NewDecimalFromInt(30),
		Active:      true,
	})

	// Baseline Customers
	pe.Customers.AddCustomer(&Customer{
		ID:                  "c-01",
		Name:                "রহিম উদ্দিন (মেসার্স রহিম ট্রেডার্স)",
		Phone:               "01711-223344",
		Address:             "চকবাজার, ঢাকা",
		TotalPurchasesMinor: 14500000, // ৳145,000
		DueBalanceMinor:     2500000,  // ৳25,000 outstanding baki
	})

	pe.Customers.AddCustomer(&Customer{
		ID:                  "c-02",
		Name:                "করিম ভাই (করিম স্টোর)",
		Phone:               "01819-887766",
		Address:             "শ্যামবাজার, ঢাকা",
		TotalPurchasesMinor: 8900000,
		DueBalanceMinor:     1200000,
	})

	pe.Customers.AddCustomer(&Customer{
		ID:                  "c-03",
		Name:                "আব্দুল লতিফ (খুচরা ক্রেতা)",
		Phone:               "01912-345678",
		Address:             "মিরপুর-১০, ঢাকা",
		TotalPurchasesMinor: 320000,
		DueBalanceMinor:     0,
	})

	// Open Initial Shift
	_, _ = pe.Shifts.OpenShift("reg-01", "cashier-01", "জয় সরকার (Joy Sarkar)", 1000000) // ৳10,000 float
}
