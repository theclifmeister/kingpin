package game

import (
	"errors"
	"fmt"
	"math"
)

var (
	ErrNoSupplier       = errors.New("no such connect")
	ErrSupplierLocked   = errors.New("they will not deal with you yet")
	ErrSupplierFrozen   = errors.New("they are not taking your calls")
	ErrSupplierCapacity = errors.New("they cannot get you that many today")
	ErrNoCredit         = errors.New("they do not give credit")
	ErrCreditLimit      = errors.New("that is past what they will run you")
)

// Supplier is a connect (#72): somebody in a city you buy from. The
// static part is copied in from suppliers.toml when the run is seeded,
// so the world never needs the config to sell you a unit; the market
// sim stamps the price, the capacity and the credit limit each morning
// from the band the relationship sits in, and the relationship itself
// is what buying, paying and getting seized move.
type Supplier struct {
	ID          string
	Name        string
	City        string
	Products    []string // product ids they sell; nil is everything the city's supplier sells
	Temper      string   // patient, sharp, connected: what a missed payment does
	Lot         int      // units they deal in: the relationship counts lots, and a buy under one pays SmallLot
	SmallLot    float64  // multiplier on the price of a buy under the lot; 1 is none
	CreditDays  int      // days to pay; 0 is no credit
	CreditRatio float64  // multiplier on the price of a unit taken on credit
	UnlockCash  int      // peak cash that opens the door; 0 is from day one
	UnlockRel   float64  // the street connect in their city vouches for you at this Rel; 0 is no need
	Wholesale   bool     // the connect the routes buy from

	// Stamped by the market sim each morning from the band Rel is in.
	Price map[string]float64 // per unit today, by product; a buy nudges it up for the day
	Cap   int                // units they can get you today
	Limit int                // credit they will run you to today
	Band  int                // the band Rel was in when they were stamped

	// The relationship.
	Rel         float64 // 0..100
	Debt        int     // what you owe them, dirty
	DebtDue     int     // the day it is due; 0 with no debt
	Extended    bool    // a patient temper has extended this debt once already
	FrozenUntil int     // day they take calls again; 0 or past is open
	Warned      int     // day of their last warning; 0 never
	Opened      bool    // the door has opened: the market sim says so once, the morning Locked turns false
	Late        int     // payments you have missed, lifetime
	BoughtToday int     // units bought today, by you, a contract or the road, against Cap
	Bought      int     // units bought from them, lifetime
	LastBought  int     // day of the last buy; read with Bought > 0
	Lots        float64 // lots bought from them, lifetime
}

// Frozen reports whether the connect is not taking calls on day.
func (s Supplier) Frozen(day int) bool { return s.FrozenUntil > day }

// Sells reports whether the connect deals in a product, by their list;
// whether the city's supplier sells it at all is the market's flag.
func (s Supplier) Sells(product string) bool {
	if len(s.Products) == 0 {
		return true
	}
	for _, p := range s.Products {
		if p == product {
			return true
		}
	}
	return false
}

// Left is how many more units the connect can get you today.
func (s Supplier) Left() int { return max(0, s.Cap-s.BoughtToday) }

// Credit is how much more credit the connect will run you today: the
// limit less what you owe; nothing where they give none.
func (s Supplier) Credit() int {
	if s.CreditDays <= 0 {
		return 0
	}
	return max(0, s.Limit-s.Debt)
}

// Locked reports whether the connect will deal with you yet: peak cash
// past their line, and the street connect in their city at UnlockRel.
func (s Supplier) Locked(w *World) bool {
	if w.Stats.PeakCash < s.UnlockCash {
		return true
	}
	if s.UnlockRel > 0 {
		st := w.StreetSupplier(s.City)
		if st == nil || st.ID == s.ID || st.Rel < s.UnlockRel {
			return true
		}
	}
	return false
}

// Open reports whether the connect sells to you today: unlocked and
// taking calls.
func (s Supplier) Open(w *World) bool { return !s.Locked(w) && !s.Frozen(w.Day) }

// Owes reports whether you owe the connect anything.
func (s Supplier) Owes() bool { return s.Debt > 0 }

// Supplier returns the connect with id, or nil.
func (w *World) Supplier(id string) *Supplier {
	for i := range w.Suppliers {
		if w.Suppliers[i].ID == id {
			return &w.Suppliers[i]
		}
	}
	return nil
}

// SuppliersIn lists the connects in a city, in the order seeded.
func (w *World) SuppliersIn(city string) []*Supplier {
	var out []*Supplier
	for i := range w.Suppliers {
		if w.Suppliers[i].City == city {
			out = append(out, &w.Suppliers[i])
		}
	}
	return out
}

// StreetSupplier is the connect every run has in a city: the first one
// seeded there that is not the wholesaler.
func (w *World) StreetSupplier(city string) *Supplier {
	for i := range w.Suppliers {
		if s := &w.Suppliers[i]; s.City == city && !s.Wholesale {
			return s
		}
	}
	return nil
}

// WholesaleSupplier is the connect the routes buy from in a city, or
// nil where nobody sells by the lot.
func (w *World) WholesaleSupplier(city string) *Supplier {
	for i := range w.Suppliers {
		if s := &w.Suppliers[i]; s.City == city && s.Wholesale {
			return s
		}
	}
	return nil
}

// AddSupplier puts a connect in the run, or replaces the one with its
// id.
func (w *World) AddSupplier(s Supplier) *Supplier {
	if s.Price == nil {
		s.Price = map[string]float64{}
	}
	if have := w.Supplier(s.ID); have != nil {
		*have = s
		return have
	}
	w.Suppliers = append(w.Suppliers, s)
	return &w.Suppliers[len(w.Suppliers)-1]
}

// Available reports whether the connect sells you a product today: they
// deal in it, the city's supplier sells it, they are open and they have
// units left.
func (w *World) Available(s *Supplier, product string) bool {
	m := w.Product(s.City, product)
	return m != nil && !m.NoSupply && s.Sells(product) && s.Price[product] > 0 && s.Open(w) && s.Left() > 0
}

// BestSupplier is the cheapest connect in a city that sells you a
// product today (Available), or nil: what a buy by hand defaults to and
// a supply contract buys from.
func (w *World) BestSupplier(city, product string) *Supplier {
	var best *Supplier
	for _, s := range w.SuppliersIn(city) {
		if !w.Available(s, product) {
			continue
		}
		if best == nil || s.Price[product] < best.Price[product] {
			best = s
		}
	}
	return best
}

// SupplierPrice is the best available connect's price for a product in
// a city (BestSupplier), or, with none available today, the cheapest
// that deals in it at all, or, with no connect there, what the market
// last stamped: the number the supplier column shows and NetWorth
// values stock at.
func (w *World) SupplierPrice(city, product string) float64 {
	if s := w.BestSupplier(city, product); s != nil {
		return s.Price[product]
	}
	price := 0.0
	for _, s := range w.SuppliersIn(city) {
		if m := w.Product(city, product); m != nil && !m.NoSupply && s.Sells(product) && (price == 0 || s.Price[product] < price) {
			price = s.Price[product]
		}
	}
	if price == 0 {
		if m := w.Product(city, product); m != nil {
			return m.SupplierPrice
		}
	}
	return price
}

// refreshSupplierPrice brings the market's SupplierPrice for a product
// in a city into line with the connects there.
func (w *World) refreshSupplierPrice(city, product string) {
	if m := w.Product(city, product); m != nil && len(w.SuppliersIn(city)) > 0 {
		m.SupplierPrice = w.SupplierPrice(city, product)
	}
}

// RefreshSupplierPrices brings every market's SupplierPrice into line
// with the connects: the market sim's, after it has stamped them.
func (w *World) RefreshSupplierPrices() {
	for _, cid := range w.CityOrder {
		for id := range w.Cities[cid].Market {
			w.refreshSupplierPrice(cid, id)
		}
	}
}

// Quote is what qty units of a product would cost from the connect right
// now, cash or on credit, before the pressure the buy itself adds: their
// price, the small-lot premium under the lot and the credit premium on
// credit, rounded up to the dollar. It is what buy charges.
func (s Supplier) Quote(product string, qty int, credit bool) int {
	unit := s.Price[product]
	if qty < s.Lot && s.SmallLot > 1 {
		unit *= s.SmallLot
	}
	if credit && s.CreditRatio > 0 {
		unit *= s.CreditRatio
	}
	return int(math.Ceil(unit * float64(qty)))
}

// Owed is what you owe every connect together.
func (w *World) Owed() int {
	n := 0
	for _, s := range w.Suppliers {
		n += s.Debt
	}
	return n
}

// DebtsDue lists the connects with a debt due on or before day, in the
// order seeded.
func (w *World) DebtsDue(day int) []*Supplier {
	var out []*Supplier
	for i := range w.Suppliers {
		if s := &w.Suppliers[i]; s.Debt > 0 && s.DebtDue <= day {
			out = append(out, s)
		}
	}
	return out
}

// buy is the one path a unit takes from a connect into a stash: Buy's,
// where the player stands at the connect's price, cash or on credit, a
// supply contract's, in its city at the contract markup and always for
// cash, and never the road's (Restock is by the lot and outside the
// stash's capacity). Cost is the unit price times the units, rounded up
// to the dollar; under the lot the connect's small-lot premium is on
// it, on credit their credit premium.
func (w *World) buy(s *Supplier, product string, qty int, markup float64, credit bool, pricePressure float64, contract bool) (Purchase, error) {
	m := w.Product(s.City, product)
	if m == nil {
		return Purchase{}, ErrUnknownProduct
	}
	if qty <= 0 {
		return Purchase{}, ErrBadQuantity
	}
	if m.NoSupply || !s.Sells(product) {
		return Purchase{}, ErrNotSupplied
	}
	if s.Locked(w) {
		return Purchase{}, fmt.Errorf("%w: %s", ErrSupplierLocked, s.Name)
	}
	if s.Frozen(w.Day) {
		return Purchase{}, fmt.Errorf("%w: %s, for %d more days", ErrSupplierFrozen, s.Name, s.FrozenUntil-w.Day)
	}
	if credit && s.CreditDays <= 0 {
		return Purchase{}, fmt.Errorf("%w: %s", ErrNoCredit, s.Name)
	}
	if left := s.Left(); qty > left {
		return Purchase{}, fmt.Errorf("%w: %s can get you %d more today", ErrSupplierCapacity, s.Name, left)
	}
	small := qty < s.Lot && s.SmallLot > 1
	unit := s.Price[product] * markup
	if small {
		unit *= s.SmallLot
	}
	if credit && s.CreditRatio > 0 {
		unit *= s.CreditRatio
	}
	cost := int(math.Ceil(unit * float64(qty)))
	if credit {
		if s.Debt+cost > s.Limit {
			return Purchase{}, fmt.Errorf("%w: %s will run you $%d, you owe $%d", ErrCreditLimit, s.Name, s.Limit, s.Debt)
		}
	} else if cost > w.Player.DirtyCash {
		return Purchase{}, fmt.Errorf("need $%d, only have $%d dirty", cost, w.Player.DirtyCash)
	}
	if free := w.Free(s.City); qty > free {
		return Purchase{}, fmt.Errorf("can only hold %d more units in %s", free, w.CityName(s.City))
	}
	p := Purchase{City: s.City, Product: product, Qty: qty, UnitPrice: unit, Cost: cost, Prior: s.Price[product], Contract: contract, Supplier: s.ID, Credit: credit, SmallLot: small}
	if contract {
		p.Day = w.Day + 1 // the morning the tick brings
	}
	if credit {
		if s.Debt == 0 {
			s.DebtDue = w.Day + s.CreditDays
		}
		s.Debt += cost
		w.Stats.Credit += cost
	} else {
		w.Player.DirtyCash -= cost
	}
	w.Stash(s.City)[product] += qty
	m.BoughtToday += qty
	s.took(w, product, qty)
	if demand := w.Demand(s.City, product); demand > 0 {
		s.Price[product] *= 1 + pricePressure*float64(qty)/demand
	}
	w.refreshSupplierPrice(s.City, product)
	w.Buys = append(w.Buys, p)
	return p, nil
}

// took books qty units against the connect: the day's capacity, the
// lifetime count and the lots the relationship counts.
func (s *Supplier) took(w *World, product string, qty int) {
	s.BoughtToday += qty
	s.Bought += qty
	s.LastBought = w.Day
	if s.Lot > 0 {
		s.Lots += float64(qty) / float64(s.Lot)
	}
}

// Buy purchases qty units of a product from a connect in the city the
// player is in (ErrElsewhere from any other), into the stash there,
// with dirty cash or, on credit, against what they will run you
// (Supplier.Credit), due CreditDays on. It applies immediately and
// nudges the connect's price up for the rest of the day.
func (w *World) Buy(supplier, product string, qty int, credit bool, pricePressure float64) (Purchase, error) {
	if w.Over != nil {
		return Purchase{}, ErrGameOver
	}
	s := w.Supplier(supplier)
	if s == nil {
		return Purchase{}, ErrNoSupplier
	}
	if s.City != w.Player.Location {
		return Purchase{}, ErrElsewhere
	}
	return w.buy(s, product, qty, 1, credit, pricePressure, false)
}

// FillSupply is the market sim buying against a supply contract (#113):
// qty units of a product from a connect in its city, wherever the
// player is, for cash at markup times the connect's price, into the
// stash there. It is the same path as Buy (the price pressure and
// BoughtToday move exactly as they do for a buy by hand, so the connect
// reacts to a contract as it does to you) with the receipt marked a
// contract's and dated, so the clock keeps it through the day for the
// cart. The sim picks the connect (the cheapest available, #72) and
// sizes qty to the shortfall, the room and the budget; the world holds
// it to the stash's room, the cash and the connect's day as it holds a
// buy.
func (w *World) FillSupply(supplier, product string, qty int, markup, pricePressure float64) (Purchase, error) {
	s := w.Supplier(supplier)
	if s == nil {
		return Purchase{}, ErrNoSupplier
	}
	if markup <= 0 {
		markup = 1
	}
	return w.buy(s, product, qty, markup, false, pricePressure, true)
}

// Restock is the logistics sim buying lots from the wholesale connect in
// a city, wherever the player is, to feed a route (#61): it is not a
// player action and not reported through Buys (the sim reports it as
// WholesaleBought). The city must have a wholesaler and the door must be
// open: locked it is ErrNoRoute as it always was, frozen
// ErrSupplierFrozen, and over what they can get you today
// ErrSupplierCapacity, a shortfall the route sends again tomorrow. The
// lots go into the stash there regardless of its capacity: the road
// takes them, and the odd lot's remainder waits for tomorrow's
// shipment. The road never takes credit.
func (w *World) Restock(city, product string, lots int, pricePressure float64) (Purchase, error) {
	c := w.Cities[city]
	if c == nil {
		return Purchase{}, ErrNoCity
	}
	s := w.WholesaleSupplier(city)
	if s == nil || s.Locked(w) {
		return Purchase{}, ErrNoRoute
	}
	m := c.Market[product]
	if m == nil {
		return Purchase{}, ErrUnknownProduct
	}
	if m.NoSupply || !s.Sells(product) {
		return Purchase{}, ErrNotSupplied
	}
	if lots <= 0 || s.Lot <= 0 {
		return Purchase{}, ErrBadQuantity
	}
	if s.Frozen(w.Day) {
		return Purchase{}, fmt.Errorf("%w: %s", ErrSupplierFrozen, s.Name)
	}
	qty := lots * s.Lot
	if qty > s.Left() {
		return Purchase{}, fmt.Errorf("%w: %s has %d left today", ErrSupplierCapacity, s.Name, s.Left())
	}
	unit := s.Price[product]
	cost := int(math.Ceil(unit * float64(qty)))
	if cost > w.Player.DirtyCash {
		return Purchase{}, fmt.Errorf("need $%d, only have $%d dirty", cost, w.Player.DirtyCash)
	}
	w.Player.DirtyCash -= cost
	w.Stash(city)[product] += qty
	m.BoughtToday += qty
	s.took(w, product, qty)
	if demand := w.Demand(city, product); demand > 0 {
		s.Price[product] *= 1 + pricePressure*float64(qty)/demand
	}
	w.refreshSupplierPrice(city, product)
	return Purchase{City: city, Product: product, Qty: qty, UnitPrice: unit, Cost: cost, Supplier: s.ID}, nil
}
