// Package ir defines AxLang's JSON-serializable runtime execution plan.
package ir

import (
	"github.com/vivek5200/axlang/ast"
	"github.com/vivek5200/axlang/token"
)

// ExecutionPlan is the validated, runtime-facing representation of an Astrixfile.
type ExecutionPlan struct {
	Version     string          `json:"version"`
	Workspace   *WorkspacePlan  `json:"workspace,omitempty"`
	Guardrails  []GuardrailPlan `json:"guardrails,omitempty"`
	Stages      []StagePlan     `json:"stages,omitempty"`
	Heals       []HealPlan      `json:"heals,omitempty"`
	Deployments []DeployPlan    `json:"deployments,omitempty"`
}

type WorkspacePlan struct {
	Name   string      `json:"name"`
	Fields []FieldPlan `json:"fields,omitempty"`
}
type GuardrailPlan struct {
	Name  string     `json:"name"`
	Rules []RulePlan `json:"rules"`
}
type RulePlan struct {
	Effect    string   `json:"effect"`
	Condition ExprPlan `json:"condition"`
}
type StagePlan struct {
	Name    string      `json:"name"`
	After   []string    `json:"after,omitempty"`
	Fields  []FieldPlan `json:"fields,omitempty"`
	Image   string      `json:"image,omitempty"`
	Command string      `json:"command,omitempty"`
	Mounts  []string    `json:"mounts,omitempty"`
}
type HealPlan struct {
	Name      string      `json:"name"`
	OnFailure string      `json:"on_failure"`
	Fields    []FieldPlan `json:"fields,omitempty"`
	Allow     *ExprPlan   `json:"allow,omitempty"`
}
type DeployPlan struct {
	Name     string      `json:"name"`
	After    string      `json:"after"`
	Fields   []FieldPlan `json:"fields,omitempty"`
	Rollback *ExprPlan   `json:"rollback,omitempty"`
}
type FieldPlan struct {
	Name  string       `json:"name"`
	Value ExprPlan     `json:"value"`
	Every *LiteralPlan `json:"every,omitempty"`
}
type LiteralPlan struct {
	Type  string `json:"type"`
	Value string `json:"value"`
	Text  string `json:"text,omitempty"`
}

// ExprPlan is a tagged expression tree suitable for JSON serialization.
type ExprPlan struct {
	Kind      string       `json:"kind"`
	Name      string       `json:"name,omitempty"`
	Operator  string       `json:"operator,omitempty"`
	Literal   *LiteralPlan `json:"literal,omitempty"`
	Left      *ExprPlan    `json:"left,omitempty"`
	Right     *ExprPlan    `json:"right,omitempty"`
	Callee    *ExprPlan    `json:"callee,omitempty"`
	Arguments []ExprPlan   `json:"arguments,omitempty"`
	Elements  []ExprPlan   `json:"elements,omitempty"`
}

// FromAST converts a semantically checked file into a runtime plan.
func FromAST(file *ast.File) ExecutionPlan {
	plan := ExecutionPlan{Version: "1"}
	if file == nil {
		return plan
	}
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.WorkspaceDecl:
			plan.Workspace = &WorkspacePlan{Name: d.Name, Fields: fields(d.Fields)}
		case *ast.GuardrailDecl:
			g := GuardrailPlan{Name: d.Name, Rules: make([]RulePlan, 0, len(d.Rules))}
			for _, r := range d.Rules {
				effect := "deny"
				if r.Kind == ast.RequireApprovalRule {
					effect = "require_approval"
				}
				g.Rules = append(g.Rules, RulePlan{Effect: effect, Condition: expr(r.Condition)})
			}
			plan.Guardrails = append(plan.Guardrails, g)
		case *ast.StageDecl:
			stage := StagePlan{Name: d.Name, Fields: fields(d.Fields)}
			for _, field := range d.Fields {
				switch field.Name {
				case "image":
					stage.Image = stringField(field.Value)
				case "command":
					stage.Command = stringField(field.Value)
				case "mounts":
					stage.Mounts = stringListField(field.Value)
				}
			}
			for _, dependency := range d.AfterStages {
				stage.After = append(stage.After, dependency.Name)
			}
			plan.Stages = append(plan.Stages, stage)
		case *ast.HealDecl:
			h := HealPlan{Name: d.Name, OnFailure: d.OnFailure, Fields: fields(d.Fields)}
			if d.Allow != nil {
				v := expr(d.Allow)
				h.Allow = &v
			}
			plan.Heals = append(plan.Heals, h)
		case *ast.DeployDecl:
			dp := DeployPlan{Name: d.Name, After: d.AfterStage, Fields: fields(d.Fields)}
			if d.Rollback != nil {
				v := expr(d.Rollback)
				dp.Rollback = &v
			}
			plan.Deployments = append(plan.Deployments, dp)
		}
	}
	return plan
}

func stringField(e ast.Expr) string {
	if v, ok := e.(*ast.LiteralExpr); ok && v.Kind == token.String {
		return v.Value
	}
	return ""
}
func stringListField(e ast.Expr) []string {
	list, ok := e.(*ast.ListExpr)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(list.Elements))
	for _, item := range list.Elements {
		if value := stringField(item); value != "" {
			out = append(out, value)
		}
	}
	return out
}

func fields(in []ast.Field) []FieldPlan {
	out := make([]FieldPlan, 0, len(in))
	for _, f := range in {
		v := expr(f.Value)
		fp := FieldPlan{Name: f.Name, Value: v}
		if f.Every != nil {
			fp.Every = literal(f.Every)
		}
		out = append(out, fp)
	}
	return out
}
func expr(in ast.Expr) ExprPlan {
	switch n := in.(type) {
	case *ast.IdentExpr:
		return ExprPlan{Kind: "ident", Name: n.Name}
	case *ast.LiteralExpr:
		return ExprPlan{Kind: "literal", Literal: literal(n)}
	case *ast.BinaryExpr:
		left, right := expr(n.Left), expr(n.Right)
		return ExprPlan{Kind: "binary", Operator: n.Operator.String(), Left: &left, Right: &right}
	case *ast.CallExpr:
		callee := expr(n.Callee)
		args := make([]ExprPlan, len(n.Arguments))
		for i, a := range n.Arguments {
			args[i] = expr(a)
		}
		return ExprPlan{Kind: "call", Callee: &callee, Arguments: args}
	case *ast.ListExpr:
		items := make([]ExprPlan, len(n.Elements))
		for i, item := range n.Elements {
			items[i] = expr(item)
		}
		return ExprPlan{Kind: "list", Elements: items}
	default:
		return ExprPlan{Kind: "invalid"}
	}
}
func literal(n *ast.LiteralExpr) *LiteralPlan {
	return &LiteralPlan{Type: n.Kind.String(), Value: n.Value, Text: n.Text}
}

// SpanOf returns source location metadata for downstream diagnostics and tooling.
func SpanOf(e ast.Expr) token.Span {
	if e == nil {
		return token.Span{}
	}
	return e.Span()
}
