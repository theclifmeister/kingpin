package game

import (
	"bytes"
	"encoding/gob"
	"errors"
	"os"
	"reflect"
	"testing"

	"github.com/theclifmeister/kingpin/internal/events"
)

// The route dial is a setting, not scratch: it turns, keeps its targets
// while off, refuses a position it does not have and a product the
// ladder does not list, and clears a target set to zero.
func TestRouteSettings(t *testing.T) {
	w := twoCityWorld()
	if rs := w.Route("road"); rs.Dial != events.RouteOff || rs.Dial.On() || len(rs.Target) != 0 {
		t.Fatalf("an untouched route: %+v", rs)
	}
	if err := w.SetRoute("", events.RouteNormal); err != ErrNoRoute {
		t.Fatalf("set no route: %v", err)
	}
	if err := w.SetRoute("road", events.RouteFast+1); err != ErrBadDial {
		t.Fatalf("set a dial past fast: %v", err)
	}
	if err := w.SetRouteTarget("road", "b", 10); err != ErrUnknownProduct {
		t.Fatalf("a target for nothing: %v", err)
	}
	if err := w.SetRouteTarget("road", "a", -1); err != ErrBadQuantity {
		t.Fatalf("a negative target: %v", err)
	}
	if err := w.SetRouteTarget("road", "a", 120); err != nil {
		t.Fatal(err)
	}
	if err := w.SetRoute("road", events.RouteSlow); err != nil {
		t.Fatal(err)
	}
	if rs := w.Route("road"); rs.Dial != events.RouteSlow || !rs.Dial.On() || rs.Dial.Ship() != events.ShipSlow || rs.Target["a"] != 120 {
		t.Fatalf("set: %+v", rs)
	}
	if err := w.SetRoute("road", events.RouteOff); err != nil || w.Route("road").Target["a"] != 120 {
		t.Fatalf("turning it off lost the target: %v %+v", err, w.Route("road"))
	}
	if err := w.SetRouteTarget("road", "a", 0); err != nil || w.Route("road").Target != nil {
		t.Fatalf("clearing the target: %v %+v", err, w.Route("road"))
	}
	// A days target (#115) is the other kind: a product keeps units or
	// days, never both, setting one clears the other, and a cleared
	// setting is the zero value again.
	if err := w.SetRouteDays("road", "b", 3); err != ErrUnknownProduct {
		t.Fatalf("days for nothing: %v", err)
	}
	if err := w.SetRouteDays("road", "a", -1); err != ErrBadQuantity {
		t.Fatalf("negative days: %v", err)
	}
	if err := w.SetRouteTarget("road", "a", 120); err != nil {
		t.Fatal(err)
	}
	if err := w.SetRouteDays("road", "a", 3); err != nil {
		t.Fatal(err)
	}
	if rs := w.Route("road"); rs.Days["a"] != 3 || rs.Target != nil || !rs.HasTargets() {
		t.Fatalf("days over units: %+v", rs)
	}
	if err := w.SetRouteTarget("road", "a", 50); err != nil {
		t.Fatal(err)
	}
	if rs := w.Route("road"); rs.Target["a"] != 50 || rs.Days != nil {
		t.Fatalf("units over days: %+v", rs)
	}
	_ = w.SetRouteDays("road", "a", 2)
	if err := w.SetRouteDays("road", "a", 0); err != nil || w.Route("road").HasTargets() || w.Route("road").Days != nil {
		t.Fatalf("clearing the days: %v %+v", err, w.Route("road"))
	}
	if rs := w.Route("road"); !reflect.DeepEqual(rs, RouteSetting{}) {
		t.Fatalf("a cleared route is not the zero value: %+v", rs)
	}
	if d := events.RouteDial(0); d.String() != "off" || events.RouteFast.String() != "fast" || events.RouteNormal.Ship() != events.ShipNormal || events.RouteFast.Ship() != events.ShipFast || events.RouteOff.Ship() != events.ShipNormal {
		t.Fatal("the dial's names or ship positions")
	}
	w.Over = &Ending{Day: 1, Cause: "test"}
	if err := w.SetRoute("road", events.RouteNormal); err != ErrGameOver {
		t.Fatalf("set after the end: %v", err)
	}
	if err := w.SetRouteTarget("road", "a", 1); err != ErrGameOver {
		t.Fatalf("target after the end: %v", err)
	}
}

// Send takes the units and the fare at once and counts the shipment;
// stock is conserved between the stash and the road, and net worth
// values the road at the far end's supplier price.
func TestSend(t *testing.T) {
	w := twoCityWorld()
	w.SetStock("test", "a", 80)
	w.Player.DirtyCash = 100
	s := w.Send(testShipment(40))
	if s.ID != 1 || s.Units != 40 || s.Cost != 80 || s.Sent != 3 || s.Arrives != 5 || s.From != "test" || s.To != "port" || s.Dial != events.ShipNormal {
		t.Fatalf("shipment: %+v", s)
	}
	if w.Stock("test", "a") != 40 || w.Player.DirtyCash != 20 || w.InTransit("a") != 40 || w.Bound("port", "a") != 40 || w.Bound("test", "a") != 0 || w.TotalStock() != 80 || w.Player.TotalStock() != 40 {
		t.Fatalf("after sending: stash %d cash %d transit %d total %d", w.Stock("test", "a"), w.Player.DirtyCash, w.InTransit("a"), w.TotalStock())
	}
	if w.Stats.Shipments != 1 || w.Stats.Shipped != 40 {
		t.Fatalf("stats: %+v", w.Stats)
	}
	same := testShipment(10)
	same.Arrives = same.Sent // a route that takes no days still takes one
	if s2 := w.Send(same); len(w.Shipments) != 2 || s2.ID != 2 || s2.Arrives != 4 || w.Shipments[1].DaysLeft(3) != 1 || w.Shipments[0].DaysLeft(5) != 0 {
		t.Fatalf("shipments: %+v", w.Shipments)
	}
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
	priceAt(w, "test", "a", 5)
	priceAt(w, "port", "a", 2)
	if _, err := w.Buy("street", "a", 10, false, 0); err != nil || w.Stock("test", "a") != 10 || w.Stock("port", "a") != 0 || w.Player.DirtyCash != 450 {
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
	if _, err := w.Buy("street", "a", 1, false, 0); err != ErrElsewhere {
		t.Fatalf("bought from the connect at home while in the port: %v", err)
	}
	if _, err := w.Buy("portstreet", "a", 20, false, 0); err != nil || w.Stock("port", "a") != 20 || w.Player.DirtyCash != 410 {
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

// The wholesaler sells by the lot, cheaper, only where one deals and
// only once the door is open, wherever the player is, and a lot is not
// held to the stash's capacity: the road takes it. Frozen they sell
// nothing (#72), and never more than their day has left.
func TestRestock(t *testing.T) {
	w := twoCityWorld()
	priceAt(w, "test", "a", 10)
	priceAt(w, "port", "a", 4)
	w.Supplier("wholesaler").Price["a"] = 2
	w.Player.DirtyCash = 10_000
	if _, err := w.Restock("test", "a", 1, 0); err != ErrNoRoute {
		t.Fatalf("bought by the lot at home: %v", err)
	}
	if _, err := w.Restock("nowhere", "a", 1, 0); err != ErrNoCity {
		t.Fatalf("bought by the lot nowhere: %v", err)
	}
	if _, err := w.Restock("port", "a", 1, 0); err != ErrNoRoute || !w.Supplier("wholesaler").Locked(w) {
		t.Fatalf("bought by the lot before the unlock: %v", err)
	}
	w.Stats.PeakCash = 5_000
	if _, err := w.Restock("port", "b", 1, 0); err != ErrUnknownProduct {
		t.Fatalf("bought nothing: %v", err)
	}
	if _, err := w.Restock("port", "a", 0, 0); err != ErrBadQuantity {
		t.Fatalf("bought no lots: %v", err)
	}
	p, err := w.Restock("port", "a", 3, 0)
	if err != nil || p.Qty != 300 || p.Cost != 600 || p.UnitPrice != 2 || p.Supplier != "wholesaler" || w.Stock("port", "a") != 300 || w.Player.DirtyCash != 9_400 || w.Player.Location != "test" {
		t.Fatalf("lots: %v %+v cash %d stash %d", err, p, w.Player.DirtyCash, w.Stock("port", "a"))
	}
	if len(w.Buys) != 0 {
		t.Fatalf("a lot is not the player's buy: %+v", w.Buys)
	}
	if w.Free("port") >= 0 {
		t.Fatalf("the lots should be past capacity: free %d", w.Free("port"))
	}
	if sup := w.Supplier("wholesaler"); sup.BoughtToday != 300 || sup.Bought != 300 || sup.Lots != 3 {
		t.Fatalf("the lots were not booked against the connect: %+v", sup)
	}
	if _, err := w.Restock("port", "a", 100, 0); err == nil {
		t.Fatal("bought lots without the cash")
	}
	w.Supplier("wholesaler").Cap = 400
	if _, err := w.Restock("port", "a", 2, 0); !errors.Is(err, ErrSupplierCapacity) {
		t.Fatalf("bought past the day's capacity: %v", err)
	}
	w.Supplier("wholesaler").FrozenUntil = w.Day + 3
	if _, err := w.Restock("port", "a", 1, 0); !errors.Is(err, ErrSupplierFrozen) {
		t.Fatalf("bought from a frozen connect: %v", err)
	}
}

// Stashes, shipments in transit and where you are survive a save.
func TestSaveKeepsLogistics(t *testing.T) {
	t.Setenv("KINGPIN_HOME", t.TempDir())
	w := twoCityWorld()
	w.SetStock("test", "a", 30)
	w.SetStock("port", "a", 7)
	w.Player.DirtyCash = 1000
	w.Send(testShipment(20))
	_ = w.Travel("port")
	w.Cities["port"].Heat = 12.5
	w.Logistics.Seizures = []Seizure{{Day: 1, Route: "road", From: "test", To: "port", Product: "a", Units: 5}}
	w.Logistics.Days = []RouteDay{{Day: 2, Route: "road", Wholesale: 300, Fares: 40}}
	w.Logistics.Lost = map[string]int{"road": 5}
	w.Stats.Seizures, w.Stats.SeizedOnRoad = 1, 5
	if err := w.SetRoute("road", events.RouteFast); err != nil {
		t.Fatal(err)
	}
	if err := w.SetRouteTarget("road", "a", 250); err != nil {
		t.Fatal(err)
	}
	if err := w.SetRouteDays("sea", "a", 3); err != nil { // a days target (#115) on another route
		t.Fatal(err)
	}
	if err := Save(1, w); err != nil {
		t.Fatal(err)
	}
	got, err := Load(1)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Player, w.Player) || !reflect.DeepEqual(got.Shipments, w.Shipments) || !reflect.DeepEqual(got.Logistics, w.Logistics) || got.Stats != w.Stats {
		t.Fatalf("logistics did not round-trip:\n%+v %+v %+v\n%+v %+v %+v", got.Player, got.Shipments, got.Logistics, w.Player, w.Shipments, w.Logistics)
	}
	if !reflect.DeepEqual(got.Routes, w.Routes) || got.Route("road").Dial != events.RouteFast || got.Route("road").Target["a"] != 250 || got.Route("sea").Days["a"] != 3 || got.Route("sea").Target != nil {
		t.Fatalf("the route settings did not round-trip: %+v, saved %+v", got.Routes, w.Routes)
	}
	if wholesale, fares := got.Logistics.RouteSpend("road", 5, 7); wholesale != 300 || fares != 40 {
		t.Fatalf("the route's week: %d %d", wholesale, fares)
	}
	if wholesale, fares := got.Logistics.RouteSpend("road", 20, 7); wholesale+fares != 0 {
		t.Fatalf("a spend past the week: %d %d", wholesale, fares)
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
	fresh.SetStock("test", "a", 12)
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
	p, _ := SavePath(1)
	if err := os.MkdirAll(t.TempDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(1); err == nil {
		t.Fatal("a schema-6 save loaded without a migration")
	}
	home := StartingCity{ID: "test", Name: "Testville", HeatMul: 1}
	// The steps past 7 are other packages' (the rival's trust, #32; the
	// chief and the DA, #41) or the fall guy's count (#117); the chain
	// only needs to reach the current schema.
	got, err := Load(1, Migration{From: 6, Apply: func(w *World) { w.MigrateCities(home) }}, Migration{From: 7, Apply: func(*World) {}}, Migration{From: 8, Apply: func(*World) {}}, Migration{From: 9, Apply: MigrateFallGuys}, Migration{From: 10, Apply: func(*World) {}})
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
	again.SetStock("test", "a", 3)
	again.MigrateCities(home)
	if len(again.CityOrder) != 2 || again.Stock("test", "a") != 3 || again.Player.Location != "test" {
		t.Fatalf("migrating a world with cities: %v %+v", again.CityOrder, again.Player)
	}
}
