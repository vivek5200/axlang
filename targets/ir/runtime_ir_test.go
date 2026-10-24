package ir

import (
	"testing"

	"github.com/vivek5200/axlang/parser"
)

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
