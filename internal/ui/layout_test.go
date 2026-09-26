package ui

import (
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/game"
)

// The layout and start nits of #507: the columns and lines cut at 80
// and 100, the overlay's narrow wrap, the map's arrows off the routes,
// the patrol's city, the kin mark, the first launch, q and the slot's
// age. TestRendersAtCommonSizes walks the late fixture below for the
// tables (unreadable).

// lateModel is the rich fixture late in a run (#507): the cartel tier,
// every product on the market, every front, an asset, a trophy and a
// deed bought with clean money, a lieutenant running the second city
// and the fat numbers that come with it, so the tables and the lines
// written at the corner's scale are drawn at the cartel's.
func lateModel(t *testing.T, w, h int) *Model {
	t.Helper()
	m := richModel(t, w, h)
	world := m.w
	world.Reach(5, world.Day)
	world.SeeStage(5)
	world.Stats.PeakCash, world.Stats.PeakClean = 90_000_000, 60_000_000
	world.Player.DirtyCash, world.Player.CleanCash = 12_400_000, 48_000_000
	for _, o := range m.frontRows() {
		if _, err := world.BuyFront(o); err != nil {
			t.Fatalf("buying %s: %v", o.Name, err)
		}
	}
	if offers := m.assetRows(); len(offers) > 0 {
		if _, err := m.sess.BuyAsset(offers[0].ID); err != nil {
			t.Fatalf("buying %s: %v", offers[0].Name, err)
		}
	}
	if offers := m.trophyRows(); len(offers) > 0 {
		if _, err := world.BuyTrophy(offers[0]); err != nil {
			t.Fatalf("buying %s: %v", offers[0].Name, err)
		}
	}
	if err := m.sess.BuyDeed(world.Home().Corners[1].ID); err != nil {
		t.Fatalf("buying the block: %v", err)
	}
	if err := world.Assign(4, world.CityOrder[1]); err != nil {
		t.Fatalf("assigning the lieutenant: %v", err)
	}
	world.Stats.Laundered, world.Stats.Shipped = 38_000_000, 184_000
	endDay(t, m)
	closeMorning(t, m)
	world.Player.DirtyCash = 12_400_000
	m.Update(key("1"))
	m.cursor, m.crewCursor, m.mapCursor, m.ledgerCursor = 0, 0, 0, 0
	m.status = ""
	if m.w.Over != nil {
		t.Fatalf("the late fixture ended: %v", m.w.Over)
	}
	return m
}

// The space overlay writes the details for its own width (#507): the
// Laundromat's words ran 32 cells wide in a 76-cell box at 80x24 and
// 100x30; the pane beside MAIN keeps its wrap.
func TestSpaceWrapsToTheBox(t *testing.T) {
	const words = "A laundromat: quarters, soap and no questions."
	for _, sz := range [][2]int{{80, 24}, {100, 30}} {
		m := richModel(t, sz[0], sz[1])
		m.Update(key("7"))
		if f := m.ledgerSelected(); f.kind != ledgerFront || m.w.Fronts[f.i].ID != "laundromat" {
			t.Fatalf("%dx%d: the ledger opens on %+v", sz[0], sz[1], f)
		}
		if sz[0] >= paneMinWidth && strings.Contains(paneText(m), words) {
			t.Errorf("%dx%d: the pane holds the words on one line of 32 cells:\n%s", sz[0], sz[1], paneText(m))
		}
		m.Update(key(" "))
		view := stripANSI(m.View())
		if !strings.Contains(view, words) {
			t.Errorf("%dx%d: the overlay keeps the pane's wrap:\n%s", sz[0], sz[1], view)
		}
		assertFits(t, m.View(), sz[0], sz[1], "the overlay")
	}
}

// The ledger's labelled lines are whole at 80 and beside the pane
// (#507): the launder dial keeps its row and its numbers go under it,
// and the wash's idle line wraps under its value.
func TestLedgerLinesAreWhole(t *testing.T) {
	for _, sz := range [][2]int{{80, 24}, {100, 30}} {
		m := lateModel(t, sz[0], sz[1])
		m.Update(key("7"))
		main := mainText(m)
		for _, want := range []string{"launder  careful  [normal]  greedy", "audit ", "up to ", "legit "} {
			if !strings.Contains(main, want) {
				t.Errorf("%dx%d: the ledger lacks %q:\n%s", sz[0], sz[1], want, main)
			}
		}
		// The wash's idle line, the pile under the till.
		m.w.Player.DirtyCash = 1_000
		prose := strings.Join(strings.Fields(mainText(m)), " ")
		if want := "the wash takes only what is over it"; !strings.Contains(prose, want) {
			t.Errorf("%dx%d: the wash line is cut:\n%s", sz[0], sz[1], mainText(m))
		}
		for _, l := range strings.Split(mainText(m), "\n") {
			if strings.Contains(l, "…") && (strings.Contains(l, "launder") || strings.Contains(l, "legit") || strings.Contains(l, "wash ")) {
				t.Errorf("%dx%d: a cut line: %q", sz[0], sz[1], l)
			}
		}
	}
}

// A report line's aside stays on one row (#507: the sale line broke as
// "(normal, standing, cut" with "$269)" alone under it at 80x24).
func TestReportKeepsItsAside(t *testing.T) {
	aside := regexp.MustCompile(`\(normal, standing, cut \$[\d,]+\)`)
	m := lateModel(t, 80, 24)
	endDay(t, m)
	for i := 0; i < 30; i++ {
		if aside.MatchString(stripANSI(m.View())) {
			return
		}
		m.Update(key("down"))
	}
	t.Fatalf("no row holds the sale's aside whole:\n%s", scrolledProse(t, m))
}

// THE STORY hangs every wrapped line under its text, whatever the day
// (#507: day 100 had one space after it and hung two cells in).
func TestStoryHangsUnderItsText(t *testing.T) {
	m := richModel(t, 80, 24)
	long := "The Projects 'spoken for', say corner boys; Mother's crew expected before the week is out"
	m.w.Journal = []game.Headline{{Day: 7, Source: "rivals", Text: long + " (1)"}, {Day: 123, Source: "rivals", Text: long + " (2)"}}
	lines := m.storyLines()
	if len(lines) != 2 {
		t.Fatalf("the story: %q", lines)
	}
	at := -1
	for _, l := range lines {
		plain := stripANSI(l)
		text := strings.Index(plain, "The Projects")
		wrapped := wrapLine(l, 60)
		if len(wrapped) < 2 {
			t.Fatalf("the line did not wrap: %q", plain)
		}
		hang := len(stripANSI(wrapped[1])) - len(strings.TrimLeft(stripANSI(wrapped[1]), " "))
		if hang != text {
			t.Errorf("%q hangs at %d, its text is at %d", plain, hang, text)
		}
		if at >= 0 && text != at {
			t.Errorf("the text starts at %d on one day and %d on another", at, text)
		}
		at = text
	}
}

// The map's arrows off the routes (#507): ←→ stay on the list (they
// turned the city and dropped the cursor on the other city's grid) and
// ↑ off the first route lands on the grid's bottom row, not wherever
// the grid's cursor last sat.
func TestRouteArrowsStayWhereTheyAre(t *testing.T) {
	m := richModel(t, 120, 40)
	m.Update(key("5"))
	m.mapCursor = 0 // The Docks, top left
	m.onRoutes, m.routeCursor = true, 0
	city := m.city
	for _, k := range []string{"right", "left"} {
		m.Update(key(k))
		if !m.onRoutes || m.city != city {
			t.Fatalf("%s on the routes: on the routes %v, city %s", k, m.onRoutes, m.city)
		}
	}
	m.Update(key("up"))
	cs := m.shown().Corners
	bottom := 0
	for _, c := range cs {
		bottom = max(bottom, c.Y)
	}
	if m.onRoutes || cs[m.mapCursor].Y != bottom {
		t.Fatalf("up off the routes: on the routes %v, on %s in row %d, want row %d", m.onRoutes, cs[m.mapCursor].Name, cs[m.mapCursor].Y, bottom)
	}
	// From a corner in the bottom row, down and up come back to it.
	sel := m.mapCursor
	m.Update(key("down"))
	m.Update(key("up"))
	if m.mapCursor != sel {
		t.Errorf("down and up from %s came back to %s", cs[sel].Name, cs[m.mapCursor].Name)
	}
}

// The patrol's cap names the city whose patrol set it (#507), and a
// save from before, with no city, reads as it did.
func TestPatrolCapNamesItsCity(t *testing.T) {
	m := richModel(t, 80, 24)
	w := m.w
	w.Heat.SellCapDays, w.Heat.SellCap = 2, 0.59
	if got := patrolCapLine(w); got != "Patrols: sales capped at 59% of demand for 2 days more." {
		t.Errorf("with no city: %q", got)
	}
	w.Heat.SellCapCity = w.Home().ID
	if got, want := patrolCapLine(w), "Patrols in "+w.Home().Name+": sales capped at 59% of demand for 2 days more."; got != want {
		t.Errorf("the line is %q, want %q", got, want)
	}
	w.Heat.SellCapDays = 0
	if got := patrolCapLine(w); got != "" {
		t.Errorf("with no cap: %q", got)
	}
}

// The kin mark says what it is under the crew's tables (#507).
func TestKinMarkHasALegend(t *testing.T) {
	m := richModel(t, 120, 40)
	m.Update(key("4"))
	legend := kinGlyph + " has kin on the payroll or looking for work"
	if strings.Contains(mainText(m), legend) {
		t.Fatalf("a legend with no kin:\n%s", mainText(m))
	}
	a, b := &m.w.Crew.Members[0], &m.w.Crew.Members[1]
	a.Kin, b.Kin = []int{b.ID}, []int{a.ID}
	main := mainText(m)
	if !strings.Contains(main, a.Name+" "+kinGlyph) || !strings.Contains(main, legend) {
		t.Fatalf("the mark or its legend is missing:\n%s", main)
	}
}

// A contract taken fits the status bar at 80 columns (#507: "Deliver
// from the stash …"), the buyer left out where it does not.
func TestTakenFitsTheStatusBar(t *testing.T) {
	m := richModel(t, 80, 24)
	c := game.Contract{ID: 77, Name: "a gallery owner with a clean name", Product: m.w.Products[len(m.w.Products)-1], City: m.w.Player.Location, Units: 120, Since: m.w.Day, Expires: m.w.Day + 5, Due: m.w.Day + 20, Status: game.ContractOffered}
	m.w.Contracts = append(m.w.Contracts, c)
	m.Update(key("2"))
	for i := 0; i < 20 && !(m.onBuyers && m.selectedContract() != nil && m.selectedContract().ID == 77); i++ {
		m.Update(key("down"))
	}
	if !m.onBuyers || m.selectedContract() == nil || m.selectedContract().ID != 77 {
		t.Fatalf("the cursor never reached the offer: on the buyers %v", m.onBuyers)
	}
	m.Update(key("a"))
	if !strings.HasPrefix(m.status, "Taken: 120") || !strings.HasSuffix(m.status, "Deliver from the stash in "+m.w.CityName(c.City)+".") {
		t.Fatalf("the status: %q", m.status)
	}
	if foot := stripANSI(m.viewFooter()); strings.Contains(foot, "…") {
		t.Errorf("the status bar cuts the message: %q", foot)
	}
}

// The market turns its city with ←→ and [ ] from every region, and the
// README names both (#507: a tester read the README's ←→ and found ]).
func TestMarketCityKeysMatchTheReadme(t *testing.T) {
	m := richModel(t, 120, 40)
	m.Update(key("2"))
	for _, k := range []string{"right", "left", "]", "["} {
		city := m.city
		m.Update(key(k))
		if m.city == city {
			t.Errorf("%s did not turn the market from %s", k, city)
		}
	}
	b, err := os.ReadFile("../../README.md")
	if err != nil {
		t.Fatal(err)
	}
	prose := strings.Join(strings.Fields(string(b)), " ")
	if !strings.Contains(prose, "**Market** — ") || !strings.Contains(prose, "`←`/`→` or `[`/`]` shows the other city") {
		t.Error("the README's market does not name both keys")
	}
}

// The first launch opens the new-run dialog for slot 1 over the menu
// (#507: it started a Dealer run on a random seed), esc shows the menu,
// and enter on the dialog's pages starts the run in slot 1.
func TestFirstLaunchShowsTheMenu(t *testing.T) {
	t.Setenv("KINGPIN_HOME", t.TempDir())
	m, err := New(duel(), Options{Seeds: testSeeds(t)})
	if err != nil {
		t.Fatal(err)
	}
	m.width, m.height = 80, 24
	if m.w != nil || m.mode != modeNewRun || m.nr.slot != 1 {
		t.Fatalf("a fresh install: world %v, mode %v, slot %d", m.w != nil, m.mode, m.nr.slot)
	}
	view := stripANSI(m.View())
	for _, ch := range m.cfg.Characters.Characters {
		if !strings.Contains(view, ch.Name) {
			t.Errorf("the dialog does not list %s:\n%s", ch.Name, view)
		}
	}
	m.Update(key("esc"))
	if m.mode != modeStart || !strings.Contains(stripANSI(m.View()), "Slot 1 · empty") {
		t.Fatalf("esc: mode %v\n%s", m.mode, stripANSI(m.View()))
	}
	m.Update(key("enter"))
	for i := 0; i < 5 && m.mode == modeNewRun; i++ {
		m.Update(key("enter"))
	}
	if m.mode != modePlay || m.w == nil || m.slot != 1 {
		t.Fatalf("the dialog did not start a run in slot 1: mode %v slot %d", m.mode, m.slot)
	}
}

// The slot's age is when its run was last played (#507: a save an hour
// old read "saved just now" after the program quit from the menu,
// which wrote the run behind it again).
func TestSlotAgeIsTheLastSave(t *testing.T) {
	m := newTestModel(t, 80, 24)
	m.Update(key("q")) // saved, and the menu
	path, err := game.SavePath(1)
	if err != nil {
		t.Fatal(err)
	}
	hour := time.Now().Add(-time.Hour)
	if err := os.Chtimes(path, hour, hour); err != nil {
		t.Fatal(err)
	}
	if _, cmd := m.Update(key("q")); cmd == nil || !m.quitting {
		t.Fatalf("q on the menu did not quit")
	}
	st, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !st.ModTime().Equal(hour) {
		t.Errorf("quitting from the menu wrote the save: %v, was %v", st.ModTime(), hour)
	}
	if line := slotLine(game.Slots()[0], time.Now(), content.MustLoad().Endings.Title); !strings.HasSuffix(line, "saved 1h ago") {
		t.Errorf("the slot reads %q", line)
	}
}
