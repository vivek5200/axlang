// Package parser implements recursive descent declarations and Pratt expressions.
package parser

import (
	"fmt"
	"strings"

	"github.com/vivek5200/axlang/ast"
	"github.com/vivek5200/axlang/lexer"
	"github.com/vivek5200/axlang/token"
)

// Parser holds the token stream and source text for one parse operation.
type Parser struct {
	filename  string
	source    string
	tokens    []token.Token
	pos       int
	diags     []Diagnostic
	exprDepth int
}

// New tokenizes source and creates a parser. Lexical errors are reported by Parse.
func New(filename, source string) *Parser { return &Parser{filename: filename, source: source} }

// Parse parses all top-level declarations and returns recoverable diagnostics.
func (p *Parser) Parse() (*ast.File, []Diagnostic) {
	p.tokens = nil
	p.pos = 0
	p.diags = nil
	p.exprDepth = 0
	tokens, err := lexer.New(p.filename, p.source).Scan()
	if err != nil {
		lexErr := err.(lexer.Error)
		p.diags = append(p.diags, Diagnostic{Code: "AX100", Message: lexErr.Message, Span: lexErr.Span})
		return &ast.File{Filename: p.filename}, p.diags
	}
	p.tokens = tokens
	file := &ast.File{Filename: p.filename, Comments: collectComments(p.source)}
	for !p.at(token.EOF) {
		start := p.pos
		decl, err := p.parseDecl()
		if err != nil {
			pe := err.(parseError)
			p.diags = append(p.diags, pe.diagnostic)
			p.synchronize(start)
			continue
		}
		file.Decls = append(file.Decls, decl)
	}
	return file, p.diags
}

func collectComments(source string) []ast.Comment {
	var comments []ast.Comment
	inString := false
	for offset := 0; offset < len(source); {
		if inString {
			switch source[offset] {
			case '\\':
				offset += 2
			case '"':
				inString = false
				offset++
			default:
				offset++
			}
			continue
		}
		if source[offset] == '"' {
			inString = true
			offset++
			continue
		}
		if source[offset] == '/' && offset+1 < len(source) && source[offset+1] == '/' {
			start := offset
			offset += 2
			for offset < len(source) && source[offset] != '\n' && source[offset] != '\r' {
				offset++
			}
			comments = append(comments, ast.Comment{Text: source[start:offset], Offset: start})
			continue
		}
		offset++
	}
	return comments
}

func (p *Parser) parseDecl() (ast.Decl, error) {
	switch p.current().Kind {
	case token.Workspace:
		return p.parseWorkspace()
	case token.Guardrail:
		return p.parseGuardrail()
	case token.Stage:
		return p.parseStage()
	case token.Heal:
		return p.parseHeal()
	case token.Deploy:
		return p.parseDeploy()
	default:
		return nil, p.fail(p.current(), "expected workspace, guardrail, stage, heal, or deploy declaration", "start a declaration with one of the supported block keywords")
	}
}

func (p *Parser) parseWorkspace() (ast.Decl, error) {
	start := p.take()
	name, err := p.expectString("workspace name")
	if err != nil {
		return nil, err
	}
	if _, err = p.expect(token.LBrace, "'{' after workspace name"); err != nil {
		return nil, err
	}
	fields, end, err := p.parseFields()
	if err != nil {
		return nil, err
	}
	return &ast.WorkspaceDecl{Name: name.Value, Fields: fields, Source: join(start.Span, end.Span)}, nil
}

func (p *Parser) parseStage() (ast.Decl, error) {
	start := p.take()
	name, err := p.expectString("stage name")
	if err != nil {
		return nil, err
	}
	var dependencies []ast.StageDependency
	if p.at(token.After) {
		p.take()
		dependencies, err = p.parseStageReferences("after")
		if err != nil {
			return nil, err
		}
	}
	if _, err = p.expect(token.LBrace, "'{' after stage name"); err != nil {
		return nil, err
	}
	fields, end, err := p.parseFields()
	if err != nil {
		return nil, err
	}
	return &ast.StageDecl{Name: name.Value, AfterStages: dependencies, Fields: fields, Source: join(start.Span, end.Span)}, nil
}

func (p *Parser) parseGuardrail() (ast.Decl, error) {
	start := p.take()
	name, err := p.expectString("guardrail name")
	if err != nil {
		return nil, err
	}
	if _, err = p.expect(token.LBrace, "'{' after guardrail name"); err != nil {
		return nil, err
	}
	var rules []ast.GuardrailRule
	for !p.at(token.RBrace) && !p.at(token.EOF) {
		ruleStart := p.current()
		kind := ast.DenyRule
		if p.at(token.Deny) {
			p.take()
		} else if p.at(token.RequireApproval) {
			p.take()
			kind = ast.RequireApprovalRule
		} else {
			return nil, p.fail(p.current(), "expected 'deny' or 'require_approval' rule", "add a guardrail rule such as 'deny when { condition }'")
		}
		if _, err = p.expect(token.When, "'when' after guardrail rule"); err != nil {
			return nil, err
		}
		if _, err = p.expect(token.LBrace, "'{' before condition"); err != nil {
			return nil, err
		}
		condition, err := p.parseExpr(precLowest)
		if err != nil {
			return nil, err
		}
		end, err := p.expect(token.RBrace, "'}' after condition")
		if err != nil {
			return nil, err
		}
		rules = append(rules, ast.GuardrailRule{Kind: kind, Condition: condition, Source: join(ruleStart.Span, end.Span)})
	}
	end, err := p.expect(token.RBrace, "'}' after guardrail")
	if err != nil {
		return nil, err
	}
	return &ast.GuardrailDecl{Name: name.Value, Rules: rules, Source: join(start.Span, end.Span)}, nil
}

func (p *Parser) parseHeal() (ast.Decl, error) {
	start := p.take()
	name, err := p.expectString("heal name")
	if err != nil {
		return nil, err
	}
	if _, err = p.expect(token.On, "'on' after heal name"); err != nil {
		return nil, err
	}
	if _, err = p.expect(token.Failure, "'failure' after 'on'"); err != nil {
		return nil, err
	}
	stage, _, err := p.parseStageReference("failure")
	if err != nil {
		return nil, err
	}
	if _, err = p.expect(token.LBrace, "'{' after heal trigger"); err != nil {
		return nil, err
	}
	var fields []ast.Field
	var allow ast.Expr
	for !p.at(token.RBrace) && !p.at(token.EOF) {
		if p.at(token.Allow) {
			fieldStart := p.take()
			if _, err = p.expect(token.When, "'when' after allow"); err != nil {
				return nil, err
			}
			if _, err = p.expect(token.LBrace, "'{' before allow condition"); err != nil {
				return nil, err
			}
			allow, err = p.parseExpr(precLowest)
			if err != nil {
				return nil, err
			}
			if _, err = p.expect(token.RBrace, "'}' after allow condition"); err != nil {
				return nil, err
			}
			_ = fieldStart
			continue
		}
		field, fieldErr := p.parseField()
		if fieldErr != nil {
			return nil, fieldErr
		}
		fields = append(fields, field)
	}
	end, err := p.expect(token.RBrace, "'}' after heal")
	if err != nil {
		return nil, err
	}
	return &ast.HealDecl{Name: name.Value, OnFailure: stage, Fields: fields, Allow: allow, Source: join(start.Span, end.Span)}, nil
}

func (p *Parser) parseDeploy() (ast.Decl, error) {
	start := p.take()
	name, err := p.expectString("deploy name")
	if err != nil {
		return nil, err
	}
	if _, err = p.expect(token.After, "'after' after deploy name"); err != nil {
		return nil, err
	}
	stages, err := p.parseStageReferences("after")
	if err != nil {
		return nil, err
	}
	if len(stages) != 1 {
		return nil, p.fail(p.current(), "deploy requires exactly one stage dependency", "use after(stage.verify)")
	}
	if _, err = p.expect(token.LBrace, "'{' after deploy dependency"); err != nil {
		return nil, err
	}
	var fields []ast.Field
	var rollback ast.Expr
	for !p.at(token.RBrace) && !p.at(token.EOF) {
		if p.at(token.Rollback) {
			p.take()
			if _, err = p.expect(token.On, "'on' after rollback"); err != nil {
				return nil, err
			}
			if _, err = p.expect(token.LBrace, "'{' before rollback condition"); err != nil {
				return nil, err
			}
			rollback, err = p.parseExpr(precLowest)
			if err != nil {
				return nil, err
			}
			if _, err = p.expect(token.RBrace, "'}' after rollback condition"); err != nil {
				return nil, err
			}
			continue
		}
		field, fieldErr := p.parseField()
		if fieldErr != nil {
			return nil, fieldErr
		}
		fields = append(fields, field)
	}
	end, err := p.expect(token.RBrace, "'}' after deploy")
	if err != nil {
		return nil, err
	}
	return &ast.DeployDecl{Name: name.Value, AfterStage: stages[0].Name, Fields: fields, Rollback: rollback, Source: join(start.Span, end.Span)}, nil
}

func (p *Parser) parseStageReferences(context string) ([]ast.StageDependency, error) {
	if _, err := p.expect(token.LParen, "'(' before stage reference"); err != nil {
		return nil, err
	}
	var dependencies []ast.StageDependency
	for {
		ref := p.current()
		if ref.Kind != token.Ident {
			return nil, p.fail(ref, fmt.Sprintf("expected stage.<name> in %s reference", context), "reference a declared stage, for example stage.verify")
		}
		p.take()
		if !strings.HasPrefix(ref.Text, "stage.") || len(ref.Text) == len("stage.") {
			return nil, p.fail(ref, fmt.Sprintf("expected stage.<name> in %s reference", context), "reference a declared stage, for example stage.verify")
		}
		dependencies = append(dependencies, ast.StageDependency{Name: strings.TrimPrefix(ref.Text, "stage."), Source: ref.Span})
		if !p.at(token.Comma) {
			break
		}
		p.take()
	}
	if _, err := p.expect(token.RParen, "')' after stage reference"); err != nil {
		return nil, err
	}
	return dependencies, nil
}

func (p *Parser) parseStageReference(context string) (string, token.Span, error) {
	open, err := p.expect(token.LParen, "'(' before stage reference")
	if err != nil {
		return "", token.Span{}, err
	}
	ref := p.current()
	if ref.Kind != token.Ident {
		return "", token.Span{}, p.fail(ref, fmt.Sprintf("expected stage.<name> in %s reference", context), "reference a declared stage, for example stage.verify")
	}
	p.take()
	if !strings.HasPrefix(ref.Text, "stage.") || len(ref.Text) == len("stage.") {
		return "", token.Span{}, p.fail(ref, fmt.Sprintf("expected stage.<name> in %s reference", context), "reference a declared stage, for example stage.verify")
	}
	close, err := p.expect(token.RParen, "')' after stage reference")
	if err != nil {
		return "", token.Span{}, err
	}
	_ = open
	return strings.TrimPrefix(ref.Text, "stage."), join(ref.Span, close.Span), nil
}

func (p *Parser) parseFields() ([]ast.Field, token.Token, error) {
	var fields []ast.Field
	for !p.at(token.RBrace) && !p.at(token.EOF) {
		field, err := p.parseField()
		if err != nil {
			return nil, token.Token{}, err
		}
		fields = append(fields, field)
	}
	end, err := p.expect(token.RBrace, "'}' after block")
	return fields, end, err
}

func (p *Parser) parseField() (ast.Field, error) {
	name := p.current()
	if !isFieldName(name.Kind) {
		return ast.Field{}, p.fail(name, "expected a field assignment", "write a field as name = value")
	}
	p.take()
	if _, err := p.expect(token.Assign, "'=' after field name"); err != nil {
		return ast.Field{}, err
	}
	value, err := p.parseExpr(precLowest)
	if err != nil {
		return ast.Field{}, err
	}
	field := ast.Field{Name: name.Text, Value: value, Source: join(name.Span, value.Span())}
	if p.at(token.Every) {
		p.take()
		interval := p.current()
		if interval.Kind != token.Duration {
			return ast.Field{}, p.fail(interval, "expected a duration after 'every'", "use a duration such as 30s or 2m")
		}
		p.take()
		field.Every = &ast.LiteralExpr{Kind: interval.Kind, Value: interval.Value, Text: interval.Text, Source: interval.Span}
		field.Source = join(field.Source, interval.Span)
	}
	return field, nil
}

const (
	precLowest = iota
	precOr
	precAnd
	precEquals
	precComparison
	precCall
)

func precedence(k token.Kind) int {
	switch k {
	case token.Or:
		return precOr
	case token.And:
		return precAnd
	case token.Equal, token.NotEqual:
		return precEquals
	case token.Less, token.Greater, token.LessEqual, token.GreaterEqual:
		return precComparison
	case token.LParen:
		return precCall
	default:
		return precLowest
	}
}

func (p *Parser) parseExpr(minPrec int) (ast.Expr, error) {
	if p.exprDepth >= 256 {
		return nil, p.fail(p.current(), "expression nesting exceeds the parser safety limit", "simplify deeply nested expressions")
	}
	p.exprDepth++
	defer func() { p.exprDepth-- }()
	var left ast.Expr
	t := p.current()
	switch t.Kind {
	case token.Ident:
		p.take()
		left = &ast.IdentExpr{Name: t.Text, Source: t.Span}
	case token.String, token.Number, token.Duration, token.Percent:
		p.take()
		left = &ast.LiteralExpr{Kind: t.Kind, Value: t.Value, Text: t.Text, Source: t.Span}
	case token.Minus:
		p.take()
		number := p.current()
		if number.Kind != token.Number {
			return nil, p.fail(number, "expected a number after '-'", "negative values must be numeric literals")
		}
		p.take()
		left = &ast.LiteralExpr{Kind: token.Number, Value: "-" + number.Value, Text: "-" + number.Text, Source: join(t.Span, number.Span)}
	case token.LParen:
		p.take()
		var err error
		left, err = p.parseExpr(precLowest)
		if err != nil {
			return nil, err
		}
		if _, err = p.expect(token.RParen, "')' after expression"); err != nil {
			return nil, err
		}
	case token.LBracket:
		p.take()
		items := []ast.Expr{}
		if !p.at(token.RBracket) {
			for {
				item, err := p.parseExpr(precLowest)
				if err != nil {
					return nil, err
				}
				items = append(items, item)
				if !p.at(token.Comma) {
					break
				}
				p.take()
				if p.at(token.RBracket) {
					break
				}
			}
		}
		end, err := p.expect(token.RBracket, "']' after list")
		if err != nil {
			return nil, err
		}
		left = &ast.ListExpr{Elements: items, Source: join(t.Span, end.Span)}
	default:
		return nil, p.fail(t, "expected expression", "use an identifier, literal, call, list, or parenthesized expression")
	}
	operators := 0
	for precedence(p.current().Kind) > minPrec {
		operators++
		if operators > 256 {
			return nil, p.fail(p.current(), "expression exceeds the parser safety limit", "split long boolean or comparison chains into smaller rules")
		}
		op := p.current()
		prec := precedence(op.Kind)
		if op.Kind == token.LParen {
			p.take()
			args := []ast.Expr{}
			if !p.at(token.RParen) {
				for {
					arg, err := p.parseExpr(precLowest)
					if err != nil {
						return nil, err
					}
					args = append(args, arg)
					if !p.at(token.Comma) {
						break
					}
					p.take()
				}
			}
			end, err := p.expect(token.RParen, "')' after arguments")
			if err != nil {
				return nil, err
			}
			left = &ast.CallExpr{Callee: left, Arguments: args, Source: join(left.Span(), end.Span)}
			continue
		}
		p.take()
		right, err := p.parseExpr(prec)
		if err != nil {
			return nil, err
		}
		left = &ast.BinaryExpr{Left: left, Operator: op.Kind, Right: right, Source: join(left.Span(), right.Span())}
	}
	return left, nil
}

func isFieldName(k token.Kind) bool {
	switch k {
	case token.Ident, token.Scope, token.QuarantineThreshold, token.Network, token.Model, token.MaxAttempts, token.Steps:
		return true
	default:
		return false
	}
}

func (p *Parser) expectString(description string) (token.Token, error) {
	t := p.current()
	if t.Kind != token.String {
		return token.Token{}, p.fail(t, "expected "+description, "put the name in double quotes")
	}
	p.take()
	return t, nil
}
func (p *Parser) expect(kind token.Kind, description string) (token.Token, error) {
	t := p.current()
	if t.Kind != kind {
		return token.Token{}, p.fail(t, "expected "+description, "")
	}
	p.take()
	return t, nil
}
func (p *Parser) fail(t token.Token, message, help string) error {
	return parseError{Diagnostic{Code: "AX101", Message: message, Span: t.Span, Help: help}}
}
func (p *Parser) current() token.Token {
	if p.pos >= len(p.tokens) {
		return token.Token{Kind: token.EOF}
	}
	return p.tokens[p.pos]
}
func (p *Parser) at(k token.Kind) bool { return p.current().Kind == k }
func (p *Parser) take() token.Token {
	t := p.current()
	if p.pos < len(p.tokens) {
		p.pos++
	}
	return t
}
func (p *Parser) synchronize(start int) {
	if p.pos <= start {
		p.pos = start + 1
	}
	for !p.at(token.EOF) {
		switch p.current().Kind {
		case token.Workspace, token.Guardrail, token.Stage, token.Heal, token.Deploy:
			return
		}
		p.take()
	}
}
func join(a, b token.Span) token.Span {
	return token.Span{Filename: a.Filename, Line: a.Line, Column: a.Column, Offset: a.Offset, Length: b.End() - a.Offset}
}
