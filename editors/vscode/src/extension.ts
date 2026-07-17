import { spawnSync } from "child_process";
import * as vscode from "vscode";
import {
  LanguageClient,
  LanguageClientOptions,
  ServerOptions,
  TransportKind,
} from "vscode-languageclient/node";

let client: LanguageClient | undefined;

export function activate(context: vscode.ExtensionContext): void {
  context.subscriptions.push(
    vscode.commands.registerCommand("pypls.restart", async () => {
      await stopClient();
      startClient(context);
    })
  );

  startClient(context);
}

export function deactivate(): Thenable<void> | undefined {
  return client ? client.stop() : undefined;
}

function startClient(context: vscode.ExtensionContext): void {
  const command = resolveCommand();
  if (!checkInstalled(command)) {
    return;
  }

  const serverOptions: ServerOptions = {
    run: { command, args: ["lsp"], transport: TransportKind.stdio },
    debug: { command, args: ["lsp"], transport: TransportKind.stdio },
  };

  const clientOptions: LanguageClientOptions = {
    documentSelector: [{ scheme: "file", language: "python" }],
  };

  client = new LanguageClient("pypls", "pypls", serverOptions, clientOptions);
  client.start();
  context.subscriptions.push(client);
}

async function stopClient(): Promise<void> {
  if (client) {
    await client.stop();
    client = undefined;
  }
}

function resolveCommand(): string {
  const configured = vscode.workspace
    .getConfiguration("pypls")
    .get<string>("path");
  return configured && configured.trim().length > 0 ? configured : "pypls";
}

// checkInstalled runs "<command> version" once. If it fails, the executable is
// not on the PATH, so we tell the user how to install it instead of leaving the
// server silently dead.
function checkInstalled(command: string): boolean {
  try {
    const result = spawnSync(command, ["version"], { timeout: 5000 });
    if (result.status === 0) {
      return true;
    }
  } catch {
    // fall through to the message below
  }

  vscode.window
    .showErrorMessage(
      `pypls was not found (tried "${command}"). Install it, then restart the language server.`,
      "Copy install command"
    )
    .then((choice) => {
      if (choice === "Copy install command") {
        vscode.env.clipboard.writeText("pip install pypls-client");
      }
    });
  return false;
}
