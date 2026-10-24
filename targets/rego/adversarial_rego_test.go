package rego

import (
	"strings"
	"testing"

	"github.com/vivek5200/axlang/parser"
)

func TestAdversarialMixedBooleanPrecedenceExpandsDeterministically(t *testing.T) {
	source := `guardrail "security" { deny when { (actor.type == "autonomous_llm" or actor.type == "bot") and (diff.touches("auth/**") or diff.touches("db/**")) } }`
	file, diagnostics := parser.New("policy.ax", source).Parse()
	if len(diagnostics) != 0 {
		t.Fatalf("parse diagnostics: %+v", diagnostics)
	}
	first, err := Compile(file)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Compile(file)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatal("same policy AST transpiled to different Rego output")
	}
	if got := strings.Count(first, "deny if {"); got != 4 {
		t.Fatalf("got %d deny alternatives, want 4\n%s", got, first)
	}
	for _, term := range []string{
		`input.actor.type == "autonomous_llm"`,
		`input.actor.type == "bot"`,
		`glob.match("auth/**", ["/"], f)`,
		`glob.match("db/**", ["/"], f)`,
	} {
		if !strings.Contains(first, term) {
			t.Errorf("generated Rego missing %q", term)
		}
	}
}
