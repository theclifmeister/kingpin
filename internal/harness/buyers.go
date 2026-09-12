package harness

import (
	"math"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/sim/heat"
)

// Dealer plays like Crewed and works the buyers (#71): every offer in
// the city it stands in that it can cover from the stash there plus one
// buy from the supplier it takes, and it declines every offer it cannot
// cover, or that is in the other city. It buys the shortfall before the
// day's restock so the stash has room for it, keeps what a contract
// still wants out of the night's street orders, and hands over what it
// holds against a contract as soon as the heat allows: a handoff is
// read off the same dial preview the market screen shows (heat.Sim), and
// it goes in only as far as it keeps the city under the sting line by
// DealerMargin with tonight's street sales counted, the rest waiting for
// a cooler day, until the due day, when everything it holds goes over
// whatever the heat (a buyer let down costs more). On a day it lies low
// it delivers nothing, like everyone else. It is the baseline for "a
// player who visits the market screen".
func Dealer(cfg *content.Config, lieLowAt float64) Policy {
	crewed := Crewed(cfg, lieLowAt)
	hot := TooHot(cfg, lieLowAt)
	hs := heat.New(cfg)
	var sting *content.ResponseConfig
	for i := range cfg.Heat.Responses {
		if cfg.Heat.Responses[i].Level == "sting" {
			sting = &cfg.Heat.Responses[i]
		}
	}
	return func(w *game.World) {
		if hot(w) {
			crewed(w) // lies low; a handoff is refused anyway
			return
		}
		answer(cfg, w)
		crewed(w)
		reserve(w)
		line := math.Inf(1)
		if sting != nil {
			line = hs.Threshold(w, *sting, w.Here()) - DealerMargin
		}
		handOver(hs, w, line)
	}
}

// reserve keeps what the contracts you hold where you stand still want
// out of tonight's street orders there: the street gets what is left,
// and the handoff can wait for a cooler night without the stock walking.
func reserve(w *game.World) {
	here := w.Player.Location
	owed := map[string]int{}
	for _, c := range w.Contracts {
		if c.City == here && c.Live(w.Day) {
			owed[c.Product] += c.Owed()
		}
	}
	for product, n := range owed {
		o, ok := w.Order(here, product)
		if !ok {
			continue
		}
		if keep := w.Stock(here, product) - n; keep <= 0 {
			w.CancelSell(here, product)
		} else if o.Qty > keep {
			_ = w.PlaceSell(here, product, keep, o.Dial)
		}
	}
}

// DealerMargin is how far under the sting line the dealer keeps the city
// on a night it hands product to a buyer.
const DealerMargin = 4

// Welsher plays like Dealer but never delivers against the first
// contract it takes: the buyer let down, for the tests that measure what
// a failure costs. It reports the id of the contract it welshed on
// through *welshed once it has taken one (0 until then).
func Welsher(cfg *content.Config, lieLowAt float64, welshed *int) Policy {
	dealer := Dealer(cfg, lieLowAt)
	return func(w *game.World) {
		dealer(w)
		if *welshed == 0 {
			for _, c := range w.Contracts {
				if c.Status == game.ContractAccepted {
					*welshed = c.ID
					break
				}
			}
		}
		if *welshed != 0 {
			delete(w.Deliveries, *welshed)
		}
	}
}

// answer takes or declines the offers where you stand and buys the
// shortfall on every contract you hold there: the Dealer's morning at
// the market, before the day's restock fills the stash.
func answer(cfg *content.Config, w *game.World) {
	here := w.Player.Location
	pressure := cfg.Market.Market.BuyPricePressure * game.FoldEffects(w, cfg.Upgrades).BuyPressureMul
	for i := range w.Contracts {
		c := &w.Contracts[i]
		if c.City != here {
			if c.Open(w.Day) {
				_ = w.DeclineContract(c.ID)
			}
			continue
		}
		if c.Open(w.Day) {
			if !canCover(w, *c) {
				_ = w.DeclineContract(c.ID)
				continue
			}
			_ = w.AcceptContract(c.ID)
		}
		if !c.Live(w.Day) {
			continue
		}
		if short := c.Owed() - w.Stock(here, c.Product); short > 0 {
			affordable(w, c.Product, short, pressure)
		}
	}
}

// handOver queues tonight's handoffs where you stand: against every
// contract you hold there, as many units as keep the city under line
// once tonight's street orders have added their heat, and everything on
// its due day.
func handOver(hs *heat.Sim, w *game.World, line float64) {
	here := w.Here()
	street := 0.0
	for _, o := range w.Orders {
		if o.City == here.ID {
			street += hs.SaleHeat(w, o.City, o.Product, o.Qty, o.Dial)
		}
	}
	room := line - here.Heat - street
	for _, c := range w.Contracts {
		if c.City != here.ID || !c.Live(w.Day) {
			continue
		}
		n := w.Deliverable(c)
		if n <= 0 {
			continue
		}
		if c.Due > w.Day {
			if per := hs.ContractHeat(w, c.City, c.Product, 1, c.HeatMul); per > 0 {
				n = min(n, int(math.Floor(room/per)))
			}
		}
		if n > 0 && w.Deliver(c.ID, n) == nil {
			room -= hs.ContractHeat(w, c.City, c.Product, n, c.HeatMul)
		}
	}
}

// canCover reports whether the stash here plus one buy the supplier will
// make covers a contract in full: the cash in hand has to pay for the
// shortfall and the stash has to have room for it, counting the room
// tonight's street sales of the product make (its demand here), since
// the handoff can wait a day for the rest.
func canCover(w *game.World, c game.Contract) bool {
	short := c.Owed() - w.Stock(c.City, c.Product)
	if short <= 0 {
		return true
	}
	if float64(short) > float64(w.Free(c.City))+w.Demand(c.City, c.Product) {
		return false
	}
	sup := retail(w, c.City, c.Product)
	return sup != nil && short <= sup.Left() && sup.Quote(c.Product, short, false) <= w.Player.DirtyCash
}

// affordable buys up to qty units of a product here, as many as the cash
// and the room allow, from the cheapest connect that sells it today.
func affordable(w *game.World, product string, qty int, pressure float64) {
	buyHere(w, product, qty, 0, pressure, false)
}
