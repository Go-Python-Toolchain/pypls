// Package parser turns Python source into an abstract syntax tree. It is a
// hand-written recursive descent parser with a precedence-climbing expression
// parser. The parser is tolerant: on an error it records a diagnostic, recovers
// to the next line, and keeps going, so a single mistake does not hide the rest
// of the file.
package parser

import (
	"fmt"

	"github.com/Go-Python-Toolchain/pypls/internal/ast"
	"github.com/Go-Python-Toolchain/pypls/internal/lexer"
	"github.com/Go-Python-Toolchain/pypls/internal/token"
)

// Error is a syntax error covering a source range. Start is inclusive and End
// is exclusive. For errors without a natural width, End equals Start.
type Error struct {
	Start token.Position
	End   token.Position
	Msg   string
}

func (e Error) Error() string {
	return fmt.Sprintf("%s: %s", e.Start, e.Msg)
}

// Parser holds the state for parsing one file.
type Parser struct {
	toks []token.Token
	pos  int
	errs []Error
}

// Parse lexes and parses source and returns the module together with any lexer
// and parser errors, in source order.
func Parse(file, source string) (*ast.Module, []Error) {
	toks, lexErrs := lexer.New(file, source).Tokenize()
	p := &Parser{toks: toks}
	for _, le := range lexErrs {
		p.errs = append(p.errs, Error{Start: le.Pos, End: le.Pos, Msg: le.Msg})
	}
	mod := p.parseModule()
	return mod, p.errs
}

// Token stream helpers.

func (p *Parser) cur() token.Token { return p.toks[p.pos] }

func (p *Parser) peek(n int) token.Token {
	i := p.pos + n
	if i >= len(p.toks) {
		return p.toks[len(p.toks)-1] // EOF
	}
	return p.toks[i]
}

func (p *Parser) at(t token.Type) bool { return p.cur().Type == t }

func (p *Parser) advance() token.Token {
	t := p.toks[p.pos]
	if p.pos < len(p.toks)-1 {
		p.pos++
	}
	return t
}

func (p *Parser) expect(t token.Type) (token.Token, bool) {
	if p.at(t) {
		return p.advance(), true
	}
	p.errorTok(p.cur(), "expected %s, found %s", t, describe(p.cur()))
	return p.cur(), false
}

func (p *Parser) prevEnd() token.Position {
	if p.pos > 0 {
		return p.toks[p.pos-1].End
	}
	return p.cur().Start
}

func (p *Parser) spanFrom(start token.Position) ast.Span {
	return ast.Span{StartPos: start, EndPos: p.prevEnd()}
}

func (p *Parser) errorf(pos token.Position, format string, args ...any) {
	p.errs = append(p.errs, Error{Start: pos, End: pos, Msg: fmt.Sprintf(format, args...)})
}

// errorTok records an error covering the full span of a token, which gives the
// most useful range for editors and command line output.
func (p *Parser) errorTok(t token.Token, format string, args ...any) {
	p.errs = append(p.errs, Error{Start: t.Start, End: t.End, Msg: fmt.Sprintf(format, args...)})
}

func describe(t token.Token) string {
	switch t.Type {
	case token.NAME, token.NUMBER, token.STRING:
		return fmt.Sprintf("%q", t.Value)
	case token.NEWLINE:
		return "end of line"
	case token.EOF:
		return "end of file"
	case token.INDENT:
		return "an indent"
	case token.DEDENT:
		return "a dedent"
	default:
		return t.Type.String()
	}
}

// synchronize skips tokens up to and including the next NEWLINE, or up to a
// DEDENT or EOF, so parsing can resume at a clean boundary.
func (p *Parser) synchronize() {
	for !p.at(token.EOF) {
		if p.at(token.NEWLINE) {
			p.advance()
			return
		}
		if p.at(token.DEDENT) {
			return
		}
		p.advance()
	}
}

// Statements.

func (p *Parser) parseModule() *ast.Module {
	start := p.cur().Start
	var body []ast.Stmt
	for !p.at(token.EOF) {
		if p.at(token.NEWLINE) || p.at(token.INDENT) || p.at(token.DEDENT) {
			p.advance()
			continue
		}
		before := p.pos
		p.parseStatementLine(&body)
		if p.pos == before {
			p.advance()
		}
	}
	return &ast.Module{Span: p.spanFrom(start), Body: body}
}

// parseStatementLine parses one logical line, which is either a single compound
// statement or a run of simple statements separated by semicolons.
func (p *Parser) parseStatementLine(body *[]ast.Stmt) {
	switch p.cur().Type {
	case token.AT:
		p.parseDecorated(body)
	case token.DEF:
		*body = append(*body, p.parseFunctionDef(nil, false))
	case token.CLASS:
		*body = append(*body, p.parseClassDef(nil))
	case token.IF:
		*body = append(*body, p.parseIf())
	case token.WHILE:
		*body = append(*body, p.parseWhile())
	case token.FOR:
		*body = append(*body, p.parseFor(false))
	case token.TRY:
		*body = append(*body, p.parseTry())
	case token.WITH:
		*body = append(*body, p.parseWith(false))
	case token.ASYNC:
		p.parseAsync(body)
	default:
		p.parseSimpleLine(body)
	}
}

func (p *Parser) parseAsync(body *[]ast.Stmt) {
	start := p.cur().Start
	p.advance() // async
	switch p.cur().Type {
	case token.DEF:
		fn := p.parseFunctionDef(nil, true)
		fn.StartPos = start
		*body = append(*body, fn)
	case token.FOR:
		st := p.parseFor(true)
		st.StartPos = start
		*body = append(*body, st)
	case token.WITH:
		st := p.parseWith(true)
		st.StartPos = start
		*body = append(*body, st)
	default:
		p.errorf(p.cur().Start, "expected def, for, or with after async")
		p.synchronize()
		*body = append(*body, &ast.BadStmt{Span: p.spanFrom(start)})
	}
}

func (p *Parser) parseDecorated(body *[]ast.Stmt) {
	var decorators []ast.Expr
	for p.at(token.AT) {
		p.advance()
		e := p.parseNamedExpr()
		decorators = append(decorators, e)
		p.expectLineEnd()
		for p.at(token.NEWLINE) {
			p.advance()
		}
	}
	switch p.cur().Type {
	case token.DEF:
		*body = append(*body, p.parseFunctionDef(decorators, false))
	case token.CLASS:
		*body = append(*body, p.parseClassDef(decorators))
	case token.ASYNC:
		start := p.cur().Start
		p.advance()
		fn := p.parseFunctionDef(decorators, true)
		fn.StartPos = start
		*body = append(*body, fn)
	default:
		p.errorf(p.cur().Start, "expected def or class after decorator")
		p.synchronize()
	}
}

func (p *Parser) parseFunctionDef(decorators []ast.Expr, async bool) *ast.FunctionDef {
	start := p.cur().Start
	if len(decorators) > 0 {
		start = decorators[0].Pos()
	}
	p.expect(token.DEF)
	name, _ := p.expect(token.NAME)
	fn := &ast.FunctionDef{
		Decorators: decorators,
		Async:      async,
		Name:       name.Value,
		NamePos:    name.Start,
	}
	p.expect(token.LPAREN)
	fn.Params = p.parseParameters(token.RPAREN, true)
	p.expect(token.RPAREN)
	if p.at(token.ARROW) {
		p.advance()
		fn.Returns = p.parseTest()
	}
	p.expect(token.COLON)
	fn.Body = p.parseSuite()
	fn.Span = p.spanFrom(start)
	return fn
}

func (p *Parser) parseClassDef(decorators []ast.Expr) *ast.ClassDef {
	start := p.cur().Start
	if len(decorators) > 0 {
		start = decorators[0].Pos()
	}
	p.expect(token.CLASS)
	name, _ := p.expect(token.NAME)
	cls := &ast.ClassDef{
		Decorators: decorators,
		Name:       name.Value,
		NamePos:    name.Start,
	}
	if p.at(token.LPAREN) {
		p.advance()
		cls.Bases, cls.Keywords = p.parseCallArguments()
		p.expect(token.RPAREN)
	}
	p.expect(token.COLON)
	cls.Body = p.parseSuite()
	cls.Span = p.spanFrom(start)
	return cls
}

func (p *Parser) parseParameters(closer token.Type, allowAnnotations bool) *ast.Parameters {
	start := p.cur().Start
	params := &ast.Parameters{}
	for !p.at(closer) && !p.at(token.EOF) && !p.at(token.COLON) && !p.at(token.NEWLINE) {
		pstart := p.cur().Start
		var param *ast.Param
		switch p.cur().Type {
		case token.STAR:
			p.advance()
			if p.at(token.COMMA) || p.at(closer) {
				param = &ast.Param{Kind: ast.ParamStarSep}
			} else {
				name, _ := p.expect(token.NAME)
				param = &ast.Param{Name: name.Value, NamePos: name.Start, Kind: ast.ParamVararg}
				if allowAnnotations && p.at(token.COLON) {
					p.advance()
					param.Annotation = p.parseTest()
				}
			}
		case token.DOUBLESTAR:
			p.advance()
			name, _ := p.expect(token.NAME)
			param = &ast.Param{Name: name.Value, NamePos: name.Start, Kind: ast.ParamKwarg}
			if allowAnnotations && p.at(token.COLON) {
				p.advance()
				param.Annotation = p.parseTest()
			}
		case token.SLASH:
			p.advance()
			param = &ast.Param{Kind: ast.ParamSlash}
		default:
			name, ok := p.expect(token.NAME)
			if !ok {
				p.advance() // ensure progress
				continue
			}
			param = &ast.Param{Name: name.Value, NamePos: name.Start, Kind: ast.ParamNormal}
			if allowAnnotations && p.at(token.COLON) {
				p.advance()
				param.Annotation = p.parseTest()
			}
			if p.at(token.ASSIGN) {
				p.advance()
				param.Default = p.parseTest()
			}
		}
		param.Span = p.spanFrom(pstart)
		params.Params = append(params.Params, param)
		if !p.at(token.COMMA) {
			break
		}
		p.advance()
	}
	params.Span = p.spanFrom(start)
	return params
}

func (p *Parser) parseIf() *ast.If {
	start := p.cur().Start
	p.expect(token.IF)
	test := p.parseNamedExpr()
	p.expect(token.COLON)
	body := p.parseSuite()
	orelse := p.parseElifElse()
	return &ast.If{Span: p.spanFrom(start), Test: test, Body: body, Orelse: orelse}
}

func (p *Parser) parseElifElse() []ast.Stmt {
	switch p.cur().Type {
	case token.ELIF:
		start := p.cur().Start
		p.advance()
		test := p.parseNamedExpr()
		p.expect(token.COLON)
		body := p.parseSuite()
		orelse := p.parseElifElse()
		return []ast.Stmt{&ast.If{Span: p.spanFrom(start), Test: test, Body: body, Orelse: orelse}}
	case token.ELSE:
		p.advance()
		p.expect(token.COLON)
		return p.parseSuite()
	}
	return nil
}

func (p *Parser) parseWhile() *ast.While {
	start := p.cur().Start
	p.expect(token.WHILE)
	test := p.parseNamedExpr()
	p.expect(token.COLON)
	body := p.parseSuite()
	var orelse []ast.Stmt
	if p.at(token.ELSE) {
		p.advance()
		p.expect(token.COLON)
		orelse = p.parseSuite()
	}
	return &ast.While{Span: p.spanFrom(start), Test: test, Body: body, Orelse: orelse}
}

func (p *Parser) parseFor(async bool) *ast.For {
	start := p.cur().Start
	p.expect(token.FOR)
	target := p.parseTargetList()
	p.expect(token.IN)
	iter := p.parseExprList()
	p.expect(token.COLON)
	body := p.parseSuite()
	var orelse []ast.Stmt
	if p.at(token.ELSE) {
		p.advance()
		p.expect(token.COLON)
		orelse = p.parseSuite()
	}
	return &ast.For{Span: p.spanFrom(start), Async: async, Target: target, Iter: iter, Body: body, Orelse: orelse}
}

func (p *Parser) parseWith(async bool) *ast.With {
	start := p.cur().Start
	p.expect(token.WITH)
	var items []*ast.WithItem
	for {
		istart := p.cur().Start
		ctx := p.parseTest()
		item := &ast.WithItem{ContextExpr: ctx}
		if p.at(token.AS) {
			p.advance()
			item.OptionalVars = p.parseTargetAtom()
		}
		item.Span = p.spanFrom(istart)
		items = append(items, item)
		if !p.at(token.COMMA) {
			break
		}
		p.advance()
	}
	p.expect(token.COLON)
	body := p.parseSuite()
	return &ast.With{Span: p.spanFrom(start), Async: async, Items: items, Body: body}
}

func (p *Parser) parseTry() *ast.Try {
	start := p.cur().Start
	p.expect(token.TRY)
	p.expect(token.COLON)
	st := &ast.Try{Body: p.parseSuite()}
	for p.at(token.EXCEPT) {
		hstart := p.cur().Start
		p.advance()
		h := &ast.ExceptHandler{}
		if p.at(token.STAR) {
			p.advance()
			h.Star = true
		}
		if !p.at(token.COLON) {
			h.Type = p.parseTest()
			if p.at(token.AS) {
				p.advance()
				name, _ := p.expect(token.NAME)
				h.Name = name.Value
			}
		}
		p.expect(token.COLON)
		h.Body = p.parseSuite()
		h.Span = p.spanFrom(hstart)
		st.Handlers = append(st.Handlers, h)
	}
	if p.at(token.ELSE) {
		p.advance()
		p.expect(token.COLON)
		st.Orelse = p.parseSuite()
	}
	if p.at(token.FINALLY) {
		p.advance()
		p.expect(token.COLON)
		st.Finalbody = p.parseSuite()
	}
	st.Span = p.spanFrom(start)
	return st
}

// parseSuite parses the block that follows a colon, either an indented block or
// a run of simple statements on the same line.
func (p *Parser) parseSuite() []ast.Stmt {
	if p.at(token.NEWLINE) {
		p.advance()
		if _, ok := p.expect(token.INDENT); !ok {
			return nil
		}
		var body []ast.Stmt
		for !p.at(token.DEDENT) && !p.at(token.EOF) {
			if p.at(token.NEWLINE) {
				p.advance()
				continue
			}
			before := p.pos
			p.parseStatementLine(&body)
			if p.pos == before {
				p.advance()
			}
		}
		p.expect(token.DEDENT)
		return body
	}
	var body []ast.Stmt
	p.parseSimpleLine(&body)
	return body
}

// parseSimpleLine parses one or more simple statements separated by semicolons
// and consumes the ending newline.
func (p *Parser) parseSimpleLine(body *[]ast.Stmt) {
	for {
		s := p.parseSimpleStmt()
		if s != nil {
			*body = append(*body, s)
		}
		if p.at(token.SEMICOLON) {
			p.advance()
			if p.at(token.NEWLINE) || p.at(token.EOF) || p.at(token.DEDENT) {
				break
			}
			continue
		}
		break
	}
	p.expectLineEnd()
}

func (p *Parser) expectLineEnd() {
	if p.at(token.NEWLINE) {
		p.advance()
		return
	}
	if p.at(token.EOF) || p.at(token.DEDENT) {
		return
	}
	p.errorTok(p.cur(), "expected end of line, found %s", describe(p.cur()))
	p.synchronize()
}

func (p *Parser) parseSimpleStmt() ast.Stmt {
	switch p.cur().Type {
	case token.PASS:
		s := &ast.Pass{Span: ast.Span{StartPos: p.cur().Start, EndPos: p.cur().End}}
		p.advance()
		return s
	case token.BREAK:
		s := &ast.Break{Span: ast.Span{StartPos: p.cur().Start, EndPos: p.cur().End}}
		p.advance()
		return s
	case token.CONTINUE:
		s := &ast.Continue{Span: ast.Span{StartPos: p.cur().Start, EndPos: p.cur().End}}
		p.advance()
		return s
	case token.RETURN:
		return p.parseReturn()
	case token.RAISE:
		return p.parseRaise()
	case token.DEL:
		return p.parseDel()
	case token.ASSERT:
		return p.parseAssert()
	case token.GLOBAL:
		return p.parseNameList(true)
	case token.NONLOCAL:
		return p.parseNameList(false)
	case token.IMPORT:
		return p.parseImport()
	case token.FROM:
		return p.parseImportFrom()
	default:
		return p.parseExprStatement()
	}
}

func (p *Parser) parseReturn() ast.Stmt {
	start := p.cur().Start
	p.advance()
	var value ast.Expr
	if p.canStartExpr() {
		value = p.parseExprList()
	}
	return &ast.Return{Span: p.spanFrom(start), Value: value}
}

func (p *Parser) parseRaise() ast.Stmt {
	start := p.cur().Start
	p.advance()
	st := &ast.Raise{}
	if p.canStartExpr() {
		st.Exc = p.parseTest()
		if p.at(token.FROM) {
			p.advance()
			st.Cause = p.parseTest()
		}
	}
	st.Span = p.spanFrom(start)
	return st
}

func (p *Parser) parseDel() ast.Stmt {
	start := p.cur().Start
	p.advance()
	targets := p.parseExprListElts()
	return &ast.Delete{Span: p.spanFrom(start), Targets: targets}
}

func (p *Parser) parseAssert() ast.Stmt {
	start := p.cur().Start
	p.advance()
	test := p.parseTest()
	st := &ast.Assert{Test: test}
	if p.at(token.COMMA) {
		p.advance()
		st.Msg = p.parseTest()
	}
	st.Span = p.spanFrom(start)
	return st
}

func (p *Parser) parseNameList(global bool) ast.Stmt {
	start := p.cur().Start
	p.advance()
	var names []string
	for {
		name, ok := p.expect(token.NAME)
		if ok {
			names = append(names, name.Value)
		}
		if !p.at(token.COMMA) {
			break
		}
		p.advance()
	}
	if global {
		return &ast.Global{Span: p.spanFrom(start), Names: names}
	}
	return &ast.Nonlocal{Span: p.spanFrom(start), Names: names}
}

func (p *Parser) parseImport() ast.Stmt {
	start := p.cur().Start
	p.advance()
	st := &ast.Import{}
	for {
		astart := p.cur().Start
		name := p.parseDottedName()
		alias := &ast.Alias{Name: name}
		if p.at(token.AS) {
			p.advance()
			as, _ := p.expect(token.NAME)
			alias.AsName = as.Value
		}
		alias.Span = p.spanFrom(astart)
		st.Names = append(st.Names, alias)
		if !p.at(token.COMMA) {
			break
		}
		p.advance()
	}
	st.Span = p.spanFrom(start)
	return st
}

func (p *Parser) parseImportFrom() ast.Stmt {
	start := p.cur().Start
	p.advance() // from
	st := &ast.ImportFrom{}
	for p.at(token.DOT) || p.at(token.ELLIPSIS) {
		if p.at(token.ELLIPSIS) {
			st.Level += 3
		} else {
			st.Level++
		}
		p.advance()
	}
	if p.at(token.NAME) {
		st.Module = p.parseDottedName()
	}
	p.expect(token.IMPORT)
	if p.at(token.STAR) {
		p.advance()
		st.Star = true
	} else if p.at(token.LPAREN) {
		p.advance()
		st.Names = p.parseImportAsNames()
		p.expect(token.RPAREN)
	} else {
		st.Names = p.parseImportAsNames()
	}
	st.Span = p.spanFrom(start)
	return st
}

func (p *Parser) parseImportAsNames() []*ast.Alias {
	var names []*ast.Alias
	for p.at(token.NAME) {
		astart := p.cur().Start
		name, _ := p.expect(token.NAME)
		alias := &ast.Alias{Name: name.Value}
		if p.at(token.AS) {
			p.advance()
			as, _ := p.expect(token.NAME)
			alias.AsName = as.Value
		}
		alias.Span = p.spanFrom(astart)
		names = append(names, alias)
		if !p.at(token.COMMA) {
			break
		}
		p.advance()
	}
	return names
}

func (p *Parser) parseDottedName() string {
	name, _ := p.expect(token.NAME)
	result := name.Value
	for p.at(token.DOT) {
		p.advance()
		part, _ := p.expect(token.NAME)
		result += "." + part.Value
	}
	return result
}

// parseExprStatement handles assignments, augmented assignments, annotated
// assignments, and bare expression statements.
func (p *Parser) parseExprStatement() ast.Stmt {
	start := p.cur().Start
	first := p.parseExprList()

	if p.at(token.COLON) {
		p.advance()
		annotation := p.parseTest()
		st := &ast.AnnAssign{Target: first, Annotation: annotation}
		if p.at(token.ASSIGN) {
			p.advance()
			st.Value = p.parseExprListOrYield()
		}
		st.Span = p.spanFrom(start)
		return st
	}

	if op, ok := augAssignOp(p.cur().Type); ok {
		p.advance()
		value := p.parseExprListOrYield()
		return &ast.AugAssign{Span: p.spanFrom(start), Target: first, Op: op, Value: value}
	}

	if p.at(token.ASSIGN) {
		exprs := []ast.Expr{first}
		for p.at(token.ASSIGN) {
			p.advance()
			exprs = append(exprs, p.parseExprListOrYield())
		}
		value := exprs[len(exprs)-1]
		targets := exprs[:len(exprs)-1]
		return &ast.Assign{Span: p.spanFrom(start), Targets: targets, Value: value}
	}

	return &ast.ExprStmt{Span: p.spanFrom(start), Value: first}
}

func augAssignOp(t token.Type) (token.Type, bool) {
	switch t {
	case token.PLUSEQ, token.MINUSEQ, token.STAREQ, token.SLASHEQ, token.PERCENTEQ,
		token.ATEQ, token.AMPEREQ, token.PIPEEQ, token.CARETEQ, token.LSHIFTEQ,
		token.RSHIFTEQ, token.DOUBLESTAREQ, token.DOUBLESLASHEQ:
		return t, true
	}
	return 0, false
}

func (p *Parser) parseExprListOrYield() ast.Expr {
	if p.at(token.YIELD) {
		return p.parseYield()
	}
	return p.parseExprList()
}

// parseExprList parses a comma-separated list of starred expressions and
// returns a single expression or a Tuple.
func (p *Parser) parseExprList() ast.Expr {
	start := p.cur().Start
	first := p.parseStarNamedExpr()
	if !p.at(token.COMMA) {
		return first
	}
	elts := []ast.Expr{first}
	for p.at(token.COMMA) {
		p.advance()
		if !p.canStartExpr() {
			break
		}
		elts = append(elts, p.parseStarNamedExpr())
	}
	return &ast.Tuple{Span: p.spanFrom(start), Elts: elts}
}

// parseExprListElts parses a comma-separated list and always returns the raw
// element slice, used by del.
func (p *Parser) parseExprListElts() []ast.Expr {
	elts := []ast.Expr{p.parseStarNamedExpr()}
	for p.at(token.COMMA) {
		p.advance()
		if !p.canStartExpr() {
			break
		}
		elts = append(elts, p.parseStarNamedExpr())
	}
	return elts
}

// parseTargetList parses assignment or loop targets up to an in or equals.
func (p *Parser) parseTargetList() ast.Expr {
	start := p.cur().Start
	first := p.parseTargetAtom()
	if !p.at(token.COMMA) {
		return first
	}
	elts := []ast.Expr{first}
	for p.at(token.COMMA) {
		p.advance()
		if p.at(token.IN) || p.at(token.ASSIGN) || p.at(token.COLON) || !p.canStartExpr() {
			break
		}
		elts = append(elts, p.parseTargetAtom())
	}
	return &ast.Tuple{Span: p.spanFrom(start), Elts: elts}
}

// parseTargetAtom parses a single assignment or loop target. Targets are
// primaries (names, attributes, subscripts), parenthesized or bracketed groups,
// or starred targets. Parsing at the primary level is important so that the in
// keyword of a for clause is not mistaken for a membership comparison.
func (p *Parser) parseTargetAtom() ast.Expr {
	if p.at(token.STAR) {
		start := p.cur().Start
		p.advance()
		value := p.parseTargetAtom()
		return &ast.Starred{Span: p.spanFrom(start), Value: value}
	}
	return p.parseAtomTrailers()
}

func (p *Parser) parseStarNamedExpr() ast.Expr {
	if p.at(token.STAR) {
		start := p.cur().Start
		p.advance()
		value := p.parseOrTest()
		return &ast.Starred{Span: p.spanFrom(start), Value: value}
	}
	return p.parseNamedExpr()
}

// parseNamedExpr adds walrus assignment on top of a test.
func (p *Parser) parseNamedExpr() ast.Expr {
	start := p.cur().Start
	e := p.parseTest()
	if p.at(token.WALRUS) {
		p.advance()
		value := p.parseTest()
		return &ast.NamedExpr{Span: p.spanFrom(start), Target: e, Value: value}
	}
	return e
}

// Expression grammar, from lowest to highest precedence.

func (p *Parser) parseTest() ast.Expr {
	if p.at(token.LAMBDA) {
		return p.parseLambda()
	}
	start := p.cur().Start
	e := p.parseOrTest()
	if p.at(token.IF) {
		p.advance()
		cond := p.parseOrTest()
		p.expect(token.ELSE)
		orelse := p.parseTest()
		return &ast.IfExp{Span: p.spanFrom(start), Body: e, Test: cond, Orelse: orelse}
	}
	return e
}

func (p *Parser) parseLambda() ast.Expr {
	start := p.cur().Start
	p.expect(token.LAMBDA)
	params := p.parseParameters(token.COLON, false)
	p.expect(token.COLON)
	body := p.parseTest()
	return &ast.Lambda{Span: p.spanFrom(start), Params: params, Body: body}
}

func (p *Parser) parseOrTest() ast.Expr {
	start := p.cur().Start
	left := p.parseAndTest()
	if !p.at(token.OR) {
		return left
	}
	values := []ast.Expr{left}
	for p.at(token.OR) {
		p.advance()
		values = append(values, p.parseAndTest())
	}
	return &ast.BoolOp{Span: p.spanFrom(start), Op: token.OR, Values: values}
}

func (p *Parser) parseAndTest() ast.Expr {
	start := p.cur().Start
	left := p.parseNotTest()
	if !p.at(token.AND) {
		return left
	}
	values := []ast.Expr{left}
	for p.at(token.AND) {
		p.advance()
		values = append(values, p.parseNotTest())
	}
	return &ast.BoolOp{Span: p.spanFrom(start), Op: token.AND, Values: values}
}

func (p *Parser) parseNotTest() ast.Expr {
	if p.at(token.NOT) {
		start := p.cur().Start
		p.advance()
		operand := p.parseNotTest()
		return &ast.UnaryOp{Span: p.spanFrom(start), Op: token.NOT, Operand: operand}
	}
	return p.parseComparison()
}

func (p *Parser) parseComparison() ast.Expr {
	start := p.cur().Start
	left := p.parseBitOr()
	var ops []ast.CmpOp
	var comps []ast.Expr
	for p.isCompareOp() {
		ops = append(ops, p.parseCompareOp())
		comps = append(comps, p.parseBitOr())
	}
	if len(ops) == 0 {
		return left
	}
	return &ast.Compare{Span: p.spanFrom(start), Left: left, Ops: ops, Comparators: comps}
}

func (p *Parser) isCompareOp() bool {
	switch p.cur().Type {
	case token.LT, token.GT, token.LE, token.GE, token.EQEQ, token.NEQ, token.IN, token.IS:
		return true
	case token.NOT:
		return p.peek(1).Type == token.IN
	}
	return false
}

func (p *Parser) parseCompareOp() ast.CmpOp {
	switch p.cur().Type {
	case token.LT:
		p.advance()
		return ast.CmpLt
	case token.GT:
		p.advance()
		return ast.CmpGt
	case token.LE:
		p.advance()
		return ast.CmpLte
	case token.GE:
		p.advance()
		return ast.CmpGte
	case token.EQEQ:
		p.advance()
		return ast.CmpEq
	case token.NEQ:
		p.advance()
		return ast.CmpNotEq
	case token.IN:
		p.advance()
		return ast.CmpIn
	case token.IS:
		p.advance()
		if p.at(token.NOT) {
			p.advance()
			return ast.CmpIsNot
		}
		return ast.CmpIs
	case token.NOT:
		p.advance()
		p.expect(token.IN)
		return ast.CmpNotIn
	}
	return ast.CmpEq
}

// leftAssoc parses a left-associative binary operator level.
func (p *Parser) leftAssoc(next func() ast.Expr, ops ...token.Type) ast.Expr {
	start := p.cur().Start
	left := next()
	for p.matchesAny(ops...) {
		op := p.advance().Type
		right := next()
		left = &ast.BinOp{Span: p.spanFrom(start), Left: left, Op: op, Right: right}
	}
	return left
}

func (p *Parser) matchesAny(ops ...token.Type) bool {
	for _, o := range ops {
		if p.at(o) {
			return true
		}
	}
	return false
}

func (p *Parser) parseBitOr() ast.Expr {
	return p.leftAssoc(p.parseBitXor, token.PIPE)
}

func (p *Parser) parseBitXor() ast.Expr {
	return p.leftAssoc(p.parseBitAnd, token.CARET)
}

func (p *Parser) parseBitAnd() ast.Expr {
	return p.leftAssoc(p.parseShift, token.AMPER)
}

func (p *Parser) parseShift() ast.Expr {
	return p.leftAssoc(p.parseArith, token.LSHIFT, token.RSHIFT)
}

func (p *Parser) parseArith() ast.Expr {
	return p.leftAssoc(p.parseTerm, token.PLUS, token.MINUS)
}

func (p *Parser) parseTerm() ast.Expr {
	return p.leftAssoc(p.parseFactor, token.STAR, token.SLASH, token.DOUBLESLASH, token.PERCENT, token.AT)
}

func (p *Parser) parseFactor() ast.Expr {
	if p.matchesAny(token.PLUS, token.MINUS, token.TILDE) {
		start := p.cur().Start
		op := p.advance().Type
		operand := p.parseFactor()
		return &ast.UnaryOp{Span: p.spanFrom(start), Op: op, Operand: operand}
	}
	return p.parsePower()
}

func (p *Parser) parsePower() ast.Expr {
	start := p.cur().Start
	base := p.parseAwaitUnary()
	if p.at(token.DOUBLESTAR) {
		p.advance()
		exp := p.parseFactor()
		return &ast.BinOp{Span: p.spanFrom(start), Left: base, Op: token.DOUBLESTAR, Right: exp}
	}
	return base
}

func (p *Parser) parseAwaitUnary() ast.Expr {
	if p.at(token.AWAIT) {
		start := p.cur().Start
		p.advance()
		value := p.parseAtomTrailers()
		return &ast.Await{Span: p.spanFrom(start), Value: value}
	}
	return p.parseAtomTrailers()
}

func (p *Parser) parseAtomTrailers() ast.Expr {
	start := p.cur().Start
	e := p.parseAtom()
	for {
		switch p.cur().Type {
		case token.DOT:
			p.advance()
			attr, _ := p.expect(token.NAME)
			e = &ast.Attribute{Span: p.spanFrom(start), Value: e, Attr: attr.Value, AttrPos: attr.Start}
		case token.LPAREN:
			p.advance()
			args, keywords := p.parseCallArguments()
			p.expect(token.RPAREN)
			e = &ast.Call{Span: p.spanFrom(start), Func: e, Args: args, Keywords: keywords}
		case token.LBRACKET:
			p.advance()
			slice := p.parseSubscript()
			p.expect(token.RBRACKET)
			e = &ast.Subscript{Span: p.spanFrom(start), Value: e, Slice: slice}
		default:
			return e
		}
	}
}

func (p *Parser) parseCallArguments() ([]ast.Expr, []*ast.Keyword) {
	var args []ast.Expr
	var keywords []*ast.Keyword
	firstArg := true
	for !p.at(token.RPAREN) && !p.at(token.EOF) {
		switch {
		case p.at(token.STAR):
			start := p.cur().Start
			p.advance()
			value := p.parseTest()
			args = append(args, &ast.Starred{Span: p.spanFrom(start), Value: value})
		case p.at(token.DOUBLESTAR):
			kstart := p.cur().Start
			p.advance()
			value := p.parseTest()
			keywords = append(keywords, &ast.Keyword{Span: p.spanFrom(kstart), Value: value})
		default:
			kstart := p.cur().Start
			e := p.parseNamedExpr()
			if firstArg && (p.at(token.FOR) || (p.at(token.ASYNC) && p.peek(1).Type == token.FOR)) {
				gens := p.parseComprehensions()
				args = append(args, &ast.GeneratorExp{Span: p.spanFrom(kstart), Elt: e, Generators: gens})
			} else if p.at(token.ASSIGN) {
				p.advance()
				value := p.parseTest()
				kw := &ast.Keyword{Span: p.spanFrom(kstart), Value: value}
				if name, ok := e.(*ast.Name); ok {
					kw.Arg = name.Id
				} else {
					p.errorf(e.Pos(), "keyword argument name must be an identifier")
				}
				keywords = append(keywords, kw)
			} else {
				args = append(args, e)
			}
		}
		firstArg = false
		if !p.at(token.COMMA) {
			break
		}
		p.advance()
	}
	return args, keywords
}

func (p *Parser) parseSubscript() ast.Expr {
	start := p.cur().Start
	first := p.parseSubscriptItem()
	if !p.at(token.COMMA) {
		return first
	}
	elts := []ast.Expr{first}
	for p.at(token.COMMA) {
		p.advance()
		if p.at(token.RBRACKET) {
			break
		}
		elts = append(elts, p.parseSubscriptItem())
	}
	return &ast.Tuple{Span: p.spanFrom(start), Elts: elts}
}

func (p *Parser) parseSubscriptItem() ast.Expr {
	start := p.cur().Start
	var lower ast.Expr
	if !p.at(token.COLON) {
		lower = p.parseTest()
	}
	if !p.at(token.COLON) {
		return lower
	}
	// It is a slice.
	p.advance() // first colon
	sl := &ast.Slice{Lower: lower}
	if !p.at(token.COLON) && !p.at(token.RBRACKET) && !p.at(token.COMMA) {
		sl.Upper = p.parseTest()
	}
	if p.at(token.COLON) {
		p.advance()
		if !p.at(token.RBRACKET) && !p.at(token.COMMA) {
			sl.Step = p.parseTest()
		}
	}
	sl.Span = p.spanFrom(start)
	return sl
}

func (p *Parser) parseAtom() ast.Expr {
	t := p.cur()
	switch t.Type {
	case token.NAME:
		p.advance()
		return &ast.Name{Span: ast.Span{StartPos: t.Start, EndPos: t.End}, Id: t.Value}
	case token.NUMBER:
		p.advance()
		return &ast.Number{Span: ast.Span{StartPos: t.Start, EndPos: t.End}, Value: t.Value}
	case token.STRING:
		start := t.Start
		var values []string
		for p.at(token.STRING) {
			values = append(values, p.cur().Value)
			p.advance()
		}
		return &ast.Str{Span: p.spanFrom(start), Values: values}
	case token.TRUE, token.FALSE, token.NONE:
		p.advance()
		return &ast.Constant{Span: ast.Span{StartPos: t.Start, EndPos: t.End}, Value: t.Type.String()}
	case token.ELLIPSIS:
		p.advance()
		return &ast.Constant{Span: ast.Span{StartPos: t.Start, EndPos: t.End}, Value: "..."}
	case token.LPAREN:
		return p.parseParenAtom()
	case token.LBRACKET:
		return p.parseListAtom()
	case token.LBRACE:
		return p.parseBraceAtom()
	case token.YIELD:
		return p.parseYield()
	default:
		p.errorTok(t, "expected an expression, found %s", describe(t))
		p.advance()
		return &ast.BadExpr{Span: ast.Span{StartPos: t.Start, EndPos: t.End}}
	}
}

func (p *Parser) parseParenAtom() ast.Expr {
	start := p.cur().Start
	p.advance() // (
	if p.at(token.RPAREN) {
		p.advance()
		return &ast.Tuple{Span: p.spanFrom(start), Elts: nil}
	}
	if p.at(token.YIELD) {
		y := p.parseYield()
		p.expect(token.RPAREN)
		return y
	}
	first := p.parseStarNamedExpr()
	if p.at(token.FOR) || (p.at(token.ASYNC) && p.peek(1).Type == token.FOR) {
		gens := p.parseComprehensions()
		p.expect(token.RPAREN)
		return &ast.GeneratorExp{Span: p.spanFrom(start), Elt: first, Generators: gens}
	}
	if p.at(token.COMMA) {
		elts := []ast.Expr{first}
		for p.at(token.COMMA) {
			p.advance()
			if p.at(token.RPAREN) {
				break
			}
			elts = append(elts, p.parseStarNamedExpr())
		}
		p.expect(token.RPAREN)
		return &ast.Tuple{Span: p.spanFrom(start), Elts: elts}
	}
	p.expect(token.RPAREN)
	return first
}

func (p *Parser) parseListAtom() ast.Expr {
	start := p.cur().Start
	p.advance() // [
	if p.at(token.RBRACKET) {
		p.advance()
		return &ast.List{Span: p.spanFrom(start), Elts: nil}
	}
	first := p.parseStarNamedExpr()
	if p.at(token.FOR) || (p.at(token.ASYNC) && p.peek(1).Type == token.FOR) {
		gens := p.parseComprehensions()
		p.expect(token.RBRACKET)
		return &ast.ListComp{Span: p.spanFrom(start), Elt: first, Generators: gens}
	}
	elts := []ast.Expr{first}
	for p.at(token.COMMA) {
		p.advance()
		if p.at(token.RBRACKET) {
			break
		}
		elts = append(elts, p.parseStarNamedExpr())
	}
	p.expect(token.RBRACKET)
	return &ast.List{Span: p.spanFrom(start), Elts: elts}
}

func (p *Parser) parseBraceAtom() ast.Expr {
	start := p.cur().Start
	p.advance() // {
	if p.at(token.RBRACE) {
		p.advance()
		return &ast.Dict{Span: p.spanFrom(start)}
	}

	// Double-star unpacking forces a dict.
	if p.at(token.DOUBLESTAR) {
		return p.parseDictBody(start, nil, nil)
	}

	if p.at(token.STAR) {
		// Starred element forces a set.
		return p.parseSetBody(start, p.parseStarNamedExpr())
	}

	first := p.parseNamedExpr()
	if p.at(token.COLON) {
		p.advance()
		value := p.parseTest()
		if p.at(token.FOR) || (p.at(token.ASYNC) && p.peek(1).Type == token.FOR) {
			gens := p.parseComprehensions()
			p.expect(token.RBRACE)
			return &ast.DictComp{Span: p.spanFrom(start), Key: first, Value: value, Generators: gens}
		}
		return p.parseDictBody(start, first, value)
	}
	if p.at(token.FOR) || (p.at(token.ASYNC) && p.peek(1).Type == token.FOR) {
		gens := p.parseComprehensions()
		p.expect(token.RBRACE)
		return &ast.SetComp{Span: p.spanFrom(start), Elt: first, Generators: gens}
	}
	return p.parseSetBody(start, first)
}

func (p *Parser) parseDictBody(start token.Position, firstKey, firstValue ast.Expr) ast.Expr {
	d := &ast.Dict{}
	appendPair := func(k, v ast.Expr) {
		d.Keys = append(d.Keys, k)
		d.Values = append(d.Values, v)
	}
	if firstKey != nil || firstValue != nil {
		appendPair(firstKey, firstValue)
		if p.at(token.COMMA) {
			p.advance()
		}
	}
	for !p.at(token.RBRACE) && !p.at(token.EOF) {
		if p.at(token.DOUBLESTAR) {
			p.advance()
			appendPair(nil, p.parseOrTest())
		} else {
			k := p.parseTest()
			p.expect(token.COLON)
			v := p.parseTest()
			appendPair(k, v)
		}
		if !p.at(token.COMMA) {
			break
		}
		p.advance()
	}
	p.expect(token.RBRACE)
	d.Span = p.spanFrom(start)
	return d
}

func (p *Parser) parseSetBody(start token.Position, first ast.Expr) ast.Expr {
	elts := []ast.Expr{first}
	for p.at(token.COMMA) {
		p.advance()
		if p.at(token.RBRACE) {
			break
		}
		elts = append(elts, p.parseStarNamedExpr())
	}
	p.expect(token.RBRACE)
	return &ast.Set{Span: p.spanFrom(start), Elts: elts}
}

func (p *Parser) parseComprehensions() []*ast.Comprehension {
	var gens []*ast.Comprehension
	for p.at(token.FOR) || (p.at(token.ASYNC) && p.peek(1).Type == token.FOR) {
		cstart := p.cur().Start
		async := false
		if p.at(token.ASYNC) {
			async = true
			p.advance()
		}
		p.expect(token.FOR)
		target := p.parseTargetList()
		p.expect(token.IN)
		iter := p.parseOrTest()
		gen := &ast.Comprehension{Target: target, Iter: iter, Async: async}
		for p.at(token.IF) {
			p.advance()
			gen.Ifs = append(gen.Ifs, p.parseOrTest())
		}
		gen.Span = p.spanFrom(cstart)
		gens = append(gens, gen)
	}
	return gens
}

func (p *Parser) parseYield() ast.Expr {
	start := p.cur().Start
	p.expect(token.YIELD)
	if p.at(token.FROM) {
		p.advance()
		value := p.parseTest()
		return &ast.YieldFrom{Span: p.spanFrom(start), Value: value}
	}
	if p.canStartExpr() {
		value := p.parseExprList()
		return &ast.Yield{Span: p.spanFrom(start), Value: value}
	}
	return &ast.Yield{Span: p.spanFrom(start)}
}

// canStartExpr reports whether the current token can begin an expression.
func (p *Parser) canStartExpr() bool {
	switch p.cur().Type {
	case token.NAME, token.NUMBER, token.STRING, token.TRUE, token.FALSE, token.NONE,
		token.ELLIPSIS, token.LPAREN, token.LBRACKET, token.LBRACE, token.PLUS,
		token.MINUS, token.TILDE, token.NOT, token.LAMBDA, token.AWAIT, token.STAR,
		token.YIELD:
		return true
	}
	return false
}
