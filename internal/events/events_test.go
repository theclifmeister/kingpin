package events_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/theclifmeister/kingpin/internal/events"
)

// All is every kind (#274): the test parses the package's source, finds
// every type with a Kind method, and fails on one missing from All, on
// one listed twice, and on a Kind that is not the one line returning
// the type's own name, so a kind's name is its type's and the list a
// test walks is the list the package declares.
func TestAllListsEveryKind(t *testing.T) {
	fset := token.NewFileSet()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	declared := map[string]bool{}
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range f.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok || fn.Name.Name != "Kind" || fn.Recv == nil || len(fn.Recv.List) != 1 {
				continue
			}
			recv, ok := fn.Recv.List[0].Type.(*ast.Ident)
			if !ok {
				t.Errorf("%s: Kind has a pointer or generic receiver; an event is a value", fset.Position(fn.Pos()))
				continue
			}
			declared[recv.Name] = true
			if got := returnedName(fn); got != recv.Name {
				t.Errorf("%s.Kind() returns %q; a kind is its type's name", recv.Name, got)
			}
		}
	}
	if len(declared) == 0 {
		t.Fatal("found no Kind method; the parse is wrong")
	}
	listed := map[string]bool{}
	for _, e := range events.All {
		name := reflect.TypeOf(e).Name()
		if listed[name] {
			t.Errorf("%s is in events.All twice", name)
		}
		listed[name] = true
		if e.Kind() != name {
			t.Errorf("%s.Kind() = %q", name, e.Kind())
		}
		if !declared[name] {
			t.Errorf("%s is in events.All but declares no Kind in the package's source", name)
		}
	}
	for name := range declared {
		if !listed[name] {
			t.Errorf("%s has a Kind method but is missing from events.All", name)
		}
	}
}

// returnedName is the string a one-line Kind returns, or "" when the
// body is anything else.
func returnedName(fn *ast.FuncDecl) string {
	if fn.Body == nil || len(fn.Body.List) != 1 {
		return ""
	}
	ret, ok := fn.Body.List[0].(*ast.ReturnStmt)
	if !ok || len(ret.Results) != 1 {
		return ""
	}
	lit, ok := ret.Results[0].(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return ""
	}
	s, err := strconv.Unquote(lit.Value)
	if err != nil {
		return ""
	}
	return s
}
