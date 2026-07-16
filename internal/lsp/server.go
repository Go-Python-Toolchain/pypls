package lsp

import (
	"encoding/json"
	"errors"
	"io"
	"strings"
	"sync"

	"github.com/Go-Python-Toolchain/pypls/internal/analyzer"
)

// document is one open file tracked by the server, with its own incremental
// analyzer so that edits only recheck the parts that changed.
type document struct {
	version  int
	text     string
	analyzer *analyzer.IncrementalAnalyzer
}

// Server is a Language Server Protocol server for pypls. It is safe for the read
// loop and the file watcher to run concurrently.
type Server struct {
	conn    *conn
	version string

	mu       sync.RWMutex
	docs     map[string]*document
	rootPath string
	watcher  *fileWatcher
}

// NewServer creates a server that reads requests from r and writes responses to
// w. The version string is reported to the client on initialize.
func NewServer(r io.Reader, w io.Writer, version string) *Server {
	return &Server{
		conn:    newConn(r, w),
		version: version,
		docs:    map[string]*document{},
	}
}

// Run serves requests until the input closes or the client sends exit.
func (s *Server) Run() error {
	defer s.stopWatcher()
	for {
		m, err := s.conn.read()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		if s.dispatch(m) {
			return nil
		}
	}
}

// dispatch routes a message. It returns true when the server should stop.
func (s *Server) dispatch(m *message) bool {
	if m.Method == "" {
		return false // a response to a request we never send
	}
	if len(m.ID) > 0 {
		s.handleRequest(m)
		return false
	}
	return s.handleNotification(m)
}

func (s *Server) handleRequest(m *message) {
	switch m.Method {
	case "initialize":
		var p initializeParams
		_ = json.Unmarshal(m.Params, &p)
		s.setRoot(p)
		_ = s.conn.respond(m.ID, initializeResult{
			Capabilities: serverCapabilities{
				TextDocumentSync:   syncFull,
				CompletionProvider: &completionOptions{},
			},
			ServerInfo: serverInfo{Name: "pypls", Version: s.version},
		})
	case "shutdown":
		_ = s.conn.respond(m.ID, nil)
	case "textDocument/completion":
		var p completionParams
		_ = json.Unmarshal(m.Params, &p)
		s.mu.RLock()
		doc, ok := s.docs[p.TextDocument.URI]
		text := ""
		if ok {
			text = doc.text
		}
		s.mu.RUnlock()
		_ = s.conn.respond(m.ID, completionList{Items: completionsFor(text)})
	default:
		_ = s.conn.respondError(m.ID, codeMethodNotFound, "unsupported method: "+m.Method)
	}
}

func (s *Server) handleNotification(m *message) bool {
	switch m.Method {
	case "initialized":
		s.startWatcher()
	case "exit":
		return true
	case "textDocument/didOpen":
		var p didOpenTextDocumentParams
		if err := json.Unmarshal(m.Params, &p); err == nil {
			s.openDoc(p.TextDocument.URI, p.TextDocument.Version, p.TextDocument.Text)
		}
	case "textDocument/didChange":
		var p didChangeTextDocumentParams
		if err := json.Unmarshal(m.Params, &p); err == nil && len(p.ContentChanges) > 0 {
			last := p.ContentChanges[len(p.ContentChanges)-1].Text
			s.changeDoc(p.TextDocument.URI, p.TextDocument.Version, last)
		}
	case "textDocument/didClose":
		var p didCloseTextDocumentParams
		if err := json.Unmarshal(m.Params, &p); err == nil {
			s.closeDoc(p.TextDocument.URI)
		}
	}
	return false
}

func (s *Server) setRoot(p initializeParams) {
	root := p.RootPath
	if root == "" && p.RootURI != "" {
		root = uriToPath(p.RootURI)
	}
	s.mu.Lock()
	s.rootPath = root
	s.mu.Unlock()
}

func (s *Server) openDoc(uri string, version int, text string) {
	s.mu.Lock()
	s.docs[uri] = &document{
		version:  version,
		text:     text,
		analyzer: analyzer.NewIncrementalAnalyzer(),
	}
	s.mu.Unlock()
	s.analyzeAndPublish(uri)
}

func (s *Server) changeDoc(uri string, version int, text string) {
	s.mu.Lock()
	doc, ok := s.docs[uri]
	if !ok {
		doc = &document{analyzer: analyzer.NewIncrementalAnalyzer()}
		s.docs[uri] = doc
	}
	doc.version = version
	doc.text = text
	s.mu.Unlock()
	s.analyzeAndPublish(uri)
}

func (s *Server) closeDoc(uri string) {
	s.mu.Lock()
	delete(s.docs, uri)
	s.mu.Unlock()
	// Clear diagnostics for the closed document.
	_ = s.conn.notify("textDocument/publishDiagnostics", publishDiagnosticsParams{
		URI:         uri,
		Diagnostics: []lspDiagnostic{},
	})
}

// analyzeAndPublish analyzes the document and sends its diagnostics. The
// analysis runs under the server lock because a document's incremental analyzer
// is not safe for concurrent use.
func (s *Server) analyzeAndPublish(uri string) {
	s.mu.Lock()
	doc, ok := s.docs[uri]
	if !ok {
		s.mu.Unlock()
		return
	}
	text := doc.text
	version := doc.version
	diags := doc.analyzer.Analyze(text)
	s.mu.Unlock()

	_ = s.conn.notify("textDocument/publishDiagnostics", publishDiagnosticsParams{
		URI:         uri,
		Version:     version,
		Diagnostics: toLSPDiagnostics(text, diags),
	})
}

// uriToPath converts a file URI to a filesystem path. It handles the common
// file:// form and leaves anything else unchanged.
func uriToPath(uri string) string {
	const prefix = "file://"
	if strings.HasPrefix(uri, prefix) {
		return uri[len(prefix):]
	}
	return uri
}
