package koine_test

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestArch_StdlibOnly is the zero-dependency LAW, not a habit: every non-test
// file in the shipped packages imports the standard library or this module,
// nothing else. The SDK embeds anywhere Go compiles — TinyGo included — and
// the dependency answer to any review is one sentence.
func TestArch_StdlibOnly(t *testing.T) {
	const modulePrefix = "github.com/sol-duara-inc/koine-go/"
	for _, dir := range []string{".", "selector"} {
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range entries {
			name := e.Name()
			if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
				continue
			}
			path := filepath.Join(dir, name)
			f, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
			if err != nil {
				t.Fatal(err)
			}
			for _, imp := range f.Imports {
				p := strings.Trim(imp.Path.Value, `"`)
				if strings.HasPrefix(p, modulePrefix) {
					continue // module-internal
				}
				first, _, _ := strings.Cut(p, "/")
				if strings.Contains(first, ".") {
					t.Errorf("%s imports %q — the SDK core is stdlib-only by law", path, p)
				}
			}
		}
	}
}
