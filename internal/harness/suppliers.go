package harness

import (
	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
)

// Leveraged plays like Crewed on credit (#72): whatever it restocks it
// takes from the cheapest connect where it stands on their book first,
// up to what they will run it, and pays cash for the rest; the debt is
// collected on its day by the market sim, out of whatever is in the
// till, so a bad week is a missed payment. It is the baseline for
// "credit is a real lever and a real risk": ahead of Crewed while the
// bag is the bind, and no better once it is not.
func Leveraged(cfg *content.Config, lieLowAt float64) Policy {
	hot := TooHot(cfg, lieLowAt)
	pressure := func(w *game.World) float64 {
		return cfg.Market.Market.BuyPricePressure * game.FoldEffects(w, cfg.Upgrades).BuyPressureMul
	}
	return func(w *game.World) {
		staff(cfg, w, w.Player.Location, 0)
		if hot(w) {
			w.SetLieLow(true)
			return
		}
		standSomewhere(w)
		city := w.Here()
		reserve := 0
		if d := w.Deal(game.DealTribute); d != nil {
			reserve = d.Terms.PerDay
		}
		total := 0.0
		for _, id := range w.Products {
			if !city.Market[id].NoSupply {
				total += city.Market[id].Demand
			}
		}
		for _, id := range w.Products {
			m := city.Market[id]
			if m.NoSupply {
				continue
			}
			target := int(float64(w.Capacity(city.ID)) * m.Demand / total)
			buyHere(w, id, target-w.Stock(city.ID, id), reserve, pressure(w), true)
		}
		sellEverything(w, events.DialNormal)
	}
}

// NoCredit is the config with every connect's credit withdrawn
// (cmd/balance -credit off): the same connects at the same prices, no
// book to run.
func NoCredit(cfg *content.Config) *content.Config {
	out := *cfg
	out.Suppliers.Deck = append([]content.SupplierConfig(nil), cfg.Suppliers.Deck...)
	for i := range out.Suppliers.Deck {
		out.Suppliers.Deck[i].CreditDays = 0
		out.Suppliers.Deck[i].CreditLimit = 0
	}
	return &out
}
