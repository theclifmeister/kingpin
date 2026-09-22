package engine_test

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/engine"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/harness"
	"github.com/theclifmeister/kingpin/internal/sim"
)

const days = 60

// handRun is the run a front end assembled itself before the session
// (#296): sim.Default, a clock with no bus, the world stepped day by
// day under the policy.
func handRun(t *testing.T, cfg *content.Config, seed uint64, policy harness.Policy) (*game.World, []events.Event) {
	t.Helper()
	_, sims, err := sim.Default(cfg)
	if err != nil {
		t.Fatal(err)
	}
	clock := game.NewClock(nil, sims...)
	w := sim.NewWorld(cfg, seed)
	var evs []events.Event
	for d := 0; d < days && w.Over == nil; d++ {
		policy(w)
		evs = append(evs, clock.EndDay(w)...)
	}
	return w, evs
}

// sameRun fails unless the two runs are the same run: every event and
// the world they end on, byte for byte.
func sameRun(t *testing.T, wa, wb *game.World, ea, eb []events.Event) {
	t.Helper()
	if len(ea) != len(eb) {
		t.Fatalf("event counts differ: %d vs %d", len(ea), len(eb))
	}
	for i := range ea {
		if fmt.Sprintf("%#v", ea[i]) != fmt.Sprintf("%#v", eb[i]) {
			t.Fatalf("event %d differs:\n%#v\n%#v", i, ea[i], eb[i])
		}
	}
	ja, err := json.Marshal(wa)
	if err != nil {
		t.Fatal(err)
	}
	jb, err := json.Marshal(wb)
	if err != nil {
		t.Fatal(err)
	}
	if string(ja) != string(jb) {
		t.Fatal("the worlds differ at the end of the run")
	}
}

// TestSessionIsTheHandAssembledRun (#296): a run through the session is
// the run a hand-assembled clock plays on the same seed, and what the
// session publishes is what EndDay returns.
func TestSessionIsTheHandAssembledRun(t *testing.T) {
	t.Parallel()
	cfg := content.MustLoad()
	policy := harness.Trader(cfg, events.DialNormal)
	want, wantEvs := handRun(t, cfg, 7, policy)

	s, err := engine.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	var published []events.Event
	s.Subscribe(func(e events.Event) { published = append(published, e) })
	w := s.NewRun(7, game.Start{})
	var got []events.Event
	for d := 0; d < days && w.Over == nil; d++ {
		policy(w)
		got = append(got, s.EndDay()...)
	}
	if s.World() != w {
		t.Fatal("the session's world is not the run it started")
	}
	sameRun(t, want, w, wantEvs, got)
	sameRun(t, w, w, got, published)
}

// TestSessionSavesAndResumes (#296): a run saved through the session
// and loaded into a fresh one plays on as the run that never stopped.
func TestSessionSavesAndResumes(t *testing.T) {
	t.Setenv("KINGPIN_HOME", t.TempDir())
	cfg := content.MustLoad()
	policy := harness.Trader(cfg, events.DialNormal)
	want, wantEvs := handRun(t, cfg, 11, policy)

	a, err := engine.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Save(1); err != engine.ErrNoRun {
		t.Fatalf("a save before a run: %v, want ErrNoRun", err)
	}
	if evs := a.EndDay(); evs != nil {
		t.Fatalf("a day before a run: %d events", len(evs))
	}
	w := a.NewRun(11, game.Start{})
	var got []events.Event
	half := days / 2
	for d := 0; d < half; d++ {
		policy(w)
		got = append(got, a.EndDay()...)
	}
	if err := a.Save(2); err != nil {
		t.Fatal(err)
	}

	b, err := engine.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.Load(3); err == nil {
		t.Fatal("an empty slot loaded")
	}
	if b.World() != nil {
		t.Fatal("a failed load left a run on the session")
	}
	w, err = b.Load(2)
	if err != nil {
		t.Fatal(err)
	}
	if w.Day != half {
		t.Fatalf("loaded day %d, want %d", w.Day, half)
	}
	for d := half; d < days && w.Over == nil; d++ {
		policy(w)
		got = append(got, b.EndDay()...)
	}
	sameRun(t, want, w, wantEvs, got)
}

// TestOneAssemblyPath (#296): outside the engine and the sims no
// program assembles a run itself. A front end that calls sim.Default or
// game.NewClock has a second step order and a second migration chain
// to keep right; it takes an engine.Session instead. Tests are exempt:
// they pin the pieces the session is built from.
func TestOneAssemblyPath(t *testing.T) {
	t.Parallel()
	root := filepath.Join("..", "..")
	allowed := []string{filepath.Join("internal", "engine"), filepath.Join("internal", "sim")}
	banned := map[string]bool{"sim.Default": true, "game.NewClock": true}
	fset := token.NewFileSet()
	var offenders []string
	walked := 0
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		for _, a := range allowed {
			if strings.HasPrefix(rel, a+string(filepath.Separator)) {
				return nil
			}
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
			if id, ok := sel.X.(*ast.Ident); ok && banned[id.Name+"."+sel.Sel.Name] {
				offenders = append(offenders, fset.Position(sel.Pos()).String()+": "+id.Name+"."+sel.Sel.Name+"; assemble a run through engine.New (#296)")
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if walked < 100 {
		t.Fatalf("only %d files walked; the root is wrong", walked)
	}
	if len(offenders) > 0 {
		t.Errorf("a run is assembled outside the engine:\n  %s", strings.Join(offenders, "\n  "))
	}
}
