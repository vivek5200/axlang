package rego

import (
	"github.com/vivek5200/axlang/parser"
	"strings"
	"testing"
)

func TestCompileBooleanPoliciesToRegoV1(t *testing.T) {
	source := `guardrail "g" { deny when { a == "x" and diff.touches("one/**", "two/**") or b != "y" } }`
	file, diags := parser.New("test.ax", source).Parse()
	if len(diags) > 0 {
		t.Fatalf("parse: %#v", diags)
	}
	got, err := Compile(file)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"import rego.v1", "default deny := false", "default allow := false", "deny if {", "input.a == \"x\"", "some f in input.modified_files", "glob.match(\"one/**\", [\"/\"], f)", "glob.match(\"two/**\", [\"/\"], f)", "input.b != \"y\""} {
		if !strings.Contains(got, want) {
			t.Errorf("Rego output missing %q:\n%s", want, got)
		}
	}
}

func TestCompileConvertsNativeUnits(t *testing.T) {
	file, diags := parser.New("test.ax", `heal "h" on failure(stage.s) { max_attempts = 1 allow when { metrics.p95_latency > 250ms and metrics.http_5xx_rate > 0.5% } }`).Parse()
	if len(diags) > 0 {
		t.Fatalf("parse: %#v", diags)
	}
	got, err := Compile(file)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "input.metrics.p95_latency > 0.25") || !strings.Contains(got, "input.metrics.http_5xx_rate > 0.005") {
		t.Fatalf("unit conversion missing:\n%s", got)
	}
}

func TestCompileModifiesTestsCallAsRegoRule(t *testing.T) {
	file, diags := parser.New("test.ax", `heal "h" on failure(stage.s) { max_attempts = 1 allow when { diff.modifies_tests("false") } }`).Parse()
	if len(diags) > 0 {
		t.Fatalf("parse: %#v", diags)
	}
	got, err := Compile(file)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "ax_diff_modifies_tests(expected) if") || !strings.Contains(got, "ax_diff_modifies_tests(\"false\")") {
		t.Fatalf("modifies_tests helper missing:\n%s", got)
	}
}
