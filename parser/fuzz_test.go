package parser_test

import (
	"strings"
	"testing"

	"github.com/vivek5200/axlang/formatter"
	"github.com/vivek5200/axlang/parser"
	"github.com/vivek5200/axlang/sema"
)

func FuzzParserAndSema(f *testing.F) {
	f.Add([]byte(`guardrail "g" { deny when { actor.type == "bot" and diff.touches("auth/**") } }`))
	f.Add([]byte(`heal "h" on failure(stage.verify) { max_attempts = -1 }`))
	f.Add([]byte("\xff\xfe\x00"))
	f.Add([]byte("guardrail \"deep\" { deny when { " + strings.Repeat("(", 300) + `true` + strings.Repeat(")", 300) + ` } }`))
	f.Add([]byte(`deploy "truncated" after(stage.`))
	f.Fuzz(func(t *testing.T, input []byte) {
		// Keep fuzz cases bounded while retaining arbitrary byte sequences,
		// including malformed UTF-8 and truncated tokens.
		if len(input) > 1<<20 {
			t.Skip()
		}
		source := string(input)
		file, diagnostics := parser.New("fuzz.ax", source).Parse()
		if file == nil {
			t.Fatal("parser returned a nil file instead of a diagnostic-bearing result")
		}
		semanticDiagnostics := sema.Check(file)
		_ = formatter.Format(file)
		for _, diagnostic := range append(diagnostics, semanticDiagnostics...) {
			if diagnostic.Code == "" || diagnostic.Message == "" || diagnostic.Span.Line < 1 || diagnostic.Span.Column < 1 {
				t.Fatalf("malformed parser diagnostic: %+v", diagnostic)
			}
			_ = diagnostic.Format(source)
		}
	})
}
