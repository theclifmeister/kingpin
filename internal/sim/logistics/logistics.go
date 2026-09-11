// Package logistics simulates the route graph (#30, #61): every route is
// an edge between two cities with a dial the player sets once, and every
// day the sim runs each route that is on, buying by the lot at the source
// where a wholesaler deals and sending what the far city is short of its
// target. A shipment rides the route for its days, every one of which the
// police may intercept it on, and lands in the destination's stash if they
// do not. A seizure loses the whole shipment; it is the biggest exposure in
// the game. The sim touches no other sim's state: it emits ShipmentSeized
// for heat (stepping after it) to react to, and keeps a record of it in
// the world for the market (stepping before it) to read the next morning.
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
	tree   content.UpgradesConfig
	float  int // dirty cash the road never spends below: the laundering float
}

// New builds a logistics sim from the routes, the cities they join, the
// market (for what a fresh city's ladder starts at when a save is
// migrated, and the pressure a lot puts on the supplier), the upgrade
// tree (which scales that pressure) and the float the road leaves in the
// till, which is the laundering float: the road never starves the street
// any more than the wash does.
func New(cfg content.RoutesConfig, cities content.CityConfig, market content.MarketConfig, tree content.UpgradesConfig, float int) *Sim {
	return &Sim{cfg: cfg, cities: cities, market: market, tree: tree, float: float}
}

func (s *Sim) Name() string { return "logistics" }

// Tuning exposes the shipping constants the UI needs to explain itself.
func (s *Sim) Tuning() content.ShippingTuning { return s.cfg.Shipping }

// Float is the dirty cash the road never spends below.
func (s *Sim) Float() int { return s.float }

// Wholesale is the wholesale supplier's offer: what the routes buy by.
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
// before it lands: what the map shows against the dial, and what the dice
// add up to over the days.
func (s *Sim) Risk(r content.RouteConfig, d events.Ship) float64 {
	return 1 - math.Pow(1-s.DayRisk(r, d), float64(s.Days(r, d)))
}

// Shortfall is how many units of a product a route owes its destination
// today: the target less what is stashed there and what is already on
// the road to it. Zero for a route with no target for the product.
func (s *Sim) Shortfall(w *game.World, r content.RouteConfig, product string) int {
	target := w.Route(r.ID).Target[product]
	if target <= 0 {
		return 0
	}
	return max(0, target-w.Stock(r.To, product)-w.Bound(r.To, product))
}

// Budget is the dirty cash the road may spend today: what is over the
// float, folded by the tree the way the wash folds it (World.Float, #118).
func (s *Sim) Budget(w *game.World) int { return max(0, w.Player.DirtyCash-w.Float(s.tree, s.float)) }

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
	return game.StartingProduct{ID: p.ID, Name: p.Name, Price: p.BasePrice * cp.Price, Demand: p.Demand * cp.Demand, NoSupply: cp.NoSupply}
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
// Seizures are recorded for the market to read tomorrow. Then it runs the
// routes: every one with its dial on sends what its far city is short,
// buying by the lot at the source to cover it. The road rolls off its own
// side stream, so a shipment never shifts the dice at home, and running a
// route needs no dice at all: a run with every route off replays as it
// did before the dial.
func (s *Sim) Step(w *game.World, t *game.Tick) {
	s.move(w, t)
	s.run(w, t)
}

// move is the road: today's rolls, and the arrivals landed.
func (s *Sim) move(w *game.World, t *game.Tick) {
	tun := s.cfg.Shipping
	rng := t.Sub("logistics")
	sort.SliceStable(w.Shipments, func(i, j int) bool { return w.Shipments[i].ID < w.Shipments[j].ID })
	kept := w.Shipments[:0]
	for _, sh := range w.Shipments {
		risk := 0.0
		if r := s.cfg.Route(sh.Route); r != nil {
			risk = s.DayRisk(*r, sh.Dial)
		}
		if rng.Float64() < risk {
			w.Stats.Seizures++
			w.Stats.SeizedOnRoad += sh.Units
			if w.Logistics.Lost == nil {
				w.Logistics.Lost = map[string]int{}
			}
			w.Logistics.Lost[sh.Route] += sh.Units
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
		days := w.Logistics.Days[:0]
		for _, d := range w.Logistics.Days {
			if t.Day-d.Day <= tun.RecordDays {
				days = append(days, d)
			}
		}
		w.Logistics.Days = days
	}
}

// run is the routes (#61). For every route in file order with its dial
// on, and every product in ladder order with a target, the shortfall
// against the target is what goes today, up to the route's capacity: the
// source stash first, then whole lots bought from the wholesaler there if
// it deals and the door is open, then the fare. Nothing is bought or sent
// out of the float: the road spends only what is over it, lots before
// fares, and a lot it cannot then afford to send waits in the stash. The
// shipment leaves this morning (the tick's day) with the lots bought for
// it, rolls from tomorrow and is in the morning report today. No dice.
func (s *Sim) run(w *game.World, t *game.Tick) {
	if w.Over != nil {
		return
	}
	offer := s.Wholesale()
	pressure := s.market.Market.BuyPricePressure * game.FoldEffects(w, s.tree).BuyPressureMul
	for _, r := range s.cfg.Routes {
		rs := w.Route(r.ID)
		if !rs.Dial.On() || w.Cities[r.From] == nil || w.Cities[r.To] == nil {
			continue
		}
		dial := rs.Dial.Ship()
		var day game.RouteDay
		for _, id := range w.Products {
			if w.Product(r.From, id) == nil || w.Product(r.To, id) == nil {
				continue
			}
			units := min(s.Shortfall(w, r, id), r.Capacity)
			if units <= 0 {
				continue
			}
			have := w.Stock(r.From, id)
			if need := units - have; need > 0 && w.Cities[r.From].Wholesale && !offer.Locked(w) && offer.Lot > 0 {
				lots := (need + offer.Lot - 1) / offer.Lot
				unit := w.Product(r.From, id).SupplierPrice * offer.Mul
				// Lots the budget covers with the fare for what they
				// make up: never a lot the road then cannot move.
				for lots > 0 {
					cost := int(math.Ceil(unit * float64(lots*offer.Lot)))
					if cost+min(units, have+lots*offer.Lot)*r.Cost <= s.Budget(w) {
						break
					}
					lots--
				}
				if lots > 0 {
					if p, err := w.Restock(r.From, id, lots, offer, pressure); err == nil {
						day.Wholesale += p.Cost
						t.Emit(events.WholesaleBought{Day: t.Day, City: r.From, Route: r.ID, Name: r.Name, Product: id, Lots: lots, Units: p.Qty, Cost: p.Cost})
					}
				}
				have = w.Stock(r.From, id)
			}
			units = min(units, have)
			if r.Cost > 0 {
				units = min(units, s.Budget(w)/r.Cost)
			}
			if units <= 0 {
				continue
			}
			days := s.Days(r, dial)
			sh := w.Send(game.Shipment{
				Route: r.ID, Mode: r.Mode, From: r.From, To: r.To, Product: id, Units: units,
				Dial: dial, Sent: t.Day, Arrives: t.Day + days, Cost: units * r.Cost,
			})
			day.Fares += sh.Cost
			t.Emit(events.ShipmentSent{
				Day: t.Day, ID: sh.ID, Route: sh.Route, Name: r.Name, Mode: sh.Mode, From: sh.From, To: sh.To,
				Product: sh.Product, Units: sh.Units, Cost: sh.Cost, Dial: sh.Dial, Days: days,
			})
		}
		if day.Wholesale+day.Fares > 0 {
			day.Day, day.Route = t.Day, r.ID
			w.Logistics.Days = append(w.Logistics.Days, day)
		}
	}
}
