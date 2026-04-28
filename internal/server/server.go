package server

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"sync"

	"github.com/sourcegraph/jsonrpc2"
	"github.com/vikranthBala/esi-lsp/internal/analyzer"
	"github.com/vikranthBala/esi-lsp/internal/completion"
	"github.com/vikranthBala/esi-lsp/internal/definition"
	"github.com/vikranthBala/esi-lsp/internal/hover"
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

type positionParams struct {
	TextDocument struct {
		URI string `json:"uri"`
	} `json:"textDocument"`
	Position struct {
		Line      int `json:"line"`
		Character int `json:"character"`
	} `json:"position"`
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

func (s *Server) publishDiagnostics(uri string, doc *parser.Document) {
	diags := analyzer.Analyze(doc)

	lspDiags := make([]map[string]any, 0, len(diags))
	for _, d := range diags {
		lspDiags = append(lspDiags, map[string]any{
			"range":    lspRange(d.Range),
			"severity": d.Severity,
			"source":   "akamai-esi-lsp",
			"message":  d.Message,
		})
	}

	_ = s.conn.Notify(context.Background(), "textDocument/publishDiagnostics", map[string]any{
		"uri":         uri,
		"diagnostics": lspDiags,
	})
}

func lspRange(r parser.Range) map[string]any {
	return map[string]any{
		"start": map[string]any{
			"line":      r.Start.Line,
			"character": r.Start.Character,
		},
		"end": map[string]any{
			"line":      r.End.Line,
			"character": r.End.Character,
		},
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

func (s *Server) handleHover(req *jsonrpc2.Request) (any, error) {
	params := new(positionParams)
	if err := json.Unmarshal(*req.Params, params); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	doc := s.docs[params.TextDocument.URI]
	if doc == nil {
		return nil, nil
	}
	result := hover.Hover(doc, parser.Position{
		Line:      params.Position.Line,
		Character: params.Position.Character,
	})
	return result, nil
}

func (s *Server) handleCompletion(req *jsonrpc2.Request) (any, error) {
	params := new(positionParams)
	if err := json.Unmarshal(*req.Params, params); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	doc := s.docs[params.TextDocument.URI]
	if doc == nil {
		return nil, nil
	}
	result := completion.Complete(doc, parser.Position{
		Line:      params.Position.Line,
		Character: params.Position.Character,
	})
	return result, nil
}

func (s *Server) handleDefinition(req *jsonrpc2.Request) (any, error) {
	params := new(positionParams)
	if err := json.Unmarshal(*req.Params, params); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	doc := s.docs[params.TextDocument.URI]
	if doc == nil {
		return nil, nil
	}
	result := definition.Definition(doc, parser.Position{
		Line:      params.Position.Line,
		Character: params.Position.Character,
	})
	return result, nil
}

// this is called, when the client looks at the server, and sends it, its capabilities,
// and expectes server to do the same, based on servers response, it will let the server know based on the capability
func (s *Server) handleInitialize(req *jsonrpc2.Request) (any, error) {

	// for now returing the server capabilities directly

	return map[string]any{
		"capabilities": map[string]any{
			"textDocumentSync": 1, // always send the whole document, when any change happens, use 2, for incremental changes
			// registers us as a completion provider.
			// triggerCharacters tells the editor which characters should automatically pop open the completion menu without the user
			// pressing Ctrl+Space. We want < (starting a tag), : (after esi), and space (inside a tag, offering attributes).
			"completionProvider": map[string][]string{
				"triggerCharacters": []string{"<", ":", " "},
			},
			"hoverProvider":      true, // says server has the capabitilty to serve hover notification
			"definitionProvider": true, // says server has the capabitilty to serve definition notification
		},
	}, nil
}

func (s *Server) handle(ctx context.Context, conn *jsonrpc2.Conn, req *jsonrpc2.Request) (any, error) {
	log.Printf("<-%s", req.Method)

	switch req.Method {
	case "initialize":
		return s.handleInitialize(req)
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

	case "textDocument/hover":
		return s.handleHover(req)
	case "textDocument/completion":
		return s.handleCompletion(req)
	case "textDocument/definition":
		return s.handleDefinition(req)

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
