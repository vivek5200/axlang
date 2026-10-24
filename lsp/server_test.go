package lsp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"testing"
)

func frame(t *testing.T, value any) []byte {
	t.Helper()
	body, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return []byte(fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(body), body))
}

func runStream(t *testing.T, requests ...any) []rpcMessage {
	t.Helper()
	var input, output bytes.Buffer
	for _, request := range requests {
		input.Write(frame(t, request))
	}
	if err := NewServer().Serve(&input, &output); err != nil {
		t.Fatal(err)
	}
	reader := bufio.NewReader(&output)
	var messages []rpcMessage
	for {
		body, err := readMessage(reader)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		var message rpcMessage
		if err := json.Unmarshal(body, &message); err != nil {
			t.Fatal(err)
		}
		messages = append(messages, message)
	}
	return messages
}

func notification(method string, params any) map[string]any {
	return map[string]any{"jsonrpc": "2.0", "method": method, "params": params}
}

func request(id int, method string, params any) map[string]any {
	return map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params}
}

func TestInitializeCapabilities(t *testing.T) {
	messages := runStream(t, request(1, "initialize", map[string]any{}))
	if len(messages) != 1 {
		t.Fatalf("messages=%#v", messages)
	}
	var result struct {
		Capabilities struct {
			TextDocumentSync int  `json:"textDocumentSync"`
			Hover            bool `json:"hoverProvider"`
			Formatting       bool `json:"documentFormattingProvider"`
		} `json:"capabilities"`
	}
	if err := json.Unmarshal(messages[0].Result, &result); err != nil {
		t.Fatal(err)
	}
	if result.Capabilities.TextDocumentSync != 1 || !result.Capabilities.Hover || !result.Capabilities.Formatting {
		t.Fatalf("capabilities=%+v", result.Capabilities)
	}
}

func TestDiagnosticsArePublishedAndClearedAfterChange(t *testing.T) {
	uri := "file:///workspace/Astrixfile.ax"
	messages := runStream(t,
		notification("textDocument/didOpen", map[string]any{"textDocument": map[string]any{"uri": uri, "text": `stage broken {}`}}),
		notification("textDocument/didChange", map[string]any{"textDocument": map[string]any{"uri": uri, "version": 2}, "contentChanges": []any{map[string]any{"text": `stage "verify" {}`}}}),
	)
	if len(messages) != 2 || messages[0].Method != "textDocument/publishDiagnostics" || messages[1].Method != "textDocument/publishDiagnostics" {
		t.Fatalf("messages=%#v", messages)
	}
	var first, second struct {
		Diagnostics []lspDiagnostic `json:"diagnostics"`
	}
	if err := json.Unmarshal(messages[0].Params, &first); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(messages[1].Params, &second); err != nil {
		t.Fatal(err)
	}
	if len(first.Diagnostics) != 1 || first.Diagnostics[0].Code != "AX101" || first.Diagnostics[0].Range.Start.Line != 0 {
		t.Fatalf("first diagnostics=%#v", first.Diagnostics)
	}
	if len(second.Diagnostics) != 0 {
		t.Fatalf("diagnostics were not cleared: %#v", second.Diagnostics)
	}
}

func TestHoverTranspilesGuardrailAndReportsDomainTypes(t *testing.T) {
	uri := "file:///workspace/Astrixfile.ax"
	source := "guardrail \"security\" { deny when { diff.touches(\"secret/**\") } }\nstage \"verify\" { scope = ast.blast_radius }\nheal \"repair\" on failure(stage.verify) { max_attempts = 2 allow when { diff.modifies_tests(\"false\") } }\ndeploy \"prod\" after(stage.verify) { steps = [100%] rollback on { metrics.p95_latency > 250ms } }"
	deployLine := strings.Split(source, "\n")[3]
	messages := runStream(t,
		notification("textDocument/didOpen", map[string]any{"textDocument": map[string]any{"uri": uri, "text": source}}),
		request(1, "textDocument/hover", map[string]any{"textDocument": map[string]any{"uri": uri}, "position": Position{Line: 0, Character: 1}}),
		request(2, "textDocument/hover", map[string]any{"textDocument": map[string]any{"uri": uri}, "position": Position{Line: 2, Character: 2}}),
		request(3, "textDocument/hover", map[string]any{"textDocument": map[string]any{"uri": uri}, "position": Position{Line: 3, Character: strings.Index(deployLine, "metrics.p95_latency") + 2}}),
		request(4, "textDocument/hover", map[string]any{"textDocument": map[string]any{"uri": uri}, "position": Position{Line: 3, Character: strings.Index(deployLine, "100%") + 1}}),
		request(5, "textDocument/hover", map[string]any{"textDocument": map[string]any{"uri": uri}, "position": Position{Line: 3, Character: strings.Index(deployLine, "250ms") + 1}}),
	)
	if len(messages) != 6 {
		t.Fatalf("messages=%#v", messages)
	}
	var regoHover, healHover, typeHover, percentHover, durationHover hoverResult
	if err := json.Unmarshal(messages[1].Result, &regoHover); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(regoHover.Contents.Value, "package astrix.generated") || !strings.Contains(regoHover.Contents.Value, "glob.match") {
		t.Fatalf("guardrail hover lacks generated Rego: %s", regoHover.Contents.Value)
	}
	if err := json.Unmarshal(messages[2].Result, &healHover); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(healHover.Contents.Value, "allow if") {
		t.Fatalf("heal hover lacks generated allow rule: %s", healHover.Contents.Value)
	}
	if err := json.Unmarshal(messages[3].Result, &typeHover); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(typeHover.Contents.Value, "Duration") {
		t.Fatalf("metric hover=%s", typeHover.Contents.Value)
	}
	if err := json.Unmarshal(messages[4].Result, &percentHover); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(percentHover.Contents.Value, "Percent") {
		t.Fatalf("percentage hover=%s", percentHover.Contents.Value)
	}
	if err := json.Unmarshal(messages[5].Result, &durationHover); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(durationHover.Contents.Value, "Duration") {
		t.Fatalf("duration hover=%s", durationHover.Contents.Value)
	}
}

func TestFormattingReturnsCanonicalTextEdit(t *testing.T) {
	uri := "file:///workspace/Astrixfile.ax"
	source := `stage "integration" after(stage.unit,stage.lint){scope=ast.blast_radius} stage "unit" {} stage "lint" {}`
	messages := runStream(t,
		notification("textDocument/didOpen", map[string]any{"textDocument": map[string]any{"uri": uri, "text": source}}),
		request(1, "textDocument/formatting", map[string]any{"textDocument": map[string]any{"uri": uri}, "options": map[string]any{"tabSize": 2, "insertSpaces": true}}),
	)
	if len(messages) != 2 {
		t.Fatalf("messages=%#v", messages)
	}
	var edits []textEdit
	if err := json.Unmarshal(messages[1].Result, &edits); err != nil {
		t.Fatal(err)
	}
	want := "stage \"integration\" after(stage.unit, stage.lint) {\n  scope = ast.blast_radius\n}\n\nstage \"unit\" {\n}\n\nstage \"lint\" {\n}\n"
	if len(edits) != 1 || edits[0].NewText != want {
		t.Fatalf("formatting edits=%#v", edits)
	}
}
