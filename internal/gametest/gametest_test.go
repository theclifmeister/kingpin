package gametest_test

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

const module = "github.com/theclifmeister/kingpin/"

// TestOnlyTestsImportGametest (#276): the fixture is for tests. No
// non-test Go file in the module (the game, the tooling, the harness's
// policies) imports gametest, and gametest itself imports game and
// events and nothing else of the module, so a sim's test that stands on
// it reaches no other sim. The check parses the import block of every
// .go file from the module root and names the file on a hit.
func TestOnlyTestsImportGametest(t *testing.T) {
	root := filepath.Join("..", "..")
	fset := token.NewFileSet()
	checked := 0
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if name := d.Name(); path != root && (strings.HasPrefix(name, ".") || name == "testdata") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		src, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		checked++
		own := filepath.Dir(path) == "."
		for _, imp := range src.Imports {
			p := strings.Trim(imp.Path.Value, `"`)
			switch {
			case own && strings.HasPrefix(p, module) && p != module+"internal/game" && p != module+"internal/events":
				t.Errorf("%s imports %s: gametest imports game and events only", path, p)
			case !own && p == module+"internal/gametest":
				t.Errorf("%s imports gametest: only a test may", path)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if checked < 100 {
		t.Fatalf("only %d files checked; the walk is wrong", checked)
	}
}
