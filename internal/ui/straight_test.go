package ui

import (
	"strings"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
)

// TestGoStraightFromTheDialog (#398): the walk-away dialog's fourth row
// is going straight. Closed, it says the streak so far and refuses;
// open, the dashboard's alert says so, the digit selects it, y ends the
// run a businessman and the summary opens.
func TestGoStraightFromTheDialog(t *testing.T) {
	m := richModel(t, 100, 30)
	w := m.w
	days := m.cfg.Laundering.Businessman.LegitDays
	w.LegitDays = 3
	m.Update(key("1"))
	m.Update(key("w"))
	if m.mode != modeExit {
		t.Fatalf("w on the dashboard: mode %v", m.mode)
	}
	if view := stripANSI(m.View()); !strings.Contains(view, "Go straight") {
		t.Fatalf("the dialog lacks going straight:\n%s", view)
	}
	m.Update(key("4"))
	if m.exit.step != 0 || !strings.Contains(m.exit.err, "3 of") {
		t.Fatalf("closed: step %d err %q", m.exit.step, m.exit.err)
	}
	m.Update(key("esc"))
	w.LegitDays = days
	if alerts := stripANSI(strings.Join(m.alertLines(100, 12), "\n")); !strings.Contains(alerts, "go straight") {
		t.Fatalf("the alerts do not say going straight is open:\n%s", alerts)
	}
	m.Update(key("w"))
	m.Update(key("4"))
	if m.exit.step != 1 {
		t.Fatalf("open: step %d status %q", m.exit.step, m.status)
	}
	m.Update(key("y"))
	if w.Over == nil || w.Over.Cause != content.CauseBusinessman || m.mode != modeOver {
		t.Fatalf("y: over %+v mode %v status %q", w.Over, m.mode, m.status)
	}
}
