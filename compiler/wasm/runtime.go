package wasm

import (
	"fmt"
)

// GenerateBrowserHarness generates modern HTML and JS to load and execute the NilLang WASM module
func GenerateBrowserHarness(appName string, wasmFilename string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>%s - NilLang WebAssembly</title>
  <style>
    :root {
      --bg: #0d1117;
      --card-bg: #161b22;
      --border: #30363d;
      --accent: #58a6ff;
      --text: #c9d1d9;
      --success: #3fb950;
    }
    body {
      margin: 0;
      padding: 2rem;
      background: var(--bg);
      color: var(--text);
      font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif;
      display: flex;
      flex-direction: column;
      align-items: center;
      min-height: 100vh;
    }
    .container {
      max-width: 800px;
      width: 100%%;
      background: var(--card-bg);
      border: 1px solid var(--border);
      border-radius: 12px;
      padding: 2rem;
      box-shadow: 0 10px 30px rgba(0,0,0,0.5);
    }
    header {
      display: flex;
      align-items: center;
      gap: 1rem;
      border-bottom: 1px solid var(--border);
      padding-bottom: 1rem;
      margin-bottom: 1.5rem;
    }
    h1 {
      margin: 0;
      font-size: 1.5rem;
      color: #f0f6fc;
    }
    .badge {
      background: rgba(88, 166, 255, 0.15);
      color: var(--accent);
      padding: 0.25rem 0.6rem;
      border-radius: 20px;
      font-size: 0.8rem;
      font-weight: 600;
    }
    #console {
      background: #010409;
      border: 1px solid var(--border);
      border-radius: 8px;
      padding: 1rem;
      font-family: ui-monospace, SFMono-Regular, Consolas, monospace;
      font-size: 0.9rem;
      min-height: 200px;
      max-height: 400px;
      overflow-y: auto;
      white-space: pre-wrap;
      color: var(--text);
    }
    .status {
      display: flex;
      align-items: center;
      gap: 0.5rem;
      margin-top: 1rem;
      font-size: 0.85rem;
      color: #8b949e;
    }
    .dot {
      width: 8px;
      height: 8px;
      border-radius: 50%%;
      background: var(--success);
    }
  </style>
</head>
<body>
  <div class="container">
    <header>
      <h1>🌌 %s</h1>
      <span class="badge">NilLang WASM • Alap Web</span>
    </header>

    <div id="console">Initializing NilLang WebAssembly Runtime...\n</div>

    <div class="status">
      <span class="dot"></span>
      <span id="runtime-status">WASM Ready</span>
    </div>
  </div>

  <script>
    const term = document.getElementById("console");
    function log(msg) {
      term.textContent += msg + "\\n";
      term.scrollTop = term.scrollHeight;
    }

    const importObject = {
      env: {
        puts: (code) => {
          log("[stdout] " + code);
        },
        time: () => Date.now(),
        memory: new WebAssembly.Memory({ initial: 1 })
      }
    };

    fetch("%s")
      .then(response => {
        if (!response.ok) throw new Error("Could not load " + response.statusText);
        return response.arrayBuffer();
      })
      .then(bytes => WebAssembly.instantiate(bytes, importObject))
      .then(results => {
        log("✅ WASM module instantiated successfully!");
        if (results.instance.exports.main) {
          const res = results.instance.exports.main();
          log("Execution result: " + res);
        }
      })
      .catch(err => {
        log("❌ Execution error: " + err.message);
        document.getElementById("runtime-status").textContent = "Runtime Error";
      });
  </script>
</body>
</html>
`, appName, appName, wasmFilename)
}
