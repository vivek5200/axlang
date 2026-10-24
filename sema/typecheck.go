// Package sema implements AxLang's static safety checks.
package sema

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/vivek5200/axlang/ast"
	"github.com/vivek5200/axlang/parser"
	"github.com/vivek5200/axlang/token"
)

// Check validates semantic invariants and returns all diagnostics found.
func Check(file *ast.File) []parser.Diagnostic {
	if file == nil {
		return []parser.Diagnostic{{Code: "AX000", Message: "cannot check an empty syntax tree", Span: token.Span{Filename: "<input>", Line: 1, Column: 1}}}
	}
	var diagnostics []parser.Diagnostic
	stages := map[string]bool{}
	for _, decl := range file.Decls {
		if d, ok := decl.(*ast.StageDecl); ok {
			stages[d.Name] = true
		}
	}
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.HealDecl:
			if !stages[d.OnFailure] {
				diagnostics = append(diagnostics, diag("AX002", fmt.Sprintf("heal references undeclared stage %q", d.OnFailure), d.Span(), "declare that stage before using it in the heal trigger"))
			}
			maxField, ok := findField(d.Fields, "max_attempts")
			if !ok || !validAttempts(maxField.Value) {
				span := d.Span()
				if ok {
					span = maxField.Span()
				}
				diagnostics = append(diagnostics, diag("AX001", "max_attempts must be an integer from 1 through 5", span, "keep autonomous repair loops bounded to at most five attempts"))
			}
			if d.Allow != nil {
				diagnostics = append(diagnostics, checkExpr(d.Allow)...)
			}
		case *ast.DeployDecl:
			if !stages[d.AfterStage] {
				diagnostics = append(diagnostics, diag("AX002", fmt.Sprintf("deploy references undeclared stage %q", d.AfterStage), d.Span(), "declare that stage before using it as a deploy dependency"))
			}
			steps, ok := findField(d.Fields, "steps")
			if !ok || !validSteps(steps.Value) {
				span := d.Span()
				if ok {
					span = steps.Span()
				}
				diagnostics = append(diagnostics, diag("AX004", "deploy steps must be strictly increasing percentages ending at 100%", span, "use a sequence such as [1%, 5%, 25%, 100%]"))
			}
			if d.Rollback != nil {
				diagnostics = append(diagnostics, checkExpr(d.Rollback)...)
			}
		case *ast.GuardrailDecl:
			for _, r := range d.Rules {
				diagnostics = append(diagnostics, checkExpr(r.Condition)...)
			}
		}
	}
	return diagnostics
}

func validAttempts(expr ast.Expr) bool {
	lit, ok := expr.(*ast.LiteralExpr)
	if !ok || lit.Kind != token.Number || strings.ContainsAny(lit.Value, ".eE") {
		return false
	}
	n, err := strconv.Atoi(lit.Value)
	return err == nil && n >= 1 && n <= 5
}

func validSteps(expr ast.Expr) bool {
	list, ok := expr.(*ast.ListExpr)
	if !ok || len(list.Elements) == 0 {
		return false
	}
	previous := 0.0
	for _, item := range list.Elements {
		lit, ok := item.(*ast.LiteralExpr)
		if !ok || lit.Kind != token.Percent {
			return false
		}
		v, err := strconv.ParseFloat(lit.Value, 64)
		if err != nil || v <= previous || v > 100 {
			return false
		}
		previous = v
	}
	return previous == 100
}

func checkExpr(expr ast.Expr) []parser.Diagnostic {
	var out []parser.Diagnostic
	visit := func(e *ast.BinaryExpr) {
		if e.Operator != token.Equal && e.Operator != token.NotEqual && e.Operator != token.Less && e.Operator != token.Greater && e.Operator != token.LessEqual && e.Operator != token.GreaterEqual {
			return
		}
		leftType, rightType := expressionType(e.Left), expressionType(e.Right)
		if leftType != typeUnknown && rightType != typeUnknown && leftType != rightType {
			out = append(out, diag("AX003", fmt.Sprintf("unit mismatch comparing %s with %s", leftType, rightType), e.Span(), "compare values with matching domain types: duration, percent, number, or string"))
		}
	}
	var walk func(ast.Expr)
	walk = func(e ast.Expr) {
		switch n := e.(type) {
		case *ast.BinaryExpr:
			visit(n)
			walk(n.Left)
			walk(n.Right)
		case *ast.CallExpr:
			walk(n.Callee)
			for _, arg := range n.Arguments {
				walk(arg)
			}
		case *ast.ListExpr:
			for _, item := range n.Elements {
				walk(item)
			}
		}
	}
	walk(expr)
	return out
}

type valueType uint8

const (
	typeUnknown valueType = iota
	typeDuration
	typePercent
	typeNumber
	typeString
	typeBoolean
)

func (t valueType) String() string {
	switch t {
	case typeDuration:
		return "Duration"
	case typePercent:
		return "Percent"
	case typeNumber:
		return "Number"
	case typeString:
		return "String"
	case typeBoolean:
		return "Boolean"
	default:
		return "Unknown"
	}
}

func expressionType(expr ast.Expr) valueType {
	switch n := expr.(type) {
	case *ast.LiteralExpr:
		switch n.Kind {
		case token.Duration:
			return typeDuration
		case token.Percent:
			return typePercent
		case token.Number:
			return typeNumber
		case token.String:
			return typeString
		}
	case *ast.IdentExpr:
		if n.Name == "true" || n.Name == "false" {
			return typeBoolean
		}
		if strings.HasPrefix(n.Name, "metrics.") {
			if strings.HasSuffix(n.Name, "_latency") {
				return typeDuration
			}
			if strings.HasSuffix(n.Name, "_rate") {
				return typePercent
			}
		}
	case *ast.CallExpr:
		if callee, ok := n.Callee.(*ast.IdentExpr); ok && callee.Name == "diff.modifies_tests" {
			return typeBoolean
		}
	}
	return typeUnknown
}

func findField(fields []ast.Field, name string) (ast.Field, bool) {
	for _, f := range fields {
		if f.Name == name {
			return f, true
		}
	}
	return ast.Field{}, false
}
func diag(code, message string, span token.Span, help string) parser.Diagnostic {
	return parser.Diagnostic{Code: code, Message: message, Span: span, Help: help}
}
