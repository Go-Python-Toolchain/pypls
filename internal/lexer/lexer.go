// Package lexer turns Python source text into a stream of tokens. It handles
// Python's significant indentation by emitting INDENT, DEDENT, and NEWLINE
// tokens, and it joins lines implicitly inside brackets and explicitly after a
// backslash. The lexer is deliberately tolerant: on malformed input it records
// an error and keeps going so that later stages can still produce useful
// results.
package lexer

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Go-Python-Toolchain/pypls/internal/token"
)

// tabWidth is the column width a tab advances to, rounded up to the next
// multiple of this value. This matches the common Python convention.
const tabWidth = 8

// Error is a lexical error at a source position.
type Error struct {
	Pos token.Position
	Msg string
}

func (e Error) Error() string {
	return fmt.Sprintf("%s: %s", e.Pos, e.Msg)
}

// Lexer scans a single source file.
type Lexer struct {
	src        []rune
	file       string
	pos        int // rune index into src
	byteOffset int // byte offset into the original source
	line       int // 1-based
	col        int // 1-based, in runes

	parenDepth     int   // depth of (), [], {} nesting
	indent         []int // stack of indentation widths, always starts with 0
	atLineStart    bool
	lineHasContent bool

	tokens []token.Token
	errs   []Error
}

// New creates a lexer for the given source. The file name is used only for
// error messages.
func New(file, source string) *Lexer {
	return &Lexer{
		src:         []rune(source),
		file:        file,
		line:        1,
		col:         1,
		indent:      []int{0},
		atLineStart: true,
	}
}

// Tokenize scans the whole input and returns the token stream and any errors.
// The stream always ends with an EOF token.
func (l *Lexer) Tokenize() ([]token.Token, []Error) {
	for {
		if l.eof() {
			l.finish()
			return l.tokens, l.errs
		}

		if l.atLineStart && l.parenDepth == 0 {
			l.handleLineStart()
			if l.atLineStart {
				// A blank or comment-only line was consumed.
				continue
			}
		}

		l.skipInlineSpace()
		if l.eof() {
			l.finish()
			return l.tokens, l.errs
		}

		r := l.peek()
		switch {
		case r == '#':
			l.skipComment()
		case r == '\n' || r == '\r':
			l.handleNewline()
		case r == '\\' && (l.peekAt(1) == '\n' || l.peekAt(1) == '\r'):
			// Explicit line continuation. Consume the backslash and newline.
			l.advance()
			l.consumeNewline()
		case isIdentStart(r):
			if l.looksLikeStringPrefix() {
				l.lexString()
			} else {
				l.lexName()
			}
			l.lineHasContent = true
		case isDigit(r) || (r == '.' && isDigit(l.peekAt(1))):
			l.lexNumber()
			l.lineHasContent = true
		case r == '"' || r == '\'':
			l.lexString()
			l.lineHasContent = true
		default:
			l.lexOperator()
			l.lineHasContent = true
		}
	}
}

// finish emits the trailing NEWLINE, any pending DEDENTs, and the EOF token.
func (l *Lexer) finish() {
	if l.lineHasContent {
		l.emit(token.NEWLINE, "\n", l.currentPos())
		l.lineHasContent = false
	}
	for len(l.indent) > 1 {
		l.indent = l.indent[:len(l.indent)-1]
		l.emit(token.DEDENT, "", l.currentPos())
	}
	l.emit(token.EOF, "", l.currentPos())
}

// handleLineStart measures the indentation of a logical line and emits INDENT
// or DEDENT tokens. Blank and comment-only lines are skipped without changing
// indentation.
func (l *Lexer) handleLineStart() {
	start := l.currentPos()
	width := l.measureIndent()

	r := l.peek()
	if l.eof() || r == '\n' || r == '\r' || r == '#' {
		if r == '#' {
			l.skipComment()
		}
		if !l.eof() {
			l.consumeNewline()
		}
		// Leave atLineStart true so the next line is measured too.
		return
	}

	top := l.indent[len(l.indent)-1]
	switch {
	case width > top:
		l.indent = append(l.indent, width)
		l.emit(token.INDENT, "", start)
	case width < top:
		for len(l.indent) > 1 && width < l.indent[len(l.indent)-1] {
			l.indent = l.indent[:len(l.indent)-1]
			l.emit(token.DEDENT, "", l.currentPos())
		}
		if l.indent[len(l.indent)-1] != width {
			l.errorf(start, "unindent does not match any outer indentation level")
			// Recover by treating this width as a new level.
			l.indent = append(l.indent, width)
		}
	}
	l.atLineStart = false
}

// handleNewline processes a newline outside of any bracket. It emits a NEWLINE
// token only when the logical line produced content.
func (l *Lexer) handleNewline() {
	if l.parenDepth > 0 {
		l.consumeNewline()
		return
	}
	if l.lineHasContent {
		l.emit(token.NEWLINE, "\n", l.currentPos())
	}
	l.consumeNewline()
	l.atLineStart = true
	l.lineHasContent = false
}

// measureIndent consumes leading whitespace and returns the indentation width.
func (l *Lexer) measureIndent() int {
	width := 0
	for !l.eof() {
		switch l.peek() {
		case ' ':
			width++
			l.advance()
		case '\t':
			width += tabWidth - (width % tabWidth)
			l.advance()
		case '\f':
			l.advance()
		default:
			return width
		}
	}
	return width
}

func (l *Lexer) skipInlineSpace() {
	for !l.eof() {
		switch l.peek() {
		case ' ', '\t', '\f':
			l.advance()
		default:
			return
		}
	}
}

func (l *Lexer) skipComment() {
	for !l.eof() && l.peek() != '\n' && l.peek() != '\r' {
		l.advance()
	}
}

func (l *Lexer) consumeNewline() {
	if l.peek() == '\r' {
		l.advance()
		if l.peek() == '\n' {
			l.advance()
		}
		return
	}
	if l.peek() == '\n' {
		l.advance()
	}
}

func (l *Lexer) lexName() {
	start := l.currentPos()
	var sb strings.Builder
	for !l.eof() && isIdentPart(l.peek()) {
		sb.WriteRune(l.advance())
	}
	val := sb.String()
	l.emit(token.Lookup(val), val, start)
}

func (l *Lexer) lexNumber() {
	start := l.currentPos()
	var sb strings.Builder

	if l.peek() == '0' && isBaseMarker(l.peekAt(1)) {
		sb.WriteRune(l.advance()) // 0
		sb.WriteRune(l.advance()) // base marker
		for !l.eof() && (isHexDigit(l.peek()) || l.peek() == '_') {
			sb.WriteRune(l.advance())
		}
		l.emit(token.NUMBER, sb.String(), start)
		return
	}

	consumeDigits := func() {
		for !l.eof() && (isDigit(l.peek()) || l.peek() == '_') {
			sb.WriteRune(l.advance())
		}
	}

	if l.peek() == '.' {
		sb.WriteRune(l.advance())
		consumeDigits()
	} else {
		consumeDigits()
		if l.peek() == '.' {
			sb.WriteRune(l.advance())
			consumeDigits()
		}
	}

	if l.peek() == 'e' || l.peek() == 'E' {
		next := l.peekAt(1)
		if isDigit(next) || ((next == '+' || next == '-') && isDigit(l.peekAt(2))) {
			sb.WriteRune(l.advance()) // e
			if l.peek() == '+' || l.peek() == '-' {
				sb.WriteRune(l.advance())
			}
			consumeDigits()
		}
	}

	if l.peek() == 'j' || l.peek() == 'J' {
		sb.WriteRune(l.advance())
	}

	l.emit(token.NUMBER, sb.String(), start)
}

func (l *Lexer) lexString() {
	start := l.currentPos()
	var sb strings.Builder

	for isStringPrefix(l.peek()) {
		sb.WriteRune(l.advance())
	}

	quote := l.advance()
	sb.WriteRune(quote)

	triple := l.peek() == quote && l.peekAt(1) == quote
	if triple {
		sb.WriteRune(l.advance())
		sb.WriteRune(l.advance())
		l.lexStringBody(&sb, quote, true, start)
	} else {
		l.lexStringBody(&sb, quote, false, start)
	}

	l.emit(token.STRING, sb.String(), start)
}

func (l *Lexer) lexStringBody(sb *strings.Builder, quote rune, triple bool, start token.Position) {
	for {
		if l.eof() {
			l.errorf(start, "unterminated string literal")
			return
		}
		c := l.peek()

		if c == '\\' {
			sb.WriteRune(l.advance())
			if !l.eof() {
				sb.WriteRune(l.advance())
			}
			continue
		}

		if !triple && (c == '\n' || c == '\r') {
			l.errorf(start, "unterminated string literal")
			return
		}

		if c == quote {
			if triple {
				if l.peekAt(1) == quote && l.peekAt(2) == quote {
					sb.WriteRune(l.advance())
					sb.WriteRune(l.advance())
					sb.WriteRune(l.advance())
					return
				}
			} else {
				sb.WriteRune(l.advance())
				return
			}
		}

		sb.WriteRune(l.advance())
	}
}

func (l *Lexer) lexOperator() {
	start := l.currentPos()
	r := l.peek()
	n1 := l.peekAt(1)
	n2 := l.peekAt(2)

	emit := func(t token.Type, length int) {
		var sb strings.Builder
		for i := 0; i < length; i++ {
			sb.WriteRune(l.advance())
		}
		l.emit(t, sb.String(), start)
	}

	switch r {
	case '+':
		if n1 == '=' {
			emit(token.PLUSEQ, 2)
		} else {
			emit(token.PLUS, 1)
		}
	case '-':
		if n1 == '=' {
			emit(token.MINUSEQ, 2)
		} else if n1 == '>' {
			emit(token.ARROW, 2)
		} else {
			emit(token.MINUS, 1)
		}
	case '*':
		if n1 == '*' {
			if n2 == '=' {
				emit(token.DOUBLESTAREQ, 3)
			} else {
				emit(token.DOUBLESTAR, 2)
			}
		} else if n1 == '=' {
			emit(token.STAREQ, 2)
		} else {
			emit(token.STAR, 1)
		}
	case '/':
		if n1 == '/' {
			if n2 == '=' {
				emit(token.DOUBLESLASHEQ, 3)
			} else {
				emit(token.DOUBLESLASH, 2)
			}
		} else if n1 == '=' {
			emit(token.SLASHEQ, 2)
		} else {
			emit(token.SLASH, 1)
		}
	case '%':
		if n1 == '=' {
			emit(token.PERCENTEQ, 2)
		} else {
			emit(token.PERCENT, 1)
		}
	case '@':
		if n1 == '=' {
			emit(token.ATEQ, 2)
		} else {
			emit(token.AT, 1)
		}
	case '&':
		if n1 == '=' {
			emit(token.AMPEREQ, 2)
		} else {
			emit(token.AMPER, 1)
		}
	case '|':
		if n1 == '=' {
			emit(token.PIPEEQ, 2)
		} else {
			emit(token.PIPE, 1)
		}
	case '^':
		if n1 == '=' {
			emit(token.CARETEQ, 2)
		} else {
			emit(token.CARET, 1)
		}
	case '~':
		emit(token.TILDE, 1)
	case '<':
		if n1 == '<' {
			if n2 == '=' {
				emit(token.LSHIFTEQ, 3)
			} else {
				emit(token.LSHIFT, 2)
			}
		} else if n1 == '=' {
			emit(token.LE, 2)
		} else {
			emit(token.LT, 1)
		}
	case '>':
		if n1 == '>' {
			if n2 == '=' {
				emit(token.RSHIFTEQ, 3)
			} else {
				emit(token.RSHIFT, 2)
			}
		} else if n1 == '=' {
			emit(token.GE, 2)
		} else {
			emit(token.GT, 1)
		}
	case '=':
		if n1 == '=' {
			emit(token.EQEQ, 2)
		} else {
			emit(token.ASSIGN, 1)
		}
	case '!':
		if n1 == '=' {
			emit(token.NEQ, 2)
		} else {
			l.errorf(start, "unexpected character %q", r)
			emit(token.ILLEGAL, 1)
		}
	case '(':
		l.parenDepth++
		emit(token.LPAREN, 1)
	case ')':
		if l.parenDepth > 0 {
			l.parenDepth--
		}
		emit(token.RPAREN, 1)
	case '[':
		l.parenDepth++
		emit(token.LBRACKET, 1)
	case ']':
		if l.parenDepth > 0 {
			l.parenDepth--
		}
		emit(token.RBRACKET, 1)
	case '{':
		l.parenDepth++
		emit(token.LBRACE, 1)
	case '}':
		if l.parenDepth > 0 {
			l.parenDepth--
		}
		emit(token.RBRACE, 1)
	case ',':
		emit(token.COMMA, 1)
	case ':':
		if n1 == '=' {
			emit(token.WALRUS, 2)
		} else {
			emit(token.COLON, 1)
		}
	case '.':
		if n1 == '.' && n2 == '.' {
			emit(token.ELLIPSIS, 3)
		} else {
			emit(token.DOT, 1)
		}
	case ';':
		emit(token.SEMICOLON, 1)
	default:
		l.errorf(start, "unexpected character %q", r)
		emit(token.ILLEGAL, 1)
	}
}

// looksLikeStringPrefix reports whether the upcoming runes form a string prefix
// followed by a quote, distinguishing r"..." from the identifier r.
func (l *Lexer) looksLikeStringPrefix() bool {
	i := l.pos
	n := 0
	for i < len(l.src) && n < 2 && isStringPrefix(l.src[i]) {
		i++
		n++
	}
	return n > 0 && i < len(l.src) && (l.src[i] == '"' || l.src[i] == '\'')
}

func (l *Lexer) eof() bool { return l.pos >= len(l.src) }

func (l *Lexer) peek() rune {
	if l.pos >= len(l.src) {
		return 0
	}
	return l.src[l.pos]
}

func (l *Lexer) peekAt(n int) rune {
	i := l.pos + n
	if i < 0 || i >= len(l.src) {
		return 0
	}
	return l.src[i]
}

func (l *Lexer) advance() rune {
	r := l.src[l.pos]
	l.pos++
	l.byteOffset += utf8.RuneLen(r)
	if r == '\n' {
		l.line++
		l.col = 1
	} else {
		l.col++
	}
	return r
}

func (l *Lexer) currentPos() token.Position {
	return token.Position{Offset: l.byteOffset, Line: l.line, Column: l.col}
}

func (l *Lexer) emit(t token.Type, value string, start token.Position) {
	l.tokens = append(l.tokens, token.Token{
		Type:  t,
		Value: value,
		Start: start,
		End:   l.currentPos(),
	})
}

func (l *Lexer) errorf(pos token.Position, format string, args ...any) {
	l.errs = append(l.errs, Error{Pos: pos, Msg: fmt.Sprintf(format, args...)})
}

func isDigit(r rune) bool { return r >= '0' && r <= '9' }

func isHexDigit(r rune) bool {
	return isDigit(r) || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')
}

func isBaseMarker(r rune) bool {
	switch r {
	case 'x', 'X', 'o', 'O', 'b', 'B':
		return true
	}
	return false
}

func isStringPrefix(r rune) bool {
	switch r {
	case 'r', 'R', 'b', 'B', 'f', 'F', 'u', 'U':
		return true
	}
	return false
}

func isIdentStart(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || r >= 0x80 && (unicode.IsLetter(r) || unicode.IsDigit(r))
}

func isIdentPart(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}
