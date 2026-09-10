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
)

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
	if free := w.Player.CarryLimit - w.Player.TotalStock(); qty > free {
		return Purchase{}, fmt.Errorf("can only carry %d more units", free)
	}
	p := Purchase{Product: product, Qty: qty, UnitPrice: m.SupplierPrice, Cost: cost}
	w.Player.DirtyCash -= cost
	w.Player.Stock[product] += qty
	m.BoughtToday += qty
	if m.Demand > 0 {
		m.SupplierPrice *= 1 + pricePressure*float64(qty)/m.Demand
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
