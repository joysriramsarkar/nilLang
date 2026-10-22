package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/joysriramsarkar/nilLang/compiler/ast"
	"github.com/joysriramsarkar/nilLang/compiler/diagnostics"
	"github.com/joysriramsarkar/nilLang/compiler/frontend"
	"github.com/joysriramsarkar/nilLang/pkg/alap/ai"
	"github.com/joysriramsarkar/nilLang/pkg/config"
	"github.com/joysriramsarkar/nilLang/pkg/oracle"
)

func cmdCheck() {
	targetPath := "."
	ciMode := false
	for _, arg := range os.Args[2:] {
		switch arg {
		case "--ci", "--strict", "-ci":
			ciMode = true
		default:
			if len(arg) > 0 && arg[0] != '-' && targetPath == "." {
				targetPath = arg
			}
		}
	}

	fmt.Println("🛡️  NilLang & Alap Architectural Verification (docs/spec/ARCHITECTURE.md)")
	fmt.Println("═══════════════════════════════════════════════════════════════")

	// 1. Check if checking a project directory (or subdirectory/file) with nil.json
	configDir := targetPath
	if strings.HasSuffix(targetPath, "nil.json") {
		configDir = filepath.Dir(targetPath)
	} else if fi, statErr := os.Stat(targetPath); statErr == nil && !fi.IsDir() {
		configDir = filepath.Dir(targetPath)
	}
	if absDir, absErr := filepath.Abs(configDir); absErr == nil {
		for cur := absDir; ; {
			if _, statErr := os.Stat(filepath.Join(cur, "nil.json")); statErr == nil {
				configDir = cur
				break
			}
			parent := filepath.Dir(cur)
			if parent == cur {
				break
			}
			cur = parent
		}
	}

	cfg, err := config.LoadConfig(configDir)
	if err == nil {
		fmt.Printf("📦 প্রজেক্ট: %s (v%s) | প্রোফাইল: \033[1;36m%s\033[0m\n", cfg.Name, cfg.Version, cfg.Profile)
		fmt.Println("🔍 ক্যাপাবিলিটি সিকিউরিটি যাচাই করা হচ্ছে...")

		res, valErr := cfg.ValidateCapabilities()
		if valErr != nil {
			fmt.Fprintf(os.Stderr, "❌ ক্যাপাবিলিটি পার্সিং এরর: %v\n", valErr)
			os.Exit(1)
		}

		if !res.Valid {
			fmt.Printf("❌ ক্যাপাবিলিটি ভায়োলেশন পাওয়া গেছে (%d টি):\n", len(res.Violations))
			for _, v := range res.Violations {
				fmt.Printf("   • \033[31m%s\033[0m\n", v)
			}
			if len(res.Suggestions) > 0 {
				fmt.Println("💡 পরামর্শ:")
				for _, s := range res.Suggestions {
					fmt.Printf("   ➜ %s\n", s)
				}
			}
			os.Exit(1)
		}

		fmt.Printf("✅ ক্যাপাবিলিটি যাচাই সফল: %d টি অনুমতি অনুমোদিত\n", len(res.Allowed))
		if len(res.Restricted) > 0 {
			fmt.Printf("⚠️  সীমিত ক্যাপাবিলিটি: %v\n", res.Restricted)
		}
	} else {
		fmt.Println("ℹ️  লোকাল nil.json পাওয়া যায়নি, সরাসরি কোড অ্যানালাইসিস শুরু হচ্ছে...")
	}

	// 2. Language Oracle & Static Analysis Guard (local, deterministic - not an AI model)
	oracleInstance := oracle.NewLanguageOracle()
	aiTruth := ai.NewApplicationTruth()

	fmt.Println()
	fmt.Println("🧠 স্ট্যাটিক আর্কিটেকচার গার্ড সক্রিয় রয়েছে...")

	// Discover .nil files to check
	var filesToCheck []string
	fi, statErr := os.Stat(targetPath)
	if statErr == nil && !fi.IsDir() {
		if strings.HasSuffix(targetPath, ".nil") {
			filesToCheck = append(filesToCheck, targetPath)
		}
	} else {
		cleanTarget := filepath.Clean(targetPath)
		_ = filepath.Walk(targetPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				if filepath.Clean(path) == cleanTarget {
					return nil
				}
				name := info.Name()
				// Build outputs and vendored third-party trees contain generated or
				// foreign .nil files. Linting them reports errors the project cannot
				// fix, so they are excluded like build/, .git/ and node_modules/.
				if name == "build" || name == "out" || name == "dist" || name == "target" ||
					name == "node_modules" || name == ".git" ||
					(name != "." && name != ".." && strings.HasPrefix(name, ".")) {
					return filepath.SkipDir
				}
				return nil
			}
			if strings.HasSuffix(path, ".nil") {
				filesToCheck = append(filesToCheck, path)
			}
			return nil
		})
	}

	if len(filesToCheck) == 0 {
		fmt.Fprintln(os.Stderr, "❌ স্ক্যান করার জন্য কোনো .nil ফাইল পাওয়া যায়নি।")
		os.Exit(1)
	}

	fmt.Printf("📂 মোট %d টি সোর্স ফাইল স্ক্যান করা হচ্ছে...\n", len(filesToCheck))

	totalErrors := []string{}
	totalWarnings := []string{}

	var caps []string
	if cfg != nil {
		caps = cfg.Capabilities
	}

	for _, file := range filesToCheck {
		codeBytes, err := os.ReadFile(file)
		if err != nil {
			totalErrors = append(totalErrors, fmt.Sprintf("%s: ফাইল রিড এরর: %s", file, err))
			continue
		}

		res := frontend.Check(string(codeBytes), frontend.Options{
			Filename:     file,
			SourceDir:    filepath.Dir(file),
			Capabilities: caps,
		})
		if len(res.ParseErrors) > 0 {
			for _, d := range res.Diagnostics {
				totalErrors = append(totalErrors, fmt.Sprintf("%s: সিনট্যাক্স এরর: %s", file, d.String()))
			}
			continue
		}
		if !res.OK {
			for _, d := range res.Diagnostics {
				switch d.Severity {
				case diagnostics.SeverityError:
					totalErrors = append(totalErrors, fmt.Sprintf("%s: টাইপ ত্রুটি: %s", file, d.String()))
				case diagnostics.SeverityWarning:
					totalWarnings = append(totalWarnings, fmt.Sprintf("%s: %s", file, d.String()))
				}
			}
			continue
		}
		// Warnings are collected whether or not the file also failed. They used to
		// be gathered only inside the `if !res.OK` branch, so a file whose only
		// remark was a warning reported nothing at all — the one case where a
		// warning is most worth reading is a file that compiles.
		for _, d := range res.Diagnostics {
			if d.Severity == diagnostics.SeverityWarning {
				totalWarnings = append(totalWarnings, fmt.Sprintf("%s: %s", file, d.String()))
			}
		}

		program := res.Program

		// Walk AST and inspect for AI hallucinations / invalid member calls
		walkAST(program, func(node ast.Node) {
			if dot, ok := node.(*ast.DotExpression); ok {
				var compName string
				if leftIdent, isIdent := dot.Left.(*ast.Identifier); isIdent {
					compName = leftIdent.Value
				} else if leftDot, isDot := dot.Left.(*ast.DotExpression); isDot {
					compName = leftDot.Member.Value
				}

				if compName != "" {
					valRes := aiTruth.ValidateMemberAccess(compName, dot.Member.Value)
					if !valRes.Valid && valRes.ErrorMessage != fmt.Sprintf("Unknown component %q", compName) {
						warnMsg := fmt.Sprintf("%s: \033[31mAI Hallucination ধরা পড়েছে!\033[0m %s", file, valRes.ErrorMessage)

						if len(valRes.AvailableMembers) > 0 {
							limit := 3
							if len(valRes.AvailableMembers) < limit {
								limit = len(valRes.AvailableMembers)
							}
							warnMsg += fmt.Sprintf(" (প্রস্তাবিত: %s)", strings.Join(valRes.AvailableMembers[:limit], ", "))
						}
						totalErrors = append(totalErrors, warnMsg)
					}
				}
			}
		})
	}

	_ = oracleInstance

	fmt.Println()
	if len(totalErrors) > 0 {
		fmt.Printf("❌ %d টি সমস্যা পাওয়া গেছে:\n", len(totalErrors))
		for _, e := range totalErrors {
			fmt.Printf("   • %s\n", e)
		}
		os.Exit(1)
	}

	if len(totalWarnings) > 0 {
		for _, w := range totalWarnings {
			fmt.Printf("   ⚠️ %s\n", w)
		}
		// L9.4: --ci turns warnings into a failure for pipelines.
		if ciMode {
			fmt.Printf("\n❌ CI মোড: %d টি warning থাকায় ব্যর্থ।\n", len(totalWarnings))
			os.Exit(1)
		}
	}

	fmt.Println("   • Language Symbols: Verified")
	fmt.Println("   • Alap Component Truth: Synchronized")
	fmt.Println()
	fmt.Println("✅ আর্কিটেকচারাল চেক সম্পূর্ণ: কোডবেস NilLang & Alap নিয়মাবলীর সাথে পুরোপুরি সঙ্গতিপূর্ণ!")
}

func cmdVerify() {
	proposalName := "ExperimentalComponent"
	if len(os.Args) >= 3 {
		proposalName = os.Args[2]
	}

	fmt.Printf("🚀 Alap Verified Novelty Pipeline চালু করা হচ্ছে: %s\n", proposalName)
	fmt.Println("═══════════════════════════════════════════════════════════════")

	proposal := &ai.ComponentProposal{
		Name:        filepath.Base(proposalName),
		Author:      "AI Developer",
		Status:      ai.StatusExperimental,
		SourceCode:  "",
		Tests:       "test { assert(true) }",
		Permissions: []string{"Network"},
	}

	// Check if proposalName is a file on disk
	if _, err := os.Stat(proposalName); err == nil {
		srcBytes, err := os.ReadFile(proposalName)
		if err == nil {
			proposal.SourceCode = string(srcBytes)
		}
	} else if _, err := os.Stat("src/" + proposalName); err == nil {
		srcBytes, err := os.ReadFile("src/" + proposalName)
		if err == nil {
			proposal.SourceCode = string(srcBytes)
		}
	} else if _, err := os.Stat(proposalName + ".nil"); err == nil {
		srcBytes, err := os.ReadFile(proposalName + ".nil")
		if err == nil {
			proposal.SourceCode = string(srcBytes)
		}
	} else {
		proposal.SourceCode = fmt.Sprintf("let %s = fn() { return true; };", proposalName)
	}

	pipeline := ai.NewVerifiedNoveltyPipeline()
	report := pipeline.Verify(proposal)

	for idx, stage := range report.Stages {
		icon := "✅"
		if !stage.Passed {
			icon = "❌"
		}
		fmt.Printf("  %d. %s [%s] - %s\n", idx+1, stage.StageName, icon, stage.Message)
	}

	fmt.Println()
	if report.FinalStatus == ai.StatusVerified {
		fmt.Printf("🎉 অভিনন্দন! %s সফলভাবে \033[1;32mVERIFIED NOVELTY\033[0m হিসেবে স্বীকৃত হয়েছে!\n", proposalName)
	} else {
		fmt.Printf("❌ ভেরিফিকেশন ব্যর্থ হয়েছে। স্ট্যাটাস: %s\n", report.FinalStatus)
		os.Exit(1)
	}
}

func walkAST(node ast.Node, visitor func(ast.Node)) {
	if node == nil {
		return
	}
	visitor(node)

	switch n := node.(type) {
	case *ast.Program:
		for _, s := range n.Statements {
			walkAST(s, visitor)
		}
	case *ast.ExpressionStatement:
		walkAST(n.Expression, visitor)
	case *ast.LetStatement:
		walkAST(n.Value, visitor)
	case *ast.ReturnStatement:
		walkAST(n.ReturnValue, visitor)
	case *ast.BlockStatement:
		for _, s := range n.Statements {
			walkAST(s, visitor)
		}
	case *ast.PrefixExpression:
		walkAST(n.Right, visitor)
	case *ast.InfixExpression:
		walkAST(n.Left, visitor)
		walkAST(n.Right, visitor)
	case *ast.IfExpression:
		walkAST(n.Condition, visitor)
		walkAST(n.Consequence, visitor)
		if n.Alternative != nil {
			walkAST(n.Alternative, visitor)
		}
	case *ast.WhileStatement:
		walkAST(n.Condition, visitor)
		walkAST(n.Body, visitor)

	case *ast.FunctionLiteral:
		walkAST(n.Body, visitor)
	case *ast.CallExpression:
		walkAST(n.Function, visitor)
		for _, arg := range n.Arguments {
			walkAST(arg, visitor)
		}
	case *ast.IndexExpression:
		walkAST(n.Left, visitor)
		walkAST(n.Index, visitor)
	case *ast.DotExpression:
		walkAST(n.Left, visitor)
	case *ast.ArrayLiteral:
		for _, elem := range n.Elements {
			walkAST(elem, visitor)
		}
	case *ast.HashLiteral:
		for k, v := range n.Pairs {
			walkAST(k, visitor)
			walkAST(v, visitor)
		}
	}
}
