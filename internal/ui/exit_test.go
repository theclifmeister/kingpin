package ui

import (
	"strings"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/game"
)

// The exits and the summary (#49): the walk-away dialog asks twice and
// ends the run there and then, a closed way out is refused with what is
// short, an ended save of every cause reloads straight to its summary,
// and the summary carries what the issue asks for.

// TestWalkAwayAsksTwice: w on the ledger opens the dialog; enter on an
// open way out turns to the confirmation and y ends the run, saved,
// with the summary up; a closed one is refused and the run goes on.
func TestWalkAwayAsksTwice(t *testing.T) {
	m := richModel(t, 100, 30)
	m.Update(key("7"))
	m.Update(key("w"))
	if m.mode != modeExit || m.exit.step != 0 {
		t.Fatalf("w on the ledger: mode %v step %d", m.mode, m.exit.step)
	}
	// Neither is open on the fixture: enter refuses and stays.
	m.Update(key("enter"))
	if m.mode != modeExit || m.exit.step != 0 || !strings.Contains(m.status, "not open") {
		t.Fatalf("enter on a closed way out: mode %v step %d status %q", m.mode, m.exit.step, m.status)
	}
	m.Update(key("esc"))
	if m.mode != modePlay || m.w.Over != nil {
		t.Fatalf("esc: mode %v over %+v", m.mode, m.w.Over)
	}
	// Retiring: the account and the quiet days in hand, enter turns
	// to the confirmation, shift+tab back, enter again, y ends it.
	off := m.set.Laundering.Offshore()
	m.w.Offshore, m.w.QuietDays, m.w.Stats.Bodies = off.RetireCash+250_000, off.RetireDays, 1
	m.Update(key("w"))
	m.Update(key("enter"))
	if m.exit.step != 1 || !strings.Contains(stripANSI(m.View()), "RETIRE?") {
		t.Fatalf("enter on retire: step %d\n%s", m.exit.step, stripANSI(m.View()))
	}
	m.Update(key("shift+tab"))
	if m.exit.step != 0 {
		t.Fatalf("shift+tab did not go back: step %d", m.exit.step)
	}
	m.Update(key("enter"))
	m.Update(key("n")) // not y: nothing happens
	if m.w.Over != nil || m.mode != modeExit {
		t.Fatalf("a stray key ended the run: over %+v mode %v", m.w.Over, m.mode)
	}
	day := m.w.Day
	m.Update(key("y"))
	if m.w.Over == nil || m.w.Over.Cause != content.CauseRetired || m.mode != modeOver || m.w.Day != day {
		t.Fatalf("y: over %+v mode %v day %d", m.w.Over, m.mode, m.w.Day)
	}
	if want := (off.RetireCash + 250_000) / 2; m.w.Stats.Score != want {
		t.Errorf("score %d, want %d: the account over one plus the bodies", m.w.Stats.Score, want)
	}
	view := stripANSI(m.View())
	for _, want := range []string{"RETIRED CLEAN · DAY", "THE MONEY", "the score"} {
		if !strings.Contains(view, want) {
			t.Errorf("the summary lacks %q:\n%s", want, view)
		}
	}
	saved, err := game.Load(m.slot, m.set.Migrations()...)
	if err != nil || saved.Over == nil || saved.Over.Cause != content.CauseRetired {
		t.Fatalf("the ending was not saved: %v %+v", err, saved.Over)
	}
	// Vanishing: on the identity, from the second row.
	v := richModel(t, 100, 30)
	v.Update(key("7"))
	v.w.Upgrades["lawyer"], v.w.Upgrades["retainer"], v.w.Upgrades["identity"] = true, true, true
	v.Update(key("w"))
	if v.exit.cursor != 1 {
		t.Fatalf("the cursor did not open on the one open way out: %d", v.exit.cursor)
	}
	v.Update(key("enter"))
	v.Update(key("y"))
	if v.w.Over == nil || v.w.Over.Cause != content.CauseVanished || v.mode != modeOver {
		t.Fatalf("vanishing: over %+v mode %v", v.w.Over, v.mode)
	}
	if !strings.Contains(stripANSI(v.View()), "VANISHED · DAY") {
		t.Errorf("the summary is not the vanished one:\n%s", stripANSI(v.View()))
	}
}

// TestEndedSaveReloadsToTheSummary: a save of a run ended by every cause
// continues straight to modeOver with that cause's title up and no
// scene, and the epilogue renders for each.
func TestEndedSaveReloadsToTheSummary(t *testing.T) {
	for _, cause := range content.Causes {
		m := richModel(t, 80, 24)
		m.w.Over = m.w.End(cause, m.w.Day, m.w.Rival().Leader)
		on := newAnimModel(t, 80, 24) // its own home: the save goes there
		if err := game.Save(2, m.w); err != nil {
			t.Fatal(err)
		}
		if err := on.continueRun(2); err != nil {
			t.Fatal(err)
		}
		if on.mode != modeOver || on.scene != nil {
			t.Fatalf("%s: mode %v scene %v", cause, on.mode, on.scene)
		}
		view := stripANSI(on.View())
		title := on.cfg.Endings.Title(cause)
		if !strings.Contains(view, title+" · DAY") {
			t.Errorf("%s: the summary is not up:\n%s", cause, view)
		}
		if ep := on.epilogue(); ep == "" || ep == strings.ToUpper(cause) || strings.Contains(ep, "{{") {
			t.Errorf("%s: the epilogue did not render: %q", cause, ep)
		}
	}
}

// TestSummaryReadsTheRun: the summary names the account as the score,
// the pile left behind, the story's headlines with their days, the
// bodies and the fallen as `(role · day N)`, the betrayals, the best of
// the crew, the reputation bars and the score line; it scrolls at 80x24
// and stands whole at 120x40.
func TestSummaryReadsTheRun(t *testing.T) {
	m := richModel(t, 120, 40)
	w := m.w
	w.Offshore, w.Stats.Bodies, w.Stats.Fallen, w.Stats.BetrayedBy = 600_000, 2, 1, 1
	w.Crew.Fallen = append(w.Crew.Fallen, game.Fallen{ID: 77, Name: "Ziggy", Role: "runner", Age: 24, Day: 3, Corner: "c1", CornerName: "Rail Yard"})
	w.Stats.Invested, w.Stats.Earned = 1_000_000, 50_000
	w.Over = w.End(content.CauseKingpin, w.Day, "")
	m.mode = modeOver
	view := stripANSI(m.View())
	for _, want := range []string{
		"KINGPIN · DAY", "It lasted", "THE STORY", "day 1", "THE MONEY", "$600K · the score", "left behind",
		"THE PEOPLE", "2, 1 of them yours", "Ziggy (runner · day 3)", "1 deal broken by them", "best of them", "Ziggy, runner, fell on day 3",
		"THE CITY", "fear", "respect", "notoriety", "SCORE  $200K", "$600K over 1 + 2 bodies",
	} {
		if !strings.Contains(view, want) {
			t.Errorf("the summary lacks %q:\n%s", want, view)
		}
	}
	// The kingpin's reign is a function of the heat and the pressure
	// at home: cool and quiet it is the whole span, hot and loud a year.
	home := w.Home()
	home.Heat, home.Pressure = 0, 0
	if ep := m.epilogue(); !strings.Contains(ep, "12 years") {
		t.Errorf("cool and quiet: %q", ep)
	}
	home.Heat, home.Pressure = 100, 100
	if ep := m.epilogue(); !strings.Contains(ep, "1 year.") {
		t.Errorf("hot and loud: %q", ep)
	}
	// At 80x24 the summary scrolls: the footer says more, and the end
	// of the body has the score.
	s := richModel(t, 80, 24)
	s.w.Over = s.w.End(content.CauseBetrayed, s.w.Day, "Ziggy")
	s.mode = modeOver
	if v := stripANSI(s.View()); !strings.Contains(v, "↓ more") || !strings.Contains(v, "Ziggy knew where everything was") {
		t.Errorf("80x24: the summary does not scroll, or the epilogue does not name the lieutenant:\n%s", v)
	}
	for i := 0; i < 40; i++ {
		s.Update(key("j"))
	}
	if v := stripANSI(s.View()); !strings.Contains(v, "SCORE") {
		t.Errorf("80x24: scrolled to the end, no score line:\n%s", v)
	}
	// At 120x40 every cause stands whole, on a run with the fallen, the
	// levels and the betrayals to print.
	for _, cause := range content.Causes {
		m.w.Over = m.w.End(cause, m.w.Day, "Ziggy")
		m.modalScroll = 0
		if v := stripANSI(m.View()); strings.Contains(v, "↓ more") || !strings.Contains(v, "SCORE") {
			t.Errorf("120x40 %s: the summary does not stand whole:\n%s", cause, v)
		}
	}
}
