// Nilang VS Code extension: LSP client + debug adapter passthrough.
//
// Zero-dependency scaffold (plain Node, no TypeScript build step):
//  - starts `nil lsp` (configurable via nilang.server.path/args),
//  - registers `nilang` documents with the language server,
//  - forwards `nilang` debug sessions to `nil dap`.
//
// This file intentionally stays small: protocol work lives in the
// `nil` binary (pkg/lsp, pkg/dap), so the editor side is wiring only.

'use strict';

// NOTE: `vscode` and `vscode-languageclient` are provided by the Extension
// Host at runtime, so they are required lazily inside activate(). Keeping
// them out of the top level lets `node --check` and the selftest run on a
// plain machine with no dependencies installed.

let client = null;

/** @param {any} context */
function activate(context) {
  const vscode = require('vscode');
  const { LanguageClient } = require('vscode-languageclient/node');
  const config = vscode.workspace.getConfiguration('nilang');
  const serverPath = config.get('server.path', 'nil');
  const serverArgs = config.get('server.args', ['lsp']);

  const serverOptions = {
    command: serverPath,
    args: serverArgs,
    options: { cwd: vscode.workspace.workspaceFolders?.[0]?.uri.fsPath }
  };
  const clientOptions = {
    documentSelector: [{ scheme: 'file', language: 'nilang' }],
    synchronize: {
      fileEvents: vscode.workspace.createFileSystemWatcher('**/*.nil')
    }
  };

  client = new LanguageClient('nilang', 'Nilang Language Server', serverOptions, clientOptions);
  context.subscriptions.push(client.start());

  context.subscriptions.push(
    vscode.debug.registerDebugAdapterDescriptorFactory('nilang', {
      createDebugAdapterDescriptor(session) {
        const program = session.configuration.program || '${file}';
        const DebugAdapterExecutable = vscode.DebugAdapterExecutable;
        return new DebugAdapterExecutable(serverPath, ['dap', program]);
      }
    })
  );

  context.subscriptions.push(
    vscode.commands.registerCommand('nilang.showDoc', async () => {
      const editor = vscode.window.activeTextEditor;
      if (!editor) {
        return;
      }
      const word = editor.document.getText(editor.document.getWordRangeAtPosition(editor.selection.active));
      const term = await vscode.window.showInputBox({ value: word, prompt: 'nil doc <symbol>' });
      if (!term) {
        return;
      }
      const panel = vscode.window.createWebviewPanel('nilangDoc', `nil doc ${term}`, vscode.ViewColumn.Beside, {});
      panel.webview.html = `<html><body><p>Run <code>nil doc ${term}</code> in the terminal for the full reference.</p></body></html>`;
    })
  );

  // Smoke-test hook: `node -e "require('./extension.js').__selftest()"`.
  return { client: () => client };
}

function deactivate() {
  if (client) {
    return client.stop();
  }
  return undefined;
}

function __selftest() {
  for (const name of ['activate', 'deactivate']) {
    if (typeof module.exports[name] !== 'function') {
      throw new Error(`missing export: ${name}`);
    }
  }
  const pkg = require('./package.json');
  if (!pkg.contributes.languages.some((l) => l.id === 'nilang')) {
    throw new Error('language contribution missing');
  }
  return true;
}

module.exports = { activate, deactivate, __selftest };
