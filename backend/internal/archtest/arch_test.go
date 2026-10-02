// Package archtest enforces the hexagonal dependency rule with a plain go test,
// so a layering mistake fails CI instead of slipping through review.
package archtest_test

import (
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

const module = "github.com/CaioAP/shinrin/backend/"

// allowed lists, per layer, which internal layers its non-test code may
// import. A layer is the first one or two path segments under internal/.
var allowed = map[string][]string{
	"domain":      {"domain"}, // domain subpackages (indicators, scoring) may share core types
	"port":        {"domain"},
	"config":      {},
	"adapter":     {}, // doc.go only
	"app":         {"domain", "port"},
	"adapter/in":  {"domain", "port"},
	"adapter/out": {"domain", "port"},
	"httpx":       {}, // outbound HTTP decorators, wired in cmd
}

func layerOf(rel string) string {
	parts := strings.Split(filepath.ToSlash(rel), "/")
	if parts[0] == "adapter" && len(parts) > 1 {
		return "adapter/" + parts[1]
	}
	return parts[0]
}

func TestDependencyRule(t *testing.T) {
	root := filepath.Join("..")
	fset := token.NewFileSet()

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		layer := layerOf(filepath.Dir(rel))
		if layer == "archtest" {
			return nil
		}
		rules, known := allowed[layer]
		if !known {
			t.Errorf("%s: package is outside the known layers; add it to archtest.allowed and docs/conventions.md", rel)
			return nil
		}

		f, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		pkgDir := filepath.ToSlash(filepath.Dir(rel))
		for _, imp := range f.Imports {
			p, _ := strconv.Unquote(imp.Path.Value)
			if !strings.HasPrefix(p, module+"internal/") {
				continue
			}
			target := strings.TrimPrefix(p, module+"internal/")
			if target == pkgDir || strings.HasPrefix(target, pkgDir+"/") {
				continue // a package may use its own subpackages
			}
			if !contains(rules, layerOf(target)) {
				t.Errorf("%s (layer %s) imports %s (layer %s), which the dependency rule forbids", rel, layer, target, layerOf(target))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestDomainUsesOnlyStdlib(t *testing.T) {
	fset := token.NewFileSet()
	dir := filepath.Join("..", "domain")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(dir, e.Name()), nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, imp := range f.Imports {
			p, _ := strconv.Unquote(imp.Path.Value)
			if first := strings.Split(p, "/")[0]; strings.Contains(first, ".") {
				t.Errorf("domain/%s imports third-party package %s", e.Name(), p)
			}
		}
	}
}

func contains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}
