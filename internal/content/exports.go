package content

import "fmt"

// ExportsConfig mirrors exports.toml (#391): the lanes the cartel sells
// abroad on, and what every lane shares. A lane is demand the corners
// do not bound: a load is bought off the owned book (the supplier
// asset, at its own_ratio of street) and shipped straight out, never
// through a stash, and paid for on landing at a multiple of the street
// price in the lane's city. The logistics sim runs them.
type ExportsConfig struct {
	Exports ExportsTuning `toml:"exports"`
	Lanes   []LaneConfig  `toml:"lane"`
}

// ExportsTuning is what every lane shares: WatchedMul multiplies a
// lane's risk while the feds watch (HeatState.WatchUntil, the task
// force's cooldown), GlutFloor is the lowest a glutted price goes (a
// fraction of the lane's price), and RecordDays how long a landed or
// seized load stays on the ledger.
type ExportsTuning struct {
	WatchedMul float64 `toml:"watched_mul"`
	GlutFloor  float64 `toml:"glut_floor"`
	RecordDays int     `toml:"record_days"`
}

// LaneConfig is one lane abroad: its id and name, the city it leaves
// from, how (Mode, for the ledger and the port), the asset that opens
// it beside the book, the units one night's load carries at most
// (Capacity; the port multiplies a boat's out of its city), the days
// in transit, the price abroad as a multiple of the street price in
// City, the chance a load is seized, how much a full load gluts the
// price abroad (Glut, the fraction it takes off the next load's price)
// and how much of that comes back a day (Recover), and the products it
// takes.
type LaneConfig struct {
	ID       string   `toml:"id"`
	Name     string   `toml:"name"`
	City     string   `toml:"city"`
	Mode     string   `toml:"mode"` // boat, plane
	Asset    string   `toml:"asset"`
	Capacity int      `toml:"capacity"`
	Days     int      `toml:"days"`
	Price    float64  `toml:"price"`
	Risk     float64  `toml:"risk"`
	Glut     float64  `toml:"glut"`
	Recover  float64  `toml:"recover"`
	Products []string `toml:"products"`
}

// Takes reports whether the lane ships the product.
func (l LaneConfig) Takes(product string) bool {
	for _, p := range l.Products {
		if p == product {
			return true
		}
	}
	return false
}

// Lane returns the lane with id, or nil.
func (e ExportsConfig) Lane(id string) *LaneConfig {
	return find(e.Lanes, func(l *LaneConfig) bool { return l.ID == id })
}

// validate checks the lanes: ids unique, every lane in a city that
// exists, opened by an asset the assets file has, shipping products
// the market has, and its numbers in range.
func (e ExportsConfig) validate(city CityConfig, market MarketConfig, assets AssetsConfig) error {
	t := e.Exports
	if t.WatchedMul < 1 || t.GlutFloor <= 0 || t.GlutFloor > 1 || t.RecordDays < 0 {
		return fmt.Errorf("bad [exports] table %+v", t)
	}
	if len(e.Lanes) > 0 && assets.ByEffect(AssetSupplier) == nil {
		return fmt.Errorf("a lane needs the book, and assets.toml has no %s asset", AssetSupplier)
	}
	seen := map[string]bool{}
	for _, l := range e.Lanes {
		if l.ID == "" || l.Name == "" || seen[l.ID] {
			return fmt.Errorf("lane %q needs a unique id and a name", l.ID)
		}
		seen[l.ID] = true
		if city.City(l.City) == nil {
			return fmt.Errorf("lane %q: unknown city %q", l.ID, l.City)
		}
		if l.Mode != "boat" && l.Mode != "plane" {
			return fmt.Errorf("lane %q: mode %q (boat or plane)", l.ID, l.Mode)
		}
		if l.Asset != "" && assets.Asset(l.Asset) == nil {
			return fmt.Errorf("lane %q opens with asset %q, which assets.toml does not have", l.ID, l.Asset)
		}
		if l.Capacity <= 0 || l.Days <= 0 || l.Price <= 0 || l.Risk < 0 || l.Risk >= 1 || l.Glut < 0 || l.Glut >= 1 || l.Recover < 0 {
			return fmt.Errorf("lane %q: bad capacity, days, price, risk, glut or recover", l.ID)
		}
		if len(l.Products) == 0 {
			return fmt.Errorf("lane %q ships nothing", l.ID)
		}
		for _, p := range l.Products {
			if market.Product(p) == nil {
				return fmt.Errorf("lane %q: unknown product %q", l.ID, p)
			}
		}
	}
	return nil
}
