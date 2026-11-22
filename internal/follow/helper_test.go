package follow

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// testHelperImport is the import path of the package the guard exempts
// because only tests import it.
const testHelperImport = "github.com/seanthegeek/doubleagent/internal/collect/imagetest"

func TestTheBinaryLinksNoTestHelper(t *testing.T) {
	importers, err := nonTestImporters("../..", testHelperImport)
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range importers {
		t.Errorf("%s imports %s, which only tests may import", file, testHelperImport)
	}
}

// nonTestImporters returns each Go file under root, not a test and not
// vendored, that imports wanted, outside the wanted package itself.
func nonTestImporters(root, wanted string) ([]string, error) {
	var importers []string
	files := token.NewFileSet()
	err := filepath.WalkDir(root, func(file string, entry fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
		case entry.IsDir() && entry.Name() == "vendor":
			return filepath.SkipDir
		case entry.IsDir() || !strings.HasSuffix(file, ".go") || strings.HasSuffix(file, "_test.go"):
			return nil
		}
		parsed, err := parser.ParseFile(files, file, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, spec := range parsed.Imports {
			if path, _ := strconv.Unquote(spec.Path.Value); path == wanted &&
				!strings.HasSuffix(filepath.ToSlash(filepath.Dir(file)), "internal/collect/imagetest") {
				importers = append(importers, file)
			}
		}
		return nil
	})
	return importers, err
}
