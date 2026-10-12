# Nilang VS Code Extension (`editors/vscode`)

LSP client + debug configuration for the Nilang language.

## Layout

| File | Purpose |
|---|---|
| `package.json` | Extension manifest: language, grammar, config, debugger |
| `extension.js` | Zero-dependency LSP client (`nil lsp`) + DAP passthrough (`nil dap`) |
| `language-configuration.json` | Brackets, comments, indentation |
| `syntaxes/nilang.tmLanguage.json` | Grammar — **symlink/copy** of the repo grammar (see below) |

## Grammar wiring

The TextMate grammar is owned by the repo at `syntaxes/nilang.tmLanguage.json`
(backend keywords like `for`/`struct`/`match`/`export` included). Wire it in:

```sh
# from editors/vscode/
mkdir -p syntaxes
cp ../../syntaxes/nilang.tmLanguage.json syntaxes/
```

CI check: the copy must be byte-identical to the repo grammar.

## Develop

```sh
node --check extension.js
node -e "require('./extension.js').__selftest()"
```

Then press F5 in VS Code (Extension Development Host) and open any `.nil` file.

## Settings

- `nilang.server.path` — path to the `nil` binary (default: `nil` on PATH).
- `nilang.server.args` — server args (default: `["lsp"]`).
