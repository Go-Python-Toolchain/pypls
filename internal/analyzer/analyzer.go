// Package analyzer runs the parsing pipeline over a source file and returns
// diagnostics. For now it reports syntax problems. Later layers add type
// diagnostics on top of the same entry point.
package analyzer

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"

	"github.com/Go-Python-Toolchain/pypls/internal/cache"
	"github.com/Go-Python-Toolchain/pypls/internal/checker"
	"github.com/Go-Python-Toolchain/pypls/internal/diagnostic"
	"github.com/Go-Python-Toolchain/pypls/internal/parser"
)

// sourceName labels every diagnostic so an editor can group problems by tool.
const sourceName = "pypls"

// cacheNamespace groups check results in the cache.
const cacheNamespace = "check"

// cachedCheck is the stored shape of a completed check.
type cachedCheck struct {
	Diagnostics []diagnostic.Diagnostic
}

// CheckCached returns diagnostics for source, using the cache to skip work when
// the same content has been checked before. A nil cache falls back to a direct
// check. The cache key is the content hash, so any edit invalidates the entry.
func CheckCached(c *cache.Cache, file, source string) []diagnostic.Diagnostic {
	if c == nil {
		return Check(file, source)
	}
	id := hashSource(source)
	if v, ok := cache.GetValue[cachedCheck](c, cacheNamespace, id); ok {
		return v.Diagnostics
	}
	diags := Check(file, source)
	_ = cache.PutValue(c, cacheNamespace, id, cachedCheck{Diagnostics: diags})
	return diags
}

func hashSource(source string) string {
	sum := sha256.Sum256([]byte(source))
	return hex.EncodeToString(sum[:])
}

// Check parses source and returns all diagnostics, ordered by position. It
// reports syntax problems from the parser and type problems from the checker.
func Check(file, source string) []diagnostic.Diagnostic {
	mod, errs := parser.Parse(file, source)
	diags := syntaxDiagnostics(errs)
	if mod != nil {
		diags = append(diags, checker.Check(mod)...)
	}
	sortDiagnostics(diags)
	return diags
}

// syntaxDiagnostics converts parser errors into diagnostics.
func syntaxDiagnostics(errs []parser.Error) []diagnostic.Diagnostic {
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
	return diags
}

// sortDiagnostics orders diagnostics by start position.
func sortDiagnostics(diags []diagnostic.Diagnostic) {
	sort.SliceStable(diags, func(i, j int) bool {
		a, b := diags[i].Range.Start, diags[j].Range.Start
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		return a.Column < b.Column
	})
}
