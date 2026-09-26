package game

import (
	"fmt"
	"math"

	"github.com/theclifmeister/kingpin/internal/events"
)

// Free is how many more units the stash in a city, the street and the
// houses, can take from the supplier.
func (w *World) Free(city string) int { return w.Capacity(city) - w.StockIn(city) }

// SupplyKey is how Supply is keyed: one contract per product per city,
// like Orders.
func SupplyKey(city, product string) string { return OrderKey(city, product) }

// Supplied returns the supply contract for a product in a city, if one
// stands.
func (w *World) Supplied(city, product string) (SupplyContract, bool) {
	c, ok := w.Supply[SupplyKey(city, product)]
	return c, ok
}

// SetSupply sets a supply contract (#113): keep the stash in a city at
// units of a product, bought each morning from the supplier there. It is
// a persistent setting like a route target: the market sim fills it
// every day until it is cleared. A city whose supplier does not sell the
// product is refused (ErrNotSupplied); a city the player holds no
// capacity in is allowed, the contract filling as far as Free(city).
func (w *World) SetSupply(city, product string, units int) error {
	if w.Over != nil {
		return ErrGameOver
	}
	if w.Cities[city] == nil {
		return ErrNoCity
	}
	m := w.Product(city, product)
	if m == nil {
		return ErrUnknownProduct
	}
	if units <= 0 {
		return ErrBadQuantity
	}
	if m.NoSupply {
		return ErrNotSupplied
	}
	if w.Supply == nil {
		w.Supply = map[string]SupplyContract{}
	}
	w.Supply[SupplyKey(city, product)] = SupplyContract{City: city, Product: product, Units: units, Since: w.Day}
	return nil
}

// ClearSupply removes the supply contract for a product in a city, if
// one stands.
func (w *World) ClearSupply(city, product string) {
	delete(w.Supply, SupplyKey(city, product))
	if len(w.Supply) == 0 {
		w.Supply = nil
	}
}

// SupplyDue is what the supply contract standing for a product in a
// city (StandingSupply: yours, else the lieutenant's) will try to buy
// in the morning: the level less the stash there and what is on the
// road to it; zero with no contract. A sell order may count on it
// (PlaceSell), since the contract fills before the orders resolve.
func (w *World) SupplyDue(city, product string) int {
	c, ok := w.StandingSupply(city, product)
	if !ok {
		return 0
	}
	return max(0, c.Units-w.Stock(city, product)-w.Bound(city, product))
}

// SupplyOutlay is what the supply contracts standing (yours, and the
// lieutenant's where you set none) are expected to spend in the
// morning, estimated on the stash as it stands (#496): each one's
// SupplyDue, the room in its city shared in city and ladder order, at
// the best connect's price there and the contract markup the market
// stamps (Markup). It is dirty cash already committed: the laundering
// sim keeps it back from the wash, so a big front cannot wash the
// contracts' morning away. It reads no tuning and rolls no dice; zero
// with no contract, so a run without one keeps the till it had.
func (w *World) SupplyOutlay() int {
	if len(w.Supply) == 0 && len(w.DelegatedSupply) == 0 {
		return 0
	}
	markup := max(w.Markup, 1)
	total := 0.0
	for _, cid := range w.CityOrder {
		room := w.Free(cid)
		for _, id := range w.Products {
			due := min(w.SupplyDue(cid, id), max(0, room))
			if due <= 0 {
				continue
			}
			room -= due
			total += w.SupplierPrice(cid, id) * markup * float64(due)
		}
	}
	return int(math.Ceil(total))
}

// Road is how many units of a product are on the road to a city that a
// supply contract standing there counts against its level (#503): a
// contract buys the level less the stash less what is Bound for it, so
// a route feeding the city holds the contract back until the shipment
// lands. Zero with no contract or nothing on the road.
func (w *World) Road(city, product string) int {
	c, ok := w.StandingSupply(city, product)
	if !ok {
		return 0
	}
	return min(w.Bound(city, product), max(0, c.Units-w.Stock(city, product)))
}

// StandingSupply returns the supply contract standing for a product in
// a city: yours first (Supplied, #113), then the one the lieutenant
// who runs the city keeps (DelegatedSupply, #174). A contract you set
// wins in a delegated city, as your standing order does.
func (w *World) StandingSupply(city, product string) (SupplyContract, bool) {
	if c, ok := w.Supplied(city, product); ok {
		return c, true
	}
	return w.DelegatedSupplied(city, product)
}

// DelegatedSupplied returns the supply contract a lieutenant keeps for
// a product in a city, if the city is run and the contract stands. It
// mirrors DelegatedOrder.
func (w *World) DelegatedSupplied(city, product string) (SupplyContract, bool) {
	if w.Crew.Lieutenant(city) == nil {
		return SupplyContract{}, false
	}
	c, ok := w.DelegatedSupply[SupplyKey(city, product)]
	return c, ok
}

// DelegateSupply records a lieutenant's supply contract for a product
// in a city (#174): keep the stash there at units, bought each morning
// by the market sim where the player has set no contract of their own.
// It is the crew sim's to set, nightly, by the temper's stock_days.
func (w *World) DelegateSupply(city, product string, units int) {
	if w.DelegatedSupply == nil {
		w.DelegatedSupply = map[string]SupplyContract{}
	}
	w.DelegatedSupply[SupplyKey(city, product)] = SupplyContract{City: city, Product: product, Units: units, Since: w.Day}
}

// SuppliedToday is what the supply contracts bought this morning, all
// products and cities: the receipts and what they cost.
func (w *World) SuppliedToday() (units, cost int) {
	for _, b := range w.Today.Buys {
		if b.Contract {
			units += b.Qty
			cost += b.Cost
		}
	}
	return units, cost
}

// Bought is how many units of a product the player has bought in a city
// today for cash and still holds the receipt for: what Return can take
// back. SuppliedIn counts a contract's, Booked what went on a connect's
// book.
func (w *World) Bought(city, product string) int { return w.receipts(city, product, false, false) }

// Booked is how many units of a product the player has taken on credit
// in a city today and still holds the receipt for (#72): what
// ReturnCredit can take back.
func (w *World) Booked(city, product string) int { return w.receipts(city, product, false, true) }

// SuppliedIn is how many units of a product the supply contract bought
// in a city this morning and still holds the receipt for: what
// ReturnSupplied can take back.
func (w *World) SuppliedIn(city, product string) int { return w.receipts(city, product, true, false) }

func (w *World) receipts(city, product string, contract, credit bool) int {
	n := 0
	for _, b := range w.Today.Buys {
		if b.City == city && b.Product == product && b.Contract == contract && b.Credit == credit {
			n += b.Qty
		}
	}
	return n
}

// Return is the exact inverse of Buy, the same day (#103): qty units of a
// product bought in a city today go back to the supplier, the last buy
// first. The units come out of the stash there (refused if they are no
// longer in it), the cost is refunded at the price paid, BoughtToday
// comes down and the supplier price goes back to what the buy found it
// at; a part of a buy is returned in proportion, so returning the whole
// of it leaves cash, stash, BoughtToday and the price exactly as they
// were. Buys is per-day scratch, so once the day ends there is nothing
// to return. It reports what was refunded. It takes back the buys made
// by hand; ReturnSupplied takes back a contract's.
func (w *World) Return(city, product string, qty int) (int, error) {
	return w.giveBack(city, product, qty, false, false)
}

// ReturnCredit is Return for what went on a connect's book today (#72):
// the units go back and the debt comes down by what they were put on
// it for; nothing comes into the till, so it reports zero.
func (w *World) ReturnCredit(city, product string, qty int) (int, error) {
	return w.giveBack(city, product, qty, false, true)
}

// ReturnSupplied is Return for what the supply contract bought this
// morning (#113): the units go back at the price the contract paid.
// The supplier price is left alone, the market having reset it since.
func (w *World) ReturnSupplied(city, product string, qty int) (int, error) {
	return w.giveBack(city, product, qty, true, false)
}

func (w *World) giveBack(city, product string, qty int, contract, credit bool) (int, error) {
	if w.Over != nil {
		return 0, ErrGameOver
	}
	if qty <= 0 {
		return 0, ErrBadQuantity
	}
	m := w.Product(city, product)
	if m == nil {
		return 0, ErrUnknownProduct
	}
	if bought := w.receipts(city, product, contract, credit); bought == 0 {
		return 0, ErrNothingBought
	} else if qty > bought {
		return 0, fmt.Errorf("only %d %s bought in %s today", bought, w.ProductName(product), w.CityName(city))
	}
	if have := w.Stock(city, product); qty > have {
		return 0, fmt.Errorf("%w: only %d %s left in %s", ErrReturnGone, have, w.ProductName(product), w.CityName(city))
	}
	refund := 0
	left := qty
	for i := len(w.Today.Buys) - 1; i >= 0 && left > 0; i-- {
		b := &w.Today.Buys[i]
		if b.City != city || b.Product != product || b.Contract != contract || b.Credit != credit {
			continue
		}
		back := min(left, b.Qty)
		keep := b.Qty - back
		// What Buy would have charged and nudged for the units kept, so
		// a part of a buy is returned in proportion and the whole of one
		// exactly. The nudge is read back off the connect's price rather
		// than the pressure, which the world does not hold; a credit
		// buy's refund comes off the debt, never into the till.
		cost := int(math.Ceil(b.UnitPrice * float64(keep)))
		s := w.Supplier(b.Supplier)
		if s != nil {
			if b.Prior > 0 {
				s.Price[product] = b.Prior * (1 + (s.Price[product]/b.Prior-1)*float64(keep)/float64(b.Qty))
			}
			s.BoughtToday -= back
			s.Bought -= back
			if s.Lot > 0 {
				s.Lots -= float64(back) / float64(s.Lot)
			}
		}
		if b.Credit && s != nil {
			s.Debt -= b.Cost - cost
			if s.Debt <= 0 {
				s.Debt, s.DebtDue, s.Extended = 0, 0, false
			}
			w.Stats.Credit -= b.Cost - cost
		} else {
			refund += b.Cost - cost
		}
		b.Qty, b.Cost = keep, cost
		left -= back
	}
	kept := w.Today.Buys[:0]
	for _, b := range w.Today.Buys {
		if b.Qty > 0 {
			kept = append(kept, b)
		}
	}
	w.Today.Buys = kept
	if len(w.Today.Buys) == 0 {
		w.Today.Buys = nil
	}
	w.Player.DirtyCash += refund
	w.TakeStock(city, product, qty)
	m.BoughtToday -= qty
	w.refreshSupplierPrice(city, product)
	return refund, nil
}

// PlaceSell queues a sell order in a city for resolution at end of day by
// whoever works corners there. One order per product per city; placing
// again replaces the previous one. An order may be for what the stash
// holds plus what the supply contract there brings in the morning
// (SupplyDue, #113): the contract fills before the orders resolve, and
// the market sells what is there either way.
func (w *World) PlaceSell(city, product string, qty int, dial events.Dial) error {
	if w.Over != nil {
		return ErrGameOver
	}
	if w.Cities[city] == nil {
		return ErrNoCity
	}
	if w.Product(city, product) == nil {
		return ErrUnknownProduct
	}
	if qty <= 0 {
		return ErrBadQuantity
	}
	if have := w.Stock(city, product) + w.SupplyDue(city, product); qty > have {
		return fmt.Errorf("only %d %s in %s", have, w.ProductName(product), w.CityName(city))
	}
	w.Today.Orders[OrderKey(city, product)] = SellOrder{City: city, Product: product, Qty: qty, Dial: dial}
	return nil
}

// Order returns the pending order for a product in a city.
func (w *World) Order(city, product string) (SellOrder, bool) {
	o, ok := w.Today.Orders[OrderKey(city, product)]
	return o, ok
}

// CancelSell removes a pending order.
func (w *World) CancelSell(city, product string) { delete(w.Today.Orders, OrderKey(city, product)) }

// AllUnits is PlaceStanding's quantity for a standing order that sells
// the whole stash every night (#503): the sell dialog's blank, "the
// most", kept as the most rather than as the number it was the day it
// was set (a playtest's froze at 16, and the report said "only 15
// stashed" every night after).
const AllUnits = -1

// Landing is how many units of a product land in a city tonight, after
// the sales (#503): the shipments on the road to it due by tomorrow's
// day on a route not shut, none seized, and the chemist's batches ready
// there by then. A standing order may be sized for them: it sells them
// from the night after.
func (w *World) Landing(city, product string) int {
	day, n := w.Day+1, 0
	for _, s := range w.Shipments {
		if s.To == city && s.Product == product && s.Arrives <= day && !w.Route(s.Route).Closed(day) {
			n += s.Units
		}
	}
	for _, k := range w.Crew.Cooks {
		if k.City == city && k.Product == product && k.Ready <= day {
			n += k.Units
		}
	}
	return n
}

// PlaceStanding sets a standing sell order (#114): the same units at
// the same dial every night until it is cancelled, resolved by the
// market sim exactly as a fresh order would be, at the crew's cut,
// wherever you placed no order of your own that day. It is checked as
// PlaceSell checks an order (a city, a product, a quantity the stash
// and the contract can cover), and may be sized over the stash for
// what lands there tonight (#503, Landing: goods arriving by the road
// or the chemist, sold from the night after); qty AllUnits stands for
// the whole stash every night (SellOrder.All). It is a persistent
// setting like a supply contract: the clock never clears it. One per
// product per city; placing again replaces the previous one.
func (w *World) PlaceStanding(city, product string, qty int, dial events.Dial) error {
	if w.Over != nil {
		return ErrGameOver
	}
	if w.Cities[city] == nil {
		return ErrNoCity
	}
	if w.Product(city, product) == nil {
		return ErrUnknownProduct
	}
	have := w.Stock(city, product) + w.SupplyDue(city, product) + w.Landing(city, product)
	all := qty == AllUnits
	if all {
		qty = have
	}
	if qty <= 0 {
		if all {
			return fmt.Errorf("only %d %s in %s", have, w.ProductName(product), w.CityName(city))
		}
		return ErrBadQuantity
	}
	if qty > have {
		return fmt.Errorf("only %d %s in %s", have, w.ProductName(product), w.CityName(city))
	}
	if w.Standing == nil {
		w.Standing = map[string]SellOrder{}
	}
	w.Standing[OrderKey(city, product)] = SellOrder{City: city, Product: product, Qty: qty, Dial: dial, All: all}
	return nil
}

// CancelStanding removes your standing order for a product in a city,
// if one stands.
func (w *World) CancelStanding(city, product string) {
	delete(w.Standing, OrderKey(city, product))
	if len(w.Standing) == 0 {
		w.Standing = nil
	}
}

// YourStanding returns your standing order for a product in a city
// (#114), if one stands: the one you set, never the lieutenant's.
func (w *World) YourStanding(city, product string) (SellOrder, bool) {
	o, ok := w.Standing[OrderKey(city, product)]
	return o, ok
}

// StandingOrder returns the order standing for a product in a city
// where you placed none today: yours first (#114, YourStanding), then
// the one the lieutenant who runs the city has standing (DelegatedOrder).
// A standing order you set wins in a delegated city, as your fresh
// order does.
func (w *World) StandingOrder(city, product string) (SellOrder, bool) {
	if o, ok := w.YourStanding(city, product); ok {
		return o, true
	}
	return w.DelegatedOrder(city, product)
}

// DelegatedOrder returns the order a lieutenant has standing for a
// product in a city, if the city is run and the order stands.
func (w *World) DelegatedOrder(city, product string) (SellOrder, bool) {
	if w.Crew.Lieutenant(city) == nil {
		return SellOrder{}, false
	}
	o, ok := w.Delegated[OrderKey(city, product)]
	return o, ok
}

// Delegate records a lieutenant's standing order for a product in a city.
// It is the crew sim's to place, and it resolves the next day unless the
// player places their own.
func (w *World) Delegate(city, product string, qty int, dial events.Dial) {
	if w.Delegated == nil {
		w.Delegated = map[string]SellOrder{}
	}
	w.Delegated[OrderKey(city, product)] = SellOrder{City: city, Product: product, Qty: qty, Dial: dial}
}

// DropStanding forgets every standing order and supply contract of the
// lieutenant's in a city: the crew sim's, when the lieutenant who
// placed them is gone (Unassign, Fire, a move to another city, a walk).
func (w *World) DropStanding(city string) {
	for k, o := range w.Delegated {
		if o.City == city {
			delete(w.Delegated, k)
		}
	}
	for k, c := range w.DelegatedSupply {
		if c.City == city {
			delete(w.DelegatedSupply, k)
		}
	}
	if len(w.DelegatedSupply) == 0 {
		w.DelegatedSupply = nil
	}
}
