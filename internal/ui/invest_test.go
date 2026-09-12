package ui

import (
	"strings"
	"testing"

	"github.com/theclifmeister/kingpin/internal/game"
)

// The invest dialog (#192): i on the ledger with the cursor on a front
// opens it, elsewhere on the ledger it is silent and off the ledger it
// points at the ledger; it refuses with no clean cash; the FRONTS table
// and the front's pane carry the level and what it earns; a level
// bought reads on the world at once and the status says what it
// earns; the modal fits 80x24 and esc leaves the world alone.
func TestInvestDialog(t *testing.T) {
	m := richModel(t, 80, 24)
	w := m.w
	l := m.set.Laundering
	m.Update(key("i"))
	if m.mode != modePlay || !strings.Contains(m.status, "ledger") {
		t.Fatalf("i on the dashboard: mode %v status %q", m.mode, m.status)
	}
	m.Update(key("7"))
	w.Player.CleanCash = 0
	m.Update(key("i"))
	if m.mode != modePlay || !strings.Contains(m.status, "clean cash") {
		t.Fatalf("i with no clean cash: mode %v status %q", m.mode, m.status)
	}
	f := w.Fronts[0]
	cost := l.LevelCost(f, 2)
	w.Player.CleanCash = cost + 1
	view := stripANSI(m.View())
	if !strings.Contains(view, "lvl") || !strings.Contains(view, "earns/day") {
		t.Fatalf("the FRONTS table has no level or income column:\n%s", view)
	}
	m.Update(key("i"))
	if m.mode != modeInvest || m.inv.front != f.ID {
		t.Fatalf("i on a front: mode %v front %q", m.mode, m.inv.front)
	}
	assertFits(t, m.View(), 80, 24, "invest dialog")
	if m.inv.levels.max != 2 {
		t.Fatalf("the field's max is %d with %s clean and two levels at %s, want 2", m.inv.levels.max, money(w.Player.CleanCash), money(cost))
	}
	m.Update(key("esc"))
	if m.mode != modePlay || w.Fronts[0].Level != 0 || w.Player.CleanCash != cost+1 {
		t.Fatal("esc invested")
	}
	m.Update(key("i"))
	m.Update(key("2"))
	m.Update(key("enter"))
	now := w.Fronts[0]
	if m.mode != modePlay || now.Level != 2 || now.Invested != cost || w.Player.CleanCash != 1 || !strings.Contains(m.status, "level 2") {
		t.Fatalf("enter: mode %v front %+v clean %d status %q", m.mode, now, w.Player.CleanCash, m.status)
	}
	if len(w.Today.Invested) != 1 || w.Today.Invested[0] != (game.Investment{Front: f.ID, Levels: 2, Cost: cost}) {
		t.Fatalf("the scratch: %+v", w.Today.Invested)
	}
	// The pane names the level, the next one and the key.
	m.Update(key(" "))
	pane := stripANSI(m.View())
	for _, want := range []string{"level", "2 of", "earns", "next", "i", "invest"} {
		if !strings.Contains(pane, want) {
			t.Fatalf("the pane lacks %q:\n%s", want, pane)
		}
	}
	m.Update(key("esc"))
	// Too many levels at once is refused in the dialog, and a front at
	// its top refuses before it opens.
	m.Update(key("i"))
	m.Update(key("9"))
	m.Update(key("9"))
	m.Update(key("enter"))
	if m.mode != modeInvest || m.inv.err == "" {
		t.Fatalf("99 levels: mode %v err %q", m.mode, m.inv.err)
	}
	m.Update(key("esc"))
	w.Fronts[0].Level = l.MaxLevel(w.Fronts[0])
	m.Update(key("i"))
	if m.mode != modePlay || !strings.Contains(m.status, "as big as it gets") {
		t.Fatalf("i on a front at its top: mode %v status %q", m.mode, m.status)
	}
	// Off a front i is silent on the ledger: the cursor on a house.
	m.ledgerCursor = 3
	m.Update(key("i"))
	if m.mode != modePlay {
		t.Fatalf("i on a house: mode %v", m.mode)
	}
}
