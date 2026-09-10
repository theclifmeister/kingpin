// Package harness runs the game headless under a scripted policy so balance
// can be measured and invariants tested without a terminal.
package harness

import (
	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/sim"
)

// Policy decides the player's actions for the coming day.
type Policy func(w *game.World)

// Result summarises a headless run.
type Result struct {
	Days     int
	Over     *game.Ending
	PeakCash int
	EndCash  int
	Events   []events.Event
	World    *game.World
}

// Run plays up to days days from a fresh world with the given seed.
func Run(cfg *content.Config, seed uint64, days int, policy Policy) (Result, error) {
	_, sims, err := sim.Default(cfg)
	if err != nil {
		return Result{}, err
	}
	w := sim.NewWorld(cfg, seed)
	clock := game.NewClock(nil, sims...)
	var all []events.Event
	for d := 0; d < days && w.Over == nil; d++ {
		if policy != nil {
			policy(w)
		}
		all = append(all, clock.EndDay(w)...)
	}
	return Result{Days: w.Day, Over: w.Over, PeakCash: w.Stats.PeakCash, EndCash: w.Cash(), Events: all, World: w}, nil
}

// Idle does nothing; prices drift on their own.
func Idle(*game.World) {}

// Trader restocks every product it can afford and sells everything it holds
// at the given dial, every day. It is deliberately greedy.
func Trader(cfg *content.Config, dial events.Dial) Policy {
	pressure := cfg.Market.Market.BuyPricePressure
	return func(w *game.World) {
		// Sell what we hold.
		for _, id := range w.Products {
			if q := w.Player.Stock[id]; q > 0 {
				_ = w.PlaceSell(id, q, dial)
			}
		}
		// Restock toward a demand-proportional mix that fits what the
		// operation can hold, so a crashed product never hogs the whole bag.
		total := 0.0
		for _, id := range w.Products {
			total += w.Market[id].Demand
		}
		for _, id := range w.Products {
			m := w.Market[id]
			target := int(float64(w.Capacity()) * m.Demand / total)
			room := w.Capacity() - w.Player.TotalStock()
			afford := int(float64(w.Player.DirtyCash) / m.SupplierPrice)
			qty := min(target-w.Player.Stock[id], afford, room)
			if qty > 0 {
				_, _ = w.Buy(id, qty, pressure)
			}
		}
	}
}

// Careful trades quietly and lies low whenever heat climbs.
func Careful(cfg *content.Config, lieLowAt float64) Policy {
	trade := Trader(cfg, events.DialQuiet)
	return func(w *game.World) {
		if w.Heat.Value >= lieLowAt {
			w.SetLieLow(true)
			return
		}
		trade(w)
	}
}

// Managed sells at the normal dial and lies low whenever heat reaches
// lieLowAt. It is the baseline for "a player who pays attention".
func Managed(cfg *content.Config, lieLowAt float64) Policy {
	trade := Trader(cfg, events.DialNormal)
	return func(w *game.World) {
		if w.Heat.Value >= lieLowAt {
			w.SetLieLow(true)
			return
		}
		trade(w)
	}
}

// Crewed plays like Managed but builds a crew: it pays fair, signs the most
// skilled runner on offer whenever it can afford the fee with cash to
// spare, and replaces anyone whose loyalty has sunk to where they skim.
func Crewed(cfg *content.Config, lieLowAt float64) Policy {
	managed := Managed(cfg, lieLowAt)
	tun := cfg.Crew.Crew
	return func(w *game.World) {
		w.SetPay(events.PayFair)
		for _, m := range w.Crew.Members {
			if m.Loyalty < tun.SkimThreshold {
				_, _ = w.Fire(m.ID)
				break // one a day; each firing sours the rest
			}
		}
		best := -1
		for i, c := range w.Crew.Candidates {
			if c.Role != "runner" {
				continue
			}
			if best < 0 || c.Skill > w.Crew.Candidates[best].Skill {
				best = i
			}
		}
		if best >= 0 && len(w.Crew.Members) < tun.MaxCrew {
			c := w.Crew.Candidates[best]
			if w.Player.DirtyCash >= c.Fee+cfg.Market.Market.StartCash {
				_, _ = w.Hire(c.ID, tun.MaxCrew)
			}
		}
		managed(w)
	}
}
