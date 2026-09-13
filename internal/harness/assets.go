package harness

import (
	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/sim/crew"
	"github.com/theclifmeister/kingpin/internal/sim/heat"
	"github.com/theclifmeister/kingpin/internal/sim/laundering"
	"github.com/theclifmeister/kingpin/internal/sim/logistics"
	"github.com/theclifmeister/kingpin/internal/sim/market"
)

// The assets (#48). Cartel is the tier-5 policy: Boss (the assets are
// measured on the operation they are for, as the tree is on upgraded
// and boss, never on the lone trader), buying each asset first thing
// in the morning its door is open and the price is in hand, cheapest
// first, before the levels take the rest of the pile (a standing
// reserve for the next asset held the levels back a few days at a time
// and cost more than the assets did); turning on the plane and the
// tunnel with the road's targets once they are open; hiring a chemist,
// renting the houses at home for the lots and cooking what the file
// cooks in the lab once it stands, the road sending only what is not
// cooking; and lying low the day a task force is announced (the night
// it comes files nothing, #27). It is the money curve's tier-5 row:
// `cmd/balance -policy cartel`.

// Cartel plays like Boss and buys the assets (#48).
func Cartel(cfg *content.Config, lieLowAt float64) Policy {
	ld := laundering.New(cfg)
	boss := Boss(cfg, lieLowAt, "")
	cs := crew.New(cfg)
	ht := heat.New(cfg)
	lg := logistics.New(cfg)
	mk, err := market.New(cfg)
	if err != nil {
		panic("harness.Cartel: " + err.Error())
	}
	home := cfg.City.Home().ID
	return func(w *game.World) {
		BuyAssets(ld, w, 1)
		boss(w)
		if w.Over != nil {
			return
		}
		// The routes an asset opened (#48): on, at the biggest open
		// route's targets, so the same seeds read what a plane and a
		// tunnel are worth beside the boat.
		var main *content.RouteConfig
		for _, r := range lg.RoutesOpen(w, home) {
			rc := r
			if w.Route(r.ID).Dial.On() && r.Asset == "" && (main == nil || r.Capacity > main.Capacity) {
				main = &rc
			}
		}
		for _, r := range lg.RoutesOpen(w, home) {
			if r.Asset == "" || r.To != home || main == nil {
				continue
			}
			if !w.Route(r.ID).Dial.On() {
				_ = w.SetRoute(r.ID, events.RouteNormal)
			}
			for _, id := range w.Products {
				_ = w.SetRouteTarget(r.ID, id, w.Route(main.ID).Target[id])
			}
		}
		// The lab: a chemist, the houses at home for the lots, and what
		// the file cooks (meth, designer) cooked at home toward the
		// road's target for it, the road sending only what the lab has
		// not got on: a lot cooking is not on the road, so the target
		// every open route keeps comes down by it.
		if cs.Lab(w, home) != nil && main != nil {
			hireChemist(cfg, w)
			labRoom(cfg, w, home)
			if w.Crew.Chemist() != nil {
				targets := map[string]int{}
				for _, id := range w.Products {
					targets[id] = w.Route(main.ID).Target[id]
				}
				cookInLab(cfg, mk, cs, w, home, targets)
				for _, r := range lg.RoutesOpen(w, home) {
					for _, id := range w.Products {
						if mk.Cooks(id) && w.Route(r.ID).Target[id] > 0 {
							_ = w.SetRouteTarget(r.ID, id, max(0, targets[id]-w.Crew.Cooking(home, id)))
						}
					}
				}
			}
		}
		// A task force announced this morning comes tonight: a quiet
		// night is stock and cash lost, never a page.
		if ht.TaskForceForming(w) {
			w.SetLieLow(true)
		}
	}
}

// NextAsset is the price of the cheapest asset on offer that is open
// and not owned, or nothing: what the cartel keeps in hand for it.
func NextAsset(ld *laundering.Sim, w *game.World) int {
	if o := nextAsset(ld, w); o != nil {
		return o.Cost
	}
	return 0
}

// nextAsset is the cheapest asset on offer, open, not owned and not
// gone for good, or nil.
func nextAsset(ld *laundering.Sim, w *game.World) *game.AssetOffer {
	for _, o := range ld.AssetOffers() {
		if w.HasAsset(o.ID) || o.Locked(w) {
			continue
		}
		if lost := w.AssetLost(o.ID); lost != nil && lost.Why == "found" {
			continue
		}
		return &o
	}
	return nil
}

// BuyAssets buys the cheapest asset on offer that is open and not owned
// when clean cash is margin times its price, one a day. It reports
// whether one was bought.
func BuyAssets(ld *laundering.Sim, w *game.World, margin float64) bool {
	o := nextAsset(ld, w)
	if o == nil || float64(w.Player.CleanCash) < margin*float64(o.Cost) {
		return false
	}
	_, err := w.BuyAsset(*o)
	return err == nil
}

// cookInLab orders a cook of every product the file cooks in city,
// toward the target given for it less what is stashed, on the road and
// cooking, up to the lab's batch and the till over the float; nothing
// with no chemist.
func cookInLab(cfg *content.Config, mk *market.Sim, cs *crew.Sim, w *game.World, city string, targets map[string]int) {
	if w.Crew.Chemist() == nil {
		return
	}
	for _, id := range w.Products {
		if !mk.Cooks(id) {
			continue
		}
		short := targets[id] - w.Stock(city, id) - w.Bound(city, id) - w.Crew.Cooking(city, id)
		units := min(short, cs.BatchIn(w, city), w.Free(city)-w.Crew.Cooking(city, id))
		cost := cs.CookCostIn(w, city, mk.CookCost(id))
		if cost > 0 {
			units = min(units, (w.Player.DirtyCash-cfg.Laundering.Laundering.Float)/cost)
		}
		if units <= 0 {
			continue
		}
		_, _ = w.CookOrder(city, id, units, cost, cs.CookDays(), cs.QualityIn(w, city), cs.BatchIn(w, city), cs.ChemistName(w))
	}
}

// labRoom rents the stash houses at home for the lab's lots (#73: a
// cook lands in the city's capacity, and the street's is the runners'
// bags), the cheapest on offer it lacks, one a day, at HouseMargin the
// price and RentCover days of the rent in clean cash.
func labRoom(cfg *content.Config, w *game.World, city string) {
	offers := HouseOffers(cfg, city)
	for _, o := range offers {
		if w.House(o.ID) != nil || o.Locked(w) {
			continue
		}
		if float64(w.Player.DirtyCash) >= HouseMargin*float64(o.Price) && w.Player.CleanCash >= RentCover*o.Rent {
			_, _ = w.BuyHouse(o)
		}
		return
	}
}

// Reckless is the cartel that stops caring (#48): Cartel that never
// lies low and, from the morning it owns its first asset, sells every
// unit it holds in every city at the aggressive dial. It is what the
// task force is for: the managed cartel's heat sits under the sting
// line and the boss's Security branch holds a loud one near the raid's,
// so only the cartel that shouts reaches the feds' line, and it loses
// an asset when it does (`-policy reckless`, TestAggressiveCartelLoses
// ItsAssets).
func Reckless(cfg *content.Config) Policy {
	managed, loud := Cartel(cfg, 40), Cartel(cfg, 100)
	return func(w *game.World) {
		if len(w.Assets) == 0 {
			managed(w)
			return
		}
		loud(w)
		w.SetLieLow(false)
		for _, cid := range w.CityOrder {
			for _, id := range w.Products {
				if q := w.Stock(cid, id); q > 0 {
					_ = w.PlaceSell(cid, id, q, events.DialAggressive)
				}
			}
		}
	}
}
