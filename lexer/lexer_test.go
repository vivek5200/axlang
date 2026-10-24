package lexer

import (
	"testing"

	"github.com/vivek5200/axlang/token"
)

func TestScanCanonicalLiteralsAndQualifiedNames(t *testing.T) {
	source := `metrics.p95_latency > 250ms and rate == 0.5% or f("hi\nthere", "x") // tail`
	got, err := New("fixture.ax", source).Scan()
	if err != nil {
		t.Fatal(err)
	}
	want := []token.Kind{token.Ident, token.Greater, token.Duration, token.And, token.Ident,
		token.Equal, token.Percent, token.Or, token.Ident, token.LParen, token.String,
		token.Comma, token.String, token.RParen, token.EOF}
	if len(got) != len(want) {
		t.Fatalf("got %d tokens, want %d: %#v", len(got), len(want), got)
	}
	for i, kind := range want {
		if got[i].Kind != kind {
			t.Errorf("token %d kind = %s, want %s", i, got[i].Kind, kind)
		}
	}
	if got[0].Text != "metrics.p95_latency" {
		t.Errorf("qualified name = %q", got[0].Text)
	}
	if got[2].Value != "250" {
		t.Errorf("duration value = %q", got[2].Value)
	}
	if got[6].Value != "0.5" {
		t.Errorf("percent value = %q", got[6].Value)
	}
	if got[10].Value != "hi\nthere" {
		t.Errorf("decoded string = %q", got[10].Value)
	}
	if got[0].Span != (token.Span{Filename: "fixture.ax", Line: 1, Column: 1, Offset: 0, Length: 19}) {
		t.Errorf("first span = %+v", got[0].Span)
	}
}

func TestScanKeywordsPunctuationAndNumericKinds(t *testing.T) {
	got, err := New("", `workspace "x" { max_attempts = 3 steps = [1%, 2s] }`).Scan()
	if err != nil {
		t.Fatal(err)
	}
	want := []token.Kind{token.Workspace, token.String, token.LBrace, token.MaxAttempts, token.Assign,
		token.Number, token.Steps, token.Assign, token.LBracket, token.Percent, token.Comma,
		token.Duration, token.RBracket, token.RBrace, token.EOF}
	if len(got) != len(want) {
		t.Fatalf("got %d tokens, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i].Kind != want[i] {
			t.Errorf("token %d = %s, want %s", i, got[i].Kind, want[i])
		}
	}
}

func TestScanTracksMultilineByteSpans(t *testing.T) {
	got, err := New("utf8.ax", "// comment\n\"é\"").Scan()
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Span != (token.Span{Filename: "utf8.ax", Line: 2, Column: 1, Offset: 11, Length: 4}) {
		t.Fatalf("string span = %+v", got[0].Span)
	}
}

func TestScanErrors(t *testing.T) {
	for _, source := range []string{`"unterminated`, `"bad\q"`, `5fortnights`, `!`} {
		t.Run(source, func(t *testing.T) {
			if _, err := New("bad.ax", source).Scan(); err == nil {
				t.Fatal("expected lexical error")
			}
		})
	}
}

func TestTrailingDotIsPunctuation(t *testing.T) {
	got, err := New("", "metrics.").Scan()
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Kind != token.Ident || got[1].Kind != token.Dot {
		t.Fatalf("tokens = %#v", got)
	}
}
