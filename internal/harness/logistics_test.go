package harness

import (
	"fmt"
	"sort"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/sim"
	"github.com/theclifmeister/kingpin/internal/sim/logistics"
)

// twoCities is the home city and the one with a route into it, from the
// config, and the route between them with the most room. A route runs
// from its source to its destination, so the hub is the route's From and
// home its To.
func twoCities(t *testing.T, cfg *content.Config) (home, hub string, route content.RouteConfig) {
	t.Helper()
	home = cfg.City.Home().ID
	lg := newLogistics(cfg)
	for _, c := range cfg.City.Cities {
		if c.ID == home {
			continue
		}
		for _, r := range lg.Routes(c.ID) {
			if r.From == c.ID && r.To == home && r.Capacity > route.Capacity {
				hub, route = c.ID, r
			}
		}
	}
	if hub == "" {
		t.Fatal("no route runs into the home city from another")
	}
	return home, hub, route
}

// newLogistics is the logistics sim as sim.Default builds it.
func newLogistics(cfg *content.Config) *logistics.Sim {
	return logistics.New(cfg)
}

// runRoute is a policy that turns the route on at the dial with a target
// for the product at its far end on day 0 and off again on day 1, so
// exactly one shipment of target units goes (the far end is empty and
// nothing sells in a quiet world), and the route never sends it again
// if it is seized.
func runRoute(route content.RouteConfig, product string, units int, d events.RouteDial) Policy {
	return func(w *game.World) {
		switch w.Day {
		case 0:
			_ = w.SetRouteTarget(route.ID, product, units)
			_ = w.SetRoute(route.ID, d)
		case 1:
			_ = w.SetRoute(route.ID, events.RouteOff)
		}
	}
}

// quiet is a world with nobody on any corner, so nothing sells and
// nothing is robbed: only the road moves stock.
func quiet(cfg *content.Config, seed uint64) *game.World {
	w := sim.NewWorld(cfg, seed)
	for _, c := range w.Corners() {
		if c.Held() {
			_ = w.Abandon(c.ID)
		}
	}
	w.Player.DirtyCash = 100_000
	return w
}

// Stock is conserved across a shipment: what left the source stash is on
// the road until it lands in the destination's, and the three add up to
// the same number on every day of the trip. A seized shipment never
// arrives: it comes off the road and lands nowhere.
func TestStockIsConservedAcrossShipments(t *testing.T) {
	cfg := content.MustLoad()
	home, hub, route := twoCities(t, cfg)
	product := cfg.Market.Products[0].ID
	lg := newLogistics(cfg)
	for _, housed := range []bool{false, true} {
		for _, seized := range []bool{false, true} {
			safe := *cfg
			safe.Routes.Routes = append([]content.RouteConfig(nil), cfg.Routes.Routes...)
			for i := range safe.Routes.Routes {
				safe.Routes.Routes[i].Risk = 0
				if seized {
					safe.Routes.Routes[i].Risk = 1
				}
			}
			w := quiet(&safe, 1)
			w.SetStock(hub, product, 500)
			units := min(200, route.Capacity)
			// With houses at both ends (#73) the shipment leaves the hub's
			// street first and lands in the emptiest house at home with
			// room, the rest on the street, never over a capacity.
			if housed {
				safe.City.Territory.RobberyChance = 0 // the houses' robbers are TestGuardIsWorthItsWage's
				w.Player.CleanCash = 100_000
				for _, o := range []game.HouseOffer{
					{ID: "hubhouse", Name: "Hub house", City: hub, Corner: cfg.City.City(hub).Corners[0].ID, Capacity: 300, Price: 1, Rent: 1},
					{ID: "homehouse", Name: "Home house", City: home, Corner: cfg.City.Home().Corners[0].ID, Capacity: units / 2, Price: 1, Rent: 1},
				} {
					if _, err := w.BuyHouse(o); err != nil {
						t.Fatal(err)
					}
				}
				w.MoveStock(hub, game.Street, "hubhouse", product, 300)
			}
			run := runRoute(route, product, units, events.RouteNormal)
			res, err := RunFrom(&safe, w, route.Days*2+3, func(w *game.World) {
				run(w)
				if w.Day == 1 {
					if len(w.Shipments) != 1 || w.Stock(hub, product) != 500-units || w.InTransit(product) != units || w.Bound(home, product) != units {
						t.Fatalf("the morning after the dial: %+v stash %d road %d", w.Shipments, w.Stock(hub, product), w.InTransit(product))
					}
					if housed && w.Street(hub, product) != 0 && w.House("hubhouse").Units() != 300 {
						t.Fatalf("housed: the shipment did not leave the street first: street %d house %d", w.Street(hub, product), w.House("hubhouse").Units())
					}
				}
				if total := w.Stock(home, product) + w.Stock(hub, product) + w.InTransit(product); total != 500 && !seized {
					t.Fatalf("day %d: %d + %d + %d on the road = %d, not 500", w.Day, w.Stock(home, product), w.Stock(hub, product), w.InTransit(product), total)
				}
				for _, h := range w.Houses {
					if h.Units() > h.Capacity {
						t.Fatalf("day %d: %s holds %d of %d", w.Day, h.Name, h.Units(), h.Capacity)
					}
				}
				if housed && !seized && w.Stock(home, product) == units && (w.House("homehouse").Units() != units/2 || w.Street(home, product) != units-units/2) {
					t.Fatalf("housed: the landing was not house-first: house %d street %d", w.House("homehouse").Units(), w.Street(home, product))
				}
			})
			if err != nil {
				t.Fatal(err)
			}
			sent, arrived, lost, bought := 0, 0, 0, 0
			for _, e := range res.Events {
				switch ev := e.(type) {
				case events.ShipmentSent:
					sent++
					if ev.Day != 1 || ev.Units != units || ev.From != hub || ev.To != home || ev.Dial != events.ShipNormal {
						t.Fatalf("sent %+v, want day 1 %d units %s to %s", ev, units, hub, home)
					}
				case events.ShipmentArrived:
					arrived++
					if ev.Day != 1+lg.Days(res.World, route, events.ShipNormal) || ev.Units != units || ev.To != home {
						t.Fatalf("arrival %+v, want day %d %d units in %s", ev, 1+lg.Days(res.World, route, events.ShipNormal), units, home)
					}
				case events.ShipmentSeized:
					lost++
				case events.WholesaleBought:
					bought++
				}
			}
			if sent != 1 || bought != 0 {
				t.Fatalf("%d sent, %d lots bought: the stash covered the target", sent, bought)
			}
			switch {
			case !seized && (arrived != 1 || lost != 0 || res.World.Stock(home, product) != units || res.World.InTransit(product) != 0):
				t.Fatalf("safe road: %d arrived %d seized, home holds %d, %d on the road", arrived, lost, res.World.Stock(home, product), res.World.InTransit(product))
			case seized && (arrived != 0 || lost != 1 || res.World.Stock(home, product) != 0 || res.World.InTransit(product) != 0 || res.World.Stock(hub, product) != 500-units):
				t.Fatalf("seized: %d arrived %d seized, home holds %d, hub %d, %d on the road", arrived, lost, res.World.Stock(home, product), res.World.Stock(hub, product), res.World.InTransit(product))
			}
			if seized && (res.World.Stats.Seizures != 1 || res.World.Stats.SeizedOnRoad != units) {
				t.Fatalf("stats: %+v", res.World.Stats)
			}
		}
	}
}

// No shipment ever exceeds its route's capacity, over a whole run of the
// policy that ships the most; and every one that leaves either lands or
// is seized, never both.
func TestNoShipmentExceedsCapacity(t *testing.T) {
	cfg := content.MustLoad()
	sent := 0
	for seed := uint64(1); seed <= 3; seed++ {
		res, err := Run(cfg, seed, Horizon, Distributor(cfg, 40))
		if err != nil {
			t.Fatal(err)
		}
		fate := map[int]string{}
		for _, e := range res.Events {
			switch ev := e.(type) {
			case events.ShipmentSent:
				sent++
				r := cfg.Routes.Route(ev.Route)
				if r == nil || ev.Units > r.Capacity || ev.Units <= 0 {
					t.Fatalf("seed %d day %d: %d units on %s (capacity %d)", seed, ev.Day, ev.Units, ev.Route, r.Capacity)
				}
			case events.ShipmentArrived:
				if fate[ev.ID] != "" {
					t.Fatalf("seed %d: shipment %d %s and then arrived", seed, ev.ID, fate[ev.ID])
				}
				fate[ev.ID] = "arrived"
			case events.ShipmentSeized:
				if fate[ev.ID] != "" {
					t.Fatalf("seed %d: shipment %d %s and then was seized", seed, ev.ID, fate[ev.ID])
				}
				fate[ev.ID] = "seized"
			}
		}
	}
	if sent < 50 {
		t.Fatalf("the distributor only sent %d shipments over three runs", sent)
	}
}

// Sent fast, a shipment is seized more often than sent slow, over 200
// days of shipping every day on the same seed.
func TestFastIsSeizedMoreThanSlow(t *testing.T) {
	cfg := content.MustLoad()
	cfg.Heat.Heat.EvidenceArrest = 0 // a fast seizure is a page in the file; this measures the road, not the case
	home, hub, route := twoCities(t, cfg)
	product := cfg.Market.Products[0].ID
	lg := newLogistics(cfg)
	units := min(50, route.Capacity)
	count := func(d events.RouteDial) (seized, sent int) {
		for seed := uint64(1); seed <= 3; seed++ {
			w := quiet(cfg, seed)
			res, err := RunFrom(cfg, w, Horizon, func(w *game.World) {
				// The fare over the float, never a pile that draws the
				// police; the source stocked, the far end drained, and
				// the target units past what is on the road, so units
				// go every day.
				w.Player.DirtyCash = cfg.Heat.Heat.DirtyCashThreshold
				w.SetStock(hub, product, 10_000)
				w.SetStock(home, product, 0)
				_ = w.SetRouteTarget(route.ID, product, w.Bound(home, product)+units)
				_ = w.SetRoute(route.ID, d)
			})
			if err != nil {
				t.Fatal(err)
			}
			if res.Over != nil {
				t.Fatalf("seed %d: the shipper ended on day %d: %s", seed, res.Days, res.Over.Cause)
			}
			for _, e := range res.Events {
				switch e.(type) {
				case events.ShipmentSeized:
					seized++
				case events.ShipmentSent:
					sent++
				}
			}
		}
		return seized, sent
	}
	slowSeized, slowSent := count(events.RouteSlow)
	fastSeized, fastSent := count(events.RouteFast)
	t.Logf("slow: %d of %d seized (%.0f%%); fast: %d of %d (%.0f%%)", slowSeized, slowSent, 100*float64(slowSeized)/float64(slowSent), fastSeized, fastSent, 100*float64(fastSeized)/float64(fastSent))
	if fastSeized <= slowSeized || slowSent < 3*(Horizon-2) || fastSent < 3*(Horizon-2) {
		t.Fatalf("fast was seized %d of %d, slow %d of %d; fast should be the risk and both should send daily", fastSeized, fastSent, slowSeized, slowSent)
	}
	if plain := quiet(cfg, 1); lg.Days(plain, route, events.ShipFast) >= lg.Days(plain, route, events.ShipSlow) {
		t.Fatalf("fast takes %d days, slow %d", lg.Days(plain, route, events.ShipFast), lg.Days(plain, route, events.ShipSlow))
	}
}

// A seizure is a market shock, not a bust (#27): heat rises in both
// cities and the street it was bound for spikes the next morning, but
// the DA's file only grows when the shipment was sent fast.
func TestSeizureIsShockNotEvidence(t *testing.T) {
	cfg := content.MustLoad()
	home, hub, route := twoCities(t, cfg)
	product := cfg.Market.Products[0].ID
	risky := *cfg
	risky.Routes.Routes = append([]content.RouteConfig(nil), cfg.Routes.Routes...)
	for i := range risky.Routes.Routes {
		risky.Routes.Routes[i].Risk = 1
	}
	for _, d := range []events.RouteDial{events.RouteNormal, events.RouteFast} {
		w := quiet(&risky, 2)
		w.SetStock(hub, product, 100)
		// Sent on the first morning, rolled and seized on the second,
		// the shock on the third.
		res, err := RunFrom(&risky, w, 4, runRoute(route, product, 50, d))
		if err != nil {
			t.Fatal(err)
		}
		var heat []events.HeatChanged
		var shocks []events.PriceShock
		seized := 0
		for _, e := range res.Events {
			switch ev := e.(type) {
			case events.ShipmentSeized:
				seized++
			case events.HeatChanged:
				if ev.Day == 2 {
					heat = append(heat, ev)
				}
			case events.PriceShock:
				if ev.Seized {
					shocks = append(shocks, ev)
				}
			case events.Enforcement:
				if ev.Evidence > 0 {
					t.Fatalf("%s: a bust with evidence on a quiet day: %+v", d, ev)
				}
			}
		}
		if seized != 1 {
			t.Fatalf("%s: %d seizures", d, seized)
		}
		for _, h := range heat {
			if h.To-h.From < cfg.Routes.Shipping.SeizureHeat/2 {
				t.Fatalf("%s: %s heat rose %.1f on the seizure, want about %.0f", d, h.City, h.To-h.From, cfg.Routes.Shipping.SeizureHeat)
			}
		}
		if len(heat) != 2 {
			t.Fatalf("%s: heat reported in %d cities", d, len(heat))
		}
		if len(shocks) != 1 || shocks[0].City != home || shocks[0].Product != product || shocks[0].Day != 3 {
			t.Fatalf("%s: shocks %+v, want one in %s the morning after", d, shocks, home)
		}
		if p := res.World.Product(home, product); p.ShockFactor != cfg.Routes.Shipping.ShockFactor || p.ShockSlump {
			t.Fatalf("%s: %s in %s after the seizure: %+v", d, product, home, p)
		}
		want := 0
		if d == events.RouteFast {
			want = cfg.Routes.Shipping.SeizureEvidence
		}
		if res.World.Heat.Evidence != want {
			t.Fatalf("%s: evidence %d after a seizure, want %d", d, res.World.Heat.Evidence, want)
		}
	}
}

// A sale in a city the player is not in is served only by the runners
// posted there: with one on a corner it sells up to that corner's demand,
// with nobody it sells nothing, and you cannot stand on a corner there
// yourself.
func TestSalesElsewhereAreRunnersOnly(t *testing.T) {
	cfg := content.MustLoad()
	home, hub, _ := twoCities(t, cfg)
	product := cfg.Market.Products[0].ID
	set, _, err := sim.Default(cfg)
	if err != nil {
		t.Fatal(err)
	}
	w := sim.NewWorld(cfg, 3)
	w.Player.DirtyCash = 100_000
	w.Crew.Members = []game.CrewMember{{ID: 1, Name: "Dre", Role: "runner", Skill: 60, Units: 120, Loyalty: 90, Nerve: 50, Wage: 50}}
	w.Crew.NextID = 1
	corner := cfg.City.City(hub).Corners[0].ID
	if err := w.Post(corner, game.You); err != game.ErrElsewhere {
		t.Fatalf("stood on %s from %s: %v", corner, home, err)
	}
	if err := w.Post(corner, 1); err != nil {
		t.Fatal(err)
	}
	if w.Capacity(hub) != 120 || w.Capacity(home) != w.Player.CarryLimit {
		t.Fatalf("capacity hub %d home %d", w.Capacity(hub), w.Capacity(home))
	}
	var sold []events.PlayerSold
	res, err := RunFrom(cfg, w, 6, func(w *game.World) {
		if w.Day == 3 {
			w.Recall(1)
		}
		w.SetStock(hub, product, 1000)
		if err := w.PlaceSell(hub, product, 1000, events.DialNormal); err != nil {
			t.Fatal(err)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range res.Events {
		if ps, ok := e.(events.PlayerSold); ok && ps.City == hub {
			sold = append(sold, ps)
		}
	}
	if len(sold) != 6 {
		t.Fatalf("%d sales reported in %s", len(sold), hub)
	}
	for i, ps := range sold {
		if i < 3 && ps.Sold == 0 {
			t.Fatalf("day %d: the runner sold nothing in %s", ps.Day, hub)
		}
		if i < 3 && float64(ps.Sold) > w.Product(hub, product).Demand*cfg.City.City(hub).Corners[0].Demand*3 {
			t.Fatalf("day %d: sold %d, more than one corner absorbs", ps.Day, ps.Sold)
		}
		if i >= 3 && ps.Sold != 0 {
			t.Fatalf("day %d: sold %d in %s with nobody on a corner there", ps.Day, ps.Sold, hub)
		}
	}
	// And the runner's sale was heat there, not at home.
	if h := res.World.City(hub).Heat; h <= 0 {
		t.Fatalf("no heat in %s after three days of selling there", hub)
	}
	_ = set
}

// Two cities with shipments in flight are as deterministic as one: the
// same seed gives the same events, and a save in the middle of a run
// changes nothing about the rest of it.
func TestDistributorIsDeterministicAndSaves(t *testing.T) {
	t.Setenv("KINGPIN_HOME", t.TempDir())
	cfg := content.MustLoad()
	a, err := Run(cfg, 4, 150, Distributor(cfg, 40))
	if err != nil {
		t.Fatal(err)
	}
	b, _ := Run(cfg, 4, 150, Distributor(cfg, 40))
	if len(a.Events) != len(b.Events) {
		t.Fatalf("event counts differ: %d vs %d", len(a.Events), len(b.Events))
	}
	for i := range a.Events {
		if fmt.Sprintf("%#v", a.Events[i]) != fmt.Sprintf("%#v", b.Events[i]) {
			t.Fatalf("event %d differs:\n%#v\n%#v", i, a.Events[i], b.Events[i])
		}
	}
	if a.World.Stats.Shipments < 20 || a.World.Player.Location == cfg.City.Home().ID {
		t.Fatalf("the distributor never left home: %d shipments, in %s", a.World.Stats.Shipments, a.World.Player.Location)
	}
	// Save on day 100 with shipments on the road, load, play on.
	c, _ := Run(cfg, 4, 100, Distributor(cfg, 40))
	if len(c.World.Shipments) == 0 {
		t.Fatal("nothing on the road on day 100 to save")
	}
	if err := game.Save(1, c.World); err != nil {
		t.Fatal(err)
	}
	set, _, err := sim.Default(cfg)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := game.Load(1, set.Migrations()...)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Player.Location != c.World.Player.Location || len(loaded.Shipments) != len(c.World.Shipments) || loaded.Player.TotalStock() != c.World.Player.TotalStock() {
		t.Fatalf("loaded: %s %d shipments %d units; saved %s %d %d", loaded.Player.Location, len(loaded.Shipments), loaded.Player.TotalStock(), c.World.Player.Location, len(c.World.Shipments), c.World.Player.TotalStock())
	}
	if fmt.Sprint(loaded.Routes) != fmt.Sprint(c.World.Routes) || len(loaded.Routes) == 0 {
		t.Fatalf("route dials %v, saved %v", loaded.Routes, c.World.Routes)
	}
	d, _ := RunFrom(cfg, loaded, 50, Distributor(cfg, 40))
	rest := a.Events[len(c.Events):]
	if len(d.Events) != len(rest) {
		t.Fatalf("after loading, %d events for the last 50 days, want %d", len(d.Events), len(rest))
	}
	for i := range rest {
		if fmt.Sprintf("%#v", rest[i]) != fmt.Sprintf("%#v", d.Events[i]) {
			t.Fatalf("event %d after the save differs:\n%#v\n%#v", i, rest[i], d.Events[i])
		}
	}
	if d.World.NetWorth() != a.World.NetWorth() {
		t.Fatalf("net worth %d after the save, %d straight through", d.World.NetWorth(), a.World.NetWorth())
	}
}

// A days target (#115) follows the ground: the route keeps the far end
// at days times what the corners worked there serve, read again each
// morning by the logistics sim, so holding more corners there raises
// what the route sends with no change to the setting, and the setting,
// the days and the units it means, survives a save.
func TestRouteDaysTargetFollowsDemand(t *testing.T) {
	t.Setenv("KINGPIN_HOME", t.TempDir())
	cfg := content.MustLoad()
	home, hub, route := twoCities(t, cfg)
	product := cfg.Market.Products[0].ID
	lg := newLogistics(cfg)
	safe := *cfg
	safe.Routes.Routes = append([]content.RouteConfig(nil), cfg.Routes.Routes...)
	for i := range safe.Routes.Routes {
		safe.Routes.Routes[i].Risk = 0
	}
	const days = 3
	// held is the quiet player holding that many corners at home with
	// runners who sell nothing (no order is placed), the route on at
	// normal with a days target, and the hub stashed so nothing is
	// bought by the lot: only the target moves.
	held := func(corners int) (*game.World, int) {
		w := quiet(&safe, 5)
		w.SetStock(hub, product, 100_000)
		w.Player.DirtyCash = cfg.Heat.Heat.DirtyCashThreshold
		for i, c := range w.Home().Corners[:corners] {
			id := 900 + i
			w.Crew.Members = append(w.Crew.Members, game.CrewMember{ID: id, Name: fmt.Sprintf("R%d", i), Role: "runner", Skill: 60, Units: 100, Loyalty: 90, Nerve: 60, Wage: 50})
			if err := w.Post(c.ID, id); err != nil {
				t.Fatal(err)
			}
		}
		_ = w.SetRouteDays(route.ID, product, days)
		_ = w.SetRoute(route.ID, events.RouteNormal)
		return w, lg.Target(w, route, product)
	}
	// sent runs the world a week and is what the route sent: the
	// setting never moves, and every morning after the first the far
	// end plus the road is at the target the road read that night (the
	// street's demand drifts with its price, so the units a days target
	// means drift too, and the boat carries a day's shortfall whole).
	sent := func(w *game.World) int {
		res, err := RunFrom(&safe, w, 8, func(w *game.World) {
			w.Player.CleanCash = 0
			if rs := w.Route(route.ID); rs.Days[product] != days || rs.Target != nil {
				t.Fatalf("day %d: the setting moved: %+v", w.Day, rs)
			}
			if w.Day > 0 && lg.Shortfall(w, route, product) != 0 {
				t.Fatalf("day %d: %d short of %d (%d there, %d on the road)", w.Day, lg.Shortfall(w, route, product), lg.Target(w, route, product), w.Stock(home, product), w.Bound(home, product))
			}
		})
		if err != nil {
			t.Fatal(err)
		}
		n := 0
		for _, e := range res.Events {
			if ev, ok := e.(events.ShipmentSent); ok {
				n += ev.Units
			}
		}
		return n
	}
	one, oneTarget := held(1)
	if oneTarget <= 0 {
		t.Fatalf("a corner worked and a target of %d", oneTarget)
	}
	three, threeTarget := held(3)
	if threeTarget <= oneTarget {
		t.Fatalf("three corners mean %d, one %d", threeTarget, oneTarget)
	}
	a, b := sent(one), sent(three)
	if b <= a || a < oneTarget || b < threeTarget {
		t.Fatalf("the route sent %d holding one corner and %d holding three (day-0 targets %d and %d)", a, b, oneTarget, threeTarget)
	}
	if three.Stock(home, product) <= one.Stock(home, product) {
		t.Fatalf("home holds %d with three corners and %d with one", three.Stock(home, product), one.Stock(home, product))
	}
	threeTarget = lg.Target(three, route, product)
	// The days survive a save, and the loaded world reads them the same.
	if err := game.Save(1, three); err != nil {
		t.Fatal(err)
	}
	set, _, err := sim.Default(cfg)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := game.Load(1, set.Migrations()...)
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(loaded.Routes) != fmt.Sprint(three.Routes) || loaded.Route(route.ID).Days[product] != days || lg.Target(loaded, route, product) != threeTarget {
		t.Fatalf("loaded %v (target %d), saved %v (target %d)", loaded.Routes, lg.Target(loaded, route, product), three.Routes, threeTarget)
	}
}

// The dial runs the route (#61): with it on and a target set, the far
// city's stash is brought to the target within the route's capacity a
// day and kept there, buying by the lot at the source; nothing moves
// while dirty cash is under the float; with the dial off nothing is
// bought or sent.
func TestRouteDialKeepsTheTarget(t *testing.T) {
	cfg := content.MustLoad()
	home, hub, route := twoCities(t, cfg)
	product := cfg.Market.Products[0].ID
	lg := newLogistics(cfg)
	safe := *cfg
	safe.Routes.Routes = append([]content.RouteConfig(nil), cfg.Routes.Routes...)
	for i := range safe.Routes.Routes {
		safe.Routes.Routes[i].Risk = 0
	}
	w := quiet(&safe, 5)
	offer := w.WholesaleSupplier(route.From)
	if offer == nil {
		t.Fatalf("no wholesaler in %s", route.From)
	}
	target := 2*route.Capacity + offer.Lot/2
	days := lg.Days(w, route, events.ShipNormal)
	w.Player.DirtyCash = cfg.Heat.Heat.DirtyCashThreshold // enough for the lots, never a pile that draws a sting on the stash
	w.Stats.PeakCash = offer.UnlockCash
	_ = w.SetRouteTarget(route.ID, product, target)
	_ = w.SetRoute(route.ID, events.RouteNormal)
	phase := "fill"
	res, err := RunFrom(&safe, w, 40, func(w *game.World) {
		w.Player.CleanCash = 0
		switch {
		case w.Day == 20:
			// Drained and broke: the road must not touch the float.
			phase = "broke"
			w.SetStock(home, product, 0)
			w.Player.DirtyCash = lg.Float() - 1
		case w.Day == 30:
			// Rich again, but off.
			phase = "off"
			w.Player.DirtyCash = cfg.Heat.Heat.DirtyCashThreshold
			_ = w.SetRoute(route.ID, events.RouteOff)
		}
		switch phase {
		case "fill":
			if w.Day > 0 && w.Day <= (target+route.Capacity-1)/route.Capacity {
				if n := w.Bound(home, product) + w.Stock(home, product); n != min(w.Day*route.Capacity, target) {
					t.Fatalf("day %d: %d sent toward the target %d, want %d", w.Day, n, target, min(w.Day*route.Capacity, target))
				}
			}
			if w.Day > days+(target+route.Capacity-1)/route.Capacity && w.Stock(home, product) != target {
				t.Fatalf("day %d: home holds %d, target %d", w.Day, w.Stock(home, product), target)
			}
			if w.Stock(hub, product) >= offer.Lot {
				t.Fatalf("day %d: %d on the dock in %s", w.Day, w.Stock(hub, product), hub)
			}
		case "broke", "off":
			if w.Day > 21 && (len(w.Shipments) != 0 || w.Stock(home, product) != 0 || w.Player.DirtyCash < lg.Float()-1) {
				t.Fatalf("day %d (%s): %+v on the road, home holds %d, dirty cash %d", w.Day, phase, w.Shipments, w.Stock(home, product), w.Player.DirtyCash)
			}
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	lots, sent := 0, 0
	for _, e := range res.Events {
		switch ev := e.(type) {
		case events.WholesaleBought:
			if ev.Day > 20 {
				t.Fatalf("bought lots %s: %+v", phase, ev)
			}
			lots += ev.Lots
		case events.ShipmentSent:
			if ev.Day > 20 {
				t.Fatalf("sent while %s: %+v", phase, ev)
			}
			sent += ev.Units
		}
	}
	if lots*offer.Lot < target || sent != target {
		t.Fatalf("%d lots bought and %d units sent for a target of %d", lots, sent, target)
	}
	if res.World.Stats.Seizures != 0 || res.World.Over != nil {
		t.Fatalf("%d seizures, over %v", res.World.Stats.Seizures, res.World.Over)
	}
}

// The route is the tier-4 multiplier: the distributor out-earns the
// launderer on median net worth at the horizon, and is never indicted.
func TestDistributorBeatsLaundered(t *testing.T) {
	cfg := content.MustLoad()
	var dist, laun []int
	for seed := uint64(1); seed <= 10; seed++ {
		d, err := Run(cfg, seed, Horizon, Distributor(cfg, 40))
		if err != nil {
			t.Fatal(err)
		}
		if d.Over != nil {
			t.Fatalf("seed %d: distributor ended on day %d: %s", seed, d.Days, d.Over.Cause)
		}
		l, _ := Run(cfg, seed, Horizon, Laundered(cfg, 40))
		dist = append(dist, d.NetWorthAt(Horizon))
		laun = append(laun, l.NetWorthAt(Horizon))
	}
	sort.Ints(dist)
	sort.Ints(laun)
	t.Logf("day %d median net worth: distributor %d, laundered %d", Horizon, dist[len(dist)/2], laun[len(laun)/2])
	if dist[len(dist)/2] <= laun[len(laun)/2] {
		t.Fatalf("distributor median %d, laundered %d; the route should pay", dist[len(dist)/2], laun[len(laun)/2])
	}
}
