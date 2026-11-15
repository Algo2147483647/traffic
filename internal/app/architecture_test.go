package app

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// This guard prevents reintroducing static->runtime and executor->world dependencies.
func TestDependencyBoundaries(t *testing.T) {
	root := filepath.Join("..", "..")
	err := filepath.WalkDir(filepath.Join(root, "internal"), func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)
		f, e := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if e != nil {
			return e
		}
		for _, i := range f.Imports {
			imp, _ := strconv.Unquote(i.Path.Value)
			runtime := strings.Contains(imp, "_scheduling") || strings.HasPrefix(imp, "traffic/internal/simulation") || strings.HasPrefix(imp, "traffic/internal/server") || strings.HasPrefix(imp, "traffic/internal/app")
			if (strings.HasPrefix(rel, "internal/infrastructure/") || strings.HasPrefix(rel, "internal/operations/") || strings.HasPrefix(rel, "internal/algorithms/")) && runtime {
				t.Errorf("%s imports higher layer %s", rel, imp)
			}
			if strings.HasPrefix(rel, "internal/road_scheduling/") && (strings.Contains(imp, "metro_scheduling") || strings.Contains(imp, "internal/simulation")) {
				t.Errorf("road executor owns another runtime: %s", imp)
			}
			if strings.HasPrefix(rel, "internal/metro_scheduling/") && (strings.Contains(imp, "road_scheduling") || strings.Contains(imp, "internal/simulation")) {
				t.Errorf("metro executor owns another runtime: %s", imp)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
