package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/joysriramsarkar/nilLang/pkg/config"
)

func cmdInit() {
	projectName := "my-nilang-app"
	if len(os.Args) > 2 {
		projectName = os.Args[2]
	}

	// Validate project name
	if !isValidProjectName(projectName) {
		fmt.Fprintf(os.Stderr, "❌ অবৈধ প্রজেক্ট নাম: %s\n", projectName)
		fmt.Println("নামে শুধু অক্ষর, সংখ্যা, এবং হাইফেন ব্যবহার করুন")
		os.Exit(1)
	}

	// Create project directory
	if err := os.MkdirAll(projectName, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "❌ ডিরেক্টরি তৈরি করতে সমস্যা: %s\n", err)
		os.Exit(1)
	}

	// Create subdirectories
	dirs := []string{"src", "resources", "build"}
	for _, dir := range dirs {
		path := filepath.Join(projectName, dir)
		if err := os.MkdirAll(path, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "❌ %s ডিরেক্টরি তৈরি করতে সমস্যা: %s\n", dir, err)
			os.Exit(1)
		}
	}

	// Create nil.json
	cfg := config.DefaultConfig(projectName)
	if err := cfg.Save(projectName); err != nil {
		fmt.Fprintf(os.Stderr, "❌ nil.json তৈরি করতে সমস্যা: %s\n", err)
		os.Exit(1)
	}

	// Create main.nil
	mainNil := generateMainNil(projectName)
	mainPath := filepath.Join(projectName, "src", "main.nil")
	if err := os.WriteFile(mainPath, []byte(mainNil), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "❌ main.nil তৈরি করতে সমস্যা: %s\n", err)
		os.Exit(1)
	}

	// Create .gitignore
	gitignore := `# Nilang Build Output
build/
*.nilax

# IDE
.vscode/
.idea/

# OS
.DS_Store
Thumbs.db
`
	gitignorePath := filepath.Join(projectName, ".gitignore")
	if err := os.WriteFile(gitignorePath, []byte(gitignore), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "⚠️ .gitignore তৈরি করতে সমস্যা: %s\n", err)
	}

	fmt.Println("✅ নীলাং প্রজেক্ট তৈরি হয়েছে!")
	fmt.Println()
	fmt.Printf("📁 %s/\n", projectName)
	fmt.Println("├── nil.json          # প্রজেক্ট কনফিগারেশন")
	fmt.Println("├── src/")
	fmt.Println("│   └── main.nil      # এন্ট্রি পয়েন্ট")
	fmt.Println("├── resources/        # অ্যাসেটস")
	fmt.Println("├── build/            # বিল্ড আউটপুট")
	fmt.Println("└── .gitignore")
	fmt.Println()
	fmt.Printf("🚀 শুরু করতে: cd %s && nil run\n", projectName)
}

func isValidProjectName(name string) bool {
	if len(name) == 0 {
		return false
	}
	for i, ch := range name {
		if ch == '-' || ch == '_' {
			continue
		}
		if ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || ch >= '0' && ch <= '9' {
			continue
		}
		if i == 0 && ch >= '0' && ch <= '9' {
			return false // Can't start with number
		}
		return false
	}
	return true
}

func generateMainNil(projectName string) string {
	title := strings.ReplaceAll(projectName, "-", " ")
	title = strings.ReplaceAll(title, "_", " ")

	return fmt.Sprintf(`// %s - Nilang Application
// Powered by Alap Framework & Onuron OS

let appName = "%s";
let version = "0.1.0";

puts("🚀 Welcome to " + appName + "!");
puts("Version: " + version);
puts("");

// Example: While loop
let i = 0;
let sum = 0;
while (i < 5) {
    let sum = sum + i;
    let i = i + 1;
}
puts("Sum of 0-4: " + sum);

// Example: String interpolation
let user = "Joyshriram";
puts("Hello, \(user)! Welcome to \(appName).");

// Example: Function
let greet = fn(name) {
    return "Hello, " + name + "!";
};
puts(greet("Onuron OS"));

puts("");
puts("✅ \(appName) is running successfully!");
`, title, projectName)
}