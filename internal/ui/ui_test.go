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
		for _, s := range []string{"1", "2", "3", "4"} {
			m.Update(key(s))
			assertFits(t, m.View(), sz[0], sz[1], "screen "+s)
		}
		m.Update(key("f"))
		assertFits(t, m.View(), sz[0], sz[1], "fire confirm")
		m.Update(key("y"))
		m.Update(key("n"))
		assertFits(t, m.View(), sz[0], sz[1], "report with crew")
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
	m.Update(key("4"))
	assertFits(t, m.View(), 80, 24, "crew screen after migration")
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
