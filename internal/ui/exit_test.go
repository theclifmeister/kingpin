package ui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/game"
)

// The exits and the summary (#49): the walk-away dialog asks twice and
// ends the run there and then, a closed way out is refused with what is
// short, an ended save of every cause reloads straight to its summary,
// and the summary carries what the issue asks for.

// TestWalkAwayAsksTwice: w on the dashboard opens the dialog; enter on an
// open way out turns to the confirmation and y ends the run, saved,
// with the summary up; a closed one is refused and the run goes on.
func TestWalkAwayAsksTwice(t *testing.T) {
	m := richModel(t, 100, 30)
	m.Update(key("1"))
	m.Update(key("w"))
	if m.mode != modeExit || m.exit.step != 0 {
		t.Fatalf("w on the dashboard: mode %v step %d", m.mode, m.exit.step)
	}
	// Neither is open on the fixture: enter refuses and stays, saying
	// what is short on the dialog itself (#468: the status bar is under
	// the modal).
	m.Update(key("enter"))
	if m.mode != modeExit || m.exit.step != 0 || !strings.Contains(m.exit.err, "not open") || !strings.Contains(m.exit.err, "short") {
		t.Fatalf("enter on a closed way out: mode %v step %d err %q", m.mode, m.exit.step, m.exit.err)
	}
	if v := stripANSI(m.View()); !strings.Contains(v, "Retire is not open") {
		t.Fatalf("the refusal is not on the dialog:\n%s", v)
	}
	m.Update(key("down"))
	if m.exit.err != "" {
		t.Fatalf("the refusal outlived the next key: %q", m.exit.err)
	}
	m.Update(key("esc"))
	if m.mode != modePlay || m.w.Over != nil {
		t.Fatalf("esc: mode %v over %+v", m.mode, m.w.Over)
	}
	// Retiring: the account and the quiet days in hand, enter turns
	// to the confirmation, shift+tab back, enter again, y ends it.
	off := m.rules.Laundering.Offshore()
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
	m.Update(key("n")) // not y: declines, as every confirmation does (#241)
	if m.w.Over != nil || m.mode != modePlay {
		t.Fatalf("a stray key did not decline: over %+v mode %v", m.w.Over, m.mode)
	}
	m.Update(key("w"))
	m.Update(key("enter"))
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
	saved, err := game.Load(m.slot, m.sess.Sims().Migrations()...)
	if err != nil || saved.Over == nil || saved.Over.Cause != content.CauseRetired {
		t.Fatalf("the ending was not saved: %v %+v", err, saved.Over)
	}
	// Vanishing: on the identity, from the second row.
	v := richModel(t, 100, 30)
	v.Update(key("1"))
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
		"KINGPIN · DAY", "the crown lasted", "THE STORY", "day 1", "THE MONEY", "$600K · the score", "left behind",
		"THE PEOPLE", "2, 1 of them yours", "Ziggy (runner · day 3)", "1 deal broken by them", "best of them", "Ziggy, runner, fell on day 3",
		"THE CITY", "fear", "respect", "notoriety", "SCORE  $200K", "the offshore account $600K ÷ (1 + 2 bodies)",
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

// The reign (#227): a fast-forward stops the morning the city becomes
// yours (ReignBegan), the report's TIER section opens on the reign, the
// dashboard's STREET carries `reign dN` and the ALERTS the crown, and
// the walk-away dialog's third row takes it: the run ends a kingpin on
// that day, the summary's reached row reads the reign's first morning
// and the epilogue the reign lived.
func TestFastForwardStopsOnTheReign(t *testing.T) {
	m := richModelSeeded(t, 100, 30, 4) // one seed: the fixture's runners keep their corners to day 15 on it
	m.w.Home().Heat = 0
	w := m.w
	// Every faction at the table gone since day 1 and more than
	// kingpin_share of home's corners held: the tick dominant_days on
	// stamps the reign, and the fast-forward stops there.
	days := m.cfg.Rivals.Endings.DominantDays
	for _, r := range w.Rivals {
		r.Arrived, r.Fragmented, r.Muscle = 1, 1, 0
	}
	home := w.Home()
	n := int(m.cfg.Rivals.Endings.KingpinShare*float64(len(home.Corners))) + 1
	for i := 0; i < n; i++ {
		// A runner on each, hired by hand: a held corner nobody works
		// drifts back to the street inside dominant_days.
		c := &home.Corners[i]
		c.Owner, c.Faction, c.Runner, c.Enforcer, c.Since = game.OwnerPlayer, "", 0, 0, 1
		w.Crew.NextID++
		w.Crew.Members = append(w.Crew.Members, game.CrewMember{ID: w.Crew.NextID, Name: fmt.Sprintf("Runner %d", i), Role: "runner", Skill: 50, Units: 100, Loyalty: 80, Nerve: 50, Wage: 50, Hired: w.Day, Age: 30})
		if err := w.Post(c.ID, w.Crew.NextID); err != nil {
			t.Fatal(err)
		}
	}
	// The wages for the month, and no pile the police read into: $5M
	// drew 25 heat a night, and with targeted investigations on (#343)
	// the named hit on the corner you stand on (day 11, then 14) cost
	// the fixture its reign; the test is the reign's stop, not the law.
	w.Player.DirtyCash = 200_000
	if w.Reign != 0 || w.CanCrown() {
		t.Fatalf("a reign before the tick: %d", w.Reign)
	}
	for w.Day < days+1 {
		fast(t, m, 30)
		if strings.Contains(m.fastStop, "the city is yours") {
			break
		}
		closeMorning(t, m)
		m.Update(key("esc"))
	}
	day := w.Day
	if day != days+1 || w.Reign != day || !strings.Contains(m.fastStop, "the city is yours") {
		t.Fatalf("F stopped on day %d, reign %d, stop %q", w.Day, w.Reign, m.fastStop)
	}
	if m.mode == modeStage {
		m.Update(key("enter"))
	}
	if m.mode == modeCard {
		m.Update(key("enter"))
		m.Update(key("enter"))
	}
	if m.mode != modeReport {
		t.Fatalf("mode %v after the stop", m.mode)
	}
	view := stripANSI(m.View())
	for _, want := range []string{"REIGN: day 1 of the reign", "The city is yours"} {
		if !strings.Contains(view, want) {
			t.Errorf("the report lacks %q:\n%s", want, view)
		}
	}
	m.Update(key("esc"))
	m.Update(key("1"))
	view = stripANSI(m.View())
	if !strings.Contains(view, "reign d1") {
		t.Errorf("the dashboard lacks %q:\n%s", "reign d1", view)
	}
	found := false
	for _, a := range m.alerts() {
		if a.key == "the city is yours" && strings.Contains(stripANSI(a.text), "day 1 of the reign") {
			found = true
		}
	}
	if !found {
		t.Errorf("no alert for the reign: %+v", m.alerts())
	}
	// A second fast-forward does not stop on the reign again.
	fast(t, m, 3)
	if strings.Contains(m.fastStop, "the city is yours") {
		t.Fatalf("the second F stopped on the reign again on day %d: %q", w.Day, m.fastStop)
	}
	closeMorning(t, m)
	m.Update(key("esc"))
	// The crown: the third row, open, asked twice, ends the run a
	// kingpin on this day with the score as it stands. The fixture's
	// runners can be arrested overnight and a corner drift under the
	// share, which breaks the reign (TestReignBreaks has that); put it
	// back on for the crown if the nights took it.
	if w.Reign == 0 {
		w.Reign = day
	}
	w.Offshore, w.Stats.Bodies = 900_000, 2
	m.Update(key("1"))
	m.Update(key("w"))
	m.Update(key("3")) // a digit moves, enter turns the page (#500)
	m.Update(key("enter"))
	if m.mode != modeExit || m.exit.step != 1 || m.exit.cursor != 2 || !strings.Contains(stripANSI(m.View()), "TAKE THE CROWN?") {
		t.Fatalf("3 on the dialog: mode %v step %d cursor %d\n%s", m.mode, m.exit.step, m.exit.cursor, stripANSI(m.View()))
	}
	m.Update(key("y"))
	if w.Over == nil || w.Over.Cause != content.CauseKingpin || w.Over.Day != w.Day || m.mode != modeOver || w.Stats.Score != 300_000 {
		t.Fatalf("y: over %+v mode %v score %d", w.Over, m.mode, w.Stats.Score)
	}
	view = stripANSI(m.View())
	for _, want := range []string{"KINGPIN", fmt.Sprintf("reached Kingpin on day %d", w.Reign), fmt.Sprintf("The city was yours %s", plural(w.ReignDay(), "day"))} {
		if !strings.Contains(view, want) {
			t.Errorf("the summary lacks %q:\n%s", want, view)
		}
	}
	// Without the reign the row is closed and refused, saying what is
	// short in the kingpin plan's steps (#399).
	m = richModel(t, 100, 30)
	m.Update(key("1"))
	m.Update(key("w"))
	m.Update(key("3"))
	m.Update(key("enter"))
	if m.mode != modeExit || m.exit.step != 0 || !strings.Contains(m.exit.err, "corners held") || !strings.Contains(m.exit.err, "still standing") {
		t.Fatalf("the crown with no reign: mode %v step %d err %q", m.mode, m.exit.step, m.exit.err)
	}
}

// THE STORY is the run's weight, not its last quiet nights (#424): a
// run that lay low to its end told its story in five "Nothing to
// report" lines, heat lines the heaviest source; the summary's quiet
// templates never push out a raid, a loss or a defection.
func TestStoryIsNotTheQuietNights(t *testing.T) {
	m := richModel(t, 120, 40)
	w := m.w
	w.Journal = append(w.Journal,
		game.Headline{Day: 90, Source: "heat", Text: "Dawn raid in Eastside: cash and product seized"},
		game.Headline{Day: 95, Source: "crew", Text: "Lost Bird (arrested)."},
	)
	for d := 100; d < 110; d++ {
		city := w.Here().Name
		w.Journal = append(w.Journal,
			game.Headline{Day: d, Source: "heat", Text: "Nothing to report from " + city + " corners"},
			game.Headline{Day: d, Source: "heat", Text: "Quiet night in " + city})
	}
	w.Over = w.End(content.CauseIndicted, 110, "")
	lines := strings.Join(m.storyLines(), "\n")
	if strings.Contains(lines, "Nothing to report") || strings.Contains(lines, "Quiet night") || !strings.Contains(lines, "Dawn raid") || !strings.Contains(lines, "Lost Bird") {
		t.Fatalf("the story:\n%s", stripANSI(lines))
	}
}

// The reign has one day on every screen (#495): the ambitions screen's
// streak read `days it holds: 48 of 14` (days since the table fell)
// while the crown's row read `day 36 of the reign`; once the reign has
// begun the step reads the reign's own day, as the row and the plan do.
func TestTheReignReadsOneDay(t *testing.T) {
	m := richModel(t, 120, 40)
	w := m.w
	w.Day = max(w.Day, 100)
	for _, r := range w.Rivals {
		r.Arrived, r.Fragmented = 1, w.Day-48
	}
	home := w.Home()
	n := int(m.cfg.Rivals.Endings.KingpinShare*float64(len(home.Corners))) + 1
	for i := range n {
		home.Corners[i].Owner, home.Corners[i].Faction = game.OwnerPlayer, ""
	}
	w.Reign, w.ReignSlip = w.Day-35, 0
	want := fmt.Sprintf("day %d of the reign", w.ReignDay())
	if w.ReignDay() != 36 {
		t.Fatalf("reign day %d", w.ReignDay())
	}

	m.Update(key("1"))
	m.Update(key("a"))
	for i, a := range m.ambitions() {
		if a.ID == content.AmbitionCity {
			m.amb.cursor = i
			for _, st := range a.Steps {
				if st.ID == "streak" && stepWords(st) != want {
					t.Errorf("the crown's streak reads %q, want %q", stepWords(st), want)
				}
			}
		}
	}
	if view := stripANSI(m.View()); !strings.Contains(view, want) || strings.Contains(view, "of 14 days") {
		t.Errorf("the ambitions screen does not read %q:\n%s", want, view)
	}
	m.Update(key("esc"))

	m.Update(key("1"))
	m.Update(key("w"))
	if view := stripANSI(m.View()); m.mode != modeExit || !strings.Contains(view, want) {
		t.Errorf("the walk-away dialog does not read %q:\n%s", want, view)
	}
}
