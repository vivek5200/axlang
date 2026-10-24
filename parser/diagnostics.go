// Package parser parses AxLang source and renders compiler diagnostics.
package parser

import (
	"fmt"
	"strings"

	"github.com/vivek5200/axlang/token"
)

// Diagnostic is a structured, source-located compiler message.
type Diagnostic struct {
	Code    string
	Message string
	Span    token.Span
	Help    string
}

func (d Diagnostic) Error() string {
	if d.Code == "" {
		return d.Message
	}
	return fmt.Sprintf("error[%s]: %s", d.Code, d.Message)
}

// Format renders a Rust/Cargo-style diagnostic with a source excerpt and caret.
func (d Diagnostic) Format(source string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "error[%s]: %s\n --> %s:%d:%d\n", d.Code, d.Message, d.Span.Filename, d.Span.Line, d.Span.Column)
	lines := strings.Split(strings.ReplaceAll(source, "\r\n", "\n"), "\n")
	line := ""
	if d.Span.Line > 0 && d.Span.Line <= len(lines) {
		line = lines[d.Span.Line-1]
	}
	width := len(fmt.Sprint(d.Span.Line))
	fmt.Fprintf(&b, "%*d | %s\n", width, d.Span.Line, line)
	caretLen := d.Span.Length
	if caretLen < 1 {
		caretLen = 1
	}
	// Tabs occupy one source column in the lexer and are preserved here.
	fmt.Fprintf(&b, "%*s | %s%s", width, "", strings.Repeat(" ", max(0, d.Span.Column-1)), strings.Repeat("^", caretLen))
	if d.Help != "" {
		fmt.Fprintf(&b, "\n = help: %s", d.Help)
	}
	return b.String()
}

type parseError struct{ diagnostic Diagnostic }

func (e parseError) Error() string { return e.diagnostic.Error() }
