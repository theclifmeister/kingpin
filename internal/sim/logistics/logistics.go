// Package logistics simulates the route graph: product sent from one city
// to another rides a shipment for the route's days, every one of which the
// police may intercept it on, and lands in the destination's stash if they
// do not. A seizure loses the whole shipment; it is the biggest exposure in
// the game. The sim touches no other sim's state: it emits ShipmentSeized
// for heat (stepping after it) to react to, and keeps a record of it in
// the world for the market (stepping before it) to read the next morning.
// It also prices the wholesale supplier and the routes for the world's
// actions, which never see the config.
package logistics

import (
	"math"
	"sort"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
)

// Sim is the logistics simulation.
type Sim struct {
	cfg    content.RoutesConfig
	cities content.CityConfig
	market content.MarketConfig
}

// New builds a logistics sim from the routes, the cities they join and the
// market (for what a fresh city's ladder starts at when a save is
// migrated).
func New(cfg content.RoutesConfig, cities content.CityConfig, market content.MarketConfig) *Sim {
	return &Sim{cfg: cfg, cities: cities, market: market}
}

func (s *Sim) Name() string { return "logistics" }

// Tuning exposes the shipping constants the UI needs to explain itself.
func (s *Sim) Tuning() content.ShippingTuning { return s.cfg.Shipping }

// Wholesale is the wholesale supplier's offer, for BuyWholesale.
func (s *Sim) Wholesale() game.WholesaleOffer {
	t := s.cfg.Wholesale
	return game.WholesaleOffer{Lot: t.Lot, Mul: t.Mul, UnlockCash: t.UnlockCash}
}

// Dial returns the tuning for a ship dial position.
func (s *Sim) Dial(d events.Ship) content.ShipDialConfig { return s.cfg.DialFor(d) }

// Routes lists the routes touching a city, in file order.
func (s *Sim) Routes(city string) []content.RouteConfig {
	var out []content.RouteConfig
	for _, r := range s.cfg.Routes {
		if r.Other(city) != "" {
			out = append(out, r)
		}
	}
	return out
}

// Route returns the route with id, or nil.
func (s *Sim) Route(id string) *content.RouteConfig { return s.cfg.Route(id) }

// Days is how long a route takes at a dial: at least a day.
func (s *Sim) Days(r content.RouteConfig, d events.Ship) int {
	return max(1, int(math.Round(float64(r.Days)*s.Dial(d).Days)))
}

// DayRisk is the chance a shipment on a route at a dial is intercepted on
// any one day in transit.
func (s *Sim) DayRisk(r content.RouteConfig, d events.Ship) float64 {
	return math.Max(0, math.Min(1, r.Risk*s.Dial(d).Risk))
}

// Risk is the chance a shipment on a route at a dial is seized at all
// before it lands: what the ship dialog shows, and what the dice add up
// to over the days.
func (s *Sim) Risk(r content.RouteConfig, d events.Ship) float64 {
	return 1 - math.Pow(1-s.DayRisk(r, d), float64(s.Days(r, d)))
}

// Offer prices a route at a dial for World.Ship.
func (s *Sim) Offer(r content.RouteConfig, d events.Ship) game.RouteOffer {
	return game.RouteOffer{
		ID: r.ID, Name: r.Name, Mode: r.Mode, From: r.From, To: r.To,
		Days: s.Days(r, d), Capacity: r.Capacity, Cost: r.Cost, Dial: d,
	}
}

// Ship sends units of a product over the route with id from one city to
// the other at a dial: World.Ship with the route priced, or ErrNoRoute
// for an id the config does not know.
func (s *Sim) Ship(w *game.World, route, from, to, product string, units int, d events.Ship) (game.Shipment, error) {
	r := s.cfg.Route(route)
	if r == nil {
		return game.Shipment{}, game.ErrNoRoute
	}
	return w.Ship(s.Offer(*r, d), from, to, product, units)
}

// StartingCities converts config into the cities a new world starts with:
// every city, home first, its ladder priced for it.
func StartingCities(cities content.CityConfig, market content.MarketConfig) []game.StartingCity {
	out := make([]game.StartingCity, 0, len(cities.Cities))
	for _, c := range cities.Cities {
		out = append(out, StartingCity(c, market))
	}
	return out
}

// StartingCity prices one city's starting products: the ladder's rungs
// the starting cash unlocks, at the city's multipliers.
func StartingCity(c content.CityEntry, market content.MarketConfig) game.StartingCity {
	sc := game.StartingCity{ID: c.ID, Name: c.Name, HeatMul: c.HeatMul(), Wholesale: c.Wholesale}
	for _, p := range market.Products {
		if p.UnlockCash <= market.Market.StartCash {
			sc.Products = append(sc.Products, Product(c, p))
		}
	}
	return sc
}

// Product is a product's starting values in a city.
func Product(c content.CityEntry, p content.ProductConfig) game.StartingProduct {
	cp := c.Product(p.ID)
	return game.StartingProduct{ID: p.ID, Name: p.Name, Price: p.BasePrice * cp.Price, Demand: p.Demand * cp.Demand}
}

// Migrate brings a save from before the second city up to date: the one
// city there was becomes home, keeping everything it had, and every other
// city in the config is laid out fresh, with every product the run has
// unlocked on its market. The territory sim's migration, stepping after
// this one in the chain's order of business, seeds any corners missing.
func (s *Sim) Migrate(w *game.World) {
	home := s.cities.Home()
	w.MigrateCities(game.StartingCity{ID: home.ID, Name: home.Name, HeatMul: home.HeatMul(), Wholesale: home.Wholesale})
	for _, c := range s.cities.Cities {
		if w.Cities[c.ID] != nil {
			continue
		}
		city := game.StartingCity{ID: c.ID, Name: c.Name, HeatMul: c.HeatMul(), Wholesale: c.Wholesale}
		for _, id := range w.Products {
			if p := s.market.Product(id); p != nil {
				city.Products = append(city.Products, Product(c, *p))
			}
		}
		w.AddCity(city)
	}
}

// Step reports what left yesterday, then moves every shipment on the road
// a day: the police roll for it, and if they miss and it is due, it lands.
// Seizures are recorded for the market to read tomorrow. The road rolls
// off its own side stream, so a shipment never shifts the dice at home.
func (s *Sim) Step(w *game.World, t *game.Tick) {
	tun := s.cfg.Shipping
	rng := t.Sub("logistics")
	sort.SliceStable(w.Shipments, func(i, j int) bool { return w.Shipments[i].ID < w.Shipments[j].ID })
	kept := w.Shipments[:0]
	for _, sh := range w.Shipments {
		if sh.Sent == t.Day-1 {
			t.Emit(events.ShipmentSent{
				Day: t.Day, ID: sh.ID, Route: sh.Route, Mode: sh.Mode, From: sh.From, To: sh.To,
				Product: sh.Product, Units: sh.Units, Cost: sh.Cost, Dial: sh.Dial, Days: sh.Arrives - sh.Sent,
			})
		}
		risk := 0.0
		if r := s.cfg.Route(sh.Route); r != nil {
			risk = s.DayRisk(*r, sh.Dial)
		}
		if rng.Float64() < risk {
			w.Stats.Seizures++
			w.Stats.SeizedOnRoad += sh.Units
			w.Logistics.Seizures = append(w.Logistics.Seizures, game.Seizure{
				Day: t.Day, Route: sh.Route, From: sh.From, To: sh.To, Product: sh.Product, Units: sh.Units,
			})
			t.Emit(events.ShipmentSeized{
				Day: t.Day, ID: sh.ID, Route: sh.Route, Mode: sh.Mode, From: sh.From, To: sh.To,
				Product: sh.Product, Units: sh.Units, Dial: sh.Dial,
			})
			continue
		}
		if t.Day >= sh.Arrives {
			w.Stash(sh.To)[sh.Product] += sh.Units
			t.Emit(events.ShipmentArrived{
				Day: t.Day, ID: sh.ID, Route: sh.Route, Mode: sh.Mode, From: sh.From, To: sh.To,
				Product: sh.Product, Units: sh.Units,
			})
			continue
		}
		kept = append(kept, sh)
	}
	w.Shipments = kept
	if tun.RecordDays > 0 {
		recent := w.Logistics.Seizures[:0]
		for _, z := range w.Logistics.Seizures {
			if t.Day-z.Day <= tun.RecordDays {
				recent = append(recent, z)
			}
		}
		w.Logistics.Seizures = recent
	}
}
