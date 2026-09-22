package game

import (
	"fmt"
	"math"
)

// stash is the player's stock on a city's street, created empty on
// first use. It is the one handle on the map: every unit that enters or
// leaves a stash goes through AddStock, TakeStock or SetStock below
// (#144), so #73 and #47 can change what the map means or holds by
// changing these and nothing else. TestStashHasNoWriters holds the rest
// of the code to it. Since #73 the map is the street and the houses
// (World.Houses) are the rest: the accessors below read and write both,
// and a house's Stock map is written only here and in houses.go.
func (w *World) stash(city string) map[string]int {
	if w.Player.Stash == nil {
		w.Player.Stash = map[string]map[string]int{}
	}
	s := w.Player.Stash[city]
	if s == nil {
		s = map[string]int{}
		w.Player.Stash[city] = s
	}
	return s
}

// quality is the quality map of a city's stash, created empty on first
// use: the one handle on it, as stash is on the units.
func (w *World) quality(city string) map[string]float64 {
	if w.Player.Quality == nil {
		w.Player.Quality = map[string]map[string]float64{}
	}
	q := w.Player.Quality[city]
	if q == nil {
		q = map[string]float64{}
		w.Player.Quality[city] = q
	}
	return q
}

// StreetQuality is the file's default quality (#47): what the connects
// sell at and what a lot with no figure of its own reads. The market
// sim stamps it (BaseQuality); before it has, the scale's middle.
func (w *World) StreetQuality() float64 {
	if w.BaseQuality > 0 {
		return w.BaseQuality
	}
	return StreetQuality
}

// Quality is the quality of what the player holds of a product in a
// city (#47), one number for the street and the houses there; the
// default where the lot has never been given one (an empty stash, a
// stash set by hand).
func (w *World) Quality(city, product string) float64 {
	if q := w.Player.Quality[city][product]; q > 0 {
		return q
	}
	return w.StreetQuality()
}

// Lot is what the player holds of a product in a city and its quality.
func (w *World) Lot(city, product string) Lot {
	return Lot{Units: w.Stock(city, product), Quality: w.Quality(city, product)}
}

// SetQuality puts a city's stash of a product at exactly a quality,
// clamped to 0..100 (a zero reads as the default). It is the tests' and
// the migration's; in play a lot's quality moves only by what comes in
// (AddStock) and the cut (Cut).
func (w *World) SetQuality(city, product string, quality float64) {
	w.quality(city)[product] = math.Max(0, math.Min(100, quality))
}

// StashOf is a copy of the player's stock in a city, product by product,
// the street and the houses there together, for a reader that walks it
// (the UI, the harness, the cart). Writing to the copy changes nothing;
// the writers are AddStock and TakeStock.
func (w *World) StashOf(city string) map[string]int {
	out := w.StreetOf(city)
	for _, h := range w.Houses {
		if h.City != city {
			continue
		}
		for id, q := range h.Stock {
			out[id] += q
		}
	}
	return out
}

// StreetOf is a copy of what is on a city's street, product by product:
// what you carry there and what the runners posted there hold.
func (w *World) StreetOf(city string) map[string]int {
	s := w.Player.Stash[city]
	out := make(map[string]int, len(s))
	for id, q := range s {
		out[id] = q
	}
	return out
}

// Stock is how many units of a product the player holds in a city: the
// street and the houses there.
func (w *World) Stock(city, product string) int {
	n := w.Player.Stash[city][product]
	for _, h := range w.Houses {
		if h.City == city {
			n += h.Stock[product]
		}
	}
	return n
}

// Street is how many units of a product are on a city's street.
func (w *World) Street(city, product string) int { return w.Player.Stash[city][product] }

// AddStock puts units of a product into a city at a quality (#47): a
// buy at the connect's, a contract's morning lot, a shipment landing
// at what it carried, the road's lots, a card at the default, a cook at
// the chemist's, a cut at nothing. The lot's quality becomes the mean
// by units of what was there and what came (a lot with nothing in it
// takes the incoming figure), clamped to 0..100; a quality of zero or
// under reads as the default. It is put away (#73): into the emptiest
// house there with room, the next once that is full, and onto the
// street when the houses are full or there are none (the road's rule
// stands: a lot that fits nowhere sits on the street until the shipment
// takes it). A negative count is a TakeStock.
func (w *World) AddStock(city, product string, units int, quality float64) {
	if units < 0 {
		w.TakeStock(city, product, -units)
		return
	}
	if units == 0 {
		return
	}
	if quality <= 0 {
		quality = w.StreetQuality()
	}
	have := w.Stock(city, product)
	q := quality
	if have > 0 {
		q = (w.Quality(city, product)*float64(have) + quality*float64(units)) / float64(have+units)
	}
	w.SetQuality(city, product, q)
	w.put(city, product, units)
}

// put is the placement half of AddStock: the units into the houses and
// onto the street, the lot's quality left as it is.
func (w *World) put(city, product string, units int) {
	for units > 0 {
		h := w.emptiest(city)
		if h == nil {
			break
		}
		n := min(units, h.Room())
		w.putInHouse(h, product, n)
		units -= n
	}
	if units > 0 {
		w.stash(city)[product] += units
	}
}

// emptiest is the house in a city with the most room, the earlier
// bought between two with the same, or nil when none has any.
func (w *World) emptiest(city string) *House {
	var best *House
	for i := range w.Houses {
		h := &w.Houses[i]
		if h.City != city || h.Room() <= 0 {
			continue
		}
		if best == nil || h.Room() > best.Room() {
			best = h
		}
	}
	return best
}

// putInHouse is the one write into a house's stock going up.
func (w *World) putInHouse(h *House, product string, units int) {
	if units <= 0 {
		return
	}
	if h.Stock == nil {
		h.Stock = map[string]int{}
	}
	h.Stock[product] += units
}

// takeFromHouse is the one write into a house's stock going down: up to
// units, clamped at what is there, and what it took.
func (w *World) takeFromHouse(h *House, product string, units int) int {
	taken := max(0, min(units, h.Stock[product]))
	if taken > 0 {
		h.Stock[product] -= taken
		if h.Stock[product] == 0 {
			delete(h.Stock, product)
		}
	}
	return taken
}

// TakeStock takes up to units of a product out of a city (a sale, a
// shipment leaving, a handoff, a collection, a card) and returns what it
// took, so the stash never goes under zero and a caller that asked for
// more than was there learns what it got. The street goes first, then
// the houses in the order bought (#73), so a sale or a shipment never
// depends on iteration order.
func (w *World) TakeStock(city, product string, units int) int {
	taken := w.TakeStreet(city, product, units)
	for i := range w.Houses {
		if taken >= units {
			break
		}
		if h := &w.Houses[i]; h.City == city {
			taken += w.takeFromHouse(h, product, units-taken)
		}
	}
	return taken
}

// TakeStreet takes up to units of a product off a city's street alone
// (a corner robbery: what the runner was carrying) and returns what it
// took.
func (w *World) TakeStreet(city, product string, units int) int {
	s := w.stash(city)
	taken := max(0, min(units, s[product]))
	s[product] -= taken
	return taken
}

// TakeFromHouse takes up to units of a product out of one house (a
// raid, a robbery there) and returns what it took; 0 for a house that
// is not there.
func (w *World) TakeFromHouse(house, product string, units int) int {
	h := w.House(house)
	if h == nil {
		return 0
	}
	return w.takeFromHouse(h, product, units)
}

// SetStock puts a city's street stock of a product at exactly units,
// whatever it was. It is the tests' and the migration's (a save from
// before the cities carried its stock on the player); nothing in play
// sets a stash to a figure, it adds to it or takes from it.
func (w *World) SetStock(city, product string, units int) {
	w.stash(city)[product] = max(0, units)
}

// StockIn is the number of units the player holds in a city, all
// products: the street and the houses there.
func (w *World) StockIn(city string) int {
	n := w.Player.StockIn(city)
	for _, h := range w.Houses {
		if h.City == city {
			n += h.Units()
		}
	}
	return n
}

// Stashed is every unit in every city, street and houses, the road left
// out.
func (w *World) Stashed() int {
	n := w.Player.TotalStock()
	for _, h := range w.Houses {
		n += h.Units()
	}
	return n
}

// InTransit is how many units of a product are on the road, bound
// anywhere.
func (w *World) InTransit(product string) int {
	n := 0
	for _, s := range w.Shipments {
		if s.Product == product {
			n += s.Units
		}
	}
	return n
}

// Bound is how many units of a product are on the road to a city.
func (w *World) Bound(to, product string) int {
	n := 0
	for _, s := range w.Shipments {
		if s.To == to && s.Product == product {
			n += s.Units
		}
	}
	return n
}

// Cut steps on a city's stash of a product (#47): ratio of the units are
// added at nothing, so the lot's quality drops by the same ratio (the
// pure weight is conserved: units x (1 + ratio) at quality / (1 +
// ratio)), and a chemist's hand puts bonus points of it back, never
// over what it was. It is instant, paid at cost a unit added in dirty
// cash, refused past the room the city has (the units have to be held)
// and past the ratio given as the most (the product's cut_max), and
// recorded on the day's scratch for the report. It returns what it did.
func (w *World) Cut(city, product string, ratio, most float64, cost int, bonus float64, chemist string) (CutRecord, error) {
	if w.Over != nil {
		return CutRecord{}, ErrGameOver
	}
	if w.Product(city, product) == nil {
		return CutRecord{}, ErrUnknownProduct
	}
	if ratio <= 0 || most <= 0 || ratio > most+1e-9 {
		return CutRecord{}, ErrBadRatio
	}
	units := w.Stock(city, product)
	if units <= 0 {
		return CutRecord{}, ErrNothingToCut
	}
	added := int(math.Round(float64(units) * ratio))
	if added <= 0 {
		return CutRecord{}, ErrBadRatio
	}
	if free := w.Free(city); added > free {
		return CutRecord{}, fmt.Errorf("can only hold %d more units in %s", free, w.CityName(city))
	}
	price := cost * added
	if err := w.payDirty(price); err != nil {
		return CutRecord{}, err
	}
	from := w.Quality(city, product)
	w.put(city, product, added)
	to := math.Min(from, from*float64(units)/float64(units+added)+math.Max(0, bonus))
	w.SetQuality(city, product, to)
	rec := CutRecord{City: city, Product: product, Units: units, Added: added, From: from, To: w.Quality(city, product), Cost: price, Chemist: chemist}
	w.Today.Cuts = append(w.Today.Cuts, rec)
	w.Stats.Cut += added
	w.Stats.CutCost += price
	return rec, nil
}

// CookOrder queues a chemist's lot (#47): units of a product cooked from
// precursors at cost a unit, dirty, paid now, landing in a city's stash
// days from now at the quality given (the chemist's today), at most
// batch units an order and one order a product a city a day. It is
// refused with no chemist (the caller says who), for a product the
// file gives no cook_cost, past the room the city has with what is
// already on its way there counted, and past the till. The crew sim
// lands it (crew.Sim.Step) and reports it.
func (w *World) CookOrder(city, product string, units, cost, days int, quality float64, batch int, chemist string) (Cook, error) {
	if w.Over != nil {
		return Cook{}, ErrGameOver
	}
	if w.Product(city, product) == nil {
		return Cook{}, ErrUnknownProduct
	}
	if chemist == "" {
		return Cook{}, ErrNoChemist
	}
	if cost <= 0 {
		return Cook{}, ErrNotCooked
	}
	if units <= 0 {
		return Cook{}, ErrBadQuantity
	}
	if units > batch {
		return Cook{}, fmt.Errorf("%w: %s cooks %d a batch", ErrBatch, chemist, batch)
	}
	for _, k := range w.Crew.Cooks {
		if k.City == city && k.Product == product && k.Ordered == w.Day {
			return Cook{}, ErrCooking
		}
	}
	if free := w.Free(city) - w.Crew.Cooking(city, product); units > free {
		return Cook{}, fmt.Errorf("can only hold %d more units in %s", max(0, free), w.CityName(city))
	}
	price := cost * units
	if err := w.payDirty(price); err != nil {
		return Cook{}, err
	}
	w.Crew.NextCook++
	k := Cook{ID: w.Crew.NextCook, City: city, Product: product, Units: units, Quality: math.Max(0, math.Min(100, quality)), Ordered: w.Day, Ready: w.Day + max(1, days), Cost: price, Chemist: chemist}
	w.Crew.Cooks = append(w.Crew.Cooks, k)
	w.Stats.Cooked += units
	w.Stats.CookCost += price
	return k, nil
}

// MigrateLots is the 12 -> 13 step (#47): a save from before quality
// carried units alone, so every lot holding anything, every shipment on
// the road and every connect's product is given the default quality,
// and every corner starts with all its customers coming back
// (repeat_start). The old save plays on as it did: the multipliers
// read 1 and no corner moves until something under the floor is sold.
func (w *World) MigrateLots(quality, repeat float64) {
	w.BaseQuality = quality
	for _, cid := range w.CityOrder {
		for id, q := range w.StashOf(cid) {
			if q > 0 && w.Player.Quality[cid][id] == 0 {
				w.SetQuality(cid, id, quality)
			}
		}
		for i := range w.Cities[cid].Corners {
			if c := &w.Cities[cid].Corners[i]; c.Repeat == 0 {
				c.Repeat = repeat
			}
		}
	}
	for i := range w.Shipments {
		if w.Shipments[i].Quality == 0 {
			w.Shipments[i].Quality = quality
		}
	}
	for i := range w.Suppliers {
		sup := &w.Suppliers[i]
		if sup.Quality == nil {
			sup.Quality = map[string]float64{}
		}
		for id := range sup.Price {
			if sup.Quality[id] == 0 {
				sup.Quality[id] = quality
			}
		}
	}
}

// TotalStock is every unit the operation holds: every street, every
// house and everything on the road.
func (w *World) TotalStock() int {
	n := w.Stashed()
	for _, s := range w.Shipments {
		n += s.Units
	}
	return n
}
