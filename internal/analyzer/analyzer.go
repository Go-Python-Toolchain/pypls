// Package analyzer runs the parsing pipeline over a source file and returns
// diagnostics. For now it reports syntax problems. Later layers add type
// diagnostics on top of the same entry point.
package analyzer

import (
	"sort"

	"github.com/Go-Python-Toolchain/pypls/internal/checker"
	"github.com/Go-Python-Toolchain/pypls/internal/diagnostic"
	"github.com/Go-Python-Toolchain/pypls/internal/parser"
)

// sourceName labels every diagnostic so an editor can group problems by tool.
const sourceName = "pypls"

// Check parses source and returns all diagnostics, ordered by position. It
// reports syntax problems from the parser and type problems from the checker.
func Check(file, source string) []diagnostic.Diagnostic {
	mod, errs := parser.Parse(file, source)

	diags := make([]diagnostic.Diagnostic, 0, len(errs))
	for _, e := range errs {
		diags = append(diags, diagnostic.Diagnostic{
			Range:    diagnostic.Range{Start: e.Start, End: e.End},
			Severity: diagnostic.SeverityError,
			Code:     "syntax",
			Source:   sourceName,
			Message:  e.Msg,
		})
	}

	if mod != nil {
		diags = append(diags, checker.Check(mod)...)
	}

	sort.SliceStable(diags, func(i, j int) bool {
		a, b := diags[i].Range.Start, diags[j].Range.Start
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		return a.Column < b.Column
	})
	return diags
}
