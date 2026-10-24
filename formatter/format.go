// Package formatter prints AxLang syntax in a stable canonical layout.
package formatter

import (
	"strconv"
	"strings"

	"github.com/vivek5200/axlang/ast"
	"github.com/vivek5200/axlang/token"
)

// Format returns the canonical representation of a parsed AxLang file.
func Format(file *ast.File) string {
	if file == nil {
		return ""
	}
	var out strings.Builder
	for i, decl := range file.Decls {
		if i > 0 {
			out.WriteByte('\n')
		}
		formatDecl(&out, decl)
	}
	if len(file.Decls) > 0 {
		out.WriteByte('\n')
	}
	return out.String()
}

func formatDecl(out *strings.Builder, decl ast.Decl) {
	switch d := decl.(type) {
	case *ast.WorkspaceDecl:
		out.WriteString("workspace ")
		quoted(out, d.Name)
		out.WriteString(" {\n")
		formatFields(out, d.Fields, 1, true)
		out.WriteString("}\n")
	case *ast.StageDecl:
		out.WriteString("stage ")
		quoted(out, d.Name)
		out.WriteString(" {\n")
		formatFields(out, d.Fields, 1, true)
		out.WriteString("}\n")
	case *ast.GuardrailDecl:
		out.WriteString("guardrail ")
		quoted(out, d.Name)
		out.WriteString(" {\n")
		for _, rule := range d.Rules {
			out.WriteString("  ")
			if rule.Kind == ast.RequireApprovalRule {
				out.WriteString("require_approval")
			} else {
				out.WriteString("deny")
			}
			out.WriteString(" when {\n    ")
			out.WriteString(formatExpr(rule.Condition, 0))
			out.WriteString("\n  }\n")
		}
		out.WriteString("}\n")
	case *ast.HealDecl:
		out.WriteString("heal ")
		quoted(out, d.Name)
		out.WriteString(" on failure(stage.")
		out.WriteString(d.OnFailure)
		out.WriteString(") {\n")
		formatFields(out, d.Fields, 1, true)
		if d.Allow != nil {
			out.WriteString("  allow when {\n    ")
			out.WriteString(formatExpr(d.Allow, 0))
			out.WriteString("\n  }\n")
		}
		out.WriteString("}\n")
	case *ast.DeployDecl:
		out.WriteString("deploy ")
		quoted(out, d.Name)
		out.WriteString(" after(stage.")
		out.WriteString(d.AfterStage)
		out.WriteString(") {\n")
		formatFields(out, d.Fields, 1, false)
		if d.Rollback != nil {
			out.WriteString("  rollback on {\n    ")
			out.WriteString(formatExpr(d.Rollback, 0))
			out.WriteString("\n  }\n")
		}
		out.WriteString("}\n")
	}
}

func formatFields(out *strings.Builder, fields []ast.Field, depth int, align bool) {
	if len(fields) == 0 {
		return
	}
	width := 0
	if align {
		for _, field := range fields {
			if len(field.Name) > width {
				width = len(field.Name)
			}
		}
	}
	indent := strings.Repeat("  ", depth)
	for _, field := range fields {
		out.WriteString(indent)
		out.WriteString(field.Name)
		if align {
			out.WriteString(strings.Repeat(" ", width-len(field.Name)))
		}
		out.WriteString(" = ")
		out.WriteString(formatExpr(field.Value, 0))
		if field.Every != nil {
			out.WriteString(" every ")
			out.WriteString(literalText(field.Every))
		}
		out.WriteByte('\n')
	}
}

func formatExpr(expr ast.Expr, parent int) string {
	if expr == nil {
		return ""
	}
	var value string
	prec := 6
	switch n := expr.(type) {
	case *ast.IdentExpr:
		value = n.Name
	case *ast.LiteralExpr:
		value = literalText(n)
	case *ast.ListExpr:
		parts := make([]string, len(n.Elements))
		for i, item := range n.Elements {
			parts[i] = formatExpr(item, 0)
		}
		value = "[" + strings.Join(parts, ", ") + "]"
	case *ast.CallExpr:
		parts := make([]string, len(n.Arguments))
		for i, arg := range n.Arguments {
			parts[i] = formatExpr(arg, 0)
		}
		value = formatExpr(n.Callee, 6) + "(" + strings.Join(parts, ", ") + ")"
		prec = 5
	case *ast.BinaryExpr:
		prec = expressionPrecedence(n.Operator)
		value = formatExpr(n.Left, prec) + " " + n.Operator.String() + " " + formatExpr(n.Right, prec+1)
	default:
		return ""
	}
	if prec < parent {
		return "(" + value + ")"
	}
	return value
}

func expressionPrecedence(op token.Kind) int {
	switch op {
	case token.Or:
		return 1
	case token.And:
		return 2
	case token.Equal, token.NotEqual:
		return 3
	case token.Less, token.Greater, token.LessEqual, token.GreaterEqual:
		return 4
	default:
		return 0
	}
}

func literalText(value *ast.LiteralExpr) string {
	if value == nil {
		return ""
	}
	if value.Kind == token.String {
		return strconv.Quote(value.Value)
	}
	if value.Text != "" {
		return value.Text
	}
	return value.Value
}

func quoted(out *strings.Builder, value string) { out.WriteString(strconv.Quote(value)) }
