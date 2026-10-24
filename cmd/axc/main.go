// Command axc checks and compiles Astrix Control Language files.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/vivek5200/axlang/formatter"
	"github.com/vivek5200/axlang/internal/scaffold"
	"github.com/vivek5200/axlang/parser"
	"github.com/vivek5200/axlang/sema"
	"github.com/vivek5200/axlang/targets/ir"
	"github.com/vivek5200/axlang/targets/rego"
)

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

func run(args []string, stdout, stderr *os.File) int {
	if len(args) > 0 && args[0] == "init" {
		return runInit(args[1:], stdout, stderr)
	}
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
		fmt.Fprintln(stderr, "usage: axc <init|check|fmt|emit-rego|emit-ir> [arguments]")
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
		fmt.Fprintf(stderr, "axc: unknown command %q\nusage: axc <init|check|fmt|emit-rego|emit-ir> [arguments]\n", command)
		return 2
	}
}

func runInit(args []string, stdout, stderr *os.File) int {
	flags := flag.NewFlagSet("init", flag.ContinueOnError)
	flags.SetOutput(stderr)
	withGitHubActions := flags.Bool("with-github-actions", false, "also add .github/workflows/axlang.yml")
	force := flags.Bool("force", false, "replace generated files if they already exist")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() > 1 {
		fmt.Fprintln(stderr, "usage: axc init [--with-github-actions] [--force] [file.ax]")
		return 2
	}

	configPath := "Astrixfile.ax"
	if flags.NArg() == 1 {
		configPath = flags.Arg(0)
	}
	configPath = filepath.Clean(configPath)

	var workflowPath string
	if *withGitHubActions {
		workflowPath = filepath.Join(".github", "workflows", "axlang.yml")
		if configPath == filepath.Clean(workflowPath) {
			fmt.Fprintf(stderr, "axc init: config path conflicts with generated workflow path %s\n", workflowPath)
			return 2
		}
	}
	for _, path := range []string{configPath, workflowPath} {
		if path == "" {
			continue
		}
		if _, err := os.Lstat(path); err == nil && !*force {
			fmt.Fprintf(stderr, "axc init: %s already exists; pass --force to replace it\n", path)
			return 1
		} else if err != nil && !os.IsNotExist(err) {
			fmt.Fprintf(stderr, "axc init: inspect %s: %v\n", path, err)
			return 1
		}
	}

	root, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(stderr, "axc init: determine current directory: %v\n", err)
		return 1
	}
	projectName := filepath.Base(root)
	if projectName == "." || projectName == string(filepath.Separator) || projectName == "" {
		projectName = "my-project"
	}
	languages := detectLanguages(root)
	config := fmt.Sprintf("workspace %q {\n  languages = [%s]\n}\n\nstage \"verify\" {\n  scope = ast.blast_radius\n}\n",
		projectName, quotedList(languages))
	if *withGitHubActions {
		if err := os.MkdirAll(filepath.Dir(workflowPath), 0o755); err != nil {
			fmt.Fprintf(stderr, "axc init: create workflow directory: %v\n", err)
			return 1
		}
	}
	if err := writeScaffoldFile(configPath, []byte(config), *force); err != nil {
		fmt.Fprintf(stderr, "axc init: write %s: %v\n", configPath, err)
		return 1
	}
	fmt.Fprintf(stdout, "created %s\n", configPath)

	if *withGitHubActions {
		if err := writeScaffoldFile(workflowPath, []byte(scaffold.GitHubActionsWorkflow), *force); err != nil {
			fmt.Fprintf(stderr, "axc init: write %s: %v\n", workflowPath, err)
			return 1
		}
		fmt.Fprintf(stdout, "created %s\n", workflowPath)
	}
	return 0
}

func detectLanguages(root string) []string {
	checks := []struct {
		files []string
		name  string
	}{
		{files: []string{"go.mod"}, name: "go"},
		{files: []string{"package.json", "tsconfig.json"}, name: "typescript"},
		{files: []string{"pyproject.toml", "requirements.txt", "setup.py"}, name: "python"},
	}
	var languages []string
	for _, check := range checks {
		for _, name := range check.files {
			if _, err := os.Stat(filepath.Join(root, name)); err == nil {
				languages = append(languages, check.name)
				break
			}
		}
	}
	return languages
}

func quotedList(values []string) string {
	quoted := make([]string, len(values))
	for i, value := range values {
		quoted[i] = fmt.Sprintf("%q", value)
	}
	return strings.Join(quoted, ", ")
}

func writeScaffoldFile(path string, content []byte, force bool) error {
	if !force {
		file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if err != nil {
			return err
		}
		if _, err := file.Write(content); err != nil {
			_ = file.Close()
			_ = os.Remove(path)
			return err
		}
		return file.Close()
	}
	info, err := os.Stat(path)
	mode := os.FileMode(0o644)
	if err == nil {
		linkInfo, linkErr := os.Lstat(path)
		if linkErr != nil {
			return linkErr
		}
		if linkInfo.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing to replace symlink %s", path)
		}
		mode = info.Mode().Perm()
	} else if !os.IsNotExist(err) {
		return err
	}
	return os.WriteFile(path, content, mode)
}
