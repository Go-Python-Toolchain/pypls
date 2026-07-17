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

There is a pypls extension for VS Code. It is a thin client that starts
`pypls lsp` and shows its results as you type. The extension is distributed as a
`.vsix` file on the GitHub releases page rather than through the Marketplace, and
VS Code installs `.vsix` files directly. The full walkthrough is below.

### Step 1: install pypls

The extension needs the pypls executable, so install it first and confirm it
runs.

```
pip install pypls-client
pypls version
```

If `pypls version` prints a version in your terminal, the extension will be able
to find it.

### Step 2: download the extension file

Every pypls release attaches a `pypls.vsix` file. This link always points at the
newest release:

```
https://github.com/Go-Python-Toolchain/pypls/releases/latest/download/pypls.vsix
```

Open it in a browser to download, or fetch it from the terminal:

```
curl -L -o pypls.vsix https://github.com/Go-Python-Toolchain/pypls/releases/latest/download/pypls.vsix
```

To pin a specific version instead, open the releases page, expand the version you
want, and download its `pypls.vsix` from the assets list:
https://github.com/Go-Python-Toolchain/pypls/releases

### Step 3: install the .vsix

There are two ways, and both install the same file. Pick whichever you prefer.

From the command line, point `code` at the file you downloaded:

```
code --install-extension pypls.vsix
```

Run this from the folder that holds `pypls.vsix`, or pass the full path to it.
The same command works in editors that reuse the VS Code extension system: use
`codium` for VSCodium, and Cursor and similar editors accept the file through
their own extensions menu.

From the VS Code UI:

1. Open the Extensions view (`Ctrl+Shift+X`, or `Cmd+Shift+X` on macOS).
2. Click the `...` menu at the top of that view.
3. Choose "Install from VSIX...".
4. Select the `pypls.vsix` file you downloaded.
5. Reload the window if VS Code asks.

### Step 4: check that it works

Open any Python file. The extension activates for Python and starts the server.
Introduce an obvious type mistake, such as adding a string to a number, and you
should see it underlined within a moment, with the same message `pypls check`
prints. You can also open the Extensions view and confirm "pypls" is listed under
the installed extensions.

If pypls is on your PATH, no configuration is needed.

### Settings

- `pypls.path`: path to the pypls executable. Leave it as `pypls` to use the one
  on your PATH, or set an absolute path if pypls lives somewhere the editor does
  not search.
- `pypls.trace.server`: set to `messages` or `verbose` to log the traffic between
  VS Code and the server when you are diagnosing a problem.

The command "pypls: Restart Language Server" (open the Command Palette with
`Ctrl+Shift+P` and type it) restarts the server without reloading the window.

### Updating to a newer version

Download the newer `pypls.vsix` from the releases page and install it the same
way. Installing over an existing version replaces it. Reload the window when
prompted.

### Uninstalling

From the Extensions view, find "pypls", click the gear, and choose "Uninstall".
Or from the command line:

```
code --uninstall-extension go-python-toolchain.pypls
```

### Build the extension from source

If you would rather build the `.vsix` yourself, it lives in the pypls repository
under `editors/vscode`:

```
cd editors/vscode
npm install
npm run package
```

That writes `pypls.vsix`, which you install with the same steps as above. This is
also how a new `.vsix` is produced for each release.

## Troubleshooting

- Nothing happens: confirm `pypls version` runs in a plain terminal. If it does
  not, `pypls` is not on your `PATH`.
- Settings ignored: make sure your `pyproject.toml` is at or above the file you
  are editing, so the server's project root includes it.
- Sanity check: run `pypls check` on the same file from the terminal. If the
  command reports a problem, the editor should too.
