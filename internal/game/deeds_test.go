package game

import (
	"bytes"
	"encoding/gob"
	"errors"
	"testing"
)

// The property (#194): BuyDeed takes clean cash only, one deed a block,
// none at a price of 0 (nothing on sale), and refuses over the pile
// naming the shortfall; a deed goes on the corner whoever holds it,
// is counted (Stats.Deeds, DeedCash, Today.DeedsBought), valued at cost
// by NetWorth, and stays on the corner through a hand-over; SeizeDeed
// takes it off and counts it; NewestDeed is the last bought; the trade
// is the corner's full share of the street at today's prices.
func TestBuyDeed(t *testing.T) {
	w := testWorld()
	w.Day = 3
	// The trade: demand 5 x price 10 on a standard corner; the docks
	// are 1.5 with a taste of 2 for a.
	if got := w.CornerTrade(*w.Corner("home")); got != 50 {
		t.Fatalf("home's trade %v, want 50", got)
	}
	if got := w.CornerTrade(*w.Corner("docks")); got != 150 {
		t.Fatalf("the docks' trade %v, want 150", got)
	}
	if err := w.BuyDeed("nowhere", 100); !errors.Is(err, ErrNoCorner) {
		t.Fatalf("a corner that is not: %v", err)
	}
	if err := w.BuyDeed("home", 0); !errors.Is(err, ErrNoDeeds) {
		t.Fatalf("a price of 0: %v", err)
	}
	if err := w.BuyDeed("home", 100); !errors.Is(err, ErrNoCleanCash) {
		t.Fatalf("no clean cash: %v", err)
	}
	w.Player.CleanCash = 50
	if err := w.BuyDeed("home", 100); err == nil || errors.Is(err, ErrNoCleanCash) {
		t.Fatalf("short of clean cash: %v", err)
	}
	dirty := w.Player.DirtyCash
	w.Player.CleanCash = 250
	worth := w.NetWorth()
	if err := w.BuyDeed("home", 100); err != nil {
		t.Fatal(err)
	}
	c := w.Corner("home")
	if c.Deed == nil || c.Deed.Bought != 3 || c.Deed.Price != 100 || !c.Deeded() {
		t.Fatalf("the deed: %+v", c.Deed)
	}
	if w.Player.CleanCash != 150 || w.Player.DirtyCash != dirty {
		t.Fatalf("clean %d dirty %d after the buy", w.Player.CleanCash, w.Player.DirtyCash)
	}
	if w.Stats.Deeds != 1 || w.Stats.DeedCash != 100 || len(w.Today.DeedsBought) != 1 || w.Today.DeedsBought[0] != "home" {
		t.Fatalf("the buy was not counted: %+v %v", w.Stats, w.Today.DeedsBought)
	}
	if w.NetWorth() != worth {
		t.Fatalf("net worth moved on a buy at cost: %d, was %d", w.NetWorth(), worth)
	}
	if err := w.BuyDeed("home", 100); !errors.Is(err, ErrDeeded) {
		t.Fatalf("buying it twice: %v", err)
	}
	// A rival's block is for sale too, and the deed outlives the hand-over.
	d := w.Corner("docks")
	d.Owner, d.Faction = OwnerRival, FactionRival
	w.Day = 5
	if err := w.BuyDeed("docks", 150); err != nil {
		t.Fatal(err)
	}
	if w.DeedValue() != 250 || w.DeedsIn("test") != 2 || len(w.Deeds()) != 2 {
		t.Fatalf("value %d in %d deeds %d", w.DeedValue(), w.DeedsIn("test"), len(w.Deeds()))
	}
	if n := w.NewestDeed(); n == nil || n.ID != "docks" {
		t.Fatalf("newest %+v, want the docks", n)
	}
	d.Owner, d.Faction, d.Since = OwnerPlayer, "", 6
	if w.Corner("docks").Deed == nil {
		t.Fatal("the deed went with the hand-over")
	}
	// The forfeiture takes the newest.
	if got := w.SeizeDeed("docks"); got == nil || got.Price != 150 || w.Corner("docks").Deed != nil || w.Stats.DeedsSeized != 1 {
		t.Fatalf("seized %+v, deed %+v, seized %d", got, w.Corner("docks").Deed, w.Stats.DeedsSeized)
	}
	if got := w.SeizeDeed("docks"); got != nil {
		t.Fatalf("seized twice: %+v", got)
	}
	if n := w.NewestDeed(); n == nil || n.ID != "home" {
		t.Fatalf("newest after the seizure %+v, want home", n)
	}
	// Over: nothing is for sale.
	w.Over = &Ending{Day: 6, Cause: "test"}
	if err := w.BuyDeed("docks", 150); !errors.Is(err, ErrGameOver) {
		t.Fatalf("after the end: %v", err)
	}
}

// A deed survives the save: a world with one on a corner of yours and
// one on the rival's decodes with both, prices and days intact, and a
// corner with none decodes with none.
func TestDeedSurvivesSave(t *testing.T) {
	w := testWorld()
	w.Day = 7
	w.Player.CleanCash = 1_000
	if err := w.BuyDeed("home", 400); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(w); err != nil {
		t.Fatal(err)
	}
	var got World
	if err := gob.NewDecoder(&buf).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if d := got.Corner("home").Deed; d == nil || *d != (Deed{Bought: 7, Price: 400}) {
		t.Fatalf("the deed after decode: %+v", d)
	}
	if got.Corner("docks").Deed != nil {
		t.Fatalf("a deed appeared on the docks: %+v", got.Corner("docks").Deed)
	}
	if got.Stats.Deeds != 1 || got.Stats.DeedCash != 400 || got.DeedValue() != 400 {
		t.Fatalf("the stats after decode: %+v value %d", got.Stats, got.DeedValue())
	}
}
