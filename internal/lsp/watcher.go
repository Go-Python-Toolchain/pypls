package lsp

import (
	"io/fs"
	"os"
	"path/filepath"

	"github.com/Go-Python-Toolchain/pypls/internal/analyzer"
	"github.com/fsnotify/fsnotify"
)

// fileWatcher notices Python files that change on disk outside the editor, for
// example after a branch switch, and republishes their diagnostics. Files that
// are open in the editor are left alone, since the editor is their source of
// truth.
type fileWatcher struct {
	w    *fsnotify.Watcher
	srv  *Server
	stop chan struct{}
}

// startWatcher begins watching the workspace root. It is a best-effort feature:
// if the root is unknown or a watcher cannot be created, the server simply runs
// without it.
func (s *Server) startWatcher() {
	s.mu.Lock()
	root := s.rootPath
	already := s.watcher != nil
	s.mu.Unlock()
	if root == "" || already {
		return
	}

	w, err := fsnotify.NewWatcher()
	if err != nil {
		return
	}
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err == nil && d.IsDir() {
			_ = w.Add(path)
		}
		return nil
	})

	fw := &fileWatcher{w: w, srv: s, stop: make(chan struct{})}
	s.mu.Lock()
	s.watcher = fw
	s.mu.Unlock()
	go fw.loop()
}

func (s *Server) stopWatcher() {
	s.mu.Lock()
	fw := s.watcher
	s.watcher = nil
	s.mu.Unlock()
	if fw != nil {
		fw.close()
	}
}

func (fw *fileWatcher) loop() {
	for {
		select {
		case <-fw.stop:
			return
		case ev, ok := <-fw.w.Events:
			if !ok {
				return
			}
			if filepath.Ext(ev.Name) != ".py" {
				continue
			}
			if ev.Op&(fsnotify.Write|fsnotify.Create) == 0 {
				continue
			}
			fw.srv.handleDiskChange(ev.Name)
		case _, ok := <-fw.w.Errors:
			if !ok {
				return
			}
		}
	}
}

func (fw *fileWatcher) close() {
	close(fw.stop)
	_ = fw.w.Close()
}

// handleDiskChange rechecks a file that changed on disk, unless it is open in
// the editor.
func (s *Server) handleDiskChange(path string) {
	uri := "file://" + path

	s.mu.RLock()
	_, open := s.docs[uri]
	s.mu.RUnlock()
	if open {
		return
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	diags := analyzer.Check(path, string(data))
	_ = s.conn.notify("textDocument/publishDiagnostics", publishDiagnosticsParams{
		URI:         uri,
		Diagnostics: toLSPDiagnostics(string(data), diags),
	})
}
