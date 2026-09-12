package market

import (
	"math"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
)

// Quality (#47). Every lot the player holds carries a quality
// (game.World.Quality, one number a product a city); the market sim is
// where it meets the street: the sale price and a buyer's premium pay
// its multiplier (QualityMul), the corners remember what they were sold
// (Corner.Repeat, the market's field on the territory's corner: the
// share of a corner's customers who come back, which World.HeldShare
// folds into the served demand so the market, the heat sim, the UI and
// the harness read one number), and bad hard product overdoses
// (events.Overdose, off Tick.Sub("overdose")). The file's default is
// stamped onto the world (World.BaseQuality) at seed and every morning
// as the markup is, and every corner's Repeat at repeat_start where it
// reads zero, so a save from before #47 follows the file. With the
// table in its box (Default 0) nothing here stamps, moves or rolls.

// boxed reports whether the quality table is in its box: no default,
// so nothing stamps, no corner moves and nothing overdoses.
func (s *Sim) boxed() bool { return s.cfg.Quality.Default <= 0 }

// Default is the file's default quality: what the connects sell at and
// where the multiplier is 1; game.StreetQuality with the table boxed.
func (s *Sim) Default() float64 {
	if s.boxed() {
		return game.StreetQuality
	}
	return s.cfg.Quality.Default
}

// Tuning is the [quality] table, for the UI's words.
func (s *Sim) Tuning() content.QualityTuning { return s.cfg.Quality }

// QualityMul is the price multiplier at a quality: low_mul at 0, 1 at
// the default, high_mul at 100, linear either side.
func (s *Sim) QualityMul(quality float64) float64 { return s.cfg.Quality.Mul(quality) }

// CutMax is the most a cut of a product can add, as a ratio of the
// units; 0 for a product that cannot be cut.
func (s *Sim) CutMax(product string) float64 {
	if p := s.cfg.Product(product); p != nil {
		return p.CutMax
	}
	return 0
}

// CutCost is what a cut of a product costs a unit added, dirty cash.
func (s *Sim) CutCost(product string) int {
	if p := s.cfg.Product(product); p != nil {
		return p.CutCost
	}
	return 0
}

// CookCost is what a chemist's precursors cost a unit of a product,
// dirty cash; 0 for a product that is bought, never cooked.
func (s *Sim) CookCost(product string) int {
	if p := s.cfg.Product(product); p != nil {
		return p.CookCost
	}
	return 0
}

// Cooks reports whether a product is one a chemist cooks.
func (s *Sim) Cooks(product string) bool { return s.CookCost(product) > 0 }

// stampQuality stamps the file's default onto the world and repeat_start
// onto every corner that reads zero: at seed, every morning and on
// migration, so a save follows the file and a corner laid out by a
// later step starts with all its customers.
func (s *Sim) stampQuality(w *game.World) {
	if s.boxed() {
		return
	}
	w.BaseQuality = s.cfg.Quality.Default
	if start := s.cfg.Quality.RepeatStart; start > 0 {
		for _, cid := range w.CityOrder {
			city := w.Cities[cid]
			for i := range city.Corners {
				// Repeat is the market's field on the territory's corner
				// (TestSimsWriteOnlyTheirOwnState lists it).
				if c := &city.Corners[i]; c.Repeat == 0 {
					c.Repeat = start
				}
			}
		}
	}
}

// MigrateLots is the 12 -> 13 step (#47): the default quality onto
// every lot, shipment and connect a save from before quality carried
// with units alone, and repeat_start onto every corner, so it plays on
// as it did (World.MigrateLots); then the connects' figures from the
// file where it gives one.
func (s *Sim) MigrateLots(w *game.World) {
	w.MigrateLots(s.Default(), s.cfg.Quality.RepeatStart)
	s.stampQuality(w)
	for i := range w.Suppliers {
		sup := &w.Suppliers[i]
		sc := s.scfg.Supplier(sup.ID)
		if sc == nil {
			continue
		}
		for id := range sup.Price {
			sup.Quality[id] = sc.QualityOf(id, s.Default()) // the price as saved, never restamped
		}
	}
}

// soldQuality is what a city's worked corners were sold tonight (#47):
// the units and the quality of every street sale, for the corners'
// repeat business. The market keeps one per city for the step in hand.
type soldQuality struct {
	units   float64
	weighed float64 // units x quality
}

func (q *soldQuality) add(units int, quality float64) {
	q.units += float64(units)
	q.weighed += float64(units) * quality
}

// mean is the quality sold, by units; the default with nothing sold.
func (q soldQuality) mean(def float64) float64 {
	if q.units <= 0 {
		return def
	}
	return q.weighed / q.units
}

// repeat moves every worked corner's repeat business in a city off what
// it was sold tonight: under the floor it falls by repeat_loss, sold
// better it recovers repeat_gain, never under repeat_min or over 1; a
// corner sold nothing (a quiet day, nobody on it) remembers what it was
// last sold and holds, so a rest day is no cure and good product is.
// Every worked corner in the city sold the same mix (an order is the
// city's), so the day's quality is one number for them all. A corner
// sold only at the default never moves, so a run that never cuts serves
// what it did.
func (s *Sim) repeat(w *game.World, city *game.City, sold soldQuality) {
	if s.boxed() || sold.units <= 0 {
		return
	}
	q := s.cfg.Quality
	for i := range city.Corners {
		c := &city.Corners[i]
		if c.Repeat == 0 || !c.Worked() {
			continue
		}
		if sold.mean(s.Default()) < q.RepeatFloor {
			c.Repeat = math.Max(q.RepeatMin, c.Repeat-q.RepeatLoss)
		} else {
			c.Repeat = math.Min(1, c.Repeat+q.RepeatGain)
		}
	}
}

// overdoses rolls for the night's overdoses on a city's corners (#47):
// hard product sold under od_quality rolls once per od_units (a part
// of one in proportion) at od_chance, off the overdoses' own side
// stream, and every hit names one of the corners worked there, drawn by
// their share. It is pressure, notoriety and a headline, never a page
// (#27): the event carries nothing the heat sim reads.
func (s *Sim) overdoses(w *game.World, t *game.Tick, city, product string, units int, quality float64) {
	q := s.cfg.Quality
	if s.boxed() || units <= 0 || q.OdUnits <= 0 || q.OdChance <= 0 || quality >= q.OdQuality || !q.Hard(product) {
		return
	}
	rng := t.Sub("overdose")
	c := w.Cities[city]
	rolls := float64(units) / q.OdUnits
	for rolls > 0 {
		chance := q.OdChance * math.Min(1, rolls)
		rolls--
		if rng.Float64() >= chance {
			continue
		}
		ev := events.Overdose{Day: t.Day, City: city, Product: product, Quality: quality}
		// The corner: one of yours worked there, by share.
		total := 0.0
		for _, k := range c.Corners {
			if k.Worked() {
				total += k.Share(product)
			}
		}
		if total > 0 {
			pick := rng.Float64() * total
			for _, k := range c.Corners {
				if !k.Worked() {
					continue
				}
				pick -= k.Share(product)
				if pick <= 0 {
					ev.Corner, ev.CornerName = k.ID, k.Name
					break
				}
			}
		}
		w.Stats.Overdoses++
		t.Emit(ev)
	}
}
