package territory_test

import (
	"math"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
)

// TestHouseRobberyChance: a house's chance is the city's robbery_chance
// times the block's risk times house_risk (times the tree's
// robbery_mul), and a guard inside cuts it exactly as an enforcer cuts a
// corner's.
func TestHouseRobberyChance(t *testing.T) {
	cfg := content.MustLoad()
	w, s := world(t, cfg)
	tun := cfg.City.Territory
	w.Player.DirtyCash = 100_000
	if _, err := w.BuyHouse(game.HouseOffer{ID: "h", Name: "H", City: w.Home().ID, Corner: "docks", Capacity: 100, Price: 1}); err != nil {
		t.Fatal(err)
	}
	h := w.House("h")
	docks := w.Corner("docks")
	want := tun.RobberyChance * docks.Risk * cfg.Houses.Houses.HouseRisk
	if got := s.HouseRobberyChance(w, h); math.Abs(got-want) > 1e-9 {
		t.Fatalf("unguarded %v, want %v", got, want)
	}
	if err := w.Guard("h", 2); err != nil {
		t.Fatal(err)
	}
	cut := 1 - tun.EnforcerCut*(0.5+50.0/200)
	if got := s.HouseRobberyChance(w, h); math.Abs(got-want*cut) > 1e-9 {
		t.Fatalf("guarded %v, want %v", got, want*cut)
	}
	// The same cut an enforcer makes on a corner.
	if err := w.Post("docks", 2); err != nil {
		t.Fatal(err)
	}
	if got, corner := s.HouseRobberyChance(w, h), s.RobberyChance(w, docks); math.Abs(got-want) > 1e-9 || math.Abs(corner-tun.RobberyChance*docks.Risk*cut) > 1e-9 {
		t.Fatalf("after the post: house %v (want %v), corner %v", got, want, corner)
	}
}

// TestRentAndTheLandlord: the rent comes off clean cash every day after
// the day of the lease, a day it cannot be paid counts against the
// house, a paid day clears the count, and at rent_days unpaid the house
// is lost with its stock and its guard recalled; the lease is reported
// the day it is taken.
func TestRentAndTheLandlord(t *testing.T) {
	cfg := content.MustLoad()
	cfg.City.Territory.RobberyChance = 0 // the robbers are the next test's
	w, s := world(t, cfg)
	w.Player.DirtyCash, w.Player.CleanCash = 100_000, 25
	if _, err := w.BuyHouse(game.HouseOffer{ID: "h", Name: "H", City: w.Home().ID, Corner: "precinct", Capacity: 100, Price: 1, Rent: 10}); err != nil {
		t.Fatal(err)
	}
	w.AddStock(w.Home().ID, "weed", 40)
	if err := w.Guard("h", 2); err != nil {
		t.Fatal(err)
	}
	evs := step(w, s)
	if kinds(evs)["HouseBought"] != 1 || kinds(evs)["RentPaid"] != 0 || w.Player.CleanCash != 25 {
		t.Fatalf("the day of the lease: %v clean %d", kinds(evs), w.Player.CleanCash)
	}
	w.Today.HousesBought = nil
	for day := 1; day <= 2; day++ {
		evs = step(w, s)
		if kinds(evs)["RentPaid"] != 1 || w.Player.CleanCash != 25-10*day || w.House("h").Unpaid != 0 {
			t.Fatalf("day %d: %v clean %d unpaid %d", day, kinds(evs), w.Player.CleanCash, w.House("h").Unpaid)
		}
	}
	// $5 left: unpaid from here. One paid day in the middle clears the count.
	days := cfg.Houses.Houses.RentDays
	for i := 1; i < days; i++ {
		evs = step(w, s)
		if w.House("h") == nil || w.House("h").Unpaid != i || len(evs) == 0 {
			t.Fatalf("unpaid day %d: %+v", i, w.House("h"))
		}
	}
	w.Player.CleanCash = 10
	step(w, s)
	if w.House("h") == nil || w.House("h").Unpaid != 0 || w.Player.CleanCash != 0 {
		t.Fatalf("a paid day did not clear the count: %+v", w.House("h"))
	}
	var lost *events.HouseLost
	for i := 0; i < days; i++ {
		for _, e := range step(w, s) {
			if ev, ok := e.(events.HouseLost); ok {
				lost = &ev
			}
		}
	}
	if lost == nil || lost.House != "h" || lost.Units != 40 || w.House("h") != nil || w.Stock(w.Home().ID, "weed") != 0 || w.GuardOf(2) != nil || w.Stats.HousesLost != 1 || w.Stats.Rent != 30 {
		t.Fatalf("the landlord: %+v house %+v stock %d guard %v stats %+v", lost, w.House("h"), w.Stock(w.Home().ID, "weed"), w.GuardOf(2), w.Stats)
	}
}

// TestHouseRobberyTakesTheHouseAndTellsTheStreet: a house whose roll
// comes up loses robbery_stock of what it holds, nothing off the street,
// becomes known, and a house holding nothing is never "robbed".
func TestHouseRobberyTakesTheHouseAndTellsTheStreet(t *testing.T) {
	cfg := content.MustLoad()
	cfg.Houses.Houses.HouseRisk = 1000 // a robbery every night
	w, s := world(t, cfg)
	w.Player.DirtyCash, w.Player.CleanCash = 100_000, 100_000
	if _, err := w.BuyHouse(game.HouseOffer{ID: "h", Name: "H", City: w.Home().ID, Corner: "docks", Capacity: 100, Price: 1, Rent: 1}); err != nil {
		t.Fatal(err)
	}
	w.Today.HousesBought = nil
	if k := kinds(step(w, s)); k["HouseRobbed"] != 0 {
		t.Fatalf("an empty house was robbed: %v", k)
	}
	w.AddStock(w.Home().ID, "weed", 60)
	w.MoveStock(w.Home().ID, "h", game.Street, "weed", 20)
	evs := step(w, s)
	var robbed *events.HouseRobbed
	for _, e := range evs {
		if ev, ok := e.(events.HouseRobbed); ok {
			robbed = &ev
		}
	}
	if robbed == nil || robbed.StockLost["weed"] != 20 || robbed.Corner != "The Docks" || robbed.Guarded {
		t.Fatalf("robbed: %+v", robbed)
	}
	if kinds(evs)["HouseCompromised"] != 1 || !w.House("h").Known || w.House("h").Units() != 20 || w.Street(w.Home().ID, "weed") != 20 || w.House("h").Robbed != 1 {
		t.Fatalf("after: %+v street %d", w.House("h"), w.Street(w.Home().ID, "weed"))
	}
	// Known once is known: the next robbery says nothing new.
	if kinds(step(w, s))["HouseCompromised"] != 0 {
		t.Fatal("a known house was compromised again")
	}
}
