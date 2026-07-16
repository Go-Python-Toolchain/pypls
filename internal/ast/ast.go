// Package ast defines the abstract syntax tree produced by the pypls parser.
// The node set follows the structure of Python's own ast module closely enough
// to be familiar, while carrying precise source positions for tooling.
package ast

import "github.com/Go-Python-Toolchain/pypls/internal/token"

// Node is any element of the tree. Pos is the start of the node and End is the
// position just past the last rune of the node.
type Node interface {
	Pos() token.Position
	End() token.Position
}

// Span carries the source range of a node. It is embedded in every node.
type Span struct {
	StartPos token.Position
	EndPos   token.Position
}

// Pos returns the start position.
func (s Span) Pos() token.Position { return s.StartPos }

// End returns the end position.
func (s Span) End() token.Position { return s.EndPos }

// Stmt is a statement node.
type Stmt interface {
	Node
	stmtNode()
}

// Expr is an expression node.
type Expr interface {
	Node
	exprNode()
}

// Module is the root of a parsed file.
type Module struct {
	Span
	Body []Stmt
}

// CmpOp is a comparison operator, including the compound forms is-not and
// not-in that do not map to a single token.
type CmpOp int

const (
	CmpEq CmpOp = iota
	CmpNotEq
	CmpLt
	CmpLte
	CmpGt
	CmpGte
	CmpIs
	CmpIsNot
	CmpIn
	CmpNotIn
)

// String renders a comparison operator.
func (c CmpOp) String() string {
	switch c {
	case CmpEq:
		return "=="
	case CmpNotEq:
		return "!="
	case CmpLt:
		return "<"
	case CmpLte:
		return "<="
	case CmpGt:
		return ">"
	case CmpGte:
		return ">="
	case CmpIs:
		return "is"
	case CmpIsNot:
		return "is not"
	case CmpIn:
		return "in"
	case CmpNotIn:
		return "not in"
	}
	return "?"
}

// ParamKind describes the role of a function parameter.
type ParamKind int

const (
	ParamNormal  ParamKind = iota // an ordinary parameter
	ParamVararg                   // *args
	ParamKwarg                    // **kwargs
	ParamStarSep                  // a bare * separating keyword-only params
	ParamSlash                    // a / marking positional-only params
)

// Param is one function parameter.
type Param struct {
	Span
	Name       string
	NamePos    token.Position
	Annotation Expr // may be nil
	Default    Expr // may be nil
	Kind       ParamKind
}

// Parameters is the ordered parameter list of a function or lambda.
type Parameters struct {
	Span
	Params []*Param
}

// Alias is a name in an import statement, with an optional as-name.
type Alias struct {
	Span
	Name   string
	AsName string
}

// Keyword is a keyword argument in a call or a class base list. An empty Arg
// means double-star unpacking, as in f(**kwargs).
type Keyword struct {
	Span
	Arg   string
	Value Expr
}

// WithItem is one item of a with statement.
type WithItem struct {
	Span
	ContextExpr  Expr
	OptionalVars Expr // may be nil
}

// ExceptHandler is one except clause.
type ExceptHandler struct {
	Span
	Type Expr // may be nil for a bare except
	Name string
	Star bool // except* for exception groups
	Body []Stmt
}

// Comprehension is one for clause of a comprehension or generator.
type Comprehension struct {
	Span
	Target Expr
	Iter   Expr
	Ifs    []Expr
	Async  bool
}

// Statement nodes.

type (
	// FunctionDef is a def or async def.
	FunctionDef struct {
		Span
		Decorators []Expr
		Async      bool
		Name       string
		NamePos    token.Position
		Params     *Parameters
		Returns    Expr // may be nil
		Body       []Stmt
	}

	// ClassDef is a class definition.
	ClassDef struct {
		Span
		Decorators []Expr
		Name       string
		NamePos    token.Position
		Bases      []Expr
		Keywords   []*Keyword
		Body       []Stmt
	}

	// Return is a return statement. Value may be nil.
	Return struct {
		Span
		Value Expr
	}

	// Delete is a del statement.
	Delete struct {
		Span
		Targets []Expr
	}

	// Assign is one or more assignment targets sharing a value.
	Assign struct {
		Span
		Targets []Expr
		Value   Expr
	}

	// AugAssign is an augmented assignment such as x += 1.
	AugAssign struct {
		Span
		Target Expr
		Op     token.Type
		Value  Expr
	}

	// AnnAssign is an annotated assignment such as x: int = 1. Value may be nil.
	AnnAssign struct {
		Span
		Target     Expr
		Annotation Expr
		Value      Expr
	}

	// If is an if statement with optional elif and else branches folded into
	// Orelse.
	If struct {
		Span
		Test   Expr
		Body   []Stmt
		Orelse []Stmt
	}

	// While is a while loop.
	While struct {
		Span
		Test   Expr
		Body   []Stmt
		Orelse []Stmt
	}

	// For is a for loop, possibly async.
	For struct {
		Span
		Async  bool
		Target Expr
		Iter   Expr
		Body   []Stmt
		Orelse []Stmt
	}

	// With is a with statement, possibly async.
	With struct {
		Span
		Async bool
		Items []*WithItem
		Body  []Stmt
	}

	// Try is a try statement.
	Try struct {
		Span
		Body      []Stmt
		Handlers  []*ExceptHandler
		Orelse    []Stmt
		Finalbody []Stmt
	}

	// Raise is a raise statement. Both fields may be nil.
	Raise struct {
		Span
		Exc   Expr
		Cause Expr
	}

	// Import is an import statement.
	Import struct {
		Span
		Names []*Alias
	}

	// ImportFrom is a from-import statement. Level is the number of leading dots
	// for relative imports.
	ImportFrom struct {
		Span
		Module string
		Level  int
		Names  []*Alias
		Star   bool
	}

	// Global is a global declaration.
	Global struct {
		Span
		Names []string
	}

	// Nonlocal is a nonlocal declaration.
	Nonlocal struct {
		Span
		Names []string
	}

	// Assert is an assert statement. Msg may be nil.
	Assert struct {
		Span
		Test Expr
		Msg  Expr
	}

	// ExprStmt is a bare expression used as a statement.
	ExprStmt struct {
		Span
		Value Expr
	}

	// Pass is a pass statement.
	Pass struct{ Span }

	// Break is a break statement.
	Break struct{ Span }

	// Continue is a continue statement.
	Continue struct{ Span }

	// Bad marks a statement the parser could not understand. It supports error
	// recovery so analysis can continue past a mistake.
	BadStmt struct{ Span }
)

func (*FunctionDef) stmtNode() {}
func (*ClassDef) stmtNode()    {}
func (*Return) stmtNode()      {}
func (*Delete) stmtNode()      {}
func (*Assign) stmtNode()      {}
func (*AugAssign) stmtNode()   {}
func (*AnnAssign) stmtNode()   {}
func (*If) stmtNode()          {}
func (*While) stmtNode()       {}
func (*For) stmtNode()         {}
func (*With) stmtNode()        {}
func (*Try) stmtNode()         {}
func (*Raise) stmtNode()       {}
func (*Import) stmtNode()      {}
func (*ImportFrom) stmtNode()  {}
func (*Global) stmtNode()      {}
func (*Nonlocal) stmtNode()    {}
func (*Assert) stmtNode()      {}
func (*ExprStmt) stmtNode()    {}
func (*Pass) stmtNode()        {}
func (*Break) stmtNode()       {}
func (*Continue) stmtNode()    {}
func (*BadStmt) stmtNode()     {}

// Expression nodes.

type (
	// Name is an identifier reference.
	Name struct {
		Span
		Id string
	}

	// Number is a numeric literal, kept as its source text.
	Number struct {
		Span
		Value string
	}

	// Str is one or more adjacent string literals, kept as source text. Adjacent
	// literals are implicitly concatenated in Python, so Values may hold more
	// than one entry.
	Str struct {
		Span
		Values []string
	}

	// Constant is a keyword literal: True, False, None, or the ellipsis.
	Constant struct {
		Span
		Value string
	}

	// Tuple is a parenthesized or bare tuple.
	Tuple struct {
		Span
		Elts []Expr
	}

	// List is a list display.
	List struct {
		Span
		Elts []Expr
	}

	// Set is a set display.
	Set struct {
		Span
		Elts []Expr
	}

	// Dict is a dict display. A nil Key marks double-star unpacking.
	Dict struct {
		Span
		Keys   []Expr
		Values []Expr
	}

	// ListComp is a list comprehension.
	ListComp struct {
		Span
		Elt        Expr
		Generators []*Comprehension
	}

	// SetComp is a set comprehension.
	SetComp struct {
		Span
		Elt        Expr
		Generators []*Comprehension
	}

	// DictComp is a dict comprehension.
	DictComp struct {
		Span
		Key        Expr
		Value      Expr
		Generators []*Comprehension
	}

	// GeneratorExp is a generator expression.
	GeneratorExp struct {
		Span
		Elt        Expr
		Generators []*Comprehension
	}

	// BoolOp is a chain of and or or operations.
	BoolOp struct {
		Span
		Op     token.Type
		Values []Expr
	}

	// BinOp is a binary arithmetic or bitwise operation.
	BinOp struct {
		Span
		Left  Expr
		Op    token.Type
		Right Expr
	}

	// UnaryOp is a unary operation: +, -, ~, or not.
	UnaryOp struct {
		Span
		Op      token.Type
		Operand Expr
	}

	// Compare is a possibly chained comparison such as a < b <= c.
	Compare struct {
		Span
		Left        Expr
		Ops         []CmpOp
		Comparators []Expr
	}

	// Call is a function call.
	Call struct {
		Span
		Func     Expr
		Args     []Expr
		Keywords []*Keyword
	}

	// Attribute is an attribute access such as a.b.
	Attribute struct {
		Span
		Value   Expr
		Attr    string
		AttrPos token.Position
	}

	// Subscript is an indexing operation such as a[b].
	Subscript struct {
		Span
		Value Expr
		Slice Expr
	}

	// Slice is a slice expression such as a:b:c inside a subscript.
	Slice struct {
		Span
		Lower Expr // may be nil
		Upper Expr // may be nil
		Step  Expr // may be nil
	}

	// Starred is a starred expression such as *x.
	Starred struct {
		Span
		Value Expr
	}

	// Lambda is a lambda expression.
	Lambda struct {
		Span
		Params *Parameters
		Body   Expr
	}

	// IfExp is a conditional expression: body if test else orelse.
	IfExp struct {
		Span
		Body   Expr
		Test   Expr
		Orelse Expr
	}

	// Await is an await expression.
	Await struct {
		Span
		Value Expr
	}

	// Yield is a yield expression. Value may be nil.
	Yield struct {
		Span
		Value Expr
	}

	// YieldFrom is a yield from expression.
	YieldFrom struct {
		Span
		Value Expr
	}

	// NamedExpr is a walrus assignment expression: target := value.
	NamedExpr struct {
		Span
		Target Expr
		Value  Expr
	}

	// BadExpr marks an expression the parser could not understand.
	BadExpr struct{ Span }
)

func (*Name) exprNode()         {}
func (*Number) exprNode()       {}
func (*Str) exprNode()          {}
func (*Constant) exprNode()     {}
func (*Tuple) exprNode()        {}
func (*List) exprNode()         {}
func (*Set) exprNode()          {}
func (*Dict) exprNode()         {}
func (*ListComp) exprNode()     {}
func (*SetComp) exprNode()      {}
func (*DictComp) exprNode()     {}
func (*GeneratorExp) exprNode() {}
func (*BoolOp) exprNode()       {}
func (*BinOp) exprNode()        {}
func (*UnaryOp) exprNode()      {}
func (*Compare) exprNode()      {}
func (*Call) exprNode()         {}
func (*Attribute) exprNode()    {}
func (*Subscript) exprNode()    {}
func (*Slice) exprNode()        {}
func (*Starred) exprNode()      {}
func (*Lambda) exprNode()       {}
func (*IfExp) exprNode()        {}
func (*Await) exprNode()        {}
func (*Yield) exprNode()        {}
func (*YieldFrom) exprNode()    {}
func (*NamedExpr) exprNode()    {}
func (*BadExpr) exprNode()      {}
