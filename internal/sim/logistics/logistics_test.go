package logistics_test

import (
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/sim/logistics"
	"github.com/theclifmeister/kingpin/internal/sim/territory"
)

// world is the config's cities with the player at home and a stash in
// both, and the sim over the routes with the risk set as asked.
func world(t *testing.T, cfg *content.Config, risk float64) (*game.World, *logistics.Sim, string, string) {
	t.Helper()
	routes := cfg.Routes
	routes.Routes = append([]content.RouteConfig(nil), cfg.Routes.Routes...)
	for i := range routes.Routes {
		routes.Routes[i].Risk = risk
	}
	s := logistics.New(routes, cfg.City, cfg.Market)
	w := game.NewWorld(7, logistics.StartingCities(cfg.City, cfg.Market), 100_000, 100)
	territory.New(cfg.City).Seed(w)
	home := cfg.City.Home().ID
	hub := ""
	for _, r := range routes.Routes {
		if o := r.Other(home); o != "" {
			hub = o
		}
	}
	if hub == "" {
		t.Fatal("no route out of home")
	}
	w.Stash(home)[w.Products[0]] = 500
	return w, s, home, hub
}

func step(w *game.World, s *logistics.Sim) []events.Event {
	t := &game.Tick{Day: w.Day + 1, RNG: game.RNGFor(w.Seed, w.Day+1), Seed: w.Seed}
	s.Step(w, t)
	w.Day++
	return t.Events()
}

// The dial scales a route: fast is fewer days at more risk per day, slow
// the reverse, and never under a day; the risk the dialog shows is what
// the days add up to.
func TestDialsAndOffers(t *testing.T) {
	cfg := content.MustLoad()
	s := logistics.New(cfg.Routes, cfg.City, cfg.Market)
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
		o := s.Offer(r, events.ShipFast)
		if o.ID != r.ID || o.Days != fast || o.Capacity != r.Capacity || o.Cost != r.Cost || o.Dial != events.ShipFast || !r.Connects(o.From, o.To) {
			t.Fatalf("%s: offer %+v", r.ID, o)
		}
	}
	if o := s.Wholesale(); o.Lot != cfg.Routes.Wholesale.Lot || o.Mul != cfg.Routes.Wholesale.Mul || o.UnlockCash != cfg.Routes.Wholesale.UnlockCash {
		t.Fatalf("wholesale offer %+v", o)
	}
	if s.Route("nowhere") != nil || len(s.Routes("nowhere")) != 0 {
		t.Fatal("a route that does not exist")
	}
}

// On a safe road a shipment is reported the morning after it leaves,
// rolls nothing, and lands on its day in the destination's stash; on a
// road that always intercepts it is seized on the first morning, the
// seizure is recorded for the market and the stats count it.
func TestStepLandsOrSeizes(t *testing.T) {
	cfg := content.MustLoad()
	for _, risk := range []float64{0, 1} {
		w, s, home, hub := world(t, cfg, risk)
		product := w.Products[0]
		r := s.Routes(home)[0]
		if _, err := s.Ship(w, "nowhere", home, hub, product, 10, events.ShipNormal); err != game.ErrNoRoute {
			t.Fatalf("shipped on no route: %v", err)
		}
		sh, err := s.Ship(w, r.ID, home, hub, product, 10, events.ShipSlow)
		if err != nil {
			t.Fatal(err)
		}
		days := s.Days(r, events.ShipSlow)
		if sh.Arrives != days || sh.Dial != events.ShipSlow {
			t.Fatalf("shipment %+v, want %d days", sh, days)
		}
		var sent, arrived, seized int
		for day := 0; day < days+1; day++ {
			for _, e := range step(w, s) {
				switch ev := e.(type) {
				case events.ShipmentSent:
					sent++
					if ev.Day != 1 || ev.Days != days || ev.ID != sh.ID {
						t.Fatalf("sent: %+v", ev)
					}
				case events.ShipmentArrived:
					arrived++
					if ev.Day != days || ev.Units != 10 {
						t.Fatalf("arrived: %+v", ev)
					}
				case events.ShipmentSeized:
					seized++
					if ev.Day != 1 || ev.Units != 10 || ev.Dial != events.ShipSlow {
						t.Fatalf("seized: %+v", ev)
					}
				}
			}
		}
		switch {
		case sent != 1:
			t.Fatalf("risk %.0f: %d sent", risk, sent)
		case risk == 0 && (arrived != 1 || seized != 0 || w.Stock(hub, product) != 10 || len(w.Shipments) != 0 || len(w.Logistics.Seizures) != 0):
			t.Fatalf("safe: %d arrived %d seized, hub %d, %d on the road", arrived, seized, w.Stock(hub, product), len(w.Shipments))
		case risk == 1 && (arrived != 0 || seized != 1 || w.Stock(hub, product) != 0 || len(w.Shipments) != 0 || len(w.Logistics.Seizures) != 1):
			t.Fatalf("seized: %d arrived %d seized, hub %d, %d on the road, record %+v", arrived, seized, w.Stock(hub, product), len(w.Shipments), w.Logistics.Seizures)
		}
		if risk == 1 {
			z := w.Logistics.Seizures[0]
			if z.Day != 1 || z.To != hub || z.From != home || z.Product != product || z.Units != 10 || w.Stats.Seizures != 1 || w.Stats.SeizedOnRoad != 10 {
				t.Fatalf("record %+v stats %+v", z, w.Stats)
			}
			// The record is kept for a while, then dropped.
			for w.Day < cfg.Routes.Shipping.RecordDays+2 {
				step(w, s)
			}
			if len(w.Logistics.Seizures) != 0 {
				t.Fatalf("record kept %d days: %+v", w.Day, w.Logistics.Seizures)
			}
		}
		if w.Stock(home, product) != 490 {
			t.Fatalf("home stash %d", w.Stock(home, product))
		}
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
	s := logistics.New(cfg.Routes, cfg.City, cfg.Market)
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
