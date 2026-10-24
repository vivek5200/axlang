package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestDetectLanguagesForPolyglotProject(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"package.json", "tsconfig.json", "Cargo.toml", "pyproject.toml", "pom.xml", "CMakeLists.txt"} {
		if err := os.WriteFile(filepath.Join(root, name), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	got := detectLanguages(root)
	want := []string{"typescript", "javascript", "python", "rust", "java", "c/c++"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("detectLanguages()=%v, want %v", got, want)
	}
}
