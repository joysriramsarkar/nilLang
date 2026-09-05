package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/joysriramsarkar/nilLang/compiler/hir"
	"github.com/joysriramsarkar/nilLang/compiler/lexer"
	"github.com/joysriramsarkar/nilLang/compiler/mir"
	"github.com/joysriramsarkar/nilLang/compiler/oracle"
	"github.com/joysriramsarkar/nilLang/compiler/parser"
)

func cmdOracle() {
	if len(os.Args) < 3 {
		printOracleUsage()
		return
	}

	o := oracle.NewCompilerOracle()
	sub := os.Args[2]

	switch sub {
	case "list-types":
		fmt.Println("📋 NilLang Registered Types:")
		for _, t := range o.ListTypes() {
			fmt.Printf("  • \033[1;36m%s\033[0m\n", t)
		}

	case "list-functions":
		fmt.Println("⚡ NilLang Known Functions & Builtins:")
		for _, fn := range o.ListFunctions() {
			effects := ""
			if len(fn.Effects) > 0 && !strings.Contains(fn.TypeSig, "[") {
				effects = fmt.Sprintf(" [%s]", strings.Join(fn.Effects, ", "))
			}
			fmt.Printf("  • \033[1;32m%s\033[0m: %s%s (Status: %s)\n", fn.Name, fn.TypeSig, effects, fn.State)
			if fn.Doc != "" {
				fmt.Printf("    \033[90m%s\033[0m\n", fn.Doc)
			}
		}

	case "inspect":
		if len(os.Args) < 4 {
			fmt.Println("ব্যবহার: nil oracle inspect <symbol-name>")
			return
		}
		name := os.Args[3]
		sym, found := o.FindSymbol(name)
		if !found {
			fmt.Printf("❌ Symbol %q not found in Language Oracle\n", name)
			return
		}
		fmt.Printf("🔍 Symbol: \033[1;34m%s\033[0m\n", sym.Name)
		fmt.Printf("  • Kind: %s\n", sym.Kind)
		fmt.Printf("  • Signature: %s\n", sym.TypeSig)
		fmt.Printf("  • State: \033[1;33m%s\033[0m\n", sym.State)
		if len(fnEffects(sym)) > 0 {
			fmt.Printf("  • Effects: %v\n", fnEffects(sym))
		}
		if sym.Doc != "" {
			fmt.Printf("  • Doc: %s\n", sym.Doc)
		}

	case "check":
		if len(os.Args) < 4 {
			fmt.Println("ব্যবহার: nil oracle check <expression>")
			return
		}
		expr := strings.Join(os.Args[3:], " ")
		res, err := o.CheckExpression(expr)
		if err != nil {
			fmt.Printf("❌ Evaluation error: %v\n", err)
			return
		}
		if res.Valid {
			fmt.Printf("✅ Expression is statically valid. (Inferred: %s)\n", res.InferredType)
		} else {
			fmt.Println("❌ Expression validation failed:")
			for _, d := range res.Diagnostics {
				fmt.Printf("  • %s\n", d)
			}
		}

	default:
		printOracleUsage()
	}
}

func fnEffects(s *oracle.Symbol) []string {
	return s.Effects
}

func printOracleUsage() {
	fmt.Println("🧠 NilLang AI Compiler Oracle")
	fmt.Println("ব্যবহার: nil oracle <সাবকমান্ড> [অপশন]")
	fmt.Println()
	fmt.Println("সাবকমান্ড:")
	fmt.Println("  list-types          সব পরিচিত প্রিমিটিভ ও যৌগিক টাইপ দেখুন")
	fmt.Println("  list-functions      সব পরিচিত ফাংশন ও বিল্ট-ইন মেম্বার দেখুন")
	fmt.Println("  inspect <symbol>    নির্দিষ্ট চিহ্নের মেটাডেটা ও ইফেক্ট পরীক্ষা করুন")
	fmt.Println("  check <expr>        একটি এক্সপ্রেশন AI হ্যালুসিনেশন ও টাইপ সেফটি চেক করুন")
}

func cmdHIR() {
	file := "main.nil"
	if len(os.Args) >= 3 {
		file = os.Args[2]
	}
	codeBytes, err := os.ReadFile(file)
	if err != nil {
		fmt.Printf("❌ ফাইল পড়তে সমস্যা: %v\n", err)
		return
	}

	l := lexer.New(string(codeBytes))
	p := parser.New(l)
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		fmt.Printf("❌ সিনট্যাক্স এরর: %s\n", strings.Join(p.Errors(), "\n"))
		return
	}

	lowerer := hir.NewLowerer()
	hProg := lowerer.LowerProgram(prog)
	opt := hir.NewOptimizer()
	optProg := opt.Optimize(hProg)

	fmt.Printf("🌳 High-Level Intermediate Representation (HIR) - %s\n", file)
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println(optProg.String())
}

func cmdMIR() {
	file := "main.nil"
	if len(os.Args) >= 3 {
		file = os.Args[2]
	}
	codeBytes, err := os.ReadFile(file)
	if err != nil {
		fmt.Printf("❌ ফাইল পড়তে সমস্যা: %v\n", err)
		return
	}

	l := lexer.New(string(codeBytes))
	p := parser.New(l)
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		fmt.Printf("❌ সিনট্যাক্স এরর: %s\n", strings.Join(p.Errors(), "\n"))
		return
	}

	lowerer := hir.NewLowerer()
	hProg := lowerer.LowerProgram(prog)
	opt := hir.NewOptimizer()
	optProg := opt.Optimize(hProg)

	mirLowerer := mir.NewLowerer()
	mirProg := mirLowerer.LowerHIR(optProg)

	fmt.Printf("⚙️  Mid-Level Intermediate Representation (MIR / CFG) - %s\n", file)
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println(mirProg.String())
}
