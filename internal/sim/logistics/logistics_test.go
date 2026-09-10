package logistics_test

import (
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/sim/logistics"
	"github.com/theclifmeister/kingpin/internal/sim/territory"
)

// world is the config's cities with the player at home, the first
// route's source stashed with 500 of the first product, and the sim over
// the routes with the risk set as asked. The float is the config's.
func world(t *testing.T, cfg *content.Config, risk float64) (*game.World, *logistics.Sim, content.RouteConfig) {
	t.Helper()
	routes := cfg.Routes
	routes.Routes = append([]content.RouteConfig(nil), cfg.Routes.Routes...)
	for i := range routes.Routes {
		routes.Routes[i].Risk = risk
	}
	s := logistics.New(routes, cfg.City, cfg.Market, cfg.Upgrades, cfg.Laundering.Laundering.Float)
	w := game.NewWorld(7, logistics.StartingCities(cfg.City, cfg.Market), 100_000, 100)
	territory.New(cfg.City).Seed(w)
	if len(routes.Routes) == 0 {
		t.Fatal("no routes")
	}
	r := routes.Routes[0]
	w.Stash(r.From)[w.Products[0]] = 500
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
	s := logistics.New(cfg.Routes, cfg.City, cfg.Market, cfg.Upgrades, cfg.Laundering.Laundering.Float)
	for _, r := range cfg.Routes.Routes {
		slow, normal, fast := s.Days(r, events.ShipSlow), s.Days(r, events.ShipNormal), s.Days(r, events.ShipFast)
		if normal != r.Days || fast > normal || slow < normal || fast < 1 {
			t.Fatalf("%s: days slow %d normal %d fast %d (route %d)", r.ID, slow, normal, fast, r.Days)
		}
		if s.DayRisk(r, events.ShipFast) <= s.DayRisk(r, events.ShipNormal) || s.DayRisk(r, events.ShipSlow) >= s.DayRisk(r, events.ShipNormal) {
			t.Fatalf("%s: day risk slow %.3f normal %.3f fast %.3f", r.ID, s.DayRisk(r, events.ShipSlow), s.DayRisk(r, events.ShipNormal), s.DayRisk(r, events.ShipFast))
		}
		if risk := s.Risk(r, events.ShipNormal); risk <= 0 || risk >= 1 || risk < s.DayRisk(r, events.ShipNormal) {
			t.Fatalf("%s: risk %.3f over %d days at %.3f a day", r.ID, risk, normal, s.DayRisk(r, events.ShipNormal))
		}
		if !r.Connects(r.From, r.To) || r.Other(r.From) != r.To {
			t.Fatalf("%s: joins %s and %s", r.ID, r.From, r.To)
		}
	}
	if o := s.Wholesale(); o.Lot != cfg.Routes.Wholesale.Lot || o.Mul != cfg.Routes.Wholesale.Mul || o.UnlockCash != cfg.Routes.Wholesale.UnlockCash {
		t.Fatalf("wholesale offer %+v", o)
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
		days := s.Days(r, events.ShipSlow)
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
	offer := s.Wholesale()
	if !w.Cities[r.From].Wholesale {
		t.Skipf("%s does not sell by the lot", r.From)
	}
	target := 3*r.Capacity + 25
	_ = w.SetRouteTarget(r.ID, product, target)
	_ = w.SetRoute(r.ID, events.RouteNormal)
	w.Stash(r.From)[product] = 0
	w.Player.DirtyCash = s.Float() // nothing over the float: nothing moves
	w.Stats.PeakCash = offer.UnlockCash
	if step(w, s); len(w.Shipments) != 0 || w.Stock(r.From, product) != 0 {
		t.Fatalf("the road spent the float: %+v %d", w.Shipments, w.Stock(r.From, product))
	}
	w.Player.DirtyCash = s.Float() + 10_000_000
	days := s.Days(r, events.ShipNormal)
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
	w.Stash(r.To)[product] -= short
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
	w.Stash(r.To)[product] -= 2 * r.Capacity
	unit := w.Product(r.From, product).SupplierPrice * offer.Mul
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
	s := logistics.New(cfg.Routes, cfg.City, cfg.Market, cfg.Upgrades, cfg.Laundering.Laundering.Float)
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
