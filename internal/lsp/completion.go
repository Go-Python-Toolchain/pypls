package lsp

import (
	"sort"

	"github.com/Go-Python-Toolchain/pypls/internal/ast"
	"github.com/Go-Python-Toolchain/pypls/internal/parser"
)

// pythonKeywords are always offered as completions.
var pythonKeywords = []string{
	"False", "None", "True", "and", "as", "assert", "async", "await", "break",
	"class", "continue", "def", "del", "elif", "else", "except", "finally",
	"for", "from", "global", "if", "import", "in", "is", "lambda", "nonlocal",
	"not", "or", "pass", "raise", "return", "try", "while", "with", "yield",
}

// completionsFor returns keyword completions plus the names defined in the
// document: functions, classes, assigned variables, and parameters.
func completionsFor(text string) []completionItem {
	items := make([]completionItem, 0, len(pythonKeywords)+16)
	for _, kw := range pythonKeywords {
		items = append(items, completionItem{Label: kw, Kind: kindKeyword})
	}

	mod, _ := parser.Parse("", text)
	if mod != nil {
		seen := map[string]bool{}
		for _, name := range collectNames(mod) {
			if seen[name.label] {
				continue
			}
			seen[name.label] = true
			items = append(items, completionItem{Label: name.label, Kind: name.kind})
		}
	}

	sort.Slice(items, func(i, j int) bool { return items[i].Label < items[j].Label })
	return items
}

type namedSymbol struct {
	label string
	kind  int
}

// collectNames gathers declared names from the module tree.
func collectNames(mod *ast.Module) []namedSymbol {
	var out []namedSymbol
	var visitStmts func(stmts []ast.Stmt)
	addTargets := func(targets []ast.Expr) {
		for _, t := range targets {
			collectTargetNames(t, &out)
		}
	}

	visitStmts = func(stmts []ast.Stmt) {
		for _, s := range stmts {
			switch st := s.(type) {
			case *ast.FunctionDef:
				out = append(out, namedSymbol{label: st.Name, kind: kindFunction})
				if st.Params != nil {
					for _, p := range st.Params.Params {
						if p.Name != "" {
							out = append(out, namedSymbol{label: p.Name, kind: kindVariable})
						}
					}
				}
				visitStmts(st.Body)
			case *ast.ClassDef:
				out = append(out, namedSymbol{label: st.Name, kind: kindClass})
				visitStmts(st.Body)
			case *ast.Assign:
				addTargets(st.Targets)
			case *ast.AnnAssign:
				collectTargetNames(st.Target, &out)
			case *ast.If:
				visitStmts(st.Body)
				visitStmts(st.Orelse)
			case *ast.For:
				collectTargetNames(st.Target, &out)
				visitStmts(st.Body)
				visitStmts(st.Orelse)
			case *ast.While:
				visitStmts(st.Body)
				visitStmts(st.Orelse)
			case *ast.With:
				visitStmts(st.Body)
			case *ast.Try:
				visitStmts(st.Body)
				for _, h := range st.Handlers {
					visitStmts(h.Body)
				}
				visitStmts(st.Orelse)
				visitStmts(st.Finalbody)
			}
		}
	}
	visitStmts(mod.Body)
	return out
}

// collectTargetNames extracts plain names from an assignment target.
func collectTargetNames(target ast.Expr, out *[]namedSymbol) {
	switch t := target.(type) {
	case *ast.Name:
		*out = append(*out, namedSymbol{label: t.Id, kind: kindVariable})
	case *ast.Tuple:
		for _, e := range t.Elts {
			collectTargetNames(e, out)
		}
	case *ast.List:
		for _, e := range t.Elts {
			collectTargetNames(e, out)
		}
	case *ast.Starred:
		collectTargetNames(t.Value, out)
	}
}
