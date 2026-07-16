// Package checker performs local type inference over a parsed module and
// reports type problems. Inference is intraprocedural for now: it reasons from
// literals, assignments, annotations, and a handful of builtin constructors. It
// never resolves imports or cross-module types yet. The guiding rule is to stay
// silent whenever a type is not known, so that untyped code produces no noise.
package checker

import (
	"fmt"
	"strings"

	"github.com/Go-Python-Toolchain/pypls/internal/ast"
	"github.com/Go-Python-Toolchain/pypls/internal/diagnostic"
	"github.com/Go-Python-Toolchain/pypls/internal/token"
	"github.com/Go-Python-Toolchain/pypls/internal/types"
)

const sourceName = "pypls"

// Checker holds inference state for one module.
type Checker struct {
	diags  []diagnostic.Diagnostic
	scopes []map[string]*types.Type
}

// Check infers types across the module and returns any type diagnostics.
func Check(mod *ast.Module) []diagnostic.Diagnostic {
	c := &Checker{}
	c.push()
	c.walk(mod.Body)
	c.pop()
	return c.diags
}

// ModuleScope builds the module-level name bindings without reporting
// diagnostics and without descending into function or class bodies. Incremental
// analysis uses it to give each unit the same module context it would have in a
// whole-module check.
func ModuleScope(mod *ast.Module) map[string]*types.Type {
	c := &Checker{}
	c.push()
	for _, s := range mod.Body {
		c.bindTopLevel(s)
	}
	return c.scopes[0]
}

func (c *Checker) bindTopLevel(s ast.Stmt) {
	switch st := s.(type) {
	case *ast.Assign:
		vt := c.infer(st.Value)
		for _, target := range st.Targets {
			c.assignTarget(target, vt)
		}
	case *ast.AnnAssign:
		if name, ok := st.Target.(*ast.Name); ok {
			c.define(name.Id, c.annType(st.Annotation))
		}
	case *ast.FunctionDef:
		c.define(st.Name, &types.Type{Kind: types.Callable, Name: st.Name})
	case *ast.ClassDef:
		c.define(st.Name, &types.Type{Kind: types.Callable, Name: st.Name})
	}
}

// CheckUnit type-checks a single top-level statement using the given module
// scope as context. It is the unit of work for incremental analysis.
func CheckUnit(unit ast.Stmt, scope map[string]*types.Type) []diagnostic.Diagnostic {
	c := &Checker{}
	c.push()
	for k, v := range scope {
		c.scopes[0][k] = v
	}
	c.checkStmt(unit)
	return c.diags
}

func (c *Checker) push() { c.scopes = append(c.scopes, map[string]*types.Type{}) }
func (c *Checker) pop()  { c.scopes = c.scopes[:len(c.scopes)-1] }

func (c *Checker) define(name string, t *types.Type) {
	if len(c.scopes) == 0 {
		return
	}
	c.scopes[len(c.scopes)-1][name] = t
}

func (c *Checker) lookup(name string) *types.Type {
	for i := len(c.scopes) - 1; i >= 0; i-- {
		if t, ok := c.scopes[i][name]; ok {
			return t
		}
	}
	return types.Any
}

func (c *Checker) report(sev diagnostic.Severity, code string, r diagnostic.Range, format string, args ...any) {
	c.diags = append(c.diags, diagnostic.Diagnostic{
		Range:    r,
		Severity: sev,
		Code:     code,
		Source:   sourceName,
		Message:  fmt.Sprintf(format, args...),
	})
}

func rangeOf(n ast.Node) diagnostic.Range {
	return diagnostic.Range{Start: n.Pos(), End: n.End()}
}

// Statement walking.

func (c *Checker) walk(stmts []ast.Stmt) {
	for _, s := range stmts {
		c.checkStmt(s)
	}
}

func (c *Checker) checkStmt(s ast.Stmt) {
	switch st := s.(type) {
	case *ast.Assign:
		vt := c.infer(st.Value)
		for _, target := range st.Targets {
			c.assignTarget(target, vt)
		}
	case *ast.AnnAssign:
		at := c.annType(st.Annotation)
		if st.Value != nil {
			vt := c.infer(st.Value)
			if !types.Assignable(vt, at) {
				c.report(diagnostic.SeverityWarning, "type-mismatch", rangeOf(st.Value),
					"value of type %s is not assignable to %s", vt, at)
			}
		}
		if name, ok := st.Target.(*ast.Name); ok {
			c.define(name.Id, at)
		}
	case *ast.AugAssign:
		lt := c.infer(st.Target)
		rt := c.infer(st.Value)
		base := augToBase(st.Op)
		if res, ok := binOpResult(base, lt, rt); !ok {
			c.report(diagnostic.SeverityError, "operand", rangeOf(st.Value),
				"unsupported operand type(s) for %s: %s and %s", base, lt, rt)
		} else if name, ok := st.Target.(*ast.Name); ok {
			c.define(name.Id, res)
		}
	case *ast.ExprStmt:
		c.infer(st.Value)
	case *ast.Return:
		if st.Value != nil {
			c.infer(st.Value)
		}
	case *ast.FunctionDef:
		c.define(st.Name, &types.Type{Kind: types.Callable, Name: st.Name})
		c.push()
		c.bindParams(st.Params)
		c.walk(st.Body)
		c.pop()
	case *ast.ClassDef:
		c.define(st.Name, &types.Type{Kind: types.Callable, Name: st.Name})
		c.push()
		c.walk(st.Body)
		c.pop()
	case *ast.If:
		c.infer(st.Test)
		c.walk(st.Body)
		c.walk(st.Orelse)
	case *ast.While:
		c.infer(st.Test)
		c.walk(st.Body)
		c.walk(st.Orelse)
	case *ast.For:
		c.infer(st.Iter)
		c.assignTarget(st.Target, elementType(c.infer(st.Iter)))
		c.walk(st.Body)
		c.walk(st.Orelse)
	case *ast.With:
		for _, item := range st.Items {
			c.infer(item.ContextExpr)
			if item.OptionalVars != nil {
				c.assignTarget(item.OptionalVars, types.Any)
			}
		}
		c.walk(st.Body)
	case *ast.Try:
		c.walk(st.Body)
		for _, h := range st.Handlers {
			if h.Name != "" {
				c.define(h.Name, types.Any)
			}
			c.walk(h.Body)
		}
		c.walk(st.Orelse)
		c.walk(st.Finalbody)
	case *ast.Assert:
		c.infer(st.Test)
		if st.Msg != nil {
			c.infer(st.Msg)
		}
	case *ast.Delete:
		for _, t := range st.Targets {
			c.infer(t)
		}
	}
}

func (c *Checker) bindParams(params *ast.Parameters) {
	if params == nil {
		return
	}
	for _, prm := range params.Params {
		if prm.Name == "" {
			continue
		}
		if prm.Annotation != nil {
			c.define(prm.Name, c.annType(prm.Annotation))
		} else {
			c.define(prm.Name, types.Any)
		}
	}
}

func (c *Checker) assignTarget(target ast.Expr, t *types.Type) {
	switch tg := target.(type) {
	case *ast.Name:
		c.define(tg.Id, t)
	case *ast.Tuple:
		c.bindSequence(tg.Elts, t)
	case *ast.List:
		c.bindSequence(tg.Elts, t)
	case *ast.Starred:
		c.assignTarget(tg.Value, types.Any)
	}
}

func (c *Checker) bindSequence(elts []ast.Expr, t *types.Type) {
	if t != nil && t.Kind == types.Tuple && len(t.Elems) == len(elts) {
		for i, e := range elts {
			c.assignTarget(e, t.Elems[i])
		}
		return
	}
	for _, e := range elts {
		c.assignTarget(e, types.Any)
	}
}

// Expression inference.

func (c *Checker) infer(e ast.Expr) *types.Type {
	switch ex := e.(type) {
	case *ast.Number:
		return classifyNumber(ex.Value)
	case *ast.Str:
		if len(ex.Values) > 0 && stringLiteralIsBytes(ex.Values[0]) {
			return types.Basic(types.Bytes)
		}
		return types.Basic(types.Str)
	case *ast.Constant:
		switch ex.Value {
		case "True", "False":
			return types.Basic(types.Bool)
		case "None":
			return types.Basic(types.None)
		default:
			return types.Basic(types.Ellipsis)
		}
	case *ast.Name:
		return c.lookup(ex.Id)
	case *ast.List:
		return &types.Type{Kind: types.List, Elem: c.joinElts(ex.Elts)}
	case *ast.Set:
		return &types.Type{Kind: types.Set, Elem: c.joinElts(ex.Elts)}
	case *ast.Dict:
		return c.inferDict(ex)
	case *ast.Tuple:
		elems := make([]*types.Type, 0, len(ex.Elts))
		for _, el := range ex.Elts {
			elems = append(elems, c.infer(el))
		}
		return &types.Type{Kind: types.Tuple, Elems: elems}
	case *ast.BinOp:
		lt := c.infer(ex.Left)
		rt := c.infer(ex.Right)
		res, ok := binOpResult(ex.Op, lt, rt)
		if !ok {
			c.report(diagnostic.SeverityError, "operand", rangeOf(ex),
				"unsupported operand type(s) for %s: %s and %s", ex.Op, lt, rt)
		}
		return res
	case *ast.UnaryOp:
		operand := c.infer(ex.Operand)
		if ex.Op == token.NOT {
			return types.Basic(types.Bool)
		}
		if operand.IsNumeric() {
			return operand
		}
		return types.Any
	case *ast.BoolOp:
		for _, v := range ex.Values {
			c.infer(v)
		}
		return types.Any
	case *ast.Compare:
		c.infer(ex.Left)
		for _, comp := range ex.Comparators {
			c.infer(comp)
		}
		return types.Basic(types.Bool)
	case *ast.Call:
		return c.inferCall(ex)
	case *ast.IfExp:
		bt := c.infer(ex.Body)
		c.infer(ex.Test)
		ot := c.infer(ex.Orelse)
		return types.Join(bt, ot)
	case *ast.NamedExpr:
		vt := c.infer(ex.Value)
		c.assignTarget(ex.Target, vt)
		return vt
	case *ast.Attribute:
		c.infer(ex.Value)
		return types.Any
	case *ast.Subscript:
		c.infer(ex.Value)
		return types.Any
	case *ast.Starred:
		c.infer(ex.Value)
		return types.Any
	case *ast.Await:
		c.infer(ex.Value)
		return types.Any
	case *ast.Yield:
		if ex.Value != nil {
			c.infer(ex.Value)
		}
		return types.Any
	case *ast.YieldFrom:
		c.infer(ex.Value)
		return types.Any
	case *ast.Lambda:
		return &types.Type{Kind: types.Callable}
	}
	return types.Any
}

func (c *Checker) joinElts(elts []ast.Expr) *types.Type {
	var result *types.Type
	for _, el := range elts {
		if _, ok := el.(*ast.Starred); ok {
			return types.Any
		}
		t := c.infer(el)
		if result == nil {
			result = t
		} else {
			result = types.Join(result, t)
		}
	}
	if result == nil {
		return types.Any
	}
	return result
}

func (c *Checker) inferDict(d *ast.Dict) *types.Type {
	var keyT, valT *types.Type
	for i := range d.Values {
		if i < len(d.Keys) && d.Keys[i] != nil {
			kt := c.infer(d.Keys[i])
			if keyT == nil {
				keyT = kt
			} else {
				keyT = types.Join(keyT, kt)
			}
		}
		vt := c.infer(d.Values[i])
		if valT == nil {
			valT = vt
		} else {
			valT = types.Join(valT, vt)
		}
	}
	return &types.Type{Kind: types.Dict, Key: keyT, Value: valT}
}

// builtinConstructors maps builtin callables to the type they produce.
var builtinConstructors = map[string]types.Kind{
	"int": types.Int, "float": types.Float, "complex": types.Complex,
	"bool": types.Bool, "str": types.Str, "bytes": types.Bytes,
	"list": types.List, "dict": types.Dict, "set": types.Set,
	"tuple": types.Tuple, "frozenset": types.Set,
}

func (c *Checker) inferCall(call *ast.Call) *types.Type {
	for _, a := range call.Args {
		c.infer(a)
	}
	for _, kw := range call.Keywords {
		c.infer(kw.Value)
	}
	if name, ok := call.Func.(*ast.Name); ok {
		// Only treat the name as a builtin when it has not been shadowed locally.
		if c.lookup(name.Id).IsUnknown() {
			if k, ok := builtinConstructors[name.Id]; ok {
				return types.Basic(k)
			}
		}
	} else {
		c.infer(call.Func)
	}
	return types.Any
}

// annType resolves a type annotation expression to a type. Unrecognized
// annotations resolve to Unknown so that custom classes never cause noise.
func (c *Checker) annType(e ast.Expr) *types.Type {
	switch ex := e.(type) {
	case *ast.Name:
		return namedType(ex.Id)
	case *ast.Constant:
		if ex.Value == "None" {
			return types.Basic(types.None)
		}
		return types.Any
	case *ast.Subscript:
		return c.annSubscript(ex)
	case *ast.Attribute:
		return types.Any
	}
	return types.Any
}

func (c *Checker) annSubscript(s *ast.Subscript) *types.Type {
	base, ok := s.Value.(*ast.Name)
	if !ok {
		return types.Any
	}
	switch base.Id {
	case "list", "List":
		return &types.Type{Kind: types.List, Elem: c.annType(s.Slice)}
	case "set", "Set", "frozenset", "FrozenSet":
		return &types.Type{Kind: types.Set, Elem: c.annType(s.Slice)}
	case "dict", "Dict":
		if tup, ok := s.Slice.(*ast.Tuple); ok && len(tup.Elts) == 2 {
			return &types.Type{Kind: types.Dict, Key: c.annType(tup.Elts[0]), Value: c.annType(tup.Elts[1])}
		}
		return types.Basic(types.Dict)
	case "tuple", "Tuple":
		if tup, ok := s.Slice.(*ast.Tuple); ok {
			elems := make([]*types.Type, len(tup.Elts))
			for i, el := range tup.Elts {
				elems[i] = c.annType(el)
			}
			return &types.Type{Kind: types.Tuple, Elems: elems}
		}
		return types.Basic(types.Tuple)
	}
	// Optional, Union, and unknown generics resolve to Unknown.
	return types.Any
}

func namedType(id string) *types.Type {
	switch id {
	case "int":
		return types.Basic(types.Int)
	case "float":
		return types.Basic(types.Float)
	case "complex":
		return types.Basic(types.Complex)
	case "bool":
		return types.Basic(types.Bool)
	case "str":
		return types.Basic(types.Str)
	case "bytes", "bytearray":
		return types.Basic(types.Bytes)
	case "list", "List":
		return types.Basic(types.List)
	case "dict", "Dict":
		return types.Basic(types.Dict)
	case "set", "Set", "frozenset", "FrozenSet":
		return types.Basic(types.Set)
	case "tuple", "Tuple":
		return types.Basic(types.Tuple)
	}
	return types.Any
}

// classifyNumber determines the type of a numeric literal from its text.
func classifyNumber(text string) *types.Type {
	lower := strings.ToLower(text)
	if strings.HasPrefix(lower, "0x") || strings.HasPrefix(lower, "0o") || strings.HasPrefix(lower, "0b") {
		return types.Basic(types.Int)
	}
	if strings.HasSuffix(lower, "j") {
		return types.Basic(types.Complex)
	}
	if strings.ContainsAny(lower, ".e") {
		return types.Basic(types.Float)
	}
	return types.Basic(types.Int)
}

// stringLiteralIsBytes reports whether a string literal has a bytes prefix.
func stringLiteralIsBytes(raw string) bool {
	for _, r := range raw {
		if r == '"' || r == '\'' {
			return false
		}
		if r == 'b' || r == 'B' {
			return true
		}
	}
	return false
}

// elementType returns the element type of an iterable, best effort.
func elementType(t *types.Type) *types.Type {
	if t == nil {
		return types.Any
	}
	switch t.Kind {
	case types.List, types.Set:
		if t.Elem != nil {
			return t.Elem
		}
	case types.Dict:
		if t.Key != nil {
			return t.Key
		}
	}
	return types.Any
}

// binOpResult returns the result type of a binary operation and whether the
// operation is valid. It only reports invalidity when both operands have known
// builtin types, so custom classes and unknown values never trigger an error.
func binOpResult(op token.Type, l, r *types.Type) (*types.Type, bool) {
	if l.IsUnknown() || r.IsUnknown() {
		return types.Any, true
	}

	switch op {
	case token.PLUS:
		if l.IsNumeric() && r.IsNumeric() {
			return types.Widen(l, r), true
		}
		if l.Kind == r.Kind {
			switch l.Kind {
			case types.Str, types.Bytes, types.List, types.Tuple:
				return l, true
			}
		}
		return types.Any, false
	case token.MINUS:
		if l.IsNumeric() && r.IsNumeric() {
			return types.Widen(l, r), true
		}
		if l.Kind == types.Set && r.Kind == types.Set {
			return types.Basic(types.Set), true
		}
		return types.Any, false
	case token.STAR:
		if l.IsNumeric() && r.IsNumeric() {
			return types.Widen(l, r), true
		}
		if seq, num := sequenceRepeat(l, r); seq != nil {
			_ = num
			return seq, true
		}
		return types.Any, false
	case token.SLASH:
		if l.IsNumeric() && r.IsNumeric() {
			if l.Kind == types.Complex || r.Kind == types.Complex {
				return types.Basic(types.Complex), true
			}
			return types.Basic(types.Float), true
		}
		return types.Any, false
	case token.DOUBLESLASH, token.PERCENT:
		if l.IsNumeric() && r.IsNumeric() {
			return types.Widen(l, r), true
		}
		if op == token.PERCENT && (l.Kind == types.Str || l.Kind == types.Bytes) {
			return l, true // old-style formatting
		}
		return types.Any, false
	case token.DOUBLESTAR:
		if l.IsNumeric() && r.IsNumeric() {
			return types.Widen(l, r), true
		}
		return types.Any, false
	case token.AMPER, token.PIPE, token.CARET:
		if isIntLike(l) && isIntLike(r) {
			return types.Basic(types.Int), true
		}
		if l.Kind == types.Set && r.Kind == types.Set {
			return types.Basic(types.Set), true
		}
		return types.Any, false
	case token.LSHIFT, token.RSHIFT:
		if isIntLike(l) && isIntLike(r) {
			return types.Basic(types.Int), true
		}
		return types.Any, false
	case token.AT:
		// Matrix multiply is defined by libraries on unknown types, so never
		// flag it here.
		return types.Any, true
	}
	return types.Any, true
}

// augToBase maps an augmented assignment operator to its base binary operator.
func augToBase(op token.Type) token.Type {
	switch op {
	case token.PLUSEQ:
		return token.PLUS
	case token.MINUSEQ:
		return token.MINUS
	case token.STAREQ:
		return token.STAR
	case token.SLASHEQ:
		return token.SLASH
	case token.PERCENTEQ:
		return token.PERCENT
	case token.ATEQ:
		return token.AT
	case token.AMPEREQ:
		return token.AMPER
	case token.PIPEEQ:
		return token.PIPE
	case token.CARETEQ:
		return token.CARET
	case token.LSHIFTEQ:
		return token.LSHIFT
	case token.RSHIFTEQ:
		return token.RSHIFT
	case token.DOUBLESTAREQ:
		return token.DOUBLESTAR
	case token.DOUBLESLASHEQ:
		return token.DOUBLESLASH
	}
	return op
}

func isIntLike(t *types.Type) bool {
	return t != nil && (t.Kind == types.Int || t.Kind == types.Bool)
}

// sequenceRepeat handles sequence times integer, in either order.
func sequenceRepeat(l, r *types.Type) (*types.Type, bool) {
	if isSequence(l) && isIntLike(r) {
		return l, true
	}
	if isIntLike(l) && isSequence(r) {
		return r, true
	}
	return nil, false
}

func isSequence(t *types.Type) bool {
	if t == nil {
		return false
	}
	switch t.Kind {
	case types.Str, types.Bytes, types.List, types.Tuple:
		return true
	}
	return false
}
