package game

import (
	"bytes"
	"encoding/gob"
	"errors"
	"os"
	"reflect"
	"testing"
)

// testHouse is a house on the test world's home corner as houses.toml
// would offer it: 100 units, $1,000, $10 a day.
func testHouse(id string, capacity int) HouseOffer {
	return HouseOffer{ID: id, Name: "House " + id, City: "test", Corner: "home", Capacity: capacity, Price: 1000, Rent: 10}
}

// TestStockGoesHouseFirstAndComesStreetFirst: what arrives is put away
// into the emptiest house with room, the next once that is full, the
// street when the houses are full; what leaves comes off the street
// first, then the houses in the order bought (#73). Stock, StockIn,
// StashOf and Capacity read the street and the houses together, Street
// and StreetOf the street alone.
func TestStockGoesHouseFirstAndComesStreetFirst(t *testing.T) {
	w := testWorld()
	w.Player.DirtyCash = 10_000
	for _, h := range []HouseOffer{testHouse("h1", 30), testHouse("h2", 50)} {
		if _, err := w.BuyHouse(h); err != nil {
			t.Fatal(err)
		}
	}
	if w.Capacity("test") != 100+30+50 || w.StreetCapacity("test") != 100 {
		t.Fatalf("capacity %d street %d", w.Capacity("test"), w.StreetCapacity("test"))
	}
	w.AddStock("test", "a", 40, 0)
	// h2 has the most room (50): it takes the 40.
	if h1, h2 := w.House("h1").Stock["a"], w.House("h2").Stock["a"]; h1 != 0 || h2 != 40 || w.Street("test", "a") != 0 {
		t.Fatalf("40 landed h1 %d h2 %d street %d", h1, h2, w.Street("test", "a"))
	}
	w.AddStock("test", "a", 50, 0)
	// h1 now has the most room (30 to h2's 10): 30 into h1, 10 into h2, 10 onto the street.
	if h1, h2, st := w.House("h1").Stock["a"], w.House("h2").Stock["a"], w.Street("test", "a"); h1 != 30 || h2 != 50 || st != 10 {
		t.Fatalf("after 90: h1 %d h2 %d street %d", h1, h2, st)
	}
	if w.Stock("test", "a") != 90 || w.StockIn("test") != 90 || w.StashOf("test")["a"] != 90 || w.StreetOf("test")["a"] != 10 || w.Free("test") != 90 {
		t.Fatalf("reads: stock %d in %d stash %v street %v free %d", w.Stock("test", "a"), w.StockIn("test"), w.StashOf("test"), w.StreetOf("test"), w.Free("test"))
	}
	// Out: the street's 10, then h1's 30 (bought first), then h2.
	if took := w.TakeStock("test", "a", 35); took != 35 || w.Street("test", "a") != 0 || w.House("h1").Stock["a"] != 5 || w.House("h2").Stock["a"] != 50 {
		t.Fatalf("took %d: street %d h1 %d h2 %d", took, w.Street("test", "a"), w.House("h1").Stock["a"], w.House("h2").Stock["a"])
	}
	if took := w.TakeStock("test", "a", 100); took != 55 || w.Stock("test", "a") != 0 {
		t.Fatalf("over-take took %d, left %d", took, w.Stock("test", "a"))
	}
	// TakeStreet and TakeFromHouse reach one place only.
	w.AddStock("test", "a", 100, 0) // h1 30 (most room? h1 30, h2 50: h2 first 50, then h1 30, then 20 street)
	if w.TakeStreet("test", "a", 99) != 20 || w.TakeFromHouse("h1", "a", 99) != 30 || w.TakeFromHouse("nope", "a", 1) != 0 || w.Stock("test", "a") != 50 {
		t.Fatalf("one-place takes: %v %v %d", w.StreetOf("test"), w.House("h2").Stock, w.Stock("test", "a"))
	}
	if w.TotalStock() != 50 || w.Stashed() != 50 {
		t.Fatalf("total %d stashed %d", w.TotalStock(), w.Stashed())
	}
}

// TestMoveIsFreeInstantAndCounted: Move shifts units between two places
// in a city at once, clamped and refused past the room, never between
// cities or to the same place, and records the units in Moved for the
// heat sim; MoveStock is the plain move.
func TestMoveIsFreeInstantAndCounted(t *testing.T) {
	w := testWorld()
	w.Player.DirtyCash = 10_000
	if _, err := w.BuyHouse(testHouse("h1", 30)); err != nil {
		t.Fatal(err)
	}
	w.SetStock("test", "a", 50) // the street, straight
	if n, err := w.Move("test", Street, "h1", "a", 20); err != nil || n != 20 || w.House("h1").Stock["a"] != 20 || w.Street("test", "a") != 30 {
		t.Fatalf("move to the house: %d %v", n, err)
	}
	if _, err := w.Move("test", Street, "h1", "a", 20); !errors.Is(err, ErrHouseFull) {
		t.Fatalf("past the room: %v", err)
	}
	if _, err := w.Move("test", "h1", Street, "a", 25); err == nil {
		t.Fatal("more than the house holds was moved")
	}
	if _, err := w.Move("test", "h1", "h1", "a", 1); !errors.Is(err, ErrSamePlace) {
		t.Fatalf("same place: %v", err)
	}
	if _, err := w.Move("port", Street, "h1", "a", 1); !errors.Is(err, ErrNoCity) && !errors.Is(err, ErrNoHouse) {
		t.Fatalf("another city: %v", err)
	}
	if n, err := w.Move("test", "h1", Street, "a", 5); err != nil || n != 5 {
		t.Fatalf("back to the street: %d %v", n, err)
	}
	if len(w.Today.Moved) != 2 || w.Today.Moved[0].Units != 20 || w.Today.Moved[1].Units != 5 || w.Today.Moved[0].To != "h1" || w.Today.Moved[1].From != "h1" {
		t.Fatalf("moved: %+v", w.Today.Moved)
	}
	if w.Player.DirtyCash != 9_000 || w.Stock("test", "a") != 50 {
		t.Fatalf("a move cost something or lost something: cash %d stock %d", w.Player.DirtyCash, w.Stock("test", "a"))
	}
	if w.MoveStock("test", Street, "h1", "a", 100) != 15 || w.House("h1").Units() != 30 {
		t.Fatalf("MoveStock past the room: %v", w.House("h1").Stock)
	}
	c := NewClock(nil, &counter{})
	c.EndDay(w)
	if w.Today.Moved != nil {
		t.Fatal("the clock kept the moves")
	}
}

// TestBuyDropAndGuard: BuyHouse takes the price in dirty cash and
// refuses a locked, owned or unaffordable house; Drop takes the house and
// its stock off the world and recalls the guard; Guard puts an enforcer
// inside and off their corner, a Post takes them off the house, and a
// runner cannot guard.
func TestBuyDropAndGuard(t *testing.T) {
	w := testWorld()
	w.Player.DirtyCash = 1_500
	locked := testHouse("h9", 10)
	locked.UnlockCash = 1_000_000
	if _, err := w.BuyHouse(locked); !errors.Is(err, ErrHouseLocked) {
		t.Fatalf("a locked house: %v", err)
	}
	if _, err := w.BuyHouse(testHouse("h1", 30)); err != nil || w.Player.DirtyCash != 500 || len(w.Houses) != 1 || w.Today.HousesBought[0] != "h1" {
		t.Fatalf("buy: %v cash %d houses %d bought %v", err, w.Player.DirtyCash, len(w.Houses), w.Today.HousesBought)
	}
	if _, err := w.BuyHouse(testHouse("h1", 30)); !errors.Is(err, ErrHouseOwned) {
		t.Fatalf("owned twice: %v", err)
	}
	if _, err := w.BuyHouse(testHouse("h2", 30)); err == nil {
		t.Fatal("bought a house with $500")
	}
	w.Crew.Members = append(w.Crew.Members,
		CrewMember{ID: 1, Name: "Moose", Role: "enforcer", Skill: 70},
		CrewMember{ID: 2, Name: "Dre", Role: "runner", Skill: 50, Units: 50})
	if err := w.Post("docks", 1); err != nil {
		t.Fatal(err)
	}
	if err := w.Guard("h1", 2); !errors.Is(err, ErrNotEnforcer) {
		t.Fatalf("a runner guarding: %v", err)
	}
	if err := w.Guard("h1", 1); err != nil || w.House("h1").Guard != 1 || w.PostOf(1) != nil || w.GuardOf(1) == nil {
		t.Fatalf("guard: %v house %+v post %v", err, w.House("h1"), w.PostOf(1))
	}
	if err := w.Post("docks", 1); err != nil || w.House("h1").Guard != 0 || w.PostOf(1) == nil {
		t.Fatalf("post off the house: %v guard %d post %v", err, w.House("h1").Guard, w.PostOf(1))
	}
	if err := w.Guard("h1", 1); err != nil {
		t.Fatal(err)
	}
	w.AddStock("test", "a", 20, 0)
	gone, err := w.Drop("h1")
	if err != nil || gone.ID != "h1" || len(w.Houses) != 0 || w.Stock("test", "a") != 0 || w.GuardOf(1) != nil {
		t.Fatalf("drop: %v %+v houses %d stock %d guard %v", err, gone, len(w.Houses), w.Stock("test", "a"), w.GuardOf(1))
	}
	if _, err := w.Drop("h1"); !errors.Is(err, ErrNoHouse) {
		t.Fatalf("dropping twice: %v", err)
	}
	if err := w.Guard("h1", 1); !errors.Is(err, ErrNoHouse) {
		t.Fatalf("guarding nothing: %v", err)
	}
}

// TestNetWorthCountsTheHouses: stock in a house is valued as stock on
// the street, and the house at its price.
func TestNetWorthCountsTheHouses(t *testing.T) {
	w := testWorld()
	w.Player.DirtyCash = 10_000
	w.SetStock("test", "a", 20)
	before := w.NetWorth()
	if _, err := w.BuyHouse(testHouse("h1", 30)); err != nil {
		t.Fatal(err)
	}
	w.MoveStock("test", Street, "h1", "a", 20)
	if got := w.NetWorth(); got != before {
		t.Fatalf("net worth %d after the house, %d before: the house and its stock should be worth what they cost", got, before)
	}
}

// TestSaveMigratesTheStashIntoAStarterHouse: a schema-11 save (the
// same World with Houses nil, which gob leaves out) loads with each
// city's pile in a rent-free starter house on the block the player
// stands on, sized at the carry limit or the pile, the street empty, a
// city holding nothing getting no house, and plays on the same.
func TestSaveMigratesTheStashIntoAStarterHouse(t *testing.T) {
	t.Setenv("KINGPIN_HOME", t.TempDir())
	old := twoCityWorld()
	old.SetStock("test", "a", 130)
	old.Day = 4
	old.SchemaVersion = 11
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(old); err != nil {
		t.Fatal(err)
	}
	p, _ := SavePath(1)
	if err := os.WriteFile(p, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(1); err == nil {
		t.Fatal("a schema-11 save loaded without a migration")
	}
	got, err := Load(1, Migration{From: 11, Apply: MigrateHouses}, Migration{From: 12, Apply: func(w *World) { w.MigrateLots(50, 1) }})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Houses) != 1 {
		t.Fatalf("houses: %+v", got.Houses)
	}
	h := got.Houses[0]
	if h.ID != StarterHouse+"test" || h.City != "test" || h.Corner != "home" || h.Capacity != 130 || h.Rent != 0 || h.Price != 0 || h.Stock["a"] != 130 {
		t.Fatalf("starter house: %+v", h)
	}
	if got.Street("test", "a") != 0 || got.Stock("test", "a") != 130 || got.Stock("port", "a") != 0 {
		t.Fatalf("stock after: street %d stock %d port %d", got.Street("test", "a"), got.Stock("test", "a"), got.Stock("port", "a"))
	}
	// Under the carry limit the house is the carry limit's size.
	small := twoCityWorld()
	small.SetStock("test", "a", 7)
	MigrateHouses(small)
	if small.Houses[0].Capacity != 100 || small.Stock("test", "a") != 7 {
		t.Fatalf("a small pile: %+v", small.Houses[0])
	}
	// Run again it is a no-op.
	MigrateHouses(small)
	if len(small.Houses) != 1 {
		t.Fatal("migrating twice made a second house")
	}
	// The migrated world and the same world with its stock on the
	// street play on the same: nothing about the stash's place moves the
	// dice.
	fresh := twoCityWorld()
	fresh.SetStock("test", "a", 130)
	fresh.Day = 4
	ca, cb := NewClock(nil, &counter{}), NewClock(nil, &counter{})
	for i := 0; i < 5; i++ {
		ea, eb := ca.EndDay(fresh), cb.EndDay(got)
		if !reflect.DeepEqual(ea, eb) {
			t.Fatalf("day %d differs after migration:\n%#v\n%#v", i, ea, eb)
		}
	}
}
