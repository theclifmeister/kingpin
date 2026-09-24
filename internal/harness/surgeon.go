package harness

import (
	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
)

// SurgeonLine is the heat, on the ladder as heat.toml prints it, at
// which the surgeon turns its dial down to quiet: over the sting line,
// where the investigations open, and well under the raid's.
const SurgeonLine = 60

// Surgeon answers a targeted investigation (#343) and never lies low: it
// sells at the normal dial, quiet from quietAt (TooHot's line, so it
// moves with the DA's sting line), and while an investigation is open it
// takes the named target out of use. A corner it stands on, it walks off
// to another one in the city (the investigation follows the corner, not
// the name); a product, it stops selling and buying in that city and
// moves what it holds there into a house when it has one; a house, it
// empties onto the street. When the investigation closes it stays where
// it went. It is the issue's player: lying low is still there, and this
// one never needs it (TestSurgeonBeatsManaged).
func Surgeon(cfg *content.Config, quietAt float64) Policy {
	hot := TooHot(cfg, quietAt)
	return func(w *game.World) {
		inv := w.Heat.Investigation
		dial := events.DialNormal
		if hot(w) {
			dial = events.DialQuiet
		}
		dodge(w, inv)
		standOffTarget(w, inv)
		skip := func(product string) bool {
			return inv.Open() && inv.Kind == game.LeadProduct && inv.Target == product && inv.City == w.Player.Location
		}
		restockOnly(cfg, w, func(id string) bool { return !skip(id) })
		for _, cid := range w.CityOrder {
			for _, id := range w.Products {
				if inv.Open() && inv.Kind == game.LeadProduct && inv.Target == id && inv.City == cid {
					continue
				}
				if q := w.Stock(cid, id); q > 0 {
					_ = w.PlaceSell(cid, id, q, dial)
				}
			}
		}
	}
}

// standOffTarget is standSomewhere that never stands on the corner an
// open investigation names: off it if you are on it, onto the biggest
// other corner in the city you hold and nobody works, else the biggest
// free one. With none, you stand nowhere tonight.
func standOffTarget(w *game.World, inv game.Investigation) {
	named := func(c game.Corner) bool {
		return inv.Open() && inv.Kind == game.LeadCorner && inv.Target == c.ID
	}
	if c := w.PostOf(game.You); c != nil {
		if !named(*c) {
			return
		}
		w.Recall(game.You)
	}
	here := w.Player.Location
	if c := pickCorner(w, func(c game.Corner) bool { return c.City == here && c.Held() && c.Runner == 0 && !named(c) }, size); c != nil {
		_ = w.Post(c.ID, game.You)
	} else if c := pickCorner(w, func(c game.Corner) bool { return c.City == here && c.Owner == game.OwnerNone && !named(c) }, size); c != nil {
		_ = w.Post(c.ID, game.You)
	}
}

// dodge moves stock out of an open investigation's way: a named house
// emptied onto the street, a named product's street stock put in a house
// in the city when there is one.
func dodge(w *game.World, inv game.Investigation) {
	if !inv.Open() {
		return
	}
	switch inv.Kind {
	case game.LeadHouse:
		if h := w.House(inv.Target); h != nil {
			for _, id := range w.SortedProducts() {
				if q := h.Stock[id]; q > 0 {
					_, _ = w.Move(h.City, h.ID, "", id, q)
				}
			}
		}
	case game.LeadProduct:
		for i := range w.Houses {
			h := &w.Houses[i]
			if h.City != inv.City {
				continue
			}
			if q := w.StreetOf(inv.City)[inv.Target]; q > 0 {
				_, _ = w.Move(inv.City, "", h.ID, inv.Target, q)
			}
			return
		}
	}
}
