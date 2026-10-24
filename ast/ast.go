// Package ast defines the syntax tree shared by the AxLang compiler stages.
package ast

import "github.com/vivek5200/axlang/token"

// Node is implemented by every source-bearing syntax node.
type Node interface{ Span() token.Span }

// File is a complete AxLang source unit.
type File struct {
	Filename string
	Decls    []Decl
	Comments []Comment
}

// Comment retains a line comment's text and byte offset for source-preserving formatting.
type Comment struct {
	Text   string
	Offset int
}

// Decl is any top-level declaration.
type Decl interface {
	Node
	declarationNode()
}

type WorkspaceDecl struct {
	Name   string
	Fields []Field
	Source token.Span
}

func (*WorkspaceDecl) declarationNode()   {}
func (d *WorkspaceDecl) Span() token.Span { return d.Source }

type GuardrailDecl struct {
	Name   string
	Rules  []GuardrailRule
	Source token.Span
}

func (*GuardrailDecl) declarationNode()   {}
func (d *GuardrailDecl) Span() token.Span { return d.Source }

type GuardrailRule struct {
	Kind      GuardrailRuleKind
	Condition Expr
	Source    token.Span
}
type GuardrailRuleKind uint8

const (
	DenyRule GuardrailRuleKind = iota + 1
	RequireApprovalRule
)

type StageDecl struct {
	Name        string
	AfterStages []StageDependency
	Fields      []Field
	Source      token.Span
}

func (*StageDecl) declarationNode()   {}
func (d *StageDecl) Span() token.Span { return d.Source }

// StageDependency names a prerequisite stage and retains the source span of its reference.
type StageDependency struct {
	Name   string
	Source token.Span
}

type HealDecl struct {
	Name      string
	OnFailure string
	Fields    []Field
	Allow     Expr
	Source    token.Span
}

func (*HealDecl) declarationNode()   {}
func (d *HealDecl) Span() token.Span { return d.Source }

type DeployDecl struct {
	Name       string
	AfterStage string
	Fields     []Field
	Rollback   Expr
	Source     token.Span
}

func (*DeployDecl) declarationNode()   {}
func (d *DeployDecl) Span() token.Span { return d.Source }

// Field stores a generic declaration assignment. Values are literals, identifiers,
// lists, or calls so later compiler phases can validate domain-specific fields.
type Field struct {
	Name   string
	Value  Expr
	Every  *LiteralExpr
	Source token.Span
}

func (f Field) Span() token.Span { return f.Source }

// Expr is an expression in a condition or assignment value.
type Expr interface {
	Node
	expressionNode()
}

type BinaryExpr struct {
	Left     Expr
	Operator token.Kind
	Right    Expr
	Source   token.Span
}

func (*BinaryExpr) expressionNode()    {}
func (e *BinaryExpr) Span() token.Span { return e.Source }

type CallExpr struct {
	Callee    Expr
	Arguments []Expr
	Source    token.Span
}

func (*CallExpr) expressionNode()    {}
func (e *CallExpr) Span() token.Span { return e.Source }

type IdentExpr struct {
	Name   string
	Source token.Span
}

func (*IdentExpr) expressionNode()    {}
func (e *IdentExpr) Span() token.Span { return e.Source }

type LiteralExpr struct {
	Kind   token.Kind
	Value  string
	Text   string
	Source token.Span
}

func (*LiteralExpr) expressionNode()    {}
func (e *LiteralExpr) Span() token.Span { return e.Source }

type ListExpr struct {
	Elements []Expr
	Source   token.Span
}

func (*ListExpr) expressionNode()    {}
func (e *ListExpr) Span() token.Span { return e.Source }
