package architecture

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

const (
	cacheImport   = "bw-mcp/internal/cache"
	cliImport     = "bw-mcp/internal/bw"
	secretsImport = "bw-mcp/internal/secrets"
)

func TestLayerDependencies(t *testing.T) {
	cacheDeps := directImports(t, cacheImport)
	requireDependency(t, "cache", cacheDeps, cliImport)
	requireDependency(t, "cache", cacheDeps, secretsImport)

	cliDeps := directImports(t, cliImport)
	secretsDeps := directImports(t, secretsImport)
	for _, lowerLayer := range []struct {
		name string
		deps map[string]bool
	}{
		{name: "CLI", deps: cliDeps},
		{name: "secrets", deps: secretsDeps},
	} {
		if lowerLayer.deps[cacheImport] {
			t.Errorf("%s layer must not depend on cache", lowerLayer.name)
		}
	}
	if secretsDeps[cliImport] {
		t.Error("secrets layer must not depend on CLI")
	}
}

func requireDependency(t *testing.T, layer string, deps map[string]bool, required string) {
	t.Helper()
	if !deps[required] {
		t.Errorf("%s layer must directly depend on %s", layer, required)
	}
}

func directImports(t *testing.T, importPath string) map[string]bool {
	t.Helper()

	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not locate architecture test source")
	}
	moduleRoot := filepath.Clean(filepath.Join(filepath.Dir(currentFile), "../.."))
	pkgDir := filepath.Join(moduleRoot, strings.TrimPrefix(importPath, "bw-mcp/"))
	entries, err := os.ReadDir(pkgDir)
	if err != nil {
		t.Fatalf("read package %s: %v", importPath, err)
	}

	fset := token.NewFileSet()
	imports := make(map[string]bool)
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		path := filepath.Join(pkgDir, name)
		file, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parse imports in %s: %v", path, err)
		}
		for _, spec := range file.Imports {
			path, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				t.Fatalf("unquote import in %s: %v", file.Name.Name, err)
			}
			imports[path] = true
		}
	}
	return imports
}
