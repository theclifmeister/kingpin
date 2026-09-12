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
	ErrNotSupplied    = errors.New("the supplier here does not sell that; it comes in by the road")
	ErrNothingBought  = errors.New("nothing bought there today to return")
	ErrReturnGone     = errors.New("the units are no longer in the stash")
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
	for _, b := range w.Buys {
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
	for _, b := range w.Buys {
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
	for i := len(w.Buys) - 1; i >= 0 && left > 0; i-- {
		b := &w.Buys[i]
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
	kept := w.Buys[:0]
	for _, b := range w.Buys {
		if b.Qty > 0 {
			kept = append(kept, b)
		}
	}
	w.Buys = kept
	if len(w.Buys) == 0 {
		w.Buys = nil
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

// PlaceStanding sets a standing sell order (#114): the same units at
// the same dial every night until it is cancelled, resolved by the
// market sim exactly as a fresh order would be, at the crew's cut,
// wherever you placed no order of your own that day. It is checked as
// PlaceSell checks an order (a city, a product, a quantity the stash
// and the contract can cover) and is a persistent setting like a supply
// contract: the clock never clears it. One per product per city;
// placing again replaces the previous one.
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
	if qty <= 0 {
		return ErrBadQuantity
	}
	if have := w.Stock(city, product) + w.SupplyDue(city, product); qty > have {
		return fmt.Errorf("only %d %s in %s", have, w.ProductName(product), w.CityName(city))
	}
	if w.Standing == nil {
		w.Standing = map[string]SellOrder{}
	}
	w.Standing[OrderKey(city, product)] = SellOrder{City: city, Product: product, Qty: qty, Dial: dial}
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
// the dial it runs at and the stock the far city is kept at, per product,
// as units (Target) or as days of the far city's demand (Days, #115: the
// logistics sim sizes the units each morning from what the corners there
// serve, so the target follows the ground held). A product has one or
// the other: SetRouteDays clears the units and SetRouteTarget the days.
// The logistics sim reads it every day; the zero value is a route that is
// off, so a save from before the dial replays as it did, and a save from
// before the days target loads with Days nil, no schema bump.
type RouteSetting struct {
	Dial   events.RouteDial
	Target map[string]int // product id -> units the route's destination is kept stocked to
	Days   map[string]int // product id -> days of the destination's demand it is kept stocked to
}

// HasTargets is whether the route keeps anything anywhere: a route with
// none sends nothing however its dial stands.
func (rs RouteSetting) HasTargets() bool { return len(rs.Target) > 0 || len(rs.Days) > 0 }

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
// product, in units; zero clears it. The route sends the shortfall
// against it, buying by the lot at the source where a wholesaler deals.
// A days target for the product (SetRouteDays) is cleared: it has one or
// the other.
func (w *World) SetRouteTarget(id, product string, units int) error {
	return w.setRouteTarget(id, product, units, false)
}

// SetRouteDays sets the stock a route keeps its destination at for a
// product as days of that city's demand (#115): the logistics sim reads
// it each morning as days times World.Demand there, rounded up, so the
// target follows the corners held without being set again. Zero clears
// it, and a units target for the product goes with it.
func (w *World) SetRouteDays(id, product string, days int) error {
	return w.setRouteTarget(id, product, days, true)
}

// setRouteTarget is SetRouteTarget and SetRouteDays: the number goes in
// the one map and comes out of the other, and a map left empty is nil
// again so a cleared setting is the zero value it was.
func (w *World) setRouteTarget(id, product string, n int, days bool) error {
	if w.Over != nil {
		return ErrGameOver
	}
	if id == "" {
		return ErrNoRoute
	}
	if w.Home() == nil || w.Home().Market[product] == nil {
		return ErrUnknownProduct
	}
	if n < 0 {
		return ErrBadQuantity
	}
	if w.Routes == nil {
		w.Routes = map[string]RouteSetting{}
	}
	rs := w.Routes[id]
	set, other := &rs.Target, &rs.Days
	if days {
		set, other = &rs.Days, &rs.Target
	}
	delete(*other, product)
	if len(*other) == 0 {
		*other = nil
	}
	if n == 0 {
		delete(*set, product)
		if len(*set) == 0 {
			*set = nil
		}
	} else {
		if *set == nil {
			*set = map[string]int{}
		}
		(*set)[product] = n
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
	w.TakeStock(s.From, s.Product, s.Units)
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

// SetLieLow toggles lying low for the day. Lying low cancels all orders,
// and the handoffs queued against the buyers' contracts (#71): it is
// everyone's day off.
func (w *World) SetLieLow(on bool) {
	w.LieLow = on
	if on {
		w.Orders = map[string]SellOrder{}
		w.Deliveries = nil
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
