package territory_test

import (
	"math"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
)

// TestDeedsPullTheirWay (#194, this sim's share): the price is days of
// the corner's street trade at today's prices and 0 with the table
// boxed; the rent is rent of the price, paid clean at the top of the
// step from the night of the purchase, deed by deed, and reported; a
// deed cuts the robbery chance on its corner by robbery_mul, and the
// chance of a house on that block by the same, and moves no other
// corner's; the purchase is reported the night it is made, and the one
// that brings a city's count to headline_deeds makes the paper.
func TestDeedsPullTheirWay(t *testing.T) {
	cfg := content.MustLoad()
	cfg.City.Territory.RobberyChance = 0.04
	cfg.City.Deed.HeadlineDeeds = 2
	w, s := world(t, cfg)
	tun := cfg.City.Deed
	home := w.Home().ID
	start := w.Corner(cfg.City.Territory.Start)
	docks := w.Corner("docks")

	// The price and the rent.
	want := int(math.Round(tun.Days * w.CornerTrade(*start)))
	if got := s.DeedPrice(w, *start); got != want || got <= 0 {
		t.Fatalf("price %d, want %d", got, want)
	}
	if (content.DeedTuning{}).On() {
		t.Fatal("an empty table is on")
	}
	off := *cfg
	off.City.Deed = content.DeedTuning{}
	_, boxed := world(t, &off)
	if got := boxed.DeedPrice(w, *start); got != 0 {
		t.Fatalf("price with the table boxed %d, want 0", got)
	}
	rent := int(math.Round(tun.Rent * float64(want)))
	if got := s.DeedRent(&game.Deed{Price: want}); got != rent || got <= 0 {
		t.Fatalf("rent %d, want %d", got, rent)
	}
	if got := s.DeedRent(nil); got != 0 {
		t.Fatalf("rent of no deed %d", got)
	}

	// The robbery chance, before and after, on the deeded corner and
	// not the other.
	before, beforeDocks := s.RobberyChance(w, start), s.RobberyChance(w, docks)
	w.Player.DirtyCash = 100_000
	if _, err := w.BuyHouse(game.HouseOffer{ID: "h", Name: "H", City: home, Corner: start.ID, Capacity: 100, Price: 1}); err != nil {
		t.Fatal(err)
	}
	house := s.HouseRobberyChance(w, w.House("h"))
	w.Player.CleanCash = want + 10
	if err := w.BuyDeed(start.ID, want); err != nil {
		t.Fatal(err)
	}
	if got := s.RobberyChance(w, start); math.Abs(got-before*tun.RobberyMul) > 1e-9 {
		t.Fatalf("robbery on the deeded corner %v, want %v x %v", got, before, tun.RobberyMul)
	}
	if got := s.RobberyChance(w, docks); got != beforeDocks {
		t.Fatalf("robbery on the docks moved: %v, was %v", got, beforeDocks)
	}
	if got := s.HouseRobberyChance(w, w.House("h")); math.Abs(got-house*tun.RobberyMul) > 1e-9 {
		t.Fatalf("robbery at the house on the block %v, want %v x %v", got, house, tun.RobberyMul)
	}

	// The night: the purchase reported, the rent paid, from the first
	// night.
	clean := w.Player.CleanCash
	evs := step(w, s)
	if k := kinds(evs); k["DeedBought"] != 1 || k["DeedRent"] != 1 || k["DeedsBought"] != 0 {
		t.Fatalf("the first night's events: %v", k)
	}
	if w.Player.CleanCash != clean+rent || w.Stats.DeedRent != rent {
		t.Fatalf("clean %d after the rent, want %d; stat %d", w.Player.CleanCash, clean+rent, w.Stats.DeedRent)
	}
	for _, e := range evs {
		switch ev := e.(type) {
		case events.DeedBought:
			if ev.Corner != start.ID || ev.Price != want || ev.Rent != rent || ev.Count != 1 || ev.City != home {
				t.Fatalf("DeedBought %+v", ev)
			}
		case events.DeedRent:
			if ev.Amount != rent || ev.Deeds != 1 {
				t.Fatalf("DeedRent %+v", ev)
			}
		}
	}
	// A second deed, on a corner nobody's: the count reaches the line
	// and makes the paper; two rents from then on.
	w.ClearToday(w.Day) // the clock's, between the days
	price := s.DeedPrice(w, *docks)
	w.Player.CleanCash = price
	if err := w.BuyDeed(docks.ID, price); err != nil {
		t.Fatal(err)
	}
	evs = step(w, s)
	if k := kinds(evs); k["DeedBought"] != 1 || k["DeedsBought"] != 1 || k["DeedRent"] != 1 {
		t.Fatalf("the second night's events: %v", k)
	}
	for _, e := range evs {
		switch ev := e.(type) {
		case events.DeedsBought:
			if ev.Count != 2 || ev.Corner != docks.ID {
				t.Fatalf("DeedsBought %+v", ev)
			}
		case events.DeedRent:
			if ev.Deeds != 2 || ev.Amount != rent+s.DeedRent(docks.Deed) {
				t.Fatalf("DeedRent %+v", ev)
			}
		}
	}
	// A quiet night with no deed emits nothing of the kind.
	w2, s2 := world(t, cfg)
	if k := kinds(step(w2, s2)); k["DeedBought"]+k["DeedsBought"]+k["DeedRent"] != 0 || w2.Stats.DeedRent != 0 {
		t.Fatalf("a run with no deed: %v", k)
	}
}
