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
	ErrNoCrew         = errors.New("nobody on the payroll to ask")
	ErrInvestigating  = errors.New("somebody is already asking around tonight")
	ErrNoCity         = errors.New("no such city")
	ErrNoRoute        = errors.New("no such route")
	ErrRouteBusy      = errors.New("a shipment already left on that route today")
	ErrNoWholesale    = errors.New("nobody sells by the lot here")
)

// WholesaleOffer is how the wholesale supplier sells, handed to
// BuyWholesale by the caller from routes.toml so the world never needs the
// config: lots of Lot units at Mul of the street supplier's price, once
// peak cash has reached UnlockCash.
type WholesaleOffer struct {
	Lot        int
	Mul        float64
	UnlockCash int
}

// Locked reports whether the offer is still gated behind peak cash.
func (o WholesaleOffer) Locked(w *World) bool { return w.Stats.PeakCash < o.UnlockCash }

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

// SupplierQuote is what qty units would cost right now from the supplier
// where the player is, before any pressure the purchase itself adds to the
// supplier price.
func (w *World) SupplierQuote(product string, qty int) (int, error) {
	m := w.Product(w.Player.Location, product)
	if m == nil {
		return 0, ErrUnknownProduct
	}
	return int(math.Ceil(m.SupplierPrice * float64(qty))), nil
}

// Free is how many more units the stash in a city can take from the
// supplier.
func (w *World) Free(city string) int { return w.Capacity(city) - w.Player.StockIn(city) }

// Buy purchases qty units from the supplier in the city the player is in,
// with dirty cash, into the stash there. It applies immediately and nudges
// the supplier price up for the rest of the day.
func (w *World) Buy(product string, qty int, pricePressure float64) (Purchase, error) {
	return w.buy(product, qty, 1, pricePressure, false)
}

// BuyWholesale purchases lots of the offer's lot size from the wholesale
// supplier in the city the player is in, at the offer's fraction of the
// street supplier's price. The city must sell by the lot and the offer
// must be unlocked; otherwise it is a Buy. A lot is delivered to the
// dock, not to the stash: it is not held to the city's capacity, and
// only the road out limits how much of it moves.
func (w *World) BuyWholesale(product string, lots int, o WholesaleOffer, pricePressure float64) (Purchase, error) {
	if w.Over != nil {
		return Purchase{}, ErrGameOver
	}
	if c := w.Here(); c == nil || !c.Wholesale {
		return Purchase{}, ErrNoWholesale
	}
	if o.Locked(w) {
		return Purchase{}, fmt.Errorf("the wholesaler will not deal with you until you have moved $%d", o.UnlockCash)
	}
	if lots <= 0 || o.Lot <= 0 {
		return Purchase{}, ErrBadQuantity
	}
	return w.buy(product, lots*o.Lot, o.Mul, pricePressure, true)
}

func (w *World) buy(product string, qty int, mul, pricePressure float64, wholesale bool) (Purchase, error) {
	if w.Over != nil {
		return Purchase{}, ErrGameOver
	}
	city := w.Player.Location
	m := w.Product(city, product)
	if m == nil {
		return Purchase{}, ErrUnknownProduct
	}
	if qty <= 0 {
		return Purchase{}, ErrBadQuantity
	}
	unit := m.SupplierPrice * mul
	cost := int(math.Ceil(unit * float64(qty)))
	if cost > w.Player.DirtyCash {
		return Purchase{}, fmt.Errorf("need $%d, only have $%d dirty", cost, w.Player.DirtyCash)
	}
	if free := w.Free(city); !wholesale && qty > free {
		return Purchase{}, fmt.Errorf("can only hold %d more units in %s", free, w.CityName(city))
	}
	p := Purchase{City: city, Product: product, Qty: qty, UnitPrice: unit, Cost: cost, Wholesale: wholesale}
	w.Player.DirtyCash -= cost
	w.Stash(city)[product] += qty
	m.BoughtToday += qty
	if demand := w.Demand(city, product); demand > 0 {
		m.SupplierPrice *= 1 + pricePressure*float64(qty)/demand
	}
	w.Buys = append(w.Buys, p)
	return p, nil
}

// PlaceSell queues a sell order in a city for resolution at end of day by
// whoever works corners there. One order per product per city; placing
// again replaces the previous one.
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
	if have := w.Stock(city, product); qty > have {
		return fmt.Errorf("only %d %s in %s", have, w.ProductName(product), w.CityName(city))
	}
	w.Orders[OrderKey(city, product)] = SellOrder{City: city, Product: product, Qty: qty, Dial: dial}
	return nil
}

// Order returns the pending order for a product in a city.
func (w *World) Order(city, product string) (SellOrder, bool) {
	o, ok := w.Orders[OrderKey(city, product)]
	return o, ok
}

// CancelSell removes a pending order.
func (w *World) CancelSell(city, product string) { delete(w.Orders, OrderKey(city, product)) }

// Travel moves the player to another city at once. Product stays where it
// is: only a shipment moves it. Whatever corner you stood on is left with
// nobody on it and drifts unless a runner takes it.
func (w *World) Travel(city string) error {
	if w.Over != nil {
		return ErrGameOver
	}
	if w.Cities[city] == nil {
		return ErrNoCity
	}
	if city == w.Player.Location {
		return nil
	}
	w.Recall(You)
	w.Player.Location = city
	return nil
}

// RouteOffer is a route as the logistics config prices it for one dial,
// handed to Ship by the caller so the world never needs the config.
type RouteOffer struct {
	ID       string
	Name     string
	Mode     string
	From     string
	To       string
	Days     int // days in transit at this dial
	Capacity int
	Cost     int // dirty cash per unit
	Dial     events.Ship
}

// Ship sends units of a product from one city's stash to another over a
// route, paying the cost up front in dirty cash. The units leave the
// source stash now and land in the destination's when the logistics sim
// brings them in, unless it is seized first. One shipment per route per
// day, never more than the route carries.
func (w *World) Ship(r RouteOffer, from, to, product string, units int) (Shipment, error) {
	if w.Over != nil {
		return Shipment{}, ErrGameOver
	}
	if w.Cities[from] == nil || w.Cities[to] == nil || from == to {
		return Shipment{}, ErrNoCity
	}
	if r.ID == "" || !((r.From == from && r.To == to) || (r.From == to && r.To == from)) {
		return Shipment{}, ErrNoRoute
	}
	if w.Product(from, product) == nil {
		return Shipment{}, ErrUnknownProduct
	}
	if units <= 0 {
		return Shipment{}, ErrBadQuantity
	}
	if units > r.Capacity {
		return Shipment{}, fmt.Errorf("%s carries %d units at most", r.Name, r.Capacity)
	}
	if have := w.Stock(from, product); units > have {
		return Shipment{}, fmt.Errorf("only %d %s in %s", have, w.ProductName(product), w.CityName(from))
	}
	for _, s := range w.Shipments {
		if s.Route == r.ID && s.Sent == w.Day {
			return Shipment{}, ErrRouteBusy
		}
	}
	cost := units * r.Cost
	if cost > w.Player.DirtyCash {
		return Shipment{}, fmt.Errorf("sending it costs $%d, only have $%d dirty", cost, w.Player.DirtyCash)
	}
	w.Player.DirtyCash -= cost
	w.Stash(from)[product] -= units
	w.Logistics.NextID++
	s := Shipment{
		ID: w.Logistics.NextID, Route: r.ID, Mode: r.Mode, From: from, To: to,
		Product: product, Units: units, Dial: r.Dial, Sent: w.Day, Arrives: w.Day + max(1, r.Days), Cost: cost,
	}
	w.Shipments = append(w.Shipments, s)
	w.Stats.Shipments++
	w.Stats.Shipped += units
	return s, nil
}

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

// spend takes cost from dirty cash first and clean cash for the rest, the
// way somebody paid off the books is paid. It reports whether there was
// enough between the two.
func (w *World) spend(cost int) bool {
	if cost > w.Cash() {
		return false
	}
	dirty := min(cost, w.Player.DirtyCash)
	w.Player.DirtyCash -= dirty
	w.Player.CleanCash -= cost - dirty
	return true
}

// Investigate pays cost to have the crew looked into tonight: the crew sim
// rolls whether it names the informant, if there is one, and everyone's
// loyalty suffers when it names nobody. One a day.
func (w *World) Investigate(cost int) error {
	if w.Over != nil {
		return ErrGameOver
	}
	if len(w.Crew.Members) == 0 {
		return ErrNoCrew
	}
	if w.Investigation != nil {
		return ErrInvestigating
	}
	if !w.spend(cost) {
		return fmt.Errorf("need $%d, only have $%d", cost, w.Cash())
	}
	w.Investigation = &InvestigationOrder{Cost: cost}
	return nil
}

// PayOff hands the member with id cost in cash for loyalty points, at
// once. It buys loyalty, not silence: an informant stays one.
func (w *World) PayOff(id, cost int, loyalty float64) (CrewMember, error) {
	if w.Over != nil {
		return CrewMember{}, ErrGameOver
	}
	m := w.Crew.Member(id)
	if m == nil {
		return CrewMember{}, ErrNoMember
	}
	if !w.spend(cost) {
		return CrewMember{}, fmt.Errorf("%s wants $%d, only have $%d", m.Name, cost, w.Cash())
	}
	m.Loyalty = math.Min(100, m.Loyalty+loyalty)
	w.Crew.PaidOffToday = append(w.Crew.PaidOffToday, Payoff{ID: m.ID, Name: m.Name, Cost: cost})
	return *m, nil
}
