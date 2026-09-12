package content

import "fmt"

// IncidentsConfig mirrors incidents.toml (#44): the table's pacing and
// the incidents, one [[incident]] a row. The world sim deals from it
// first thing every day; a config with an empty table (the harness's,
// cmd/balance -incidents off) deals nothing and draws nothing.
type IncidentsConfig struct {
	Incidents IncidentsTuning  `toml:"incidents"`
	Table     []IncidentConfig `toml:"incident"`
}

// IncidentsTuning paces the table the way the deck is paced: none for
// MinGap days after the last incident, then a chance rising each day
// until one is certain at MaxGap, if any is eligible.
type IncidentsTuning struct {
	MinGap int `toml:"min_gap"`
	MaxGap int `toml:"max_gap"`
}

// IncidentConfig is one row of the table: when it can fire (Weight is
// its share of the pick, MinDay the first day it can, MinGap its own
// cooldown in days since it last fired, Once retires it after one), the
// trigger every set field of which must hold (CardTrigger, the deck's
// vocabulary: city names the city it lands in, else the home city),
// RouteMode the mode of the routes a closure shuts (boat, truck, car;
// the first of them fills the Route slot), Names the pool the Name slot
// is drawn from (names.toml: celebrities, reporters), the report's
// line (a text/template over the news sim's slots; the headline is
// headlines.toml's under Key, so it has variants like every other) and
// what it does to the world.
type IncidentConfig struct {
	ID        string          `toml:"id"`
	Name      string          `toml:"name"`
	Weight    float64         `toml:"weight"`
	MinDay    int             `toml:"min_day"`
	MinGap    int             `toml:"min_gap"`
	Once      bool            `toml:"once"`
	RouteMode string          `toml:"route_mode"`
	Names     string          `toml:"names"`
	Report    string          `toml:"report"`
	Trigger   CardTrigger     `toml:"trigger"`
	Effects   IncidentEffects `toml:"effects"`
}

// IncidentEffects is the fixed key set an incident may carry (#44): a
// key the world does not apply is an unknown key in the file and fails
// at start-up (decodeBytes refuses undecoded keys in incidents.toml).
// Every effect writes state a sim already reads and nothing into a sim:
// RouteClosed shuts the routes of the row's RouteMode for that many
// nights (RouteSetting.ClosedUntil, the logistics sim reads it);
// MarketShock multiplies a product's price target in the incident's
// city for Days (the market's own shock state, PriceShock's shape) and
// DemandShift its demand (the slump's shape); Pressure and Heat are
// deltas on the incident's city, 0..100; Notoriety on the player's
// axis; HeatDecay multiplies the heat sim's decay for Days
// (HeatState.FederalUntil); ChiefReplaced and ElectionCalled ride the
// Incident event, which the law sim honours the same tick (a new chief
// at once, the election ElectionCalled days out). rival_leader_killed
// waits for #43's fragmentation and is not in the set.
type IncidentEffects struct {
	RouteClosed    int          `toml:"route_closed"`
	MarketShock    ProductShock `toml:"market_shock"`
	DemandShift    ProductShock `toml:"demand_shift"`
	Pressure       float64      `toml:"pressure"`
	Heat           float64      `toml:"heat"`
	Notoriety      float64      `toml:"notoriety"`
	HeatDecay      TimedMul     `toml:"heat_decay"`
	ChiefReplaced  bool         `toml:"chief_replaced"`
	ElectionCalled int          `toml:"election_called"`
}

// ProductShock is a product's price target (market_shock) or demand
// (demand_shift) multiplied by Mul for Days in the incident's city.
type ProductShock struct {
	Product string  `toml:"product"`
	Mul     float64 `toml:"mul"`
	Days    int     `toml:"days"`
}

// TimedMul is a multiplier that holds for Days.
type TimedMul struct {
	Mul  float64 `toml:"mul"`
	Days int     `toml:"days"`
}

// Set reports whether the shock does anything.
func (p ProductShock) Set() bool { return p.Product != "" && p.Days > 0 && p.Mul > 0 }

// Set reports whether the multiplier does anything.
func (t TimedMul) Set() bool { return t.Days > 0 && t.Mul > 0 }

// Key is the headline template key for the row: `Incident` and the id
// in camel case, `IncidentPortStrike` for port_strike, the way an
// enforcement's is `Enforcement` and its level.
func (i IncidentConfig) Key() string {
	out := []byte("Incident")
	up := true
	for _, c := range []byte(i.ID) {
		switch {
		case c == '_':
			up = true
		case up && c >= 'a' && c <= 'z':
			out = append(out, c-'a'+'A')
			up = false
		default:
			out = append(out, c)
			up = false
		}
	}
	return string(out)
}

// Incident returns the row with id, or nil.
func (c IncidentsConfig) Incident(id string) *IncidentConfig {
	for i := range c.Table {
		if c.Table[i].ID == id {
			return &c.Table[i]
		}
	}
	return nil
}

// validate checks the table reads as a table: ids unique, every row
// with a name and a report, the pacing sane, every product,
// city, route mode and name pool one the files know, and every row
// doing something.
func (c IncidentsConfig) validate(city CityConfig, market MarketConfig, routes RoutesConfig, names NamesConfig) error {
	if len(c.Table) > 0 && (c.Incidents.MinGap < 1 || c.Incidents.MaxGap < c.Incidents.MinGap) {
		return fmt.Errorf("min_gap %d and max_gap %d must be 1 <= min <= max", c.Incidents.MinGap, c.Incidents.MaxGap)
	}
	seen := map[string]bool{}
	for _, inc := range c.Table {
		if inc.ID == "" || inc.Name == "" || inc.Report == "" {
			return fmt.Errorf("incident %q needs an id, a name and a report", inc.ID)
		}
		if seen[inc.ID] {
			return fmt.Errorf("incident %q is defined twice", inc.ID)
		}
		seen[inc.ID] = true
		if inc.Weight < 0 || inc.MinDay < 0 || inc.MinGap < 0 {
			return fmt.Errorf("incident %q: weight, min_day and min_gap cannot be negative", inc.ID)
		}
		if inc.Trigger.City != "" && city.City(inc.Trigger.City) == nil {
			return fmt.Errorf("incident %q: unknown city %q", inc.ID, inc.Trigger.City)
		}
		if inc.RouteMode != "" {
			found := false
			for _, r := range routes.Routes {
				found = found || r.Mode == inc.RouteMode
			}
			if !found {
				return fmt.Errorf("incident %q: no route of mode %q", inc.ID, inc.RouteMode)
			}
		}
		if inc.Names != "" && len(names.Pool(inc.Names)) == 0 {
			return fmt.Errorf("incident %q: no name pool %q in names.toml", inc.ID, inc.Names)
		}
		e := inc.Effects
		if e.RouteClosed > 0 && inc.RouteMode == "" {
			return fmt.Errorf("incident %q: route_closed needs a route_mode", inc.ID)
		}
		for _, sh := range []ProductShock{e.MarketShock, e.DemandShift} {
			if sh.Product == "" && sh.Days == 0 && sh.Mul == 0 {
				continue
			}
			if !sh.Set() {
				return fmt.Errorf("incident %q: a shock needs a product, a mul over 0 and days", inc.ID)
			}
			if market.Product(sh.Product) == nil {
				return fmt.Errorf("incident %q: unknown product %q", inc.ID, sh.Product)
			}
		}
		if (e.HeatDecay.Days != 0 || e.HeatDecay.Mul != 0) && !e.HeatDecay.Set() {
			return fmt.Errorf("incident %q: heat_decay needs a mul over 0 and days", inc.ID)
		}
		if e.RouteClosed < 0 || e.ElectionCalled < 0 {
			return fmt.Errorf("incident %q: days cannot be negative", inc.ID)
		}
		if e == (IncidentEffects{}) {
			return fmt.Errorf("incident %q has no effects", inc.ID)
		}
	}
	return nil
}
