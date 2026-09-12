package sim_test

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSimsNeverImportEachOther: no package under internal/sim/ imports
// another under internal/sim/ (#144). Events and World state are the only
// cross-sim channel; a query two sims share (news.Eligible was one, read
// by the market's buyers) lives in game beside FoldEffects. The check
// parses the import block of every non-test Go file under internal/sim/*
// (this package, the assembler, is exempt: it imports every sim by
// design) and names the file and the edge on a hit.
func TestSimsNeverImportEachOther(t *testing.T) {
	const prefix = "github.com/theclifmeister/kingpin/internal/sim/"
	dirs, err := filepath.Glob("*")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	checked := 0
	for _, dir := range dirs {
		if st, err := os.Stat(dir); err != nil || !st.IsDir() {
			continue
		}
		files, err := filepath.Glob(filepath.Join(dir, "*.go"))
		if err != nil {
			t.Fatal(err)
		}
		for _, f := range files {
			if strings.HasSuffix(f, "_test.go") {
				continue
			}
			src, err := parser.ParseFile(fset, f, nil, parser.ImportsOnly)
			if err != nil {
				t.Fatal(err)
			}
			checked++
			for _, imp := range src.Imports {
				path := strings.Trim(imp.Path.Value, `"`)
				if strings.HasPrefix(path, prefix) {
					t.Errorf("%s imports %s: sims never import each other", f, strings.TrimPrefix(path, prefix))
				}
			}
		}
	}
	if checked < 10 {
		t.Fatalf("only %d sim files checked; the glob is wrong", checked)
	}
}
