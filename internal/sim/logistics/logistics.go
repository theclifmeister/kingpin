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
	"github.com/theclifmeister/kingpin/internal/format"
	"github.com/theclifmeister/kingpin/internal/game"
)

// Sim is the logistics simulation.
type Sim struct {
	cfg    content.RoutesConfig
	cities content.CityConfig
	market content.MarketConfig
	tree   content.UpgradesConfig
	law    content.LawFX        // #42: what a bought checkpoint or customs agent takes off an edge's risk
	driver float64              // #46: what a skill-100 driver takes off a shipment's risk per day (crew.toml [role.driver] driver_cut)
	float  int                  // dirty cash the road never spends below: the laundering float
	assets content.AssetsConfig // #48: the port (capacity and customs on the boats into its city) and the routes an asset opens
	intel  content.IntelTuning  // #45: what a seizure's fact fades at
}

// New builds a logistics sim from the config, copying what it reads
// (#144): the routes, the cities they join, the market (for what a
// fresh city's ladder starts at when a save is migrated, and the
// pressure a lot puts on the supplier), the upgrade tree (which scales
// that pressure) and the float the road leaves in the till, which is
// the laundering float: the road never starves the street any more than
// the wash does.
func New(cfg *content.Config) *Sim {
	return &Sim{cfg: cfg.Routes, cities: cfg.City, market: cfg.Market, tree: cfg.Upgrades, law: cfg.Law.Effects, driver: cfg.Crew.Role[game.RoleDriver].DriverCut, float: cfg.Laundering.Laundering.Float, assets: cfg.Assets, intel: cfg.Intel.Intel}
}

// Open reports whether a route is there to run (#48): every route in
// the file that names no asset, and one that does while the asset is
// owned and standing. A route that is not open is not on the map, the
// ledger or the road, so a run without the asset is the run before the
// route existed.
func (s *Sim) Open(w *game.World, r content.RouteConfig) bool {
	return r.Asset == "" || w.AssetLive(r.Asset)
}

// RoutesOpen lists the routes touching a city that are open (#48), in
// file order: what the map and the ledger show.
func (s *Sim) RoutesOpen(w *game.World, city string) []content.RouteConfig {
	var out []content.RouteConfig
	for _, r := range s.Routes(city) {
		if s.Open(w, r) {
			out = append(out, r)
		}
	}
	return out
}

// port is the port asset's row while it is owned and standing and the
// route is a boat touching its city (#48; the file's one boat sails
// out of Bayport, the port city, so the issue's "into" reads either
// way), else nil: the boats through your own port carry capacity_mul
// the capacity and clear customs for nothing.
func (s *Sim) port(w *game.World, r content.RouteConfig) *content.AssetConfig {
	row := s.assets.ByEffect(content.AssetPort)
	if row == nil || r.Mode != "boat" || r.Other(row.City) == "" || !w.AssetLive(row.ID) {
		return nil
	}
	return row
}

// Watched reports whether the feds are watching the skies on day (#48,
// HeatState.WatchUntil, stamped when a task force fires): the plane
// route's risk is the file's while they are and zero otherwise, the one
// edge only the task force touches.
func Watched(w *game.World, day int) bool { return day < w.Heat.WatchUntil }

// Watched is the package's Watched as the sim's, the way a front end
// reads it through engine.Rules (#297).
func (s *Sim) Watched(w *game.World, day int) bool { return Watched(w, day) }

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

// routeEffects is Effects with the fronts that stand at either end of
// r folded in (#344): what a route's risk reads, the car wash's
// route_risk_mul on the roads out of its city.
func (s *Sim) routeEffects(w *game.World, r content.RouteConfig) game.Effects {
	return game.FoldEffectsIn(w, s.tree, r.From, r.To)
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
// tree's route_risk_mul (the tyres, the compartments; and a car wash at
// either end, #344).
func (s *Sim) DayRisk(w *game.World, r content.RouteConfig, d events.Ship) float64 {
	return s.dayRisk(s.routeEffects(w, r), r, d, s.Cut(w, r, w.Day), Watched(w, w.Day+1))
}

// dayRisk is DayRisk with the fold, the cut and the watch given: a
// plane's risk is nothing unless the feds are watching (#48).
func (s *Sim) dayRisk(fx game.Effects, r content.RouteConfig, d events.Ship, cut float64, watched bool) float64 {
	if r.Mode == "plane" && !watched {
		return 0
	}
	return max(0, min(1, r.Risk*s.Dial(d).Risk*fx.RouteRiskMul*(1-cut)))
}

// Cut is what comes off an edge's risk per day: a bought checkpoint (a
// car or truck edge) or customs agent (a boat edge) while the deal is
// live on day (#42, law.toml's checkpoint_cut / customs_cut), and the
// route's driver (#46, DriverCut) while they are fit to ride, the two
// compounding; nothing otherwise. The odds the map shows are the ones
// the dice use.
func (s *Sim) Cut(w *game.World, r content.RouteConfig, day int) float64 {
	return s.cut(w, r, day, w.RouteDriver(r.ID, day))
}

// cut is Cut with the driver given: the route's for the odds the map
// shows and a new shipment's, the one riding it for a shipment on the
// road (a driver moved to another route rides this one home).
func (s *Sim) cut(w *game.World, r content.RouteConfig, day int, m *game.CrewMember) float64 {
	deal := 0.0
	if w.CheckpointLive(r.ID, day) {
		deal = max(0, min(1, s.DealCut(r)))
	}
	if s.port(w, r) != nil {
		deal = 1 // your own port clears customs (#48): the agent's envelope is redundant
	}
	drv := 0.0
	if m != nil {
		drv = s.DriverCut(m.Skill)
	}
	return 1 - (1-deal)*(1-drv)
}

// riding is the driver on a shipment on day, or nil: the one it left
// with, if they are still on the payroll and fit.
func riding(w *game.World, sh game.Shipment, day int) *game.CrewMember {
	m := w.Crew.Driver(sh.Driver)
	if m == nil || !m.Fit(day) {
		return nil
	}
	return m
}

// DriverCut is what a driver of a skill takes off a shipment's risk per
// day on the road (#46): driver_cut x skill/100.
func (s *Sim) DriverCut(skill int) float64 {
	return max(0, min(1, s.driver*float64(skill)/100))
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

// Customs is the package's Customs as the sim's, the way a front end
// reads it through engine.Rules (#297).
func (s *Sim) Customs(r content.RouteConfig) bool { return Customs(r) }

// Risk is the chance a shipment on a route at a dial is seized at all
// before it lands: what the map shows against the dial, and what the dice
// add up to over the days.
func (s *Sim) Risk(w *game.World, r content.RouteConfig, d events.Ship) float64 {
	return s.risk(s.routeEffects(w, r), r, d, s.Cut(w, r, w.Day), Watched(w, w.Day+1))
}

func (s *Sim) risk(fx game.Effects, r content.RouteConfig, d events.Ship, cut float64, watched bool) float64 {
	return 1 - math.Pow(1-s.dayRisk(fx, r, d, cut, watched), float64(s.days(fx, r, d)))
}

// RiskFrom is Risk with the route's risk a day given (#45): what the
// map, the pane and the checkpoint dialog show for the risk the file
// holds (game.Known(w).Risk), folded the way the dice fold the truth
// (the dial, the tree, the checkpoint's and the driver's cuts, the
// days). The panels never read Route.Risk.
func (s *Sim) RiskFrom(w *game.World, r content.RouteConfig, d events.Ship, base float64) float64 {
	r.Risk = base
	return s.Risk(w, r, d)
}

// Capacity is the most one shipment on a route carries: the route's
// capacity times the tree's route_capacity_mul (the trucks), at least a
// unit, and times the port's capacity_mul on a boat into your own port
// (#48).
func (s *Sim) Capacity(w *game.World, r content.RouteConfig) int {
	return s.capacity(w, s.Effects(w), r)
}

func (s *Sim) capacity(w *game.World, fx game.Effects, r content.RouteConfig) int {
	n := capacity(fx, r)
	if port := s.port(w, r); port != nil {
		n = max(1, int(math.Round(float64(n)*port.CapacityMul)))
	}
	return n
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
			sc.Products = append(sc.Products, Product(c, p, market.Market.SupplierRatio))
		}
	}
	return sc
}

// Product is a product's starting values in a city, its supplier price
// at the file's flat ratio.
func Product(c content.CityEntry, p content.ProductConfig, ratio float64) game.StartingProduct {
	cp := c.Product(p.ID)
	return game.StartingProduct{ID: p.ID, Name: p.Name, Price: p.BasePrice * cp.Price, Demand: p.Demand * cp.Demand, NoSupply: cp.NoSupply, SupplierRatio: ratio}
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
				city.Products = append(city.Products, Product(c, *p, s.market.Market.SupplierRatio))
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
	rng := t.Sub(game.StreamLogistics)
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
		r := s.cfg.Route(sh.Route)
		if r != nil {
			risk = s.dayRisk(s.routeEffects(w, *r), *r, sh.Dial, s.cut(w, *r, t.Day, riding(w, sh, t.Day)), Watched(w, t.Day))
		}
		// A road a faction fed you as quiet (#45) is a road it has the
		// customs watching: the first shipment on it is taken, whatever
		// the roll (made all the same, so the road's dice do not move),
		// and the lie names its author.
		seized, lured := rng.Float64() < risk, false
		if lure := w.Lure(sh.Route, game.FactRisk); lure != nil {
			seized, lured = true, true
			fed := w.Expose(sh.Route, game.FactRisk)
			w.Stats.Bitten++
			t.Emit(events.IntelFalse{Day: t.Day, Subject: sh.Route, FactKind: game.FactRisk, Name: s.name(sh.Route), Rival: w.Faction(fed).Leader, Faction: fed})
		}
		if seized {
			w.Stats.Seizures++
			w.Stats.SeizedOnRoad += sh.Units
			if w.Logistics.Lost == nil {
				w.Logistics.Lost = map[string]int{}
			}
			w.Logistics.Lost[sh.Route] += sh.Units
			w.Logistics.Seizures = append(w.Logistics.Seizures, game.Seizure{
				Day: t.Day, Route: sh.Route, From: sh.From, To: sh.To, Product: sh.Product, Units: sh.Units,
			})
			ev := events.ShipmentSeized{
				Day: t.Day, ID: sh.ID, Route: sh.Route, Mode: sh.Mode, From: sh.From, To: sh.To,
				Product: sh.Product, Units: sh.Units, Dial: sh.Dial, Driver: sh.Driver,
			}
			if m := w.Crew.Member(sh.Driver); m != nil {
				ev.DriverName = m.Name // the crew sim jails them with it (#46)
			}
			t.Emit(ev)
			// The tunnel is found once (#48): a seizure on it is the
			// police finding the way in, the route is shut for good and
			// the asset goes with it (the heat sim, stepping after,
			// takes it off the books on this event).
			if r != nil && r.Mode == "tunnel" && r.Asset != "" && w.HasAsset(r.Asset) {
				t.Emit(events.TunnelFound{Day: t.Day, Route: r.ID, Name: r.Name, Asset: r.Asset, Product: sh.Product, Units: sh.Units})
			}
			if !lured {
				s.learn(w, t, sh.Route) // a staged seizure says nothing about the road: the lie stands, named
			}
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
		if !rs.Dial.On() || rs.Closed(t.Day) || w.Cities[r.From] == nil || w.Cities[r.To] == nil || !s.Open(w, r) {
			continue // a shut route (#44) refuses new shipments until it reopens; one whose asset is gone (#48) is not there
		}
		dial := rs.Dial.Ship()
		var day game.RouteDay
		for _, id := range w.Products {
			if w.Product(r.From, id) == nil || w.Product(r.To, id) == nil {
				continue
			}
			units := min(s.Shortfall(w, r, id), s.capacity(w, fx, r))
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
			driver := 0
			if m := w.RouteDriver(r.ID, t.Day); m != nil {
				driver = m.ID // the route's driver rides it (#46)
			}
			sh := w.Send(game.Shipment{
				Route: r.ID, Mode: r.Mode, From: r.From, To: r.To, Product: id, Units: units,
				Dial: dial, Sent: t.Day, Arrives: t.Day + days, Cost: fareFor(fx, r, units), Driver: driver,
			})
			day.Fares += sh.Cost
			t.Emit(events.ShipmentSent{
				Day: t.Day, ID: sh.ID, Route: sh.Route, Name: r.Name, Mode: sh.Mode, From: sh.From, To: sh.To,
				Product: sh.Product, Units: sh.Units, Cost: sh.Cost, Dial: sh.Dial, Days: days, Driver: sh.Driver,
			})
		}
		if day.Wholesale+day.Fares > 0 {
			day.Day, day.Route = t.Day, r.ID
			w.Logistics.Days = append(w.Logistics.Days, day)
		}
	}
}

// learn files a route's risk a day (#45): a seizure is the one thing
// that shows you what a road is, at full confidence, fading at
// stale_rate. No dice.
func (s *Sim) learn(w *game.World, t *game.Tick, route string) {
	r := s.cfg.Route(route)
	if r == nil {
		return
	}
	f := game.Fact{
		Subject: route, Kind: game.FactRisk, Value: "~" + format.Pct(r.Risk, 0) + "/day", Number: r.Risk,
		Confidence: 1, Day: t.Day, Source: game.SourceSeen, Stale: s.intel.StaleRate, Forget: s.intel.Forget,
	}
	w.Learn(f)
	t.Emit(events.IntelGained{Day: t.Day, Subject: route, FactKind: game.FactRisk, Value: f.Value, Confidence: 1, Source: f.Source, Name: r.Name})
}

// name is a route's name, or its id.
func (s *Sim) name(route string) string {
	if r := s.cfg.Route(route); r != nil {
		return r.Name
	}
	return route
}
