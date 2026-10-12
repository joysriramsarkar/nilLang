/* NilLang WebAssembly runtime bridge (nil_runtime.js).
 *
 * This file is the host half of a NilLang WASM module: the module imports
 * `env.puts`, exports linear `memory` and a `main` entry point, and this
 * script wires those to the DOM, a 2D canvas, timers and fetch. It has no
 * dependencies and tolerates both browsers and Node (for smoke tests):
 * every browser-only surface is feature-checked before use.
 *
 * Usage (see GenerateBrowserHarness in compiler/wasm/runtime.go):
 *
 *   <script src="nil_runtime.js"></script>
 *   <script>
 *     NilRuntime.mount("#nil-app");
 *     WebAssembly.instantiateStreaming(fetch("app.wasm"),
 *       NilRuntime.makeImportObject()).then(({instance}) => {
 *       NilRuntime.attach(instance);
 *       NilRuntime.log("result: " + instance.exports.main());
 *     });
 *   </script>
 */
(function (global) {
  "use strict";

  var decoder =
    typeof TextDecoder !== "undefined" ? new TextDecoder("utf-8") : null;

  var NilRuntime = {
    _instance: null,
    _memory: null,
    _app: null,
    _canvas: null,
    _ctx: null,
    _logLines: [],

    /* -- module plumbing ------------------------------------------- */

    // attach remembers the instantiated module and its linear memory.
    attach: function (instance) {
      this._instance = instance || null;
      var mem =
        instance && instance.exports && instance.exports.memory;
      this._memory = mem || null;
      return this;
    },

    // makeImportObject builds the import object a NilLang module expects:
    // env.puts for console output and env.time for the clock.
    makeImportObject: function () {
      var self = this;
      return {
        env: {
          puts: function (code) {
            self.puts(code);
          },
          nil_log: function (ptr, len) {
            self.log(self.readString(ptr, len));
          },
          time: function () {
            return Date.now();
          },
        },
      };
    },

    // readString decodes [ptr, ptr+len) from linear memory as UTF-8.
    readString: function (ptr, len) {
      if (!this._memory || !decoder) return "";
      var bytes = new Uint8Array(this._memory.buffer, ptr >>> 0, len >>> 0);
      try {
        return decoder.decode(bytes);
      } catch (e) {
        return "";
      }
    },

    /* -- logging ---------------------------------------------------- */

    // puts is the module's stdout: a numeric code today, a pointer when the
    // string ABI lands. It always reaches the developer console and, once
    // mounted, the on-page console as well.
    puts: function (code) {
      this.log("[stdout] " + code);
    },

    // log appends a line to the developer console and the mounted console.
    log: function (msg) {
      var line = String(msg);
      this._logLines.push(line);
      if (typeof console !== "undefined" && console.log) {
        console.log(line);
      }
      if (this._app) {
        var term = this._app.querySelector("[data-nil-console]");
        if (term) {
          term.textContent += line + "\n";
          term.scrollTop = term.scrollHeight;
        }
        var status = this._app.querySelector("[data-nil-status]");
        if (status) status.textContent = line;
      }
    },

    /* -- DOM -------------------------------------------------------- */

    // mount binds the runtime to a container (e.g. "#nil-app"). The
    // container keeps working unmounted: log falls back to the console.
    mount: function (selector) {
      if (typeof document === "undefined") return null;
      this._app = document.querySelector(selector || "#nil-app");
      return this._app;
    },

    // setText replaces the text of every element matching selector.
    setText: function (selector, text) {
      if (typeof document === "undefined") return 0;
      var nodes = document.querySelectorAll(selector);
      for (var i = 0; i < nodes.length; i++) {
        nodes[i].textContent = String(text);
      }
      return nodes.length;
    },

    // onEvent registers a DOM listener by selector and event name.
    onEvent: function (selector, event, handler) {
      if (typeof document === "undefined") return 0;
      var nodes = document.querySelectorAll(selector);
      for (var i = 0; i < nodes.length; i++) {
        nodes[i].addEventListener(event, handler);
      }
      return nodes.length;
    },

    /* -- canvas ----------------------------------------------------- */

    // bindCanvas selects a <canvas> for the 2D helpers below.
    bindCanvas: function (selector) {
      if (typeof document === "undefined") return false;
      this._canvas = document.querySelector(selector || "#nil-canvas");
      this._ctx =
        this._canvas && this._canvas.getContext
          ? this._canvas.getContext("2d")
          : null;
      return !!this._ctx;
    },

    clearCanvas: function (color) {
      if (!this._ctx || !this._canvas) return false;
      this._ctx.fillStyle = color || "#0d1117";
      this._ctx.fillRect(0, 0, this._canvas.width, this._canvas.height);
      return true;
    },

    drawRect: function (x, y, w, h, color) {
      if (!this._ctx) return false;
      this._ctx.fillStyle = color || "#58a6ff";
      this._ctx.fillRect(x, y, w, h);
      return true;
    },

    drawText: function (text, x, y, color, font) {
      if (!this._ctx) return false;
      this._ctx.fillStyle = color || "#c9d1d9";
      this._ctx.font = font || "14px sans-serif";
      this._ctx.fillText(String(text), x, y);
      return true;
    },

    /* -- timers ----------------------------------------------------- */

    // delay resolves after ms milliseconds (a setTimeout promise).
    delay: function (ms) {
      return new Promise(function (resolve) {
        setTimeout(resolve, ms);
      });
    },

    // wrapTimeout runs fn after ms milliseconds, catching synchronous
    // throws into a rejected promise rather than an uncaught error.
    wrapTimeout: function (fn, ms) {
      return new Promise(function (resolve, reject) {
        setTimeout(function () {
          try {
            resolve(fn());
          } catch (e) {
            reject(e);
          }
        }, ms);
      });
    },

    /* -- fetch ------------------------------------------------------ */

    // nilFetch GETs a URL and resolves with its text. It is a stub with
    // honest errors: without a fetch implementation (very old browsers) or
    // on a network failure the promise rejects with why.
    nilFetch: function (url, opts) {
      var fetchFn =
        typeof fetch !== "undefined"
          ? fetch
          : global && global.fetch
            ? global.fetch
            : null;
      if (!fetchFn) {
        return Promise.reject(new Error("nilFetch: no fetch implementation"));
      }
      return fetchFn(url, opts).then(function (resp) {
        if (!resp.ok) {
          throw new Error("nilFetch: HTTP " + resp.status + " for " + url);
        }
        return resp.text();
      });
    },
  };

  if (typeof module !== "undefined" && module.exports) {
    module.exports = NilRuntime;
  }
  global.NilRuntime = NilRuntime;
})(typeof globalThis !== "undefined" ? globalThis : this);
