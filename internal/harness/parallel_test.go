package harness

import (
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// The harness runs its tests beside each other (#212): every test is a
// pure function of its own content.MustLoad(), its own world and its
// seed's dice, so t.Parallel() is the first line of each one and the
// package-level state is read-only. This is the guard that keeps it
// so, since CI runs the harness without the race detector (#211) and
// a shared config or a memo would race silently:
//
//   - no function in the package, test or helper, assigns to or takes
//     the address of a package-level variable (TierDays, RetireAfter,
//     PaceDays, TreeOrder, moneyCurve, seedDigest stay read-only; a
//     cache would be a write);
//   - no package-level variable holds a content.Config, by value or
//     pointer, in any shape: a test loads its own;
//   - every Test function opens with t.Parallel(), unless it calls
//     t.Setenv (the save round-trip tests set KINGPIN_HOME, and Setenv
//     panics under Parallel), in which case it must not.
func TestHarnessTestsShareNothing(t *testing.T) {
	t.Parallel()
	fset := token.NewFileSet()
	names, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(names)
	var files []*ast.File
	for _, name := range names {
		f, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, f)
	}
	if len(files) < 20 {
		t.Fatalf("only %d files parsed; the glob is wrong", len(files))
	}
	info := &types.Info{Defs: map[*ast.Ident]types.Object{}, Uses: map[*ast.Ident]types.Object{}}
	conf := types.Config{Importer: importer.ForCompiler(fset, "source", nil)}
	pkg, err := conf.Check("harness", fset, files, info)
	if err != nil {
		t.Fatal(err)
	}
	scope := pkg.Scope()
	global := func(id *ast.Ident) *types.Var {
		obj := info.Uses[id]
		if obj == nil {
			obj = info.Defs[id]
		}
		v, ok := obj.(*types.Var)
		if !ok || v.Parent() != scope {
			return nil
		}
		return v
	}
	// root is the identifier a write lands on: x in x, x.f, x[i], *x, (x).
	var root func(ast.Expr) *ast.Ident
	root = func(e ast.Expr) *ast.Ident {
		switch e := e.(type) {
		case *ast.Ident:
			return e
		case *ast.SelectorExpr:
			return root(e.X)
		case *ast.IndexExpr:
			return root(e.X)
		case *ast.SliceExpr:
			return root(e.X)
		case *ast.StarExpr:
			return root(e.X)
		case *ast.ParenExpr:
			return root(e.X)
		}
		return nil
	}
	// A config is content.Config in any shape: the value, a pointer, a
	// slice or map of either, a struct that holds one.
	var holdsConfig func(types.Type, map[types.Type]bool) bool
	holdsConfig = func(ty types.Type, seen map[types.Type]bool) bool {
		if seen[ty] {
			return false
		}
		seen[ty] = true
		if n, ok := ty.(*types.Named); ok && n.Obj().Pkg() != nil && n.Obj().Pkg().Path() == "github.com/theclifmeister/kingpin/internal/content" && n.Obj().Name() == "Config" {
			return true
		}
		switch u := ty.Underlying().(type) {
		case *types.Pointer:
			return holdsConfig(u.Elem(), seen)
		case *types.Slice:
			return holdsConfig(u.Elem(), seen)
		case *types.Array:
			return holdsConfig(u.Elem(), seen)
		case *types.Map:
			return holdsConfig(u.Key(), seen) || holdsConfig(u.Elem(), seen)
		case *types.Struct:
			for i := 0; i < u.NumFields(); i++ {
				if holdsConfig(u.Field(i).Type(), seen) {
					return true
				}
			}
		}
		return false
	}
	var offenders []string
	at := func(p token.Pos) string { return fset.Position(p).String() }
	for _, name := range scope.Names() {
		if v, ok := scope.Lookup(name).(*types.Var); ok && holdsConfig(v.Type(), map[types.Type]bool{}) {
			offenders = append(offenders, at(v.Pos())+": package-level "+name+" holds a content.Config; a test loads its own")
		}
	}
	tests := 0
	for _, f := range files {
		for _, d := range f.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				var targets []ast.Expr
				switch s := n.(type) {
				case *ast.AssignStmt:
					if s.Tok != token.DEFINE {
						targets = s.Lhs
					}
				case *ast.IncDecStmt:
					targets = []ast.Expr{s.X}
				case *ast.UnaryExpr:
					if s.Op == token.AND {
						targets = []ast.Expr{s.X}
					}
				}
				for _, e := range targets {
					if id := root(e); id != nil {
						if v := global(id); v != nil {
							offenders = append(offenders, at(id.Pos())+": "+fn.Name.Name+" writes package-level "+v.Name()+"; a parallel test shares nothing")
						}
					}
				}
				return true
			})
			if !strings.HasPrefix(fn.Name.Name, "Test") || fn.Recv != nil || !strings.HasSuffix(fset.Position(fn.Pos()).Filename, "_test.go") {
				continue
			}
			tests++
			setenv := false
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				if c, ok := n.(*ast.CallExpr); ok {
					if sel, ok := c.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "Setenv" {
						if id, ok := sel.X.(*ast.Ident); ok && id.Name == "t" {
							setenv = true
						}
					}
				}
				return true
			})
			parallel := false
			if len(fn.Body.List) > 0 {
				if es, ok := fn.Body.List[0].(*ast.ExprStmt); ok {
					if c, ok := es.X.(*ast.CallExpr); ok {
						if sel, ok := c.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "Parallel" {
							parallel = true
						}
					}
				}
			}
			switch {
			case setenv && parallel:
				offenders = append(offenders, at(fn.Pos())+": "+fn.Name.Name+" calls t.Setenv under t.Parallel(); it panics")
			case !setenv && !parallel:
				offenders = append(offenders, at(fn.Pos())+": "+fn.Name.Name+" does not open with t.Parallel(); a harness test runs beside the others unless it sets the environment")
			}
		}
	}
	if tests < 150 {
		t.Fatalf("only %d tests seen; the walk is wrong", tests)
	}
	if len(offenders) > 0 {
		t.Errorf("the harness shares state between tests (#212):\n  %s", strings.Join(offenders, "\n  "))
	}
}
