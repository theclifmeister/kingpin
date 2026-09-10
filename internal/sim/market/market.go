// Package market simulates street prices, demand, shocks and the
// resolution of the player's sell orders. Demand is per standard corner;
// what the player can actually serve is that times the corners they work
// (game.World.Demand).
package market

import (
	"math"
	"sort"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
)

// Sim is the market simulation.
type Sim struct {
	cfg  content.MarketConfig
	tree content.UpgradesConfig
}

// New builds a market sim from config. The upgrade tree is what the
// Operations branch multiplies: carry, supplier price and fill.
func New(cfg content.MarketConfig, tree content.UpgradesConfig) *Sim {
	return &Sim{cfg: cfg, tree: tree}
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

// Step reports upgrades bought, resolves sell orders, then drifts prices
// and demand, then rolls for shocks. Sales resolve first so the dial
// interacts with today's price.
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

	for _, id := range ids {
		m := w.Market[id]
		pc := s.cfg.Product(id)
		if m == nil || pc == nil {
			continue
		}
		open := m.Price

		// 1. Resolve the player's order for this product.
		if o, ok := w.Orders[id]; ok && !w.LieLow {
			s.resolve(w, t, m, o)
		}

		// 2. Shock bookkeeping.
		if m.ShockDays > 0 {
			m.ShockDays--
			if m.ShockDays == 0 {
				m.ShockFactor = 1
				m.ShockSlump = false
			}
		} else {
			r := t.RNG.Float64()
			switch {
			case r < tun.ShockChance:
				m.ShockFactor = 1.5 + t.RNG.Float64()*1.5
				m.ShockDays = 3 + t.RNG.IntN(5)
				m.ShockSlump = false
				t.Emit(events.PriceShock{Day: t.Day, Product: id, Factor: m.ShockFactor, Days: m.ShockDays})
			case r < tun.ShockChance+tun.SlumpChance:
				m.ShockFactor = 0.5 + t.RNG.Float64()*0.2
				m.ShockDays = 3 + t.RNG.IntN(5)
				m.ShockSlump = true
				t.Emit(events.PriceShock{Day: t.Day, Product: id, Factor: m.ShockFactor, Days: m.ShockDays, Slump: true})
			}
		}

		// 3. Glut clears, price reverts toward target with noise.
		m.Glut *= 1 - tun.GlutDecay
		target := pc.BasePrice / (1 + m.Glut)
		if !m.ShockSlump {
			target *= m.ShockFactor
		}
		noise := t.RNG.NormFloat64() * pc.Volatility
		m.Price += (target - m.Price) * tun.Reversion
		m.Price *= 1 + noise
		m.Price = clamp(m.Price, pc.BasePrice*tun.PriceFloorRatio, pc.BasePrice*tun.PriceCeilingRatio)

		// 4. Demand per standard corner wanders around its base; slumps
		// cut it. The corners the player works scale it (World.Demand).
		base := pc.Demand
		if m.ShockSlump {
			base *= m.ShockFactor
		}
		m.Demand = base * (1 + t.RNG.NormFloat64()*pc.DemandNoise)
		m.Demand = math.Max(1, m.Demand)

		// 5. Supplier resets to a fraction of street price, less what
		// your contact there takes off.
		m.SupplierPrice = m.Price * tun.SupplierRatio * fx.SupplierMul

		m.History = append(m.History, m.Price)
		if n := tun.HistoryDays; n > 0 && len(m.History) > n {
			m.History = m.History[len(m.History)-n:]
		}
		t.Emit(events.PriceMove{Day: t.Day, Product: id, From: open, To: m.Price})
	}
}

// unlock lists every product the player's peak cash has earned. The
// supplier offers it from tomorrow; nothing about the offer is random, so
// old saves catch up the first day they are stepped.
func (s *Sim) unlock(w *game.World, t *game.Tick) {
	for _, p := range s.cfg.Products {
		if w.Market[p.ID] != nil || w.Stats.PeakCash < p.UnlockCash {
			continue
		}
		w.AddProduct(startingProduct(p))
		t.Emit(events.ProductUnlocked{Day: t.Day, Product: p.ID, Name: p.Name, Price: p.BasePrice})
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

// Capacity is how many units of a product the street will take at dial d
// today: the demand of the corners the player works, at the dial's fill.
// Without a worked corner there is nowhere to sell.
func (s *Sim) Capacity(w *game.World, product string, d events.Dial) int {
	return int(math.Round(w.Demand(product) * s.Fill(w, d)))
}

// resolve turns a sell order into cash, price impact and a PlayerSold event.
func (s *Sim) resolve(w *game.World, t *game.Tick, m *game.ProductMarket, o game.SellOrder) {
	d := s.Dial(o.Dial)
	demand := w.Demand(o.Product)
	sold := min(o.Qty, s.Capacity(w, o.Product, o.Dial), w.Player.Stock[o.Product])
	if sold < 0 {
		sold = 0
	}
	// Impact grows with the square of volume over demand: moving what the
	// street absorbs barely dents the price, flooding it craters it.
	impact := 0.0
	if demand > 0 {
		ratio := float64(sold) / demand
		impact = s.cfg.Market.SaleImpact * d.Impact * ratio * ratio
	}
	impact = math.Min(impact, 0.6)
	avg := m.Price * d.Price * (1 - impact/2)
	revenue := int(math.Round(avg * float64(sold)))

	w.Player.Stock[o.Product] -= sold
	w.Player.DirtyCash += revenue
	w.Stats.TotalRevenue += revenue
	w.Stats.UnitsSold += sold
	m.Price *= 1 - impact
	m.Glut += impact

	t.Emit(events.PlayerSold{
		Day: t.Day, Product: o.Product, Wanted: o.Qty, Sold: sold,
		Dial: o.Dial, AvgPrice: avg, Revenue: revenue,
	})
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

// StartingProducts converts config into the starting state NewWorld needs:
// the products the supplier offers to someone with the starting cash.
func StartingProducts(cfg content.MarketConfig) []game.StartingProduct {
	out := make([]game.StartingProduct, 0, len(cfg.Products))
	for _, p := range cfg.Products {
		if p.UnlockCash <= cfg.Market.StartCash {
			out = append(out, startingProduct(p))
		}
	}
	return out
}

func startingProduct(p content.ProductConfig) game.StartingProduct {
	return game.StartingProduct{ID: p.ID, Name: p.Name, Price: p.BasePrice, Demand: p.Demand}
}
