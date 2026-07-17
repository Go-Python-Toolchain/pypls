# Editor Setup

Besides the `pypls check` command, pypls is a language server. When your editor
talks to it, you get diagnostics live as you type: the same syntax and type
problems `pypls check` prints, underlined right in the file, updating on every
keystroke. It also offers basic completion.

The command that starts the server is:

```
pypls lsp
```

You do not run this by hand. It speaks the Language Server Protocol over standard
input and output, which is how editors launch and talk to it. Point your
editor's Python language client at `pypls lsp` and the editor does the rest.

Make sure `pypls` is on your `PATH` first. If `pypls version` works in your
terminal, your editor can find it too.

## What you will see

- Diagnostics as you type. A type mismatch or a syntax error is underlined where
  it happens, with the same message the command line prints. Warnings and errors
  are distinguished by your editor's usual styling.
- Basic completion. As you type a name, pypls offers candidates from the current
  file's scope.
- Fast, incremental updates. pypls re-checks only the part of the file you
  changed, so feedback stays quick even in large files.

## Neovim

Neovim has a built-in LSP client, so no plugin is strictly required.

### Built-in, with `vim.lsp.start`

Drop this in your config (for example `init.lua`). It starts pypls for Python
buffers:

```lua
vim.api.nvim_create_autocmd("FileType", {
  pattern = "python",
  callback = function(args)
    vim.lsp.start({
      name = "pypls",
      cmd = { "pypls", "lsp" },
      root_dir = vim.fs.dirname(
        vim.fs.find({ "pyproject.toml", ".git" }, { upward = true })[1]
      ),
    }, { bufnr = args.buf })
  end,
})
```

`root_dir` tells pypls where your project starts, so it can find the nearest
`pyproject.toml` and read your `[tool.pypls]` settings.

### With nvim-lspconfig

If you use [nvim-lspconfig](https://github.com/neovim/nvim-lspconfig), register
pypls as a custom server. nvim-lspconfig does not ship a pypls entry, so you
define one:

```lua
local configs = require("lspconfig.configs")
local lspconfig = require("lspconfig")

if not configs.pypls then
  configs.pypls = {
    default_config = {
      cmd = { "pypls", "lsp" },
      filetypes = { "python" },
      root_dir = lspconfig.util.root_pattern("pyproject.toml", ".git"),
    },
  }
end

lspconfig.pypls.setup({})
```

Open a Python file and diagnostics appear as you edit. `:LspInfo` shows pypls
attached to the buffer.

## VS Code

There is no dedicated pypls extension yet, but VS Code talks to any language
server through a small client extension. The minimal version is a handful of
lines.

Create an extension folder with this `package.json`:

```json
{
  "name": "pypls-vscode",
  "displayName": "pypls",
  "version": "0.0.1",
  "engines": { "vscode": "^1.75.0" },
  "activationEvents": ["onLanguage:python"],
  "main": "./extension.js",
  "dependencies": {
    "vscode-languageclient": "^9.0.0"
  }
}
```

And this `extension.js`:

```javascript
const { LanguageClient } = require("vscode-languageclient/node");

let client;

function activate(context) {
  const serverOptions = {
    command: "pypls",
    args: ["lsp"],
  };
  const clientOptions = {
    documentSelector: [{ scheme: "file", language: "python" }],
  };
  client = new LanguageClient("pypls", "pypls", serverOptions, clientOptions);
  context.subscriptions.push(client.start());
}

function deactivate() {
  return client ? client.stop() : undefined;
}

module.exports = { activate, deactivate };
```

Run `npm install` in the folder, then open it in VS Code and press `F5` to
launch an Extension Development Host. Open a Python file there and pypls
diagnostics appear as you type.

If you prefer not to write an extension, a generic LSP bridge extension that
lets you register an arbitrary `stdio` server works too: set the command to
`pypls` and the argument to `lsp`, with `python` as the language.

## Troubleshooting

- Nothing happens: confirm `pypls version` runs in a plain terminal. If it does
  not, `pypls` is not on your `PATH`.
- Settings ignored: make sure your `pyproject.toml` is at or above the file you
  are editing, so the server's project root includes it.
- Sanity check: run `pypls check` on the same file from the terminal. If the
  command reports a problem, the editor should too.
