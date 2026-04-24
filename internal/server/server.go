package server

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"sync"

	"github.com/sourcegraph/jsonrpc2"
	"github.com/vikranthBala/esi-lsp/internal/parser"
)

// stdrwc bridges stdin+stdout into a single ReadWriteCloser.
type stdrwc struct{}

// implement methods so it can be a readwrite closer
func (stdrwc) Read(p []byte) (int, error)  { return os.Stdin.Read(p) }
func (stdrwc) Write(p []byte) (int, error) { return os.Stdout.Write(p) }
func (stdrwc) Close() error                { return nil }

// param structs for didOpen/didClose/didChange
type textDocumentItem struct {
	URI  string `json:"uri"`
	Text string `json:"text"`
}

type didOpenParams struct {
	TextDocument textDocumentItem `json:"textDocument"`
}

type didChangeParams struct {
	TextDocument struct {
		URI string `json:"uri"`
	} `json:"textDocument"`
	ContentChanges []struct {
		Text string `json:"text"`
	} `json:"contentChanges"`
}

type didCloseParams struct {
	TextDocument struct {
		URI string `json:"uri"`
	} `json:"textDocument"`
}

type Server struct {
	mu   sync.RWMutex
	docs map[string]*parser.Document

	// Cancellation for the latest parse goroutine per document.
	// Key: document URI, Value: cancel function for the running parse.
	parseCancels map[string]context.CancelFunc

	conn *jsonrpc2.Conn
}

func New() *Server {
	return &Server{
		docs:         make(map[string]*parser.Document),
		parseCancels: make(map[string]context.CancelFunc),
	}
}

func (s *Server) handleOpen(req *jsonrpc2.Request) error {
	params := new(didOpenParams)
	if err := json.Unmarshal(*req.Params, params); err != nil {
		return err
	}

	doc := parser.ParseDocument(params.TextDocument.URI, params.TextDocument.Text)

	s.mu.Lock()
	s.docs[params.TextDocument.URI] = doc
	s.mu.Unlock()

	return nil
}

func (s *Server) handleClose(req *jsonrpc2.Request) error {
	params := new(didCloseParams)
	if err := json.Unmarshal(*req.Params, params); err != nil {
		return err
	}

	s.mu.Lock()
	delete(s.docs, params.TextDocument.URI)
	s.mu.Unlock()

	return nil
}

func (s *Server) handleChange(req *jsonrpc2.Request) error {
	params := new(didChangeParams)
	if err := json.Unmarshal(*req.Params, params); err != nil {
		return err
	}

	if len(params.ContentChanges) == 0 {
		return nil
	}

	doc := parser.ParseDocument(
		params.TextDocument.URI,
		params.ContentChanges[0].Text,
	)

	s.mu.Lock()
	s.docs[params.TextDocument.URI] = doc
	s.mu.Unlock()

	return nil
}

func (s *Server) handle(ctx context.Context, conn *jsonrpc2.Conn, req *jsonrpc2.Request) (any, error) {
	log.Printf("<-%s", req.Method)

	switch req.Method {
	case "initialize":
		return s.handleInitalize(req)
	case "initialized", "shutdown":
		return nil, nil
	case "exit":
		os.Exit(0)
		return nil, nil

	case "textDocument/didOpen":
		return nil, s.handleOpen(req)
	case "textDocument/didChange":
		return nil, s.handleChange(req)
	case "textDocument/didClose":
		return nil, s.handleClose(req)

	default:
		return nil, nil
	}
}

func (s *Server) Run(ctx context.Context) error {
	stream := jsonrpc2.NewBufferedStream(stdrwc{}, jsonrpc2.VSCodeObjectCodec{})
	conn := jsonrpc2.NewConn(ctx, stream, jsonrpc2.HandlerWithError(s.handle))
	s.conn = conn
	<-s.conn.DisconnectNotify()
	return nil
}
