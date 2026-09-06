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

	if selectedProfile == "web" {
		// Create public directory
		pubDir := filepath.Join(projectName, "public")
		_ = os.MkdirAll(pubDir, 0755)

		// Create alap.yaml
		alapYaml := fmt.Sprintf(`app:
  name: "%s"
  version: "1.0.0"
  target: "web"

server:
  host: "0.0.0.0"
  port: 8080
  cors:
    allowed_origins: ["*"]

database:
  driver: "sqlite"
  dsn: "app.db"
`, projectName)
		_ = os.WriteFile(filepath.Join(projectName, "alap.yaml"), []byte(alapYaml), 0644)

		// Create public/index.html
		pubHTML := fmt.Sprintf(`<!DOCTYPE html>
<html lang="bn">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>%s | Nilang Web App</title>
  <link rel="stylesheet" href="/style.css">
</head>
<body>
  <div class="container">
    <div class="badge">⚡ Powered by Nilang & Alap Web</div>
    <h1>স্বাগতম — %s</h1>
    <p class="subtitle">টাইপ-সেফ, মেমরি-সেফ এবং উচ্চ গতির ফুলস্ট্যাক ওয়েব অ্যাপ্লিকেশন।</p>
    <div class="card">
      <h2>🚀 প্রস্তুত ফিচারসমূহ</h2>
      <ul>
        <li>⚡ Radix Tree $O(k)$ রাউটার</li>
        <li>🛡️ বিল্ট-ইন XSS ও CSRF প্রোটেকশন</li>
        <li>🔄 রিয়্যাক্টিভ স্টেট ও সিগন্যাল ইঞ্জিন</li>
        <li>🗄️ এন্টারপ্রাইজ টাইপ-সেফ ORM</li>
      </ul>
    </div>
  </div>
</body>
</html>`, projectName, projectName)
		_ = os.WriteFile(filepath.Join(pubDir, "index.html"), []byte(pubHTML), 0644)

		// Create public/style.css
		pubCSS := `* { box-sizing: border-box; margin: 0; padding: 0; }
body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; background: #090a0f; color: #f1f5f9; min-height: 100vh; display: flex; align-items: center; justify-content: center; }
.container { max-width: 640px; padding: 32px; text-align: center; }
.badge { display: inline-block; background: rgba(0, 212, 255, 0.15); color: #00d4ff; padding: 6px 16px; border-radius: 9999px; font-weight: 600; font-size: 0.85rem; margin-bottom: 20px; border: 1px solid rgba(0, 212, 255, 0.3); }
h1 { font-size: 2.2rem; margin-bottom: 12px; background: linear-gradient(135deg, #fff, #94a3b8); -webkit-background-clip: text; -webkit-text-fill-color: transparent; }
.subtitle { color: #94a3b8; font-size: 1.1rem; line-height: 1.6; margin-bottom: 28px; }
.card { background: rgba(255, 255, 255, 0.03); border: 1px solid rgba(255, 255, 255, 0.08); border-radius: 16px; padding: 24px; text-align: left; }
.card h2 { font-size: 1.2rem; color: #38bdf8; margin-bottom: 16px; }
.card ul { list-style: none; display: flex; flex-direction: column; gap: 10px; }
.card li { font-size: 0.95rem; color: #cbd5e1; }
`
		_ = os.WriteFile(filepath.Join(pubDir, "style.css"), []byte(pubCSS), 0644)
	}

	fmt.Println("✅ নীলাং প্রজেক্ট তৈরি হয়েছে!")
	fmt.Println()
	fmt.Printf("📁 %s/\n", projectName)
	if selectedProfile == "web" {
		fmt.Println("├── alap.yaml         # Alap ওয়েব কনফিগারেশন")
	}
	fmt.Println("├── nil.json          # প্রজেক্ট কনফিগারেশন")
	fmt.Println("├── src/")
	fmt.Println("│   └── main.nil      # এন্ট্রি পয়েন্ট")
	if selectedProfile == "web" {
		fmt.Println("├── public/")
		fmt.Println("│   ├── index.html    # ওয়েব ল্যান্ডিং পেজ")
		fmt.Println("│   └── style.css     # স্টাইলশিট")
	}
	fmt.Println("├── resources/        # অ্যাসেটস")
	fmt.Println("├── build/            # বিল্ড আউটপুট")
	fmt.Println("└── .gitignore")
	fmt.Println()
	if selectedProfile == "web" {
		fmt.Println("🚀 শুরু করতে রান করুন:")
		fmt.Printf("   cd %s\n", projectName)
		fmt.Println("   nil dev --port 8080")
	} else {
		fmt.Println("🚀 শুরু করতে রান করুন:")
		fmt.Printf("   cd %s\n", projectName)
		fmt.Println("   nil run")
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
