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
	selectedProfile := "os"

	for i := 2; i < len(os.Args); i++ {
		arg := os.Args[i]
		if arg == "--profile" && i+1 < len(os.Args) {
			selectedProfile = os.Args[i+1]
			i++
		} else if !strings.HasPrefix(arg, "-") && projectName == "my-nilang-app" {
			projectName = arg
		}
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
	cfg.Profile = selectedProfile
	switch selectedProfile {
	case "web":
		cfg.Capabilities = []string{"Network", "Crypto", "AI"}
		cfg.Targets = []string{"wasm", "web"}
	case "server":
		cfg.Capabilities = []string{"Network", "Database", "Filesystem", "Process", "Crypto"}
		cfg.Targets = []string{"linux", "server"}
	case "data":
		cfg.Capabilities = []string{"Filesystem", "Database", "GPU", "AI"}
		cfg.Targets = []string{"data", "linux"}
	case "mobile":
		cfg.Capabilities = []string{"Network", "Camera", "GPS", "Sensors", "Storage"}
		cfg.Targets = []string{"android", "ios"}
	default:
		cfg.Capabilities = []string{"Network", "Filesystem"}
		cfg.Targets = []string{"onuron", "linux"}
	}

	if err := cfg.Save(projectName); err != nil {
		fmt.Fprintf(os.Stderr, "❌ nil.json তৈরি করতে সমস্যা: %s\n", err)
		os.Exit(1)
	}

	// Create main.nil
	mainNil := generateMainNil(projectName, selectedProfile)
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
	if selectedProfile == "web" {
		fmt.Printf("🚀 শুরু করতে: cd %s && nil render (বা nil run)\n", projectName)
	} else {
		fmt.Printf("🚀 শুরু করতে: cd %s && nil run\n", projectName)
	}
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

func generateMainNil(projectName, profile string) string {
	title := strings.ReplaceAll(projectName, "-", " ")
	title = strings.ReplaceAll(title, "_", " ")

	switch profile {
	case "web":
		return fmt.Sprintf(`// %s - NilLang Web Application
// Powered by Alap UI Framework

let siteTitle = "%s";
let appName = "%s";

puts("🌐 Initializing Web Application: \(siteTitle)");

// Alap UI Web Layout
let page = ui.NewPage(siteTitle);

// 1. Navigation Bar
let nav = ui.NewNavigation(appName)
    .AddItem("Home", "/")
    .AddItem("Features", "/features")
    .AddItem("About", "/about");
page.SetNav(nav);

// 2. Metrics / Dashboard
let dash = ui.NewDashboard("Live Metrics")
    .AddMetric("Total Requests", "12,450", "+14%%")
    .AddMetric("Avg Latency", "12ms", "-8%%")
    .AddMetric("Uptime", "99.98%%", "+0.01%%");
page.Add(dash);

// 3. Data Table
let table = ui.NewTable("Service", "Profile", "Status")
    .AddRow("web-frontend", "WASM", "ONLINE")
    .AddRow("api-backend", "Server", "ACTIVE")
    .AddRow("data-pipeline", "Data", "HEALTHY");
page.Add(table);

// 4. Interactive Form
let form = ui.NewForm("Connect with Us")
    .AddField("Your Name", "name", "Enter your name...")
    .AddField("Work Email", "email", "you@example.com");
page.Add(form);

// 5. Footer
page.SetFooter("Built with NilLang & Alap Web Framework • Powered by Onuron OS");

puts("✅ %s web structure and UI ready for rendering!");
`, title, title, appName(projectName), title)

	case "server":
		return fmt.Sprintf(`// %s - Alap Server Microservice
// Profile: server

let serviceName = "%s";
let port = 8080;

puts("🚀 Starting Alap Microservice: \(serviceName)");
puts("📡 Listening on port: \(port)");

// Microservice routes & health status
puts("  • Endpoint registered: GET /health");
puts("  • Endpoint registered: GET /api/v1/status");
puts("  • Endpoint registered: POST /api/v1/data");

puts("✅ Microservice \(serviceName) is running and accepting requests!");
`, title, projectName)

	case "data":
		return fmt.Sprintf(`// %s - NilLang Data Science Pipeline
// Profile: data

let pipelineName = "%s";
let sampleCount = 5000;

puts("📊 Initializing Data Science Pipeline: \(pipelineName)");
puts("📥 Ingesting \(sampleCount) dataset observations...");
puts("⚙️  Applying normalization & feature engineering transforms...");
puts("🧠 Model training complete: Linear Regression [MSE: 0.038, R²: 0.985]");
puts("✅ Data Pipeline evaluation complete!");
`, title, projectName)

	default:
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
puts("Sum of 0-4: \(sum)");

// Example: String interpolation
let user = "Developer";
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
}

func appName(project string) string {
	parts := strings.Split(project, "-")
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, "")
}
