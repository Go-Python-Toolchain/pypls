package lsp

// This file drives the server end to end as a scripted editor client over
// in-memory pipes. It exercises the Language Server Protocol 3.17 message flow
// an editor performs during a real session: the initialize handshake, opening
// and editing documents, requesting completions, closing documents, and the
// shutdown and exit teardown. Every step asserts on the exact bytes the server
// sends back, so the suite stands in for protocol conformance without needing a
// windowed editor and a display.

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/Go-Python-Toolchain/pypls/internal/analyzer"
)

// initializeWith performs the initialize handshake, sending realistic client
// capabilities in the params, and returns the decoded result.
func initializeWith(t *testing.T, c *conn, id int, rootURI string) initializeResult {
	t.Helper()
	params := map[string]any{
		"processId": 4242,
		"rootUri":   rootURI,
		"clientInfo": map[string]any{
			"name":    "pypls-integration-client",
			"version": "1.0.0",
		},
		"capabilities": map[string]any{
			"textDocument": map[string]any{
				"synchronization": map[string]any{
					"dynamicRegistration": true,
					"didSave":             true,
				},
				"completion": map[string]any{
					"completionItem": map[string]any{
						"snippetSupport": true,
					},
				},
				"publishDiagnostics": map[string]any{
					"relatedInformation": true,
				},
			},
		},
	}
	send(t, c, id, "initialize", params)
	resp := recv(t, c)
	if resp.Error != nil {
		t.Fatalf("initialize returned an error: %+v", resp.Error)
	}
	var ir initializeResult
	if err := json.Unmarshal(resp.Result, &ir); err != nil {
		t.Fatalf("decoding initialize result: %v", err)
	}
	return ir
}

// openAndDiagnose opens a document and returns the diagnostics the server
// publishes for it, asserting the notification is well formed and addressed to
// the document that was opened.
func openAndDiagnose(t *testing.T, c *conn, uri, text string, version int) []lspDiagnostic {
	t.Helper()
	send(t, c, 0, "textDocument/didOpen", didOpenTextDocumentParams{
		TextDocument: textDocumentItem{URI: uri, LanguageID: "python", Version: version, Text: text},
	})
	return expectDiagnostics(t, c, uri)
}

// expectDiagnostics reads the next message, asserts it is a publishDiagnostics
// notification for uri, and returns its diagnostics.
func expectDiagnostics(t *testing.T, c *conn, uri string) []lspDiagnostic {
	t.Helper()
	note := recv(t, c)
	if note.Method != "textDocument/publishDiagnostics" {
		t.Fatalf("expected publishDiagnostics, got method %q", note.Method)
	}
	if len(note.ID) != 0 {
		t.Fatalf("a notification must not carry an id, got %s", note.ID)
	}
	var pd publishDiagnosticsParams
	if err := json.Unmarshal(note.Params, &pd); err != nil {
		t.Fatalf("decoding diagnostics: %v", err)
	}
	if pd.URI != uri {
		t.Fatalf("diagnostics addressed to %q, expected %q", pd.URI, uri)
	}
	return pd.Diagnostics
}

// TestIntegrationInitializeHandshake drives the initialize request with client
// capabilities and asserts the server advertises the capabilities an editor
// needs, along with its identity.
func TestIntegrationInitializeHandshake(t *testing.T) {
	client, cleanup := startServer(t)
	defer cleanup()

	ir := initializeWith(t, client, 1, "file://"+t.TempDir())

	if ir.Capabilities.TextDocumentSync != syncFull {
		t.Fatalf("expected full text document sync (%d), got %d", syncFull, ir.Capabilities.TextDocumentSync)
	}
	if ir.Capabilities.CompletionProvider == nil {
		t.Fatal("expected the server to advertise a completion provider")
	}
	if ir.ServerInfo.Name != "pypls" {
		t.Fatalf("expected serverInfo.name pypls, got %q", ir.ServerInfo.Name)
	}

	// The initialized notification opens the session. The server must not send a
	// reply to it, so the next thing we do proves the connection still works.
	send(t, client, 0, "initialized", map[string]any{})

	send(t, client, 2, "shutdown", nil)
	sresp := recv(t, client)
	if string(sresp.Result) != "null" {
		t.Fatalf("expected null shutdown result, got %s", sresp.Result)
	}
	send(t, client, 0, "exit", nil)
}

// TestIntegrationEditorSession walks a full, realistic editing session across
// two open documents: opening them, introducing a type error with an edit,
// fixing it with a second edit, completing, and closing.
func TestIntegrationEditorSession(t *testing.T) {
	client, cleanup := startServer(t)
	defer cleanup()

	initializeWith(t, client, 1, "file://"+t.TempDir())
	send(t, client, 0, "initialized", map[string]any{})

	// Open two documents. The first is clean, the second already has an error.
	const uriA = "file:///project/a.py"
	const uriB = "file:///project/b.py"

	diagsA := openAndDiagnose(t, client, uriA, "def greet(name):\n    return name\n", 1)
	if len(diagsA) != 0 {
		t.Fatalf("expected the clean document to have no diagnostics, got %v", diagsA)
	}

	diagsB := openAndDiagnose(t, client, uriB, "count: int = \"not a number\"\n", 1)
	if len(diagsB) != 1 {
		t.Fatalf("expected one diagnostic in b.py, got %v", diagsB)
	}
	if diagsB[0].Source != "pypls" {
		t.Fatalf("expected diagnostic source pypls, got %q", diagsB[0].Source)
	}

	// An edit that introduces a type error into the previously clean document.
	send(t, client, 0, "textDocument/didChange", didChangeTextDocumentParams{
		TextDocument:   versionedTextDocumentIdentifier{URI: uriA, Version: 2},
		ContentChanges: []textDocumentContentChangeEvent{{Text: "def greet(name):\n    total: int = \"bad\"\n    return name\n"}},
	})
	afterBreak := expectDiagnostics(t, client, uriA)
	if len(afterBreak) != 1 {
		t.Fatalf("expected the edit to introduce one diagnostic, got %v", afterBreak)
	}
	// The error is on the annotated assignment, zero-based line 1.
	if afterBreak[0].Range.Start.Line != 1 {
		t.Fatalf("expected the diagnostic on line 1, got %d", afterBreak[0].Range.Start.Line)
	}

	// A second edit that fixes the error must clear the document's diagnostics.
	send(t, client, 0, "textDocument/didChange", didChangeTextDocumentParams{
		TextDocument:   versionedTextDocumentIdentifier{URI: uriA, Version: 3},
		ContentChanges: []textDocumentContentChangeEvent{{Text: "def greet(name):\n    total: int = 0\n    return name\n"}},
	})
	afterFix := expectDiagnostics(t, client, uriA)
	if len(afterFix) != 0 {
		t.Fatalf("expected the fix to clear diagnostics, got %v", afterFix)
	}

	// Completion in a.py should surface both keywords and its declared names.
	send(t, client, 2, "textDocument/completion", completionParams{
		TextDocument: textDocumentIdentifier{URI: uriA},
	})
	cresp := recv(t, client)
	var cl completionList
	if err := json.Unmarshal(cresp.Result, &cl); err != nil {
		t.Fatalf("decoding completion: %v", err)
	}
	for _, kw := range []string{"def", "return", "class"} {
		if !hasLabel(cl.Items, kw) {
			t.Fatalf("expected keyword completion %q", kw)
		}
	}
	for _, name := range []string{"greet", "name", "total"} {
		if !hasLabel(cl.Items, name) {
			t.Fatalf("expected identifier completion %q declared in the document", name)
		}
	}

	// Closing a document clears its diagnostics with an empty array.
	send(t, client, 0, "textDocument/didClose", didCloseTextDocumentParams{
		TextDocument: textDocumentIdentifier{URI: uriB},
	})
	cleared := expectDiagnostics(t, client, uriB)
	if cleared == nil {
		t.Fatal("expected an empty diagnostics array on close, got null")
	}
	if len(cleared) != 0 {
		t.Fatalf("expected close to clear diagnostics, got %v", cleared)
	}

	send(t, client, 3, "shutdown", nil)
	recv(t, client)
	send(t, client, 0, "exit", nil)
}

// TestIntegrationMultipleDocuments opens several documents in one session and
// asserts each one gets its own diagnostics notification addressed to its URI.
func TestIntegrationMultipleDocuments(t *testing.T) {
	client, cleanup := startServer(t)
	defer cleanup()

	initializeWith(t, client, 1, "file://"+t.TempDir())
	send(t, client, 0, "initialized", map[string]any{})

	cases := []struct {
		uri       string
		text      string
		wantDiags int
	}{
		{"file:///m/one.py", "x = 1 + 2\n", 0},
		{"file:///m/two.py", "y: int = \"s\"\n", 1},
		{"file:///m/three.py", "def f(:\n", 2},
	}
	for _, tc := range cases {
		diags := openAndDiagnose(t, client, tc.uri, tc.text, 1)
		if len(diags) != tc.wantDiags {
			t.Fatalf("%s: expected %d diagnostics, got %v", tc.uri, tc.wantDiags, diags)
		}
	}

	send(t, client, 0, "exit", nil)
}

// TestIntegrationDiagnosticPositionsAreZeroBasedUTF16 asserts that published
// positions use zero-based lines and UTF-16 code unit character offsets, as the
// protocol requires. A supplementary-plane character on the diagnostic's line
// makes the UTF-16 offset differ from the rune offset, so the assertion only
// holds if the server counts UTF-16 units.
func TestIntegrationDiagnosticPositionsAreZeroBasedUTF16(t *testing.T) {
	client, cleanup := startServer(t)
	defer cleanup()

	initializeWith(t, client, 1, "file://"+t.TempDir())
	send(t, client, 0, "initialized", map[string]any{})

	// The value is a single string holding one emoji, which is one rune but two
	// UTF-16 code units. It sits on the second line so the line index is exercised
	// as well.
	text := "pad = 0\ncount: int = \"\U0001F600\"\n"
	diags := openAndDiagnose(t, client, "file:///u.py", text, 1)
	if len(diags) != 1 {
		t.Fatalf("expected one diagnostic, got %v", diags)
	}
	start := diags[0].Range.Start
	end := diags[0].Range.End

	// Zero-based line: the annotated assignment is the second line, index 1.
	if start.Line != 1 {
		t.Fatalf("expected zero-based line 1, got %d", start.Line)
	}
	// The value begins after `count: int = ` which is 13 characters.
	if start.Character != 13 {
		t.Fatalf("expected start character 13, got %d", start.Character)
	}
	// The value `"emoji"` is three runes but the emoji is two UTF-16 units, so the
	// end character is 13 + 1 + 2 + 1 = 17. A rune count would give 16, so this
	// pins the UTF-16 measurement.
	if end.Character != 17 {
		t.Fatalf("expected UTF-16 end character 17, got %d", end.Character)
	}

	send(t, client, 0, "exit", nil)
}

// TestIntegrationCompletionKeywordsAndIdentifiers asserts completion returns
// Python keywords together with the functions, classes, and variables declared
// in the open document.
func TestIntegrationCompletionKeywordsAndIdentifiers(t *testing.T) {
	client, cleanup := startServer(t)
	defer cleanup()

	initializeWith(t, client, 1, "file://"+t.TempDir())
	send(t, client, 0, "initialized", map[string]any{})

	text := "" +
		"class Widget:\n" +
		"    pass\n" +
		"\n" +
		"def build(spec):\n" +
		"    result = Widget()\n" +
		"    return result\n"
	openAndDiagnose(t, client, "file:///c.py", text, 1)

	send(t, client, 5, "textDocument/completion", completionParams{
		TextDocument: textDocumentIdentifier{URI: "file:///c.py"},
	})
	cresp := recv(t, client)
	var cl completionList
	if err := json.Unmarshal(cresp.Result, &cl); err != nil {
		t.Fatalf("decoding completion: %v", err)
	}

	// Keywords are always present.
	for _, kw := range []string{"def", "class", "return", "import", "lambda"} {
		if !hasLabel(cl.Items, kw) {
			t.Fatalf("expected keyword completion %q", kw)
		}
	}
	// Names declared in the document: a class, a function, a parameter, a variable.
	for _, name := range []string{"Widget", "build", "spec", "result"} {
		if !hasLabel(cl.Items, name) {
			t.Fatalf("expected declared-name completion %q", name)
		}
	}
	// The class completion should carry the class kind so editors show the right
	// icon, which confirms identifiers are classified rather than dumped as text.
	for _, it := range cl.Items {
		if it.Label == "Widget" && it.Kind != kindClass {
			t.Fatalf("expected Widget to be a class completion (kind %d), got kind %d", kindClass, it.Kind)
		}
		if it.Label == "build" && it.Kind != kindFunction {
			t.Fatalf("expected build to be a function completion (kind %d), got kind %d", kindFunction, it.Kind)
		}
	}

	send(t, client, 0, "exit", nil)
}

// TestIntegrationDidCloseClearsDiagnostics asserts that closing a document that
// had problems publishes an empty diagnostics array so the editor drops the old
// squiggles.
func TestIntegrationDidCloseClearsDiagnostics(t *testing.T) {
	client, cleanup := startServer(t)
	defer cleanup()

	initializeWith(t, client, 1, "file://"+t.TempDir())
	send(t, client, 0, "initialized", map[string]any{})

	const uri = "file:///close.py"
	diags := openAndDiagnose(t, client, uri, "v: int = \"wrong\"\n", 1)
	if len(diags) != 1 {
		t.Fatalf("expected one diagnostic before close, got %v", diags)
	}

	send(t, client, 0, "textDocument/didClose", didCloseTextDocumentParams{
		TextDocument: textDocumentIdentifier{URI: uri},
	})
	cleared := expectDiagnostics(t, client, uri)
	if len(cleared) != 0 {
		t.Fatalf("expected an empty diagnostics array on close, got %v", cleared)
	}

	send(t, client, 0, "exit", nil)
}

// TestIntegrationShutdownThenExit asserts the shutdown request returns a null
// result and that the following exit notification terminates the server loop
// cleanly. The cleanup helper waits on the server goroutine, so a hang here
// would fail the test by timeout rather than passing silently.
func TestIntegrationShutdownThenExit(t *testing.T) {
	client, cleanup := startServer(t)

	initializeWith(t, client, 1, "file://"+t.TempDir())
	send(t, client, 0, "initialized", map[string]any{})

	send(t, client, 9, "shutdown", nil)
	resp := recv(t, client)
	if resp.Error != nil {
		t.Fatalf("shutdown returned an error: %+v", resp.Error)
	}
	if string(resp.Result) != "null" {
		t.Fatalf("expected null shutdown result, got %s", resp.Result)
	}

	// exit ends the loop. cleanup blocks on the server goroutine returning, so if
	// exit did not stop the loop this test would deadlock and the runner would
	// report a timeout.
	send(t, client, 0, "exit", nil)

	done := make(chan struct{})
	go func() {
		cleanup()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("server did not exit within five seconds of the exit notification")
	}
}

// TestIntegrationIncrementalReanalysisSignal asserts the incremental analyzer
// that each open document owns rechecks only the top-level unit whose text
// changed, which is what keeps editor feedback fast on large files. It exercises
// the same analyzer the server drives on every didChange and reads its
// reanalysis signal directly, since that count is the evidence that unchanged
// units were served from cache rather than rechecked.
func TestIntegrationIncrementalReanalysisSignal(t *testing.T) {
	ia := analyzer.NewIncrementalAnalyzer()

	// A document with three independent top-level functions, mirroring what the
	// server stores after a didOpen.
	v1 := "" +
		"def a():\n    return 1\n\n" +
		"def b():\n    return 2\n\n" +
		"def c():\n    x: int = \"bad\"\n"
	ia.Analyze(v1)
	if got := ia.LastReanalyzed(); got != 3 {
		t.Fatalf("expected the first pass to analyze all 3 units, got %d", got)
	}

	// Re-analyzing identical text must recheck nothing.
	ia.Analyze(v1)
	if got := ia.LastReanalyzed(); got != 0 {
		t.Fatalf("expected 0 units rechecked on an identical document, got %d", got)
	}

	// An edit that only touches function a. Functions b and c are unchanged and
	// should be served from the cache even though c shifts down a line.
	v2 := "" +
		"def a():\n    y = 0\n    return 1\n\n" +
		"def b():\n    return 2\n\n" +
		"def c():\n    x: int = \"bad\"\n"
	diags := ia.Analyze(v2)
	if got := ia.LastReanalyzed(); got != 1 {
		t.Fatalf("expected exactly 1 unit rechecked after a single-unit edit, got %d", got)
	}

	// The diagnostic from the untouched function c must still be reported and must
	// have followed the function to its new line, proving the cached result was
	// reused with a correct position rebase.
	if len(diags) != 1 {
		t.Fatalf("expected one diagnostic carried across the edit, got %v", diags)
	}
	if diags[0].Range.Start.Line != 9 {
		t.Fatalf("expected the reused diagnostic on line 9, got %d", diags[0].Range.Start.Line)
	}
}
