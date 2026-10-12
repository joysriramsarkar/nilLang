package wasm

import (
	_ "embed"
	"fmt"
)

//go:embed nil_runtime.js
var nilRuntimeJS string

// NilRuntimeJS returns the bundled WebAssembly runtime bridge (L7.7): DOM
// binding, canvas helpers, setTimeout wrappers, fetch stubs and puts/log.
// `nil build wasm` writes it beside index.html so the harness can load it
// with a plain script tag and no bundler.
func NilRuntimeJS() string {
	return nilRuntimeJS
}

// GenerateBrowserHarness generates modern HTML and JS to load and execute the NilLang WASM module
//
// The page mounts the runtime bridge (nil_runtime.js, L7.7) into a #nil-app
// container (L7.8): the bridge owns the import object, the on-page console
// and the status line, and the bootstrap below only instantiates the module
// and calls its main export.
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
  <div class="container" id="nil-app" data-nil-app="%s">
    <header>
      <h1>🌌 %s</h1>
      <span class="badge">NilLang WASM • Alap Web</span>
    </header>

    <div id="console" data-nil-console>Initializing Nilang WebAssembly Runtime...\n</div>
    <canvas id="nil-canvas" width="640" height="360" style="width:100%%;border:1px solid var(--border);border-radius:8px;margin-top:1rem;"></canvas>

    <div class="status">
      <span class="dot"></span>
      <span id="runtime-status" data-nil-status>WASM Ready</span>
    </div>
  </div>

  <script src="nil_runtime.js"></script>
  <script>
    (function () {
      var appName = %q;
      var wasmFile = %q;

      function setStatus(msg) {
        var el = document.getElementById("runtime-status");
        if (el) el.textContent = msg;
      }

      if (typeof NilRuntime === "undefined") {
        setStatus("nil_runtime.js missing: cannot start " + appName);
        return;
      }

      NilRuntime.mount("#nil-app");
      NilRuntime.bindCanvas("#nil-canvas");

      var importObject = NilRuntime.makeImportObject();

      function instantiate(bytes) {
        return WebAssembly.instantiate(bytes, importObject);
      }

      function loadBytes() {
        if (WebAssembly.instantiateStreaming) {
          return fetch(wasmFile).then(function (response) {
            if (!response.ok) throw new Error("Could not load " + response.statusText);
            return WebAssembly.instantiateStreaming(response, importObject).then(function (results) {
              return results;
            });
          }).catch(function () {
            return fetch(wasmFile)
              .then(function (response) {
                if (!response.ok) throw new Error("Could not load " + response.statusText);
                return response.arrayBuffer();
              })
              .then(instantiate);
          });
        }
        return fetch(wasmFile)
          .then(function (response) {
            if (!response.ok) throw new Error("Could not load " + response.statusText);
            return response.arrayBuffer();
          })
          .then(instantiate);
      }

      loadBytes()
        .then(function (results) {
          NilRuntime.attach(results.instance);
          NilRuntime.log("WASM module instantiated successfully!");
          if (results.instance.exports.main) {
            var res = results.instance.exports.main();
            NilRuntime.log("Execution result: " + res);
            setStatus("Ran OK: " + res);
          } else {
            setStatus("Module has no main export");
          }
        })
        .catch(function (err) {
          NilRuntime.log("Execution error: " + (err && err.message));
          setStatus("Runtime Error");
        });
    })();
  </script>
</body>
</html>
`, appName, appName, appName, appName, wasmFilename)
}
