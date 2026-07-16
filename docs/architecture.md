# pypls Architecture

This document describes how pypls is built and the reasoning behind the main design choices. It grows as new layers land.

## Pipeline

Source text moves through a small set of stages, each in its own package under `internal/`.

1. `token` defines the lexical tokens and source positions.
2. `lexer` turns source text into a token stream.
3. `ast` defines the tree node types.
4. `parser` turns the token stream into an abstract syntax tree.
5. `diagnostic` defines the shape of a reported problem, mirroring the Language Server Protocol.
6. `analyzer` runs the pipeline and returns diagnostics for a file.

Later stages (type inference, caching, and the language server) build on these foundations without changing them.

## Diagnostics

A diagnostic carries a source range, a severity, a short code, and a message. The range comes straight from the token or node that caused the problem, so an editor can underline exactly the offending text. Severities follow the Language Server Protocol numbering, which lets the same value be printed on the command line or sent to an editor without translation.

The `analyzer` package is the single entry point that turns source into diagnostics. Today it reports syntax problems. Type diagnostics will be added to the same function so that callers never need to change.

A full check of a generated ten thousand line file, covering both parsing and diagnostics, runs in roughly twenty milliseconds.

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
