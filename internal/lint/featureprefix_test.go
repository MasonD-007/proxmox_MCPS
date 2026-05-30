package lint

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func checkFile(t testing.TB, path, feat string) error {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		return err
	}
	return checkAST(f, path, feat)
}

func checkSource(t testing.TB, src, feat string) error {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "", src, 0)
	if err != nil {
		return err
	}
	return checkAST(f, "<source>", feat)
}

func checkAST(f *ast.File, name, feat string) error {
	lower := strings.ToLower(feat)
	var failures []string

	for _, decl := range f.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			if d.Name.Name == "init" {
				continue
			}
			if !strings.Contains(strings.ToLower(d.Name.Name), lower) {
				failures = append(failures, fmt.Sprintf("func %s", d.Name.Name))
			}
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				switch s := spec.(type) {
				case *ast.ValueSpec:
					for _, name := range s.Names {
						if !strings.Contains(strings.ToLower(name.Name), lower) {
							failures = append(failures, fmt.Sprintf("%s %s", strings.ToLower(d.Tok.String()), name.Name))
						}
					}
				case *ast.TypeSpec:
					if !strings.Contains(strings.ToLower(s.Name.Name), lower) {
						failures = append(failures, fmt.Sprintf("type %s", s.Name.Name))
					}
				}
			}
		}
	}

	if len(failures) > 0 {
		return fmt.Errorf("%s: identifiers must contain %q (case-insensitive): %s",
			name, feat, strings.Join(failures, ", "))
	}
	return nil
}

func TestFeaturePrefixGuard(t *testing.T) {
	tests := []struct {
		name     string
		src      string
		token    string
		wantFail bool
	}{
		{
			name: "prefixed func and const pass, init exempted",
			src: `package snapshot
func handleSnapshot() {}
const snapshotDefaultTimeout = 30
func init() {}`,
			token:    "snapshot",
			wantFail: false,
		},
		{
			name: "bare identifier fails",
			src: `package snapshot
var defaultTimeout = 30`,
			token:    "snapshot",
			wantFail: true,
		},
		{
			name: "all identifiers prefixed pass",
			src: `package thing
func thingHandler() {}
var thingValue = 1
const thingDefault = 42
type thingConfig struct{}
func init() {}`,
			token:    "thing",
			wantFail: false,
		},
		{
			name: "case insensitive match",
			src: `package snapshot
func HandleSnapshot() {}`,
			token:    "snapshot",
			wantFail: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkSource(t, tt.src, tt.token)
			if tt.wantFail && err == nil {
				t.Error("expected failure but got none")
			}
			if !tt.wantFail && err != nil {
				t.Errorf("unexpected failure: %v", err)
			}
		})
	}
}

func TestFeaturePrefixesInDirectory(t *testing.T) {
	entries, err := os.ReadDir("../features")
	if err != nil {
		t.Fatal(err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".go") {
			continue
		}
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		if name == "doc.go" {
			continue
		}

		token := strings.TrimSuffix(name, ".go")
		path := filepath.Join("../features", name)

		t.Run(name, func(t *testing.T) {
			if err := checkFile(t, path, token); err != nil {
				t.Error(err)
			}
		})
	}
}
