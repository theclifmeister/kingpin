package game

import (
	"errors"
	"fmt"
)

// The stash houses (#73): where the stock sits beyond the street, and
// which one the raid finds. A house is a place in a city on a block (the
// corner whose heat and risk it takes), with a capacity, a rent in clean
// cash a day and its own stock; the street (Player.Stash) is the rest.
// Everything that arrives in a city is put away, into the emptiest house
// there with room, else onto the street (AddStock); everything that
// leaves comes off the street first, then the houses in the order bought
// (TakeStock). Buying is putting it away; Move is for spreading, and for
// leaving a house the police know about.

var (
	ErrNoHouse     = errors.New("no such house")
	ErrHouseOwned  = errors.New("you already have that house")
	ErrHouseLocked = errors.New("that house is not on offer yet")
	ErrHouseFull   = errors.New("no room there")
	ErrSamePlace   = errors.New("that is where it is")
	ErrNotEnforcer = errors.New("only an enforcer can guard a house")
)

// House is one stash house the player rents: which block of which city
// it is on, what it holds and can hold, what it costs a day, who guards
// it, and whether the police know about it. Known is set by an informant
// on the payroll, by a robbery (word gets out) and by a bust there, and
// never unset: a known house is the one the raid finds, so move the
// stock and drop it. Unpaid counts the days the rent has gone unpaid;
// at rent_days the landlord throws you out, the stock with you.
type House struct {
	ID       string
	Name     string
	City     string
	Corner   string         // the block: its heat is the raid's weight, its risk the robbers'
	Capacity int            // units
	Price    int            // what it cost, dirty cash, once
	Rent     int            // clean cash a day
	Stock    map[string]int // product id -> units; written only in world.go and houses.go
	Guard    int            // crew id of the enforcer inside, 0 for nobody
	Known    bool           // the police have it in the file
	Bought   int            // day bought
	Unpaid   int            // days the rent has gone unpaid in a row
	Robbed   int            // times it has been robbed
	Raided   int            // times the police have hit it
}

// Units is what the house holds, all products.
func (h House) Units() int {
	n := 0
	for _, q := range h.Stock {
		n += q
	}
	return n
}

// Room is what the house can still take.
func (h House) Room() int { return max(0, h.Capacity-h.Units()) }

// Guarded reports whether an enforcer is inside.
func (h House) Guarded() bool { return h.Guard != 0 }

// House returns the owned house with id, or nil.
func (w *World) House(id string) *House {
	for i := range w.Houses {
		if w.Houses[i].ID == id {
			return &w.Houses[i]
		}
	}
	return nil
}

// HousesIn lists the houses in a city, in the order bought.
func (w *World) HousesIn(city string) []House {
	var out []House
	for _, h := range w.Houses {
		if h.City == city {
			out = append(out, h)
		}
	}
	return out
}

// GuardOf returns the house a crew member is guarding, or nil.
func (w *World) GuardOf(id int) *House {
	if id == 0 {
		return nil
	}
	for i := range w.Houses {
		if w.Houses[i].Guard == id {
			return &w.Houses[i]
		}
	}
	return nil
}

// HouseOffer is a house as houses.toml prices it, handed to BuyHouse by
// the caller so the world never needs the config.
type HouseOffer struct {
	ID         string
	Name       string
	City       string
	Corner     string
	Capacity   int
	Price      int // dirty cash, once
	Rent       int // clean cash a day
	UnlockCash int // peak cash that puts it on offer
}

// Locked reports whether the offer is still gated behind peak cash.
func (o HouseOffer) Locked(w *World) bool { return w.Stats.PeakCash < o.UnlockCash }

// BuyHouse takes the lease on a house for its price in dirty cash. It
// applies at once (the house is empty and takes what arrives from now
// on); the territory sim reports it and collects the rent from tomorrow.
func (w *World) BuyHouse(o HouseOffer) (House, error) {
	if w.Over != nil {
		return House{}, ErrGameOver
	}
	if w.House(o.ID) != nil {
		return House{}, ErrHouseOwned
	}
	if w.Cities[o.City] == nil {
		return House{}, ErrNoCity
	}
	if o.Locked(w) {
		return House{}, ErrHouseLocked
	}
	if o.Price > w.Player.DirtyCash {
		return House{}, fmt.Errorf("need $%d, only have $%d dirty", o.Price, w.Player.DirtyCash)
	}
	w.Player.DirtyCash -= o.Price
	h := House{ID: o.ID, Name: o.Name, City: o.City, Corner: o.Corner, Capacity: o.Capacity, Price: o.Price, Rent: o.Rent, Bought: w.Day}
	w.Houses = append(w.Houses, h)
	w.Today.HousesBought = append(w.Today.HousesBought, h.ID)
	return h, nil
}

// Drop walks away from a house's lease: whatever is in it goes with it
// and the guard is recalled. It is instant and free; the price is not
// refunded.
func (w *World) Drop(id string) (House, error) {
	if w.Over != nil {
		return House{}, ErrGameOver
	}
	h := w.House(id)
	if h == nil {
		return House{}, ErrNoHouse
	}
	return w.LoseHouse(id), nil
}

// LoseHouse takes a house off the world, its stock with it, and returns
// what it was: the landlord's (the territory sim) and Drop's.
func (w *World) LoseHouse(id string) House {
	var gone House
	kept := w.Houses[:0]
	for _, h := range w.Houses {
		if h.ID == id {
			gone = h
			continue
		}
		kept = append(kept, h)
	}
	w.Houses = kept
	if len(w.Houses) == 0 {
		w.Houses = nil
	}
	return gone
}

// Street is the place id of a city's street in Move: from or to the
// street rather than a house.
const Street = ""

// Move moves units of a product between two places in a city, a house
// or the street (Street), at once and for nothing: it is a car ride.
// The units moved are recorded in Moved for the day and the heat sim
// counts them as exposure at move_heat, so shuffling everything every
// day is not free. It is clamped at what the source holds and refused
// past the destination's room (a house's capacity; the street has none)
// and between cities.
func (w *World) Move(city, from, to, product string, units int) (int, error) {
	if w.Over != nil {
		return 0, ErrGameOver
	}
	if w.Cities[city] == nil {
		return 0, ErrNoCity
	}
	if w.Product(city, product) == nil {
		return 0, ErrUnknownProduct
	}
	if units <= 0 {
		return 0, ErrBadQuantity
	}
	if from == to {
		return 0, ErrSamePlace
	}
	var src, dst *House
	if from != Street {
		if src = w.House(from); src == nil || src.City != city {
			return 0, ErrNoHouse
		}
	}
	if to != Street {
		if dst = w.House(to); dst == nil || dst.City != city {
			return 0, ErrNoHouse
		}
	}
	have := w.Street(city, product)
	if src != nil {
		have = src.Stock[product]
	}
	if units > have {
		return 0, fmt.Errorf("only %d %s there", have, w.ProductName(product))
	}
	if dst != nil && units > dst.Room() {
		return 0, fmt.Errorf("%w: %s has room for %d", ErrHouseFull, dst.Name, dst.Room())
	}
	moved := w.MoveStock(city, from, to, product, units)
	w.Today.Moved = append(w.Today.Moved, Move{City: city, From: from, To: to, Product: product, Units: moved})
	return moved, nil
}

// Move is a move of stock between two places in a city today, for the
// heat sim and the report.
type Move struct {
	City    string
	From    string // house id, or Street
	To      string
	Product string
	Units   int
}

// MoveStock is the move itself, no checks and no record: up to units of
// a product from one place to another in a city, clamped at what the
// source holds and the destination's room, and what moved. The sims and
// the migration use it; Move is the player's.
func (w *World) MoveStock(city, from, to, product string, units int) int {
	var src, dst *House
	if from != Street {
		if src = w.House(from); src == nil {
			return 0
		}
	}
	if to != Street {
		if dst = w.House(to); dst == nil {
			return 0
		}
		units = min(units, dst.Room())
	}
	var taken int
	if src != nil {
		taken = w.takeFromHouse(src, product, units)
	} else {
		taken = w.TakeStreet(city, product, units)
	}
	if dst != nil {
		w.putInHouse(dst, product, taken)
	} else {
		w.stash(city)[product] += taken
	}
	return taken
}

// Guard puts an enforcer inside a house: one enforcer, one job, so they
// come off whatever corner they were on (and a Post takes them off the
// house). id 0 takes the guard off.
func (w *World) Guard(house string, id int) error {
	if w.Over != nil {
		return ErrGameOver
	}
	h := w.House(house)
	if h == nil {
		return ErrNoHouse
	}
	if id == 0 {
		h.Guard = 0
		return nil
	}
	m := w.Crew.Member(id)
	if m == nil {
		return ErrNoMember
	}
	if m.Role != "enforcer" {
		return ErrNotEnforcer
	}
	w.Recall(id)
	h.Guard = id
	return nil
}

// Fullest is the house in a city holding the most, the earlier bought
// between two with the same, or nil with none holding anything; known
// narrows it to the houses the police know about.
func (w *World) Fullest(city string, known bool) *House {
	var best *House
	for i := range w.Houses {
		h := &w.Houses[i]
		if h.City != city || h.Units() == 0 || (known && !h.Known) {
			continue
		}
		if best == nil || h.Units() > best.Units() {
			best = h
		}
	}
	return best
}

// StarterHouse is the id prefix of the rent-free house the 11 -> 12
// migration puts an old save's stash in, one a city.
const StarterHouse = "starter:"

// MigrateHouses is the 11 -> 12 step (#73): a save from before the
// houses kept its stock in one pile a city, so each city holding
// anything gets a rent-free starter house on the block the player
// stands on there (else its first corner), sized at the carry limit or
// the pile, whichever is bigger, and the pile goes into it. The old
// save plays on as it did: the raid finds the one house, the informant's
// raid takes the whole of it, and nothing was bought.
func MigrateHouses(w *World) {
	for _, cid := range w.CityOrder {
		c := w.Cities[cid]
		if c == nil || w.Player.StockIn(cid) == 0 || w.House(StarterHouse+cid) != nil {
			continue
		}
		corner := ""
		if p := w.PostOf(You); p != nil && p.City == cid {
			corner = p.ID
		} else if len(c.Corners) > 0 {
			corner = c.Corners[0].ID
		}
		h := House{
			ID: StarterHouse + cid, Name: "The old stash", City: cid, Corner: corner,
			Capacity: max(w.Player.CarryLimit, w.Player.StockIn(cid)), Bought: w.Day,
		}
		w.Houses = append(w.Houses, h)
		for id, q := range w.StreetOf(cid) {
			w.MoveStock(cid, Street, h.ID, id, q)
		}
	}
}
