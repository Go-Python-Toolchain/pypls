# pypls Architecture

This document describes how pypls is built and the reasoning behind the main design choices. It grows as new layers land.

## Pipeline

Source text moves through a small set of stages, each in its own package under `internal/`.

1. `token` defines the lexical tokens and source positions.
2. `lexer` turns source text into a token stream.
3. `ast` defines the tree node types.
4. `parser` turns the token stream into an abstract syntax tree.
5. `diagnostic` defines the shape of a reported problem, mirroring the Language Server Protocol.
6. `types` defines the type lattice and the rules for combining and comparing types.
7. `checker` performs local type inference and reports type problems.
8. `cache` is a persistent store that lets unchanged inputs skip recomputation.
9. `analyzer` runs the pipeline and returns diagnostics for a file, and drives incremental re-analysis as a document is edited.
10. `lsp` is the language server that speaks to editors.

## Diagnostics

A diagnostic carries a source range, a severity, a short code, and a message. The range comes straight from the token or node that caused the problem, so an editor can underline exactly the offending text. Severities follow the Language Server Protocol numbering, which lets the same value be printed on the command line or sent to an editor without translation.

The `analyzer` package is the single entry point that turns source into diagnostics. It reports syntax problems from the parser and type problems from the checker, so callers get everything from one call.

A full check of a generated ten thousand line file, covering parsing, type inference, and diagnostics, runs in roughly twenty milliseconds.

## Type inference

The `checker` package infers types locally: within a function or module it reasons from literals, assignments, annotations, and a small set of builtin constructors such as `int` and `str`. It does not resolve imports or cross-module types yet.

The guiding rule is to stay silent when a type is not known. Unknown types are treated as compatible with everything, so untyped code produces no diagnostics at all. The checker only reports a problem when it is confident: an annotated assignment whose value has a clearly incompatible builtin type, or a binary operation between two known builtin types that Python does not allow, such as adding a string to an integer. This keeps the signal high and false positives near zero.

The type lattice in the `types` package is coarse on purpose: scalars, the common containers, callables, and an explicit Unknown. Numeric widening follows Python, so an integer fits where a float is expected and a bool fits where an int is expected.

## Caching

The `cache` package is a persistent key-value store backed by BadgerDB, kept under the standard cache directory (`~/.cache/gpt/pypls`, honoring `XDG_CACHE_HOME`). Values are encoded with msgpack. Every key carries a schema version, so a change to a stored value's shape quietly retires old entries.

The `analyzer` uses the cache to skip work: a check result is stored under the hash of the file's content, so an unchanged file is served from the cache and any edit produces a new key that misses and is recomputed. On a ten thousand line file this turns a full check of around twenty milliseconds into a cache hit of well under a millisecond.

The cache is defensive. A read or decode failure is treated as a miss rather than an error, and a cache directory that cannot be opened is reset and recreated once, so a damaged cache slows the tool down at worst rather than breaking it. This same store will later hold resolved types for the standard library and popular packages, so that heavy cross-module analysis is done once and reused.

## Incremental analysis

When a file is edited, parsing is always rerun because it is cheap and gives fresh, correct positions. The expensive type work is where incremental analysis pays off. The analyzer splits the module into top-level units, one per top-level statement, and remembers each unit's diagnostics keyed by the hash of its text. On the next edit, a unit whose text is unchanged is served from memory, and its diagnostics are shifted to the unit's new line and byte position, which is exact because the unit's internal layout has not changed. Only the units that actually changed are type-checked again. Editing one function in a file of many re-checks exactly that one function.

## Language server

The `lsp` package speaks JSON-RPC 2.0 over standard input and output. It implements the core of the protocol: the initialize and shutdown lifecycle, open, change, and close notifications for documents, diagnostics publishing, and completion. Each open document keeps its own incremental analyzer, so edits are cheap. Internal positions, which use one-based lines and rune columns, are converted to the protocol's zero-based lines and UTF-16 character offsets on the way out.

The server is safe for concurrent use. Document state is guarded by a read-write mutex, and writes to the connection are serialized, so the read loop and the file watcher can run at the same time. A file watcher based on fsnotify notices Python files that change on disk outside the editor and republishes their diagnostics, while leaving files that are open in the editor to the editor. Starting the server and answering the initialize request takes a few microseconds, well under the ten millisecond target.

## Positions

Every token and every tree node carries a start and an end position. Positions record a 1-based line, a 1-based column counted in runes, and a 0-based byte offset. Ends are exclusive, so the span of a node covers exactly the source it was built from. Precise spans are what let later stages point at the exact range of a problem.

## Lexer

Python is indentation sensitive, so the lexer does more than split text into tokens. It tracks an indentation stack and emits INDENT and DEDENT tokens when the indentation of a logical line changes, and a NEWLINE token at the end of each logical line that produced content. Blank lines and comment-only lines never change indentation. Inside brackets, newlines are joined implicitly, and a backslash at the end of a line joins the next line explicitly.

The lexer is deliberately tolerant. On malformed input it records an error and keeps scanning, so later stages still receive a usable stream.

## Parser

The parser is a hand-written recursive descent parser. Expression precedence is handled by a layered set of functions, one per precedence level, with a small precedence-climbing helper for the left-associative binary operators.

We chose a hand-written parser over a parser generator for three reasons that map directly to the goals of the project:

1. Indentation. Python's INDENT and DEDENT structure is produced by the lexer and consumed naturally by a hand-written parser. Grammar generators that assume a context-free token stream fit this poorly.
2. Precise errors. Tooling lives or dies on the quality of its error positions. A hand-written parser gives full control over where an error is reported and how recovery proceeds.
3. Speed. A direct parser avoids reflection and generic machinery, which keeps parsing fast enough to run on every keystroke.

The parser recovers from errors by recording a diagnostic and skipping to the next clean line boundary, so a single mistake never hides the rest of a file.

## Performance

Parsing is measured with a benchmark over a generated ten thousand line file. The parser handles that input in roughly twenty milliseconds on a typical development machine, which leaves ample headroom for the responsiveness targets of the language server.
