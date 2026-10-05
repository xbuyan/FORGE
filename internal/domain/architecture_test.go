package domain_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Architecture tests. They scan production (non _test.go) Go files in the
// module.

func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}

func productionGoFiles(t *testing.T, root string) []string {
	t.Helper()
	var files []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "vendor", "testdata":
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", root, err)
	}
	return files
}

// referencesIssuer reports whether src refers to IssueTrustedActor as a
// selector, for example domain.IssueTrustedActor.
func referencesIssuer(filename string, src []byte) (bool, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filename, src, 0)
	if err != nil {
		return false, err
	}
	found := false
	ast.Inspect(f, func(n ast.Node) bool {
		if sel, ok := n.(*ast.SelectorExpr); ok && sel.Sel.Name == "IssueTrustedActor" {
			found = true
		}
		return true
	})
	return found, nil
}

func TestIssuerDetectorWorks(t *testing.T) {
	hit := []byte("package x\nimport \"d\"\nfunc f() { _, _ = d.IssueTrustedActor(a) }\n")
	miss := []byte("package x\nimport \"d\"\nfunc f() { d.Other() }\n")
	if got, err := referencesIssuer("hit.go", hit); err != nil || !got {
		t.Errorf("detector missed a reference: %v, %v", got, err)
	}
	if got, err := referencesIssuer("miss.go", miss); err != nil || got {
		t.Errorf("detector flagged a non-reference: %v, %v", got, err)
	}
}

// Only the domain package itself and the future authentication subsystem may
// reference the trusted-actor issuer in production code. Test code may.
func TestTrustedActorIssuerBoundary(t *testing.T) {
	root := moduleRoot(t)
	for _, path := range productionGoFiles(t, root) {
		rel, err := filepath.Rel(root, path)
		if err != nil {
			t.Fatal(err)
		}
		dir := filepath.ToSlash(filepath.Dir(rel))
		if dir == "internal/domain" || dir == "internal/authn" || strings.HasPrefix(dir, "internal/authn/") {
			continue
		}
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		found, err := referencesIssuer(path, src)
		if err != nil {
			t.Fatalf("parsing %s: %v", path, err)
		}
		if found {
			t.Errorf("%s references IssueTrustedActor but is outside the permitted issuer boundary", rel)
		}
	}
}

func TestProductionImportsAreRestricted(t *testing.T) {
	banned := map[string]bool{
		"os/exec":      true,
		"net/http":     true,
		"net":          true,
		"database/sql": true,
	}
	root := moduleRoot(t)
	for _, path := range productionGoFiles(t, root) {
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parsing %s: %v", path, err)
		}
		for _, imp := range f.Imports {
			if p := strings.Trim(imp.Path.Value, "\""); banned[p] {
				t.Errorf("%s imports %q, which is not allowed in production code", path, p)
			}
		}
	}
}
