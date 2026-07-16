package lsp

import (
	"strings"

	"github.com/Go-Python-Toolchain/pypls/internal/diagnostic"
	"github.com/Go-Python-Toolchain/pypls/internal/token"
)

// toLSPDiagnostics converts internal diagnostics, which use one-based lines and
// rune columns, into protocol diagnostics, which use zero-based lines and
// UTF-16 character offsets.
func toLSPDiagnostics(text string, diags []diagnostic.Diagnostic) []lspDiagnostic {
	lines := strings.Split(text, "\n")
	out := make([]lspDiagnostic, 0, len(diags))
	for _, d := range diags {
		out = append(out, lspDiagnostic{
			Range: lspRange{
				Start: toLSPPosition(lines, d.Range.Start),
				End:   toLSPPosition(lines, d.Range.End),
			},
			Severity: int(d.Severity),
			Code:     d.Code,
			Source:   d.Source,
			Message:  d.Message,
		})
	}
	return out
}

// toLSPPosition converts a one-based rune position into a zero-based UTF-16
// position, using the line text to measure UTF-16 code units.
func toLSPPosition(lines []string, p token.Position) position {
	line := p.Line - 1
	if line < 0 {
		line = 0
	}
	runeCol := p.Column - 1
	if runeCol < 0 {
		runeCol = 0
	}
	character := runeCol
	if line < len(lines) {
		character = utf16Column(lines[line], runeCol)
	}
	return position{Line: line, Character: character}
}

// utf16Column returns the number of UTF-16 code units in the first runeCol runes
// of lineText. Characters above the basic plane count as two units.
func utf16Column(lineText string, runeCol int) int {
	units := 0
	count := 0
	for _, r := range lineText {
		if count >= runeCol {
			break
		}
		if r >= 0x10000 {
			units += 2
		} else {
			units++
		}
		count++
	}
	return units
}
