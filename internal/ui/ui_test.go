package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/sim"
)

func newTestModel(t *testing.T, w, h int) *Model {
	t.Helper()
	t.Setenv("KINGPIN_HOME", t.TempDir())
	m, err := New(content.MustLoad())
	if err != nil {
		t.Fatal(err)
	}
	m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	return m
}

func key(s string) tea.KeyMsg {
	if len(s) == 1 {
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
	switch s {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	case "left":
		return tea.KeyMsg{Type: tea.KeyLeft}
	case "right":
		return tea.KeyMsg{Type: tea.KeyRight}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func assertFits(t *testing.T, view string, w, h int, what string) {
	t.Helper()
	ls := strings.Split(view, "\n")
	if len(ls) > h {
		t.Errorf("%s: %d lines > height %d", what, len(ls), h)
	}
	for i, l := range ls {
		if lw := lipgloss.Width(l); lw > w {
			t.Errorf("%s: line %d is %d cells wide > %d: %q", what, i, lw, w, l)
		}
	}
}

func TestRendersAtCommonSizes(t *testing.T) {
	for _, sz := range [][2]int{{80, 24}, {120, 40}, {100, 30}} {
		m := newTestModel(t, sz[0], sz[1])
		// Play a few days with some trading so every panel has content.
		for i := 0; i < 5; i++ {
			m.w.Player.Stock[m.w.Products[0]] = 40
			m.Update(key("s"))
			m.Update(key("enter")) // product
			m.Update(key("enter")) // qty (blank = all)
			m.Update(key("3"))     // aggressive
			m.Update(key("enter")) // confirm
			m.Update(key("n"))     // end day -> report
			assertFits(t, m.View(), sz[0], sz[1], "report")
			m.Update(key("enter"))
		}
		// A full crew with a skim on record exercises every crew-screen line.
		m.Update(key("4"))
		m.w.Player.DirtyCash += 5000
		for i := 0; i < 6; i++ {
			m.Update(key("h"))
		}
		m.w.Crew.LastSkim = m.w.Day
		// Post the crew across the map so every cell shape is drawn.
		m.Update(key("5"))
		for i := range m.w.Territory.Corners {
			m.mapCursor = i
			m.Update(key("c"))
			assertFits(t, m.View(), sz[0], sz[1], "post picker")
			m.Update(key("j"))
			m.Update(key("enter"))
			m.Update(key("e"))
			m.Update(key("enter"))
		}
		// A rival in town, at war, with the enforcers queued against it,
		// exercises the rival cells, the picker and the dashboard panel.
		m.Update(key("esc")) // a stray enter above may be asking to end the day
		m.w.Territory.Corners[0].Owner, m.w.Territory.Corners[0].Runner, m.w.Territory.Corners[0].Enforcer = game.OwnerRival, 0, 0
		m.w.Rival.Arrived, m.w.Rival.Muscle, m.w.Rival.War, m.w.Rival.Observed = 1, 4, 47, true
		m.w.Crew.Members = append(m.w.Crew.Members, game.CrewMember{ID: 900, Name: "Moose", Role: "enforcer", Skill: 70, Loyalty: 70, Nerve: 60, Wage: 65})
		m.w.Crew.NextID = 900
		m.mapCursor = 0
		m.Update(key("w"))
		assertFits(t, m.View(), sz[0], sz[1], "strike picker")
		m.Update(key("enter"))
		if m.w.Strike == nil {
			t.Fatalf("%dx%d: no strike queued: %q", sz[0], sz[1], m.status)
		}
		for i := range m.w.Territory.Corners {
			m.mapCursor = i
			assertFits(t, m.View(), sz[0], sz[1], "map")
		}
		for _, s := range []string{"1", "2", "3", "4", "5"} {
			m.Update(key(s))
			assertFits(t, m.View(), sz[0], sz[1], "screen "+s)
		}
		m.Update(key("4"))
		m.Update(key("f"))
		assertFits(t, m.View(), sz[0], sz[1], "fire confirm")
		m.Update(key("y"))
		m.w.Player.Stock[m.w.Products[0]] = 200
		m.Update(key("s"))
		m.Update(key("enter"))
		m.Update(key("enter"))
		m.Update(key("enter"))
		m.w.Corner(m.cfg.City.Territory.Start).Risk = 100 // a robbery for the report
		m.Update(key("n"))
		assertFits(t, m.View(), sz[0], sz[1], "report with crew")
		if len(m.w.Report.Territory) == 0 {
			t.Fatalf("%dx%d: report has no territory lines: %+v", sz[0], sz[1], m.w.Report)
		}
		m.Update(key("enter"))
		m.Update(key("1"))
		m.Update(key("b"))
		assertFits(t, m.View(), sz[0], sz[1], "buy dialog")
		m.Update(key("enter"))
		assertFits(t, m.View(), sz[0], sz[1], "buy qty")
		m.Update(key("esc"))
		m.Update(key("esc"))
		m.Update(key("?"))
		assertFits(t, m.View(), sz[0], sz[1], "help")
		m.Update(key("x"))

		// A cartel-scale world: ten-digit cash on every screen, then the
		// money moved clean (dirty cash on that scale is an arrest) so the
		// next day unlocks every rung of the product ladder.
		m.w.Player.DirtyCash = 1_234_567_890
		m.w.Stats.PeakCash = m.w.Player.DirtyCash
		for _, s := range []string{"1", "2", "3", "4", "5"} {
			m.Update(key(s))
			assertFits(t, m.View(), sz[0], sz[1], "rich screen "+s)
		}
		m.Update(key("1"))
		m.Update(key("b"))
		m.Update(key("enter"))
		assertFits(t, m.View(), sz[0], sz[1], "rich buy qty")
		m.Update(key("esc"))
		m.Update(key("esc"))
		m.w.Player.CleanCash, m.w.Player.DirtyCash = m.w.Player.DirtyCash, 50_000
		m.Update(key("n"))
		m.Update(key("enter"))
		if got := len(m.w.Products); got != len(m.cfg.Market.Products) {
			t.Fatalf("%d of %d products unlocked with a billion in the bank", got, len(m.cfg.Market.Products))
		}
		last := m.w.Products[len(m.w.Products)-1]
		m.w.Player.Stock[last] = 20
		for _, s := range []string{"1", "2", "5"} {
			m.Update(key(s))
			assertFits(t, m.View(), sz[0], sz[1], "ladder screen "+s)
		}
		m.Update(key("1"))
		m.Update(key("s"))
		for range m.w.Products {
			m.Update(key("j"))
		}
		m.Update(key("enter"))
		m.Update(key("enter"))
		assertFits(t, m.View(), sz[0], sz[1], "ladder sell dialog")
		m.Update(key("enter"))
		m.Update(key("n"))
		assertFits(t, m.View(), sz[0], sz[1], "ladder report")
		m.Update(key("enter"))
		m.w.Over = &game.Ending{Day: m.w.Day, Cause: "indicted", PeakCash: m.w.Stats.PeakCash}
		m.Update(key("n"))
		assertFits(t, m.View(), sz[0], sz[1], "rich game over")
	}
}

func TestCashFormatting(t *testing.T) {
	cases := map[int]string{
		0: "$0", 500: "$500", 9_999: "$9,999", -2_500: "-$2,500",
		10_000: "$10K", 45_000: "$45K", 123_456: "$123K", 999_499: "$999K", 999_600: "$1.0M",
		1_234_567: "$1.2M", 12_345_678: "$12M", 3_400_000_000: "$3.4B", -1_500_000: "-$1.5M",
		1_234_567_890_123: "$1.2T",
	}
	for n, want := range cases {
		if got := cash(n); got != want {
			t.Errorf("cash(%d) = %q, want %q", n, got, want)
		}
	}
	for v, want := range map[float64]string{19.5: "$19.50", 999.99: "$999.99", 2500: "$2,500", 10000: "$10,000"} {
		if got := price(v); got != want {
			t.Errorf("price(%v) = %q, want %q", v, got, want)
		}
	}
}

func TestBuyThenSellFlow(t *testing.T) {
	m := newTestModel(t, 100, 30)
	m.Update(key("b"))
	m.Update(key("enter")) // pick first product
	for _, r := range "10" {
		m.Update(key(string(r)))
	}
	m.Update(key("enter"))
	if m.mode != modePlay {
		t.Fatalf("buy did not complete: mode=%v err=%q", m.mode, m.dlg.err)
	}
	id := m.w.Products[0]
	if m.w.Player.Stock[id] != 10 {
		t.Fatalf("stock after buy = %d", m.w.Player.Stock[id])
	}
	m.Update(key("s"))
	m.Update(key("enter"))
	m.Update(key("enter")) // blank = all
	m.Update(key("1"))     // quiet
	m.Update(key("enter"))
	if o, ok := m.w.Orders[id]; !ok || o.Qty != 10 {
		t.Fatalf("order not placed: %+v", m.w.Orders)
	}
	m.Update(key("n"))
	if m.mode != modeReport || m.w.Day != 1 {
		t.Fatalf("end day: mode=%v day=%d", m.mode, m.w.Day)
	}
}

func TestTickerNeverWiderThanTerminal(t *testing.T) {
	m := newTestModel(t, 60, 20)
	for i := 0; i < 30; i++ {
		m.Update(key("n"))
		m.Update(key("enter"))
	}
	for i := 0; i < 200; i++ {
		m.Update(tickMsg{})
		if w := lipgloss.Width(m.viewTicker()); w > 60 {
			t.Fatalf("ticker width %d at tick %d", w, i)
		}
	}
}

// Enter confirms dialogs and closes the report, so a stray extra press must
// never burn a day: on the play screen it only asks. n ends the day at once;
// enter ends it after a confirmation.
func TestEnterDoesNotEndDay(t *testing.T) {
	m := newTestModel(t, 80, 24)
	m.Update(key("n"))
	m.Update(key("enter")) // close report
	day := m.w.Day
	m.Update(key("enter"))
	if m.w.Day != day || m.mode != modeConfirmEnd {
		t.Fatalf("one enter: day %d -> %d, mode %v", day, m.w.Day, m.mode)
	}
	assertFits(t, m.View(), 80, 24, "end-day confirm")
	m.Update(key("esc"))
	if m.w.Day != day || m.mode != modePlay {
		t.Fatalf("esc on the confirm: day %d -> %d, mode %v", day, m.w.Day, m.mode)
	}
	m.Update(key("enter"))
	m.Update(key("enter"))
	if m.w.Day != day+1 || m.mode != modeReport {
		t.Fatalf("enter, enter: day %d -> %d, mode %v", day, m.w.Day, m.mode)
	}
	m.Update(key("enter")) // close report: never a day
	m.Update(key("enter"))
	m.Update(key("y"))
	if m.w.Day != day+2 {
		t.Fatalf("enter, y: day %d -> %d", day+1, m.w.Day)
	}
	m.Update(key("enter"))
	m.Update(key("n"))
	if m.w.Day != day+3 {
		t.Fatalf("n did not advance the day: %d -> %d", day+2, m.w.Day)
	}
}

// The crew screen: h hires the selected candidate, f asks before firing,
// p cycles the pay dial, and none of it happens from other screens.
func TestCrewScreenKeys(t *testing.T) {
	m := newTestModel(t, 100, 30)
	m.w.Player.DirtyCash = 5000
	m.Update(key("h"))
	if len(m.w.Crew.Members) != 0 {
		t.Fatal("hired from the dashboard")
	}
	m.Update(key("4"))
	if m.screen != screenCrew {
		t.Fatalf("screen = %v", m.screen)
	}
	m.Update(key("h")) // cursor starts on the first candidate when the roster is empty
	if len(m.w.Crew.Members) != 1 {
		t.Fatalf("hire failed: %q", m.status)
	}
	hired := m.w.Crew.Members[0]
	if m.w.Capacity() != m.w.Player.CarryLimit+hired.Units || m.w.Player.DirtyCash != 5000-hired.Fee {
		t.Fatalf("after hire: capacity %d cash %d, member %+v", m.w.Capacity(), m.w.Player.DirtyCash, hired)
	}
	m.Update(key("p"))
	if m.w.Crew.Pay != events.PayGenerous {
		t.Fatalf("pay after one p = %v", m.w.Crew.Pay)
	}
	m.Update(key("p"))
	m.Update(key("p"))
	if m.w.Crew.Pay != events.PayFair {
		t.Fatalf("pay after three p = %v", m.w.Crew.Pay)
	}
	// Cursor is on the new hire; f asks, anything but y backs out.
	m.Update(key("f"))
	if m.mode != modeConfirmFire {
		t.Fatalf("f did not ask: mode %v status %q", m.mode, m.status)
	}
	m.Update(key("esc"))
	if m.mode != modePlay || len(m.w.Crew.Members) != 1 {
		t.Fatal("esc fired someone")
	}
	m.Update(key("f"))
	m.Update(key("y"))
	if len(m.w.Crew.Members) != 0 || len(m.w.Crew.FiredToday) != 1 {
		t.Fatalf("y did not fire: %+v", m.w.Crew)
	}
	m.Update(key("n"))
	if m.mode != modeReport || len(m.w.Report.Crew) == 0 {
		t.Fatalf("report has no crew lines: %+v", m.w.Report)
	}
	if !strings.Contains(strings.Join(m.w.Report.Crew, "\n"), hired.Name) {
		t.Fatalf("report does not mention %s: %v", hired.Name, m.w.Report.Crew)
	}
}

// A schema-1 save (before the crew) continues: the run is upgraded with a
// hiring pool and fair pay, and the day is kept.
func TestOldSaveIsMigrated(t *testing.T) {
	t.Setenv("KINGPIN_HOME", t.TempDir())
	w := sim.NewWorld(content.MustLoad(), 1)
	w.SchemaVersion = 1
	w.Day = 9
	w.Crew = game.CrewState{}
	w.Territory = game.TerritoryState{}
	w.Rival = game.RivalState{}
	if err := game.Save(w); err != nil {
		t.Fatal(err)
	}
	m, err := New(content.MustLoad())
	if err != nil {
		t.Fatal(err)
	}
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m.Update(key("c"))
	if m.mode != modePlay || m.w.Day != 9 || m.w.SchemaVersion != game.SchemaVersion {
		t.Fatalf("continue: mode %v day %d schema %d status %q", m.mode, m.w.Day, m.w.SchemaVersion, m.status)
	}
	if len(m.w.Crew.Candidates) == 0 || m.w.Crew.Pay != events.PayFair {
		t.Fatalf("migrated crew state: %+v", m.w.Crew)
	}
	if m.w.Worked() != 1 || m.w.Corner(m.cfg.City.Territory.Start).Runner != game.You {
		t.Fatalf("migrated territory: %d worked, corners %+v", m.w.Worked(), m.w.Territory.Corners)
	}
	if m.w.Rival.Leader == "" || m.w.Rival.Personality == "" || m.w.Rival.Arrived != 0 {
		t.Fatalf("migrated rival: %+v", m.w.Rival)
	}
	m.Update(key("4"))
	assertFits(t, m.View(), 80, 24, "crew screen after migration")
	m.Update(key("5"))
	assertFits(t, m.View(), 80, 24, "map after migration")
}

// A save this build cannot read is refused with a readable message and the
// start menu moves the player to New run.
func TestUnreadableSaveIsRefused(t *testing.T) {
	t.Setenv("KINGPIN_HOME", t.TempDir())
	w := sim.NewWorld(content.MustLoad(), 1)
	w.SchemaVersion = game.SchemaVersion + 1
	if err := game.Save(w); err != nil {
		t.Fatal(err)
	}
	m, err := New(content.MustLoad())
	if err != nil {
		t.Fatal(err)
	}
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	if m.mode != modeStart {
		t.Fatalf("mode = %v, want the start menu", m.mode)
	}
	m.Update(key("c"))
	if m.mode != modeStart || !strings.Contains(m.status, "newer version") || m.startChoice != 1 {
		t.Fatalf("continue: mode %v status %q choice %d", m.mode, m.status, m.startChoice)
	}
	assertFits(t, m.View(), 80, 24, "start menu with error")
	m.Update(key("enter"))
	if m.mode != modePlay || m.w.SchemaVersion != game.SchemaVersion || m.w.Day != 0 {
		t.Fatalf("new run: mode %v schema %d day %d", m.mode, m.w.SchemaVersion, m.w.Day)
	}
}

// Left and right walk the tab bar and wrap; inside the sell dialog they
// still move the dial.
func TestArrowsSwitchTabs(t *testing.T) {
	m := newTestModel(t, 80, 24)
	if m.screen != screenDashboard {
		t.Fatalf("start screen %v", m.screen)
	}
	m.Update(key("left"))
	if m.screen != screenCount-1 {
		t.Fatalf("left from the first tab went to %v", m.screen)
	}
	m.Update(key("right"))
	if m.screen != screenDashboard {
		t.Fatalf("right from the last tab went to %v", m.screen)
	}
	for i := 1; i < int(screenCount); i++ {
		m.Update(key("right"))
		if m.screen != screen(i) {
			t.Fatalf("right %d: screen %v", i, m.screen)
		}
	}
	m.Update(key("1"))
	m.w.Player.Stock[m.w.Products[0]] = 5
	m.Update(key("s"))
	m.Update(key("enter"))
	m.Update(key("enter"))
	m.Update(key("right"))
	if m.mode != modeSell || m.dlg.dial != events.DialAggressive || m.screen != screenDashboard {
		t.Fatalf("right in the sell dialog: mode %v dial %v screen %v", m.mode, m.dlg.dial, m.screen)
	}
}

// The map screen: c posts a runner (you, or one of the crew) on the
// selected corner and claims it, e posts an enforcer, a abandons; the
// dashboard says so when nothing is held; none of it happens elsewhere.
func TestMapScreenKeys(t *testing.T) {
	m := newTestModel(t, 100, 30)
	m.w.Player.DirtyCash = 5000
	start := m.w.Corner(m.cfg.City.Territory.Start)
	if m.screen != screenDashboard || m.w.Territory.Corners[m.mapCursor].ID != start.ID {
		t.Fatalf("cursor starts on corner %d, not yours", m.mapCursor)
	}
	m.Update(key("c"))
	if m.mode != modePlay || !strings.Contains(m.status, "map") {
		t.Fatalf("c on the dashboard: mode %v status %q", m.mode, m.status)
	}
	m.w.Crew.Members = append(m.w.Crew.Members,
		game.CrewMember{ID: 101, Name: "Dre", Role: "runner", Skill: 60, Loyalty: 70, Units: 120, Wage: 56},
		game.CrewMember{ID: 102, Name: "Tank", Role: "enforcer", Skill: 50, Loyalty: 70, Wage: 55},
	)
	m.w.Crew.NextID = 102
	runner, enforcer := m.w.Crew.Member(101), m.w.Crew.Member(102)
	m.Update(key("5"))
	if m.screen != screenMap {
		t.Fatalf("screen %v", m.screen)
	}
	// Move to a free corner and post the runner: the picker lists you first.
	m.Update(key("j"))
	target := m.w.Territory.Corners[m.mapCursor]
	if target.ID == start.ID {
		m.Update(key("j"))
		target = m.w.Territory.Corners[m.mapCursor]
	}
	m.Update(key("c"))
	if m.mode != modePost || m.postRole != "runner" {
		t.Fatalf("c on the map: mode %v role %q", m.mode, m.postRole)
	}
	rows := m.postRows("runner")
	if rows[0].ID != game.You {
		t.Fatalf("picker rows %+v", rows)
	}
	for i, r := range rows {
		if r.ID == runner.ID {
			for j := 0; j < i; j++ {
				m.Update(key("j"))
			}
		}
	}
	m.Update(key("enter"))
	c := m.w.Corner(target.ID)
	if m.mode != modePlay || !c.Worked() || c.Runner != runner.ID || m.w.Held() != 2 {
		t.Fatalf("after posting: mode %v corner %+v held %d status %q", m.mode, *c, m.w.Held(), m.status)
	}
	m.Update(key("e"))
	m.Update(key("enter"))
	if c.Enforcer != enforcer.ID {
		t.Fatalf("after posting an enforcer: %+v status %q", *c, m.status)
	}
	assertFits(t, m.View(), 100, 30, "map with posts")
	// Firing the runner leaves the corner held but unworked; the day
	// report and the dashboard both say so once it drifts.
	m.Update(key("4"))
	for i, r := range m.crewRows() {
		if r.ID == runner.ID {
			m.crewCursor = i
		}
	}
	m.Update(key("f"))
	m.Update(key("y"))
	if c.Runner != 0 || !c.Held() {
		t.Fatalf("after firing the runner: %+v", *c)
	}
	for i := 0; i < m.cfg.City.Territory.DriftDays; i++ {
		m.Update(key("n"))
		m.Update(key("enter"))
	}
	if c.Held() || c.Enforcer != 0 {
		t.Fatalf("corner did not drift: %+v", *c)
	}
	if !strings.Contains(strings.Join(m.w.Report.Territory, "\n"), target.Name) {
		t.Fatalf("report does not mention %s: %v", target.Name, m.w.Report.Territory)
	}
	// Abandon your own corner: nothing sells and the dashboard says why.
	m.Update(key("5"))
	m.mapCursor = m.yourCorner()
	m.Update(key("a"))
	if m.w.Worked() != 0 {
		t.Fatalf("abandon: %d worked, status %q", m.w.Worked(), m.status)
	}
	m.Update(key("1"))
	if !strings.Contains(stripANSI(m.View()), "hold no corner") {
		t.Fatal("dashboard does not say why nothing sells")
	}
	m.w.Player.Stock[m.w.Products[0]] = 10
	m.Update(key("s"))
	m.Update(key("enter"))
	m.Update(key("enter"))
	assertFits(t, m.View(), 100, 30, "sell dialog with no corner")
	m.Update(key("enter"))
	m.Update(key("n"))
	if m.w.Player.Stock[m.w.Products[0]] != 10 {
		t.Fatalf("sold %d units with no corner", 10-m.w.Player.Stock[m.w.Products[0]])
	}
}

// The map screen's w: enforcers go against a rival corner at a force the
// picker chooses, one strike a day that can be called off, and nothing
// happens from other screens, on your own corners, or without enforcers.
func TestStrikeKeys(t *testing.T) {
	m := newTestModel(t, 100, 30)
	m.Update(key("w"))
	if m.mode != modePlay || !strings.Contains(m.status, "map") {
		t.Fatalf("w on the dashboard: mode %v status %q", m.mode, m.status)
	}
	m.Update(key("5"))
	m.mapCursor = m.yourCorner()
	m.Update(key("w"))
	if m.mode != modePlay || !strings.Contains(m.status, "rival") {
		t.Fatalf("w on your own corner: mode %v status %q", m.mode, m.status)
	}
	docks := m.w.Corner("docks")
	docks.Owner = game.OwnerRival
	m.w.Rival.Arrived, m.w.Rival.Muscle = 1, 3
	for i, c := range m.w.Territory.Corners {
		if c.ID == "docks" {
			m.mapCursor = i
		}
	}
	m.Update(key("w"))
	if m.mode != modePlay || !strings.Contains(m.status, "enforcers") {
		t.Fatalf("w with no enforcers: mode %v status %q", m.mode, m.status)
	}
	m.w.Crew.Members = append(m.w.Crew.Members, game.CrewMember{ID: 102, Name: "Tank", Role: "enforcer", Skill: 50, Loyalty: 70, Nerve: 60, Wage: 55})
	m.w.Crew.NextID = 102
	m.Update(key("w"))
	if m.mode != modeStrike || len(m.strikeRows()) != 3 {
		t.Fatalf("w with an enforcer: mode %v rows %v", m.mode, m.strikeRows())
	}
	m.Update(key("3")) // hit
	if m.mode != modePlay || m.w.Strike == nil || m.w.Strike.Corner != "docks" || m.w.Strike.Force != events.ForceHit {
		t.Fatalf("after picking hit: mode %v strike %+v status %q", m.mode, m.w.Strike, m.status)
	}
	if !strings.Contains(stripANSI(m.View()), "hit tonight") {
		t.Fatal("the map does not show where the enforcers go")
	}
	m.Update(key("1"))
	if !strings.Contains(stripANSI(m.View()), "tonight") {
		t.Fatal("the dashboard does not show where the enforcers go")
	}
	m.Update(key("5"))
	m.Update(key("w"))
	rows := m.strikeRows()
	if len(rows) != 4 {
		t.Fatalf("picker with a strike queued: %v", rows)
	}
	m.Update(key("4")) // stop
	if m.w.Strike != nil {
		t.Fatalf("stop did not call it off: %+v", m.w.Strike)
	}
	m.Update(key("w"))
	m.Update(key("j"))
	m.Update(key("j"))
	m.Update(key("enter")) // hit again
	m.Update(key("n"))
	if m.mode != modeReport || m.w.Strike != nil || m.w.Stats.Strikes != 1 {
		t.Fatalf("after the night: mode %v strike %+v stats %+v", m.mode, m.w.Strike, m.w.Stats)
	}
	if !strings.Contains(strings.Join(m.w.Report.Territory, "\n"), "The Docks") {
		t.Fatalf("report does not mention the strike: %v", m.w.Report.Territory)
	}
	if !strings.Contains(strings.Join(m.w.Report.Heat, "\n"), "enforcers") {
		t.Fatalf("report does not charge heat for it: %v", m.w.Report.Heat)
	}
}
