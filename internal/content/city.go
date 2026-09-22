package content

import "fmt"

// CityConfig mirrors city.toml: the cities, each with its own corners and
// its own take on every product, and how corners are held. The first city
// is home: where a run starts and where the rival sets up.
type CityConfig struct {
	Territory TerritoryTuning `toml:"territory"`
	Deed      DeedTuning      `toml:"deed"`
	Tax       TaxTuning       `toml:"tax"`
	Cities    []CityEntry     `toml:"city"`
}

// TaxTuning is city.toml [tax] (#231): once you hold more than Share of
// a city's corners (and MinHeld at least), every corner nobody holds
// there is worked by independents who pay you Cut of its trade a night
// in dirty cash. Cut at 0 boxes it.
type TaxTuning struct {
	Share   float64 `toml:"share"`
	Cut     float64 `toml:"cut"`
	MinHeld int     `toml:"min_held"`
}

// On reports whether the tax is in the file.
func (t TaxTuning) On() bool { return t.Cut > 0 && t.Share > 0 }

// DeedTuning mirrors city.toml [deed] (#194): what buying the block a
// corner is on costs and does. The price is Days of the corner's street
// trade at purchase (World.CornerTrade); Rent is the share of the price
// the block pays back a day, clean; RobberyMul, PushMul and RaidMul are
// what the deed does to the robbery chance on the block, the rival's
// push on it (and its defence of it, where the block is theirs) and a
// house's weight in the raid's roll, each read by the sim that owns the
// number; Pressure is what a deed adds to its city's pressure a day;
// HeadlineDeeds is the count in a city from which a purchase makes the
// paper; ForfeitRatio is the multiple of Stats.Laundered the deeds held
// may cost before the DA seizes the newest, and ForfeitEvidence the
// pages that files the morning after. Days of 0 puts no deed on sale.
type DeedTuning struct {
	Days            float64 `toml:"days"`
	Rent            float64 `toml:"rent"`
	RobberyMul      float64 `toml:"robbery_mul"`
	PushMul         float64 `toml:"push_mul"`
	RaidMul         float64 `toml:"raid_mul"`
	Pressure        float64 `toml:"pressure"`
	HeadlineDeeds   int     `toml:"headline_deeds"`
	ForfeitRatio    float64 `toml:"forfeit_ratio"`
	ForfeitEvidence int     `toml:"forfeit_evidence"`
}

// On reports whether deeds are for sale: Days in the file.
func (d DeedTuning) On() bool { return d.Days > 0 }

// validate: a deed slows the rival and never stops it (push_mul is
// never zero while deeds are on sale), and nothing else is negative.
func (d DeedTuning) validate() error {
	if !d.On() {
		return nil
	}
	if d.PushMul <= 0 {
		return fmt.Errorf("[deed] push_mul must be over 0: a deed slows the rival and never stops it")
	}
	if d.Rent < 0 || d.RobberyMul < 0 || d.RaidMul < 0 || d.Pressure < 0 || d.HeadlineDeeds < 0 || d.ForfeitRatio < 0 || d.ForfeitEvidence < 0 {
		return fmt.Errorf("[deed] has a negative number")
	}
	return nil
}

// CityEntry is one city. Heat multiplies the sale heat of every unit
// moved there (a port town's police have other things to look at);
// Wholesale says a connect there sells by the lot (suppliers.toml);
// Market is how its street differs from the product ladder, per product.
type CityEntry struct {
	ID        string                 `toml:"id"`
	Name      string                 `toml:"name"`
	Heat      float64                `toml:"heat"`
	Wholesale bool                   `toml:"wholesale"`
	Market    map[string]CityProduct `toml:"market"`
	Corners   []CornerConfig         `toml:"corner"`
}

// CityProduct is a city's multipliers on a product's base price and
// per-corner demand; a product the city does not list is at 1 and 1.
// NoSupply says the city's supplier does not sell it (#60: designer is
// the port's product; the road is the only way it reaches home).
type CityProduct struct {
	Price    float64 `toml:"price"`
	Demand   float64 `toml:"demand"`
	NoSupply bool    `toml:"no_supply"`
}

// Product returns the city's multipliers for a product, 1 and 1 when it
// has no view of it.
func (c CityEntry) Product(id string) CityProduct {
	if p, ok := c.Market[id]; ok {
		if p.Price <= 0 {
			p.Price = 1
		}
		if p.Demand <= 0 {
			p.Demand = 1
		}
		return p
	}
	return CityProduct{Price: 1, Demand: 1}
}

// HeatMul is the city's sale-heat multiplier, 1 when unset.
func (c CityEntry) HeatMul() float64 {
	if c.Heat <= 0 {
		return 1
	}
	return c.Heat
}

// Corner returns the city's corner with id, or nil.
func (c CityEntry) Corner(id string) *CornerConfig {
	for i := range c.Corners {
		if c.Corners[i].ID == id {
			return &c.Corners[i]
		}
	}
	return nil
}

type TerritoryTuning struct {
	Start         string  `toml:"start"`
	DriftDays     int     `toml:"drift_days"`
	RobberyChance float64 `toml:"robbery_chance"`
	RobberyStock  float64 `toml:"robbery_stock"`
	RobberyCash   float64 `toml:"robbery_cash"`
	EnforcerCut   float64 `toml:"enforcer_cut"`
}

type CornerConfig struct {
	ID     string             `toml:"id"`
	Name   string             `toml:"name"`
	X      int                `toml:"x"` // map cell
	Y      int                `toml:"y"`
	Demand float64            `toml:"demand"` // size relative to one standard corner
	Heat   float64            `toml:"heat"`   // sale-heat multiplier for units moved here
	Risk   float64            `toml:"risk"`   // robbery-chance multiplier
	Taste  map[string]float64 `toml:"taste"`  // per-product demand multiplier; missing = 1
}

// Corner returns the config for a corner id in any city, or nil.
func (c CityConfig) Corner(id string) *CornerConfig {
	for i := range c.Cities {
		if k := c.Cities[i].Corner(id); k != nil {
			return k
		}
	}
	return nil
}

// City returns the city with id, or nil.
func (c CityConfig) City(id string) *CityEntry {
	for i := range c.Cities {
		if c.Cities[i].ID == id {
			return &c.Cities[i]
		}
	}
	return nil
}

// Home is the first city: where a run starts.
func (c CityConfig) Home() CityEntry { return c.Cities[0] }

// Corners lists every corner of every city, in file order.
func (c CityConfig) Corners() []CornerConfig {
	var out []CornerConfig
	for _, city := range c.Cities {
		out = append(out, city.Corners...)
	}
	return out
}

// validate checks the cities read as a map: at least one, ids unique,
// every one with corners, corner ids unique across the lot, and the start
// corner in the home city.
func (c CityConfig) validate() error {
	if len(c.Cities) == 0 {
		return fmt.Errorf("no cities defined")
	}
	cities := map[string]bool{}
	corners := map[string]bool{}
	for _, city := range c.Cities {
		if city.ID == "" || city.Name == "" {
			return fmt.Errorf("city %q needs an id and a name", city.ID)
		}
		if cities[city.ID] {
			return fmt.Errorf("city %q is defined twice", city.ID)
		}
		cities[city.ID] = true
		if len(city.Corners) == 0 {
			return fmt.Errorf("city %q has no corners", city.ID)
		}
		for _, k := range city.Corners {
			if k.ID == "" || corners[k.ID] {
				return fmt.Errorf("city %q: corner %q missing or defined twice", city.ID, k.ID)
			}
			corners[k.ID] = true
		}
	}
	if c.Home().Corner(c.Territory.Start) == nil {
		return fmt.Errorf("start corner %q is not in %s", c.Territory.Start, c.Home().ID)
	}
	return c.Deed.validate()
}
