package game

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// Every side stream is named by a constant in streams.go (#274): a
// string literal handed to Tick.Sub, SubRNG or the news sim's addOff is
// a stream nobody declared, and a typo in one would open a fresh stream
// and silently change a run. No two constants name the same stream.
func TestStreamsAreNamed(t *testing.T) {
	root := filepath.Join("..", "..")
	fset := token.NewFileSet()
	for _, dir := range []string{"internal", "cmd"} {
		err := filepath.WalkDir(filepath.Join(root, dir), func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return err
			}
			f, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				return err
			}
			ast.Inspect(f, func(n ast.Node) bool {
				c, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				sel, ok := c.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				var arg ast.Expr
				switch {
				case (sel.Sel.Name == "Sub" || sel.Sel.Name == "addOff") && len(c.Args) >= 1:
					arg = c.Args[0]
				case sel.Sel.Name == "SubRNG" && len(c.Args) == 3:
					arg = c.Args[2]
				default:
					return true
				}
				ast.Inspect(arg, func(n ast.Node) bool {
					if b, ok := n.(*ast.BasicLit); ok && b.Kind == token.STRING {
						t.Errorf("%s: stream %s is a literal: declare it in game/streams.go", fset.Position(b.Pos()), b.Value)
					}
					return true
				})
				return true
			})
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	f, err := parser.ParseFile(fset, "streams.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]string{}
	for _, d := range f.Decls {
		g, ok := d.(*ast.GenDecl)
		if !ok || g.Tok != token.CONST {
			continue
		}
		for _, sp := range g.Specs {
			vs := sp.(*ast.ValueSpec)
			v, _ := strconv.Unquote(vs.Values[0].(*ast.BasicLit).Value)
			if other, dup := seen[v]; dup {
				t.Errorf("%s and %s are both the stream %q", other, vs.Names[0].Name, v)
			}
			seen[v] = vs.Names[0].Name
		}
	}
	if len(seen) < 30 {
		t.Errorf("streams.go declares %d streams: the parse missed some", len(seen))
	}
}
