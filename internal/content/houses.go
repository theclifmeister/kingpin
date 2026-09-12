package content

import "fmt"

// HousesConfig mirrors houses.toml (#73): the tuning every house shares
// and the offers, one a block.
type HousesConfig struct {
	Houses HousesTuning  `toml:"houses"`
	Offers []HouseConfig `toml:"house"`
}

// HousesTuning is what every house shares: HouseRisk multiplies the
// city's robbery chance times the block's risk for a house there a day
// (the territory sim rolls it, a guard cutting it as an enforcer cuts a
// corner's), RentDays is how long the rent can go unpaid before the
// landlord throws you out, and MoveHeat is what a unit moved between
// places draws relative to one sold on a standard corner (the heat sim
// reads it).
type HousesTuning struct {
	HouseRisk float64 `toml:"house_risk"`
	RentDays  int     `toml:"rent_days"`
	MoveHeat  float64 `toml:"move_heat"`
}

// HouseConfig is one offer: a place in a city on a block (the corner
// whose heat and risk it takes), its capacity in units, its price in
// dirty cash, its rent in clean cash a day, and the peak cash that puts
// it on offer.
type HouseConfig struct {
	ID         string `toml:"id"`
	Name       string `toml:"name"`
	City       string `toml:"city"`
	Corner     string `toml:"corner"`
	Capacity   int    `toml:"capacity"`
	Price      int    `toml:"price"`
	Rent       int    `toml:"rent"`
	UnlockCash int    `toml:"unlock_cash"`
}

// House returns the offer with id, or nil.
func (h HousesConfig) House(id string) *HouseConfig {
	for i := range h.Offers {
		if h.Offers[i].ID == id {
			return &h.Offers[i]
		}
	}
	return nil
}

// validate checks the offers against the cities: every house on a
// block of its city, ids unique, capacity and price positive, rent not
// negative, and the tuning in range.
func (h HousesConfig) validate(city CityConfig) error {
	if h.Houses.HouseRisk < 0 || h.Houses.RentDays <= 0 || h.Houses.MoveHeat < 0 {
		return fmt.Errorf("bad [houses] table %+v", h.Houses)
	}
	seen := map[string]bool{}
	for _, o := range h.Offers {
		if o.ID == "" || o.Name == "" || seen[o.ID] {
			return fmt.Errorf("house %q needs a unique id and a name", o.ID)
		}
		seen[o.ID] = true
		if o.Capacity <= 0 || o.Price <= 0 || o.Rent < 0 || o.UnlockCash < 0 {
			return fmt.Errorf("house %q: bad capacity, price, rent or unlock_cash", o.ID)
		}
		c := city.City(o.City)
		if c == nil {
			return fmt.Errorf("house %q: unknown city %q", o.ID, o.City)
		}
		found := false
		for _, k := range c.Corners {
			found = found || k.ID == o.Corner
		}
		if !found {
			return fmt.Errorf("house %q: no corner %q in %s", o.ID, o.Corner, o.City)
		}
	}
	return nil
}
