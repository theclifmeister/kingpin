package ui

import (
	"strings"
	"testing"

	"github.com/theclifmeister/kingpin/internal/engine"
	"github.com/theclifmeister/kingpin/internal/game"
)

// An open investigation (#343) names its target and the nights to the
// hit: the ALERTS line with the answer and the screen it is given on,
// the stop's reason, and HEAT's last ladder line, at every common size,
// red the night it lands.
func TestInvestigationWords(t *testing.T) {
	for _, size := range [][2]int{{80, 24}, {100, 30}, {120, 40}} {
		m := newTestModel(t, size[0], size[1])
		w := m.w
		c := w.Here().Corners[0]
		w.Heat.Investigation = game.Investigation{City: w.Here().ID, Kind: game.LeadCorner, Target: c.ID, Opened: w.Day, Due: w.Day + 2}
		lines := m.alertsOf(engine.AlertInvestigation)
		if len(lines) != 1 {
			t.Fatalf("%v: investigation alerts %+v", size, lines)
		}
		if got := stripANSI(lines[0].text); !strings.HasPrefix(got, "Police are working "+c.Name+": they hit in 2 days.") || !strings.Contains(got, screenPointer(screenMap)) {
			t.Errorf("%v: the alert reads %q", size, got)
		}
		if lines[0].why != "police working "+c.Name {
			t.Errorf("%v: the stop reads %q", size, lines[0].why)
		}
		if v := stripANSI(m.View()); !strings.Contains(v, "police on") {
			t.Errorf("%v: HEAT does not carry the investigation:\n%s", size, v)
		}
		w.Heat.Investigation.Due = w.Day + 1
		if got := stripANSI(m.alertsOf(engine.AlertInvestigation)[0].text); !strings.Contains(got, "they hit tonight") {
			t.Errorf("%v: the last night reads %q", size, got)
		}
	}
}
