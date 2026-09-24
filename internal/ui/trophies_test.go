package ui

import (
	"strings"
	"testing"
)

// The trophies in the grammar (#392): no TROPHIES block while the money
// is nowhere near; with the peak clean cash over the first line the
// block lists them under the one cursor; t on an open offer asks, y
// buys it with clean cash and the row reads it as yours; a locked one
// is refused before asking. The ledger carries the pile's weight and
// its rot once the pile is over the line. At the three sizes.
func TestTrophiesInTheGrammar(t *testing.T) {
	for _, sz := range [][2]int{{80, 24}, {100, 30}, {120, 40}} {
		m := richModel(t, sz[0], sz[1])
		w := m.w
		w.Stats.PeakClean = 0
		m.Update(key("7"))
		if m.trophiesShown() || strings.Contains(stripANSI(m.View()), "TROPHIES") {
			t.Fatalf("%dx%d: TROPHIES on the ledger with no clean money", sz[0], sz[1])
		}
		first := m.trophyRows()[0]
		w.Stats.PeakClean = first.UnlockCash
		w.Player.CleanCash = first.Cost + 1_000
		w.Player.DirtyCash = 400_000_000
		cursor := -1
		for i, r := range m.ledgerRows() {
			if r.kind == ledgerTrophyOffer && m.trophyRows()[r.i].ID == first.ID {
				cursor = i
			}
		}
		if cursor < 0 {
			t.Fatalf("%dx%d: no row for %s", sz[0], sz[1], first.Name)
		}
		m.ledgerCursor = cursor
		assertFrame(t, m, "ledger on a trophy")
		view := stripANSI(m.View())
		for _, want := range []string{"TROPHIES", first.Name, "tonnes"} {
			if !strings.Contains(view, want) {
				t.Fatalf("%dx%d: the ledger lacks %q:\n%s", sz[0], sz[1], want, view)
			}
		}
		m.Update(key("t"))
		if m.mode != modeConfirm {
			t.Fatalf("%dx%d: t on %s: mode %v status %q", sz[0], sz[1], first.Name, m.mode, m.status)
		}
		m.Update(key("y"))
		if len(w.Trophies) != 1 || w.Trophies[0].ID != first.ID || w.Player.CleanCash != 1_000 {
			t.Fatalf("%dx%d: bought %+v, clean %d, status %q", sz[0], sz[1], w.Trophies, w.Player.CleanCash, m.status)
		}
		if !strings.Contains(stripANSI(m.View()), "yours since") {
			t.Fatalf("%dx%d: the row does not read as yours", sz[0], sz[1])
		}
		// A locked one is refused before the question.
		for i, r := range m.ledgerRows() {
			if r.kind == ledgerTrophyOffer && m.trophyRows()[r.i].Locked(w) {
				m.ledgerCursor = i
				break
			}
		}
		m.Update(key("t"))
		if m.mode != modePlay || !strings.Contains(m.status, "peak clean cash") {
			t.Fatalf("%dx%d: t on a locked trophy: mode %v status %q", sz[0], sz[1], m.mode, m.status)
		}
	}
}
