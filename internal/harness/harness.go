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
		// Restock toward a demand-proportional mix that fits the carry limit,
		// so a crashed product never hogs the whole bag.
		total := 0.0
		for _, id := range w.Products {
			total += w.Market[id].Demand
		}
		for _, id := range w.Products {
			m := w.Market[id]
			target := int(float64(w.Player.CarryLimit) * m.Demand / total)
			room := w.Player.CarryLimit - w.Player.TotalStock()
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
