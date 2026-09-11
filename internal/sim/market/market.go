// Package market simulates street prices, demand, shocks and the
// resolution of the player's sell orders, city by city. Demand is per
// standard corner; what the player can actually serve in a city is that
// times the corners they work there (game.World.Demand). Every city has
// its own take on the ladder (city.toml), which is what makes a route
// worth the risk.
package market

import (
	"fmt"
	"math"
	"sort"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
)

// Sim is the market simulation.
type Sim struct {
	cfg    content.MarketConfig
	cities content.CityConfig
	ship   content.ShippingTuning
	tree   content.UpgradesConfig
	rep    content.ReputationFX
	bcfg   content.BuyersConfig
	buyers []buyer
}

// New builds a market sim from config. The cities say how each one
// prices the ladder; the shipping tuning is what a seizure on the road
// does to the street that was waiting for it; the upgrade tree is what
// the Operations branch multiplies: carry, supplier price and fill. Of
// the reputation effects it reads one: respect makes the supplier
// generous. The buyers (#71) are the deck of off-corner contracts it
// deals and resolves; it refuses one whose pitch does not parse.
func New(cfg content.MarketConfig, cities content.CityConfig, ship content.ShippingTuning, tree content.UpgradesConfig, rep content.ReputationFX, buyers content.BuyersConfig) (*Sim, error) {
	deck, err := parseBuyers(buyers)
	if err != nil {
		return nil, fmt.Errorf("buyers: %w", err)
	}
	return &Sim{cfg: cfg, cities: cities, ship: ship, tree: tree, rep: rep, bcfg: buyers, buyers: deck}, nil
}

// cityProduct is a city's multipliers on a product, 1 and 1 for a city
// the config does not know.
func (s *Sim) cityProduct(city, product string) content.CityProduct {
	if c := s.cities.City(city); c != nil {
		return c.Product(product)
	}
	return content.CityProduct{Price: 1, Demand: 1}
}

// BasePrice is what a product's price reverts toward in a city.
func (s *Sim) BasePrice(city string, pc content.ProductConfig) float64 {
	return pc.BasePrice * s.cityProduct(city, pc.ID).Price
}

// SupplierRatio is the supplier's price as a fraction of street today:
// the tuning, less what a supplier contact and the player's respect
// take off.
func (s *Sim) SupplierRatio(w *game.World) float64 {
	return s.cfg.Market.SupplierRatio * game.FoldEffects(w, s.tree).SupplierMul * content.Cut(w.Player.Reputation.Respect, s.rep.RespectSupplierCut)
}

func (s *Sim) Name() string { return "market" }

// Dial returns the tuning for a dial position.
func (s *Sim) Dial(d events.Dial) content.DialConfig {
	switch d {
	case events.DialQuiet:
		return s.cfg.Dial.Quiet
	case events.DialAggressive:
		return s.cfg.Dial.Aggressive
	default:
		return s.cfg.Dial.Normal
	}
}

// BuyPressure is how much a buy pushes the supplier price today, per unit
// over demand: the tuning, less what a supplier contact takes off.
func (s *Sim) BuyPressure(w *game.World) float64 {
	return s.cfg.Market.BuyPricePressure * game.FoldEffects(w, s.tree).BuyPressureMul
}

// Step reports upgrades bought, then in every city hands over what was
// queued against the buyers' contracts, resolves sell orders, drifts
// prices and demand, and rolls for shocks; then it settles the contracts
// that ran out and deals the next offer. Sales resolve first so the dial
// interacts with today's price; a handoff comes before them so a
// contract has first call on the stash. A shipment the police took on
// the road yesterday (the logistics sim steps after this one) is a supply
// shock this morning on the street that was waiting for it.
func (s *Sim) Step(w *game.World, t *game.Tick) {
	tun := s.cfg.Market
	fx := game.FoldEffects(w, s.tree)
	for _, id := range w.UpgradesToday {
		if u := s.tree.Upgrade(id); u != nil {
			t.Emit(events.UpgradeBought{Day: t.Day, ID: u.ID, Name: u.Name, Branch: u.Branch, Cost: u.Cost, Clean: u.Clean})
		}
	}
	s.unlock(w, t)
	ids := append([]string(nil), w.Products...)
	sort.Strings(ids) // deterministic regardless of map order

	for _, cid := range w.CityOrder {
		city := w.Cities[cid]
		// The home city's market rolls off the day's stream; every other
		// city's off its own side stream, so it never shifts what happens
		// at home.
		rng := t.RNG
		if city != w.Home() {
			rng = t.Sub("market:" + cid)
		}
		s.deliver(w, t, cid)
		for _, id := range ids {
			m := city.Market[id]
			pc := s.cfg.Product(id)
			if m == nil || pc == nil {
				continue
			}
			cp := s.cityProduct(cid, id)
			basePrice := pc.BasePrice * cp.Price
			open := m.Price
			// Whether the supplier here sells it is the config's to say,
			// stamped every day so a save from before the port product
			// (#60) plays by the file it is loaded under.
			m.NoSupply = cp.NoSupply

			// 1. Resolve the player's order for this product here, or
			// the standing order of the lieutenant who runs the city:
			// the player's wins the day. Lying low is everyone's day off.
			if !w.LieLow {
				if o, ok := w.Order(cid, id); ok {
					s.resolve(w, t, cid, m, o, false)
				} else if o, ok := w.StandingOrder(cid, id); ok {
					s.resolve(w, t, cid, m, o, true)
				}
			}

			// 2. Shock bookkeeping. Yesterday's seizure on the road
			// comes first: the street was counting on that product.
			seized := 0
			for _, z := range w.Logistics.Seizures {
				if z.Day == t.Day-1 && z.To == cid && z.Product == id {
					seized += z.Units
				}
			}
			switch {
			case seized > 0 && s.ship.ShockDays > 0 && s.ship.ShockFactor > 1:
				m.ShockFactor = s.ship.ShockFactor
				m.ShockDays = s.ship.ShockDays
				m.ShockSlump = false
				t.Emit(events.PriceShock{Day: t.Day, City: cid, Product: id, Factor: m.ShockFactor, Days: m.ShockDays, Seized: true})
			case m.ShockDays > 0:
				m.ShockDays--
				if m.ShockDays == 0 {
					m.ShockFactor = 1
					m.ShockSlump = false
				}
			default:
				r := rng.Float64()
				switch {
				case r < tun.ShockChance:
					m.ShockFactor = 1.5 + rng.Float64()*1.5
					m.ShockDays = 3 + rng.IntN(5)
					m.ShockSlump = false
					t.Emit(events.PriceShock{Day: t.Day, City: cid, Product: id, Factor: m.ShockFactor, Days: m.ShockDays})
				case r < tun.ShockChance+tun.SlumpChance:
					m.ShockFactor = 0.5 + rng.Float64()*0.2
					m.ShockDays = 3 + rng.IntN(5)
					m.ShockSlump = true
					t.Emit(events.PriceShock{Day: t.Day, City: cid, Product: id, Factor: m.ShockFactor, Days: m.ShockDays, Slump: true})
				}
			}

			// 3. Glut clears (faster with a price runner on the
			// street), price reverts toward target with noise.
			m.Glut *= 1 - math.Min(1, tun.GlutDecay*fx.GlutDecayMul)
			target := basePrice / (1 + m.Glut)
			if !m.ShockSlump {
				target *= m.ShockFactor
			}
			noise := rng.NormFloat64() * pc.Volatility
			m.Price += (target - m.Price) * tun.Reversion
			m.Price *= 1 + noise
			m.Price = clamp(m.Price, basePrice*tun.PriceFloorRatio, basePrice*tun.PriceCeilingRatio)

			// 4. Demand per standard corner wanders around the city's
			// base; slumps cut it. The corners the player works there
			// scale it (World.Demand).
			base := pc.Demand * cp.Demand
			if m.ShockSlump {
				base *= m.ShockFactor
			}
			m.Demand = base * (1 + rng.NormFloat64()*pc.DemandNoise)
			m.Demand = math.Max(1, m.Demand)

			// 5. Supplier resets to a fraction of street price, less what
			// your contact there, and your name, take off.
			m.SupplierPrice = m.Price * s.SupplierRatio(w)

			m.History = append(m.History, m.Price)
			if n := tun.HistoryDays; n > 0 && len(m.History) > n {
				m.History = m.History[len(m.History)-n:]
			}
			t.Emit(events.PriceMove{Day: t.Day, City: cid, Product: id, From: open, To: m.Price})
		}
	}
	s.settle(w, t)
	s.deal(w, t)
}

// unlock lists every product the player's peak cash has earned, in every
// city at once. The supplier offers it from tomorrow; nothing about the
// offer is random, so old saves catch up the first day they are stepped.
func (s *Sim) unlock(w *game.World, t *game.Tick) {
	for _, p := range s.cfg.Products {
		if w.Stats.PeakCash < p.UnlockCash {
			continue
		}
		fresh := false
		for _, cid := range w.CityOrder {
			if w.Product(cid, p.ID) != nil {
				continue
			}
			fresh = true
			cp := s.cityProduct(cid, p.ID)
			w.AddProduct(cid, game.StartingProduct{ID: p.ID, Name: p.Name, Price: p.BasePrice * cp.Price, Demand: p.Demand * cp.Demand, NoSupply: cp.NoSupply})
		}
		if fresh {
			t.Emit(events.ProductUnlocked{Day: t.Day, Product: p.ID, Name: p.Name, Price: p.BasePrice})
		}
	}
}

// Fill is the fraction of demand a sale at dial d can move today: the dial's
// fill, stretched by a street network, capped by patrols.
func (s *Sim) Fill(w *game.World, d events.Dial) float64 {
	fill := s.Dial(d).Fill * game.FoldEffects(w, s.tree).FillMul
	if w.Heat.SellCapDays > 0 && w.Heat.SellCap > 0 {
		fill = math.Min(fill, w.Heat.SellCap)
	}
	return fill
}

// Demand is what the player's worked corners in a city serve of a
// product today: World.Demand (the city's demand per standard corner
// times the share of corners worked there, which the UI shows as the
// street's demand), stretched by the Operations branch (demand_mul: the
// regulars and the name on the street). It is what the market serves an
// order against and what a sale's impact is measured against; the heat
// sim reads the same multiplier for what a sale attempts, so the dial
// preview stays honest.
func (s *Sim) Demand(w *game.World, city, product string) float64 {
	return w.Demand(city, product) * game.FoldEffects(w, s.tree).DemandMul
}

// Capacity is how many units of a product the street of a city will take
// at dial d today: the demand of the corners the player works there, at
// the dial's fill. Without a worked corner there is nowhere to sell.
func (s *Sim) Capacity(w *game.World, city, product string, d events.Dial) int {
	return int(math.Round(s.Demand(w, city, product) * s.Fill(w, d)))
}

// resolve turns a sell order into cash, price impact and a PlayerSold
// event, out of the city's stash. standing says the order was the
// lieutenant's; either way the event names whoever runs the city, for
// the crew sim's cut and the heat sim's temper.
func (s *Sim) resolve(w *game.World, t *game.Tick, city string, m *game.ProductMarket, o game.SellOrder, standing bool) {
	d := s.Dial(o.Dial)
	demand := s.Demand(w, city, o.Product)
	sold := min(o.Qty, s.Capacity(w, city, o.Product, o.Dial), w.Stock(city, o.Product))
	if sold < 0 {
		sold = 0
	}
	// Impact grows with the square of volume over demand: moving what the
	// street absorbs barely dents the price, flooding it craters it. A
	// set of scales (sale_impact_mul) dents it less.
	impact := 0.0
	if demand > 0 {
		ratio := float64(sold) / demand
		impact = s.cfg.Market.SaleImpact * game.FoldEffects(w, s.tree).SaleImpactMul * d.Impact * ratio * ratio
	}
	impact = math.Min(impact, 0.6)
	avg := m.Price * d.Price * (1 - impact/2)
	revenue := int(math.Round(avg * float64(sold)))

	w.Stash(city)[o.Product] -= sold
	w.Player.DirtyCash += revenue
	w.Stats.TotalRevenue += revenue
	w.Stats.UnitsSold += sold
	m.Price *= 1 - impact
	m.Glut += impact

	ev := events.PlayerSold{
		Day: t.Day, City: city, Product: o.Product, Wanted: o.Qty, Sold: sold,
		Dial: o.Dial, AvgPrice: avg, Revenue: revenue, Standing: standing,
	}
	if lt := w.Crew.Lieutenant(city); lt != nil {
		ev.Lieutenant, ev.LieutenantName = lt.ID, lt.Name
	}
	t.Emit(ev)
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
