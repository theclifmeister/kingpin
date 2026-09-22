package ui

import (
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/theclifmeister/kingpin/internal/game"
)

// fillIntel files n facts of every kind and source on the world (#45),
// the newest first, so a screen has a full table to show.
func fillIntel(w *game.World, n int) {
	kinds := []string{game.FactPersonality, game.FactMuscle, game.FactCash, game.FactIncome, game.FactWages, game.FactMove, game.FactStash, game.FactResponse, game.FactRisk}
	sources := []string{game.SourceSeen, game.SourceBooks, game.SourceCop, game.SourceSpy, game.SourceContact}
	home := w.Home()
	for i := 0; i < n; i++ {
		kind := kinds[i%len(kinds)]
		f := game.Fact{Kind: kind, Confidence: 1 - float64(i%7)*0.1, Day: max(0, w.Day-i%4), Source: sources[i%len(sources)], Stale: 0.05, Forget: 0.2}
		switch kind {
		case game.FactResponse:
			f.Subject, f.Value, f.Number = fmt.Sprintf("city%d", i), "sting", float64(w.Day+1+i%5)
		case game.FactRisk:
			f.Subject, f.Value, f.Number = fmt.Sprintf("route%d", i), "~2%/day", 0.02
		case game.FactMove, game.FactStash:
			f.Subject, f.Value = fmt.Sprintf("f%d", 1+i%3), home.Corners[i%len(home.Corners)].ID
		case game.FactPersonality:
			if i == 0 {
				f.Subject, f.Value = game.SubjectChief, "zealous"
			} else {
				f.Subject, f.Value = fmt.Sprintf("f%d", 1+i%3), "defensive"
			}
		case game.FactMuscle:
			f.Subject, f.Value, f.Number = fmt.Sprintf("f%d", 1+i%3), "3–5", 4
		default:
			f.Subject, f.Value, f.Number = fmt.Sprintf("f%d", 1+i%3), fmt.Sprint(1000*(i+1)), float64(1000*(i+1))
		}
		if i%len(sources) == 4 {
			f.Planted = "f2"
		}
		// A distinct key per fact: the subject carries the index.
		if kind != game.FactResponse && kind != game.FactRisk && f.Subject != game.SubjectChief {
			f.Subject += fmt.Sprintf("-%d", i)
		}
		w.Learn(f)
	}
}

// The intel screen (#45): 9 opens it, the table lists the file's live
// facts newest first with the confidence as a percent, the pane carries
// the fact under the cursor with its bar and its story, `$` opens the
// cop dialog (blank the price; the money is gone at once, one a day)
// and `p` the spy dialog (the faction, then who; a spy under stands on
// no corner and sells nothing), `i` on the map's rival corner and on
// the dashboard jump here, and the rivals table's muscle column reads
// what the file holds.
func TestIntelKeys(t *testing.T) {
	m := richModel(t, 120, 40)
	w := m.w
	m.Update(key("9"))
	if m.screen != screenIntel {
		t.Fatalf("9 opened screen %d", m.screen)
	}
	main := mainText(m)
	for _, want := range []string{"INTEL · 4 facts", "sting from d", "a cop", "a contact says", "3–5 heads", "SPIES", "Lena", "next in 3d"} {
		if !strings.Contains(main, want) {
			t.Errorf("MAIN lacks %q:\n%s", want, main)
		}
	}
	pane := paneText(m)
	for _, want := range []string{"POLICE", "sure", "%", "a cop", "cooldown", "$  pay a cop", "p  plant a spy"} {
		if !strings.Contains(pane, want) {
			t.Errorf("the pane lacks %q:\n%s", want, pane)
		}
	}
	// The cursor walks the file; the pane follows.
	m.Update(key("j"))
	if pane := paneText(m); !strings.Contains(pane, "a contact says") || !strings.Contains(pane, "Nobody vouches") {
		t.Errorf("the pane did not follow the cursor to the lure:\n%s", pane)
	}
	// The cop: blank is the price, the money is gone at once, and a
	// second cop is refused.
	tun := m.cfg.Intel.Intel
	dirty := w.Player.DirtyCash
	m.Update(key("$"))
	if m.mode != modePayCop {
		t.Fatalf("$ on the intel screen: mode %v status %q", m.mode, m.status)
	}
	assertFits(t, m.View(), 120, 40, "the cop dialog")
	m.Update(key("enter"))
	if m.mode != modePlay || w.Today.Cop == nil || w.Today.Cop.Amount != tun.CopPrice || w.Player.DirtyCash != dirty-tun.CopPrice || w.Stats.CopsPaid != 1 {
		t.Fatalf("enter on the cop dialog: mode %v cop %+v dirty %d -> %d", m.mode, w.Today.Cop, dirty, w.Player.DirtyCash)
	}
	if !strings.Contains(m.status, "A cop takes") {
		t.Errorf("status %q", m.status)
	}
	m.Update(key("$"))
	if m.mode != modePlay || !strings.HasPrefix(m.status, "Can't pay a cop") {
		t.Fatalf("a second $: mode %v status %q", m.mode, m.status)
	}
	// The spy: one faction on the ground opens the dialog on the crew;
	// enter sends the one under the cursor, who is then no longer at
	// work: Post refuses them the morning after.
	m.Update(key("p"))
	if m.mode != modeSpy || m.spy.step != 1 {
		t.Fatalf("p on the intel screen: mode %v step %d status %q", m.mode, m.spy.step, m.status)
	}
	assertFits(t, m.View(), 120, 40, "the spy dialog")
	m.Update(key("enter"))
	if m.mode != modePlay || w.Today.Spy == nil || w.Today.Spy.Member != 1 {
		t.Fatalf("enter on the spy dialog: mode %v spy %+v status %q", m.mode, w.Today.Spy, m.status)
	}
	if main := mainText(m); !strings.Contains(main, "Tonight  Dre goes under") {
		t.Errorf("MAIN does not say who goes under tonight:\n%s", main)
	}
	m.Update(key("p"))
	if m.mode != modePlay || !strings.HasPrefix(m.status, "Can't plant another") {
		t.Fatalf("a second p: mode %v status %q", m.mode, m.status)
	}
	endDay(t, m)
	m.Update(key("enter"))
	dre := w.Crew.Member(1)
	if dre == nil || dre.Undercover != w.Rival().Faction() || dre.Working() || w.PostOf(1) != nil {
		t.Fatalf("Dre the morning after: %+v post %v", dre, w.PostOf(1))
	}
	if err := w.Post(w.Home().Corners[1].ID, 1); err != game.ErrUndercover {
		t.Fatalf("posting a spy: %v", err)
	}
	report := strings.Join(w.Report.Intel, "\n")
	for _, want := range []string{"Dre went under", "The cop says"} {
		if !strings.Contains(report, want) {
			t.Errorf("the report's INTEL lacks %q:\n%s", want, report)
		}
	}
	m.Update(key("r"))
	if view := stripANSI(m.View()); m.mode != modeReport || !strings.Contains(view, "INTEL") || !strings.Contains(view, "The cop says") {
		t.Errorf("the report modal lacks the INTEL section (mode %v):\n%s", m.mode, view)
	}
	m.Update(key("esc"))
	// i on the map with the rival's corner under the cursor jumps to the
	// faction's facts; on the dashboard to the chief's (none yet: the
	// status says so); on a corner of yours it is silent.
	m.Update(key("5"))
	m.mapCursor = 0
	m.Update(key("i"))
	if m.screen != screenIntel || m.intelSelected() == nil || m.intelSelected().Subject != w.Rival().Faction() {
		t.Fatalf("i on the rival's corner: screen %d fact %+v", m.screen, m.intelSelected())
	}
	m.Update(key("1"))
	m.Update(key("i"))
	onChief := m.intelSelected() != nil && m.intelSelected().Subject == game.SubjectChief
	if m.screen != screenIntel || (!onChief && !strings.Contains(m.status, "Nothing is known about the chief")) {
		t.Fatalf("i on the dashboard: screen %d status %q fact %+v", m.screen, m.status, m.intelSelected())
	}
	m.Update(key("5"))
	m.mapCursor = 1
	m.status = ""
	m.Update(key("i"))
	if m.screen != screenMap || m.status != "" {
		t.Fatalf("i on your own corner: screen %d status %q", m.screen, m.status)
	}
	// The rivals table and the strike picker read the file: the band
	// (the fixture's, or a fresher one the night's push wrote), and
	// after the fact fades, a question mark and blind odds.
	m.Update(key("8"))
	if main := mainText(m); !strings.Contains(main, "muscle "+m.known().MuscleWord(w.Rival().Faction())) || strings.Contains(main, "muscle ?") {
		t.Errorf("the rivals screen does not read the band:\n%s", main)
	}
	w.Unlearn(w.Rival().Faction(), game.FactMuscle)
	if main := mainText(m); !strings.Contains(main, "muscle ?") {
		t.Errorf("the rivals screen reads a muscle the file does not hold:\n%s", main)
	}
	m.Update(key("5"))
	m.mapCursor = 0
	m.Update(key("w"))
	if view := stripANSI(m.View()); !strings.Contains(view, "odds read blind") || !strings.Contains(view, "muscle ?") {
		t.Errorf("the strike picker with the muscle unknown:\n%s", view)
	}
	m.Update(key("esc"))
}

// The intel screen renders with thirty facts at 80x24, 100x30 and
// 120x40, the table under the cursor at the top and the bottom of the
// file, and every fact's confidence reads in 0..100.
func TestIntelScreenFits(t *testing.T) {
	for _, sz := range [][2]int{{80, 24}, {100, 30}, {120, 40}} {
		m := newTestModel(t, sz[0], sz[1])
		fillIntel(m.w, 30)
		m.Update(key("9"))
		facts := m.known().Facts()
		if len(facts) != 30 {
			t.Fatalf("%d facts filed, want 30", len(facts))
		}
		for _, f := range facts {
			if c := f.Now(m.w.Day); c < 0 || c > 1 {
				t.Fatalf("confidence %v on %+v", c, f)
			}
		}
		for _, cursor := range []int{0, 15, 29} {
			m.intelCursor = cursor
			view := m.View()
			assertFits(t, view, sz[0], sz[1], fmt.Sprintf("intel at cursor %d", cursor))
			if m.mode == modePlay && sz[0] >= paneMinWidth {
				assertFrame(t, m, "intel")
			}
			if !strings.Contains(stripANSI(view), "INTEL · 30 facts") {
				t.Errorf("%dx%d: no count:\n%s", sz[0], sz[1], stripANSI(view))
			}
		}
		if sz[0] < paneMinWidth {
			m.Update(key(" "))
			if m.mode != modeDetails {
				t.Fatalf("%dx%d: space did not open the overlay", sz[0], sz[1])
			}
			assertFits(t, m.View(), sz[0], sz[1], "intel overlay")
			m.Update(key("esc"))
		}
		m.Update(tea.WindowSizeMsg{Width: sz[0], Height: sz[1]})
	}
}

// TestPanelsReadTheFile (#45): the UI reads what the player knows,
// never the truth. No file in internal/ui but the demo's fixture
// (demo.go builds a world by hand) selects RivalState.Personality,
// RivalState.Muscle, Chief.Personality, RouteConfig.Risk or Fact.Planted,
// or calls the logistics sim's Risk or DayRisk (RiskFrom, over the risk
// the file holds, is the fold the panels use) or the rivals sim's Odds,
// OddsOn, PushOdds or Defence (OddsOnAt, PushOddsAt and DefenceAt take
// the muscle the file holds). The check type-checks
// the package from source (go/types with the source importer) so a
// selector is read by its receiver's type, not its spelling: a corner's
// Risk and a lieutenant's Personality are not the truth this guards.
func TestPanelsReadTheFile(t *testing.T) {
	fset := token.NewFileSet()
	// One file at a time, not parser.ParseDir: staticcheck flags that as
	// deprecated since Go 1.25 (SA1019, #276), and the package's files
	// are just the directory's non-test .go files.
	names, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	var files []*ast.File
	for _, name := range names {
		if strings.HasSuffix(name, "_test.go") || name == "demo.go" {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, f)
	}
	if len(files) < 20 {
		t.Fatalf("only %d files parsed; the glob is wrong", len(files))
	}
	conf := types.Config{Importer: importer.ForCompiler(fset, "source", nil)}
	info := &types.Info{Selections: map[*ast.SelectorExpr]*types.Selection{}}
	if _, err := conf.Check("ui", fset, files, info); err != nil {
		t.Fatal(err)
	}
	forbidden := map[string][]string{ // the named type -> the fields and methods the panels never read off it
		"RivalState":  {"Personality", "Muscle"},
		"Chief":       {"Personality"},
		"RouteConfig": {"Risk"},
		"Fact":        {"Planted"},
		"Sim":         {"Risk", "DayRisk", "Odds", "OddsOn", "PushOdds", "Defence"}, // the logistics sim's folds of the truth, and the rivals sim's (OddsOnAt, PushOddsAt, DefenceAt take the muscle the file holds)
	}
	named := func(typ types.Type) string {
		if p, ok := typ.(*types.Pointer); ok {
			typ = p.Elem()
		}
		if n, ok := typ.(*types.Named); ok {
			return n.Obj().Name()
		}
		return ""
	}
	var hits []string
	for sel, s := range info.Selections {
		tn := named(s.Recv())
		for _, name := range forbidden[tn] {
			if s.Obj().Name() != name {
				continue
			}
			if tn == "Sim" && !strings.Contains(s.Recv().String(), "/logistics.") && !strings.Contains(s.Recv().String(), "/rivals.") {
				continue
			}
			pos := fset.Position(sel.Pos())
			hits = append(hits, fmt.Sprintf("%s:%d: %s.%s", filepath.Base(pos.Filename), pos.Line, tn, name))
		}
	}
	if len(hits) > 0 {
		t.Errorf("the panels read the truth; read game.Known(w) instead:\n  %s", strings.Join(hits, "\n  "))
	}
}
