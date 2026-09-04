package ui

import (
	"strings"
	"testing"
)

func TestAlapUIComponents(t *testing.T) {
	theme := OnuronTheme()
	page := NewPage("Shop Manager")

	nav := NewNavigation("ShopApp").
		AddItem("Products", "/products").
		AddItem("Orders", "/orders")
	page.SetNav(nav)

	dash := NewDashboard("Overview").
		AddMetric("Sales", "$12,450", "+14%").
		AddMetric("Orders", "342", "+8%")
	page.Add(dash)

	table := NewTable("Product ID", "Title", "Stock").
		AddRow("P-101", "Nil Book", "50").
		AddRow("P-102", "Onuron T-Shirt", "120")
	page.Add(table)

	form := NewForm("Add Product").
		AddField("Title", "title", "Product Title").
		AddField("Price", "price", "0.00")
	page.Add(form)

	page.SetFooter("Powered by NilLang & Alap Framework")

	// Test ANSI rendering
	ansi := page.RenderANSI(theme)
	if !strings.Contains(ansi, "Shop Manager") || !strings.Contains(ansi, "Products (") {
		t.Errorf("ANSI render missing required elements: %s", ansi)
	}

	// Test HTML rendering
	html := page.RenderHTML(theme)
	if !strings.Contains(html, "<!DOCTYPE html>") || !strings.Contains(html, "Shop Manager") {
		t.Errorf("HTML render missing required elements: %s", html)
	}

	// Test nil/ui Primitive building
	primitive := page.Build(theme)
	if primitive == nil {
		t.Fatalf("expected page.Build to return a primitive")
	}
}
