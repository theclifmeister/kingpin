package ui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/theclifmeister/kingpin/internal/engine"
	"github.com/theclifmeister/kingpin/internal/format"
	"github.com/theclifmeister/kingpin/internal/game"
)

// The loyalty counts agree with the roster (#522): "N near or under the
// line" counts every member in the band (it read "1 near the line" with
// all four under 30), and a member a hair under a line reads under it on
// the roster as in the stop (29.6 is 29, not 30).
func TestLoyaltyCountsAgree(t *testing.T) {
	m := newTestModel(t, 120, 40)
	w := m.w
	w.Player.DirtyCash = 100000
	skim := m.cfg.Crew.Crew.SkimThreshold
	w.Crew.Members = nil
	for i, name := range []string{"Ace", "Bo", "Cy", "Di"} {
		w.Crew.Members = append(w.Crew.Members, game.CrewMember{ID: i + 1, Name: name, Role: game.RoleRunner, Skill: 50, Loyalty: skim - 3 - float64(i), Nerve: 90, Wage: 50})
	}
	if got := m.crewTrouble(); !strings.HasPrefix(got, "4 near or under the line") {
		t.Errorf("the crew trouble with all four under the skim line reads %q", got)
	}

	c := game.CrewMember{ID: 9, Name: "Yaya", Role: game.RoleRunner, Loyalty: skim - 0.4}
	if got := loyaltyShown(c.Loyalty); got != skim-1 {
		t.Errorf("%.1f loyalty under the %.0f line shows as %.0f", c.Loyalty, skim, got)
	}
	c.Loyalty = skim + 0.4
	if got := loyaltyShown(c.Loyalty); got != skim {
		t.Errorf("%.1f loyalty over the %.0f line shows as %.0f", c.Loyalty, skim, got)
	}

	m = newTestModel(t, 120, 40)
	lt := withLieutenant(t, m)
	lt.Loyalty = m.rules.Crew.FlipLine() - 0.4
	for _, a := range m.alertsOf(engine.AlertCrewLine) {
		if strings.HasPrefix(a.why, lt.Name) && strings.Contains(a.why, "under the") {
			if want := "at 29 loyalty"; !strings.Contains(a.why, want) {
				t.Errorf("the stop %q, want %q beside the roster's figure", a.why, want)
			}
		}
	}
}

// The fire dialog says what firing costs the rest (#522: firing a named
// informant said "the rest lose 8 loyalty", and nobody's moved): the
// snitch the investigation named costs nothing, anyone else the tuning's
// fire_loyalty, with the snitch's exception said.
func TestFireDialogSaysTheCost(t *testing.T) {
	m := newTestModel(t, 120, 40)
	w := m.w
	w.Crew.Members = []game.CrewMember{
		{ID: 1, Name: "Rat", Role: game.RoleRunner, Skill: 50, Loyalty: 20, Nerve: 10, Wage: 50, Informant: true},
		{ID: 2, Name: "Ace", Role: game.RoleRunner, Skill: 50, Loyalty: 80, Nerve: 90, Wage: 50},
	}
	w.Crew.Exposed = 1
	if got := m.fireCost(w.Crew.Member(1)); !strings.Contains(got, "no loyalty") {
		t.Errorf("firing the named snitch reads %q", got)
	}
	got := m.fireCost(w.Crew.Member(2))
	if want := fmt.Sprintf("lose %.0f loyalty", m.rules.Crew.Tuning().FireLoyalty); !strings.Contains(got, want) || !strings.Contains(got, "unless") {
		t.Errorf("firing Ace reads %q, want %q and the snitch's exception", got, want)
	}
}

// The fire dialog names who the firing would take to the walk line
// (#532): every other member within fire_loyalty of quit_threshold, by
// name and loyalty; nobody further off, and nobody when the one fired is
// the named snitch, who costs nothing. A playtest fired a cheap hand and
// two runners under the line defected with no word first.
func TestFireNamesWhoWouldWalk(t *testing.T) {
	m := newTestModel(t, 120, 40)
	w := m.w
	tun := m.rules.Crew.Tuning()
	w.Crew.Members = []game.CrewMember{
		{ID: 1, Name: "Cheap", Role: game.RoleRunner, Skill: 20, Loyalty: 60, Nerve: 50, Wage: 20},
		{ID: 2, Name: "Dre", Role: game.RoleRunner, Skill: 50, Loyalty: tun.QuitThreshold + tun.FireLoyalty, Nerve: 50, Wage: 50},
		{ID: 3, Name: "Kit", Role: game.RoleRunner, Skill: 50, Loyalty: tun.QuitThreshold + 2.5, Nerve: 50, Wage: 50},
		{ID: 4, Name: "Ace", Role: game.RoleRunner, Skill: 50, Loyalty: tun.QuitThreshold + tun.FireLoyalty + 1, Nerve: 50, Wage: 50},
	}
	got := m.fireWalkLine(w.Crew.Member(1))
	for _, want := range []string{fmt.Sprintf("Dre (%.0f)", tun.QuitThreshold+tun.FireLoyalty), fmt.Sprintf("Kit (%.0f)", tun.QuitThreshold+2), fmt.Sprintf("walk line (%.0f)", tun.QuitThreshold)} {
		if !strings.Contains(got, want) {
			t.Errorf("firing Cheap reads %q, want %q", got, want)
		}
	}
	if strings.Contains(got, "Ace") || strings.Contains(got, "Cheap") {
		t.Errorf("firing Cheap names Ace, clear of the line, or Cheap: %q", got)
	}
	m.subjectID = 1
	if box := stripANSI(m.fireConfirm()); !strings.Contains(box, "Dre") || !strings.Contains(box, "walk") {
		t.Errorf("the fire confirmation does not name who would walk:\n%s", box)
	}
	w.Crew.Exposed = 1
	if got := m.fireWalkLine(w.Crew.Member(1)); got != "" {
		t.Errorf("firing the named snitch warns %q", got)
	}
	w.Crew.Exposed = 0
	if got := m.fireWalkLine(w.Crew.Member(4)); strings.Contains(got, "Ace") || !strings.Contains(got, "Dre") {
		t.Errorf("firing Ace reads %q", got)
	}
}

// A greedy lieutenant's take is said as theirs (#521): their details say
// what the temper cost last night once it shows, and the crew screen's
// warning never blames loyalty for it.
func TestGreedyTakeOnTheCrewScreen(t *testing.T) {
	m := newTestModel(t, 120, 40)
	lt := withLieutenant(t, m)
	lt.Personality, lt.Observed, lt.Extra = "greedy", true, 755
	got := stripANSI(strings.Join(m.lieutenantLines(*lt), "\n"))
	share := format.Pct(m.cfg.Crew.Lieutenant.Temper("greedy").Skim, 0)
	if !strings.Contains(got, "takes "+share+" more") || !strings.Contains(got, "$755 last night on top of the cut") {
		t.Errorf("the greedy lieutenant's lines:\n%s", got)
	}
	if w := m.crewWarning(); w != "" {
		t.Errorf("a greedy take with nobody skimming warns %q", w)
	}
}
