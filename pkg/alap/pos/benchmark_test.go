package pos

import (
	"testing"

	"github.com/joysriramsarkar/nilLang/pkg/alap/data"
)

// BenchmarkProductSearch tests product catalog search performance (<100ms target)
func BenchmarkProductSearch(b *testing.B) {
	engine := NewPOSEngine()
	engine.SeedDefaultEnterpriseData()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = engine.Catalog.Search("মিনিকেট")
	}
}

// BenchmarkCartUpdate tests cart line item addition and pricing recalculation (≈16ms target)
func BenchmarkCartUpdate(b *testing.B) {
	engine := NewPOSEngine()
	engine.SeedDefaultEnterpriseData()
	prod, _ := engine.Catalog.FindByID("p-01")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cart := NewCart("bench-cart", "Benchmark")
		cart.AddProduct(prod, data.NewDecimalFromInt(1))
		cart.Recalculate()
	}
}

// BenchmarkTransactionalCheckout tests complete atomic checkout pipeline (<100ms target)
func BenchmarkTransactionalCheckout(b *testing.B) {
	pool := data.NewDBPool(data.DBPoolConfig{Driver: data.DriverSQLite})
	engine := NewPOSEngine()
	engine.SetDBPool(pool)
	engine.SeedDefaultEnterpriseData()

	prod, _ := engine.Catalog.FindByID("p-01")
	// Add abundant stock for benchmark runs
	_, _ = engine.Catalog.UpdateStock(prod.ID, data.NewDecimalFromInt(int64(b.N*5)))

	payments := []PaymentRecord{
		{Method: MethodCash, AmountMinor: 340000},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cart := NewCart("bench-cart", "Benchmark")
		cart.AddProduct(prod, data.NewDecimalFromInt(1))
		cart.Recalculate()
		_, _ = engine.Checkout.Execute(cart, payments, "cashier-01", "reg-01", "c-01")
	}
}
