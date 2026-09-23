package ui

import (
	"strings"
	"testing"

	"github.com/theclifmeister/kingpin/internal/engine"
	"github.com/theclifmeister/kingpin/internal/game"
)

// TestCrewTroubleWords (#345): a member near the walk and a corner
// nobody works are alerts in the voice, named, with the pointer to where
// the fix is; the dashboard carries the counts as a crew fact.
func TestCrewTroubleWords(t *testing.T) {
	m := newTestModel(t, 120, 40)
	w := m.w
	tun := m.cfg.Crew.Crew
	w.Player.DirtyCash = 100000
	w.Crew.Members = []game.CrewMember{{ID: 1, Name: "Deshawn", Role: game.RoleRunner, Skill: 50, Loyalty: tun.QuitThreshold + 3.5, Nerve: 50, Wage: 50}}
	c := &w.Here().Corners[0]
	c.Owner, c.Runner, c.Idle = game.OwnerPlayer, 0, 0

	lines := m.alertsOf(engine.AlertCrewLine)
	if len(lines) != 1 {
		t.Fatalf("crew line alerts %+v", lines)
	}
	if got := stripANSI(lines[0].text); !strings.HasPrefix(got, "Deshawn is 4 from walking") || !strings.Contains(got, screenPointer(screenCrew)) {
		t.Errorf("the crew line alert reads %q", got)
	}
	if lines[0].why != "Deshawn 4 from walking" {
		t.Errorf("the stop reads %q", lines[0].why)
	}
	idle := m.alertsOf(engine.AlertIdleCorner)
	if len(idle) != 1 {
		t.Fatalf("idle corner alerts %+v", idle)
	}
	drift := m.rules.Territory.DriftDays(w)
	if got := stripANSI(idle[0].text); !strings.HasPrefix(got, "Nobody works "+c.Name+": back to the street in "+plural(drift, "day")) || !strings.Contains(got, screenPointer(screenMap)) {
		t.Errorf("the idle corner alert reads %q", got)
	}
	if got, want := m.crewTrouble(), "1 near the line, 1 corner unworked"; got != want {
		t.Errorf("the crew trouble reads %q, want %q", got, want)
	}
	if v := stripANSI(m.View()); !strings.Contains(v, "1 near the line, 1 corner unworked") {
		t.Errorf("the dashboard does not carry the crew trouble:\n%s", v)
	}
	c.Idle = drift - 1
	if got := stripANSI(m.alertsOf(engine.AlertIdleCorner)[0].text); !strings.Contains(got, "back to the street tonight") {
		t.Errorf("the last day reads %q", got)
	}
}
