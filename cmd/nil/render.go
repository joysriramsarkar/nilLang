package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/joysriramsarkar/nilLang/compiler/evaluator"
	"github.com/joysriramsarkar/nilLang/compiler/lexer"
	"github.com/joysriramsarkar/nilLang/compiler/object"
	"github.com/joysriramsarkar/nilLang/compiler/parser"
	"github.com/joysriramsarkar/nilLang/pkg/alap/ui"
)

func cmdRender() {
	fmt.Println("🎨 Alap UI Component Renderer - Onuron OS & Web (refactor.md Section 12)")
	fmt.Println("═══════════════════════════════════════════════════════════════")

	entryFile := ""
	if len(os.Args) > 2 {
		entryFile = os.Args[2]
	} else if _, err := os.Stat("src/main.nil"); err == nil {
		entryFile = "src/main.nil"
	} else if _, err := os.Stat("main.nil"); err == nil {
		entryFile = "main.nil"
	}

	theme := ui.OnuronTheme()
	var page *ui.Page

	if entryFile != "" {
		code, err := os.ReadFile(entryFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "❌ ফাইল পড়তে সমস্যা (%s): %s\n", entryFile, err)
			return
		}

		l := lexer.New(string(code))
		p := parser.New(l)
		program := p.ParseProgram()
		if len(p.Errors()) > 0 {
			fmt.Fprintf(os.Stderr, "❌ পার্সিং ভুল (%s):\n", entryFile)
			for _, e := range p.Errors() {
				fmt.Fprintf(os.Stderr, "  - %s\n", e)
			}
			return
		}

		env := object.NewEnvironment()
		evaluated := evaluator.Eval(program, env)

		// 1. Check if 'page' variable exists in environment
		if pageVal, ok := env.Get("page"); ok {
			if hash, isHash := pageVal.(*object.Hash); isHash {
				page = convertHashToPage(hash)
			}
		}

		// 2. If evaluated expression itself is a Hash with type Page
		if page == nil {
			if hash, ok := evaluated.(*object.Hash); ok {
				page = convertHashToPage(hash)
			}
		}

		// 3. Fallback: inspect store for any Page hash object
		if page == nil {
			for _, val := range env.Store() {
				if hash, isHash := val.(*object.Hash); isHash {
					if getHashStr(hash, "type") == "Page" {
						page = convertHashToPage(hash)
						break
					}
				}
			}
		}
	}

	if page == nil {
		page = ui.NewPage("NilLang Control Center")
		nav := ui.NewNavigation("Alap UI").
			AddItem("Dashboard", "/dashboard").
			AddItem("Services", "/services").
			AddItem("Settings", "/settings")
		page.SetNav(nav)

		dash := ui.NewDashboard("System Metrics").
			AddMetric("Active Tasks", "14", "+2").
			AddMetric("Memory Usage", "128 MB", "-4%").
			AddMetric("Softbus Messages", "8,940", "+25%")
		page.Add(dash)

		table := ui.NewTable("Service ID", "Profile", "Status").
			AddRow("srv-web-1", "Web (WASM)", "ACTIVE").
			AddRow("srv-data-2", "Data Science", "TRAINING").
			AddRow("srv-onuron-3", "Onuron Native", "STANDBY")
		page.Add(table)

		form := ui.NewForm("Deploy Microservice").
			AddField("Service Name", "name", "e.g. auth-service").
			AddField("Profile", "profile", "web / server / mobile")
		page.Add(form)

		page.SetFooter("Alap Application Framework • Powered by NilLang Core")
	}

	// Render ANSI to terminal
	fmt.Println()
	fmt.Print(page.RenderANSI(theme))

	// Render HTML Preview
	html := page.RenderHTML(theme)
	outputPath := "build/preview.html"
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		fmt.Fprintf(os.Stderr, "❌ ডিরেক্টরি তৈরি করতে সমস্যা: %s\n", err)
		return
	}
	if err := os.WriteFile(outputPath, []byte(html), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "❌ HTML সেভ করতে সমস্যা: %s\n", err)
	} else {
		fmt.Println()
		fmt.Printf("✅ আধুনিক ওয়েব প্রিভিউ সেভ হয়েছে: %s\n", outputPath)
		fmt.Println("   ব্রাউজারে দেখতে: Invoke-Item " + outputPath + " (বা open " + outputPath + ")")
	}
}

func convertHashToPage(hash *object.Hash) *ui.Page {
	title := getHashStr(hash, "title")
	if title == "" {
		title = "Alap Web App"
	}
	page := ui.NewPage(title)

	if navObj := getHashObj(hash, "navigation"); navObj != nil {
		if navHash, ok := navObj.(*object.Hash); ok {
			brand := getHashStr(navHash, "brand")
			nav := ui.NewNavigation(brand)
			if itemsArr := getHashArr(navHash, "items"); itemsArr != nil {
				for _, elem := range itemsArr.Elements {
					if itemHash, ok := elem.(*object.Hash); ok {
						label := getHashStr(itemHash, "label")
						path := getHashStr(itemHash, "path")
						nav.AddItem(label, path)
					}
				}
			}
			page.SetNav(nav)
		}
	}

	if contentArr := getHashArr(hash, "content"); contentArr != nil {
		for _, elem := range contentArr.Elements {
			if elemHash, ok := elem.(*object.Hash); ok {
				comp := convertHashToComponent(elemHash)
				if comp != nil {
					page.Add(comp)
				}
			}
		}
	}

	if footer := getHashStr(hash, "footer"); footer != "" {
		page.SetFooter(footer)
	}

	return page
}

func convertHashToComponent(hash *object.Hash) ui.Component {
	typ := getHashStr(hash, "type")
	switch typ {
	case "Dashboard":
		title := getHashStr(hash, "title")
		dash := ui.NewDashboard(title)
		if metricsArr := getHashArr(hash, "metrics"); metricsArr != nil {
			for _, elem := range metricsArr.Elements {
				if metricHash, ok := elem.(*object.Hash); ok {
					label := getHashStr(metricHash, "label")
					value := getHashStr(metricHash, "value")
					delta := getHashStr(metricHash, "delta")
					dash.AddMetric(label, value, delta)
				}
			}
		}
		return dash

	case "Table":
		var headers []string
		if headersArr := getHashArr(hash, "headers"); headersArr != nil {
			for _, h := range headersArr.Elements {
				headers = append(headers, getObjStr(h))
			}
		}
		tbl := ui.NewTable(headers...)
		if rowsArr := getHashArr(hash, "rows"); rowsArr != nil {
			for _, rElem := range rowsArr.Elements {
				var cells []string
				if rowArr, ok := rElem.(*object.Array); ok {
					for _, c := range rowArr.Elements {
						cells = append(cells, getObjStr(c))
					}
				}
				tbl.AddRow(cells...)
			}
		}
		return tbl

	case "Form":
		title := getHashStr(hash, "title")
		form := ui.NewForm(title)
		if fieldsArr := getHashArr(hash, "fields"); fieldsArr != nil {
			for _, elem := range fieldsArr.Elements {
				if fieldHash, ok := elem.(*object.Hash); ok {
					label := getHashStr(fieldHash, "label")
					name := getHashStr(fieldHash, "name")
					placeholder := getHashStr(fieldHash, "placeholder")
					form.AddField(label, name, placeholder)
				}
			}
		}
		return form

	case "Card":
		title := getHashStr(hash, "title")
		body := getHashStr(hash, "body")
		return ui.NewCard(title, body)

	default:
		return nil
	}
}

func getHashStr(hash *object.Hash, key string) string {
	val := getHashObj(hash, key)
	if val == nil {
		return ""
	}
	return getObjStr(val)
}

func getObjStr(val object.Object) string {
	if val == nil {
		return ""
	}
	if str, ok := val.(*object.String); ok {
		return str.Value
	}
	return val.Inspect()
}

func getHashObj(hash *object.Hash, key string) object.Object {
	if hash == nil {
		return nil
	}
	strKey := &object.String{Value: key}
	pair, ok := hash.Pairs[strKey.HashKey()]
	if ok {
		return pair.Value
	}
	return nil
}

func getHashArr(hash *object.Hash, key string) *object.Array {
	obj := getHashObj(hash, key)
	if obj == nil {
		return nil
	}
	if arr, ok := obj.(*object.Array); ok {
		return arr
	}
	return nil
}
