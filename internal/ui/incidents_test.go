package ui

import (
	"strings"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// The world's incident (#44) opens the report, before TIER, in the
// world's own colour; the journal lists the world source first in its
// legend and colours its headlines with it; the routes screen says a
// route is shut and for how many more nights.
func TestIncidentInTheGrammar(t *testing.T) {
	for _, size := range [][2]int{{80, 24}, {120, 40}} {
		m := richModel(t, size[0], size[1])
		m.w.Report.Tier = []string{"TERRITORY, tier 3 of 4."}
		m.w.Report.Incident = []string{"The dockers are out: The Channel is shut for 6 days."}
		m.mode = modeReport
		view := stripANSI(m.View())
		at := strings.Index(view, "INCIDENT")
		if at < 0 || at > strings.Index(view, "TIER") || !strings.Contains(view, "The dockers are out") {
			t.Errorf("%dx%d: the report does not open with the INCIDENT section:\n%s", size[0], size[1], view)
		}
		if raw := m.View(); !strings.Contains(raw, theme.Fg(theme.World).Bold(true).Render("INCIDENT")) {
			t.Errorf("%dx%d: the INCIDENT heading is not in the world's colour", size[0], size[1])
		}
	}
	if journalSources[0] != "world" || theme.Source("world") != theme.World || theme.Source("world") == theme.Source("news") {
		t.Fatalf("the world source: first %q, colour %v", journalSources[0], theme.Source("world"))
	}

	// The route's pane says it is shut.
	m := richModel(t, 120, 40)
	route := m.set.Logistics.Routes(m.w.Home().ID)[0]
	m.w.ApplyIncident(m.w.Day+1, content.IncidentEffects{RouteClosed: 3}, game.IncidentTarget{Routes: []string{route.ID}})
	if !m.w.RouteClosed(route.ID) {
		t.Fatal("the route is not closed")
	}
	sec := m.routeSection(route)
	lines := stripANSI(strings.Join(sec.lines, "\n"))
	if !strings.Contains(lines, "closed") || !strings.Contains(lines, "3 nights to go") {
		t.Errorf("the route's section does not say it is shut for three nights:\n%s", lines)
	}
}
