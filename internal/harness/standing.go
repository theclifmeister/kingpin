package harness

import (
	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
)

// Routine plays like Crewed and, once the operation is running, never
// places an order by hand (#114): it restocks as Crewed does, sells by
// hand until its peak cash clears RoutineCash (nobody automates on day
// one with a $500 bag, and the early game has no free cash flow for a
// cut to come out of) and from then on keeps a standing order for every
// stash at the normal dial, which the market resolves every night at
// the crew's cut. It raises the order when the stash outgrows it (a
// standing order sells at most its units) and never lowers it (a
// standing order sells at most what is stashed). It lies low when hot
// as Crewed does, and a standing order sells nothing on a lie-low day.
// It is the baseline for what the routine costs against the hand.
func Routine(cfg *content.Config, lieLowAt float64) Policy {
	hot := TooHot(cfg, lieLowAt)
	return func(w *game.World) {
		staff(cfg, w, w.Player.Location, 0)
		if hot(w) {
			w.SetLieLow(true)
			return
		}
		standSomewhere(w)
		restock(cfg, w)
		if w.Stats.PeakCash < RoutineCash {
			sellEverything(w, events.DialNormal)
			return
		}
		stand(w, events.DialNormal)
	}
}

// RoutineCash is the peak cash at which the routine player stops
// selling by hand and sets its standing orders: the player the feature
// is for has an operation to leave to the crew.
const RoutineCash = 20_000

// stand keeps a standing order at the dial for every stash: one is set
// where none stands, or where the stash (and what the contract brings)
// has outgrown the one that does.
func stand(w *game.World, dial events.Dial) {
	for _, cid := range w.CityOrder {
		for _, id := range w.Products {
			q := w.Stock(cid, id) + w.SupplyDue(cid, id)
			if q <= 0 {
				continue
			}
			if o, ok := w.YourStanding(cid, id); ok && o.Qty >= q && o.Dial == dial {
				continue
			}
			_ = w.PlaceStanding(cid, id, q, dial)
		}
	}
}
