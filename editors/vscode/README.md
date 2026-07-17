# pypls for VS Code

Fast, incremental type checking and diagnostics for Python, right in the editor.
This extension is a thin client: it starts the pypls language server and shows
its results as you type. The checking is done by pypls, so you get the same
syntax and type problems that `pypls check` prints on the command line.

## Requirements

The extension needs the pypls executable on your machine.

```
pip install pypls-client
pypls version
```

If `pypls version` prints a version in your terminal, the extension can find it.

## Install the extension

The extension is distributed as a `.vsix` file on the GitHub releases page, not
through the Marketplace. VS Code installs `.vsix` files directly.

1. Download `pypls.vsix`. This link always points at the newest release:
   https://github.com/Go-Python-Toolchain/pypls/releases/latest/download/pypls.vsix
   To pin a version instead, pick it from the releases page:
   https://github.com/Go-Python-Toolchain/pypls/releases

2. Install it, either from the command line:

   ```
   code --install-extension pypls.vsix
   ```

   (Use `codium` for VSCodium. Cursor and similar editors accept the file
   through their own extensions menu.)

   Or from the VS Code UI: open the Extensions view, click the `...` menu at the
   top, choose "Install from VSIX...", and pick `pypls.vsix`. Reload if asked.

3. Open a Python file. Diagnostics appear as you type.

To update, download a newer `pypls.vsix` and install it the same way; it
replaces the old one. To remove it, run
`code --uninstall-extension go-python-toolchain.pypls`.

## What you get

- Diagnostics as you type, underlined where the problem is, with the same
  message the command line prints.
- Basic completion from names in the current file's scope.
- Incremental checking, so feedback stays fast in large files.

## Settings

- `pypls.path`: path to the pypls executable. Leave it as `pypls` to use the one
  on your PATH.
- `pypls.trace.server`: set to `messages` or `verbose` to log the traffic
  between VS Code and the server when you are diagnosing a problem.

The command "pypls: Restart Language Server" restarts the server without
reloading the window.

## Build from source

From this folder:

```
npm install
npm run package
```

That writes `pypls.vsix`, which you can install with the steps above.

## License

Apache-2.0. See the LICENSE file in the pypls repository.
