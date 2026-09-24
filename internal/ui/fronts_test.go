package ui

import (
	"strings"
	"testing"
)

// The fronts' roles (#344): the ledger's pane on a front says what the
// place is for and what it does, in the city it stands in and on the
// whole run, and the invest dialog opens on the line.
func TestFrontRoleInThePane(t *testing.T) {
	m := richModel(t, 120, 40)
	w := m.w
	if len(w.Fronts) == 0 {
		t.Fatal("the rich fixture has no front")
	}
	m.Update(key("7"))
	for i, f := range w.Fronts {
		m.ledgerCursor = i
		r := m.cfg.Upgrades.Front(f.ID)
		if r == nil {
			t.Fatalf("%s has no role", f.ID)
		}
		pane := strings.Join(strings.Fields(stripANSI(paneText(m))), " ")
		words := strings.Fields(r.Role)
		if !strings.Contains(pane, strings.Join(words[:3], " ")) {
			t.Errorf("%s: the pane lacks its role %q:\n%s", f.ID, r.Role, pane)
		}
		if len(effectWords(r.Effects.Reaching("city"))) > 0 && !strings.Contains(pane, "In "+w.CityName(w.FrontCity(f))+":") {
			t.Errorf("%s: the pane does not say where its role reaches:\n%s", f.ID, pane)
		}
		if len(effectWords(r.Effects.Reaching("run"))) > 0 && !strings.Contains(pane, "Everywhere:") {
			t.Errorf("%s: the pane does not say its role reaches the run:\n%s", f.ID, pane)
		}
	}
	m.ledgerCursor = 0
	w.Player.CleanCash = 10_000_000
	m.Update(key("u"))
	if m.mode != modeInvest {
		t.Fatalf("u on a front: mode %v status %q", m.mode, m.status)
	}
	role := m.cfg.Upgrades.Front(w.Fronts[0].ID).Role
	if view := strings.Join(strings.Fields(stripANSI(m.View())), " "); !strings.Contains(view, strings.Join(strings.Fields(role)[:3], " ")) {
		t.Fatalf("the invest dialog lacks the role %q:\n%s", role, view)
	}
}
