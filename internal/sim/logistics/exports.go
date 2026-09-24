package logistics

import (
	"math"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
)

// The export lanes (#391, exports.toml, docs/exports.md): demand the
// corners do not bound. A lane is open while the Dutchman's book (the
// supplier asset) stands and, if it names one, its own asset too. Each
// night a lane with a standing order loads what it carries and the till
// pays for, bought straight off the book at its own_ratio of street in
// the book's city, and sends it; a load lands paid in dirty cash at the
// rate locked the night it left, or is seized on the way. One roll a
// load, on landing, off the exports side stream: a run with no lane open
// draws nothing and replays as it did before the file.

// Lanes lists every lane in the file, in file order.
func (s *Sim) Lanes() []content.LaneConfig { return s.exports.Lanes }

// Lane returns the lane with id, or nil.
func (s *Sim) Lane(id string) *content.LaneConfig { return s.exports.Lane(id) }

// ExportsTuning is the lanes' shared numbers, for the UI and the harness.
func (s *Sim) ExportsTuning() content.ExportsTuning { return s.exports.Exports }

// book is the supplier asset's row: the book a lane buys off.
func (s *Sim) book() *content.AssetConfig { return s.assets.ByEffect(content.AssetSupplier) }

// LaneOpen reports whether a lane can ship tonight: the book standing
// and the lane's own asset, if it names one.
func (s *Sim) LaneOpen(w *game.World, l content.LaneConfig) bool {
	b := s.book()
	return b != nil && w.AssetLive(b.ID) && (l.Asset == "" || w.AssetLive(l.Asset))
}

// LanesOpen lists the lanes open tonight, in file order.
func (s *Sim) LanesOpen(w *game.World) []content.LaneConfig {
	var out []content.LaneConfig
	for _, l := range s.exports.Lanes {
		if s.LaneOpen(w, l) {
			out = append(out, l)
		}
	}
	return out
}

// LaneCapacity is the most a night's load on the lane carries: the
// file's, times the port's capacity_mul for a boat out of its city
// while it stands (the port's rule for the road, #48).
func (s *Sim) LaneCapacity(w *game.World, l content.LaneConfig) int {
	c := float64(l.Capacity)
	if a := s.assets.ByEffect(content.AssetPort); a != nil && l.Mode == "boat" && a.City == l.City && w.AssetLive(a.ID) {
		c *= a.CapacityMul
	}
	return int(c)
}

// ExportCost is what a unit of the product costs off the book tonight:
// its own_ratio of the street price in the book's city, zero where the
// product is not listed there or there is no book.
func (s *Sim) ExportCost(w *game.World, product string) float64 {
	b := s.book()
	if b == nil {
		return 0
	}
	m := w.Product(b.City, product)
	if m == nil {
		return 0
	}
	return m.Price * b.OwnRatio
}

// ExportPrice is what a unit of the product sent on the lane tonight
// would be paid on landing: the street price in the lane's city times
// the lane's price, less the glut, never under the floor.
func (s *Sim) ExportPrice(w *game.World, l content.LaneConfig, product string) float64 {
	m := w.Product(l.City, product)
	if m == nil {
		return 0
	}
	return m.Price * l.Price * s.GlutMul(w, l, product)
}

// GlutMul is what the glut leaves of the lane's price on a product.
func (s *Sim) GlutMul(w *game.World, l content.LaneConfig, product string) float64 {
	return math.Max(s.exports.Exports.GlutFloor, 1-w.Glut(l.ID, product))
}

// LaneRisk is the chance a load on the lane is seized tonight: the
// file's, times watched_mul while the feds watch.
func (s *Sim) LaneRisk(w *game.World, l content.LaneConfig, day int) float64 {
	r := l.Risk
	if Watched(w, day) {
		r *= s.exports.Exports.WatchedMul
	}
	return math.Min(1, r)
}

// LoadTonight is what the lane's standing order would load tonight: the
// order, capped by the lane's capacity and by what the till over the
// float pays for off the book. Zero when the lane is shut or the
// product cannot be had.
func (s *Sim) LoadTonight(w *game.World, l content.LaneConfig, budget int) (units, cost int) {
	o := w.ExportOrder(l.ID)
	if !o.On() || !l.Takes(o.Product) || !s.LaneOpen(w, l) || s.ExportPrice(w, l, o.Product) <= 0 {
		return 0, 0
	}
	unit := s.ExportCost(w, o.Product)
	if unit <= 0 {
		return 0, 0
	}
	units = min(o.Units, s.LaneCapacity(w, l), int(float64(budget)/unit))
	if units <= 0 {
		return 0, 0
	}
	return units, int(math.Round(unit * float64(units)))
}

// exportsStep is the lanes' night: the loads due land or are seized, the
// glut eases, the record is pruned, then every open lane with an order
// loads and sends what it can. Landing first, so what lands is in the
// till for tonight's loads.
func (s *Sim) exportsStep(w *game.World, t *game.Tick) {
	ex := &w.Exports
	if len(ex.Loads) == 0 && len(ex.Orders) == 0 && len(ex.Glut) == 0 && len(ex.Record) == 0 {
		return
	}
	tun := s.exports.Exports
	// The landings, in the order they left; one roll each.
	if len(ex.Loads) > 0 {
		rng := t.Sub(game.StreamExports)
		kept := ex.Loads[:0]
		for _, ld := range ex.Loads {
			if ld.Lands > t.Day {
				kept = append(kept, ld)
				continue
			}
			l := s.exports.Lane(ld.Lane)
			name, mode, city, risk := ld.Lane, "", "", 0.0
			if l != nil {
				name, mode, city, risk = l.Name, l.Mode, l.City, s.LaneRisk(w, *l, t.Day)
			}
			watched := Watched(w, t.Day)
			if rng.Float64() < risk {
				ld.Seized = t.Day
				w.Stats.ExportsSeized++
				t.Emit(events.ExportSeized{Day: t.Day, Lane: ld.Lane, Name: name, Mode: mode, City: city, Product: ld.Product, Units: ld.Units, Cost: ld.Cost, Watched: watched})
			} else {
				ld.Paid = ld.Revenue()
				w.Player.DirtyCash += ld.Paid
				w.Stats.TotalRevenue += ld.Paid
				w.Stats.ExportUnits += ld.Units
				w.Stats.ExportCash += ld.Paid
				t.Emit(events.ExportLanded{Day: t.Day, Lane: ld.Lane, Name: name, Mode: mode, City: city, Product: ld.Product, Units: ld.Units, Cost: ld.Cost, Revenue: ld.Paid})
			}
			ex.Record = append(ex.Record, ld)
		}
		ex.Loads = kept
		if len(ex.Loads) == 0 {
			ex.Loads = nil
		}
	}
	// The glut eases by each lane's recover a day; a key the file no
	// longer has a lane for goes.
	if len(ex.Glut) > 0 {
		eased := map[string]float64{}
		for _, l := range s.exports.Lanes {
			for _, p := range l.Products {
				k := game.GlutKey(l.ID, p)
				if g := ex.Glut[k] - l.Recover; g > 1e-9 {
					eased[k] = g
				}
			}
		}
		ex.Glut = eased
		if len(ex.Glut) == 0 {
			ex.Glut = nil
		}
	}
	// The record keeps record_days.
	if len(ex.Record) > 0 {
		rec := ex.Record[:0]
		for _, ld := range ex.Record {
			if max(ld.Lands, ld.Seized)+tun.RecordDays >= t.Day {
				rec = append(rec, ld)
			}
		}
		ex.Record = rec
		if len(ex.Record) == 0 {
			ex.Record = nil
		}
	}
	// Tonight's loads, lane by lane in file order.
	for _, l := range s.exports.Lanes {
		units, cost := s.LoadTonight(w, l, s.Budget(w))
		if units <= 0 {
			continue
		}
		o := w.ExportOrder(l.ID)
		price := s.ExportPrice(w, l, o.Product)
		w.Player.DirtyCash -= cost
		ex.NextID++
		ld := game.ExportLoad{ID: ex.NextID, Lane: l.ID, Product: o.Product, Units: units, Cost: cost, Price: price, Left: t.Day, Lands: t.Day + l.Days}
		ex.Loads = append(ex.Loads, ld)
		if l.Glut > 0 {
			if ex.Glut == nil {
				ex.Glut = map[string]float64{}
			}
			k := game.GlutKey(l.ID, o.Product)
			ex.Glut[k] = math.Min(1, ex.Glut[k]+l.Glut*float64(units)/float64(l.Capacity))
		}
		w.Stats.ExportLoads++
		w.Stats.ExportCost += cost
		t.Emit(events.ExportShipped{Day: t.Day, Lane: l.ID, Name: l.Name, Mode: l.Mode, City: l.City, Product: o.Product, Units: units, Cost: cost, Price: price, Lands: ld.Lands})
	}
}
