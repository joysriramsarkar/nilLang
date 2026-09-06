package evaluator

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/joysriramsarkar/nilLang/compiler/ast"
	"github.com/joysriramsarkar/nilLang/compiler/lexer"
	"github.com/joysriramsarkar/nilLang/compiler/object"
	"github.com/joysriramsarkar/nilLang/compiler/parser"
)

var (
	nativeModulesMu sync.RWMutex
	nativeModules   = make(map[string]func() *object.Hash)

	scriptDirMu    sync.Mutex
	scriptDirStack []string
)

// PushScriptDir pushes a directory onto the script import resolution stack
func PushScriptDir(dir string) {
	scriptDirMu.Lock()
	defer scriptDirMu.Unlock()
	scriptDirStack = append(scriptDirStack, dir)
}

// PopScriptDir pops a directory from the script import resolution stack
func PopScriptDir() {
	scriptDirMu.Lock()
	defer scriptDirMu.Unlock()
	if len(scriptDirStack) > 0 {
		scriptDirStack = scriptDirStack[:len(scriptDirStack)-1]
	}
}

// GetCurrentScriptDir returns the current directory of the executing script
func GetCurrentScriptDir() string {
	scriptDirMu.Lock()
	defer scriptDirMu.Unlock()
	if len(scriptDirStack) > 0 {
		return scriptDirStack[len(scriptDirStack)-1]
	}
	return ""
}

// RegisterNativeModule registers a factory creating a native module object
func RegisterNativeModule(name string, factory func() *object.Hash) {
	nativeModulesMu.Lock()
	defer nativeModulesMu.Unlock()
	nativeModules[name] = factory
	nativeModules[strings.ToLower(name)] = factory
}

// GetNativeModule returns a registered native module
func GetNativeModule(name string) (*object.Hash, bool) {
	nativeModulesMu.RLock()
	factory, ok := nativeModules[strings.ToLower(name)]
	nativeModulesMu.RUnlock()

	if ok {
		return factory(), true
	}
	return nil, false
}

func init() {
	// Register standard native libraries
	RegisterNativeModule("web", createWebModule)
	RegisterNativeModule("alap/web", createWebModule)
	RegisterNativeModule("data", createDataModule)
	RegisterNativeModule("alap/data", createDataModule)
	RegisterNativeModule("db", createDataModule)
	RegisterNativeModule("money", createMoneyModule)
	RegisterNativeModule("alap/money", createMoneyModule)
	RegisterNativeModule("realtime", createRealtimeModule)
	RegisterNativeModule("alap/realtime", createRealtimeModule)
	RegisterNativeModule("security", createSecurityModule)
	RegisterNativeModule("alap/security", createSecurityModule)
}

func evalImportStatement(node *ast.ImportStatement, env *object.Environment) object.Object {
	if node.Path == nil {
		return newError("import path cannot be empty")
	}

	importPath := node.Path.Value

	// Check if this is a native module
	if mod, ok := GetNativeModule(importPath); ok {
		// Case 1: import { a, b } from "module"
		if len(node.Names) > 0 {
			for _, ident := range node.Names {
				keyObj := &object.String{Value: ident.Value}
				if pair, hasKey := mod.Pairs[keyObj.HashKey()]; hasKey {
					env.Set(ident.Value, pair.Value)
				} else {
					// Also check case-insensitive
					found := false
					for _, p := range mod.Pairs {
						if kStr, isStr := p.Key.(*object.String); isStr && strings.EqualFold(kStr.Value, ident.Value) {
							env.Set(ident.Value, p.Value)
							found = true
							break
						}
					}
					if !found {
						env.Set(ident.Value, NULL)
					}
				}
			}
			return mod
		}

		// Case 2: import "module" as alias
		if node.Alias != nil {
			env.Set(node.Alias.Value, mod)
			return mod
		}

		// Case 3: import "module" -> bound to module base name
		baseName := filepath.Base(importPath)
		baseName = strings.TrimSuffix(baseName, filepath.Ext(baseName))
		env.Set(baseName, mod)
		return mod
	}

	// Case 4: File-based import (.nil file)
	filePath := importPath
	if !strings.HasSuffix(filePath, ".nil") && !strings.Contains(filePath, ".") {
		filePath = filePath + ".nil"
	}

	var resolvedPath string
	currDir := GetCurrentScriptDir()
	if currDir != "" && !filepath.IsAbs(filePath) {
		candidate := filepath.Join(currDir, filePath)
		if _, err := os.Stat(candidate); err == nil {
			resolvedPath = candidate
		}
	}

	if resolvedPath == "" {
		if _, err := os.Stat(filePath); err == nil {
			resolvedPath = filePath
		} else {
			cwd, _ := os.Getwd()
			candidate := filepath.Join(cwd, filePath)
			if _, err := os.Stat(candidate); err == nil {
				resolvedPath = candidate
			}
		}
	}

	if resolvedPath == "" {
		return newError("cannot find module or file '%s'", importPath)
	}

	content, err := os.ReadFile(resolvedPath)
	if err != nil {
		return newError("cannot read module file '%s': %s", resolvedPath, err)
	}

	l := lexer.New(string(content))
	p := parser.New(l)
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		return newError("parse error in imported file %s: %s", resolvedPath, strings.Join(p.Errors(), "; "))
	}

	// Push the imported file's directory so nested imports resolve relative to it
	PushScriptDir(filepath.Dir(resolvedPath))
	defer PopScriptDir()

	// Evaluate file in sub-environment
	subEnv := object.NewEnclosedEnvironment(env)
	res := Eval(prog, subEnv)
	if isError(res) {
		return res
	}

	// Collect declared symbols into module hash
	modPairs := make(map[object.HashKey]object.HashPair)
	// We expose symbols from subEnv store
	for k, v := range subEnv.Store() {
		sk := &object.String{Value: k}
		modPairs[sk.HashKey()] = object.HashPair{Key: sk, Value: v}
	}
	// If the file explicitly returns a Hash, merge that
	if resHash, isHash := res.(*object.Hash); isHash {
		for k, v := range resHash.Pairs {
			modPairs[k] = v
		}
	}

	modObj := &object.Hash{Pairs: modPairs}

	if node.Alias != nil {
		env.Set(node.Alias.Value, modObj)
	} else if len(node.Names) > 0 {
		for _, ident := range node.Names {
			k := &object.String{Value: ident.Value}
			if pair, exists := modPairs[k.HashKey()]; exists {
				env.Set(ident.Value, pair.Value)
			}
		}
	} else {
		baseName := filepath.Base(importPath)
		baseName = strings.TrimSuffix(baseName, filepath.Ext(baseName))
		env.Set(baseName, modObj)
	}

	return modObj
}

func evalComponentLiteral(node *ast.ComponentLiteral, env *object.Environment) object.Object {
	name := "Anonymous"
	if node.Name != nil {
		name = node.Name.Value
	}

	// Build Component definition Hash
	compPairs := make(map[object.HashKey]object.HashPair)

	setCompProp := func(k string, v object.Object) {
		strKey := &object.String{Value: k}
		compPairs[strKey.HashKey()] = object.HashPair{Key: strKey, Value: v}
	}

	setCompProp("__type", &object.String{Value: "Component"})
	setCompProp("name", &object.String{Value: name})

	// Initial state map
	initialState := make(map[object.HashKey]object.HashPair)
	for _, st := range node.States {
		if st.Name != nil {
			var initVal object.Object = NULL
			if st.Value != nil {
				initVal = Eval(st.Value, env)
			}
			sk := &object.String{Value: st.Name.Value}
			initialState[sk.HashKey()] = object.HashPair{Key: sk, Value: initVal}
		}
	}
	setCompProp("state", &object.Hash{Pairs: initialState})

	// Render method
	setCompProp("render", &object.Builtin{
		Fn: func(args ...object.Object) object.Object {
			// Renders HTML string representation for SSR
			title := name
			if len(args) > 0 {
				title = args[0].Inspect()
			}
			htmlStr := fmt.Sprintf(`<div class="alap-component" data-component="%s" data-ssr="true"><h3>%s</h3></div>`, name, title)
			return &object.String{Value: htmlStr}
		},
	})

	compObj := &object.Hash{Pairs: compPairs}

	if node.Name != nil {
		env.Set(node.Name.Value, compObj)
	}

	return compObj
}

// ─── NATIVE MODULE IMPLEMENTATIONS ──────────────────────────────────────────

func createWebModule() *object.Hash {
	m := make(map[string]object.Object)

	m["new"] = &object.Builtin{
		Fn: func(args ...object.Object) object.Object {
			appName := "AlapWebApp"
			if len(args) > 0 {
				appName = args[0].Inspect()
			}
			return createAppObject(appName)
		},
	}

	m["json"] = &object.Builtin{
		Fn: func(args ...object.Object) object.Object {
			if len(args) == 0 {
				return &object.String{Value: "{}"}
			}
			return &object.String{Value: args[0].Inspect()}
		},
	}

	m["html"] = &object.Builtin{
		Fn: func(args ...object.Object) object.Object {
			if len(args) == 0 {
				return &object.String{Value: ""}
			}
			return &object.String{Value: args[0].Inspect()}
		},
	}

	// UI Component Helpers
	m["Button"] = &object.Builtin{
		Fn: func(args ...object.Object) object.Object {
			label := "Click Me"
			if len(args) > 0 {
				label = args[0].Inspect()
			}
			return MakeHashObj(map[string]object.Object{
				"tag":   &object.String{Value: "Button"},
				"label": &object.String{Value: label},
			})
		},
	}

	m["Text"] = &object.Builtin{
		Fn: func(args ...object.Object) object.Object {
			content := ""
			if len(args) > 0 {
				content = args[0].Inspect()
			}
			return MakeHashObj(map[string]object.Object{
				"tag":  &object.String{Value: "Text"},
				"text": &object.String{Value: content},
			})
		},
	}

	m["Card"] = &object.Builtin{
		Fn: func(args ...object.Object) object.Object {
			title := ""
			if len(args) > 0 {
				title = args[0].Inspect()
			}
			return MakeHashObj(map[string]object.Object{
				"tag":   &object.String{Value: "Card"},
				"title": &object.String{Value: title},
			})
		},
	}

	return MakeHashObj(m)
}

func createAppObject(name string) *object.Hash {
	routes := make([]object.Object, 0)
	var routesMu sync.Mutex

	appMap := make(map[string]object.Object)
	appMap["name"] = &object.String{Value: name}

	addRoute := func(method string, args []object.Object) object.Object {
		if len(args) < 2 {
			return newError("%s expects path and handler", method)
		}
		path := args[0].Inspect()
		routeDesc := MakeHashObj(map[string]object.Object{
			"method":  &object.String{Value: method},
			"path":    &object.String{Value: path},
			"handler": args[1],
		})
		routesMu.Lock()
		routes = append(routes, routeDesc)
		routesMu.Unlock()
		return routeDesc
	}

	appMap["get"] = &object.Builtin{
		Fn: func(args ...object.Object) object.Object {
			return addRoute("GET", args)
		},
	}
	appMap["post"] = &object.Builtin{
		Fn: func(args ...object.Object) object.Object {
			return addRoute("POST", args)
		},
	}
	appMap["put"] = &object.Builtin{
		Fn: func(args ...object.Object) object.Object {
			return addRoute("PUT", args)
		},
	}
	appMap["delete"] = &object.Builtin{
		Fn: func(args ...object.Object) object.Object {
			return addRoute("DELETE", args)
		},
	}

	appMap["routes"] = &object.Builtin{
		Fn: func(args ...object.Object) object.Object {
			routesMu.Lock()
			defer routesMu.Unlock()
			return &object.Array{Elements: routes}
		},
	}

	appMap["listen"] = &object.Builtin{
		Fn: func(args ...object.Object) object.Object {
			port := "8080"
			if len(args) > 0 {
				port = args[0].Inspect()
			}
			fmt.Printf("🚀 [Alap Web App: %s] Server configured on port :%s\n", name, port)
			return TRUE
		},
	}

	return MakeHashObj(appMap)
}

func createDataModule() *object.Hash {
	m := make(map[string]object.Object)

	// In-memory store for .nil data sessions
	var dbMu sync.RWMutex
	dbTables := make(map[string][]map[string]interface{})

	m["table"] = &object.Builtin{
		Fn: func(args ...object.Object) object.Object {
			if len(args) == 0 {
				return newError("table name required")
			}
			tableName := args[0].Inspect()
			return createTableObject(tableName, &dbTables, &dbMu)
		},
	}

	m["query"] = &object.Builtin{
		Fn: func(args ...object.Object) object.Object {
			if len(args) == 0 {
				return newError("SQL query required")
			}
			return &object.Array{Elements: []object.Object{}}
		},
	}

	return MakeHashObj(m)
}

func createTableObject(name string, tables *map[string][]map[string]interface{}, mu *sync.RWMutex) *object.Hash {
	t := make(map[string]object.Object)
	t["name"] = &object.String{Value: name}

	t["insert"] = &object.Builtin{
		Fn: func(args ...object.Object) object.Object {
			if len(args) == 0 {
				return newError("record map required")
			}
			hash, ok := args[0].(*object.Hash)
			if !ok {
				return newError("insert argument must be a Hash")
			}

			rec := make(map[string]interface{})
			for _, pair := range hash.Pairs {
				k := pair.Key.Inspect()
				rec[k] = pair.Value.Inspect()
			}

			mu.Lock()
			(*tables)[name] = append((*tables)[name], rec)
			mu.Unlock()

			return hash
		},
	}

	t["all"] = &object.Builtin{
		Fn: func(args ...object.Object) object.Object {
			mu.RLock()
			rows := (*tables)[name]
			mu.RUnlock()

			arr := make([]object.Object, len(rows))
			for i, r := range rows {
				pairs := make(map[string]object.Object)
				for k, v := range r {
					pairs[k] = &object.String{Value: fmt.Sprintf("%v", v)}
				}
				arr[i] = MakeHashObj(pairs)
			}
			return &object.Array{Elements: arr}
		},
	}

	t["count"] = &object.Builtin{
		Fn: func(args ...object.Object) object.Object {
			mu.RLock()
			cnt := len((*tables)[name])
			mu.RUnlock()
			return &object.Integer{Value: int64(cnt)}
		},
	}

	return MakeHashObj(t)
}

func createMoneyModule() *object.Hash {
	m := make(map[string]object.Object)

	m["ofMinor"] = &object.Builtin{
		Fn: func(args ...object.Object) object.Object {
			var minor int64
			curr := "BDT"
			if len(args) > 0 {
				if i, ok := args[0].(*object.Integer); ok {
					minor = i.Value
				}
			}
			if len(args) > 1 {
				curr = args[1].Inspect()
			}
			return createMoneyObj(minor, curr)
		},
	}

	m["ofMajor"] = &object.Builtin{
		Fn: func(args ...object.Object) object.Object {
			var major float64
			curr := "BDT"
			if len(args) > 0 {
				switch v := args[0].(type) {
				case *object.Float:
					major = v.Value
				case *object.Integer:
					major = float64(v.Value)
				}
			}
			if len(args) > 1 {
				curr = args[1].Inspect()
			}
			minor := int64(math.Round(major * 100))
			return createMoneyObj(minor, curr)
		},
	}

	m["add"] = &object.Builtin{
		Fn: func(args ...object.Object) object.Object {
			if len(args) < 2 {
				return newError("add requires two money objects")
			}
			m1 := getMinor(args[0])
			m2 := getMinor(args[1])
			return createMoneyObj(m1+m2, "BDT")
		},
	}

	m["sub"] = &object.Builtin{
		Fn: func(args ...object.Object) object.Object {
			if len(args) < 2 {
				return newError("sub requires two money objects")
			}
			m1 := getMinor(args[0])
			m2 := getMinor(args[1])
			return createMoneyObj(m1-m2, "BDT")
		},
	}

	m["mulQty"] = &object.Builtin{
		Fn: func(args ...object.Object) object.Object {
			if len(args) < 2 {
				return newError("mulQty requires money and quantity minor")
			}
			unit := getMinor(args[0])
			var qtyMinor int64 = 1000
			if i, ok := args[1].(*object.Integer); ok {
				qtyMinor = i.Value
			}
			var scale int64 = 1000
			if len(args) > 2 {
				if s, ok := args[2].(*object.Integer); ok && s.Value > 0 {
					scale = s.Value
				}
			}
			res := (unit * qtyMinor) / scale
			return createMoneyObj(res, "BDT")
		},
	}

	m["format"] = &object.Builtin{
		Fn: func(args ...object.Object) object.Object {
			if len(args) == 0 {
				return &object.String{Value: "৳0.00"}
			}
			minor := getMinor(args[0])
			sym := "৳"
			if len(args) > 1 {
				sym = args[1].Inspect()
			}
			return &object.String{Value: formatMoneyString(minor, sym)}
		},
	}

	return MakeHashObj(m)
}

func getMinor(obj object.Object) int64 {
	if h, ok := obj.(*object.Hash); ok {
		k := &object.String{Value: "minor"}
		if p, has := h.Pairs[k.HashKey()]; has {
			if i, isInt := p.Value.(*object.Integer); isInt {
				return i.Value
			}
		}
	} else if i, ok := obj.(*object.Integer); ok {
		return i.Value
	}
	return 0
}

func formatMoneyString(n int64, symbol string) string {
	neg := n < 0
	if neg {
		n = -n
	}
	maj := n / 100
	min := n % 100
	res := fmt.Sprintf("%s%d.%02d", symbol, maj, min)
	if neg {
		return "-" + res
	}
	return res
}

func createMoneyObj(minor int64, currency string) *object.Hash {
	sym := "৳"
	if currency == "USD" {
		sym = "$"
	}
	return MakeHashObj(map[string]object.Object{
		"minor":     &object.Integer{Value: minor},
		"currency":  &object.String{Value: currency},
		"symbol":    &object.String{Value: sym},
		"formatted": &object.String{Value: formatMoneyString(minor, sym)},
	})
}

func createRealtimeModule() *object.Hash {
	m := make(map[string]object.Object)

	m["hub"] = &object.Builtin{
		Fn: func(args ...object.Object) object.Object {
			hubMap := make(map[string]object.Object)
			hubMap["broadcast"] = &object.Builtin{
				Fn: func(args ...object.Object) object.Object {
					msg := ""
					if len(args) > 0 {
						msg = args[0].Inspect()
					}
					fmt.Printf("📡 [Realtime WebSocket Broadcast]: %s\n", msg)
					return TRUE
				},
			}
			hubMap["clientsCount"] = &object.Builtin{
				Fn: func(args ...object.Object) object.Object {
					return &object.Integer{Value: 1}
				},
			}
			return MakeHashObj(hubMap)
		},
	}

	return MakeHashObj(m)
}

func createSecurityModule() *object.Hash {
	m := make(map[string]object.Object)

	m["jwtSign"] = &object.Builtin{
		Fn: func(args ...object.Object) object.Object {
			if len(args) < 2 {
				return newError("jwtSign requires payload and secret")
			}
			secret := args[1].Inspect()

			header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
			payloadBytes, _ := json.Marshal(map[string]string{
				"sub": args[0].Inspect(),
				"iat": strconv.FormatInt(time.Now().Unix(), 10),
			})
			payload := base64.RawURLEncoding.EncodeToString(payloadBytes)

			sigData := header + "." + payload
			h := hmac.New(sha256.New, []byte(secret))
			h.Write([]byte(sigData))
			sig := base64.RawURLEncoding.EncodeToString(h.Sum(nil))

			token := sigData + "." + sig
			return &object.String{Value: token}
		},
	}

	m["csrfToken"] = &object.Builtin{
		Fn: func(args ...object.Object) object.Object {
			b := make([]byte, 16)
			_, _ = rand.Read(b)
			return &object.String{Value: hex.EncodeToString(b)}
		},
	}

	return MakeHashObj(m)
}
