package sema

import (
	"testing"

	"github.com/vivek5200/axlang/parser"
)

func TestAdversarialSemanticBoundaries(t *testing.T) {
	tests := []struct {
		name, source, code string
	}{
		{"attempt zero", `stage "s" {} heal "h" on failure(stage.s) { max_attempts = 0 }`, "AX001"},
		{"attempt negative", `stage "s" {} heal "h" on failure(stage.s) { max_attempts = -1 }`, "AX001"},
		{"attempt over limit", `stage "s" {} heal "h" on failure(stage.s) { max_attempts = 6 }`, "AX001"},
		{"attempt fractional", `stage "s" {} heal "h" on failure(stage.s) { max_attempts = 1.5 }`, "AX001"},
		{"heal missing stage", `heal "h" on failure(stage.missing) { max_attempts = 1 }`, "AX002"},
		{"deploy missing stage", `deploy "d" after(stage.missing) { steps = [100%] }`, "AX002"},
		{"duration greater than percent", `stage "s" {} deploy "d" after(stage.s) { steps = [100%] rollback on { metrics.p95_latency > 5% } }`, "AX003"},
		{"percent equals duration", `stage "s" {} deploy "d" after(stage.s) { steps = [100%] rollback on { metrics.http_5xx_rate == 250ms } }`, "AX003"},
		{"string less than duration", `stage "s" {} deploy "d" after(stage.s) { steps = [100%] rollback on { "slow" < metrics.p95_latency } }`, "AX003"},
		{"number greater than percent", `stage "s" {} deploy "d" after(stage.s) { steps = [100%] rollback on { 1 > metrics.http_5xx_rate } }`, "AX003"},
		{"zero percent step", `stage "s" {} deploy "d" after(stage.s) { steps = [0%, 100%] }`, "AX004"},
		{"descending steps", `stage "s" {} deploy "d" after(stage.s) { steps = [10%, 5%, 100%] }`, "AX004"},
		{"duplicate steps", `stage "s" {} deploy "d" after(stage.s) { steps = [5%, 5%, 100%] }`, "AX004"},
		{"missing final rollout", `stage "s" {} deploy "d" after(stage.s) { steps = [1%, 50%] }`, "AX004"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			file, parseDiagnostics := parser.New("adversarial.ax", test.source).Parse()
			if len(parseDiagnostics) != 0 {
				t.Fatalf("parse diagnostics: %+v", parseDiagnostics)
			}
			diagnostics := Check(file)
			for _, diagnostic := range diagnostics {
				if diagnostic.Code == test.code {
					return
				}
			}
			t.Fatalf("diagnostics=%+v; expected %s", diagnostics, test.code)
		})
	}
}

func TestStageCyclesAreNotExpressibleInAxLang(t *testing.T) {
	// Stage declarations have no dependency field in the grammar, so a stage
	// cycle cannot be encoded. A heal/deploy self-reference is still rejected
	// unless an actual stage with that name is declared.
	file, parseDiagnostics := parser.New("cycle.ax", `heal "h" on failure(stage.h) { max_attempts = 1 }`).Parse()
	if len(parseDiagnostics) != 0 {
		t.Fatalf("parse diagnostics: %+v", parseDiagnostics)
	}
	diagnostics := Check(file)
	if len(diagnostics) == 0 || diagnostics[0].Code != "AX002" {
		t.Fatalf("self-reference diagnostics=%+v; expected AX002", diagnostics)
	}
}
