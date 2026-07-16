// Package diagnostic defines the shape of a problem reported about a source
// file. The types mirror the Language Server Protocol so that the same values
// can be sent to an editor or printed on the command line.
package diagnostic

import (
	"fmt"

	"github.com/Go-Python-Toolchain/pypls/internal/token"
)

// Severity ranks how serious a diagnostic is. The numeric values match the
// Language Server Protocol.
type Severity int

const (
	SeverityError       Severity = 1
	SeverityWarning     Severity = 2
	SeverityInformation Severity = 3
	SeverityHint        Severity = 4
)

// String renders the severity as a short label.
func (s Severity) String() string {
	switch s {
	case SeverityError:
		return "error"
	case SeverityWarning:
		return "warning"
	case SeverityInformation:
		return "info"
	case SeverityHint:
		return "hint"
	}
	return "unknown"
}

// Range is a half-open span of source, from Start up to but not including End.
type Range struct {
	Start token.Position
	End   token.Position
}

// Diagnostic is a single reported problem.
type Diagnostic struct {
	Range    Range
	Severity Severity
	Code     string
	Source   string
	Message  string
}

// String renders the diagnostic as line:column: severity: message.
func (d Diagnostic) String() string {
	return fmt.Sprintf("%s: %s: %s", d.Range.Start, d.Severity, d.Message)
}
