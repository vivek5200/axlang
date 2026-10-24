package parser

import (
	"github.com/vivek5200/axlang/ast"
	"github.com/vivek5200/axlang/token"
	"strings"
	"testing"
)

const canonical = "workspace \"core-platform\" {\n" +
	" languages = [\"go\", \"typescript\"]\n cache = \"s3://astrix-remote-cache\"\n}\n" +
	"guardrail \"security_boundaries\" {\n deny when { actor.type == \"autonomous_llm\" and diff.touches(\"internal/auth/**\", \"migrations/**\") }\n" +
	" require_approval when { target.env == \"production\" or diff.touches(\"infra/**\") }\n}\n" +
	"stage \"verify\" { scope = ast.blast_radius quarantine_threshold = 0.05 network = \"none\" }\n" +
	"heal \"triage_loop\" on failure(stage.verify) { model = \"qwen/qwen-2.5-coder-32b-instruct\" max_attempts = 3 allow when { actor.type == \"autonomous_llm\" and diff.modifies_tests(\"false\") } }\n" +
	"deploy \"prod_canary\" after(stage.verify) { steps = [1%, 5%, 25%, 100%] every 2m rollback on { metrics.p95_latency > 250ms or metrics.http_5xx_rate > 0.5% } }"

func TestParseCanonicalFile(t *testing.T) {
	file, diags := New("Astrixfile.ax", canonical).Parse()
	if len(diags) != 0 {
		for _, d := range diags {
			t.Error(d.Format(canonical))
		}
	}
	if len(file.Decls) != 5 {
		t.Fatalf("declarations=%d, want 5", len(file.Decls))
	}
	heal := file.Decls[3].(*ast.HealDecl)
	if heal.OnFailure != "verify" || heal.Allow == nil {
		t.Fatalf("heal parsed incorrectly: %#v", heal)
	}
	deploy := file.Decls[4].(*ast.DeployDecl)
	if deploy.AfterStage != "verify" || deploy.Rollback == nil {
		t.Fatalf("deploy parsed incorrectly: %#v", deploy)
	}
	steps := deploy.Fields[0]
	if steps.Every == nil || steps.Every.Value != "2" {
		t.Fatalf("interval was not parsed: %#v", steps)
	}
}

func TestPrattPrecedenceAndGrouping(t *testing.T) {
	file, diags := New("expr.ax", `guardrail "g" { deny when { a == b or c == d and e == f } }`).Parse()
	if len(diags) != 0 {
		t.Fatalf("diagnostics: %#v", diags)
	}
	expr := file.Decls[0].(*ast.GuardrailDecl).Rules[0].Condition
	or, ok := expr.(*ast.BinaryExpr)
	if !ok || or.Operator != token.Or {
		t.Fatalf("root=%#v, want OR", expr)
	}
	and, ok := or.Right.(*ast.BinaryExpr)
	if !ok || and.Operator != token.And {
		t.Fatalf("right=%#v, want AND", or.Right)
	}
	grouped, diags := New("group.ax", `guardrail "g" { deny when { (a == b or c == d) and e == f } }`).Parse()
	if len(diags) != 0 {
		t.Fatalf("diagnostics: %#v", diags)
	}
	root := grouped.Decls[0].(*ast.GuardrailDecl).Rules[0].Condition.(*ast.BinaryExpr)
	if root.Operator != token.And {
		t.Fatalf("grouped root = %s", root.Operator)
	}
	if root.Left.(*ast.BinaryExpr).Operator != token.Or {
		t.Fatalf("grouped left = %#v", root.Left)
	}
}

func TestParserRecoversAtNextTopLevelDeclaration(t *testing.T) {
	source := "stage \"broken\" { scope ast.blast_radius }\nstage \"valid\" { network = \"none\" }"
	file, diags := New("recover.ax", source).Parse()
	if len(diags) != 1 {
		t.Fatalf("diagnostics=%d: %#v", len(diags), diags)
	}
	if len(file.Decls) != 1 || file.Decls[0].(*ast.StageDecl).Name != "valid" {
		t.Fatalf("recovered declarations: %#v", file.Decls)
	}
}

func TestDiagnosticFormatIncludesCaretAndHelp(t *testing.T) {
	d := Diagnostic{Code: "AX101", Message: "expected expression", Span: token.Span{Filename: "x.ax", Line: 2, Column: 3, Length: 2}, Help: "provide a value"}
	formatted := d.Format("one\n  xx\n")
	for _, part := range []string{"error[AX101]", "--> x.ax:2:3", "2 |   xx", "^^", "= help: provide a value"} {
		if !strings.Contains(formatted, part) {
			t.Errorf("diagnostic lacks %q: %s", part, formatted)
		}
	}
}
