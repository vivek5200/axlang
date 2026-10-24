// Command axls runs the AxLang Language Server Protocol server over stdio.
package main

import (
	"fmt"
	"os"

	"github.com/vivek5200/axlang/lsp"
)

func main() {
	if err := lsp.NewServer().Serve(os.Stdin, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "axls: %v\n", err)
		os.Exit(1)
	}
}
