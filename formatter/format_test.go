package formatter

import (
	"strings"
	"testing"

	"github.com/vivek5200/axlang/parser"
)

const messy = `workspace "core-platform" { languages=["go","typescript"] cache="s3://cache" }
guardrail "security_boundaries" { deny when { actor.type=="autonomous_llm" and diff.touches("internal/auth/**","migrations/**") } require_approval when { target.env=="production" or diff.touches("infra/**") } }
stage "verify" { scope=ast.blast_radius quarantine_threshold=0.05 network="none" }
heal "triage_loop" on failure(stage.verify) { model="model" max_attempts=3 allow when { actor.type=="autonomous_llm" and diff.modifies_tests("false") } }
deploy "prod_canary" after(stage.verify) { steps=[1%,5%,25%,100%] every 2m rollback on { metrics.p95_latency>250ms or metrics.http_5xx_rate>0.5% } }
`

func TestFormatIsCanonicalAndIdempotent(t *testing.T) {
	firstFile, diags := parser.New("test.ax", messy).Parse()
	if len(diags) != 0 {
		t.Fatalf("parse input: %v", diags)
	}
	first := Format(firstFile)
	secondFile, diags := parser.New("test.ax", first).Parse()
	if len(diags) != 0 {
		t.Fatalf("parse formatted output: %v\n%s", diags, first)
	}
	second := Format(secondFile)
	if first != second {
		t.Fatalf("formatter is not idempotent:\n--- first ---\n%s\n--- second ---\n%s", first, second)
	}
	for _, expected := range []string{
		"  languages = [\"go\", \"typescript\"]",
		"  cache     = \"s3://cache\"",
		"  deny when {\n    actor.type == \"autonomous_llm\" and diff.touches(\"internal/auth/**\", \"migrations/**\")",
		"  steps = [1%, 5%, 25%, 100%] every 2m",
		"    metrics.p95_latency > 250ms or metrics.http_5xx_rate > 0.5%",
	} {
		if !strings.Contains(first, expected) {
			t.Errorf("canonical output missing %q:\n%s", expected, first)
		}
	}
}

func TestFormatEmptyFile(t *testing.T) {
	file, diags := parser.New("empty.ax", "\n// comment-only file\n").Parse()
	if len(diags) != 0 {
		t.Fatal(diags)
	}
	if got := Format(file); got != "" {
		t.Fatalf("Format(empty)=%q", got)
	}
}
