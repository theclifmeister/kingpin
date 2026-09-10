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
	ErrBadDial        = errors.New("no such dial position")
)

// WholesaleOffer is how the wholesale supplier sells, handed to Restock
// by the logistics sim from routes.toml so the world never needs the
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
	cost := int(math.Ceil(m.SupplierPrice * float64(qty)))
	if cost > w.Player.DirtyCash {
		return Purchase{}, fmt.Errorf("need $%d, only have $%d dirty", cost, w.Player.DirtyCash)
	}
	if free := w.Free(city); qty > free {
		return Purchase{}, fmt.Errorf("can only hold %d more units in %s", free, w.CityName(city))
	}
	p := Purchase{City: city, Product: product, Qty: qty, UnitPrice: m.SupplierPrice, Cost: cost}
	w.Player.DirtyCash -= cost
	w.Stash(city)[product] += qty
	m.BoughtToday += qty
	if demand := w.Demand(city, product); demand > 0 {
		m.SupplierPrice *= 1 + pricePressure*float64(qty)/demand
	}
	w.Buys = append(w.Buys, p)
	return p, nil
}

// Restock is the logistics sim buying lots from the wholesale supplier in
// a city, wherever the player is, to feed a route (#61): it is not a
// player action and not reported through Buys (the sim reports it as
// WholesaleBought). The city must sell by the lot and the offer must be
// open. The lots go into the stash there regardless of its capacity: the
// road takes them, and the odd lot's remainder waits for tomorrow's
// shipment.
func (w *World) Restock(city, product string, lots int, o WholesaleOffer, pricePressure float64) (Purchase, error) {
	c := w.Cities[city]
	if c == nil {
		return Purchase{}, ErrNoCity
	}
	if !c.Wholesale || o.Locked(w) {
		return Purchase{}, ErrNoRoute
	}
	m := c.Market[product]
	if m == nil {
		return Purchase{}, ErrUnknownProduct
	}
	if lots <= 0 || o.Lot <= 0 {
		return Purchase{}, ErrBadQuantity
	}
	qty := lots * o.Lot
	unit := m.SupplierPrice * o.Mul
	cost := int(math.Ceil(unit * float64(qty)))
	if cost > w.Player.DirtyCash {
		return Purchase{}, fmt.Errorf("need $%d, only have $%d dirty", cost, w.Player.DirtyCash)
	}
	w.Player.DirtyCash -= cost
	w.Stash(city)[product] += qty
	m.BoughtToday += qty
	if demand := w.Demand(city, product); demand > 0 {
		m.SupplierPrice *= 1 + pricePressure*float64(qty)/demand
	}
	return Purchase{City: city, Product: product, Qty: qty, UnitPrice: unit, Cost: cost}, nil
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
// is: only the road moves it. Whatever corner you stood on is left with
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

// RouteSetting is the player's standing instruction for one route (#61):
// the dial it runs at and the stock the far city is kept at, per product.
// The logistics sim reads it every day; the zero value is a route that is
// off, so a save from before the dial replays as it did.
type RouteSetting struct {
	Dial   events.RouteDial
	Target map[string]int // product id -> units the route's destination is kept stocked to
}

// Route returns the setting for a route, off with no targets if it has
// never been set.
func (w *World) Route(id string) RouteSetting {
	if rs, ok := w.Routes[id]; ok {
		return rs
	}
	return RouteSetting{}
}

// SetRoute turns a route's dial. It is a persistent setting like the pay
// dial, not per-day scratch: the logistics sim runs the route at it every
// day until it is turned again. The targets are kept when it is turned
// off, so turning it back on picks up where it left off.
func (w *World) SetRoute(id string, d events.RouteDial) error {
	if w.Over != nil {
		return ErrGameOver
	}
	if id == "" {
		return ErrNoRoute
	}
	if d < events.RouteOff || d > events.RouteFast {
		return ErrBadDial
	}
	if w.Routes == nil {
		w.Routes = map[string]RouteSetting{}
	}
	rs := w.Routes[id]
	rs.Dial = d
	w.Routes[id] = rs
	return nil
}

// SetRouteTarget sets the stock a route keeps its destination at for a
// product; zero clears it. The route sends the shortfall against it,
// buying by the lot at the source where a wholesaler deals.
func (w *World) SetRouteTarget(id, product string, units int) error {
	if w.Over != nil {
		return ErrGameOver
	}
	if id == "" {
		return ErrNoRoute
	}
	if w.Home() == nil || w.Home().Market[product] == nil {
		return ErrUnknownProduct
	}
	if units < 0 {
		return ErrBadQuantity
	}
	if w.Routes == nil {
		w.Routes = map[string]RouteSetting{}
	}
	rs := w.Routes[id]
	if units == 0 {
		delete(rs.Target, product)
		if len(rs.Target) == 0 {
			rs.Target = nil
		}
	} else {
		if rs.Target == nil {
			rs.Target = map[string]int{}
		}
		rs.Target[product] = units
	}
	w.Routes[id] = rs
	return nil
}

// Send is the logistics sim putting a shipment on the road (#61): the
// units leave the source stash and the fare leaves dirty cash now, and
// the sim lands or loses it. It is not a player action; the route dial
// is. The sim fills in the route, the days and the fare; the world gives
// it an id and keeps the count.
func (w *World) Send(s Shipment) Shipment {
	w.Player.DirtyCash -= s.Cost
	w.Stash(s.From)[s.Product] -= s.Units
	w.Logistics.NextID++
	s.ID = w.Logistics.NextID
	if s.Arrives <= s.Sent {
		s.Arrives = s.Sent + 1
	}
	w.Shipments = append(w.Shipments, s)
	w.Stats.Shipments++
	w.Stats.Shipped += s.Units
	return s
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
			if m.Runs() {
				w.DropStanding(m.City)
			}
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

var (
	ErrNotLieutenant = errors.New("only a lieutenant can run a city")
	ErrCityRun       = errors.New("somebody already runs that city")
)

// Assign gives a lieutenant on the payroll a city to run. From the next
// crew step they post the idle crew on its corners, sell its stash at
// their dial and keep their cut. One lieutenant per city; assigning to
// another city moves them.
func (w *World) Assign(id int, city string) error {
	if w.Over != nil {
		return ErrGameOver
	}
	m := w.Crew.Member(id)
	if m == nil {
		return ErrNoMember
	}
	if !m.Lieutenant() {
		return ErrNotLieutenant
	}
	if w.Cities[city] == nil {
		return ErrNoCity
	}
	if lt := w.Crew.Lieutenant(city); lt != nil && lt.ID != id {
		return ErrCityRun
	}
	if m.City == city {
		return nil
	}
	w.DropStanding(m.City)
	m.City = city
	m.Assigned = w.Day
	return nil
}

// Unassign takes a lieutenant off their city. The crew they posted stay
// where they are; the standing orders go.
func (w *World) Unassign(id int) error {
	if w.Over != nil {
		return ErrGameOver
	}
	m := w.Crew.Member(id)
	if m == nil {
		return ErrNoMember
	}
	if !m.Lieutenant() {
		return ErrNotLieutenant
	}
	w.DropStanding(m.City)
	m.City = ""
	return nil
}

// StandingOrder returns the order a lieutenant has standing for a product
// in a city, if the city is run and the order stands.
func (w *World) StandingOrder(city, product string) (SellOrder, bool) {
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

// DropStanding forgets every standing order in a city: the crew sim's,
// when the lieutenant who placed them is gone.
func (w *World) DropStanding(city string) {
	for k, o := range w.Delegated {
		if o.City == city {
			delete(w.Delegated, k)
		}
	}
}
