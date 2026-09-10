package game

import (
	"errors"
	"fmt"
	"math"

	"github.com/theclifmeister/kingpin/internal/events"
)

var (
	ErrGameOver       = errors.New("the run is over")
	ErrUnknownProduct = errors.New("unknown product")
	ErrBadQuantity    = errors.New("quantity must be positive")
	ErrNoCandidate    = errors.New("nobody by that name is looking for work")
	ErrNoMember       = errors.New("nobody by that name works for you")
	ErrCrewFull       = errors.New("the crew is as big as you can manage")
	ErrNoFront        = errors.New("no such front")
	ErrFrontOwned     = errors.New("you already own that front")
)

// FrontOffer is a front as the laundering config prices it, handed to
// BuyFront by the caller so the world never needs the config.
type FrontOffer struct {
	ID         string
	Name       string
	Cost       int // dirty cash
	Throughput int // dirty cash washed per day at the normal dial
	Upkeep     int // clean cash per day
	AuditRisk  float64
	UnlockCash int // peak cash that puts it on offer
}

// Locked reports whether the offer is still gated behind peak cash.
func (o FrontOffer) Locked(w *World) bool { return w.Stats.PeakCash < o.UnlockCash }

// SupplierQuote is what qty units would cost right now, before any pressure
// the purchase itself adds to the supplier price.
func (w *World) SupplierQuote(product string, qty int) (int, error) {
	m := w.Market[product]
	if m == nil {
		return 0, ErrUnknownProduct
	}
	return int(math.Ceil(m.SupplierPrice * float64(qty))), nil
}

// Buy purchases qty units from the supplier with dirty cash. It applies
// immediately and nudges the supplier price up for the rest of the day.
func (w *World) Buy(product string, qty int, pricePressure float64) (Purchase, error) {
	if w.Over != nil {
		return Purchase{}, ErrGameOver
	}
	m := w.Market[product]
	if m == nil {
		return Purchase{}, ErrUnknownProduct
	}
	if qty <= 0 {
		return Purchase{}, ErrBadQuantity
	}
	cost, _ := w.SupplierQuote(product, qty)
	if cost > w.Player.DirtyCash {
		return Purchase{}, fmt.Errorf("need $%d, only have $%d dirty", cost, w.Player.DirtyCash)
	}
	if free := w.Capacity() - w.Player.TotalStock(); qty > free {
		return Purchase{}, fmt.Errorf("can only carry %d more units", free)
	}
	p := Purchase{Product: product, Qty: qty, UnitPrice: m.SupplierPrice, Cost: cost}
	w.Player.DirtyCash -= cost
	w.Player.Stock[product] += qty
	m.BoughtToday += qty
	if demand := w.Demand(product); demand > 0 {
		m.SupplierPrice *= 1 + pricePressure*float64(qty)/demand
	}
	w.Buys = append(w.Buys, p)
	return p, nil
}

// PlaceSell queues a sell order for resolution at end of day. One order per
// product; placing again replaces the previous one. Stock is checked against
// what is not already committed to other orders.
func (w *World) PlaceSell(product string, qty int, dial events.Dial) error {
	if w.Over != nil {
		return ErrGameOver
	}
	if w.Market[product] == nil {
		return ErrUnknownProduct
	}
	if qty <= 0 {
		return ErrBadQuantity
	}
	if qty > w.Player.Stock[product] {
		return fmt.Errorf("only %d %s in stock", w.Player.Stock[product], w.ProductName(product))
	}
	w.Orders[product] = SellOrder{Product: product, Qty: qty, Dial: dial}
	return nil
}

// CancelSell removes a pending order.
func (w *World) CancelSell(product string) { delete(w.Orders, product) }

// SetLieLow toggles lying low for the day. Lying low cancels all orders.
func (w *World) SetLieLow(on bool) {
	w.LieLow = on
	if on {
		w.Orders = map[string]SellOrder{}
	}
}

// Hire signs the candidate with id, paying the fee in dirty cash. maxCrew
// caps the roster. The crew sim reports the signing at end of day.
func (w *World) Hire(id, maxCrew int) (CrewMember, error) {
	if w.Over != nil {
		return CrewMember{}, ErrGameOver
	}
	idx := -1
	for i, c := range w.Crew.Candidates {
		if c.ID == id {
			idx = i
		}
	}
	if idx < 0 {
		return CrewMember{}, ErrNoCandidate
	}
	if len(w.Crew.Members) >= maxCrew {
		return CrewMember{}, ErrCrewFull
	}
	c := w.Crew.Candidates[idx]
	if c.Fee > w.Player.DirtyCash {
		return CrewMember{}, fmt.Errorf("%s wants $%d up front, only have $%d dirty", c.Name, c.Fee, w.Player.DirtyCash)
	}
	w.Player.DirtyCash -= c.Fee
	c.Hired = w.Day
	w.Crew.Candidates = append(w.Crew.Candidates[:idx], w.Crew.Candidates[idx+1:]...)
	w.Crew.Members = append(w.Crew.Members, c)
	w.Crew.HiredToday = append(w.Crew.HiredToday, c)
	return c, nil
}

// Fire removes the member with id and pulls them off their corner. The
// rest of the crew take it badly when the crew sim steps.
func (w *World) Fire(id int) (CrewMember, error) {
	if w.Over != nil {
		return CrewMember{}, ErrGameOver
	}
	for i, m := range w.Crew.Members {
		if m.ID == id {
			w.Recall(id)
			w.Crew.Members = append(w.Crew.Members[:i], w.Crew.Members[i+1:]...)
			w.Crew.FiredToday = append(w.Crew.FiredToday, m)
			return m, nil
		}
	}
	return CrewMember{}, ErrNoMember
}

// SetPay sets the pay dial for the whole crew. It persists until changed.
func (w *World) SetPay(p events.Pay) { w.Crew.Pay = p }

// BuyFront buys a front with dirty cash. It applies immediately: the place
// opens tomorrow and the laundering sim reports the purchase at end of day.
// The offer must be on offer (peak cash past its unlock), not already
// owned, and affordable.
func (w *World) BuyFront(o FrontOffer) (Front, error) {
	if w.Over != nil {
		return Front{}, ErrGameOver
	}
	if o.ID == "" {
		return Front{}, ErrNoFront
	}
	if w.Front(o.ID) != nil {
		return Front{}, ErrFrontOwned
	}
	if o.Locked(w) {
		return Front{}, fmt.Errorf("nobody will sell you %s until you have moved $%d", o.Name, o.UnlockCash)
	}
	if o.Cost > w.Player.DirtyCash {
		return Front{}, fmt.Errorf("%s costs $%d, only have $%d dirty", o.Name, o.Cost, w.Player.DirtyCash)
	}
	w.Player.DirtyCash -= o.Cost
	f := Front{ID: o.ID, Name: o.Name, Cost: o.Cost, Bought: w.Day}
	w.Fronts = append(w.Fronts, f)
	return f, nil
}

// SetLaunderDial sets the launder dial for every front. It persists until
// changed.
func (w *World) SetLaunderDial(d events.Launder) { w.Laundering.Dial = d }
