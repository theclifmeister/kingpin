package law_test

import (
	"math"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/sim"
	"github.com/theclifmeister/kingpin/internal/sim/law"
)

// TestDeedsPullTheirWay (#194, this sim's share): every deed held in a
// city is pressure there a day, before the fade, and none anywhere
// else; the forfeiture is a threshold and not a die: with the deeds
// held costing more than forfeit_ratio times what the fronts have
// washed the DA seizes the newest one a night (no refund; DeedSeized
// with the numbers, Law.Forfeited stamped for the heat sim's morning,
// Stats.DeedsSeized) until they are under it, and a run under the line
// never loses one; a run with no fronts and a deed (clean cash from a
// card, nothing washed) is the first to lose it, the first night; the
// election's dice are what they were with a deed in the world.
func TestDeedsPullTheirWay(t *testing.T) {
	cfg := content.MustLoad()
	cfg.Law.Law.TermDays = 0 // no election: the pressure alone
	s := law.New(cfg)
	tun := cfg.Law.Law
	deed := cfg.City.Deed
	w := sim.NewWorld(cfg, 6)
	home, hub := w.CityOrder[0], w.CityOrder[1]
	w.Player.CleanCash = 100_000_000
	w.Stats.Laundered = 100_000_000 // the money has a story

	// Pressure: two deeds at home, none in the hub.
	for _, c := range w.Cities[home].Corners[:2] {
		if err := w.BuyDeed(c.ID, 1000); err != nil {
			t.Fatal(err)
		}
	}
	for _, cid := range w.CityOrder {
		w.Cities[cid].Pressure, w.Cities[cid].Goodwill = 50, 0
	}
	s.Step(w, tick(w, 1))
	fade := func(p float64) float64 { return p - (p-tun.Baseline)*tun.Decay }
	if got, want := w.Cities[home].Pressure, fade(50+2*deed.Pressure); math.Abs(got-want) > 1e-9 {
		t.Fatalf("home pressure %v, want %v (two deeds)", got, want)
	}
	if got, want := w.Cities[hub].Pressure, fade(50); math.Abs(got-want) > 1e-9 {
		t.Fatalf("hub pressure %v, want %v (no deed)", got, want)
	}
	if w.Stats.DeedsSeized != 0 || w.Law.Forfeited != 0 {
		t.Fatalf("under the line, a seizure: %+v %d", w.Stats, w.Law.Forfeited)
	}

	// The line: forfeit_ratio x washed.
	w.Stats.Laundered = 10_000
	if got := s.DeedLimit(w); got != int(math.Floor(deed.ForfeitRatio*10_000)) {
		t.Fatalf("limit %d, want %v x 10000", got, deed.ForfeitRatio)
	}
	if s.Forfeits(w) {
		t.Fatalf("$2,000 in deeds against a line of %d forfeits", s.DeedLimit(w))
	}
	// Over it: the newest goes, one a night, until under.
	third := w.Cities[home].Corners[2]
	w.Day = 5
	if err := w.BuyDeed(third.ID, s.DeedLimit(w)); err != nil {
		t.Fatal(err)
	}
	if !s.Forfeits(w) {
		t.Fatal("over the line and not forfeiting")
	}
	clean := w.Player.CleanCash
	tk := tick(w, 6)
	s.Step(w, tk)
	var seized *events.DeedSeized
	for _, e := range tk.Events() {
		if ev, ok := e.(events.DeedSeized); ok {
			seized = &ev
		}
	}
	if seized == nil || seized.Corner != third.ID || seized.Price != s.DeedLimit(w) || seized.Washed != 10_000 || seized.Limit != s.DeedLimit(w) || seized.Spent != 2000+s.DeedLimit(w) {
		t.Fatalf("DeedSeized %+v", seized)
	}
	if w.Corner(third.ID).Deed != nil || w.Stats.DeedsSeized != 1 || w.Law.Forfeited != 6 || w.Player.CleanCash != clean {
		t.Fatalf("after the seizure: deed %+v seized %d forfeited %d clean %d (was %d)", w.Corner(third.ID).Deed, w.Stats.DeedsSeized, w.Law.Forfeited, w.Player.CleanCash, clean)
	}
	if s.Forfeits(w) {
		t.Fatalf("still over the line with $%d held against %d", w.DeedValue(), s.DeedLimit(w))
	}
	tk = tick(w, 7)
	s.Step(w, tk)
	if kinds(tk)["DeedSeized"] != 0 || w.Stats.DeedsSeized != 1 || w.Law.Forfeited != 6 {
		t.Fatalf("a second night under the line seized again: %v", kinds(tk))
	}
	// Two over the line: two nights, the newest first.
	w.Stats.Laundered = 0
	tk = tick(w, 8)
	s.Step(w, tk)
	if kinds(tk)["DeedSeized"] != 1 || w.DeedValue() != 1000 || w.Cities[home].Corners[1].Deed != nil {
		t.Fatalf("the first of two nights: %v, held %d", kinds(tk), w.DeedValue())
	}
	tk = tick(w, 9)
	s.Step(w, tk)
	if kinds(tk)["DeedSeized"] != 1 || w.DeedValue() != 0 || w.Stats.DeedsSeized != 3 {
		t.Fatalf("the second of two nights: %v, held %d", kinds(tk), w.DeedValue())
	}

	// No fronts, a deed off a card's clean cash: the first to lose it.
	w2 := sim.NewWorld(cfg, 6)
	w2.Player.CleanCash = 50_000
	if err := w2.BuyDeed(w2.Cities[home].Corners[0].ID, 50_000); err != nil {
		t.Fatal(err)
	}
	tk = tick(w2, 1)
	s.Step(w2, tk)
	if kinds(tk)["DeedSeized"] != 1 || w2.DeedValue() != 0 {
		t.Fatalf("a deed with nothing washed survived the night: %v", kinds(tk))
	}

	// The table boxed: no pressure, no forfeiture, whatever is held.
	off := *cfg
	off.City.Deed = content.DeedTuning{}
	s2 := law.New(&off)
	w3 := sim.NewWorld(&off, 6)
	w3.Player.CleanCash = 50_000
	w3.Cities[home].Corners[0].Deed = &game.Deed{Bought: 0, Price: 50_000}
	w3.Cities[home].Pressure, w3.Cities[home].Goodwill = 50, 0
	tk = tick(w3, 1)
	s2.Step(w3, tk)
	if kinds(tk)["DeedSeized"] != 0 || math.Abs(w3.Cities[home].Pressure-fade(50)) > 1e-9 || s2.Forfeits(w3) {
		t.Fatalf("the boxed table: %v pressure %v", kinds(tk), w3.Cities[home].Pressure)
	}
}
