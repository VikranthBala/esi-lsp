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

// parses, and if a valid parse, directly updates s.docs. Doesnt return anything
func (s *Server) scheduleParse(uri, text string) {
	//check if a parse operation is running for the uri
	s.mu.Lock()
	if cancel, ok := s.parseCancels[uri]; ok {
		cancel()
	}

	// once the delete operation is done from the parseCancels, we need to start a new parse
	// calling cancel() causes ctx.Done() channel to close
	ctx, cancel := context.WithCancel(context.Background())

	// once the new cancel context is created, we need to add it to the list
	// this is done, so that any event that comes, after this knows to cancel this call
	s.parseCancels[uri] = cancel
	s.mu.Unlock()

	go func(ctx context.Context, uri, text string) {
		doc := parser.ParseDocument(uri, text)
		if ctx.Err() != nil {
			return
		}

		// i.e, context is not cancelled, i.e, ctx.Err will be nil
		// update docs
		s.mu.Lock()
		s.docs[uri] = doc
		s.mu.Unlock()
		// tell the editor about any errors found
		s.publishDiagnostics(uri, doc)
	}(ctx, uri, text)

}

func (s *Server) handleOpen(req *jsonrpc2.Request) error {
	params := new(didOpenParams)
	if err := json.Unmarshal(*req.Params, params); err != nil {
		return err
	}

	s.scheduleParse(params.TextDocument.URI, params.TextDocument.Text)
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

	s.scheduleParse(params.TextDocument.URI, params.ContentChanges[0].Text)

	return nil
}

func (s *Server) handleClose(req *jsonrpc2.Request) error {
	params := new(didCloseParams)
	if err := json.Unmarshal(*req.Params, params); err != nil {
		return err
	}

	s.mu.Lock()
	if cancel, ok := s.parseCancels[params.TextDocument.URI]; ok {
		cancel()
		delete(s.parseCancels, params.TextDocument.URI)
	}
	delete(s.docs, params.TextDocument.URI)
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
