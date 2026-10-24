const vscode = require('vscode');
const { LanguageClient, TransportKind } = require('vscode-languageclient/node');

let client;

function activate(context) {
  const serverCommand = vscode.workspace.getConfiguration('axlang.server').get('path', 'axls');
  const serverOptions = {
    command: serverCommand,
    args: [],
    transport: TransportKind.stdio
  };
  const clientOptions = {
    documentSelector: [{ scheme: 'file', language: 'axlang' }],
    synchronize: { configurationSection: 'axlang' }
  };

  client = new LanguageClient('axlang', 'AxLang Language Server', serverOptions, clientOptions);
  context.subscriptions.push(client);
  client.start();
}

function deactivate() {
  return client ? client.stop() : undefined;
}

module.exports = { activate, deactivate };
