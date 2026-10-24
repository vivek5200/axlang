// Package token defines the lexical vocabulary and source locations used by AxLang.
package token

import "fmt"

// Kind identifies a lexical token.
type Kind uint8

const (
	Illegal Kind = iota
	EOF
	Ident
	String
	Number
	Duration
	Percent

	Workspace
	Guardrail
	Deny
	When
	RequireApproval
	Stage
	Scope
	QuarantineThreshold
	Network
	Heal
	On
	Failure
	Model
	MaxAttempts
	Allow
	Deploy
	After
	Steps
	Every
	Rollback

	LBrace
	RBrace
	LParen
	RParen
	LBracket
	RBracket
	Comma
	Dot
	Assign
	Equal
	NotEqual
	Less
	Greater
	LessEqual
	GreaterEqual
	Minus
	And
	Or
)

var kindNames = map[Kind]string{
	Illegal: "ILLEGAL", EOF: "EOF", Ident: "IDENT", String: "STRING",
	Number: "NUMBER", Duration: "DURATION", Percent: "PERCENT",
	Workspace: "workspace", Guardrail: "guardrail", Deny: "deny", When: "when",
	RequireApproval: "require_approval", Stage: "stage", Scope: "scope",
	QuarantineThreshold: "quarantine_threshold", Network: "network", Heal: "heal",
	On: "on", Failure: "failure", Model: "model", MaxAttempts: "max_attempts",
	Allow: "allow", Deploy: "deploy", After: "after", Steps: "steps", Every: "every",
	Rollback: "rollback", LBrace: "{", RBrace: "}", LParen: "(", RParen: ")",
	LBracket: "[", RBracket: "]", Comma: ",", Dot: ".", Assign: "=", Equal: "==",
	NotEqual: "!=", Less: "<", Greater: ">", LessEqual: "<=", GreaterEqual: ">=", Minus: "-",
	And: "and", Or: "or",
}

// String returns a readable spelling for diagnostics and debugging.
func (k Kind) String() string {
	if s, ok := kindNames[k]; ok {
		return s
	}
	return fmt.Sprintf("token(%d)", k)
}

// Span identifies a half-open range in a source file. Line and Column are one-based;
// Offset and Length are byte-based so they can index the original UTF-8 source.
type Span struct {
	Filename string
	Line     int
	Column   int
	Offset   int
	Length   int
}

// End returns the byte offset immediately after the span.
func (s Span) End() int { return s.Offset + s.Length }

// Token is one lexical item and its original source spelling/value.
type Token struct {
	Kind  Kind
	Text  string
	Value string
	Span  Span
}

// Keywords maps reserved words to their token kinds. Other words remain Ident tokens.
var Keywords = map[string]Kind{
	"workspace": Workspace, "guardrail": Guardrail, "deny": Deny, "when": When,
	"require_approval": RequireApproval, "stage": Stage, "scope": Scope,
	"quarantine_threshold": QuarantineThreshold, "network": Network, "heal": Heal,
	"on": On, "failure": Failure, "model": Model, "max_attempts": MaxAttempts,
	"allow": Allow, "deploy": Deploy, "after": After, "steps": Steps,
	"every": Every, "rollback": Rollback, "and": And, "or": Or,
}
