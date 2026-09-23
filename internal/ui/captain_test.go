package ui

import (
	"strings"
	"testing"
)

// Veterans and the captain (#346): a 150-day veteran reads differently
// from a new hire on the crew screen (the trait column, the trait and
// the days served in the pane, what it does; the new hire's pane says
// when theirs will show), and c names a trusted one captain of a city
// at the budget ←→ picks, which the pane then reads.
func TestVeteranReadsDifferently(t *testing.T) {
	m := richModel(t, 120, 40)
	w := m.w
	vet := &w.Crew.Members[0]
	vet.Hired, vet.Trait = w.Day-150, "steady"
	m.Update(key("4"))
	m.crewCursor = 0
	if view := stripANSI(m.View()); !strings.Contains(view, "trait") || !strings.Contains(view, "steady") {
		t.Fatalf("the crew screen with a veteran:\n%s", view)
	}
	if pane := paneText(m); !strings.Contains(pane, "steady · served 150 days") || !strings.Contains(pane, "takes a bad night at half the loss") || !strings.Contains(pane, "c  make them captain of a city") {
		t.Fatalf("the veteran's pane:\n%s", pane)
	}
	m.crewCursor = 1
	if pane := paneText(m); !strings.Contains(pane, "known in") || strings.Contains(pane, "served") || strings.Contains(pane, "make them captain") {
		t.Fatalf("the new hire's pane:\n%s", pane)
	}
	m.Update(key("c"))
	if m.mode == modeCaptain {
		t.Fatal("a new hire was offered the captaincy")
	}
	m.crewCursor = 0
	m.Update(key("c"))
	if m.mode != modeCaptain {
		t.Fatalf("c on the veteran opened mode %v: %s", m.mode, m.status)
	}
	assertFits(t, m.View(), 120, 40, "the captain picker")
	m.Update(key("right"))
	m.Update(key("enter"))
	budgets := m.rules.Crew.Captaincy().Budgets
	if vet.Captain != w.Player.Location || vet.Budget != budgets[1] {
		t.Fatalf("named captain of %q at %d, want %q at %d: %s", vet.Captain, vet.Budget, w.Player.Location, budgets[1], m.status)
	}
	if pane := paneText(m); !strings.Contains(pane, "captain     "+w.CityName(vet.Captain)) {
		t.Fatalf("the captain's pane:\n%s", pane)
	}
	// And off again: the last row of the picker.
	m.Update(key("c"))
	for range w.CityOrder {
		m.Update(key("down"))
	}
	m.Update(key("enter"))
	if vet.Captain != "" || vet.Budget != 0 {
		t.Fatalf("the captaincy stayed: %+v", *vet)
	}
}
