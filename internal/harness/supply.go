package harness

import (
	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/sim/crew"
	"github.com/theclifmeister/kingpin/internal/sim/market"
)

// Stocked plays like Crewed and never buys by hand (#113): every morning
// it sets a supply contract for every product the supplier where it
// stands sells, at StockDays days of the demand the corners it works
// there serve, shared out by demand where the stash cannot hold that
// much of everything (the way Crewed shares the bag), and sells the
// stash at the normal dial, lying low when hot as Crewed does. The
// contract buys at the markup, so it is the baseline for what the
// routine costs against the hand.
func Stocked(cfg *content.Config, lieLowAt float64) Policy {
	hot := TooHot(cfg, lieLowAt)
	mk, err := market.New(cfg)
	if err != nil {
		panic(err)
	}
	cs := crew.New(cfg)
	return func(w *game.World) {
		staff(cfg, cs, w, w.Player.Location, 0)
		if hot(w) {
			w.SetLieLow(true)
			return
		}
		standSomewhere(w)
		contract(w)
		// The orders count on the morning's buys (PlaceSell allows what
		// the contract brings, and the sim's plan is what it will), so
		// the stash sells the night it lands; an order for more than
		// lands would cost heat for units never moved.
		city := w.Player.Location
		for _, id := range w.Products {
			if q := w.Stock(city, id) + mk.Due(w, city, id); q > 0 {
				_ = w.PlaceSell(city, id, q, events.DialNormal)
			}
		}
		for _, cid := range w.CityOrder {
			if cid == city {
				continue
			}
			for _, id := range w.Products {
				if q := w.Stock(cid, id); q > 0 {
					_ = w.PlaceSell(cid, id, q, events.DialNormal)
				}
			}
		}
	}
}

// StockDays is how many days of the worked corners' demand the stocked
// player keeps the stash at: the night's sales and one more.
const StockDays = 1

// contract sets the stocked player's supply contracts where it stands:
// StockDays of demand per product, cut to the bag's share of the
// product by demand where the levels together would overfill it
// (World.StockLevels, the market screen's restock too, #356).
func contract(w *game.World) {
	city := w.Player.Location
	levels := w.StockLevels(city, StockDays)
	for _, id := range w.Products {
		units, ok := levels[id]
		if !ok {
			continue
		}
		if units > 0 {
			_ = w.SetSupply(city, id, units)
		} else {
			w.ClearSupply(city, id)
		}
	}
}
