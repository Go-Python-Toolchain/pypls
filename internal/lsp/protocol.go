package lsp

// This file defines the subset of Language Server Protocol types that pypls
// uses. Field names and JSON tags follow the protocol so that standard editors
// interoperate without adaptation.

// position is a zero-based line and character offset. Character offsets are
// counted in UTF-16 code units, as the protocol requires.
type position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

// lspRange is a span between two positions.
type lspRange struct {
	Start position `json:"start"`
	End   position `json:"end"`
}

// lspDiagnostic is a problem reported for a document.
type lspDiagnostic struct {
	Range    lspRange `json:"range"`
	Severity int      `json:"severity,omitempty"`
	Code     string   `json:"code,omitempty"`
	Source   string   `json:"source,omitempty"`
	Message  string   `json:"message"`
}

// publishDiagnosticsParams is the payload of textDocument/publishDiagnostics.
type publishDiagnosticsParams struct {
	URI         string          `json:"uri"`
	Version     int             `json:"version,omitempty"`
	Diagnostics []lspDiagnostic `json:"diagnostics"`
}

// initializeParams is the subset of the initialize request we read.
type initializeParams struct {
	RootURI  string `json:"rootUri"`
	RootPath string `json:"rootPath"`
}

// initializeResult announces the server's capabilities.
type initializeResult struct {
	Capabilities serverCapabilities `json:"capabilities"`
	ServerInfo   serverInfo         `json:"serverInfo"`
}

type serverInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type serverCapabilities struct {
	TextDocumentSync   int                `json:"textDocumentSync"`
	CompletionProvider *completionOptions `json:"completionProvider,omitempty"`
}

type completionOptions struct {
	TriggerCharacters []string `json:"triggerCharacters,omitempty"`
}

// textDocumentSyncKind values. Full means the whole document text is sent on
// every change.
const (
	syncNone        = 0
	syncFull        = 1
	syncIncremental = 2
)

type textDocumentItem struct {
	URI        string `json:"uri"`
	LanguageID string `json:"languageId"`
	Version    int    `json:"version"`
	Text       string `json:"text"`
}

type textDocumentIdentifier struct {
	URI string `json:"uri"`
}

type versionedTextDocumentIdentifier struct {
	URI     string `json:"uri"`
	Version int    `json:"version"`
}

type didOpenTextDocumentParams struct {
	TextDocument textDocumentItem `json:"textDocument"`
}

type textDocumentContentChangeEvent struct {
	Text string `json:"text"`
}

type didChangeTextDocumentParams struct {
	TextDocument   versionedTextDocumentIdentifier  `json:"textDocument"`
	ContentChanges []textDocumentContentChangeEvent `json:"contentChanges"`
}

type didCloseTextDocumentParams struct {
	TextDocument textDocumentIdentifier `json:"textDocument"`
}

type completionParams struct {
	TextDocument textDocumentIdentifier `json:"textDocument"`
	Position     position               `json:"position"`
}

// completionItemKind values used by the server.
const (
	kindVariable = 6
	kindFunction = 3
	kindClass    = 7
	kindKeyword  = 14
)

type completionItem struct {
	Label  string `json:"label"`
	Kind   int    `json:"kind,omitempty"`
	Detail string `json:"detail,omitempty"`
}

type completionList struct {
	IsIncomplete bool             `json:"isIncomplete"`
	Items        []completionItem `json:"items"`
}
