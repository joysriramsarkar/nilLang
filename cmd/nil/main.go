package main

import (
	"fmt"
	"os"
	"strings"
)

const (
	VERSION = "0.1.0"
	BANNER  = `
╔══════════════════════════════════════════════════╗
║                                                  ║
║   ███╗   ██╗██╗██╗     █████╗ ███╗   ██╗ ███╗  ║
║   ████╗  ██║██║██║    ██╔══██╗████╗  ██║██╔══╝  ║
║   ██╔██╗ ██║██║██║    ███████║██╔██╗ ██║██║  ███║
║   ██║╚██╗██║██║██║    ██╔══██║██║╚██╗██║██║   ║
║   ██║ ╚████║██║██████╗██║  ██║██║ ╚████║╚██████║
║   ╚═╝  ╚═══╝╚═╝╚═════╝╚═╝  ╚═╝╚═╝  ╚═══╝ ╚═════╝
║                                                  ║
║   Nilang Compiler & Build System                 ║
║   Powered by Alap Framework • Onuron OS          ║
║                                                  ║
╚══════════════════════════════════════════════════╝
`
)

func main() {
	if len(os.Args) < 2 {
		cmdRepl()
		return
	}

	command := os.Args[1]

	// If argument ends in .nil, run it directly
	if strings.HasSuffix(command, ".nil") {
		runDirectFile(command, false)
		return
	}

	switch command {
	case "build":
		cmdBuild()
	case "run":
		cmdRun()
	case "init":
		cmdInit()
	case "fmt":
		cmdFmt()
	case "clean":
		cmdClean()
	case "render":
		cmdRender()
	case "repl":
		cmdRepl()
	case "version", "-v", "--version":
		fmt.Printf("Nilang Compiler v%s\n", VERSION)
		fmt.Println("Alap Framework • Onuron OS")
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "❌ অজানা কমান্ড: %s\n\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Print(BANNER)
	fmt.Printf("Nilang Compiler v%s\n\n", VERSION)
	fmt.Println("ব্যবহার: nil <কমান্ড> [অপশন]")
	fmt.Println()
	fmt.Println("কমান্ড:")
	fmt.Println("  init [name]        নতুন নীলাং প্রজেক্ট তৈরি করুন")
	fmt.Println("  build              প্রজেক্ট বিল্ড করে .nilax বান্ডিল তৈরি করুন")
	fmt.Println("  run [file] [-vm]   প্রজেক্ট বা .nil ফাইল রান করুন")
	fmt.Println("  repl               ইন্টারঅ্যাক্টিভ REPL চালু করুন")
	fmt.Println("  render [file.nil]  ডিক্লারেটিভ UI কম্পোনেন্ট রেন্ডার করুন")
	fmt.Println("  fmt                কোড ফরম্যাট করুন")
	fmt.Println("  clean              বিল্ড আর্টিফ্যাক্ট মুছে ফেলুন")
	fmt.Println("  version            ভার্সন তথ্য দেখুন")
	fmt.Println("  help               এই সাহায্য বার্তা দেখুন")
	fmt.Println()
	fmt.Println("উদাহরণ:")
	fmt.Println("  nil init my-app")
	fmt.Println("  nil run app.nil")
	fmt.Println("  nil run app.nil -vm")
	fmt.Println("  nil build")
	fmt.Println("  nil render component.nil")
}
