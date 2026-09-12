package harness

import (
	"sort"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/game"
)

// Stashed plays like Laundered (the fronts are what pay the rent) and
// keeps its stock in stash houses (#73): in the city it stands in it
// takes the cheapest house on offer it lacks once dirty cash is
// HouseMargin times the price and clean cash covers RentCover days of
// the rent, a second (up to houses of them; 0 is StashHouses) once one
// there is over HouseFull of its capacity, and after the day's buying
// moves stock between its houses so none holds more than Spread of what
// it keeps in the city. With fronts false it is Crewed with the houses:
// whatever clean cash it started with is all it has, the rent runs it
// out and the landlord throws it out, which is the pull the rent is for.
func Stashed(cfg *content.Config, lieLowAt float64, houses int, fronts bool) Policy {
	base := Laundered(cfg, lieLowAt)
	if !fronts {
		base = Crewed(cfg, lieLowAt)
	}
	if houses <= 0 {
		houses = StashHouses
	}
	return func(w *game.World) {
		base(w)
		house(cfg, w, houses)
		spread(w)
	}
}

// The stashed policy's numbers: how many houses it keeps a city, the
// margin over the price it buys at, how full a house is before it
// wants another, and the most of a city's stock it leaves in one.
const (
	StashHouses = 3
	HouseMargin = 3.0
	RentCover   = 30
	HouseFull   = 0.8
	Spread      = 0.6
)

// house buys the stashed policy's houses in the city the player stands
// in: the first once it can, the next once one is over HouseFull.
func house(cfg *content.Config, w *game.World, max int) {
	city := w.Player.Location
	have := w.HousesIn(city)
	if len(have) >= max {
		return
	}
	if len(have) > 0 {
		full := false
		for _, h := range have {
			full = full || float64(h.Units()) > HouseFull*float64(h.Capacity)
		}
		if !full {
			return
		}
	}
	offers := HouseOffers(cfg, city)
	sort.SliceStable(offers, func(i, j int) bool { return offers[i].Price < offers[j].Price })
	for _, o := range offers {
		if w.House(o.ID) != nil || o.Locked(w) {
			continue
		}
		if float64(w.Player.DirtyCash) >= HouseMargin*float64(o.Price) && w.Player.CleanCash >= RentCover*o.Rent {
			_, _ = w.BuyHouse(o)
		}
		return // the cheapest one it lacks, or nothing
	}
}

// HouseOffers is houses.toml's offers for a city as BuyHouse takes them.
func HouseOffers(cfg *content.Config, city string) []game.HouseOffer {
	var out []game.HouseOffer
	for _, o := range cfg.Houses.Offers {
		if o.City == city {
			out = append(out, game.HouseOffer{ID: o.ID, Name: o.Name, City: o.City, Corner: o.Corner, Capacity: o.Capacity, Price: o.Price, Rent: o.Rent, UnlockCash: o.UnlockCash})
		}
	}
	return out
}

// spread moves stock between the houses in the player's city until none
// holds more than Spread of what the city keeps, product by product,
// out of the fullest house into the one with the most room. A city with
// one house has nothing to spread.
func spread(w *game.World) {
	city := w.Player.Location
	if len(w.HousesIn(city)) < 2 {
		return
	}
	total := w.StockIn(city)
	if total == 0 {
		return
	}
	for guard := 0; guard < 8; guard++ {
		var fullest, roomiest *game.House
		for i := range w.Houses {
			h := &w.Houses[i]
			if h.City != city {
				continue
			}
			if fullest == nil || h.Units() > fullest.Units() {
				fullest = h
			}
			if roomiest == nil || h.Room() > roomiest.Room() {
				roomiest = h
			}
		}
		excess := fullest.Units() - int(Spread*float64(total))
		if excess <= 0 || roomiest == fullest || roomiest.Room() == 0 {
			return
		}
		moved := 0
		for _, id := range w.Products {
			if excess-moved <= 0 {
				break
			}
			n := min(fullest.Stock[id], excess-moved, roomiest.Room())
			if n <= 0 {
				continue
			}
			got, _ := w.Move(city, fullest.ID, roomiest.ID, id, n)
			moved += got
		}
		if moved == 0 {
			return
		}
	}
}
