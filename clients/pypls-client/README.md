# pypls-client

A small installer and launcher for [pypls](https://github.com/Go-Python-Toolchain/pypls), a fast type checker and language server for Python written in Go.

Installing this package gives you a `pypls` command. The first time you run it, it downloads the native binary that matches your platform from the project's GitHub releases, verifies its checksum, and caches it. Every later run reuses the cached binary, so there is no per-run overhead.

## Install

```
pip install pypls-client
```

## Use

```
pypls version
pypls check path/to/file.py
pypls lsp
```

`check` reports syntax and type problems. `lsp` runs the language server for editor integration. See the [main project](https://github.com/Go-Python-Toolchain/pypls) for full documentation.

## Supported platforms

Linux and macOS on x86_64 and arm64, and Windows on x86_64.

## License

Apache License 2.0.
