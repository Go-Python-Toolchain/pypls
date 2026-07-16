package lsp

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"
)

// startServer wires a client conn to a running server over in-memory pipes.
func startServer(t *testing.T) (*conn, func()) {
	t.Helper()
	reqR, reqW := io.Pipe()
	respR, respW := io.Pipe()
	srv := NewServer(reqR, respW, "test")

	done := make(chan struct{})
	go func() {
		_ = srv.Run()
		close(done)
	}()

	client := newConn(respR, reqW)
	cleanup := func() {
		_ = reqW.Close()
		_ = respW.Close()
		<-done
	}
	return client, cleanup
}

func send(t *testing.T, c *conn, id int, method string, params any) {
	t.Helper()
	raw, err := json.Marshal(params)
	if err != nil {
		t.Fatal(err)
	}
	m := &message{Method: method, Params: raw}
	if id != 0 {
		idRaw, _ := json.Marshal(id)
		m.ID = idRaw
	}
	if err := c.write(m); err != nil {
		t.Fatal(err)
	}
}

func recv(t *testing.T, c *conn) *message {
	t.Helper()
	m, err := c.read()
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestFullLifecycle(t *testing.T) {
	client, cleanup := startServer(t)
	defer cleanup()

	// initialize.
	send(t, client, 1, "initialize", initializeParams{})
	resp := recv(t, client)
	var ir initializeResult
	if err := json.Unmarshal(resp.Result, &ir); err != nil {
		t.Fatal(err)
	}
	if ir.Capabilities.TextDocumentSync != syncFull {
		t.Fatalf("expected full sync, got %d", ir.Capabilities.TextDocumentSync)
	}
	if ir.Capabilities.CompletionProvider == nil {
		t.Fatal("expected completion capability")
	}
	if ir.ServerInfo.Name != "pypls" {
		t.Fatalf("unexpected server name %q", ir.ServerInfo.Name)
	}

	send(t, client, 0, "initialized", map[string]any{})

	// didOpen a document with a type error.
	send(t, client, 0, "textDocument/didOpen", didOpenTextDocumentParams{
		TextDocument: textDocumentItem{URI: "file:///t.py", Version: 1, Text: "x: int = \"bad\"\n"},
	})
	note := recv(t, client)
	if note.Method != "textDocument/publishDiagnostics" {
		t.Fatalf("expected publishDiagnostics, got %q", note.Method)
	}
	var pd publishDiagnosticsParams
	if err := json.Unmarshal(note.Params, &pd); err != nil {
		t.Fatal(err)
	}
	if len(pd.Diagnostics) != 1 {
		t.Fatalf("expected one diagnostic, got %v", pd.Diagnostics)
	}
	// The value "bad" is at zero-based line 0, character 9.
	if pd.Diagnostics[0].Range.Start.Line != 0 || pd.Diagnostics[0].Range.Start.Character != 9 {
		t.Fatalf("unexpected diagnostic position: %+v", pd.Diagnostics[0].Range.Start)
	}

	// didChange to fix the error.
	send(t, client, 0, "textDocument/didChange", didChangeTextDocumentParams{
		TextDocument:   versionedTextDocumentIdentifier{URI: "file:///t.py", Version: 2},
		ContentChanges: []textDocumentContentChangeEvent{{Text: "x: int = 5\n"}},
	})
	note2 := recv(t, client)
	if err := json.Unmarshal(note2.Params, &pd); err != nil {
		t.Fatal(err)
	}
	if len(pd.Diagnostics) != 0 {
		t.Fatalf("expected the fix to clear diagnostics, got %v", pd.Diagnostics)
	}

	// completion should include keywords and the declared name.
	send(t, client, 2, "textDocument/completion", completionParams{
		TextDocument: textDocumentIdentifier{URI: "file:///t.py"},
	})
	cresp := recv(t, client)
	var cl completionList
	if err := json.Unmarshal(cresp.Result, &cl); err != nil {
		t.Fatal(err)
	}
	if !hasLabel(cl.Items, "def") {
		t.Fatal("expected keyword completion for def")
	}
	if !hasLabel(cl.Items, "x") {
		t.Fatal("expected completion for the declared variable x")
	}

	// shutdown then exit.
	send(t, client, 3, "shutdown", nil)
	sresp := recv(t, client)
	if string(sresp.Result) != "null" {
		t.Fatalf("expected null shutdown result, got %s", sresp.Result)
	}
	send(t, client, 0, "exit", nil)
}

func TestIncrementalReuseAcrossDidChange(t *testing.T) {
	client, cleanup := startServer(t)
	defer cleanup()

	send(t, client, 1, "initialize", initializeParams{})
	recv(t, client)
	send(t, client, 0, "initialized", map[string]any{})

	text := "def a():\n    return 1\n\ndef b():\n    y: int = \"bad\"\n"
	send(t, client, 0, "textDocument/didOpen", didOpenTextDocumentParams{
		TextDocument: textDocumentItem{URI: "file:///m.py", Version: 1, Text: text},
	})
	recv(t, client) // initial diagnostics

	// Insert a line into function a. Function b is unchanged but shifts down.
	edited := "def a():\n    z = 0\n    return 1\n\ndef b():\n    y: int = \"bad\"\n"
	send(t, client, 0, "textDocument/didChange", didChangeTextDocumentParams{
		TextDocument:   versionedTextDocumentIdentifier{URI: "file:///m.py", Version: 2},
		ContentChanges: []textDocumentContentChangeEvent{{Text: edited}},
	})
	note := recv(t, client)
	var pd publishDiagnosticsParams
	if err := json.Unmarshal(note.Params, &pd); err != nil {
		t.Fatal(err)
	}
	if len(pd.Diagnostics) != 1 {
		t.Fatalf("expected one diagnostic, got %v", pd.Diagnostics)
	}
	// b's diagnostic followed it to zero-based line 5.
	if pd.Diagnostics[0].Range.Start.Line != 5 {
		t.Fatalf("expected diagnostic at line 5, got %d", pd.Diagnostics[0].Range.Start.Line)
	}

	send(t, client, 0, "exit", nil)
}

func TestLaunchIsFast(t *testing.T) {
	best := time.Hour
	for i := 0; i < 25; i++ {
		var buf bytes.Buffer
		start := time.Now()
		s := NewServer(strings.NewReader(""), &buf, "test")
		s.dispatch(&message{ID: json.RawMessage("1"), Method: "initialize", Params: json.RawMessage(`{}`)})
		if d := time.Since(start); d < best {
			best = d
		}
	}
	t.Logf("fastest initialize handling: %v", best)
	if best > 10*time.Millisecond {
		t.Fatalf("expected initialize handling under 10ms, got %v", best)
	}
}

func hasLabel(items []completionItem, label string) bool {
	for _, it := range items {
		if it.Label == label {
			return true
		}
	}
	return false
}
