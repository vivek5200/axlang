package sema

import (
	"github.com/vivek5200/axlang/parser"
	"testing"
)

func checkFixture(t *testing.T, source string) []parser.Diagnostic {
	t.Helper()
	file, parseDiags := parser.New("test.ax", source).Parse()
	if len(parseDiags) > 0 {
		t.Fatalf("parse diagnostics: %#v", parseDiags)
	}
	return Check(file)
}

func TestValidSemanticInvariants(t *testing.T) {
	source := `stage "verify" {} heal "h" on failure(stage.verify) { max_attempts = 5 } deploy "d" after(stage.verify) { steps = [1%, 5%, 100%] }`
	if d := checkFixture(t, source); len(d) != 0 {
		t.Fatalf("unexpected diagnostics: %#v", d)
	}
}

func TestAX001AttemptBoundsAndIntegerRequirement(t *testing.T) {
	for _, value := range []string{"0", "6", "2.5"} {
		t.Run(value, func(t *testing.T) {
			d := checkFixture(t, `stage "s" {} heal "h" on failure(stage.s) { max_attempts = `+value+` }`)
			if len(d) == 0 || d[0].Code != "AX001" {
				t.Fatalf("diagnostics=%#v, want AX001", d)
			}
		})
	}
}

func TestAX002StageReferences(t *testing.T) {
	d := checkFixture(t, `heal "h" on failure(stage.missing) { max_attempts = 1 }`)
	if len(d) == 0 || d[0].Code != "AX002" {
		t.Fatalf("diagnostics=%#v, want AX002", d)
	}
}

func TestAX002ValidStageDependencyDAG(t *testing.T) {
	d := checkFixture(t, `stage "unit" {} stage "lint" {} stage "integration" after(stage.unit, stage.lint) {} stage "e2e" after(stage.integration) {}`)
	if len(d) != 0 {
		t.Fatalf("unexpected diagnostics for valid stage DAG: %#v", d)
	}
}

func TestAX002RejectsStageDependencyCycles(t *testing.T) {
	d := checkFixture(t, `stage "a" after(stage.b) {} stage "b" after(stage.c) {} stage "c" after(stage.a) {} stage "downstream" after(stage.a) {}`)
	if len(d) != 3 {
		t.Fatalf("diagnostics=%#v, want one AX002 for each cyclic stage and none for downstream", d)
	}
	for _, diagnostic := range d {
		if diagnostic.Code != "AX002" || diagnostic.Span.Length == 0 {
			t.Fatalf("cycle diagnostic missing AX002 or stage span: %#v", diagnostic)
		}
	}
}

func TestAX002RejectsUndeclaredStagePrerequisite(t *testing.T) {
	d := checkFixture(t, `stage "integration" after(stage.missing) {}`)
	if len(d) != 1 || d[0].Code != "AX002" || d[0].Span.Line != 1 {
		t.Fatalf("diagnostics=%#v, want AX002 at dependency reference", d)
	}
}

func TestAX003MetricUnits(t *testing.T) {
	source := `stage "s" {} deploy "d" after(stage.s) { steps = [100%] rollback on { metrics.p95_latency > 5% or metrics.http_5xx_rate > 2s or metrics.p95_latency == 1 } }`
	d := checkFixture(t, source)
	if len(d) != 3 || d[0].Code != "AX003" || d[1].Code != "AX003" || d[2].Code != "AX003" {
		t.Fatalf("diagnostics=%#v, want three AX003 errors", d)
	}
}

func TestAX004MonotonicCanarySteps(t *testing.T) {
	for _, steps := range []string{"[5%, 1%, 100%]", "[1%, 5%]", "[1%, 1%, 100%]"} {
		t.Run(steps, func(t *testing.T) {
			d := checkFixture(t, `stage "s" {} deploy "d" after(stage.s) { steps = `+steps+` }`)
			if len(d) == 0 || d[0].Code != "AX004" {
				t.Fatalf("diagnostics=%#v, want AX004", d)
			}
		})
	}
}
