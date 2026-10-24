// Command axc checks and compiles Astrix Control Language files.
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/vivek5200/axlang/formatter"
	"github.com/vivek5200/axlang/parser"
	"github.com/vivek5200/axlang/sema"
	"github.com/vivek5200/axlang/targets/ir"
	"github.com/vivek5200/axlang/targets/rego"
)

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

func run(args []string, stdout, stderr *os.File) int {
	write := false
	if len(args) > 0 && (args[0] == "fmt" || args[0] == "format") {
		if len(args) == 3 && (args[1] == "--write" || args[1] == "-w") {
			write = true
			args = []string{args[0], args[2]}
		}
		if len(args) != 2 {
			fmt.Fprintln(stderr, "usage: axc fmt [--write|-w] <file.ax>")
			return 2
		}
	} else if len(args) != 2 {
		fmt.Fprintln(stderr, "usage: axc <check|fmt|emit-rego|emit-ir> <file.ax>")
		return 2
	}
	command, path := args[0], args[1]
	source, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(stderr, "axc: %v\n", err)
		return 1
	}
	file, parseDiags := parser.New(path, string(source)).Parse()
	if len(parseDiags) > 0 {
		for _, d := range parseDiags {
			fmt.Fprintln(stderr, d.Format(string(source)))
		}
		return 1
	}
	if command == "fmt" || command == "format" {
		formatted := formatter.Format(file)
		if write {
			mode := os.FileMode(0o644)
			if info, statErr := os.Stat(path); statErr == nil {
				mode = info.Mode().Perm()
			}
			if err := os.WriteFile(path, []byte(formatted), mode); err != nil {
				fmt.Fprintf(stderr, "axc: %v\n", err)
				return 1
			}
		} else {
			fmt.Fprint(stdout, formatted)
		}
		return 0
	}
	semanticDiags := sema.Check(file)
	if len(semanticDiags) > 0 {
		for _, d := range semanticDiags {
			fmt.Fprintln(stderr, d.Format(string(source)))
		}
		return 1
	}
	switch command {
	case "check":
		fmt.Fprintf(stdout, "%s: valid\n", path)
		return 0
	case "emit-rego":
		out, err := rego.Compile(file)
		if err != nil {
			fmt.Fprintf(stderr, "axc: %v\n", err)
			return 1
		}
		fmt.Fprint(stdout, out)
		return 0
	case "emit-ir":
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(ir.FromAST(file)); err != nil {
			fmt.Fprintf(stderr, "axc: %v\n", err)
			return 1
		}
		return 0
	default:
		fmt.Fprintf(stderr, "axc: unknown command %q\nusage: axc <check|fmt|emit-rego|emit-ir> <file.ax>\n", command)
		return 2
	}
}
