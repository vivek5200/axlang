// Package lexer implements the hand-written, linear-time AxLang scanner.
package lexer

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/vivek5200/axlang/token"
)

// Error describes an invalid source sequence with its exact source span.
type Error struct {
	Message string
	Span    token.Span
}

func (e Error) Error() string {
	return fmt.Sprintf("%s at %s:%d:%d", e.Message, e.Span.Filename, e.Span.Line, e.Span.Column)
}

// Lexer scans a source string without regular expressions or external dependencies.
type Lexer struct {
	filename string
	source   string
	offset   int
	line     int
	column   int
}

// New constructs a lexer. An empty filename is reported as <input>.
func New(filename, source string) *Lexer {
	if filename == "" {
		filename = "<input>"
	}
	return &Lexer{filename: filename, source: source, line: 1, column: 1}
}

// Scan tokenizes the complete input, returning an EOF token or the first lexical error.
func (l *Lexer) Scan() ([]token.Token, error) {
	var out []token.Token
	for {
		l.skipTrivia()
		if l.offset >= len(l.source) {
			out = append(out, token.Token{Kind: token.EOF, Span: l.span(l.offset, l.line, l.column, 0)})
			return out, nil
		}
		t, err := l.nextToken()
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
}

func (l *Lexer) skipTrivia() {
	for l.offset < len(l.source) {
		r, _ := utf8.DecodeRuneInString(l.source[l.offset:])
		if r == '/' && l.offset+1 < len(l.source) && l.source[l.offset+1] == '/' {
			for l.offset < len(l.source) && l.source[l.offset] != '\n' && l.source[l.offset] != '\r' {
				l.advanceRune()
			}
			continue
		}
		if r != ' ' && r != '\t' && r != '\n' && r != '\r' && r != '\f' {
			return
		}
		l.advanceRune()
	}
}

func (l *Lexer) nextToken() (token.Token, error) {
	start, line, col := l.offset, l.line, l.column
	r, _ := utf8.DecodeRuneInString(l.source[l.offset:])
	switch {
	case isIdentStart(r):
		l.scanIdent()
		text := l.source[start:l.offset]
		kind := token.Ident
		if k, ok := token.Keywords[text]; ok {
			kind = k
		}
		return l.make(kind, text, text, start, line, col), nil
	case isDigit(r):
		return l.scanNumber(start, line, col)
	case r == '"':
		return l.scanString(start, line, col)
	}

	if t, ok, err := l.scanOperator(start, line, col); ok || err != nil {
		return t, err
	}
	bad, size := utf8.DecodeRuneInString(l.source[l.offset:])
	l.advanceRune()
	return token.Token{}, Error{Message: fmt.Sprintf("unexpected character %q", bad), Span: l.span(start, line, col, size)}
}

func (l *Lexer) scanIdent() {
	for l.offset < len(l.source) {
		r, _ := utf8.DecodeRuneInString(l.source[l.offset:])
		if isIdentContinue(r) {
			l.advanceRune()
			continue
		}
		// Dots are part of a qualified identifier only when followed by an identifier
		// segment; a trailing dot remains a separate punctuation token.
		if r == '.' && l.offset+1 < len(l.source) && isIdentStart(rune(l.source[l.offset+1])) {
			l.advanceRune()
			continue
		}
		break
	}
}

func (l *Lexer) scanNumber(start, line, col int) (token.Token, error) {
	for l.offset < len(l.source) && isDigit(rune(l.source[l.offset])) {
		l.advanceRune()
	}
	if l.offset < len(l.source) && l.source[l.offset] == '.' && l.offset+1 < len(l.source) && isDigit(rune(l.source[l.offset+1])) {
		l.advanceRune()
		for l.offset < len(l.source) && isDigit(rune(l.source[l.offset])) {
			l.advanceRune()
		}
	}
	text := l.source[start:l.offset]
	unitStart := l.offset
	for l.offset < len(l.source) && isASCIILetter(l.source[l.offset]) {
		l.advanceRune()
	}
	unit := l.source[unitStart:l.offset]
	if unit != "" {
		switch unit {
		case "ms", "s", "m", "h":
			return l.make(token.Duration, l.source[start:l.offset], text, start, line, col), nil
		default:
			return token.Token{}, Error{Message: fmt.Sprintf("unknown numeric unit %q", unit), Span: l.span(start, line, col, l.offset-start)}
		}
	}
	if l.offset < len(l.source) && l.source[l.offset] == '%' {
		l.advanceRune()
		return l.make(token.Percent, l.source[start:l.offset], text, start, line, col), nil
	}
	return l.make(token.Number, text, text, start, line, col), nil
}

func (l *Lexer) scanString(start, line, col int) (token.Token, error) {
	l.advanceRune() // opening quote
	var value strings.Builder
	for l.offset < len(l.source) {
		r, _ := utf8.DecodeRuneInString(l.source[l.offset:])
		if r == '"' {
			l.advanceRune()
			return l.make(token.String, l.source[start:l.offset], value.String(), start, line, col), nil
		}
		if r == '\n' || r == '\r' {
			return token.Token{}, Error{Message: "newline in string literal", Span: l.span(start, line, col, l.offset-start)}
		}
		if r == '\\' {
			escapeStart, escapeLine, escapeColumn := l.offset, l.line, l.column
			l.advanceRune()
			if l.offset >= len(l.source) {
				break
			}
			esc := l.source[l.offset]
			l.advanceRune()
			switch esc {
			case 'n':
				value.WriteByte('\n')
			case 'r':
				value.WriteByte('\r')
			case 't':
				value.WriteByte('\t')
			case '"':
				value.WriteByte('"')
			case '\\':
				value.WriteByte('\\')
			default:
				return token.Token{}, Error{Message: fmt.Sprintf("invalid escape sequence \\%c", esc), Span: l.span(escapeStart, escapeLine, escapeColumn, l.offset-escapeStart)}
			}
			continue
		}
		value.WriteRune(r)
		l.advanceRune()
	}
	return token.Token{}, Error{Message: "unterminated string literal", Span: l.span(start, line, col, l.offset-start)}
}

func (l *Lexer) scanOperator(start, line, col int) (token.Token, bool, error) {
	ch := l.source[l.offset]
	var kind token.Kind
	width := 1
	switch ch {
	case '{':
		kind = token.LBrace
	case '}':
		kind = token.RBrace
	case '(':
		kind = token.LParen
	case ')':
		kind = token.RParen
	case '[':
		kind = token.LBracket
	case ']':
		kind = token.RBracket
	case ',':
		kind = token.Comma
	case '.':
		kind = token.Dot
	case '-':
		kind = token.Minus
	case '=':
		kind = token.Assign
	case '!':
		if l.peek('=') {
			kind = token.NotEqual
			width = 2
		} else {
			return token.Token{}, false, Error{Message: "expected '=' after '!'", Span: l.span(start, line, col, 1)}
		}
	case '<':
		kind = token.Less
		if l.peek('=') {
			kind = token.LessEqual
			width = 2
		}
	case '>':
		kind = token.Greater
		if l.peek('=') {
			kind = token.GreaterEqual
			width = 2
		}
	default:
		return token.Token{}, false, nil
	}
	if ch == '=' && l.peek('=') {
		kind = token.Equal
		width = 2
	}
	for i := 0; i < width; i++ {
		l.advanceRune()
	}
	text := l.source[start:l.offset]
	return l.make(kind, text, text, start, line, col), true, nil
}

func (l *Lexer) peek(ch byte) bool { return l.offset+1 < len(l.source) && l.source[l.offset+1] == ch }
func (l *Lexer) make(kind token.Kind, text, value string, offset, line, col int) token.Token {
	return token.Token{Kind: kind, Text: text, Value: value, Span: l.span(offset, line, col, l.offset-offset)}
}
func (l *Lexer) span(offset, line, col, length int) token.Span {
	return token.Span{Filename: l.filename, Line: line, Column: col, Offset: offset, Length: length}
}
func (l *Lexer) advanceRune() {
	r, size := utf8.DecodeRuneInString(l.source[l.offset:])
	l.offset += size
	if r == '\r' {
		// Treat CRLF as one line break while retaining byte-accurate offsets.
		if l.offset < len(l.source) && l.source[l.offset] == '\n' {
			l.offset++
		}
		l.line++
		l.column = 1
	} else if r == '\n' {
		l.line++
		l.column = 1
	} else {
		l.column += size
	}
}
func isDigit(r rune) bool         { return r >= '0' && r <= '9' }
func isASCIILetter(b byte) bool   { return b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' }
func isIdentStart(r rune) bool    { return r == '_' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' }
func isIdentContinue(r rune) bool { return isIdentStart(r) || isDigit(r) }
