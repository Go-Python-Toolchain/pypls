// Package lsp implements a Language Server Protocol server for pypls. It speaks
// JSON-RPC 2.0 over a stream, keeps open documents in memory, and publishes
// diagnostics as documents change.
package lsp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
)

// message is a JSON-RPC 2.0 envelope. A request carries ID and Method, a
// notification carries only Method, and a response carries ID with Result or
// Error.
type message struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *responseError  `json:"error,omitempty"`
}

// responseError is a JSON-RPC error object.
type responseError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Standard JSON-RPC error codes used by the server.
const (
	codeParseError     = -32700
	codeMethodNotFound = -32601
	codeInternalError  = -32603
)

// conn reads and writes framed JSON-RPC messages over a stream. Writes are
// serialized so the server can publish from multiple goroutines safely.
type conn struct {
	r   *bufio.Reader
	w   io.Writer
	wmu sync.Mutex
}

func newConn(r io.Reader, w io.Writer) *conn {
	return &conn{r: bufio.NewReader(r), w: w}
}

// read reads the next message, decoding the Content-Length framed body.
func (c *conn) read() (*message, error) {
	contentLength := -1
	for {
		line, err := c.r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		if name, value, ok := strings.Cut(line, ":"); ok {
			if strings.EqualFold(strings.TrimSpace(name), "Content-Length") {
				n, err := strconv.Atoi(strings.TrimSpace(value))
				if err != nil {
					return nil, fmt.Errorf("invalid Content-Length: %w", err)
				}
				contentLength = n
			}
		}
	}
	if contentLength < 0 {
		return nil, fmt.Errorf("message is missing Content-Length")
	}
	body := make([]byte, contentLength)
	if _, err := io.ReadFull(c.r, body); err != nil {
		return nil, err
	}
	var m message
	if err := json.Unmarshal(body, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

// write frames and sends a message.
func (c *conn) write(m *message) error {
	m.JSONRPC = "2.0"
	body, err := json.Marshal(m)
	if err != nil {
		return err
	}
	c.wmu.Lock()
	defer c.wmu.Unlock()
	if _, err := fmt.Fprintf(c.w, "Content-Length: %d\r\n\r\n", len(body)); err != nil {
		return err
	}
	_, err = c.w.Write(body)
	return err
}

// respond sends a successful response to a request id.
func (c *conn) respond(id json.RawMessage, result any) error {
	raw, err := json.Marshal(result)
	if err != nil {
		return err
	}
	return c.write(&message{ID: id, Result: raw})
}

// respondError sends an error response to a request id.
func (c *conn) respondError(id json.RawMessage, code int, msg string) error {
	return c.write(&message{ID: id, Error: &responseError{Code: code, Message: msg}})
}

// notify sends a notification, which has no id and expects no reply.
func (c *conn) notify(method string, params any) error {
	raw, err := json.Marshal(params)
	if err != nil {
		return err
	}
	return c.write(&message{Method: method, Params: raw})
}
