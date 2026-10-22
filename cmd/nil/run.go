package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/joysriramsarkar/nilLang/compiler/ast"
	"github.com/joysriramsarkar/nilLang/compiler/capability"
	"github.com/joysriramsarkar/nilLang/compiler/compiler"
	"github.com/joysriramsarkar/nilLang/compiler/evaluator"
	"github.com/joysriramsarkar/nilLang/compiler/frontend"
	"github.com/joysriramsarkar/nilLang/compiler/object"
	"github.com/joysriramsarkar/nilLang/compiler/vm"
	"github.com/joysriramsarkar/nilLang/pkg/bundle"
	pkgcompiler "github.com/joysriramsarkar/nilLang/pkg/compiler"
	"github.com/joysriramsarkar/nilLang/pkg/config"
)

func runProjectScript(scriptName string) bool {
	projectDir, err := os.Getwd()
	if err != nil {
		return false
	}
	cfg, err := config.LoadConfig(projectDir)
	if err != nil || cfg.Scripts == nil {
		return false
	}
	cmdLine, ok := cfg.Scripts[scriptName]
	if !ok {
		return false
	}
	runShellCommand(cmdLine)
	return true
}

func runShellCommand(cmdLine string) {
	fmt.Printf("➜ নীলাং স্ক্রিপ্ট: %s\n", cmdLine)
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/C", cmdLine)
	} else {
		cmd = exec.Command("sh", "-c", cmdLine)
	}
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		os.Exit(1)
	}
}

func cmdRun() {
	useVM := false
	useMir := false
	var targetFile string
	var scriptName string

	for _, arg := range os.Args[2:] {
		if arg == "-vm" || arg == "--vm" {
			useVM = true
		} else if arg == "--backend=mir" {
			// The canonical IR path: AST -> HIR -> MIR -> bytecode, the same
			// route WebAssembly takes. `L3.4` is about making this the only
			// backend path; it is selectable now so the two can be compared on any
			// program rather than only on the twenty-two the conformance test
			// covers.
			useMir = true
		} else if strings.HasSuffix(arg, ".nil") || strings.HasSuffix(arg, ".nilax") {
			targetFile = arg
		} else if scriptName == "" && !strings.HasPrefix(arg, "-") {
			scriptName = arg
		}
	}

	// Case 1: Direct file execution
	if targetFile != "" {
		// Capabilities come from the nearest nil.json, searching upward from
		// the file being run so that a script inside a project inherits the
		// project's grants.
		runDirectFile(targetFile, useVM, declaredCapabilitiesFor(targetFile), useMir)
		return
	}

	// Case 2: Project directory execution
	projectDir, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ কারেন্ট ডিরেক্টরি পেতে সমস্যা: %s\n", err)
		os.Exit(1)
	}

	if scriptName != "" {
		if fi, statErr := os.Stat(scriptName); statErr == nil && fi.IsDir() {
			candidate := filepath.Join(scriptName, "nil.json")
			if _, jsonErr := os.Stat(candidate); jsonErr == nil {
				projectDir = scriptName
				scriptName = ""
			}
		}
	}

	cfg, err := config.LoadConfig(projectDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ %s\n", err)
		os.Exit(1)
	}

	// Check if scriptName matches a script in nil.json
	if scriptName != "" && cfg.Scripts != nil {
		if cmdStr, ok := cfg.Scripts[scriptName]; ok {
			runShellCommand(cmdStr)
			return
		}
	}

	entryPath := cfg.GetEntryPath(projectDir)
	runDirectFile(entryPath, useVM, cfg.Capabilities, useMir)
}

// declaredCapabilitiesFor finds the nearest nil.json at or above a file and
// returns its declared capabilities.
//
// A missing manifest is not an error: a standalone script has no manifest, so it
// runs with no grants. Under the VM path that means any native call is refused,
// which is the correct default for an undeclared program rather than silently
// granting everything.
func declaredCapabilitiesFor(filePath string) []string {
	abs, err := filepath.Abs(filePath)
	if err != nil {
		abs = filePath
	}

	dir := filepath.Dir(abs)
	for {
		cfg, cfgErr := config.LoadConfig(dir)
		if cfgErr == nil && cfg != nil && len(cfg.Capabilities) > 0 {
			return cfg.Capabilities
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return nil
		}
		dir = parent
	}
}

func runDirectFile(filePath string, useVM bool, granted []string, useMir bool) {
	if strings.HasSuffix(filePath, ".nilax") {
		runBundleFile(filePath, useVM, granted, useMir)
		return
	}

	absPath, err := filepath.Abs(filePath)
	sourceDir := ""
	if err == nil {
		sourceDir = filepath.Dir(absPath)
		evaluator.PushScriptDir(sourceDir)
		defer evaluator.PopScriptDir()
	}

	source, err := os.ReadFile(filePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ ফাইল পড়তে সমস্যা: %s\n", err)
		os.Exit(1)
	}

	executeSource(string(source), sourceDir, useVM, granted, useMir)
}

func runBundleFile(bundlePath string, useVM bool, granted []string, useMir bool) {
	r, err := bundle.OpenBundle(bundlePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ .nilax বান্ডিল খুলতে সমস্যা: %s\n", err)
		os.Exit(1)
	}
	defer r.Close()

	manifest := r.GetManifest()
	fmt.Printf("📦 নির্বাহ হচ্ছে: %s v%s\n", manifest.AppName, manifest.AppVersion)

	// Prefer the compiled NABC bytecode: it is fully self-contained
	// (file-module imports are inlined at compile time), so the bundle
	// runs on any host without the original source tree.
	if bcBytes, err := r.GetBytecode(); err == nil && len(bcBytes) > 0 {
		bytecode, err := pkgcompiler.DecodeBytecode(bcBytes)
		if err != nil {
			fmt.Fprintf(os.Stderr, "❌ অবৈধ NABC বাইটকোড: %s\n", err)
			os.Exit(1)
		}
		machine := vm.New(bytecode)
		machine.CapabilityChecker = capability.GrantChecker(granted)
		if err := machine.Run(); err != nil {
			if denied, isDenied := capability.AsDenied(err); isDenied {
				fmt.Fprintf(os.Stderr, "❌ %s: %s\n", denied.DiagnosticCode(), denied.Error())
				fmt.Fprintf(os.Stderr, "   = help: %s\n", denied.Suggestion())
				os.Exit(1)
			}
			fmt.Fprintf(os.Stderr, "❌ রানটাইম ত্রুটি: %s\n", err)
			os.Exit(1)
		}
		return
	}

	// Fallback: execute embedded source (requires the source tree's
	// relative imports to resolve from the bundle directory).
	if srcBytes, err := r.GetFile("src/main.nil"); err == nil {
		bundleDir := ""
		if abs, err := filepath.Abs(bundlePath); err == nil {
			bundleDir = filepath.Dir(abs)
		}
		executeSource(string(srcBytes), bundleDir, useVM, granted, useMir)
		return
	}

	fmt.Fprintf(os.Stderr, "❌ বান্ডিলে কোনো এক্সিকিউটেবল কোড পাওয়া যায়নি\n")
	os.Exit(1)
}

// executeSource parses, type-checks, and executes Nilang source.
//
// All source execution paths route through here so that frontend.Check is the
// single enforcement point for the front end. granted carries the capabilities
// declared in the nearest nil.json; the VM path enforces them at every native
// call site, so a program that reaches for the filesystem without declaring it
// is refused rather than trusted.
func executeSource(source string, sourceDir string, useVM bool, granted []string, useMir bool) {
	result := frontend.Check(source, frontend.Options{
		SourceDir: sourceDir,

		// The grants go to the front end as well as the VM. A `requires` clause
		// is a static claim, and a checker that cannot see the manifest reports
		// every such function as ungranted whatever nil.json says — which made
		// `fn f() requires "filesystem"` unusable while the same capability
		// passed silently through the VM path.
		Capabilities: granted,
	})

	if len(result.ParseErrors) > 0 {
		fmt.Fprintf(os.Stderr, "❌ সিনট্যাক্স ত্রুটি:\n")
		for _, e := range result.ParseErrors {
			fmt.Fprintf(os.Stderr, "   %s\n", e)
		}
		os.Exit(1)
	}

	if !result.OK {
		fmt.Fprintln(os.Stderr, "❌ টাইপ ত্রুটি:")
		for _, d := range result.Diagnostics {
			fmt.Fprintf(os.Stderr, "   %s\n", d.String())
		}
		os.Exit(1)
	}

	// Warnings are printed on the way through, not on failure.
	//
	// They only ever reach a developer who is reading the output, and a warning
	// that is stored but never shown is indistinguishable from one that was never
	// produced. The program continues either way: the severity is the whole
	// meaning of the word.
	printWarnings(result.Diagnostics)

	program := result.Lowered

	// ── Backend: VM or Tree-Walking Evaluator ────────────────────────────────
	if useVM {
		// The canonical IR path compiles through HIR and MIR, the same route
		// WebAssembly takes. It is selectable rather than default because
		// `TestThreeEnginesAgree` covers the language's core and not yet its
		// modules, components or capabilities; promoting it is the remaining half
		// of `L3.4`, and the gate is full conformance rather than a flag.
		var bytecode *compiler.Bytecode
		if useMir {
			mirBytecode, err := pkgcompiler.CompileViaMir(program)
			if err != nil {
				fmt.Fprintf(os.Stderr, "❌ MIR কম্পাইলেশন ত্রুটি: %s\n", err)
				os.Exit(1)
			}
			bytecode = mirBytecode
		} else {
			comp := compiler.New()
			pkgcompiler.ConfigureCompiler(comp, sourceDir)
			if err := comp.Compile(program); err != nil {
				fmt.Fprintf(os.Stderr, "❌ কম্পাইলেশন ত্রুটি: %s\n", err)
				os.Exit(1)
			}
			bytecode = comp.Bytecode()
		}
		machine := vm.New(bytecode)
		machine.CapabilityChecker = capability.GrantChecker(granted)
		if err := machine.Run(); err != nil {
			if denied, isDenied := capability.AsDenied(err); isDenied {
				fmt.Fprintf(os.Stderr, "❌ %s: %s\n", denied.DiagnosticCode(), denied.Error())
				fmt.Fprintf(os.Stderr, "   = help: %s\n", denied.Suggestion())
				os.Exit(1)
			}
			fmt.Fprintf(os.Stderr, "❌ রানটাইম ত্রুটি: %s\n", err)
			os.Exit(1)
		}
		return
	}

	// Tree-walking evaluator
	//
	// The evaluator has no capability gate: builtins are Go function values
	// dispatched inside a switch, with no call site to intercept. It therefore
	// requires every declared capability to be present, which is checked before
	// evaluation rather than during. A program needing only pure operations runs
	// under a manifest that declares nothing. See docs/spec/CAPABILITIES.md and
	// the master plan item L6 for closing the evaluator side properly.
	if err := requireDeclaredCapabilities(program, granted); err != nil {
		if denied, isDenied := capability.AsDenied(err); isDenied {
			fmt.Fprintf(os.Stderr, "❌ %s: %s\n", denied.DiagnosticCode(), denied.Error())
			fmt.Fprintf(os.Stderr, "   = help: %s\n", denied.Suggestion())
		} else {
			fmt.Fprintf(os.Stderr, "❌ %s: %s\n", capability.Code, err.Error())
		}
		os.Exit(1)
	}

	env := object.NewEnvironment()
	registerBuiltins(env)

	// The evaluator enforces the same manifest at the call site that the VM
	// does: a denied builtin fails with CategoryCapabilityDenied rather than
	// running. The conservative pre-scan above stays as the early refusal
	// with the `--vm` hint; this is the enforcement underneath it.
	previousChecker := evaluator.GetCapabilityChecker()
	evaluator.SetCapabilityChecker(capability.GrantChecker(granted))
	defer func() { evaluator.SetCapabilityChecker(previousChecker) }()

	result2 := evaluator.Eval(program, env)
	if result2 != nil {
		if result2.Type() == object.ERROR_OBJ {
			fmt.Fprintf(os.Stderr, "❌ রানটাইম ত্রুটি: %s\n", result2.Inspect())
			os.Exit(1)
		}
	}
}

func registerBuiltins(env *object.Environment) {
	for name, builtin := range evaluator.Builtins {
		env.Set(name, builtin)
	}
}

// requireDeclaredCapabilities reports an error when a program reaches the
// ungated evaluator path without having declared the capabilities it needs.
//
// The check is conservative by construction: it inspects the program's source
// for calls to gated operations and requires each one's capability to appear in
// the manifest. It cannot catch a call made indirectly, through a function value
// or a module, which is precisely why the VM path is the one that enforces at
// the call site.
func requireDeclaredCapabilities(program *ast.Program, granted []string) error {
	if program == nil {
		return nil
	}

	set := make(map[string]bool)
	for _, name := range capability.NormalizeList(granted) {
		set[name] = true
	}

	used := capabilitiesUsedInProgram(program)
	for _, op := range used {
		req, known := capability.RequirementsFor(op)
		if !known || req.Grants(set) {
			continue
		}
		return &capability.DeniedError{
			Operation: op,
			Required:  req,
			Granted:   capability.NormalizeList(granted),
			Reason:    "capability not granted",
		}
	}
	return nil
}

// capabilitiesUsedInProgram walks an AST and collects the names of gated
// operations it calls directly.
func capabilitiesUsedInProgram(program *ast.Program) []string {
	var used []string
	seen := map[string]bool{}

	ast.Inspect(program, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpression)
		if !ok {
			return true
		}
		ident, ok := call.Function.(*ast.Identifier)
		if !ok {
			return true
		}
		if _, known := capability.RequirementsFor(ident.Value); known && !seen[ident.Value] {
			seen[ident.Value] = true
			used = append(used, ident.Value)
		}
		return true
	})

	return used
}
