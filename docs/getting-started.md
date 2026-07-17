# Getting Started

pypls is a fast type checker and language server for Python. This guide gets it
installed and runs your first check in a couple of minutes.

## What pypls does

pypls reads your Python code and points out two kinds of problems: syntax errors
that stop the file from parsing, and type problems where the values do not line
up with the annotations, such as assigning a string to a variable you declared
as an `int`, or adding a string to a number. It reasons locally from literals,
annotations, and a handful of builtin types, and it stays quiet whenever it is
not sure, so plain untyped code produces no noise. It runs entirely on your
machine and never sends your code anywhere.

You can use pypls two ways: as a command you run (`pypls check`) and as a
language server your editor talks to (`pypls lsp`) for live feedback as you type.

## Install

The easiest way is the Python launcher. It downloads the native binary for your
platform the first time you run it:

```
pip install pypls-client
pypls version
```

You should see a line like this:

```
pypls 0.1.0 (commit ..., built ..., linux/amd64)
```

### Build from source

If you have Go 1.22 or newer, you can build the binary yourself:

```
git clone https://github.com/Go-Python-Toolchain/pypls
cd pypls
go build -o pypls .
./pypls version
```

## Your first check

Create a file called `example.py`:

```python
count: int = "three"
```

Run pypls on it:

```
pypls check example.py
```

pypls reports the mismatch and tells you how many files it looked at:

```
example.py:1:14: warning: value of type str is not assignable to int
checked 1 file(s), found 1 problem(s)
```

Every problem is printed on its own line as
`file:line:column: severity: message`, so it is easy to read and easy for other
tools to parse. The severity is either `error` or `warning`.

Change the value to a number:

```python
count: int = 3
```

Run it again and pypls is happy:

```
checked 1 file(s), no problems found
```

## Where to go next

- [Tutorial](tutorial.md): a hands-on walkthrough that catches real problems,
  fixes them, and shows strict mode and project configuration.
- [Editor setup](editors.md): wire `pypls lsp` into VS Code or Neovim for live
  diagnostics as you type.
- [examples/basic](../examples/basic): a tiny project you can check right away.
