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
	law    content.LawFX // #42: what a bought checkpoint or customs agent takes off an edge's risk
	float  int           // dirty cash the road never spends below: the laundering float
}

// New builds a logistics sim from the config, copying what it reads
// (#144): the routes, the cities they join, the market (for what a
// fresh city's ladder starts at when a save is migrated, and the
// pressure a lot puts on the supplier), the upgrade tree (which scales
// that pressure) and the float the road leaves in the till, which is
// the laundering float: the road never starves the street any more than
// the wash does.
func New(cfg *content.Config) *Sim {
	return &Sim{cfg: cfg.Routes, cities: cfg.City, market: cfg.Market, tree: cfg.Upgrades, law: cfg.Law.Effects, float: cfg.Laundering.Laundering.Float}
}

func (s *Sim) Name() string { return "logistics" }

// Tuning exposes the shipping constants the UI needs to explain itself.
func (s *Sim) Tuning() content.ShippingTuning { return s.cfg.Shipping }

// Float is the dirty cash the road never spends below.
func (s *Sim) Float() int { return s.float }

// Effects is what the owned upgrades do to the road (#119): the fold the
// sim reads its own tuning through at the top of its step and in every
// number the map and the pane show, so the odds the player reads are
// the ones the dice use.
func (s *Sim) Effects(w *game.World) game.Effects { return game.FoldEffects(w, s.tree) }

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

// Days is how long a route takes at a dial: the route's days times the
// dial's and the tree's route_days_mul (the drivers), at least a day.
func (s *Sim) Days(w *game.World, r content.RouteConfig, d events.Ship) int {
	return s.days(s.Effects(w), r, d)
}

func (s *Sim) days(fx game.Effects, r content.RouteConfig, d events.Ship) int {
	return max(1, int(math.Round(float64(r.Days)*s.Dial(d).Days*fx.RouteDaysMul)))
}

// DayRisk is the chance a shipment on a route at a dial is intercepted on
// any one day in transit: the route's risk times the dial's and the
// tree's route_risk_mul (the tyres, the compartments).
func (s *Sim) DayRisk(w *game.World, r content.RouteConfig, d events.Ship) float64 {
	return s.dayRisk(s.Effects(w), r, d, s.Cut(w, r, w.Day))
}

func (s *Sim) dayRisk(fx game.Effects, r content.RouteConfig, d events.Ship, cut float64) float64 {
	return math.Max(0, math.Min(1, r.Risk*s.Dial(d).Risk*fx.RouteRiskMul*(1-cut)))
}

// Cut is what a bought checkpoint (a car or truck edge) or customs
// agent (a boat edge) takes off an edge's risk per day (#42, law.toml's
// checkpoint_cut / customs_cut) while the deal is live on day; nothing
// otherwise. The odds the map shows are the ones the dice use.
func (s *Sim) Cut(w *game.World, r content.RouteConfig, day int) float64 {
	if !w.CheckpointLive(r.ID, day) {
		return 0
	}
	return math.Max(0, math.Min(1, s.DealCut(r)))
}

// DealCut is the cut a bought deal on the route would take, live or not.
func (s *Sim) DealCut(r content.RouteConfig) float64 {
	if r.Mode == "boat" || r.Mode == "plane" {
		return s.law.CustomsCut
	}
	return s.law.CheckpointCut
}

// Customs reports whether the route's deal is a customs agent (a boat
// or plane edge) rather than a checkpoint (car, truck).
func Customs(r content.RouteConfig) bool { return r.Mode == "boat" || r.Mode == "plane" }

// Risk is the chance a shipment on a route at a dial is seized at all
// before it lands: what the map shows against the dial, and what the dice
// add up to over the days.
func (s *Sim) Risk(w *game.World, r content.RouteConfig, d events.Ship) float64 {
	return s.risk(s.Effects(w), r, d, s.Cut(w, r, w.Day))
}

func (s *Sim) risk(fx game.Effects, r content.RouteConfig, d events.Ship, cut float64) float64 {
	return 1 - math.Pow(1-s.dayRisk(fx, r, d, cut), float64(s.days(fx, r, d)))
}

// Capacity is the most one shipment on a route carries: the route's
// capacity times the tree's route_capacity_mul (the trucks), at least a
// unit.
func (s *Sim) Capacity(w *game.World, r content.RouteConfig) int {
	return capacity(s.Effects(w), r)
}

func capacity(fx game.Effects, r content.RouteConfig) int {
	return max(1, int(math.Round(float64(r.Capacity)*fx.RouteCapacityMul)))
}

// Fare is what a route charges a unit: the route's cost times the tree's
// fare_mul (the forwarder). It is a price, so a discount on a dollar
// fare is fifty cents, not nothing or the dollar; a shipment's cost is
// the fare over its units, rounded up (fare).
func (s *Sim) Fare(w *game.World, r content.RouteConfig) float64 {
	return fare(s.Effects(w), r)
}

func fare(fx game.Effects, r content.RouteConfig) float64 {
	return float64(r.Cost) * fx.FareMul
}

// fareFor is what sending units on a route costs, whole dollars, rounded
// up.
func fareFor(fx game.Effects, r content.RouteConfig, units int) int {
	return int(math.Ceil(fare(fx, r) * float64(units)))
}

// Target is the units a route keeps its destination at for a product
// today: the units target as set, or a days target (#115) read as days
// times World.Demand at the far end this morning, the number the market
// screen shows as demand/day, rounded up; no dice. Zero for a route with
// no target for the product, and for a days target where nothing sells.
func (s *Sim) Target(w *game.World, r content.RouteConfig, product string) int {
	rs := w.Route(r.ID)
	if d := rs.Days[product]; d > 0 {
		return s.DaysTarget(w, r, product, d)
	}
	return max(0, rs.Target[product])
}

// DaysTarget is the units that many days of the far end's demand for a
// product are this morning, rounded up: what a days target of that many
// days would keep there today. The target dialog previews a number
// with it before it is set.
func (s *Sim) DaysTarget(w *game.World, r content.RouteConfig, product string, days int) int {
	if days <= 0 {
		return 0
	}
	// A hair under the product, so a share that multiplies to a whole
	// number is that number and not the one over it.
	return int(math.Ceil(float64(days)*w.Demand(r.To, product) - 1e-9))
}

// Shortfall is how many units of a product a route owes its destination
// today: the target less what is stashed there and what is already on
// the road to it. Zero for a route with no target for the product.
func (s *Sim) Shortfall(w *game.World, r content.RouteConfig, product string) int {
	target := s.Target(w, r, product)
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
	fx := s.Effects(w)
	s.deals(w, t)
	s.move(w, t, fx)
	s.run(w, t, fx)
}

// deals reports today's checkpoints and customs agents bought (#42) and,
// the day after the cold (w.Law.Cold: a law-and-order DA's calls_stop_days
// up), clears the dead deals off the routes: nothing can be bought while
// the cold stands, so every deal on the books then is one from before.
func (s *Sim) deals(w *game.World, t *game.Tick) {
	for _, o := range w.Today.Checkpoints {
		name, mode := o.Route, ""
		if r := s.cfg.Route(o.Route); r != nil {
			name, mode = r.Name, r.Mode
		}
		t.Emit(events.CheckpointBought{Day: t.Day, Route: o.Route, Name: name, Mode: mode, Cost: o.Cost, Until: w.Route(o.Route).Bought})
	}
	if w.Law.Cold > 0 && t.Day > w.Law.Cold {
		for id, rs := range w.Routes {
			if rs.Bought > 0 {
				rs.Bought = 0
				w.Routes[id] = rs
			}
		}
	}
}

// move is the road: today's rolls, and the arrivals landed.
func (s *Sim) move(w *game.World, t *game.Tick, fx game.Effects) {
	tun := s.cfg.Shipping
	rng := t.Sub("logistics")
	sort.SliceStable(w.Shipments, func(i, j int) bool { return w.Shipments[i].ID < w.Shipments[j].ID })
	kept := w.Shipments[:0]
	for _, sh := range w.Shipments {
		// A route shut by an incident (#44) moves nothing: the shipment
		// sits where it is, its days left intact (it lands a day later
		// for every night shut) and nothing on it is seized.
		if w.Route(sh.Route).Closed(t.Day) {
			sh.Arrives++
			kept = append(kept, sh)
			continue
		}
		risk := 0.0
		if r := s.cfg.Route(sh.Route); r != nil {
			risk = s.dayRisk(fx, *r, sh.Dial, s.Cut(w, *r, t.Day))
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
			w.AddStock(sh.To, sh.Product, sh.Units, sh.Quality) // at the quality it left with (#47)
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
// The tree (#119) scales the capacity, the days and the fare the road
// runs at; the wholesaler's price is the connect's (#72: the market sim
// stamps it, the ticket folded in), and the road buys from them only
// while they take calls and only as many lots as their day has left,
// the rest a shortfall it sends again tomorrow.
func (s *Sim) run(w *game.World, t *game.Tick, fx game.Effects) {
	if w.Over != nil {
		return
	}
	pressure := s.market.Market.BuyPricePressure * fx.BuyPressureMul
	for _, r := range s.cfg.Routes {
		rs := w.Route(r.ID)
		if !rs.Dial.On() || rs.Closed(t.Day) || w.Cities[r.From] == nil || w.Cities[r.To] == nil {
			continue // a shut route (#44) refuses new shipments until it reopens
		}
		dial := rs.Dial.Ship()
		var day game.RouteDay
		for _, id := range w.Products {
			if w.Product(r.From, id) == nil || w.Product(r.To, id) == nil {
				continue
			}
			units := min(s.Shortfall(w, r, id), capacity(fx, r))
			if units <= 0 {
				continue
			}
			have := w.Stock(r.From, id)
			sup := w.WholesaleSupplier(r.From)
			if need := units - have; need > 0 && sup != nil && sup.Open(w) && sup.Sells(id) && sup.Lot > 0 && sup.Price[id] > 0 {
				lots := min((need+sup.Lot-1)/sup.Lot, sup.Left()/sup.Lot)
				unit := sup.Price[id]
				// Lots the budget covers with the fare for what they
				// make up: never a lot the road then cannot move.
				for lots > 0 {
					cost := int(math.Ceil(unit * float64(lots*sup.Lot)))
					if cost+fareFor(fx, r, min(units, have+lots*sup.Lot)) <= s.Budget(w) {
						break
					}
					lots--
				}
				if lots > 0 {
					if p, err := w.Restock(r.From, id, lots, pressure); err == nil {
						day.Wholesale += p.Cost
						t.Emit(events.WholesaleBought{Day: t.Day, City: r.From, Route: r.ID, Name: r.Name, Product: id, Lots: lots, Units: p.Qty, Cost: p.Cost, Supplier: sup.ID})
					}
				}
				have = w.Stock(r.From, id)
			}
			units = min(units, have)
			if f := fare(fx, r); f > 0 {
				units = min(units, int(float64(s.Budget(w))/f))
				for units > 0 && fareFor(fx, r, units) > s.Budget(w) {
					units-- // a cent of rounding never takes the road under the float
				}
			}
			if units <= 0 {
				continue
			}
			days := s.days(fx, r, dial)
			sh := w.Send(game.Shipment{
				Route: r.ID, Mode: r.Mode, From: r.From, To: r.To, Product: id, Units: units,
				Dial: dial, Sent: t.Day, Arrives: t.Day + days, Cost: fareFor(fx, r, units),
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
