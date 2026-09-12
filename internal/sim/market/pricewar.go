package market

import (
	"math"

	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
)

// The price war (#68): working a corner cheap next to one of the rival's
// takes a share of its demand. The player queues it (World.Undercut, a
// rival corner and a dial, per-day scratch); the night's orders at home
// serve that share on top of the player's own corners' demand, at
// price_cut off, out of what is left of the order once their own corners
// have had theirs (resolveAt); and the rival corner's Squeeze carries the
// share of its trade taken that day, which the rivals sim reads as less
// income there without knowing who did it. No dice anywhere in it, and
// a run that never undercuts is the old run: the rival's corners start
// every day unsqueezed, as they always were.

// warBook is the step's books on the price war: the rival corners
// undercut tonight, in the home city's corner order, each with the value
// of its trade at the day's opening prices and what the orders took of
// it as they resolved product by product.
type warBook struct {
	corners []*game.Corner
	dials   map[string]events.Dial
	total   map[string]float64 // corner id -> the day's trade there, units times price over every product
	taken   map[string]float64 // corner id -> what the orders took of it, units times price
}

// take books units of trade taken off a corner.
func (b *warBook) take(corner string, value float64) {
	if b == nil {
		return
	}
	b.taken[corner] += value
}

// cut is what one order moved off one rival corner tonight.
type cut struct {
	Corner  string
	Name    string
	Dial    events.Dial
	Units   int
	Share   float64 // of the corner's demand for the product
	Revenue int
}

// Steal is the share of a rival corner's demand a price war on it at
// the dial takes off it today: steal at normal, scaled by the dial's
// impact and by the worked share of its neighbours you hold
// (World.NextDoor). Zero on a corner that is not the rival's or that
// none of your worked corners borders. The picker shows it and the
// resolution uses it.
func (s *Sim) Steal(w *game.World, c game.Corner, dial events.Dial) float64 {
	return math.Min(1, s.war.Steal*s.Dial(dial).Impact*w.NextDoor(c))
}

// PriceCut is the discount off street price the undercut units sell at.
func (s *Sim) PriceCut() float64 { return s.war.PriceCut }

// UndercutPrice is what a unit of a product moved off a rival corner in a
// city at the dial makes today: the street price at the dial's price,
// less price_cut. The night's price impact comes off it as it does off
// any sale.
func (s *Sim) UndercutPrice(w *game.World, city, product string, dial events.Dial) float64 {
	m := w.Product(city, product)
	if m == nil {
		return 0
	}
	return m.Price * s.Dial(dial).Price * (1 - s.war.PriceCut)
}

// UndercutUnits is how many units a day a price war on a rival corner at
// the dial takes off it across every product the city lists, if the
// night's orders cover them: the corner's full demand for each times
// Steal. The picker's estimate.
func (s *Sim) UndercutUnits(w *game.World, c game.Corner, dial events.Dial) float64 {
	n := 0.0
	steal := s.Steal(w, c, dial)
	for _, id := range w.Products {
		if m := w.Product(c.City, id); m != nil {
			n += m.Demand * c.Full(id) * steal
		}
	}
	return n
}

// openWar starts the day's books for a city: the rival's corners there
// begin the day unsqueezed, and every undercut queued on one of them
// that is still legal tonight (World.CanUndercut: still theirs, still
// next door to a corner you work, no deal covering it) is booked with
// the value of its trade at the opening prices. Nil for a city with no
// rival corners undercut, which is every city but home and every day
// with nothing queued.
func (s *Sim) openWar(w *game.World, t *game.Tick, city *game.City) *warBook {
	var book *warBook
	for i := range city.Corners {
		c := &city.Corners[i]
		if c.Owner != game.OwnerRival {
			continue
		}
		c.Squeeze = 0
		dial, ok := w.Undercutting(c.ID)
		if !ok || w.CanUndercut(c.ID) != nil {
			continue
		}
		if book == nil {
			book = &warBook{dials: map[string]events.Dial{}, total: map[string]float64{}, taken: map[string]float64{}}
		}
		total := 0.0
		for _, id := range w.Products {
			if m := city.Market[id]; m != nil {
				total += m.Demand * c.Full(id) * m.Price
			}
		}
		book.corners = append(book.corners, c)
		book.dials[c.ID] = dial
		book.total[c.ID] = total
	}
	return book
}

// undercut is what the rival's corners booked tonight would take of an
// order for a product, in the book's order: up to Steal of each corner's
// demand for the product, capped, never stretched, by the fill the dial
// gets today (a patrol cap holds here as on any sale). Nil with nothing
// booked.
func (s *Sim) undercut(w *game.World, product string, m *game.ProductMarket) []cut {
	b := s.book
	if b == nil {
		return nil
	}
	var cuts []cut
	for _, c := range b.corners {
		dial := b.dials[c.ID]
		full := m.Demand * c.Full(product)
		want := full * s.Steal(w, *c, dial) * math.Min(1, s.Fill(w, dial))
		units := int(math.Round(want))
		if units <= 0 || full <= 0 {
			continue
		}
		cuts = append(cuts, cut{Corner: c.ID, Name: c.Name, Dial: dial, Units: units, Share: float64(units) / full})
	}
	return cuts
}

// share is how an order for a product is shared between your own
// corners and the rival's corners undercut tonight (#68): each pool
// takes what it can absorb when the order and the stash cover both, and
// an order too small for both is split pro rata by what each absorbs,
// your own corners taking the rounding. It returns what each rival
// corner takes, their sum, and what your own corners take, which is
// sold as the caller worked it out with nothing booked, so a run with
// no price war is the old run.
func (s *Sim) share(w *game.World, city, product string, m *game.ProductMarket, o game.SellOrder, sold int) ([]cut, int, int) {
	cuts := s.undercut(w, product, m)
	if len(cuts) == 0 {
		return nil, 0, sold
	}
	own := s.Capacity(w, city, product, o.Dial)
	avail := max(0, min(o.Qty, w.Stock(city, product)))
	total := own
	for _, u := range cuts {
		total += u.Units
	}
	if avail < total && total > 0 {
		for i := range cuts {
			cuts[i].Units = int(float64(avail) * float64(cuts[i].Units) / float64(total))
		}
	}
	undercut := 0
	kept := cuts[:0]
	for _, u := range cuts {
		if u.Units <= 0 {
			continue
		}
		full := m.Demand * w.Corner(u.Corner).Full(product)
		u.Share = float64(u.Units) / full
		undercut += u.Units
		kept = append(kept, u)
	}
	return kept, undercut, min(own, avail-undercut)
}

// closeWar writes the day's squeeze onto every corner in the books: the
// share of its trade the orders took, which the rivals sim reads as
// less income there. Nothing booked, nothing written.
func (s *Sim) closeWar(city *game.City) {
	b := s.book
	if b == nil {
		return
	}
	for _, c := range b.corners {
		if total := b.total[c.ID]; total > 0 {
			c.Squeeze = math.Max(0, math.Min(1, b.taken[c.ID]/total))
		}
	}
}
