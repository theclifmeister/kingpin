package ui

import (
	"strings"
	"testing"
)

// TestAmbitionsPanel (#347): a on the walk-away dialog opens the panel
// with every plan; enter pins the one under the cursor, which the
// dashboard's street then carries with its next step and the report
// with its PLAN line; enter again unpins it; esc closes onto play and
// the day never moves.
func TestAmbitionsPanel(t *testing.T) {
	m := richModel(t, 80, 24)
	day := m.w.Day
	m.Update(key("1"))
	m.Update(key("w"))
	m.Update(key("a"))
	if m.mode != modeAmbitions {
		t.Fatalf("a on the walk away: mode %v", m.mode)
	}
	view := stripANSI(m.View())
	assertFits(t, m.View(), 80, 24, "ambitions")
	for _, want := range []string{"AMBITIONS", "Retire clean", "Go legitimate", "Take the city", "Disappear", "Two-city operation"} {
		if !strings.Contains(view, want) {
			t.Errorf("the panel lacks %q:\n%s", want, view)
		}
	}
	m.Update(key("enter"))
	if m.w.Ambition != "retire" || m.mode != modeAmbitions || m.w.Day != day {
		t.Fatalf("enter pinned %q, mode %v, day %d", m.w.Ambition, m.mode, m.w.Day)
	}
	m.Update(key("esc"))
	if m.mode != modePlay {
		t.Fatalf("esc: mode %v", m.mode)
	}
	if view := stripANSI(m.View()); !strings.Contains(view, "plan Retire clean") {
		t.Errorf("the dashboard does not carry the plan:\n%s", view)
	}
	if lines := strings.Join(m.reportLines(), "\n"); !strings.Contains(stripANSI(lines), "PLAN") {
		t.Errorf("the report has no PLAN line:\n%s", stripANSI(lines))
	}
	m.Update(key("w"))
	m.Update(key("a"))
	m.Update(key("enter"))
	if m.w.Ambition != "" {
		t.Fatalf("enter on the plan left %q pinned", m.w.Ambition)
	}
	if lines := strings.Join(m.reportLines(), "\n"); strings.Contains(stripANSI(lines), "PLAN") {
		t.Error("the report carries a PLAN line with no plan")
	}
}

// TestAmbitionsFromTheStage (#347): a on the stage modal marks it seen
// and opens the panel; esc goes on to the card as closing the stage
// would.
func TestAmbitionsFromTheStage(t *testing.T) {
	m := newTestModel(t, 80, 24)
	hireOne(m)
	m.w.Dilemmas.Pending = testCard(m.w.Day + 1)
	day := m.w.Day
	m.Update(key("n"))
	if m.mode != modeStage {
		t.Fatalf("after n: mode %v", m.mode)
	}
	m.Update(key("a"))
	if m.mode != modeAmbitions || m.w.StagePending() != 0 {
		t.Fatalf("a on the stage: mode %v, stage pending %d", m.mode, m.w.StagePending())
	}
	m.Update(key("esc"))
	if m.mode != modeCard || m.w.Day != day+1 {
		t.Fatalf("esc from the panel: mode %v day %d", m.mode, m.w.Day)
	}
}
