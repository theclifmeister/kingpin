package ui

import (
	"strings"
	"testing"

	"github.com/theclifmeister/kingpin/internal/game"
)

// The tier is on the dashboard, in the summary and at the top of the
// report (#147): the street's facts read `tier Distribution` at 80x24
// and 120x40 on the rich fixture, the summary's `reached` row names the
// tier and the day, and the report opens with the TIER section.
func TestTierIsNamed(t *testing.T) {
	for _, size := range [][2]int{{80, 24}, {120, 40}} {
		m := richModel(t, size[0], size[1])
		if view := stripANSI(m.View()); !strings.Contains(view, "tier Distribution") {
			t.Errorf("%dx%d: the dashboard does not name the tier:\n%s", size[0], size[1], view)
		}
		m.w.Over = &game.Ending{Day: m.w.Day, Cause: "arrested", PeakCash: m.w.Stats.PeakCash}
		m.mode = modeOver
		if view := stripANSI(m.View()); !strings.Contains(view, "reached") || !strings.Contains(view, "Distribution on day 3") {
			t.Errorf("%dx%d: the summary does not name the tier reached:\n%s", size[0], size[1], view)
		}
		m.w.Over = nil
		m.mode = modePlay
		m.w.Report.Tier = []string{"TERRITORY, tier 3 of 4. Ground to hold and a front to wash the take.", "Opens: the first front and the wash", "Next: move $500K"}
		m.mode = modeReport
		// TIER is the first section, before UNLOCKED (#148: the rich
		// fixture's peak opens every gate at once, so PRICES is below
		// the fold at 80x24) and PRICES.
		view := stripANSI(m.View())
		next := strings.Index(view, "UNLOCKED")
		if next < 0 {
			next = strings.Index(view, "PRICES")
		}
		if !strings.Contains(view, "TIER") || strings.Index(view, "TIER") > next {
			t.Errorf("%dx%d: the report does not open with the TIER section:\n%s", size[0], size[1], view)
		}
	}
	// A fresh run is a Corner trader, on day 0, so the summary reads the
	// name alone.
	m := newTestModel(t, 80, 24)
	m.startRun(1)
	if got := m.reachedLine(); got != "Corner" {
		t.Errorf("a fresh run reads %q, want Corner", got)
	}
}
