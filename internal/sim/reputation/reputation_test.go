package reputation_test

import (
	"math"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/sim/reputation"
)

func world() *game.World {
	return game.NewWorld(1, []game.StartingCity{{ID: "test", Name: "Test", Products: []game.StartingProduct{{ID: "weed", Name: "Weed", Price: 20, Demand: 60}}}}, 500, 100)
}

// step runs one day with the given events already emitted, as if by the
// sims before this one, and returns what the reputation sim emitted.
func step(s *reputation.Sim, w *game.World, day int, before ...events.Event) []events.Event {
	t := &game.Tick{Day: day, RNG: game.RNGFor(1, day)}
	for _, e := range before {
		t.Emit(e)
	}
	s.Step(w, t)
	return t.Events()[len(before):]
}

// Every source lands on the axis the config says, at the amount it says,
// less the day's fade.
func TestSourcesAddUp(t *testing.T) {
	all := content.MustLoad()
	cfg := all.Reputation
	s := reputation.New(all)
	w := world()
	w.Journal = []game.Headline{
		{Day: 0, Source: "heat", Text: "about you"},
		{Day: 0, Source: "news", Text: "about the weather"},
		{Day: 0, Source: "market", Text: "about the city"},
	}
	step(s, w, 1,
		events.CornerStruck{Day: 1, Taken: true},
		events.CornerStruck{Day: 1},
		events.RivalPushed{Day: 1},
		events.CrewPaid{Day: 1, Pay: events.PayGenerous, Wages: 100},
		events.CrewPaidOff{Day: 1, Cost: 100},
		events.PlayerSold{Day: 1, Wanted: 500, Sold: 500},
	)
	fade := 1 - cfg.Reputation.Decay
	want := game.Reputation{
		Fear:      (cfg.Fear.StrikeTaken + cfg.Fear.StrikeHeld + cfg.Fear.PushHeld) * fade,
		Respect:   (cfg.Respect.GenerousPay + cfg.Respect.Payoff) * fade,
		Notoriety: (cfg.Notoriety.StrikeTaken + cfg.Notoriety.StrikeHeld + 500/cfg.Notoriety.Units + cfg.Notoriety.Headline) * fade,
	}
	got := w.Player.Reputation
	for _, a := range game.Axes {
		if math.Abs(*got.Axis(a)-*want.Axis(a)) > 1e-9 {
			t.Fatalf("%s: got %.4f want %.4f", a, *got.Axis(a), *want.Axis(a))
		}
	}

	// Short pay is a mark against you; a fair day in full is nothing.
	w = world()
	step(s, w, 1, events.CrewPaid{Day: 1, Pay: events.PayGenerous, Wages: 50, Short: 50})
	if w.Player.Reputation.Respect != 0 || w.Player.Reputation.Fear != 0 {
		t.Fatalf("short pay: %+v", w.Player.Reputation)
	}
	w.Player.Reputation.Respect = 50
	step(s, w, 2, events.CrewPaid{Day: 2, Pay: events.PayGenerous, Wages: 50, Short: 50})
	if got := w.Player.Reputation.Respect; math.Abs(got-(50+cfg.Respect.ShortPay)*fade) > 1e-9 {
		t.Fatalf("short pay took respect to %.4f", got)
	}
	w.Player.Reputation.Respect = 50
	step(s, w, 3, events.CrewPaid{Day: 3, Pay: events.PayFair, Wages: 50})
	if got := w.Player.Reputation.Respect; math.Abs(got-50*fade) > 1e-9 {
		t.Fatalf("fair pay took respect to %.4f", got)
	}
}

// Nothing happening fades every axis toward the baseline and never past
// it, and the axes stay in 0..100 however much is fed in.
func TestFadeAndBounds(t *testing.T) {
	all := content.MustLoad()
	cfg := all.Reputation
	s := reputation.New(all)
	w := world()
	w.Player.Reputation = game.Reputation{Fear: 100, Respect: 50, Notoriety: 1}
	for d := 1; d <= 500; d++ {
		step(s, w, d)
	}
	r := w.Player.Reputation
	for _, a := range game.Axes {
		if v := *r.Axis(a); v < cfg.Reputation.Baseline || v > cfg.Reputation.Baseline+1 {
			t.Fatalf("%s faded to %.3f, baseline %.0f", a, v, cfg.Reputation.Baseline)
		}
	}
	w = world()
	for d := 1; d <= 100; d++ {
		step(s, w, d, events.CornerStruck{Day: d, Taken: true})
	}
	if r := w.Player.Reputation; r.Fear > 100 || r.Fear < 99 {
		t.Fatalf("100 won corners: fear %.2f", r.Fear)
	}
}

// Past the street's attention the three shrink to fit, in proportion.
func TestCrowding(t *testing.T) {
	all := content.MustLoad()
	cfg := all.Reputation
	s := reputation.New(all)
	w := world()
	w.Player.Reputation = game.Reputation{Fear: 100, Respect: 100, Notoriety: 100}
	step(s, w, 1)
	r := w.Player.Reputation
	if sum := r.Fear + r.Respect + r.Notoriety; math.Abs(sum-cfg.Reputation.Total) > 1e-9 {
		t.Fatalf("crowded to %.3f, want %.0f: %+v", sum, cfg.Reputation.Total, r)
	}
	if r.Fear != r.Respect || r.Respect != r.Notoriety {
		t.Fatalf("crowding was not proportional: %+v", r)
	}
	if r.Fear > 70 && r.Respect > 70 && r.Notoriety > 70 {
		t.Fatalf("all three above 70: %+v", r)
	}
}

// Crossing a band line either way is an event with the axis on it, and
// only crossing is: a day inside a band is quiet.
func TestBandCrossings(t *testing.T) {
	all := content.MustLoad()
	cfg := all.Reputation
	s := reputation.New(all)
	band := cfg.Reputation.Band
	w := world()
	w.Player.Reputation.Fear = band - 1
	var up, down, quiet int
	for _, e := range step(s, w, 1, events.CornerStruck{Day: 1, Taken: true}) {
		if rs, ok := e.(events.ReputationShifted); ok && rs.Axis == "fear" && rs.Up() {
			up++
		}
	}
	quiet = len(step(s, w, 2))
	w.Player.Reputation.Fear = band + 0.1
	for _, e := range step(s, w, 3) {
		if rs, ok := e.(events.ReputationShifted); ok && rs.Axis == "fear" && !rs.Up() {
			down++
		}
	}
	if up != 1 || down != 1 || quiet != 0 {
		t.Fatalf("up %d down %d quiet %d", up, down, quiet)
	}
}
