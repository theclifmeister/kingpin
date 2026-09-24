package engine_test

import (
	"encoding/json"
	"flag"
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/engine"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/harness"
)

var update = flag.Bool("update", false, "rewrite testdata/view_shape.txt from engine.View")

const shapeFile = "testdata/view_shape.txt"

// shape is View's shape, one line a field: its JSON path and its Go
// kind, depth first in declaration order. Values never enter it, so it
// is the same on every machine.
func shape() []string {
	var out []string
	seen := map[reflect.Type]bool{}
	var walk func(path string, t reflect.Type)
	walk = func(path string, t reflect.Type) {
		switch t.Kind() {
		case reflect.Pointer:
			walk(path, t.Elem())
			return
		case reflect.Slice:
			out = append(out, path+" []")
			walk(path+"[]", t.Elem())
			return
		case reflect.Map:
			out = append(out, path+" map["+t.Key().Kind().String()+"]")
			walk(path+"{}", t.Elem())
			return
		case reflect.Struct:
			if seen[t] {
				out = append(out, path+" "+t.Name())
				return
			}
			seen[t] = true
			defer delete(seen, t)
			for i := 0; i < t.NumField(); i++ {
				f := t.Field(i)
				name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
				if name == "" {
					name = f.Name
				}
				walk(path+"."+name, f.Type)
			}
			return
		}
		out = append(out, path+" "+t.Kind().String())
	}
	walk("", reflect.TypeOf(engine.View{}))
	return out
}

// TestViewShapeIsPinned (#299): the view is the contract a front end in
// another process is written against, so its shape is pinned in
// testdata/view_shape.txt under the version it had. A field added,
// renamed, retyped or dropped without moving engine.ViewVersion fails
// here; with it moved, `go test ./internal/engine -run
// TestViewShapeIsPinned -update` writes the new shape.
func TestViewShapeIsPinned(t *testing.T) {
	got := append([]string{fmt.Sprintf("version %d", engine.ViewVersion)}, shape()...)
	if *update {
		if err := os.WriteFile(shapeFile, []byte(strings.Join(got, "\n")+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	raw, err := os.ReadFile(shapeFile)
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Split(strings.TrimRight(string(raw), "\n"), "\n")
	if strings.Join(got, "\n") == strings.Join(want, "\n") {
		return
	}
	if got[0] == want[0] {
		t.Fatalf("engine.View changed shape at %s; bump engine.ViewVersion and run with -update\n%s", got[0], diff(want[1:], got[1:]))
	}
	t.Fatalf("engine.ViewVersion moved to %d; run with -update to pin its shape\n%s", engine.ViewVersion, diff(want[1:], got[1:]))
}

// diff is the lines only one side has.
func diff(want, got []string) string {
	in := func(xs []string) map[string]bool {
		m := map[string]bool{}
		for _, x := range xs {
			m[x] = true
		}
		return m
	}
	w, g := in(want), in(got)
	var out []string
	for _, x := range want {
		if !g[x] {
			out = append(out, "- "+x)
		}
	}
	for _, x := range got {
		if !w[x] {
			out = append(out, "+ "+x)
		}
	}
	return strings.Join(out, "\n")
}

// playedSession is a session thirty days into the informed player's run,
// the policy that pays the cop, scouts and plants: a file with facts in
// it.
func playedSession(t *testing.T, days int) (*engine.Session, *game.World) {
	t.Helper()
	cfg := content.MustLoad()
	s, err := engine.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	w := s.NewRun(7, game.Start{})
	policy := harness.Trader(cfg, events.DialNormal)
	for _, p := range harness.Policies {
		if p.Name == "informed" {
			policy = p.Make(cfg, harness.PolicyOpts{})
		}
	}
	for d := 0; d < days && w.Over == nil; d++ {
		policy(w)
		s.EndDay()
	}
	return s, w
}

// TestViewRoundTripsJSON (#299): the view is plain data, the same after
// a trip through JSON.
func TestViewRoundTripsJSON(t *testing.T) {
	t.Parallel()
	s, _ := playedSession(t, 40)
	v := s.View()
	if v.Version != engine.ViewVersion || len(v.Cities) == 0 || len(v.Factions) == 0 {
		t.Fatalf("a thin view: version %d, %d cities, %d factions", v.Version, len(v.Cities), len(v.Factions))
	}
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var back engine.View
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatal(err)
	}
	again, err := json.Marshal(back)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != string(again) {
		t.Fatal("the view is not the same after a trip through JSON")
	}
	if (&engine.Session{}).View().Version != engine.ViewVersion {
		t.Fatal("the view before a run has no version")
	}
	// Day 0: no morning report yet (World.Report is nil until the first
	// day ends), and the view is still whole.
	fresh, err := engine.New(content.MustLoad())
	if err != nil {
		t.Fatal(err)
	}
	fresh.NewRun(7, game.Start{})
	if v := fresh.View(); v.Day != 0 || v.Report.Day != 0 || len(v.Cities) == 0 {
		t.Fatalf("the day-0 view: day %d, report day %d, %d cities", v.Day, v.Report.Day, len(v.Cities))
	}
}

// TestViewHoldsNothingOfTheWorld (#299): a front end may keep and change
// a view; the run does not move with it.
func TestViewHoldsNothingOfTheWorld(t *testing.T) {
	t.Parallel()
	s, w := playedSession(t, 20)
	before, _ := json.Marshal(w)
	v := s.View()
	for i := range v.Cities {
		for j := range v.Cities[i].Products {
			for k := range v.Cities[i].Products[j].History {
				v.Cities[i].Products[j].History[k] = -1
			}
		}
		v.Cities[i].Corners = nil
	}
	for city := range v.You.Stock {
		for p := range v.You.Stock[city] {
			v.You.Stock[city][p] = -1
		}
	}
	for i := range v.Houses {
		for p := range v.Houses[i].Stock {
			v.Houses[i].Stock[p] = -1
		}
	}
	for _, sec := range v.Report.Sections {
		for i := range sec.Lines {
			sec.Lines[i] = "changed"
		}
	}
	for i := range v.Report.Lead {
		v.Report.Lead[i].Text = "changed"
	}
	after, _ := json.Marshal(w)
	if string(before) != string(after) {
		t.Fatal("changing the view changed the world")
	}
}

// TestViewReadsTheFile (#299): a faction's temper and heads and the
// chief's temper are what the intel file holds (game.Known), on a run
// whose file has facts in it; and view.go reads none of the truths the
// panels never read (TestPanelsReadTheFile's list, with who is talking).
func TestViewReadsTheFile(t *testing.T) {
	t.Parallel()
	s, w := playedSession(t, 60)
	v := s.View()
	known := game.Known(w)
	facts := 0
	for _, f := range v.Factions {
		if want := known.Personality(f.ID); f.Personality != want {
			t.Errorf("%s: personality %q, the file says %q", f.ID, f.Personality, want)
		}
		if f.Personality != game.Unknown {
			facts++
		}
		lo, hi, _, ok := known.Muscle(f.ID)
		if f.MuscleKnown != ok || f.MuscleLo != lo || f.MuscleHi != hi {
			t.Errorf("%s: muscle %d..%d (%v), the file says %d..%d (%v)", f.ID, f.MuscleLo, f.MuscleHi, f.MuscleKnown, lo, hi, ok)
		}
	}
	if v.Law.ChiefTemper != known.Chief() {
		t.Errorf("the chief's temper %q, the file says %q", v.Law.ChiefTemper, known.Chief())
	}
	t.Logf("%d of %d factions' tempers known on day %d", facts, len(v.Factions), w.Day)

	forbidden := map[string][]string{
		"RivalState":  {"Personality", "Muscle", "Cash", "Grudge", "Arrears", "Eyeing"},
		"Chief":       {"Personality"},
		"RouteConfig": {"Risk"},
		"Fact":        {"Planted"},
		"CrewMember":  {"Informant", "Greed", "Nerve"},
		"HeatState":   {"Leaks", "LeakDay"},
	}
	fset := token.NewFileSet()
	names, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	var files []*ast.File
	var view *ast.File
	for _, name := range names {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, f)
		if name == "view.go" {
			view = f
		}
	}
	info := &types.Info{Selections: map[*ast.SelectorExpr]*types.Selection{}}
	conf := types.Config{Importer: importer.ForCompiler(fset, "source", nil)}
	if _, err := conf.Check("engine", fset, files, info); err != nil {
		t.Fatal(err)
	}
	var hits []string
	ast.Inspect(view, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		s := info.Selections[sel]
		if s == nil {
			return true
		}
		recv := s.Recv()
		if p, ok := recv.(*types.Pointer); ok {
			recv = p.Elem()
		}
		named, ok := recv.(*types.Named)
		if !ok {
			return true
		}
		for _, f := range forbidden[named.Obj().Name()] {
			if s.Obj().Name() == f {
				hits = append(hits, fmt.Sprintf("%s: %s.%s", fset.Position(sel.Pos()), named.Obj().Name(), f))
			}
		}
		return true
	})
	sort.Strings(hits)
	if len(hits) > 0 {
		t.Errorf("the view reads the truth; read game.Known(w):\n  %s", strings.Join(hits, "\n  "))
	}
}

// TestViewHasNoNull (#333): no list or map in the view is ever null,
// before a run, on day 0 and forty days in, so a client need not guard
// one. A pointer that is absent (over, card, books) is left out, never
// null.
func TestViewHasNoNull(t *testing.T) {
	t.Parallel()
	fresh, err := engine.New(content.MustLoad())
	if err != nil {
		t.Fatal(err)
	}
	before := fresh.View()
	fresh.NewRun(7, game.Start{})
	day0 := fresh.View()
	s, _ := playedSession(t, 40)
	for name, v := range map[string]engine.View{"before a run": before, "day 0": day0, "day 40": s.View()} {
		raw, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		var doc any
		if err := json.Unmarshal(raw, &doc); err != nil {
			t.Fatal(err)
		}
		var nulls []string
		var walk func(path string, x any)
		walk = func(path string, x any) {
			switch x := x.(type) {
			case nil:
				nulls = append(nulls, path)
			case map[string]any:
				for k, e := range x {
					walk(path+"."+k, e)
				}
			case []any:
				for _, e := range x {
					walk(path+"[]", e)
				}
			}
		}
		walk("", doc)
		if len(nulls) > 0 {
			sort.Strings(nulls)
			t.Errorf("the view %s has null at %v", name, nulls)
		}
	}
	for _, list := range []string{`"houses":[]`, `"shipments":[]`, `"alerts":[]`, `"crew":[]`} {
		if raw, _ := json.Marshal(before); !strings.Contains(string(raw), list) {
			t.Errorf("the view before a run does not say %s", list)
		}
	}
}

// TestViewCarriesWhatTheScreensList (#332): what the TUI's screens list
// to act on is in the view, by the id the command takes: the pool the
// crew screen hires from, the buyers' contracts, the factions' offers
// and the upgrade tree with where each node stands. A front end that
// sees only the view hires, and buys an upgrade, by those ids.
func TestViewCarriesWhatTheScreensList(t *testing.T) {
	t.Parallel()
	s, w := playedSession(t, 60)
	v := s.View()

	if len(v.Pool) != len(w.Crew.Candidates) || len(v.Pool) == 0 {
		t.Fatalf("the view's pool has %d, the world's %d", len(v.Pool), len(w.Crew.Candidates))
	}
	for i, c := range v.Pool {
		m := w.Crew.Candidates[i]
		if c.ID != m.ID || c.Fee != m.Fee || c.Carry != m.Units || c.Personality != "" {
			t.Errorf("pool %d: %+v, the candidate %+v", i, c, m)
		}
	}
	live := 0
	for _, c := range w.Contracts {
		if !c.Done() {
			live++
		}
	}
	if len(v.Contracts) != live {
		t.Errorf("the view has %d contracts, the world %d live", len(v.Contracts), live)
	}
	for _, c := range v.Contracts {
		if wc := w.Contract(c.ID); wc == nil || wc.Status.String() != c.Status || wc.Units != c.Units {
			t.Errorf("contract %d: %+v, the world's %+v", c.ID, c, wc)
		}
	}
	if len(v.Offers) != len(w.Offers) {
		t.Errorf("the view has %d offers, the world %d", len(v.Offers), len(w.Offers))
	}
	for _, o := range v.Offers {
		if wo := w.Offer(o.ID); wo == nil || wo.Deal.Kind != o.Kind || wo.With() != o.Faction {
			t.Errorf("offer %d: %+v, the world's %+v", o.ID, o, wo)
		}
	}
	tree := content.MustLoad().Upgrades.Nodes
	if len(v.Upgrades) != len(tree) {
		t.Fatalf("the view has %d upgrades, the tree %d", len(v.Upgrades), len(tree))
	}
	for i, u := range v.Upgrades {
		want := "locked"
		if w.Owns(u.ID) {
			want = "owned"
		} else if len(w.Missing(tree[i])) == 0 {
			want = "available"
		}
		if u.ID != tree[i].ID || u.State != want {
			t.Errorf("upgrade %s: %s, want %s", u.ID, u.State, want)
		}
	}

	// Sixty days in, the informed player's crew is as big as it can
	// manage: the hire and the buy are a fresh run's.
	fresh, err := engine.New(content.MustLoad())
	if err != nil {
		t.Fatal(err)
	}
	fw := fresh.NewRun(7, game.Start{})
	fw.Player.DirtyCash += 10_000_000
	v = fresh.View()
	if len(v.Pool) == 0 {
		t.Fatal("nobody looking for work on day 0")
	}
	hired := v.Pool[0].ID
	if _, err := fresh.Hire(hired); err != nil {
		t.Fatalf("hiring pool[0] by the view's id: %v", err)
	}
	var buy *engine.UpgradeView
	for i, u := range v.Upgrades {
		if buy == nil && u.State == "available" && !u.Clean {
			buy = &v.Upgrades[i]
		}
	}
	if buy == nil {
		t.Fatal("no dirty-cash upgrade available to buy on day 0")
	}
	if _, err := fresh.BuyUpgrade(buy.ID); err != nil {
		t.Fatalf("buying %s by the view's id: %v", buy.ID, err)
	}
	after := fresh.View()
	onPayroll := false
	for _, m := range after.Crew {
		onPayroll = onPayroll || m.ID == hired
	}
	for _, c := range after.Pool {
		if c.ID == hired {
			t.Error("the hired candidate is still in the pool")
		}
	}
	if !onPayroll {
		t.Error("the hired candidate is not on the payroll")
	}
	for _, u := range after.Upgrades {
		if u.ID == buy.ID && u.State != "owned" {
			t.Errorf("%s bought and %s", u.ID, u.State)
		}
	}
}

// TestViewCarriesThePolice (#355): every city's ladder is the heat
// sim's Rungs, rung for rung; the law carries the arrest line, the
// exposure line and the cover the sim charges against; and the police's
// next move is the file's, with how sure it is today.
func TestViewCarriesThePolice(t *testing.T) {
	t.Parallel()
	s, w := playedSession(t, 60)
	if w.Over == nil && w.Player.DirtyCash > 5000 {
		// A cop's word where you stand, so the forecast is in the view.
		if err := s.PayCop(5000); err != nil {
			t.Fatal(err)
		}
		s.EndDay()
	}
	v := s.View()
	heat := s.Rules().Heat
	known := game.Known(w)
	words := 0
	for _, c := range v.Cities {
		rungs := heat.Rungs(w, w.City(c.ID))
		if len(c.Ladder) != len(rungs) || len(rungs) == 0 {
			t.Fatalf("%s: %d rungs in the view, %d in the sim", c.ID, len(c.Ladder), len(rungs))
		}
		for i, r := range rungs {
			got := c.Ladder[i]
			if got.Level != r.Level || got.Line != r.Threshold || got.StockLoss != r.StockLoss || got.CashLoss != r.CashLoss || got.Pages != r.Evidence || got.Cap != r.Cap || got.CapDays != r.CapDays {
				t.Errorf("%s rung %d: %+v, the sim's %+v", c.ID, i, got, r)
			}
		}
		if f, ok := known.Fact(c.ID, game.FactResponse); ok {
			words++
			if c.Response != f.Value || c.Sure != f.Now(w.Day) {
				t.Errorf("%s: the word %q at %.2f, the file's %q at %.2f", c.ID, c.Response, c.Sure, f.Value, f.Now(w.Day))
			}
		} else if c.Response != "" || c.Sure != 0 {
			t.Errorf("%s: a word %q at %.2f with nothing in the file", c.ID, c.Response, c.Sure)
		}
	}
	if v.Law.ArrestLine != heat.EvidenceArrest(w) || v.Law.ExposureLine != heat.ExposureLine(w) || v.Law.Cover != heat.Cover(w) {
		t.Errorf("the law: arrest %d, exposure %d, cover %d; the sim's %d, %d, %d", v.Law.ArrestLine, v.Law.ExposureLine, v.Law.Cover, heat.EvidenceArrest(w), heat.ExposureLine(w), heat.Cover(w))
	}
	if words == 0 {
		t.Errorf("day %d: no city carries a cop's word, and one was paid", w.Day)
	}
}
