package ui

import (
	"strings"
	"testing"
)

func TestPageRenderSSRIncludesInteractiveRoot(t *testing.T) {
	page := NewPage("Interactive").Add(NewButton("increment", "Increment"))
	button := page.Content[0].(*Button)
	button.OnClick = `increment<&"`
	button.Payload = map[string]interface{}{"name": `Ada & "Lin"`}

	html := page.RenderSSR(DefaultTheme(), map[string]interface{}{"count": int64(0)})
	for _, expected := range []string{
		"data-alap-root",
		`data-alap-click="increment&lt;&amp;&#34;"`,
		`data-alap-payload="{&#34;name&#34;:&#34;Ada \u0026 \&#34;Lin\&#34;&#34;}"`,
		`<script id="__NILANG_STATE__"`,
		`<script src="/alap-runtime.js"></script>`,
	} {
		if !strings.Contains(html, expected) {
			t.Fatalf("SSR HTML does not contain %q: %s", expected, html)
		}
	}
	if strings.Contains(html, "onclick=") {
		t.Fatalf("SSR HTML contains executable inline onclick: %s", html)
	}
}

func TestAlapExtendedPrimitives(t *testing.T) {
	theme := DefaultTheme()

	// 1. NumberInput
	numInput := NewNumberInput("qty", "Quantity", 5, 1, 100, 1)
	numInput.OnChange = "handleQtyChange"
	htmlNum := numInput.RenderHTML(theme)
	if !strings.Contains(htmlNum, `id="qty"`) || !strings.Contains(htmlNum, `value="5.00"`) || !strings.Contains(htmlNum, `data-alap-change="handleQtyChange"`) {
		t.Errorf("NumberInput HTML failed: %s", htmlNum)
	}

	// 2. Badge
	badgeSuccess := NewBadge("Paid", "success")
	badgeDanger := NewBadge("Due", "danger")
	if !strings.Contains(badgeSuccess.RenderHTML(theme), "Paid") || !strings.Contains(badgeDanger.RenderHTML(theme), "Due") {
		t.Errorf("Badge HTML failed")
	}

	// 3. DataGrid
	cols := []TableColumn{
		{Key: "sku", Title: "SKU", Sortable: true},
		{Key: "name", Title: "Product Name"},
		{Key: "price", Title: "Price", Align: "right"},
	}
	grid := NewDataGrid("dg-products", cols)
	grid.AddRow(map[string]string{"sku": "P-01", "name": "Miniket Rice", "price": "৳3,400.00"})
	htmlGrid := grid.RenderHTML(theme)
	if !strings.Contains(htmlGrid, "Miniket Rice") || !strings.Contains(htmlGrid, "৳3,400.00") {
		t.Errorf("DataGrid HTML failed: %s", htmlGrid)
	}

	// 4. Tabs
	tabs := NewTabs("tabs-pos")
	tabs.AddTab("catalog", "Catalog", NewButton("b1", "Catalog Btn"))
	tabs.AddTab("orders", "Orders", NewButton("b2", "Orders Btn"))
	htmlTabs := tabs.RenderHTML(theme)
	if !strings.Contains(htmlTabs, "Catalog") || !strings.Contains(htmlTabs, "Orders") {
		t.Errorf("Tabs HTML failed: %s", htmlTabs)
	}

	// 5. DatePicker
	dp := NewDatePicker("dp-1", "Sale Date", "2026-09-10")
	htmlDP := dp.RenderHTML(theme)
	if !strings.Contains(htmlDP, `type="date"`) || !strings.Contains(htmlDP, `value="2026-09-10"`) {
		t.Errorf("DatePicker HTML failed: %s", htmlDP)
	}

	// 6. CartPanelUI
	cartPanel := &CartPanelUI{
		Items: []CartLineItemUI{
			{ProductID: "p-01", Name: "Rice Bag", Quantity: "1", PriceStr: "৳3,400.00", TotalStr: "৳3,400.00"},
		},
		SubtotalStr:   "৳3,400.00",
		TaxStr:        "৳0.00",
		GrandTotalStr: "৳3,400.00",
		OnCheckout:    "startCheckout",
	}
	htmlCart := cartPanel.RenderHTML(theme)
	if !strings.Contains(htmlCart, "Rice Bag") || !strings.Contains(htmlCart, `data-alap-click="startCheckout"`) {
		t.Errorf("CartPanelUI HTML failed: %s", htmlCart)
	}

	// 7. ReceiptPreviewUI
	receipt := &ReceiptPreviewUI{
		StoreName:     "Kachabazar Superstore",
		InvoiceNumber: "INV-20260910-0001",
		DateStr:       "2026-09-10",
		CashierName:   "Alamin",
		Lines:         []string{"Rice Bag x 1 = ৳3,400.00"},
		GrandTotalStr: "৳3,400.00",
		PaidStr:       "৳3,500.00",
		ChangeStr:     "৳100.00",
	}
	htmlReceipt := receipt.RenderHTML(theme)
	if !strings.Contains(htmlReceipt, "Kachabazar Superstore") || !strings.Contains(htmlReceipt, "INV-20260910-0001") {
		t.Errorf("ReceiptPreviewUI HTML failed: %s", htmlReceipt)
	}
}
