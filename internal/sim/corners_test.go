package sim_test

import (
	"errors"
	"go/ast"
	"go/printer"
	"go/token"
	"go/types"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"

	"golang.org/x/tools/go/packages"
)

// The corner is the territory sim's (#274): what it holds, who holds it
// and who stands on it. TestSimsWriteOnlyTheirOwnState sees only the
// writes that start with `w.`, and every sim reaches a corner through a
// *City (`c := &city.Corners[i]`), so the two tests here read the tree
// type-checked instead: every assignment into a field of a game.Corner,
// however the corner was reached, and every call of a method of the
// corner's that writes it (Hand).

const module = "github.com/theclifmeister/kingpin/"

// cornerWrite is one write into a game.Corner found in the tree: the
// package it is in (the path under internal/, or cmd/...), the file and
// line, the field (or "Hand()" for a call of the method; "*" for a whole
// Corner assigned) and the statement.
type cornerWrite struct {
	pkg, file   string
	line        int
	field, stmt string
}

func (w cornerWrite) String() string {
	return w.file + ":" + strconv.Itoa(w.line) + ": " + w.stmt + "  (" + w.pkg + " writes " + w.field + ")"
}

var (
	cornerWritesOnce sync.Once
	cornerWritesAll  []cornerWrite
	cornerWritesErr  error
)

// cornerWrites loads every package in the module, type-checked, without
// its tests, and returns every statement that writes a game.Corner:
// `c.Squeeze = x` through a `c := &city.Corners[i]`,
// `w.Home().Corners[0].Owner = y`, `c.Taste[p] *= 2` (a write to Taste),
// `c.Deed.Price = n` (to Deed), `c.Robbed++`, `*c = x` and
// `city.Corners[i] = x` (to "*"), `c.Hand(...)` ("Hand()"). A pointer to a
// corner re-aimed (`best = c`) or a copy in a local is not a write.
// A World method that writes a corner (Post, Recall, Abandon, SeizeDeed)
// is game's, the package this test leaves alone, and is not listed at
// its callers.
func cornerWrites(t *testing.T) []cornerWrite {
	t.Helper()
	cornerWritesOnce.Do(func() {
		fset := token.NewFileSet()
		root, err := filepath.Abs(filepath.Join("..", ".."))
		if err != nil {
			cornerWritesErr = err
			return
		}
		cfg := &packages.Config{
			Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo,
			Dir:  root,
			Fset: fset,
		}
		pkgs, err := packages.Load(cfg, "./...")
		if err != nil {
			cornerWritesErr = err
			return
		}
		if len(pkgs) < 20 {
			cornerWritesErr = errors.New("only " + strconv.Itoa(len(pkgs)) + " packages loaded; the pattern is wrong")
			return
		}
		for _, p := range pkgs {
			if len(p.Errors) > 0 {
				cornerWritesErr = p.Errors[0]
				return
			}
			name := strings.TrimPrefix(strings.TrimPrefix(p.PkgPath, module), "internal/")
			add := func(n ast.Node, field string) {
				at := fset.Position(n.Pos())
				rel, _ := filepath.Rel(root, at.Filename)
				var b strings.Builder
				_ = printer.Fprint(&b, fset, n)
				cornerWritesAll = append(cornerWritesAll, cornerWrite{
					pkg:   name,
					file:  filepath.ToSlash(rel),
					line:  at.Line,
					field: field,
					stmt:  strings.Join(strings.Fields(b.String()), " "),
				})
			}
			for _, f := range p.Syntax {
				ast.Inspect(f, func(n ast.Node) bool {
					var lhs []ast.Expr
					switch s := n.(type) {
					case *ast.AssignStmt:
						if s.Tok == token.DEFINE {
							return true
						}
						lhs = s.Lhs
					case *ast.IncDecStmt:
						lhs = []ast.Expr{s.X}
					case *ast.CallExpr:
						if m := cornerMethod(p.TypesInfo, s); m != "" {
							add(n, m)
						}
						return true
					default:
						return true
					}
					for _, x := range lhs {
						if field := cornerField(p.TypesInfo, x); field != "" {
							add(n, field)
						}
					}
					return true
				})
			}
		}
	})
	if cornerWritesErr != nil {
		t.Fatal(cornerWritesErr)
	}
	return cornerWritesAll
}

// cornerField is the Corner field an assignment's left-hand side writes,
// walking in through indexes, derefs and selectors to the outermost
// field of a Corner; "*" for a whole Corner, "" for anything else.
func cornerField(info *types.Info, x ast.Expr) string {
	if t := info.TypeOf(x); t != nil && isCorner(t) {
		if _, ptr := types.Unalias(t).(*types.Pointer); ptr {
			return "" // a *Corner aimed somewhere else writes no corner
		}
		if _, local := x.(*ast.Ident); local {
			return "" // a copy in a local is not the world's
		}
		return "*"
	}
	for {
		switch e := x.(type) {
		case *ast.ParenExpr:
			x = e.X
		case *ast.StarExpr:
			x = e.X
		case *ast.IndexExpr:
			x = e.X
		case *ast.SelectorExpr:
			if sel := info.Selections[e]; sel != nil && sel.Kind() == types.FieldVal && isCorner(sel.Recv()) {
				return e.Sel.Name
			}
			x = e.X
		default:
			return ""
		}
	}
}

// cornerMethod is "Name()" for a call of a method of game.Corner with a
// pointer receiver (one that can write the corner: Hand), "" otherwise.
func cornerMethod(info *types.Info, call *ast.CallExpr) string {
	e, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return ""
	}
	sel := info.Selections[e]
	if sel == nil || sel.Kind() != types.MethodVal {
		return ""
	}
	recv := sel.Obj().Type().(*types.Signature).Recv().Type()
	if _, ptr := types.Unalias(recv).(*types.Pointer); !ptr || !isCorner(recv) {
		return ""
	}
	return e.Sel.Name + "()"
}

// isCorner reports whether t is game.Corner or a pointer to one.
func isCorner(t types.Type) bool {
	if p, ok := types.Unalias(t).(*types.Pointer); ok {
		t = p.Elem()
	}
	n, ok := types.Unalias(t).(*types.Named)
	return ok && n.Obj().Name() == "Corner" && n.Obj().Pkg() != nil && n.Obj().Pkg().Path() == module+"internal/game"
}

// TestCornerOwnerIsHanded (#274): outside internal/game nothing assigns a
// corner's Owner or Faction, or a whole Corner; a change of holder goes
// through game.(*Corner).Hand (#281), so no two hand-overs can disagree
// on what a new holder inherits. The exceptions are named here so the
// list can only shrink: the demo world ui/demo.go builds for the scenes
// (cmd/anim, the captures) paints home's first corner the rival's and
// the strike's corner yours for a frame; it is a picture of a map, never
// a hand-over in a run, and Hand's resets (Since, the rival's Faction)
// would move what the scenes draw.
func TestCornerOwnerIsHanded(t *testing.T) {
	allowed := map[string]bool{
		"internal/ui/demo.go: home.Corners[0].Owner, home.Corners[0].Runner, home.Corners[0].Enforcer = game.OwnerRival, 0, 0": true,
		"internal/ui/demo.go: w.Home().Corners[0].Owner = game.OwnerRival":                                                     true,
		"internal/ui/demo.go: c.Owner = game.OwnerPlayer":                                                                      true,
	}
	seen := map[string]bool{}
	var offenders []string
	for _, w := range cornerWrites(t) {
		if w.pkg == "game" || (w.field != "Owner" && w.field != "Faction" && w.field != "*") {
			continue
		}
		key := w.file + ": " + w.stmt
		if allowed[key] {
			seen[key] = true
			continue
		}
		offenders = append(offenders, w.String())
	}
	if len(offenders) > 0 {
		t.Errorf("a corner changes hands outside Corner.Hand:\n  %s", strings.Join(offenders, "\n  "))
	}
	for key := range allowed {
		if !seen[key] {
			t.Errorf("exception no longer in the source, drop it from the list: %s", key)
		}
	}
}

// cornerWriters is who writes a corner besides its owner, the territory
// sim (#274, docs/corners.md "Who writes a corner"), and game (the World
// methods: Hand, Post, Recall, Abandon, BuyDeed, SeizeDeed, the
// migrations). A field listed is written directly; "Hand()" is a
// hand-over. Each row is ruled on in the doc:
//
//   - sim (the assembler, not a sim): the day-0 corners of a character
//     (#232), handed to you or, the one you stand on, its clock restarted.
//   - sim/crew: the lieutenant's walk hands a corner to the faction he
//     walks to or the street (#144 PR 2), and his claim stamps Since on
//     a street corner he posts on the night he takes it.
//   - sim/market: Repeat, its field on the corner (#47); Squeeze on the
//     rival's corners, the player's price war (#68: zeroed and written
//     each step, openWar and closeWar).
//   - sim/rivals: Squeeze on the player's corners, the undercut (zeroed
//     once a step, then the deepest faction's); Starved and StarvedDay,
//     the price war's count on its own corners; every hand-over a faction
//     makes (take, the strike, a crackdown, books, the war, fragmenting).
//   - ui: the demo world's corner (TestCornerOwnerIsHanded says why).
//
// Squeeze is one field two sims write, on disjoint corners (the player's
// for the rivals, the rival's for the market); splitting it would change
// the save's shape, left as a follow-up.
var cornerWriters = map[string][]string{
	"sim":        {"Hand()", "Idle", "Since"},
	"sim/crew":   {"Hand()", "Since"},
	"sim/market": {"Repeat", "Squeeze"},
	"sim/rivals": {"Hand()", "Squeeze", "Starved", "StarvedDay"},
	"ui":         {"Enforcer", "Owner", "Runner"},
}

// TestCornerWritersAreDeclared (#274): the writes into a corner from
// every package but game and the territory sim are exactly the table
// above, so a new cross-sim write fails until it is declared (and ruled
// on in docs/corners.md), and a row the code no longer needs fails until
// it is dropped.
func TestCornerWritersAreDeclared(t *testing.T) {
	got := map[string][]string{}
	where := map[string]string{}
	for _, w := range cornerWrites(t) {
		if w.pkg == "game" || w.pkg == "sim/territory" {
			continue
		}
		if !slices.Contains(got[w.pkg], w.field) {
			got[w.pkg] = append(got[w.pkg], w.field)
			where[w.pkg+" "+w.field] = w.String()
		}
	}
	for pkg, fields := range got {
		for _, f := range fields {
			if !slices.Contains(cornerWriters[pkg], f) {
				t.Errorf("an undeclared corner write; add %s %s to cornerWriters and docs/corners.md, or hand it to the territory sim:\n  %s", pkg, f, where[pkg+" "+f])
			}
		}
	}
	for pkg, fields := range cornerWriters {
		for _, f := range fields {
			if !slices.Contains(got[pkg], f) {
				t.Errorf("%s no longer writes a corner's %s; drop it from cornerWriters and docs/corners.md", pkg, f)
			}
		}
	}
}
