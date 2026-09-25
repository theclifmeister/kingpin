package ui

import (
	"strings"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/game"
)

// The ways out as choices (#478, docs/endings.md): every walk-away row
// says what it scores, the vanish plan is the whole Legal chain, a war
// you declare short of muscle says what losing it costs, and the
// ledger's sweep is the offshore account's standing order.

// TestEveryWayOutSaysItsScore: each of the walk-away dialog's rows
// carries `scores  the account: $X`, the same for all four, and names
// the bodies where there are some.
func TestEveryWayOutSaysItsScore(t *testing.T) {
	m := richModel(t, 120, 40)
	m.w.Offshore = 420_000
	m.Update(key("1"))
	m.Update(key("w"))
	v := stripANSI(m.View())
	if n := strings.Count(v, "the account: $420,000"); n != len(m.exitRows()) {
		t.Fatalf("%d rows say the score, want %d:\n%s", n, len(m.exitRows()), v)
	}
	m.Update(key("esc"))
	m.w.Stats.Bodies = 1
	m.Update(key("w"))
	if v := stripANSI(m.View()); !strings.Contains(v, "the account over 1 + 1 body: $210,000") {
		t.Fatalf("the bodies are not in the score:\n%s", v)
	}
}

// TestVanishPlanStartsWithTheLawyer: the plan's first step is the
// lawyer on call, paid in dirty cash, before the retainer and the
// identity (the tree's chain).
func TestVanishPlanStartsWithTheLawyer(t *testing.T) {
	m := newTestModel(t, 120, 40)
	for _, a := range m.ambitions() {
		if a.ID != content.AmbitionVanish {
			continue
		}
		if len(a.Steps) != 3 || a.Steps[0].ID != "lawyer" || a.Steps[0].Unit != game.UnitDirty || a.Steps[1].ID != "retainer" || a.Steps[2].ID != "identity" {
			t.Fatalf("the vanish plan's steps: %+v", a.Steps)
		}
		if got := stepWords(a.Steps[0]); !strings.Contains(got, "dirty") {
			t.Fatalf("the lawyer's step reads %q", got)
		}
		return
	}
	t.Fatal("no vanish plan")
}

// TestWarWarnsShortOfMuscle: with fewer enforcers on the payroll than
// taken_out_muscle, the war declaration and the rivals pane of the
// faction you are at war with say a lost war ends the run; with muscle
// enough neither does.
func TestWarWarnsShortOfMuscle(t *testing.T) {
	m := richModel(t, 120, 40)
	need := m.cfg.Rivals.Endings.TakenOutMuscle
	enforcers := func() int { return m.w.Crew.OnPayroll(game.RoleEnforcer) }
	for enforcers() < need {
		id := m.w.Crew.NextID + 1
		m.w.Crew.NextID = id
		m.w.Crew.Members = append(m.w.Crew.Members, game.CrewMember{ID: id, Name: "Muscle", Role: game.RoleEnforcer, Skill: 50, Loyalty: 80, Nerve: 50, Wage: 50})
	}
	const warn = "a war you lose ends the run"
	if line := m.warMuscleLine(); line != "" {
		t.Fatalf("with %d enforcers: %q", enforcers(), line)
	}
	var kept []game.CrewMember
	seen := 0
	for _, c := range m.w.Crew.Members {
		if c.Role == game.RoleEnforcer {
			if seen++; seen >= need {
				continue
			}
		}
		kept = append(kept, c)
	}
	m.w.Crew.Members = kept
	if enforcers() != need-1 {
		t.Fatalf("%d enforcers, want %d", enforcers(), need-1)
	}
	if line := m.warMuscleLine(); !strings.Contains(line, warn) {
		t.Fatalf("short of muscle: %q", line)
	}
	m.Update(key("8"))
	if !strings.Contains(stripANSI(m.warConfirm()), warn) {
		t.Fatalf("the declaration does not warn:\n%s", stripANSI(m.warConfirm()))
	}
	m.w.War = m.faction().Faction()
	var pane []string
	for _, s := range m.rivalsDetails() {
		pane = append(pane, s.lines...)
	}
	if !strings.Contains(stripANSI(strings.Join(pane, " ")), "ends the run") {
		t.Fatalf("the rivals pane does not warn at war:\n%s", stripANSI(strings.Join(pane, "\n")))
	}
}

// TestSweepFromTheLedger: S on the ledger opens the sweep dialog; a
// line typed and enter turns the sweep on at it; S again and x turn it
// off. Nothing moves until the night.
func TestSweepFromTheLedger(t *testing.T) {
	m := richModel(t, 120, 40)
	m.w.Player.CleanCash = 200_000
	clean, offshore := m.w.Player.CleanCash, m.w.Offshore
	m.Update(key("7"))
	m.Update(key("S"))
	if m.mode != modeSweep {
		t.Fatalf("S on the ledger: mode %v", m.mode)
	}
	if v := stripANSI(m.View()); !strings.Contains(v, "SWEEP OFFSHORE") || !strings.Contains(v, "off") {
		t.Fatalf("the dialog:\n%s", v)
	}
	for _, k := range []string{"3", "0", "0", "0", "0"} {
		m.Update(key(k))
	}
	if v := stripANSI(m.View()); !strings.Contains(v, "$50,000 moves") {
		t.Fatalf("the dialog does not say tonight's move:\n%s", v)
	}
	m.Update(key("enter"))
	if m.mode != modePlay || m.w.Laundering.Sweep != (game.OffshoreSweep{On: true, Keep: 30_000}) {
		t.Fatalf("enter: mode %v sweep %+v", m.mode, m.w.Laundering.Sweep)
	}
	if m.w.Player.CleanCash != clean || m.w.Offshore != offshore {
		t.Fatalf("the sweep moved money before the night: clean %d offshore %d", m.w.Player.CleanCash, m.w.Offshore)
	}
	m.Update(key("S"))
	m.Update(key("x"))
	if m.mode != modePlay || m.w.Laundering.Sweep.On {
		t.Fatalf("x: mode %v sweep %+v", m.mode, m.w.Laundering.Sweep)
	}
}

// TestCrownSaysWhoIsYetToCome: the closed crown's shortfall counts a
// crew still in the wings apart from one on the street: a seat yet to
// arrive blocks Dominant() as a crew standing does, and read as one it
// looked like a crew to fight.
func TestCrownSaysWhoIsYetToCome(t *testing.T) {
	m := richModel(t, 120, 40)
	if got := m.crownShort(); strings.Contains(got, "yet to arrive") {
		t.Fatalf("every crew is here, and the shortfall reads %q", got)
	}
	m.w.Rivals[len(m.w.Rivals)-1].Arrived = 0 // back in the wings
	if got := m.crownShort(); !strings.Contains(got, "1 crew yet to arrive") {
		t.Fatalf("the crown's shortfall: %q", got)
	}
}
