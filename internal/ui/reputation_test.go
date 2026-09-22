package ui

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/sim/crew"
	"github.com/theclifmeister/kingpin/internal/sim/heat"
	"github.com/theclifmeister/kingpin/internal/sim/market"
	"github.com/theclifmeister/kingpin/internal/sim/rivals"
)

// What your name buys (#233): the percentages the pane prints for fear
// 62, respect 40 and notoriety 70 are the numbers the sims apply
// (rivals.Sim.ClaimPace and PushPace, market.Sim.SupplierRatio,
// crew.Sim.LoyaltyLoss and HireFee, heat.Sim.PersonalHeat and Floor),
// read off the same world; the section is absent with nothing over
// zero; the rivals screen's header and the pool's copy carry it; and
// the dashboard, the rivals and the crew screens fit at the three
// sizes with the section on.
func TestEffectsInWordsAgreeWithTheSims(t *testing.T) {
	m := richModel(t, 100, 30)
	w := m.w
	w.Player.Reputation = game.Reputation{}
	if secs := m.nameSection(); len(secs) != 0 {
		t.Fatalf("a section with nothing over zero: %+v", secs)
	}
	w.Player.Reputation = game.Reputation{Fear: 62, Respect: 40, Notoriety: 70}
	secs := m.nameSection()
	if len(secs) != 1 || secs[0].title != "YOUR NAME" {
		t.Fatalf("the section: %+v", secs)
	}
	text := stripANSI(strings.Join(secs[0].lines, " "))
	pct := func(v float64) string { return fmt.Sprintf("%.0f%%", math.Round(v*100)) }
	cfg := m.cfg
	rv, hs, cs := rivals.New(cfg), heat.New(cfg), crew.New(cfg)
	mk, err := market.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	// The same world with nothing on the name, for the ratios.
	plain := *w
	plain.Player.Reputation = game.Reputation{}
	for _, want := range []struct {
		name string
		pct  string
	}{
		{"claims", pct(1 - rv.ClaimPace(w))},
		{"pushes", pct(1 - rv.PushPace(w)/rv.PushPace(&plain))},
		{"the connect", pct(1 - mk.SupplierRatio(w, &w.Suppliers[0])/mk.SupplierRatio(&plain, &plain.Suppliers[0]))},
		{"loyalty", pct(1 - cs.LoyaltyLoss(w)/cs.LoyaltyLoss(&plain))},
		{"hire fees", pct(1 - float64(cs.HireFee(w, 1000))/float64(cs.HireFee(&plain, 1000)))},
		{"personal heat", pct(hs.PersonalHeat(w)/hs.PersonalHeat(&plain) - 1)},
	} {
		if !strings.Contains(text, want.pct) {
			t.Errorf("the pane lacks %s's %s:\n%s", want.name, want.pct, text)
		}
	}
	if floor := fmt.Sprintf("under %.0f", hs.Floor(w)); !strings.Contains(text, floor) {
		t.Errorf("the pane lacks the floor %q:\n%s", floor, text)
	}
	for _, l := range secs[0].lines {
		if lipglossWidth(l) > paneTextW {
			t.Errorf("a line is wider than the pane: %q", stripANSI(l))
		}
	}
	// The rivals screen's header and the pool's copy.
	m.Update(key("8"))
	if view := stripANSI(m.View()); !strings.Contains(view, "your name: fear 62") {
		t.Errorf("the rivals screen lacks the name line:\n%s", view)
	}
	m.Update(key("4"))
	m.crewCursor = len(w.Crew.Members) // the first face in the pool
	if len(w.Crew.Candidates) > 0 {
		if view := stripANSI(m.View()); !strings.Contains(view, "asked to work for you") {
			t.Errorf("the pool's copy lacks the ask at notoriety 70:\n%s", view)
		}
	}
	for _, size := range [][2]int{{80, 24}, {100, 30}, {120, 40}} {
		mm := richModel(t, size[0], size[1])
		mm.w.Player.Reputation = game.Reputation{Fear: 62, Respect: 40, Notoriety: 70}
		for _, screen := range []string{"1", "8", "4"} {
			mm.Update(key(screen))
			assertFits(t, mm.View(), size[0], size[1], "screen "+screen+" with the name on")
		}
	}
}

func lipglossWidth(s string) int { return lipgloss.Width(s) }
