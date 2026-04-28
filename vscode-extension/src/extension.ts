import * as path from "path";
import * as fs from "fs";
import { workspace, ExtensionContext, window } from "vscode";
import {
  LanguageClient,
  LanguageClientOptions,
  ServerOptions,
  TransportKind,
} from "vscode-languageclient/node";

let client: LanguageClient;

export function activate(context: ExtensionContext) {
  // Find the binary — look next to the extension first, then PATH
  const binaryName = process.platform === "win32" ? "esi-lsp.exe" : "esi-lsp";
  const bundledPath = context.asAbsolutePath(path.join("..", binaryName));
  const serverPath = fs.existsSync(bundledPath) ? bundledPath : binaryName;

  // Tell vscode-languageclient how to launch our server
  const serverOptions: ServerOptions = {
    command: serverPath,
    transport: TransportKind.stdio,
  };

  // Tell the client which files to activate for
  const clientOptions: LanguageClientOptions = {
    documentSelector: [
      { scheme: "file", language: "html" },
      { scheme: "file", language: "esi" },
    ],
    synchronize: {
      fileEvents: workspace.createFileSystemWatcher("**/*.{html,esi}"),
    },
  };

  client = new LanguageClient(
    "akamai-esi-lsp",
    "Akamai ESI Language Server",
    serverOptions,
    clientOptions,
  );

  client.start();
  window.showInformationMessage("Akamai ESI language server started");
}

export function deactivate(): Thenable<void> | undefined {
  if (!client) {
    return undefined;
  }
  return client.stop();
}
