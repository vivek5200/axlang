package ir

import (
	"testing"

	"github.com/vivek5200/axlang/parser"
)

func TestSandboxStageFieldsRoundTripToRuntimePlan(t *testing.T) {
	source := `stage "verify" {
 image = "rust:1.82-slim"
 command = "cargo test --offline"
 mounts = ["~/.cargo:/root/.cargo:ro"]
}
`
	file, diagnostics := parser.New("Astrixfile.ax", source).Parse()
	if len(diagnostics) != 0 {
		t.Fatalf("parse diagnostics: %+v", diagnostics)
	}
	plan := FromAST(file)
	if len(plan.Stages) != 1 || plan.Stages[0].Image != "rust:1.82-slim" || plan.Stages[0].Command != "cargo test --offline" || len(plan.Stages[0].Mounts) != 1 || plan.Stages[0].Mounts[0] != "~/.cargo:/root/.cargo:ro" {
		t.Fatalf("sandbox stage fields lost: %+v", plan.Stages)
	}
}

func TestFromASTPreservesStageDependencies(t *testing.T) {
	file, diagnostics := parser.New("stages.ax", `stage "unit" {} stage "integration" after(stage.unit, stage.lint) {} stage "lint" {}`).Parse()
	if len(diagnostics) != 0 {
		t.Fatal(diagnostics)
	}
	plan := FromAST(file)
	if len(plan.Stages) != 3 {
		t.Fatalf("stages = %#v", plan.Stages)
	}
	got := plan.Stages[1].After
	if len(got) != 2 || got[0] != "unit" || got[1] != "lint" {
		t.Fatalf("stage dependencies = %#v", got)
	}
}
