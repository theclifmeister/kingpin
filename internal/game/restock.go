package game

import (
	"errors"
	"fmt"
	"math"
	"sort"
)

// ErrNoRoom is the stash refusing more than it holds (#356): a buy, a
// cut or a cook past the room in a city. The refusal itself is a
// RoomError, which carries the room there is, so a front end can offer
// what fits rather than a bare no; errors.Is(err, ErrNoRoom) matches it.
var ErrNoRoom = errors.New("no room in the stash")

// RoomError is the one refusal for a stash too full for what was asked
// (#356): Free is how many more units the city holds, never negative,
// City its name.
type RoomError struct {
	Free int
	City string
}

func (e *RoomError) Error() string {
	return fmt.Sprintf("can only hold %d more units in %s", e.Free, e.City)
}

// Is makes a RoomError match ErrNoRoom.
func (e *RoomError) Is(target error) bool { return target == ErrNoRoom }

// noRoom is the RoomError for a city with free units of room.
func (w *World) noRoom(city string, free int) error {
	return &RoomError{Free: max(0, free), City: w.CityName(city)}
}

// Affords is how many units of a product cash, or the connect's book on
// credit, covers at what Buy would charge (World.Quote: the markup
// where the buy goes through a lieutenant, #174): the plain price
// first, then the small-lot premium once the buy is under the lot.
func (w *World) Affords(s *Supplier, product string, credit bool, cash int) int {
	unit := s.Price[product] * w.BuyMarkup(s.City)
	if unit <= 0 || cash <= 0 {
		return 0
	}
	if credit {
		unit *= s.CreditRatio
	}
	n := int(math.Floor(float64(cash) / unit))
	if n < s.Lot && s.SmallLot > 1 {
		n = int(math.Floor(float64(cash) / (unit * s.SmallLot)))
	}
	for n > 0 && w.Quote(s, product, n, credit) > cash {
		n--
	}
	return n
}

// MaxBuy is the most of a product a buy from the connect can take right
// now (#356): what the dirty cash (or, on credit, their book) covers,
// the room in the stash there and what they have left today, the least
// of the three; 0 where they do not sell it today. Buy never refuses
// MaxBuy units for any of those reasons.
func (w *World) MaxBuy(s *Supplier, product string, credit bool) int {
	if s == nil || !w.Available(s, product) {
		return 0
	}
	cash := w.Player.DirtyCash
	if credit {
		cash = s.Credit()
	}
	return max(0, min(w.Affords(s, product, credit, cash), w.Free(s.City), s.Left()))
}

// StockLevels is the stash a city wants for days of the demand the
// corners you work there serve (#356), a level per product its market
// supplies: days times World.Demand, cut to the stash's share of the
// product by the market's demand where the levels together would
// overfill it. It is the stocked player's contract (harness.Stocked)
// and the market screen's restock (RestockPlan), one sizing.
func (w *World) StockLevels(city string, days float64) map[string]int {
	c := w.Cities[city]
	if c == nil {
		return nil
	}
	total, want := 0.0, 0.0
	levels := map[string]float64{}
	for _, id := range w.Products {
		m := c.Market[id]
		if m == nil || m.NoSupply {
			continue
		}
		levels[id] = days * w.Demand(city, id)
		total += m.Demand
		want += levels[id]
	}
	room := float64(w.Capacity(city))
	out := make(map[string]int, len(levels))
	for _, id := range w.Products {
		level, ok := levels[id]
		if !ok {
			continue
		}
		if want > room && total > 0 {
			level = math.Min(level, room*c.Market[id].Demand/total)
		}
		out[id] = max(0, int(level))
	}
	return out
}

// RestockLine is one buy of a restock (#356): a product, the connect it
// comes from, the level it tops up to, what is stashed and on the road
// to it, and the units and the cost the buy would take.
type RestockLine struct {
	Product  string `json:"product"`
	Supplier string `json:"supplier"`
	Level    int    `json:"level"`
	Have     int    `json:"have"`
	Units    int    `json:"units"`
	Cost     int    `json:"cost"`
}

// RestockPlan is what topping a city's stash up to days of demand would
// buy by hand right now (#356): one line per product, sized to the
// StockLevels level less what is stashed there and on the road to it
// (a supply contract's shortfall), from the cheapest connect with units
// left after the lines before it, cut to the room and the dirty cash
// over keep after the lines before it have had theirs. The lines go in
// ladder order where the cash and the room go round, and by margin
// where they do not (ByMargin, #470: the order the supply contracts
// fill in, so the plan is still the stocked player's). A product nobody
// sells today, or with nothing to buy, has no line. It changes nothing:
// the front end reviews it and buys the lines.
func (w *World) RestockPlan(city string, days float64, keep int) []RestockLine {
	if !w.CanBuyIn(city) {
		return nil
	}
	levels := w.StockLevels(city, days)
	order := make([]SupplyContract, 0, len(w.Products))
	for _, id := range w.Products {
		order = append(order, SupplyContract{City: city, Product: id, Units: levels[id]})
	}
	plan, cash, room := w.restock(order, keep)
	if cash || room {
		plan, _, _ = w.restock(w.ByMargin(order, cash), keep)
	}
	return plan
}

// restock lays out a restock's lines in the order given, and says
// whether a line was cut short of the cash or the room.
func (w *World) restock(order []SupplyContract, keep int) (plan []RestockLine, cashShort, roomShort bool) {
	if len(order) == 0 {
		return nil, false, false
	}
	city := order[0].City
	budget := max(0, w.Player.DirtyCash-keep)
	room := max(0, w.Free(city))
	taken := map[string]int{}
	for _, c := range order {
		id, level := c.Product, c.Units
		have := w.Stock(city, id) + w.Bound(city, id)
		short := level - have
		if short <= 0 {
			continue
		}
		var sup *Supplier
		for _, s := range w.SuppliersIn(city) {
			if !w.Available(s, id) || s.Left()-taken[s.ID] <= 0 {
				continue
			}
			if sup == nil || s.Price[id] < sup.Price[id] {
				sup = s
			}
		}
		if sup == nil {
			continue
		}
		afford := w.Affords(sup, id, false, budget)
		units := max(0, min(short, room, afford, sup.Left()-taken[sup.ID]))
		if units < short {
			cashShort = cashShort || afford < short
			roomShort = roomShort || room < short
		}
		if units == 0 {
			continue
		}
		cost := w.Quote(sup, id, units, false)
		plan = append(plan, RestockLine{Product: id, Supplier: sup.ID, Level: level, Have: have, Units: units, Cost: cost})
		budget -= cost
		room -= units
		taken[sup.ID] += units
	}
	return plan, cashShort, roomShort
}

// Margin is what a unit of a product bought in a city today is expected
// to earn on the street there (#470): the street price less the
// supplier price (World.SupplierPrice) or, perDollar, the street price
// over it, what a dollar spent there brings back. The contract markup
// scales the supplier price alike for every product, so the order it
// puts products in is the hand's and the contract's alike. Zero where
// the city does not trade the product or nobody sells it.
func (w *World) Margin(city, product string, perDollar bool) float64 {
	m := w.Product(city, product)
	if m == nil {
		return 0
	}
	cost := w.SupplierPrice(city, product)
	if cost <= 0 {
		return 0
	}
	if perDollar {
		return m.Price / cost
	}
	return m.Price - cost
}

// ByMargin is the contracts (or a restock's products) in the order the
// cash or the room is best spent on when it runs short (#470): short of
// cash (perDollar), by what a dollar brings back, since a dollar is what
// runs out; short of room alone, by what a unit earns, since a unit of
// the stash is. Ties keep the order given, the city and ladder order.
func (w *World) ByMargin(order []SupplyContract, perDollar bool) []SupplyContract {
	rank := make([]float64, len(order))
	for i, c := range order {
		rank[i] = w.Margin(c.City, c.Product, perDollar)
	}
	idx := make([]int, len(order))
	for i := range idx {
		idx[i] = i
	}
	sort.SliceStable(idx, func(a, b int) bool { return rank[idx[a]] > rank[idx[b]] })
	out := make([]SupplyContract, len(order))
	for i, j := range idx {
		out[i] = order[j]
	}
	return out
}
