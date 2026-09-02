package main

import (
	"fmt"
	"os"

	"github.com/joysriramsarkar/nilLang/pkg/alap"
)

func cmdRender() {
	fmt.Println("🎨 Alap UI Renderer - Onuron OS")
	fmt.Println()

	// Create a sample UI tree
	renderer := alap.NewRenderer(alap.OnuronTheme())

	root := alap.Column(
		alap.Text("Hello, Onuron OS! 🎉"),
		alap.Text("Powered by Nilang & Alap Framework"),
		alap.Row(
			alap.Button("Click Me", func() {
				fmt.Println("Button clicked!")
			}),
			alap.Button("Cancel", nil),
		),
		alap.Input("Enter your name..."),
		alap.Container(
			alap.Text("This is a container"),
			alap.Image("resources/icon.png"),
		),
	)

	renderer.SetRoot(root)

	// Render as ANSI
	fmt.Println("=== ANSI Render ===")
	fmt.Print(renderer.RenderToANSI())

	fmt.Println()
	fmt.Println("=== HTML Render ===")
	html := renderer.RenderToHTML()
	
	// Save HTML to file
	outputPath := "build/preview.html"
	os.MkdirAll("build", 0755)
	if err := os.WriteFile(outputPath, []byte(html), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "❌ HTML সেভ করতে সমস্যা: %s\n", err)
	} else {
		fmt.Printf("✅ HTML প্রিভিউ সেভ হয়েছে: %s\n", outputPath)
		fmt.Println("   ব্রাউজারে খুলুন: open " + outputPath)
	}
}