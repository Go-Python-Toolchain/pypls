# pypls

A fast, incremental type checker and language server for Python, written in Go.

pypls is a persistent daemon. It keeps a type graph in memory and re-analyzes only the code you change, so your editor stays responsive on large projects. It runs entirely on your machine and never sends your code anywhere.

pypls is part of the [Go-Python Toolchain](https://github.com/Go-Python-Toolchain). It sits alongside your existing tools and does not ask you to change your code, your project layout, or your commands.

## Status

Active development. The parser, syntax and type diagnostics, a persistent cache, and the language server are in place and working. Distribution and packaging are next. Track progress in the issues and releases.

## Install

While pre-release, build from source:

```
git clone https://github.com/Go-Python-Toolchain/pypls
cd pypls
go build -o pypls .
./pypls version
```

Requires Go 1.22 or newer.

## Usage

```
pypls --help
pypls version
pypls check path/to/file.py
pypls check src/
```

`check` parses the given files or directories and reports any problems it finds,
one per line, as `file:line:column: severity: message`. It reports both syntax
errors and type problems, and exits with a non-zero status when any error is
found, so it fits cleanly into continuous integration.

Results are cached on disk under your cache directory, so unchanged files are not
rechecked. Pass `--no-cache` to check everything from scratch.

### Editor integration

pypls is also a language server. Point your editor's Python language client at:

```
pypls lsp
```

It speaks the Language Server Protocol over stdin and stdout, publishes syntax
and type diagnostics as you type, re-checks only the parts of a file that
changed, and offers basic completion.

## Design

- Persistent daemon with incremental analysis.
- Simplified Hindley-Milner type inference.
- Local, embedded cache for standard library and popular package types.
- Full Language Server Protocol support for editor integration.

## License

Apache License 2.0. See [LICENSE](LICENSE) and [NOTICE](NOTICE).
