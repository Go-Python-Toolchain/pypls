package analyzer

import (
	"hash/fnv"
	"strconv"
	"strings"

	"github.com/Go-Python-Toolchain/pypls/internal/ast"
	"github.com/Go-Python-Toolchain/pypls/internal/checker"
	"github.com/Go-Python-Toolchain/pypls/internal/diagnostic"
	"github.com/Go-Python-Toolchain/pypls/internal/parser"
)

// IncrementalAnalyzer re-checks a document as it is edited, doing type work only
// for the top-level units whose text actually changed. Parsing is always run
// because it is cheap and gives fresh, correct positions. The expensive type
// check is skipped for unchanged units, whose cached diagnostics are shifted to
// their new line position. It is intended to be owned by one document and is not
// safe for concurrent use on its own.
type IncrementalAnalyzer struct {
	units          map[string]cachedUnit
	lastReanalyzed int
}

// cachedUnit stores a unit's diagnostics together with the line and byte offset
// the unit started on when they were computed, so they can be shifted to a new
// position on reuse. The unit's internal layout is identical when reused, so a
// single line and offset delta rebases every diagnostic exactly.
type cachedUnit struct {
	startLine   int
	startOffset int
	diags       []diagnostic.Diagnostic
}

// NewIncrementalAnalyzer creates an analyzer with an empty cache.
func NewIncrementalAnalyzer() *IncrementalAnalyzer {
	return &IncrementalAnalyzer{units: map[string]cachedUnit{}}
}

// Analyze returns diagnostics for the current source, reusing cached results for
// unchanged top-level units.
func (a *IncrementalAnalyzer) Analyze(source string) []diagnostic.Diagnostic {
	mod, errs := parser.Parse("", source)
	diags := syntaxDiagnostics(errs)

	if mod != nil {
		scope := checker.ModuleScope(mod)
		lines := strings.Split(source, "\n")
		next := make(map[string]cachedUnit, len(mod.Body))
		reanalyzed := 0

		for _, stmt := range mod.Body {
			key := hashUnit(unitText(lines, stmt))
			start := stmt.Pos()
			if cu, ok := a.units[key]; ok {
				lineDelta := start.Line - cu.startLine
				offsetDelta := start.Offset - cu.startOffset
				for _, d := range cu.diags {
					shifted := d
					shifted.Range.Start.Line += lineDelta
					shifted.Range.Start.Offset += offsetDelta
					shifted.Range.End.Line += lineDelta
					shifted.Range.End.Offset += offsetDelta
					diags = append(diags, shifted)
				}
				// Carry the original baseline forward so future shifts stay exact.
				next[key] = cu
			} else {
				ud := checker.CheckUnit(stmt, scope)
				reanalyzed++
				diags = append(diags, ud...)
				next[key] = cachedUnit{startLine: start.Line, startOffset: start.Offset, diags: ud}
			}
		}

		a.units = next
		a.lastReanalyzed = reanalyzed
	}

	sortDiagnostics(diags)
	return diags
}

// LastReanalyzed reports how many top-level units were type-checked on the most
// recent call to Analyze. Unchanged units are served from the cache and are not
// counted.
func (a *IncrementalAnalyzer) LastReanalyzed() int {
	return a.lastReanalyzed
}

// unitText returns the source lines spanned by a top-level statement.
func unitText(lines []string, stmt ast.Stmt) string {
	start := stmt.Pos().Line - 1
	end := stmt.End().Line - 1
	if start < 0 {
		start = 0
	}
	if end >= len(lines) {
		end = len(lines) - 1
	}
	if end < start {
		end = start
	}
	return strings.Join(lines[start:end+1], "\n")
}

func hashUnit(text string) string {
	h := fnv.New64a()
	_, _ = h.Write([]byte(text))
	return strconv.FormatUint(h.Sum64(), 16)
}
