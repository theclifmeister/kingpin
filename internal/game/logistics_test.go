package game

import (
	"bytes"
	"encoding/gob"
	"os"
	"reflect"
	"testing"

	"github.com/theclifmeister/kingpin/internal/events"
)

// Shipping takes the units and the cost at once, refuses what the route
// cannot carry, what is not in the stash, a second shipment on the route
// the same day, and a route that does not join the cities; stock is
// conserved between the stash and the road.
func TestShip(t *testing.T) {
	w := twoCityWorld()
	w.Stash("test")["a"] = 80
	w.Player.DirtyCash = 100
	if _, err := w.Ship(testRoute, "test", "nowhere", "a", 10); err != ErrNoCity {
		t.Fatalf("shipped to nowhere: %v", err)
	}
	if _, err := w.Ship(RouteOffer{ID: "x", From: "test", To: "test"}, "test", "port", "a", 10); err != ErrNoRoute {
		t.Fatalf("shipped on a route that goes elsewhere: %v", err)
	}
	if _, err := w.Ship(testRoute, "test", "port", "b", 10); err != ErrUnknownProduct {
		t.Fatalf("shipped nothing: %v", err)
	}
	if _, err := w.Ship(testRoute, "test", "port", "a", 60); err == nil {
		t.Fatal("shipped more than the route carries")
	}
	if _, err := w.Ship(testRoute, "port", "test", "a", 10); err == nil {
		t.Fatal("shipped from an empty stash")
	}
	if _, err := w.Ship(testRoute, "test", "port", "a", 0); err != ErrBadQuantity {
		t.Fatalf("shipped nothing: %v", err)
	}
	w.Player.DirtyCash = 50
	if _, err := w.Ship(testRoute, "test", "port", "a", 40); err == nil {
		t.Fatal("shipped without the fare")
	}
	w.Player.DirtyCash = 100
	w.Day = 3
	s, err := w.Ship(testRoute, "test", "port", "a", 40)
	if err != nil {
		t.Fatal(err)
	}
	if s.ID != 1 || s.Units != 40 || s.Cost != 80 || s.Sent != 3 || s.Arrives != 5 || s.From != "test" || s.To != "port" || s.Dial != events.ShipNormal {
		t.Fatalf("shipment: %+v", s)
	}
	if w.Stock("test", "a") != 40 || w.Player.DirtyCash != 20 || w.InTransit("a") != 40 || w.TotalStock() != 80 || w.Player.TotalStock() != 40 {
		t.Fatalf("after shipping: stash %d cash %d transit %d total %d", w.Stock("test", "a"), w.Player.DirtyCash, w.InTransit("a"), w.TotalStock())
	}
	if w.Stats.Shipments != 1 || w.Stats.Shipped != 40 {
		t.Fatalf("stats: %+v", w.Stats)
	}
	if _, err := w.Ship(testRoute, "test", "port", "a", 10); err != ErrRouteBusy {
		t.Fatalf("second shipment on the route today: %v", err)
	}
	w.Day = 4
	w.Player.DirtyCash = 100
	if _, err := w.Ship(testRoute, "test", "port", "a", 10); err != nil {
		t.Fatalf("tomorrow's shipment: %v", err)
	}
	if len(w.Shipments) != 2 || w.Shipments[1].ID != 2 || w.Shipments[1].DaysLeft(4) != 2 || w.Shipments[0].DaysLeft(5) != 0 {
		t.Fatalf("shipments: %+v", w.Shipments)
	}
	// Net worth counts what is on the road at the far end's supplier price.
	want := w.Cash() + int(30*w.Home().Market["a"].SupplierPrice) + int(50*w.Cities["port"].Market["a"].SupplierPrice)
	if w.NetWorth() != want {
		t.Fatalf("net worth %d, want %d", w.NetWorth(), want)
	}
}

// Travel moves you at once, leaves your corner with nobody on it and
// your stock where it was; you can only stand on a corner where you are,
// and you buy where you are, into the stash there.
func TestTravelAndBuyWhereYouAre(t *testing.T) {
	w := twoCityWorld()
	w.Home().Market["a"].SupplierPrice = 5
	w.Cities["port"].Market["a"].SupplierPrice = 2
	if _, err := w.Buy("a", 10, 0); err != nil || w.Stock("test", "a") != 10 || w.Stock("port", "a") != 0 || w.Player.DirtyCash != 450 {
		t.Fatalf("buy at home: %v %+v", err, w.Player)
	}
	if err := w.Post("wharf", You); err != ErrElsewhere {
		t.Fatalf("stood on a corner in a city you are not in: %v", err)
	}
	if err := w.Travel("nowhere"); err != ErrNoCity {
		t.Fatalf("travelled nowhere: %v", err)
	}
	if err := w.Travel("port"); err != nil {
		t.Fatal(err)
	}
	if w.Player.Location != "port" || w.Here().ID != "port" || w.PostOf(You) != nil || !w.Corner("home").Held() || w.Corner("home").Runner != 0 {
		t.Fatalf("after travelling: %+v home %+v", w.Player, *w.Corner("home"))
	}
	if w.Stock("test", "a") != 10 || w.Stock("port", "a") != 0 {
		t.Fatal("stock travelled with you")
	}
	if w.Capacity("test") != 0 || w.Capacity("port") != 100 || w.Free("test") != -10 {
		t.Fatalf("capacity follows you: test %d port %d", w.Capacity("test"), w.Capacity("port"))
	}
	if _, err := w.Buy("a", 20, 0); err != nil || w.Stock("port", "a") != 20 || w.Player.DirtyCash != 410 {
		t.Fatalf("buy at the port: %v %+v", err, w.Player)
	}
	if err := w.Post("wharf", You); err != nil || w.Corner("wharf").Runner != You {
		t.Fatalf("stand on the wharf: %v", err)
	}
	if err := w.Travel("port"); err != nil || w.Corner("wharf").Runner != You {
		t.Fatal("travelling to where you are moved you")
	}
	// A runner counts where they are posted; an idle one, where you are.
	w.Crew.Members = []CrewMember{{ID: 1, Name: "Dre", Role: "runner", Units: 30}}
	if w.Capacity("port") != 130 || w.Capacity("test") != 0 {
		t.Fatalf("idle runner: port %d test %d", w.Capacity("port"), w.Capacity("test"))
	}
	if err := w.Post("docks", 1); err != nil || w.Capacity("test") != 30 || w.Capacity("port") != 100 {
		t.Fatalf("posted runner: %v test %d port %d", err, w.Capacity("test"), w.Capacity("port"))
	}
	// Demand is per city: the wharf's, not the docks'.
	if w.Demand("port", "a") != 2 || w.Demand("test", "a") != 5*3 {
		t.Fatalf("demand port %v test %v", w.Demand("port", "a"), w.Demand("test", "a"))
	}
}

// The wholesaler sells by the lot, cheaper, only where it deals and only
// once the door is open, and a lot goes to the dock: the stash's capacity
// does not hold it.
func TestBuyWholesale(t *testing.T) {
	w := twoCityWorld()
	w.Home().Market["a"].SupplierPrice = 10
	w.Cities["port"].Market["a"].SupplierPrice = 4
	w.Player.DirtyCash = 10_000
	offer := WholesaleOffer{Lot: 100, Mul: 0.5, UnlockCash: 5_000}
	if _, err := w.BuyWholesale("a", 1, offer, 0); err != ErrNoWholesale {
		t.Fatalf("bought by the lot at home: %v", err)
	}
	_ = w.Travel("port")
	if _, err := w.BuyWholesale("a", 1, offer, 0); err == nil || !offer.Locked(w) {
		t.Fatalf("bought by the lot before the unlock: %v", err)
	}
	w.Stats.PeakCash = 5_000
	if _, err := w.BuyWholesale("a", 0, offer, 0); err != ErrBadQuantity {
		t.Fatalf("bought no lots: %v", err)
	}
	p, err := w.BuyWholesale("a", 3, offer, 0)
	if err != nil || p.Qty != 300 || p.Cost != 600 || p.UnitPrice != 2 || !p.Wholesale || w.Stock("port", "a") != 300 || w.Player.DirtyCash != 9_400 {
		t.Fatalf("lots: %v %+v cash %d stash %d", err, p, w.Player.DirtyCash, w.Stock("port", "a"))
	}
	if w.Free("port") >= 0 {
		t.Fatalf("the lots should be past capacity: free %d", w.Free("port"))
	}
	if _, err := w.Buy("a", 1, 0); err == nil {
		t.Fatal("bought at retail past capacity")
	}
	if _, err := w.BuyWholesale("a", 100, offer, 0); err == nil {
		t.Fatal("bought lots without the cash")
	}
}

// Stashes, shipments in transit and where you are survive a save.
func TestSaveKeepsLogistics(t *testing.T) {
	t.Setenv("KINGPIN_HOME", t.TempDir())
	w := twoCityWorld()
	w.Stash("test")["a"] = 30
	w.Stash("port")["a"] = 7
	w.Player.DirtyCash = 1000
	if _, err := w.Ship(testRoute, "test", "port", "a", 20); err != nil {
		t.Fatal(err)
	}
	_ = w.Travel("port")
	w.Cities["port"].Heat = 12.5
	w.Logistics.Seizures = []Seizure{{Day: 1, Route: "road", From: "test", To: "port", Product: "a", Units: 5}}
	w.Stats.Seizures, w.Stats.SeizedOnRoad = 1, 5
	if err := Save(w); err != nil {
		t.Fatal(err)
	}
	got, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Player, w.Player) || !reflect.DeepEqual(got.Shipments, w.Shipments) || !reflect.DeepEqual(got.Logistics, w.Logistics) || got.Stats != w.Stats {
		t.Fatalf("logistics did not round-trip:\n%+v %+v %+v\n%+v %+v %+v", got.Player, got.Shipments, got.Logistics, w.Player, w.Shipments, w.Logistics)
	}
	if !reflect.DeepEqual(got.CityOrder, w.CityOrder) || got.Cities["port"].Heat != 12.5 || got.Cities["port"].Wholesale != true || got.Cities["port"].HeatMul != 0.5 {
		t.Fatalf("cities did not round-trip: %v %+v", got.CityOrder, got.Cities["port"])
	}
	if got.Stock("port", "a") != 7 || got.InTransit("a") != 20 || got.Here().ID != "port" {
		t.Fatalf("loaded: stash %d transit %d here %s", got.Stock("port", "a"), got.InTransit("a"), got.Here().ID)
	}
}

// v6World is the shape a schema-6 save had: the one city's market,
// corners, stock and heat on World, Player and HeatState themselves.
// Only the fields the migration wraps need writing: gob fills the rest
// with zero values, the way a real old save has none of the new ones.
type v6World struct {
	SchemaVersion int
	Seed          uint64
	Day           int
	City          string
	Player        struct {
		DirtyCash  int
		CleanCash  int
		Stock      map[string]int
		CarryLimit int
	}
	Products  []string
	Market    map[string]*ProductMarket
	Heat      struct{ Value float64 }
	Territory struct{ Corners []Corner }
	Upgrades  map[string]bool
	Orders    map[string]SellOrder
}

// A schema-6 save migrates to a world whose home city is the city it
// had, market, corners, stock and heat intact, and plays on identically
// to the same world built with cities from the start.
func TestSaveMigratesTheOneCity(t *testing.T) {
	t.Setenv("KINGPIN_HOME", t.TempDir())
	// The world as it would have been on day 3 of a schema-6 run.
	fresh := testWorld()
	fresh.Day = 3
	fresh.Player.DirtyCash = 777
	fresh.Stash("test")["a"] = 12
	fresh.Home().Heat = 21.5
	fresh.Home().Market["a"].Price = 11
	fresh.Home().Market["a"].History = []float64{10, 11}
	fresh.Corner("docks").Owner, fresh.Corner("docks").Since = OwnerPlayer, 2
	old := v6World{SchemaVersion: 6, Seed: fresh.Seed, Day: fresh.Day, City: "Testville", Products: fresh.Products, Market: fresh.Home().Market, Upgrades: map[string]bool{}, Orders: map[string]SellOrder{}}
	old.Player.DirtyCash, old.Player.Stock, old.Player.CarryLimit = 777, map[string]int{"a": 12}, 100
	old.Heat.Value = 21.5
	for _, c := range fresh.Home().Corners {
		c.City = "" // a pre-7 corner knew no city
		old.Territory.Corners = append(old.Territory.Corners, c)
	}
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(old); err != nil {
		t.Fatal(err)
	}
	p, _ := SavePath()
	if err := os.MkdirAll(t.TempDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(); err == nil {
		t.Fatal("a schema-6 save loaded without a migration")
	}
	home := StartingCity{ID: "test", Name: "Testville", HeatMul: 1}
	// The steps past 7 are other packages' (the rival's trust, #32; the
	// chief and the DA, #41); the chain only needs to reach the current
	// schema.
	got, err := Load(Migration{From: 6, Apply: func(w *World) { w.MigrateCities(home) }}, Migration{From: 7, Apply: func(*World) {}}, Migration{From: 8, Apply: func(*World) {}})
	if err != nil {
		t.Fatal(err)
	}
	if got.SchemaVersion != SchemaVersion || got.Day != 3 || got.Player.Location != "test" || len(got.CityOrder) != 1 || got.CityOrder[0] != "test" {
		t.Fatalf("migrated: schema %d day %d location %q order %v", got.SchemaVersion, got.Day, got.Player.Location, got.CityOrder)
	}
	if !reflect.DeepEqual(got.Home().Market, fresh.Home().Market) || !reflect.DeepEqual(got.Home().Corners, fresh.Home().Corners) || got.Home().Heat != 21.5 {
		t.Fatalf("home city:\n%+v\n%+v", got.Home(), fresh.Home())
	}
	if got.Stock("test", "a") != 12 || got.Player.DirtyCash != 777 || got.Home().Name != "Testville" {
		t.Fatalf("player: %+v", got.Player)
	}
	// And it plays on the same as the fresh world.
	ca, cb := NewClock(nil, &counter{}), NewClock(nil, &counter{})
	for i := 0; i < 5; i++ {
		ea, eb := ca.EndDay(fresh), cb.EndDay(got)
		if !reflect.DeepEqual(ea, eb) {
			t.Fatalf("day %d differs after migration:\n%#v\n%#v", i, ea, eb)
		}
	}
	// A save that already has cities is left alone by the same step.
	again := twoCityWorld()
	again.Stash("test")["a"] = 3
	again.MigrateCities(home)
	if len(again.CityOrder) != 2 || again.Stock("test", "a") != 3 || again.Player.Location != "test" {
		t.Fatalf("migrating a world with cities: %v %+v", again.CityOrder, again.Player)
	}
}
