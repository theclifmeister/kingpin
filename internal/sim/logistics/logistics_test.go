package logistics_test

import (
	"math"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/sim/logistics"
	"github.com/theclifmeister/kingpin/internal/sim/market"
	"github.com/theclifmeister/kingpin/internal/sim/territory"
)

// world is the config's cities with the player at home, the first
// route's source stashed with 500 of the first product, and the sim over
// the routes with the risk set as asked. The float is the config's.
func world(t *testing.T, cfg *content.Config, risk float64) (*game.World, *logistics.Sim, content.RouteConfig) {
	t.Helper()
	risky := *cfg
	risky.Routes.Routes = append([]content.RouteConfig(nil), cfg.Routes.Routes...)
	for i := range risky.Routes.Routes {
		risky.Routes.Routes[i].Risk = risk
	}
	s := logistics.New(&risky)
	w := game.NewWorld(7, logistics.StartingCities(cfg.City, cfg.Market), 100_000, 100)
	territory.New(cfg).Seed(w)
	// The connects (#72): the road buys from the wholesaler.
	mk, err := market.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	mk.Seed(w, game.RNGFor(7, 0))
	if len(risky.Routes.Routes) == 0 {
		t.Fatal("no routes")
	}
	r := risky.Routes.Routes[0]
	w.SetStock(r.From, w.Products[0], 500)
	return w, s, r
}

func step(w *game.World, s *logistics.Sim) []events.Event {
	t := &game.Tick{Day: w.Day + 1, RNG: game.RNGFor(w.Seed, w.Day+1), Seed: w.Seed}
	s.Step(w, t)
	w.Day++
	return t.Events()
}

// The dial scales a route: fast is fewer days at more risk per day, slow
// the reverse, and never under a day; the risk the map shows is what the
// days add up to.
func TestDialsAndOffers(t *testing.T) {
	cfg := content.MustLoad()
	s := logistics.New(cfg)
	w := game.NewWorld(7, logistics.StartingCities(cfg.City, cfg.Market), 100_000, 100)
	for _, r := range cfg.Routes.Routes {
		slow, normal, fast := s.Days(w, r, events.ShipSlow), s.Days(w, r, events.ShipNormal), s.Days(w, r, events.ShipFast)
		if normal != r.Days || fast > normal || slow < normal || fast < 1 {
			t.Fatalf("%s: days slow %d normal %d fast %d (route %d)", r.ID, slow, normal, fast, r.Days)
		}
		if s.DayRisk(w, r, events.ShipFast) <= s.DayRisk(w, r, events.ShipNormal) || s.DayRisk(w, r, events.ShipSlow) >= s.DayRisk(w, r, events.ShipNormal) {
			t.Fatalf("%s: day risk slow %.3f normal %.3f fast %.3f", r.ID, s.DayRisk(w, r, events.ShipSlow), s.DayRisk(w, r, events.ShipNormal), s.DayRisk(w, r, events.ShipFast))
		}
		if risk := s.Risk(w, r, events.ShipNormal); risk <= 0 || risk >= 1 || risk < s.DayRisk(w, r, events.ShipNormal) {
			t.Fatalf("%s: risk %.3f over %d days at %.3f a day", r.ID, risk, normal, s.DayRisk(w, r, events.ShipNormal))
		}
		if s.Capacity(w, r) != r.Capacity || s.Fare(w, r) != float64(r.Cost) {
			t.Fatalf("%s: capacity %d fare %.2f with nothing owned (route %d, %d)", r.ID, s.Capacity(w, r), s.Fare(w, r), r.Capacity, r.Cost)
		}
		if !r.Connects(r.From, r.To) || r.Other(r.From) != r.To {
			t.Fatalf("%s: joins %s and %s", r.ID, r.From, r.To)
		}
	}
	if s.Float() != cfg.Laundering.Laundering.Float {
		t.Fatalf("float %d", s.Float())
	}
	if s.Route("nowhere") != nil || len(s.Routes("nowhere")) != 0 {
		t.Fatal("a route that does not exist")
	}
}

// A route that is on sends the far city's shortfall the morning it is
// turned, and reports it that morning; on a safe road the shipment rolls
// nothing and lands on its day in the destination's stash; on a road
// that always intercepts it is seized on the first roll, the seizure is
// recorded for the market and the stats count it. A route that is off
// sends nothing.
func TestStepLandsOrSeizes(t *testing.T) {
	cfg := content.MustLoad()
	for _, risk := range []float64{0, 1} {
		w, s, r := world(t, cfg, risk)
		product := w.Products[0]
		if err := w.SetRouteTarget(r.ID, product, 10); err != nil {
			t.Fatal(err)
		}
		if step(w, s); len(w.Shipments) != 0 || w.Stock(r.From, product) != 500 {
			t.Fatalf("a route that is off sent %+v", w.Shipments)
		}
		if err := w.SetRoute(r.ID, events.RouteSlow); err != nil {
			t.Fatal(err)
		}
		days := s.Days(w, r, events.ShipSlow)
		var sent, arrived, seized int
		var sh events.ShipmentSent
		for day := 0; day < days+1; day++ {
			if day == 1 {
				// Off again once it has gone: a seized shipment is a
				// shortfall the route would otherwise send again.
				_ = w.SetRoute(r.ID, events.RouteOff)
			}
			for _, e := range step(w, s) {
				switch ev := e.(type) {
				case events.ShipmentSent:
					sent++
					sh = ev
					if ev.Day != 2 || ev.Days != days || ev.Units != 10 || ev.Dial != events.ShipSlow || ev.From != r.From || ev.To != r.To || ev.Cost != 10*r.Cost {
						t.Fatalf("sent: %+v", ev)
					}
					if len(w.Shipments) != 1 || w.Shipments[0].Sent != 2 || w.Shipments[0].Arrives != 2+days || w.Shipments[0].ID != ev.ID {
						t.Fatalf("on the road: %+v", w.Shipments)
					}
				case events.ShipmentArrived:
					arrived++
					if ev.Day != 2+days || ev.Units != 10 || ev.ID != sh.ID {
						t.Fatalf("arrived: %+v", ev)
					}
				case events.ShipmentSeized:
					seized++
					if ev.Day != 3 || ev.Units != 10 || ev.Dial != events.ShipSlow || ev.ID != sh.ID {
						t.Fatalf("seized: %+v", ev)
					}
				case events.WholesaleBought:
					t.Fatalf("bought lots where nobody sells them: %+v", ev)
				}
			}
		}
		switch {
		case sent != 1:
			t.Fatalf("risk %.0f: %d sent", risk, sent)
		case risk == 0 && (arrived != 1 || seized != 0 || w.Stock(r.To, product) != 10 || len(w.Shipments) != 0 || len(w.Logistics.Seizures) != 0):
			t.Fatalf("safe: %d arrived %d seized, far end %d, %d on the road", arrived, seized, w.Stock(r.To, product), len(w.Shipments))
		case risk == 1 && (arrived != 0 || seized != 1 || w.Stock(r.To, product) != 0 || len(w.Shipments) != 0 || len(w.Logistics.Seizures) != 1):
			t.Fatalf("seized: %d arrived %d seized, far end %d, %d on the road, record %+v", arrived, seized, w.Stock(r.To, product), len(w.Shipments), w.Logistics.Seizures)
		}
		if w.Stats.Shipments != 1 || w.Stats.Shipped != 10 || w.Player.DirtyCash != 100_000-10*r.Cost {
			t.Fatalf("stats %+v cash %d", w.Stats, w.Player.DirtyCash)
		}
		if d := w.Logistics.Days; len(d) != 1 || d[0].Route != r.ID || d[0].Day != 2 || d[0].Fares != 10*r.Cost || d[0].Wholesale != 0 {
			t.Fatalf("the route's books: %+v", d)
		}
		if risk == 1 {
			z := w.Logistics.Seizures[0]
			if z.Day != 3 || z.To != r.To || z.From != r.From || z.Product != product || z.Units != 10 || w.Stats.Seizures != 1 || w.Stats.SeizedOnRoad != 10 || w.Logistics.Lost[r.ID] != 10 {
				t.Fatalf("record %+v stats %+v lost %v", z, w.Stats, w.Logistics.Lost)
			}
			// The record is kept for a while, then dropped.
			for w.Day < cfg.Routes.Shipping.RecordDays+4 {
				step(w, s)
			}
			if len(w.Logistics.Seizures) != 0 || len(w.Logistics.Days) != 0 {
				t.Fatalf("record kept %d days: %+v %+v", w.Day, w.Logistics.Seizures, w.Logistics.Days)
			}
		}
		if w.Stock(r.From, product) != 490 {
			t.Fatalf("source stash %d", w.Stock(r.From, product))
		}
	}
}

// The route keeps the far city at its target: the shortfall against the
// target, less what is on the road, goes every day up to the capacity,
// and nothing goes once the target is met. The source stash goes first;
// where the wholesaler deals and the door is open the shortfall is bought
// in whole lots and reported; nothing is bought or sent out of the float;
// a lot the fare could not then move is not bought.
func TestRunKeepsTheTarget(t *testing.T) {
	cfg := content.MustLoad()
	w, s, r := world(t, cfg, 0)
	product := w.Products[0]
	offer := w.WholesaleSupplier(r.From)
	if offer == nil {
		t.Skipf("%s does not sell by the lot", r.From)
	}
	target := 3*r.Capacity + 25
	_ = w.SetRouteTarget(r.ID, product, target)
	_ = w.SetRoute(r.ID, events.RouteNormal)
	w.SetStock(r.From, product, 0)
	w.Player.DirtyCash = s.Float() // nothing over the float: nothing moves
	w.Stats.PeakCash = offer.UnlockCash
	if step(w, s); len(w.Shipments) != 0 || w.Stock(r.From, product) != 0 {
		t.Fatalf("the road spent the float: %+v %d", w.Shipments, w.Stock(r.From, product))
	}
	w.Player.DirtyCash = s.Float() + 10_000_000
	days := s.Days(w, r, events.ShipNormal)
	var bought []events.WholesaleBought
	for day := 0; day < days+5; day++ {
		for _, e := range step(w, s) {
			if ev, ok := e.(events.WholesaleBought); ok {
				bought = append(bought, ev)
				if ev.City != r.From || ev.Route != r.ID || ev.Product != product || ev.Units != ev.Lots*offer.Lot || ev.Lots <= 0 || ev.Cost <= 0 {
					t.Fatalf("bought: %+v", ev)
				}
			}
		}
		for _, sh := range w.Shipments {
			if sh.Units > r.Capacity || sh.Units <= 0 {
				t.Fatalf("day %d: %d units on %s (capacity %d)", w.Day, sh.Units, r.ID, r.Capacity)
			}
		}
		if have := w.Stock(r.To, product) + w.Bound(r.To, product); have > target+offer.Lot {
			t.Fatalf("day %d: %d at the far end and on the road, target %d", w.Day, have, target)
		}
		if rem := w.Stock(r.From, product); rem >= offer.Lot {
			t.Fatalf("day %d: %d left on the dock", w.Day, rem)
		}
	}
	if len(bought) == 0 {
		t.Fatal("nothing bought by the lot")
	}
	if w.Stock(r.To, product) < target || w.Stock(r.To, product) >= target+offer.Lot {
		t.Fatalf("far end holds %d, target %d", w.Stock(r.To, product), target)
	}
	if w.Stock(r.To, product) != w.Stats.Shipped || w.Stats.Shipments < 3 {
		t.Fatalf("shipped %d in %d, far end %d", w.Stats.Shipped, w.Stats.Shipments, w.Stock(r.To, product))
	}
	// Every day on the books paid a fare, and some bought lots.
	lots := 0
	for _, d := range w.Logistics.Days {
		if d.Fares <= 0 || d.Route != r.ID {
			t.Fatalf("the books: %+v", d)
		}
		lots += d.Wholesale
	}
	if lots == 0 {
		t.Fatalf("no lots on the books: %+v", w.Logistics.Days)
	}
	// Met, the route rests; short again, it sends the difference out of
	// the remainder of the last lot, buying nothing.
	step(w, s)
	if len(w.Shipments) != 0 {
		t.Fatalf("a met target still sent: %+v", w.Shipments)
	}
	rem := w.Stock(r.From, product)
	short := min(7, rem)
	w.TakeStock(r.To, product, short)
	before := w.Player.DirtyCash
	bought = nil
	for _, e := range step(w, s) {
		if ev, ok := e.(events.WholesaleBought); ok {
			bought = append(bought, ev)
		}
	}
	if len(bought) != 0 || len(w.Shipments) != 1 || w.Shipments[0].Units != short || w.Player.DirtyCash != before-short*r.Cost || w.Stock(r.From, product) != rem-short {
		t.Fatalf("short by %d: bought %+v sent %+v, cash %d -> %d, left %d", short, bought, w.Shipments, before, w.Player.DirtyCash, w.Stock(r.From, product))
	}
	for len(w.Shipments) > 0 {
		step(w, s)
	}
	// Out of the float a lot is not bought, though what is stashed still
	// goes if the fare fits; with a lot's price and its fare over the
	// float, one lot is bought and the capacity goes.
	rem = w.Stock(r.From, product)
	w.TakeStock(r.To, product, 2*r.Capacity)
	unit := offer.Price[product]
	lotCost := int(unit * float64(offer.Lot))
	w.Player.DirtyCash = s.Float() + lotCost - 1
	bought = nil
	for _, e := range step(w, s) {
		if ev, ok := e.(events.WholesaleBought); ok {
			bought = append(bought, ev)
		}
	}
	if len(bought) != 0 || len(w.Shipments) != 1 || w.Shipments[0].Units != min(rem, r.Capacity) || w.Player.DirtyCash < s.Float() {
		t.Fatalf("under a lot's price: bought %+v sent %+v cash %d (float %d)", bought, w.Shipments, w.Player.DirtyCash, s.Float())
	}
	w.Player.DirtyCash = s.Float() + lotCost + r.Capacity*r.Cost + 1
	bought = nil
	for _, e := range step(w, s) {
		if ev, ok := e.(events.WholesaleBought); ok {
			bought = append(bought, ev)
		}
	}
	if len(bought) != 1 || bought[0].Lots != 1 || len(w.Shipments) != 2 || w.Shipments[1].Units != r.Capacity || w.Player.DirtyCash < s.Float() {
		t.Fatalf("a lot's budget: bought %+v sent %+v cash %d (float %d)", bought, w.Shipments, w.Player.DirtyCash, s.Float())
	}
}

// StartingCities prices every city's ladder for it, and Migrate lays out
// a missing city with every product the run has unlocked.
func TestStartingCitiesAndMigrate(t *testing.T) {
	cfg := content.MustLoad()
	cities := logistics.StartingCities(cfg.City, cfg.Market)
	if len(cities) != len(cfg.City.Cities) || cities[0].ID != cfg.City.Home().ID {
		t.Fatalf("cities %+v", cities)
	}
	for i, c := range cities {
		entry := cfg.City.Cities[i]
		if c.HeatMul != entry.HeatMul() || c.Wholesale != entry.Wholesale {
			t.Fatalf("%s: %+v", c.ID, c)
		}
		for _, p := range c.Products {
			pc := cfg.Market.Product(p.ID)
			if pc.UnlockCash > cfg.Market.Market.StartCash || p.Price != pc.BasePrice*entry.Product(p.ID).Price || p.Demand != pc.Demand*entry.Product(p.ID).Demand {
				t.Fatalf("%s %s: %+v", c.ID, p.ID, p)
			}
		}
	}
	// A world with only the home city, and a rung unlocked, gets the rest.
	home := cities[0]
	w := game.NewWorld(3, []game.StartingCity{home}, 500, 100)
	top := cfg.Market.Products[len(cfg.Market.Products)-1]
	w.AddProduct(home.ID, game.StartingProduct{ID: top.ID, Name: top.Name, Price: top.BasePrice, Demand: top.Demand})
	s := logistics.New(cfg)
	s.Migrate(w)
	if len(w.CityOrder) != len(cfg.City.Cities) || w.Player.Location != home.ID {
		t.Fatalf("migrated: %v in %s", w.CityOrder, w.Player.Location)
	}
	for _, cid := range w.CityOrder[1:] {
		c := w.Cities[cid]
		if len(c.Market) != len(w.Products) || c.Market[top.ID] == nil {
			t.Fatalf("%s: %d products of %d, top rung %v", cid, len(c.Market), len(w.Products), c.Market[top.ID])
		}
		if want := top.BasePrice * cfg.City.City(cid).Product(top.ID).Price; c.Market[top.ID].Price != want {
			t.Fatalf("%s: %s at %.2f, want %.2f", cid, top.ID, c.Market[top.ID].Price, want)
		}
	}
	// Running it again changes nothing.
	s.Migrate(w)
	if len(w.CityOrder) != len(cfg.City.Cities) {
		t.Fatalf("migrated twice: %v", w.CityOrder)
	}
}

// The Logistics branch (#119): each node moves the one number it names
// and nothing else, read through the sim's own accessors, which are what
// the map, the pane and the dice use. The drivers never take a route
// under a day; the forwarder's fare is a price, so a dollar fare halves
// to fifty cents.
func TestLogisticsNodesMoveTheirNumbers(t *testing.T) {
	cfg := content.MustLoad()
	s := logistics.New(cfg)
	type numbers struct {
		days, capacity int
		risk, fare     float64
	}
	read := func(w *game.World, r content.RouteConfig, d events.Ship) numbers {
		return numbers{s.Days(w, r, d), s.Capacity(w, r), s.DayRisk(w, r, d), s.Fare(w, r)}
	}
	own := func(ids ...string) *game.World {
		w := game.NewWorld(7, logistics.StartingCities(cfg.City, cfg.Market), 100_000, 100)
		w.Upgrades = map[string]bool{}
		for _, id := range ids {
			if cfg.Upgrades.Upgrade(id) == nil {
				t.Fatalf("no node %s", id)
			}
			w.Upgrades[id] = true
		}
		return w
	}
	plain := own()
	cases := []struct {
		node string
		want func(base numbers, r content.RouteConfig, d events.Ship) numbers
	}{
		{"tyres", func(b numbers, r content.RouteConfig, d events.Ship) numbers { b.risk *= 0.8; return b }},
		{"compartments", func(b numbers, r content.RouteConfig, d events.Ship) numbers { b.risk *= 0.7; return b }},
		{"trucks", func(b numbers, r content.RouteConfig, d events.Ship) numbers {
			b.capacity = int(math.Round(float64(r.Capacity) * 1.5))
			return b
		}},
		{"drivers", func(b numbers, r content.RouteConfig, d events.Ship) numbers {
			b.days = max(1, int(math.Round(float64(r.Days)*s.Dial(d).Days*0.75)))
			return b
		}},
		{"ticket", func(b numbers, r content.RouteConfig, d events.Ship) numbers { return b }}, // the wholesaler's price is the market sim's (#72): TestTicketFoldsOnTheWholesaler
		{"forwarder", func(b numbers, r content.RouteConfig, d events.Ship) numbers { b.fare *= 0.5; return b }},
	}
	near := func(a, b numbers) bool {
		return a.days == b.days && a.capacity == b.capacity && math.Abs(a.risk-b.risk) < 1e-12 && math.Abs(a.fare-b.fare) < 1e-12
	}
	for _, tc := range cases {
		w := own(tc.node)
		for _, r := range cfg.Routes.Routes {
			for _, d := range []events.Ship{events.ShipSlow, events.ShipNormal, events.ShipFast} {
				base := read(plain, r, d)
				got, want := read(w, r, d), tc.want(base, r, d)
				if !near(got, want) {
					t.Errorf("%s on %s at %s: %+v, want %+v (bare %+v)", tc.node, r.ID, d, got, want, base)
				}
				if got.days < 1 {
					t.Errorf("%s on %s at %s: %d days", tc.node, r.ID, d, got.days)
				}
			}
		}
	}
	// The two risk nodes stack as a product, and the road's risk over the
	// days is what the day risk compounds to.
	w := own("tyres", "compartments")
	for _, r := range cfg.Routes.Routes {
		if got, want := s.DayRisk(w, r, events.ShipNormal), s.DayRisk(plain, r, events.ShipNormal)*0.8*0.7; math.Abs(got-want) > 1e-12 {
			t.Errorf("tyres and compartments on %s: day risk %.5f, want %.5f", r.ID, got, want)
		}
		if got, want := s.Risk(w, r, events.ShipNormal), 1-math.Pow(1-s.DayRisk(w, r, events.ShipNormal), float64(s.Days(w, r, events.ShipNormal))); math.Abs(got-want) > 1e-12 {
			t.Errorf("risk on %s: %.5f, want %.5f", r.ID, got, want)
		}
	}
	// Drivers on a two-day car at fast: never under a day.
	fast := content.RouteConfig{ID: "sprint", Days: 1, Capacity: 10, Cost: 1, Risk: 0.1, Mode: "car", From: "a", To: "b"}
	if got := s.Days(own("drivers"), fast, events.ShipFast); got != 1 {
		t.Errorf("a one-day route at fast with drivers takes %d days", got)
	}
}

// The road runs on the folded numbers (#119): with the trucks a route
// sends half as much again a day, with the forwarder the shipment's
// fare is half, rounded up to the dollar, and the road still never
// spends under the float.
func TestRunReadsTheTree(t *testing.T) {
	cfg := content.MustLoad()
	send := func(ids ...string) (game.Shipment, *game.World) {
		w, s, r := world(t, cfg, 0)
		product := w.Products[0]
		w.Upgrades = map[string]bool{}
		for _, id := range ids {
			w.Upgrades[id] = true
		}
		w.SetStock(r.From, product, 10*r.Capacity)
		_ = w.SetRouteTarget(r.ID, product, 10*r.Capacity)
		_ = w.SetRoute(r.ID, events.RouteNormal)
		w.Player.DirtyCash = s.Float() + 1_000_000
		step(w, s)
		if len(w.Shipments) != 1 {
			t.Fatalf("%v: %d shipments", ids, len(w.Shipments))
		}
		return w.Shipments[0], w
	}
	r := cfg.Routes.Routes[0]
	plain, _ := send()
	if plain.Units != r.Capacity || plain.Cost != r.Capacity*r.Cost {
		t.Fatalf("bare: %+v", plain)
	}
	trucks, _ := send("trucks")
	if want := int(math.Round(float64(r.Capacity) * 1.5)); trucks.Units != want || trucks.Cost != want*r.Cost {
		t.Fatalf("trucks: %+v, want %d units at $%d", trucks, want, want*r.Cost)
	}
	fwd, _ := send("drivers", "trucks", "forwarder")
	if want := int(math.Ceil(float64(trucks.Units*r.Cost) * 0.5)); fwd.Cost != want || fwd.Units != trucks.Units {
		t.Fatalf("forwarder: %+v, want %d units at $%d", fwd, trucks.Units, want)
	}
	if got, want := fwd.Arrives-fwd.Sent, max(1, int(math.Round(float64(r.Days)*0.75))); got != want {
		t.Fatalf("drivers: %d days, want %d", got, want)
	}
	// The float: on the dollar-a-unit boat with the forwarder, fifty
	// cents a unit, three dollars over the float send six units and the
	// till stays at the float; two dollars and fifty cents' worth never
	// goes a cent under it.
	w, s, _ := world(t, cfg, 0)
	var boat *content.RouteConfig
	for _, r := range cfg.Routes.Routes {
		if r.Cost == 1 {
			rc := r
			boat = &rc
		}
	}
	if boat == nil {
		t.Skip("no route charges a dollar a unit")
	}
	product := w.Products[0]
	w.Upgrades = map[string]bool{"trucks": true, "drivers": true, "forwarder": true}
	w.SetStock(boat.From, product, 400)
	_ = w.SetRouteTarget(boat.ID, product, 400)
	_ = w.SetRoute(boat.ID, events.RouteNormal)
	w.Player.DirtyCash = s.Float() + 3
	step(w, s)
	if len(w.Shipments) != 1 || w.Shipments[0].Units != 6 || w.Shipments[0].Cost != 3 || w.Player.DirtyCash != s.Float() {
		t.Fatalf("three dollars over the float: %+v, cash %d (float %d)", w.Shipments, w.Player.DirtyCash, s.Float())
	}
}

// A days target (#115) is read each morning as days times what the
// corners at the far end serve (World.Demand there), rounded up, with no
// dice: nothing while nobody works a corner there, more as corners are
// worked, and the road sends the shortfall against it the way it does
// against units. The units read (Sim.Target) is what the dialog and the
// pane show and what run sends against.
func TestDaysTargetIsDemand(t *testing.T) {
	cfg := content.MustLoad()
	w, s, r := world(t, cfg, 0)
	product := w.Products[0]
	// Nobody on any corner at the far end: a days target is nothing.
	for _, c := range w.Corners() {
		if c.Held() {
			_ = w.Abandon(c.ID)
		}
	}
	_ = w.SetRouteDays(r.ID, product, 3)
	_ = w.SetRoute(r.ID, events.RouteNormal)
	if got := s.Target(w, r, product); got != 0 || s.Shortfall(w, r, product) != 0 {
		t.Fatalf("3 days of nothing: target %d shortfall %d", got, s.Shortfall(w, r, product))
	}
	if step(w, s); len(w.Shipments) != 0 {
		t.Fatalf("the road sent %+v against a days target with no demand", w.Shipments)
	}
	// One corner worked at the far end, then two: the target follows.
	w.Player.Location = r.To
	corners := w.Cities[r.To].Corners
	if err := w.Post(corners[0].ID, game.You); err != nil {
		t.Fatal(err)
	}
	one := s.Target(w, r, product)
	if want := int(math.Ceil(3*w.Demand(r.To, product) - 1e-9)); one != want || one <= 0 {
		t.Fatalf("one corner: target %d, want %d (3 x %.2f)", one, want, w.Demand(r.To, product))
	}
	if got := s.DaysTarget(w, r, product, 3); got != one {
		t.Fatalf("DaysTarget %d, Target %d", got, one)
	}
	if got := s.Shortfall(w, r, product); got != one-w.Stock(r.To, product) {
		t.Fatalf("shortfall %d, target %d over %d there", got, one, w.Stock(r.To, product))
	}
	w.Crew.Members = append(w.Crew.Members, game.CrewMember{ID: 900, Name: "Runner", Role: "runner", Skill: 60, Units: 100, Loyalty: 80, Nerve: 50, Wage: 50})
	if err := w.Post(corners[1].ID, 900); err != nil {
		t.Fatal(err)
	}
	two := s.Target(w, r, product)
	if two <= one {
		t.Fatalf("two corners: target %d, one corner %d", two, one)
	}
	// The setting is untouched: the sim sized the units.
	if rs := w.Route(r.ID); rs.Days[product] != 3 || rs.Target != nil {
		t.Fatalf("the setting moved: %+v", rs)
	}
	// The road sends the shortfall against today's read, up to the
	// route's capacity, out of the source stash.
	w.Player.DirtyCash = s.Float() + 1_000_000
	short := s.Shortfall(w, r, product)
	evs := step(w, s)
	sent := 0
	for _, sh := range w.Shipments {
		sent += sh.Units
	}
	if want := min(short, r.Capacity); sent != want || len(evs) == 0 {
		t.Fatalf("sent %d against a shortfall of %d (capacity %d)", sent, short, r.Capacity)
	}
	// Units are the other kind, and setting them puts the days away.
	_ = w.SetRouteTarget(r.ID, product, 7)
	if got := s.Target(w, r, product); got != 7 || w.Route(r.ID).Days != nil {
		t.Fatalf("units after days: target %d, setting %+v", got, w.Route(r.ID))
	}
}
