package pos

import (
	"github.com/joysriramsarkar/nilLang/pkg/alap/entity"
)

// CanonicalEntitySchemas returns the complete set of enterprise POS Entity schemas
// specified in web-implications.md Section 4.
func CanonicalEntitySchemas() map[string]*entity.Entity {
	schemas := make(map[string]*entity.Entity)

	// 1. Organization
	org := entity.NewEntity("Organization").
		AddField("id", entity.TypeUUID, true).
		AddField("name", entity.TypeString, true).
		AddField("tax_number", entity.TypeString, false).
		AddField("active", entity.TypeBool, true)
	schemas["Organization"] = org

	// 2. Store
	store := entity.NewEntity("Store").
		AddField("id", entity.TypeUUID, true).
		AddField("org_id", entity.TypeUUID, true).
		AddField("name", entity.TypeString, true).
		AddField("address", entity.TypeString, false).
		AddField("phone", entity.TypeString, false)
	schemas["Store"] = store

	// 3. Register
	reg := entity.NewEntity("Register").
		AddField("id", entity.TypeUUID, true).
		AddField("store_id", entity.TypeUUID, true).
		AddField("name", entity.TypeString, true).
		AddField("device_id", entity.TypeString, false).
		AddField("status", entity.TypeString, true)
	schemas["Register"] = reg

	// 4. Shift
	shift := entity.NewEntity("Shift").
		AddField("id", entity.TypeUUID, true).
		AddField("register_id", entity.TypeUUID, true).
		AddField("cashier_id", entity.TypeString, true).
		AddField("opening_cash", entity.TypeMoney, true).
		AddField("closing_cash", entity.TypeMoney, false).
		AddField("status", entity.TypeString, true).
		AddField("started_at", entity.TypeDate, true).
		AddField("closed_at", entity.TypeDate, false)
	schemas["Shift"] = shift

	// 5. User
	user := entity.NewEntity("User").
		AddField("id", entity.TypeUUID, true).
		AddField("username", entity.TypeString, true).
		AddField("name", entity.TypeString, true).
		AddField("role", entity.TypeString, true).
		AddField("active", entity.TypeBool, true)
	schemas["User"] = user

	// 6. Category
	cat := entity.NewEntity("Category").
		AddField("id", entity.TypeUUID, true).
		AddField("name", entity.TypeString, true).
		AddField("name_bn", entity.TypeString, false).
		AddField("icon", entity.TypeString, false)
	schemas["Category"] = cat

	// 7. Brand
	brand := entity.NewEntity("Brand").
		AddField("id", entity.TypeUUID, true).
		AddField("name", entity.TypeString, true).
		AddField("name_bn", entity.TypeString, false)
	schemas["Brand"] = brand

	// 8. Unit
	unit := entity.NewEntity("Unit").
		AddField("id", entity.TypeUUID, true).
		AddField("symbol", entity.TypeString, true).
		AddField("name", entity.TypeString, true).
		AddField("dimension", entity.TypeString, true)
	schemas["Unit"] = unit

	// 9. Product
	prod := entity.NewEntity("Product").
		AddField("id", entity.TypeUUID, true).
		AddField("sku", entity.TypeString, true).
		AddField("barcode", entity.TypeString, false).
		AddField("name", entity.TypeString, true).
		AddField("name_bn", entity.TypeString, false).
		AddField("category_id", entity.TypeString, true).
		AddField("brand_id", entity.TypeString, false).
		AddField("cost", entity.TypeMoney, true).
		AddField("price", entity.TypeMoney, true).
		AddField("stock", entity.TypeQuantity, true).
		AddField("unit", entity.TypeString, true).
		AddField("active", entity.TypeBool, true)
	schemas["Product"] = prod

	// 10. ProductVariant
	variant := entity.NewEntity("ProductVariant").
		AddField("id", entity.TypeUUID, true).
		AddField("product_id", entity.TypeUUID, true).
		AddField("sku", entity.TypeString, true).
		AddField("barcode", entity.TypeString, false).
		AddField("name", entity.TypeString, true).
		AddField("price", entity.TypeMoney, true)
	schemas["ProductVariant"] = variant

	// 11. Inventory
	inv := entity.NewEntity("Inventory").
		AddField("id", entity.TypeUUID, true).
		AddField("product_id", entity.TypeUUID, true).
		AddField("warehouse_id", entity.TypeUUID, false).
		AddField("stock", entity.TypeQuantity, true).
		AddField("low_stock_min", entity.TypeQuantity, false)
	schemas["Inventory"] = inv

	// 12. StockMovement
	sm := entity.NewEntity("StockMovement").
		AddField("id", entity.TypeUUID, true).
		AddField("product_id", entity.TypeUUID, true).
		AddField("type", entity.TypeString, true).
		AddField("delta", entity.TypeQuantity, true).
		AddField("balance_after", entity.TypeQuantity, true).
		AddField("unit_cost", entity.TypeMoney, false).
		AddField("reference", entity.TypeString, false)
	schemas["StockMovement"] = sm

	// 13. Warehouse
	wh := entity.NewEntity("Warehouse").
		AddField("id", entity.TypeUUID, true).
		AddField("store_id", entity.TypeUUID, false).
		AddField("name", entity.TypeString, true).
		AddField("location", entity.TypeString, false)
	schemas["Warehouse"] = wh

	// 14. Customer
	cust := entity.NewEntity("Customer").
		AddField("id", entity.TypeUUID, true).
		AddField("name", entity.TypeString, true).
		AddField("phone", entity.TypeString, false).
		AddField("address", entity.TypeString, false).
		AddField("due_balance", entity.TypeMoney, false)
	schemas["Customer"] = cust

	// 15. Supplier
	supp := entity.NewEntity("Supplier").
		AddField("id", entity.TypeUUID, true).
		AddField("name", entity.TypeString, true).
		AddField("phone", entity.TypeString, false).
		AddField("contact_person", entity.TypeString, false)
	schemas["Supplier"] = supp

	// 16. Purchase & PurchaseItem
	purch := entity.NewEntity("Purchase").
		AddField("id", entity.TypeUUID, true).
		AddField("supplier_id", entity.TypeUUID, true).
		AddField("invoice_no", entity.TypeString, true).
		AddField("total_amount", entity.TypeMoney, true)
	schemas["Purchase"] = purch

	purchItem := entity.NewEntity("PurchaseItem").
		AddField("id", entity.TypeUUID, true).
		AddField("purchase_id", entity.TypeUUID, true).
		AddField("product_id", entity.TypeUUID, true).
		AddField("quantity", entity.TypeQuantity, true).
		AddField("unit_cost", entity.TypeMoney, true)
	schemas["PurchaseItem"] = purchItem

	// 17. Sale & SaleItem
	sale := entity.NewEntity("Sale").
		AddField("id", entity.TypeUUID, true).
		AddField("invoice_number", entity.TypeString, true).
		AddField("register_id", entity.TypeString, true).
		AddField("cashier_id", entity.TypeString, true).
		AddField("customer_id", entity.TypeString, false).
		AddField("subtotal", entity.TypeMoney, true).
		AddField("discount", entity.TypeMoney, false).
		AddField("tax", entity.TypeMoney, false).
		AddField("total", entity.TypeMoney, true).
		AddField("paid", entity.TypeMoney, true).
		AddField("change", entity.TypeMoney, false).
		AddField("status", entity.TypeString, true)
	schemas["Sale"] = sale

	saleItem := entity.NewEntity("SaleItem").
		AddField("id", entity.TypeUUID, true).
		AddField("sale_id", entity.TypeUUID, true).
		AddField("product_id", entity.TypeUUID, true).
		AddField("name", entity.TypeString, true).
		AddField("sku", entity.TypeString, false).
		AddField("quantity", entity.TypeQuantity, true).
		AddField("unit_price", entity.TypeMoney, true).
		AddField("cost_price", entity.TypeMoney, false).
		AddField("subtotal", entity.TypeMoney, true)
	schemas["SaleItem"] = saleItem

	// 18. Payment
	pmt := entity.NewEntity("Payment").
		AddField("id", entity.TypeUUID, true).
		AddField("sale_id", entity.TypeUUID, true).
		AddField("method", entity.TypeString, true).
		AddField("amount", entity.TypeMoney, true).
		AddField("reference", entity.TypeString, false)
	schemas["Payment"] = pmt

	// 19. Refund & RefundItem
	ref := entity.NewEntity("Refund").
		AddField("id", entity.TypeUUID, true).
		AddField("sale_id", entity.TypeUUID, true).
		AddField("cashier_id", entity.TypeString, true).
		AddField("total_refund", entity.TypeMoney, true).
		AddField("reason", entity.TypeString, false)
	schemas["Refund"] = ref

	refItem := entity.NewEntity("RefundItem").
		AddField("id", entity.TypeUUID, true).
		AddField("refund_id", entity.TypeUUID, true).
		AddField("product_id", entity.TypeUUID, true).
		AddField("quantity", entity.TypeQuantity, true).
		AddField("amount", entity.TypeMoney, true)
	schemas["RefundItem"] = refItem

	// 20. Tax & Discount & Promotion
	tax := entity.NewEntity("Tax").
		AddField("id", entity.TypeUUID, true).
		AddField("name", entity.TypeString, true).
		AddField("rate", entity.TypeQuantity, true).
		AddField("type", entity.TypeString, true).
		AddField("enabled", entity.TypeBool, true)
	schemas["Tax"] = tax

	disc := entity.NewEntity("Discount").
		AddField("id", entity.TypeUUID, true).
		AddField("name", entity.TypeString, true).
		AddField("type", entity.TypeString, true).
		AddField("value", entity.TypeQuantity, true).
		AddField("reason", entity.TypeString, false)
	schemas["Discount"] = disc

	// 21. Receipt & AuditLog
	rcpt := entity.NewEntity("Receipt").
		AddField("id", entity.TypeUUID, true).
		AddField("sale_id", entity.TypeUUID, true).
		AddField("invoice_number", entity.TypeString, true).
		AddField("rendered_text", entity.TypeMarkdown, true)
	schemas["Receipt"] = rcpt

	audit := entity.NewEntity("AuditLog").
		AddField("id", entity.TypeUUID, true).
		AddField("action", entity.TypeString, true).
		AddField("entity_id", entity.TypeString, true).
		AddField("user_id", entity.TypeString, true).
		AddField("details", entity.TypeMarkdown, false)
	schemas["AuditLog"] = audit

	// 22. SyncOperation
	syncOp := entity.NewEntity("SyncOperation").
		AddField("id", entity.TypeUUID, true).
		AddField("operation_id", entity.TypeString, true).
		AddField("device_id", entity.TypeString, true).
		AddField("entity_type", entity.TypeString, true).
		AddField("status", entity.TypeString, true)
	schemas["SyncOperation"] = syncOp

	return schemas
}
