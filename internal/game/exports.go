package game

import "errors"

// ErrNoLane is an export order for a lane with no id.
var ErrNoLane = errors.New("no such lane")

// ExportsState is the export lanes as they stand (#391): the standing
// order on each lane, the loads at sea or in the air, the glut each has
// put on the price abroad, and the record of loads landed or seized.
// The zero value is a run before the lanes: nothing ordered, nothing
// out, so no schema bump. The logistics sim writes it; the player's
// action is SetExport.
type ExportsState struct {
	NextID int
	Orders map[string]ExportOrder // lane id -> the standing order
	Loads  []ExportLoad           // out, in the order they left
	Record []ExportLoad           // landed or seized, newest last, kept record_days
	Glut   map[string]float64     // lane id + ":" + product -> the fraction off the lane's price
}

// ExportOrder is a lane's standing order: so many units of a product a
// night, as many as the lane carries and the till pays for. Zero units
// is the lane off.
type ExportOrder struct {
	Product string
	Units   int
}

// On reports whether the order ships anything.
func (o ExportOrder) On() bool { return o.Product != "" && o.Units > 0 }

// ExportLoad is one night's load on a lane: bought off the book for
// Cost, gone on Left, due on Lands, paid Price a unit on landing (the
// rate locked the night it left). Paid is what it paid on landing and
// Seized the day it was taken, for the record.
type ExportLoad struct {
	ID      int
	Lane    string
	Product string
	Units   int
	Cost    int
	Price   float64
	Left    int
	Lands   int
	Paid    int
	Seized  int
}

// Revenue is what the load pays on landing.
func (l ExportLoad) Revenue() int { return int(float64(l.Units)*l.Price + 0.5) }

// GlutKey is the key a lane's glut on a product is kept under.
func GlutKey(lane, product string) string { return lane + ":" + product }

// SetExport sets a lane's standing order: units of a product a night,
// shipped from tonight; zero units (or no product) turns the lane off.
// It applies at once; the logistics sim loads what the lane carries and
// the till pays for, and ignores a lane that is not open.
func (w *World) SetExport(lane, product string, units int) error {
	if w.Over != nil {
		return ErrGameOver
	}
	if lane == "" {
		return ErrNoLane
	}
	if units < 0 {
		return ErrBadQuantity
	}
	if units == 0 || product == "" {
		delete(w.Exports.Orders, lane)
		if len(w.Exports.Orders) == 0 {
			w.Exports.Orders = nil
		}
		return nil
	}
	if w.Home() == nil || w.Home().Market[product] == nil {
		return ErrUnknownProduct
	}
	if w.Exports.Orders == nil {
		w.Exports.Orders = map[string]ExportOrder{}
	}
	w.Exports.Orders[lane] = ExportOrder{Product: product, Units: units}
	return nil
}

// ExportOrder is the standing order on a lane, zero when it is off.
func (w *World) ExportOrder(lane string) ExportOrder { return w.Exports.Orders[lane] }

// ExportsOut lists the loads out on a lane, oldest first.
func (w *World) ExportsOut(lane string) []ExportLoad {
	var out []ExportLoad
	for _, l := range w.Exports.Loads {
		if l.Lane == lane {
			out = append(out, l)
		}
	}
	return out
}

// Glut is the fraction a lane's price for a product is down by.
func (w *World) Glut(lane, product string) float64 { return w.Exports.Glut[GlutKey(lane, product)] }
