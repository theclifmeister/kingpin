package anim

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// Play's guards (#322): an effect a scene plays falls back to the Still
// of its text wherever its Needs are not met, and plays wherever they
// are; and no scene calls the Maker of an effect that needs more than a
// cell but through Play.

// TestPlayFallsBackToStill: every effect through Play, over both of the
// set's texts, one column under its minimum width and one row under its
// minimum height (where the minimum is above one), is Still's frame at
// every moment of its run, by Frame and by paint under a Layer alike;
// with no text where it needs one, too. At 80x24, and at the exact
// minimum over a one-cell text, some frame before Done is not Still's:
// the effect plays.
func TestPlayFallsBackToStill(t *testing.T) {
	for _, name := range Names() {
		e := Effects[name]
		var under [][2]int
		if e.Needs.MinW > 1 {
			under = append(under, [2]int{e.Needs.MinW - 1, max(1, e.Needs.MinH)})
		}
		if e.Needs.MinH > 1 {
			under = append(under, [2]int{max(1, e.Needs.MinW), e.Needs.MinH - 1})
		}
		for _, tx := range effectTexts {
			still := Still(tx.text, theme.Money)
			for _, sz := range under {
				s := Play(e, tx.text, theme.Money, tx.over, Seed(1, 0, name))
				l := Layer(Play(e, tx.text, theme.Money, tx.over, Seed(1, 0, name)))
				want := strings.Join(still.Frame(0, sz[0], sz[1]), "\n")
				for at := time.Duration(0); at <= tx.over; at += Frame {
					if got := strings.Join(s.Frame(at, sz[0], sz[1]), "\n"); got != want {
						t.Errorf("%s over %s at %dx%d, %v: not the still:\n%s", name, tx.name, sz[0], sz[1], at, plain(s.Frame(at, sz[0], sz[1])))
						break
					}
					if got := strings.Join(l.Frame(at, sz[0], sz[1]), "\n"); got != want {
						t.Errorf("%s over %s at %dx%d, %v: not the still under a layer:\n%s", name, tx.name, sz[0], sz[1], at, plain(l.Frame(at, sz[0], sz[1])))
						break
					}
				}
			}
			s := Play(e, tx.text, theme.Money, tx.over, Seed(1, 0, name))
			if s.Done(tx.over-Frame) || !s.Done(tx.over) {
				t.Errorf("%s over %s: Done is not the effect's", name, tx.name)
			}
			if !moves(s, still, tx.over, 80, 24) {
				t.Errorf("%s over %s: at 80x24 every frame is the still", name, tx.name)
			}
		}
		if e.Needs.Text {
			s := Play(e, Text{}, theme.Money, time.Second, Seed(1, 0, name))
			for at := time.Duration(0); at <= time.Second; at += Frame {
				if strings.TrimSpace(plain(s.Frame(at, 80, 24))) != "" {
					t.Errorf("%s with no text at %v draws something", name, at)
					break
				}
			}
		}
		one := NewText("X")
		w, h := max(1, e.Needs.MinW), max(1, e.Needs.MinH)
		if !moves(Play(e, one, theme.Money, time.Second, Seed(1, 0, name)), Still(one, theme.Money), time.Second, w, h) {
			t.Errorf("%s at its minimum %dx%d: every frame is the still", name, w, h)
		}
	}
}

// moves reports whether some frame of s before over differs from the
// still's at the size.
func moves(s, still Scene, over time.Duration, w, h int) bool {
	want := strings.Join(still.Frame(0, w, h), "\n")
	for at := time.Duration(0); at < over; at += Frame {
		if strings.Join(s.Frame(at, w, h), "\n") != want {
			return true
		}
	}
	return false
}

// TestEffectsArePlayed: in the package's own code, the Maker of every
// effect in the table whose Needs are more than one cell (beams, slide,
// vhstape, pour, rain, matrix) is named only in the file that defines
// it and in the table itself; every scene takes it through Play, so a
// canvas too small for it plays the Still. Makers built from them
// (SlideFrom, PourFrom, Reverse of one) are the scenes' own business.
func TestEffectsArePlayed(t *testing.T) {
	guarded := map[string]string{} // the Maker's name, the effect's
	for _, name := range Names() {
		e := Effects[name]
		if e.Needs.MinW <= 1 && e.Needs.MinH <= 1 {
			continue
		}
		fn := runtime.FuncForPC(reflect.ValueOf(e.New).Pointer()).Name()
		guarded[fn[strings.LastIndex(fn, ".")+1:]] = name
	}
	if len(guarded) == 0 {
		t.Fatal("no effect needs more than a cell: the guard guards nothing")
	}
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	parsed := map[string]*ast.File{}
	home := map[string]string{} // the Maker's name, the file declaring it
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		file, err := parser.ParseFile(fset, f, src, 0)
		if err != nil {
			t.Fatal(err)
		}
		parsed[f] = file
		for _, d := range file.Decls {
			if fd, ok := d.(*ast.FuncDecl); ok && fd.Recv == nil && guarded[fd.Name.Name] != "" {
				home[fd.Name.Name] = f
			}
		}
	}
	for m := range guarded {
		if home[m] == "" {
			t.Errorf("%s: no file declares it", m)
		}
	}
	for f, file := range parsed {
		// x.Rain is a field or a method, never the Maker.
		sel := map[*ast.Ident]bool{}
		ast.Inspect(file, func(n ast.Node) bool {
			if se, ok := n.(*ast.SelectorExpr); ok {
				sel[se.Sel] = true
			}
			return true
		})
		ast.Inspect(file, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.ValueSpec:
				// The table names every Maker: that is what it is for.
				for _, id := range n.Names {
					if id.Name == "Effects" {
						return false
					}
				}
			case *ast.Ident:
				if name := guarded[n.Name]; name != "" && !sel[n] && f != home[n.Name] {
					t.Errorf("%s: %s names %s directly; play it with Play(Effects[%q], …)", fset.Position(n.Pos()), f, n.Name, name)
				}
			}
			return true
		})
	}
}
