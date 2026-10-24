// Package rego compiles AxLang policy conditions to OPA Rego v1 source.
package rego

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/vivek5200/axlang/ast"
	"github.com/vivek5200/axlang/token"
)

// Compile renders the policy-bearing parts of a validated AxLang file as Rego v1.
// Guardrail alternatives become separate rules with a shared rule head.
func Compile(file *ast.File) (string, error) {
	if file == nil {
		return "", fmt.Errorf("cannot compile a nil AxLang file")
	}
	var b strings.Builder
	b.WriteString("package astrix.generated\n\nimport rego.v1\n\ndefault deny := false\ndefault require_approval := false\ndefault allow := false\n\nax_diff_modifies_tests(expected) if {\n    input.diff.modifies_tests == expected\n}\n")
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.GuardrailDecl:
			for _, rule := range d.Rules {
				head := "deny"
				if rule.Kind == ast.RequireApprovalRule {
					head = "require_approval"
				}
				terms, err := expand(rule.Condition)
				if err != nil {
					return "", fmt.Errorf("guardrail %q: %w", d.Name, err)
				}
				for _, term := range terms {
					writeRule(&b, head, term)
				}
			}
		case *ast.HealDecl:
			if d.Allow != nil {
				terms, err := expand(d.Allow)
				if err != nil {
					return "", fmt.Errorf("heal %q: %w", d.Name, err)
				}
				for _, term := range terms {
					writeRule(&b, "allow", term)
				}
			}
		}
	}
	return b.String(), nil
}

func writeRule(b *strings.Builder, head string, atoms []string) {
	fmt.Fprintf(b, "\n%s if {\n", head)
	for _, atom := range atoms {
		for _, line := range strings.Split(atom, "\n") {
			fmt.Fprintf(b, "    %s\n", line)
		}
	}
	b.WriteString("}\n")
}

// expand converts a boolean condition into disjunctive normal form. Keeping each
// clause as a separate rule preserves the source's and/or semantics in Rego.
func expand(expr ast.Expr) ([][]string, error) {
	switch n := expr.(type) {
	case *ast.BinaryExpr:
		if n.Operator == token.Or {
			left, err := expand(n.Left)
			if err != nil {
				return nil, err
			}
			right, err := expand(n.Right)
			if err != nil {
				return nil, err
			}
			return append(left, right...), nil
		}
		if n.Operator == token.And {
			left, err := expand(n.Left)
			if err != nil {
				return nil, err
			}
			right, err := expand(n.Right)
			if err != nil {
				return nil, err
			}
			var combined [][]string
			for _, a := range left {
				for _, c := range right {
					combined = append(combined, append(append([]string{}, a...), c...))
				}
			}
			return combined, nil
		}
		left, err := render(n.Left)
		if err != nil {
			return nil, err
		}
		right, err := render(n.Right)
		if err != nil {
			return nil, err
		}
		return [][]string{{left + " " + n.Operator.String() + " " + right}}, nil
	case *ast.CallExpr:
		callee, ok := n.Callee.(*ast.IdentExpr)
		if !ok {
			return nil, fmt.Errorf("unsupported call target")
		}
		if callee.Name == "diff.touches" {
			if len(n.Arguments) == 0 {
				return nil, fmt.Errorf("diff.touches requires at least one glob pattern")
			}
			terms := make([][]string, 0, len(n.Arguments))
			for _, arg := range n.Arguments {
				lit, ok := arg.(*ast.LiteralExpr)
				if !ok || lit.Kind != token.String {
					return nil, fmt.Errorf("diff.touches arguments must be strings")
				}
				terms = append(terms, []string{"some f in input.modified_files\nglob.match(" + quote(lit.Value) + ", [\"/\"], f)"})
			}
			return terms, nil
		}
		call, err := render(n)
		if err != nil {
			return nil, err
		}
		return [][]string{{call}}, nil
	default:
		atom, err := render(expr)
		if err != nil {
			return nil, err
		}
		return [][]string{{atom}}, nil
	}
}

func render(expr ast.Expr) (string, error) {
	switch n := expr.(type) {
	case *ast.IdentExpr:
		if n.Name == "true" || n.Name == "false" {
			return n.Name, nil
		}
		return "input." + n.Name, nil
	case *ast.LiteralExpr:
		switch n.Kind {
		case token.String:
			return quote(n.Value), nil
		case token.Number:
			return n.Value, nil
		case token.Percent:
			v, err := strconv.ParseFloat(n.Value, 64)
			if err != nil {
				return "", err
			}
			return strconv.FormatFloat(v/100, 'f', -1, 64), nil
		case token.Duration:
			d, err := time.ParseDuration(n.Text)
			if err != nil {
				return "", fmt.Errorf("invalid duration %q", n.Text)
			}
			return strconv.FormatFloat(d.Seconds(), 'f', -1, 64), nil
		default:
			return "", fmt.Errorf("unsupported literal kind %s", n.Kind)
		}
	case *ast.BinaryExpr:
		left, err := render(n.Left)
		if err != nil {
			return "", err
		}
		right, err := render(n.Right)
		if err != nil {
			return "", err
		}
		return "(" + left + " " + n.Operator.String() + " " + right + ")", nil
	case *ast.CallExpr:
		callee, ok := n.Callee.(*ast.IdentExpr)
		if !ok {
			return "", fmt.Errorf("unsupported call target")
		}
		if callee.Name == "diff.modifies_tests" {
			if len(n.Arguments) != 1 {
				return "", fmt.Errorf("diff.modifies_tests requires one expected value")
			}
			arg, err := render(n.Arguments[0])
			if err != nil {
				return "", err
			}
			return "ax_diff_modifies_tests(" + arg + ")", nil
		}
		args := make([]string, len(n.Arguments))
		for i, arg := range n.Arguments {
			value, err := render(arg)
			if err != nil {
				return "", err
			}
			args[i] = value
		}
		return "input." + callee.Name + "(" + strings.Join(args, ", ") + ")", nil
	case *ast.ListExpr:
		items := make([]string, len(n.Elements))
		for i, item := range n.Elements {
			value, err := render(item)
			if err != nil {
				return "", err
			}
			items[i] = value
		}
		return "[" + strings.Join(items, ", ") + "]", nil
	default:
		return "", fmt.Errorf("unsupported expression %T", expr)
	}
}

func quote(s string) string { return strconv.Quote(s) }
