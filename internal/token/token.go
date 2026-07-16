// Package token defines the lexical tokens of the Python language as produced
// by the pypls lexer, along with source position information.
package token

import "fmt"

// Type is the set of lexical token types.
type Type int

const (
	// Special tokens.
	ILLEGAL Type = iota // an unexpected or malformed token
	EOF                 // end of input
	COMMENT             // a comment, retained for tooling that wants it

	// Structural tokens driven by significant indentation.
	NEWLINE // the end of a logical line
	INDENT  // an increase in indentation
	DEDENT  // a decrease in indentation

	// Literals and identifiers.
	NAME   // an identifier
	NUMBER // an integer, float, or imaginary literal
	STRING // any string or bytes literal, including f-strings, with its prefix and quotes intact

	// Keywords.
	keywordsStart
	FALSE
	NONE
	TRUE
	AND
	AS
	ASSERT
	ASYNC
	AWAIT
	BREAK
	CLASS
	CONTINUE
	DEF
	DEL
	ELIF
	ELSE
	EXCEPT
	FINALLY
	FOR
	FROM
	GLOBAL
	IF
	IMPORT
	IN
	IS
	LAMBDA
	NONLOCAL
	NOT
	OR
	PASS
	RAISE
	RETURN
	TRY
	WHILE
	WITH
	YIELD
	keywordsEnd

	// Operators and delimiters.
	PLUS        // +
	MINUS       // -
	STAR        // *
	SLASH       // /
	PERCENT     // %
	AT          // @
	DOUBLESTAR  // **
	DOUBLESLASH // //
	AMPER       // &
	PIPE        // |
	CARET       // ^
	TILDE       // ~
	LSHIFT      // <<
	RSHIFT      // >>

	LT   // <
	GT   // >
	LE   // <=
	GE   // >=
	EQEQ // ==
	NEQ  // !=

	ASSIGN        // =
	PLUSEQ        // +=
	MINUSEQ       // -=
	STAREQ        // *=
	SLASHEQ       // /=
	PERCENTEQ     // %=
	ATEQ          // @=
	AMPEREQ       // &=
	PIPEEQ        // |=
	CARETEQ       // ^=
	LSHIFTEQ      // <<=
	RSHIFTEQ      // >>=
	DOUBLESTAREQ  // **=
	DOUBLESLASHEQ // //=
	WALRUS        // :=

	LPAREN   // (
	RPAREN   // )
	LBRACKET // [
	RBRACKET // ]
	LBRACE   // {
	RBRACE   // }

	COMMA     // ,
	COLON     // :
	DOT       // .
	SEMICOLON // ;
	ARROW     // ->
	ELLIPSIS  // ...
)

var names = map[Type]string{
	ILLEGAL: "ILLEGAL",
	EOF:     "EOF",
	COMMENT: "COMMENT",
	NEWLINE: "NEWLINE",
	INDENT:  "INDENT",
	DEDENT:  "DEDENT",
	NAME:    "NAME",
	NUMBER:  "NUMBER",
	STRING:  "STRING",

	FALSE:    "False",
	NONE:     "None",
	TRUE:     "True",
	AND:      "and",
	AS:       "as",
	ASSERT:   "assert",
	ASYNC:    "async",
	AWAIT:    "await",
	BREAK:    "break",
	CLASS:    "class",
	CONTINUE: "continue",
	DEF:      "def",
	DEL:      "del",
	ELIF:     "elif",
	ELSE:     "else",
	EXCEPT:   "except",
	FINALLY:  "finally",
	FOR:      "for",
	FROM:     "from",
	GLOBAL:   "global",
	IF:       "if",
	IMPORT:   "import",
	IN:       "in",
	IS:       "is",
	LAMBDA:   "lambda",
	NONLOCAL: "nonlocal",
	NOT:      "not",
	OR:       "or",
	PASS:     "pass",
	RAISE:    "raise",
	RETURN:   "return",
	TRY:      "try",
	WHILE:    "while",
	WITH:     "with",
	YIELD:    "yield",

	PLUS:        "+",
	MINUS:       "-",
	STAR:        "*",
	SLASH:       "/",
	PERCENT:     "%",
	AT:          "@",
	DOUBLESTAR:  "**",
	DOUBLESLASH: "//",
	AMPER:       "&",
	PIPE:        "|",
	CARET:       "^",
	TILDE:       "~",
	LSHIFT:      "<<",
	RSHIFT:      ">>",

	LT:   "<",
	GT:   ">",
	LE:   "<=",
	GE:   ">=",
	EQEQ: "==",
	NEQ:  "!=",

	ASSIGN:        "=",
	PLUSEQ:        "+=",
	MINUSEQ:       "-=",
	STAREQ:        "*=",
	SLASHEQ:       "/=",
	PERCENTEQ:     "%=",
	ATEQ:          "@=",
	AMPEREQ:       "&=",
	PIPEEQ:        "|=",
	CARETEQ:       "^=",
	LSHIFTEQ:      "<<=",
	RSHIFTEQ:      ">>=",
	DOUBLESTAREQ:  "**=",
	DOUBLESLASHEQ: "//=",
	WALRUS:        ":=",

	LPAREN:   "(",
	RPAREN:   ")",
	LBRACKET: "[",
	RBRACKET: "]",
	LBRACE:   "{",
	RBRACE:   "}",

	COMMA:     ",",
	COLON:     ":",
	DOT:       ".",
	SEMICOLON: ";",
	ARROW:     "->",
	ELLIPSIS:  "...",
}

var keywords = map[string]Type{}

func init() {
	for t := keywordsStart + 1; t < keywordsEnd; t++ {
		keywords[names[t]] = t
	}
}

// Lookup returns the keyword token type for an identifier, or NAME if the
// identifier is not a reserved keyword. The soft keywords match and case are
// intentionally not reserved here and are handled by the parser in context.
func Lookup(ident string) Type {
	if t, ok := keywords[ident]; ok {
		return t
	}
	return NAME
}

// IsKeyword reports whether the token type is a reserved keyword.
func (t Type) IsKeyword() bool {
	return t > keywordsStart && t < keywordsEnd
}

// String returns a readable name for the token type.
func (t Type) String() string {
	if s, ok := names[t]; ok {
		return s
	}
	return fmt.Sprintf("Type(%d)", int(t))
}

// Position is a source position. Lines and columns are 1-based. Column counts
// runes, not bytes. Offset is the 0-based byte offset into the source.
type Position struct {
	Offset int
	Line   int
	Column int
}

// Valid reports whether the position has been set.
func (p Position) Valid() bool { return p.Line > 0 }

// String renders the position as line:column.
func (p Position) String() string {
	return fmt.Sprintf("%d:%d", p.Line, p.Column)
}

// Token is a single lexical token with its source span. Start is inclusive and
// End is exclusive.
type Token struct {
	Type  Type
	Value string
	Start Position
	End   Position
}

// String renders the token for debugging.
func (t Token) String() string {
	switch t.Type {
	case NAME, NUMBER, STRING, COMMENT, ILLEGAL:
		return fmt.Sprintf("%s(%q)", t.Type, t.Value)
	default:
		return t.Type.String()
	}
}
