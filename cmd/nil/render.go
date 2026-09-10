package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/joysriramsarkar/nilLang/compiler/evaluator"
	"github.com/joysriramsarkar/nilLang/compiler/lexer"
	"github.com/joysriramsarkar/nilLang/compiler/object"
	"github.com/joysriramsarkar/nilLang/compiler/parser"
	"github.com/joysriramsarkar/nilLang/compiler/typecheck"
	"github.com/joysriramsarkar/nilLang/pkg/alap/ui"
)

type declarativeUIApp struct {
	component *object.Hash
	page      *object.Hash
}

func loadDeclarativeUI(source string) (*declarativeUIApp, error) {
	l := lexer.New(source)
	p := parser.New(l)
	program := p.ParseProgram()
	if len(p.Errors()) > 0 {
		return nil, fmt.Errorf("parse errors: %s", strings.Join(p.Errors(), "; "))
	}
	checker := typecheck.NewChecker()
	if !checker.CheckProgram(program) {
		diagnostics := make([]string, 0, len(checker.Diagnostics))
		for _, diagnostic := range checker.Diagnostics {
			diagnostics = append(diagnostics, diagnostic.String())
		}
		return nil, fmt.Errorf("type errors: %s", strings.Join(diagnostics, "; "))
	}

	env := object.NewEnvironment()
	for name, builtin := range evaluator.Builtins {
		env.Set(name, builtin)
	}
	evaluated := evaluator.Eval(program, env)
	if evalError, ok := evaluated.(*object.Error); ok {
		return nil, fmt.Errorf("evaluation error: %s", evalError.Inspect())
	}

	if component := findComponent(evaluated, env); component != nil {
		return &declarativeUIApp{component: component}, nil
	}
	if page := findPage(evaluated, env); page != nil {
		return &declarativeUIApp{page: page}, nil
	}
	return nil, fmt.Errorf("no declarative component or Page found")
}

func findComponent(evaluated object.Object, env *object.Environment) *object.Hash {
	if hash, ok := evaluated.(*object.Hash); ok && getHashStr(hash, "__type") == "Component" {
		return hash
	}
	for _, value := range env.Store() {
		if hash, ok := value.(*object.Hash); ok && getHashStr(hash, "__type") == "Component" {
			return hash
		}
	}
	return nil
}

func findPage(evaluated object.Object, env *object.Environment) *object.Hash {
	if hash, ok := evaluated.(*object.Hash); ok && getHashStr(hash, "type") == "Page" {
		return hash
	}
	if value, ok := env.Get("page"); ok {
		if hash, ok := value.(*object.Hash); ok && getHashStr(hash, "type") == "Page" {
			return hash
		}
	}
	for _, value := range env.Store() {
		if hash, ok := value.(*object.Hash); ok && getHashStr(hash, "type") == "Page" {
			return hash
		}
	}
	return nil
}

func (app *declarativeUIApp) Render() (*ui.Page, error) {
	tree := app.page
	if app.component != nil {
		tree = renderComponentHash(app.component)
	}
	if tree == nil {
		return nil, fmt.Errorf("component render/build must return a Page hash")
	}
	if getHashStr(tree, "type") != "Page" {
		return nil, fmt.Errorf("component render/build returned %q, want Page", getHashStr(tree, "type"))
	}
	return convertHashToPage(tree), nil
}

func (app *declarativeUIApp) Dispatch(event string, payload ...interface{}) error {
	if app.component == nil {
		return fmt.Errorf("a static Page does not support event dispatch")
	}
	dispatch, ok := getHashObj(app.component, "dispatch").(*object.Builtin)
	if !ok {
		return fmt.Errorf("component does not support event dispatch")
	}
	args := []object.Object{&object.String{Value: event}}
	if len(payload) > 1 {
		return fmt.Errorf("dispatch accepts at most one payload")
	}
	if len(payload) == 1 {
		args = append(args, eventPayloadObject(payload[0]))
	}
	result := dispatch.Fn(args...)
	if evalError, ok := result.(*object.Error); ok {
		return fmt.Errorf("dispatch %q failed: %s", event, evalError.Inspect())
	}
	return nil
}

func eventPayloadObject(value interface{}) object.Object {
	switch value := value.(type) {
	case nil:
		return &object.Null{}
	case object.Object:
		return value
	case string:
		return &object.String{Value: value}
	case bool:
		return &object.Boolean{Value: value}
	case float64:
		return &object.Float{Value: value}
	case float32:
		return &object.Float{Value: float64(value)}
	case int:
		return &object.Integer{Value: int64(value)}
	case int64:
		return &object.Integer{Value: value}
	case []interface{}:
		elements := make([]object.Object, len(value))
		for index, item := range value {
			elements[index] = eventPayloadObject(item)
		}
		return &object.Array{Elements: elements}
	case map[string]interface{}:
		values := make(map[string]object.Object, len(value))
		for key, item := range value {
			values[key] = eventPayloadObject(item)
		}
		return evaluator.MakeHashObj(values)
	default:
		return &object.String{Value: fmt.Sprint(value)}
	}
}

func (app *declarativeUIApp) State() *object.Hash {
	if app.component == nil {
		return nil
	}
	state, _ := getHashObj(app.component, "state").(*object.Hash)
	return state
}

func (app *declarativeUIApp) LastEvent() *object.Hash {
	if app.component == nil {
		return nil
	}
	event, _ := getHashObj(app.component, "lastEvent").(*object.Hash)
	return event
}

func cmdRender() {
	fmt.Println("🎨 Alap UI Component Renderer - Onuron OS & Web (Alap UI Component Engine)")
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
	var app *declarativeUIApp

	if entryFile != "" {
		code, err := os.ReadFile(entryFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "❌ ফাইল পড়তে সমস্যা (%s): %s\n", entryFile, err)
			return
		}

		app, err = loadDeclarativeUI(string(code))
		if err != nil {
			fmt.Fprintf(os.Stderr, "❌ %s-এ declarative UI লোড করা যায়নি: %s\n", entryFile, err)
			return
		}
		if event := renderEventArg(os.Args[3:]); event != "" {
			if err := app.Dispatch(event); err != nil {
				fmt.Fprintf(os.Stderr, "❌ %s event dispatch করা যায়নি: %s\n", entryFile, err)
				return
			}
		}
		page, err = app.Render()
		if err != nil {
			fmt.Fprintf(os.Stderr, "❌ %s render করা যায়নি: %s\n", entryFile, err)
			return
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
	if app != nil && app.State() != nil {
		html = page.RenderSSR(theme, hashToGoMap(app.State()))
	}
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

func renderEventArg(args []string) string {
	for index := 0; index < len(args); index++ {
		if args[index] == "--event" && index+1 < len(args) {
			return args[index+1]
		}
		if strings.HasPrefix(args[index], "--event=") {
			return strings.TrimPrefix(args[index], "--event=")
		}
	}
	return ""
}

func hashToGoMap(hash *object.Hash) map[string]interface{} {
	result := make(map[string]interface{}, len(hash.Pairs))
	for _, pair := range hash.Pairs {
		key, ok := pair.Key.(*object.String)
		if !ok {
			continue
		}
		result[key.Value] = objectToGoValue(pair.Value)
	}
	return result
}

func objectToGoValue(value object.Object) interface{} {
	switch value := value.(type) {
	case *object.Integer:
		return value.Value
	case *object.Float:
		return value.Value
	case *object.Boolean:
		return value.Value
	case *object.String:
		return value.Value
	case *object.Array:
		items := make([]interface{}, len(value.Elements))
		for index, element := range value.Elements {
			items[index] = objectToGoValue(element)
		}
		return items
	case *object.Hash:
		return hashToGoMap(value)
	default:
		return nil
	}
}

func renderComponentHash(component *object.Hash) *object.Hash {
	renderer := getHashObj(component, "render")
	if renderer == nil {
		renderer = getHashObj(component, "build")
	}
	function, ok := renderer.(*object.Function)
	if !ok {
		return nil
	}
	env := object.NewEnclosedEnvironment(function.Env)
	result := evaluator.Eval(function.Body, env)
	if returned, ok := result.(*object.ReturnValue); ok {
		result = returned.Value
	}
	hash, _ := result.(*object.Hash)
	return hash
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

	case "Button":
		button := ui.NewButton(getHashStr(hash, "id"), getHashStr(hash, "label"))
		button.OnClick = getHashStr(hash, "event")
		if payload := getHashObj(hash, "payload"); payload != nil {
			button.Payload = objectToGoValue(payload)
		}
		button.Variant = getHashStr(hash, "variant")
		button.Disabled = getHashBool(hash, "disabled")
		return button

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

func getHashBool(hash *object.Hash, key string) bool {
	value, ok := getHashObj(hash, key).(*object.Boolean)
	return ok && value.Value
}
