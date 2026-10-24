// Package lsp implements the AxLang Language Server Protocol over JSON-RPC stdio.
package lsp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/vivek5200/axlang/ast"
	"github.com/vivek5200/axlang/formatter"
	"github.com/vivek5200/axlang/lexer"
	"github.com/vivek5200/axlang/parser"
	"github.com/vivek5200/axlang/sema"
	"github.com/vivek5200/axlang/targets/rego"
	"github.com/vivek5200/axlang/token"
)

const maxMessageBytes = 16 << 20

type rpcMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type Server struct {
	documents map[string]string
}

func NewServer() *Server { return &Server{documents: make(map[string]string)} }

// Serve reads and writes Content-Length-framed JSON-RPC messages until EOF or exit.
func (s *Server) Serve(input io.Reader, output io.Writer) error {
	reader := bufio.NewReader(input)
	for {
		body, err := readMessage(reader)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		var request rpcMessage
		if err := json.Unmarshal(body, &request); err != nil {
			if err := writeMessage(output, rpcMessage{JSONRPC: "2.0", ID: json.RawMessage("null"), Error: &rpcError{Code: -32700, Message: "parse error"}}); err != nil {
				return err
			}
			continue
		}
		response, notifications, done := s.handle(request)
		for _, notification := range notifications {
			if err := writeMessage(output, notification); err != nil {
				return err
			}
		}
		if response != nil {
			if err := writeMessage(output, *response); err != nil {
				return err
			}
		}
		if done {
			return nil
		}
	}
}

func (s *Server) handle(request rpcMessage) (*rpcMessage, []rpcMessage, bool) {
	if request.JSONRPC != "2.0" {
		return response(request.ID, nil, &rpcError{Code: -32600, Message: "invalid JSON-RPC version"}), nil, false
	}
	var notifications []rpcMessage
	switch request.Method {
	case "initialize":
		result := map[string]any{"capabilities": map[string]any{
			"textDocumentSync":           1,
			"hoverProvider":              true,
			"documentFormattingProvider": true,
		}, "serverInfo": map[string]string{"name": "axls", "version": "0.1.0"}}
		return response(request.ID, result, nil), nil, false
	case "initialized", "$/setTrace", "textDocument/didClose":
		if request.Method == "textDocument/didClose" {
			var params documentParams
			if json.Unmarshal(request.Params, &params) == nil {
				delete(s.documents, params.TextDocument.URI)
				notifications = append(notifications, publishDiagnostics(params.TextDocument.URI, nil))
			}
		}
		return nil, notifications, false
	case "textDocument/didOpen":
		var params didOpenParams
		if json.Unmarshal(request.Params, &params) != nil {
			return nil, nil, false
		}
		s.documents[params.TextDocument.URI] = params.TextDocument.Text
		return nil, []rpcMessage{s.diagnostics(params.TextDocument.URI, params.TextDocument.Text)}, false
	case "textDocument/didChange":
		var params didChangeParams
		if json.Unmarshal(request.Params, &params) != nil || len(params.ContentChanges) == 0 {
			return nil, nil, false
		}
		text := params.ContentChanges[len(params.ContentChanges)-1].Text
		s.documents[params.TextDocument.URI] = text
		return nil, []rpcMessage{s.diagnostics(params.TextDocument.URI, text)}, false
	case "textDocument/hover":
		var params textPositionParams
		if json.Unmarshal(request.Params, &params) != nil {
			return response(request.ID, nil, nil), nil, false
		}
		return response(request.ID, s.hover(params.TextDocument.URI, params.Position), nil), nil, false
	case "textDocument/formatting":
		var params formattingParams
		if json.Unmarshal(request.Params, &params) != nil {
			return response(request.ID, []textEdit{}, nil), nil, false
		}
		text := s.documents[params.TextDocument.URI]
		file, diagnostics := parser.New(params.TextDocument.URI, text).Parse()
		if len(diagnostics) != 0 {
			return response(request.ID, []textEdit{}, nil), nil, false
		}
		edit := textEdit{Range: rangeForWholeDocument(text), NewText: formatter.Format(file)}
		return response(request.ID, []textEdit{edit}, nil), nil, false
	case "shutdown":
		return response(request.ID, nil, nil), nil, false
	case "exit":
		return nil, nil, true
	default:
		if len(request.ID) > 0 {
			return response(request.ID, nil, &rpcError{Code: -32601, Message: "method not found"}), nil, false
		}
		return nil, nil, false
	}
}

func response(id json.RawMessage, result any, rpcErr *rpcError) *rpcMessage {
	if len(id) == 0 {
		return nil
	}
	message := &rpcMessage{JSONRPC: "2.0", ID: id, Error: rpcErr}
	if rpcErr == nil {
		message.Result, _ = json.Marshal(result)
	}
	return message
}

type documentID struct {
	URI string `json:"uri"`
}
type documentParams struct {
	TextDocument documentID `json:"textDocument"`
}
type didOpenParams struct {
	TextDocument struct {
		URI  string `json:"uri"`
		Text string `json:"text"`
	} `json:"textDocument"`
}
type didChangeParams struct {
	TextDocument   documentID `json:"textDocument"`
	ContentChanges []struct {
		Text string `json:"text"`
	} `json:"contentChanges"`
}
type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}
type textPositionParams struct {
	TextDocument documentID `json:"textDocument"`
	Position     Position   `json:"position"`
}
type formattingParams struct {
	TextDocument documentID     `json:"textDocument"`
	Options      map[string]any `json:"options"`
}
type lspRange struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}
type lspDiagnostic struct {
	Range    lspRange `json:"range"`
	Severity int      `json:"severity"`
	Code     string   `json:"code"`
	Source   string   `json:"source"`
	Message  string   `json:"message"`
}
type textEdit struct {
	Range   lspRange `json:"range"`
	NewText string   `json:"newText"`
}
type markupContent struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
}
type hoverResult struct {
	Contents markupContent `json:"contents"`
}

func (s *Server) diagnostics(uri, source string) rpcMessage {
	file, parseDiagnostics := parser.New(uri, source).Parse()
	diagnostics := append([]parser.Diagnostic(nil), parseDiagnostics...)
	if len(parseDiagnostics) == 0 {
		diagnostics = append(diagnostics, sema.Check(file)...)
	}
	out := make([]lspDiagnostic, 0, len(diagnostics))
	for _, diagnostic := range diagnostics {
		out = append(out, lspDiagnostic{
			Range: spanRange(source, diagnostic.Span), Severity: 1,
			Code: diagnostic.Code, Source: "axlang", Message: diagnostic.Message,
		})
	}
	return publishDiagnostics(uri, out)
}

func publishDiagnostics(uri string, diagnostics []lspDiagnostic) rpcMessage {
	if diagnostics == nil {
		diagnostics = []lspDiagnostic{}
	}
	params := map[string]any{"uri": uri, "diagnostics": diagnostics}
	encoded, _ := json.Marshal(params)
	return rpcMessage{JSONRPC: "2.0", Method: "textDocument/publishDiagnostics", Params: encoded}
}

func (s *Server) hover(uri string, position Position) any {
	source, exists := s.documents[uri]
	if !exists {
		return nil
	}
	offset := offsetAt(source, position)
	file, _ := parser.New(uri, source).Parse()
	for _, declaration := range file.Decls {
		switch declaration.(type) {
		case *ast.GuardrailDecl, *ast.HealDecl:
			if !inHeader(source, declaration.Span(), offset) {
				continue
			}
			module, err := rego.Compile(&ast.File{Filename: file.Filename, Decls: []ast.Decl{declaration}})
			if err != nil {
				return nil
			}
			return hoverResult{Contents: markupContent{Kind: "markdown", Value: "```rego\n" + strings.TrimSpace(module) + "\n```"}}
		}
	}
	tokens, err := lexer.New(uri, source).Scan()
	if err != nil {
		return nil
	}
	for _, item := range tokens {
		if item.Kind == token.EOF || !containsOffset(item.Span, offset) {
			continue
		}
		var domain string
		switch item.Kind {
		case token.Duration:
			domain = "Duration"
		case token.Percent:
			domain = "Percent"
		case token.Ident:
			if strings.HasPrefix(item.Text, "metrics.") {
				switch {
				case strings.HasSuffix(item.Text, "_latency"):
					domain = "Duration"
				case strings.HasSuffix(item.Text, "_rate"):
					domain = "Percent"
				default:
					domain = "Unknown metric type"
				}
			}
		}
		if domain != "" {
			return hoverResult{Contents: markupContent{Kind: "markdown", Value: fmt.Sprintf("**AxLang type:** `%s`\n\n`%s`", domain, item.Text)}}
		}
	}
	return nil
}

func inHeader(source string, span token.Span, offset int) bool {
	if span.Offset < 0 || span.Offset >= len(source) || offset < span.Offset {
		return false
	}
	end := span.End()
	if end > len(source) {
		end = len(source)
	}
	brace := bytes.IndexByte([]byte(source[span.Offset:end]), '{')
	if brace < 0 {
		return false
	}
	return offset < span.Offset+brace
}

func containsOffset(span token.Span, offset int) bool {
	return offset >= span.Offset && offset < span.End()
}

func spanRange(source string, span token.Span) lspRange {
	start := span.Offset
	end := span.End()
	if start < 0 {
		start = 0
	}
	if start > len(source) {
		start = len(source)
	}
	if end < start {
		end = start
	}
	if end > len(source) {
		end = len(source)
	}
	return lspRange{Start: positionAt(source, start), End: positionAt(source, end)}
}

func rangeForWholeDocument(source string) lspRange {
	return lspRange{Start: Position{}, End: positionAt(source, len(source))}
}

func positionAt(source string, offset int) Position {
	if offset < 0 {
		offset = 0
	}
	if offset > len(source) {
		offset = len(source)
	}
	line, character := 0, 0
	for i := 0; i < offset; {
		r, size := utf8.DecodeRuneInString(source[i:])
		if size == 0 {
			break
		}
		if r == '\r' {
			if i+size < offset && source[i+size] == '\n' {
				i += size
			}
			line++
			character = 0
		} else if r == '\n' {
			line++
			character = 0
		} else {
			character += len(utf16.Encode([]rune{r}))
		}
		i += size
	}
	return Position{Line: line, Character: character}
}

func offsetAt(source string, position Position) int {
	if position.Line < 0 || position.Character < 0 {
		return 0
	}
	line, character := 0, 0
	for offset := 0; offset < len(source); {
		if line == position.Line && character >= position.Character {
			return offset
		}
		r, size := utf8.DecodeRuneInString(source[offset:])
		if r == '\r' || r == '\n' {
			if r == '\r' && offset+size < len(source) && source[offset+size] == '\n' {
				size++
			}
			if line == position.Line {
				return offset
			}
			line++
			character = 0
			offset += size
			continue
		}
		units := len(utf16.Encode([]rune{r}))
		if line == position.Line && position.Character < character+units {
			return offset
		}
		character += units
		offset += size
	}
	return len(source)
}

func readMessage(reader *bufio.Reader) ([]byte, error) {
	length := -1
	headerBytes := 0
	for {
		line, err := reader.ReadString('\n')
		headerBytes += len(line)
		if headerBytes > 8192 {
			return nil, fmt.Errorf("LSP header exceeds 8192 bytes")
		}
		if err != nil {
			return nil, err
		}
		line = strings.TrimSpace(line)
		if line == "" {
			break
		}
		name, value, ok := strings.Cut(line, ":")
		if ok && strings.EqualFold(strings.TrimSpace(name), "Content-Length") {
			if _, err := fmt.Sscanf(strings.TrimSpace(value), "%d", &length); err != nil {
				return nil, fmt.Errorf("invalid Content-Length: %w", err)
			}
		}
	}
	if length < 0 || length > maxMessageBytes {
		return nil, fmt.Errorf("invalid or excessive Content-Length %d", length)
	}
	body := make([]byte, length)
	if _, err := io.ReadFull(reader, body); err != nil {
		return nil, err
	}
	return body, nil
}

func writeMessage(writer io.Writer, message rpcMessage) error {
	body, err := json.Marshal(message)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(writer, "Content-Length: %d\r\n\r\n", len(body)); err != nil {
		return err
	}
	_, err = writer.Write(body)
	return err
}
