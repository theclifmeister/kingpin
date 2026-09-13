package harness

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// CI runs internal/harness and internal/ui without the race detector
// (#211): the detector only reports a conflict between two goroutines,
// the tree starts none, and the two packages paid 7.4x and 5.7x for it.
// This is the guard that keeps the exemption honest. It parses every
// .go file under internal/ and fails on
//
//   - a go statement or a channel type (chan T, a send or a receive, a
//     select) in any file, test files included, since a parallel test
//     that starts a goroutine is exactly what the detector would have
//     caught;
//   - an import of sync or sync/atomic in any non-test file: a lock is
//     a goroutine's tell, and a lock-free counter is a race waiting for
//     one. A test file may import sync (TestEveryIncidentFires's tally
//     behind a mutex, #212: its chunks are the testing package's own
//     goroutines, and TestHarnessTestsShareNothing guards what they
//     share).
//
// The exceptions, each named so the list can only shrink:
//
//   - internal/ui/anim/canvas.go imports sync for the one mutex in the
//     tree, round the package-level style cache that every rendered run
//     reads: the cache is shared by whatever renders a frame, and the
//     package stays under -race in ci.yml (only internal/ui itself is
//     exempt, not internal/ui/...), so the detector still watches it.
//   - Bubble Tea's own runtime (tea.Program: its event loop, the
//     renderer, a Cmd's goroutine) is started by cmd/kingpin and
//     cmd/anim and never by a test: the UI tests call Model.Update and
//     View directly and a tea.Cmd is returned, never run, so no
//     goroutine of Bubble Tea's exists under go test. It lives outside
//     internal/ and is not walked.
//
// The PR that first adds a goroutine to internal/harness or internal/ui
// adds the file here and puts -race back on that package in ci.yml in
// the same PR.
func TestNoGoroutineInTheTree(t *testing.T) {
	t.Parallel()
	root := filepath.Join("..")
	syncAllowed := map[string]bool{
		filepath.Join("ui", "anim", "canvas.go"): true,
	}
	fset := token.NewFileSet()
	var names []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, ".go") {
			names = append(names, path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(names)
	if len(names) < 100 {
		t.Fatalf("only %d files walked; the root is wrong", len(names))
	}
	var offenders []string
	at := func(p token.Pos) string { return fset.Position(p).String() }
	for _, name := range names {
		f, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		rel, _ := filepath.Rel(root, name)
		test := strings.HasSuffix(name, "_test.go")
		for _, imp := range f.Imports {
			path := strings.Trim(imp.Path.Value, `"`)
			if (path == "sync" || path == "sync/atomic") && !test && !syncAllowed[rel] {
				offenders = append(offenders, at(imp.Pos())+": imports "+path+"; nothing in the tree locks, since nothing in it starts a goroutine")
			}
		}
		ast.Inspect(f, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.GoStmt:
				offenders = append(offenders, at(n.Pos())+": a go statement; CI runs internal/harness and internal/ui without -race, so a goroutine there puts it back (#211)")
			case *ast.ChanType:
				offenders = append(offenders, at(n.Pos())+": a channel type; a channel is a goroutine's tell")
			case *ast.SendStmt:
				offenders = append(offenders, at(n.Pos())+": a channel send")
			case *ast.SelectStmt:
				offenders = append(offenders, at(n.Pos())+": a select; a channel is a goroutine's tell")
			case *ast.UnaryExpr:
				if n.Op == token.ARROW {
					offenders = append(offenders, at(n.Pos())+": a channel receive")
				}
			}
			return true
		})
	}
	if len(offenders) > 0 {
		t.Errorf("the tree starts a goroutine, or looks like it (#211):\n  %s", strings.Join(offenders, "\n  "))
	}
}

// TestNoWallClockInTheSims (#50): time.Now is read in the UI alone
// (the start menu's daily and the profile's date) and passed in, so a
// run is a function of its seed and its inputs and every test passes
// a date. The guard parses every non-test .go file under
// internal/game, internal/sim, internal/content and internal/harness
// and fails on a call of time.Now (or time.Since, time.Until, which
// read it), cmd/balance walked with them.
func TestNoWallClockInTheSims(t *testing.T) {
	t.Parallel()
	roots := []string{filepath.Join("..", "game"), filepath.Join("..", "sim"), filepath.Join("..", "content"), filepath.Join("..", "harness"), filepath.Join("..", "..", "cmd", "balance")}
	fset := token.NewFileSet()
	var offenders []string
	walked := 0
	for _, root := range roots {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			walked++
			f, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				return err
			}
			ast.Inspect(f, func(n ast.Node) bool {
				sel, ok := n.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				if id, ok := sel.X.(*ast.Ident); ok && id.Name == "time" && (sel.Sel.Name == "Now" || sel.Sel.Name == "Since" || sel.Sel.Name == "Until") {
					offenders = append(offenders, fset.Position(sel.Pos()).String()+": time."+sel.Sel.Name+"; the wall clock is the UI's alone (#50)")
				}
				return true
			})
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if walked < 50 {
		t.Fatalf("only %d files walked; the roots are wrong", walked)
	}
	if len(offenders) > 0 {
		t.Errorf("the sims read the wall clock:\n  %s", strings.Join(offenders, "\n  "))
	}
}
