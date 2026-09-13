package content

import "fmt"

// AssetsConfig mirrors assets.toml (#48): the tier-5 assets, one row
// each, and the tuning every one shares. An asset is a clean-cash
// purchase on the supply side, the way a front is on the money side:
// bought through one action (World.BuyAsset), owned as state
// (World.Assets), read by the sim that owns the number it moves through
// its own copy of this file, as the fronts and the tree are.
type AssetsConfig struct {
	Assets AssetsTuning  `toml:"assets"`
	Offers []AssetConfig `toml:"asset"`
}

// AssetsTuning is what every asset shares: UpkeepFreezeDays is how long
// an asset stands idle when its upkeep goes unpaid (a front's rule).
type AssetsTuning struct {
	UpkeepFreezeDays int `toml:"upkeep_freeze_days"`
}

// The effects an asset can have: exactly one a row.
const (
	AssetSupplier = "supplier" // the wholesale connect in City sells at OwnRatio of street, unlimited, no buy pressure
	AssetPort     = "port"     // boat routes through City at zero customs risk and CapacityMul the capacity
	AssetAirstrip = "airstrip" // the plane route (routes.toml, asset = "airstrip") is open
	AssetLab      = "lab"      // the chemist's cook in City at LabMul the batch, LabQuality, precursors at LabCostMul
	AssetTunnel   = "tunnel"   // the tunnel route (routes.toml, asset = "tunnel") is open until it is found
)

// AssetEffects is every effect, for validate.
var AssetEffects = []string{AssetSupplier, AssetPort, AssetAirstrip, AssetLab, AssetTunnel}

// AssetConfig is one asset: its id and name, the city it is in (the
// one its pressure lands in and its effect works in), the peak clean
// cash that puts it on offer (UnlockCash: the wash is the clock), its
// price and daily upkeep in clean cash, the heat floor it puts under
// every city (federal attention: the heat sim takes the highest owned),
// the pressure it adds in its city every day, and its one effect with
// the numbers that effect reads.
type AssetConfig struct {
	ID         string  `toml:"id"`
	Name       string  `toml:"name"`
	City       string  `toml:"city"`
	UnlockCash int     `toml:"unlock_cash"` // peak clean cash
	Cost       int     `toml:"cost"`        // clean cash
	Upkeep     int     `toml:"upkeep"`      // clean cash a day
	HeatFloor  float64 `toml:"heat_floor"`
	Pressure   float64 `toml:"pressure"` // a day, in City
	Effect     string  `toml:"effect"`

	OwnRatio    float64 `toml:"own_ratio"`    // supplier: the connect's price as a fraction of street
	CapacityMul float64 `toml:"capacity_mul"` // port: on every boat route through City
	LabMul      float64 `toml:"lab_mul"`      // lab: on the chemist's batch
	LabQuality  float64 `toml:"lab_quality"`  // lab: what a cook there lands at, if over the chemist's
	LabCostMul  float64 `toml:"lab_cost_mul"` // lab: on the precursors' cost a unit
}

// Asset returns the row with id, or nil.
func (a AssetsConfig) Asset(id string) *AssetConfig {
	for i := range a.Offers {
		if a.Offers[i].ID == id {
			return &a.Offers[i]
		}
	}
	return nil
}

// ByEffect returns the row with the effect, or nil: one asset an
// effect, so a sim asks for the one it reads.
func (a AssetsConfig) ByEffect(effect string) *AssetConfig {
	for i := range a.Offers {
		if a.Offers[i].Effect == effect {
			return &a.Offers[i]
		}
	}
	return nil
}

// validate checks the offers: ids and effects unique and known, every
// asset in a city that exists, the money not negative, the cost
// positive, the floor on the scale, and the effect's numbers in range.
func (a AssetsConfig) validate(city CityConfig) error {
	if a.Assets.UpkeepFreezeDays < 0 {
		return fmt.Errorf("bad [assets] table %+v", a.Assets)
	}
	seen, effects := map[string]bool{}, map[string]bool{}
	for _, o := range a.Offers {
		if o.ID == "" || o.Name == "" || seen[o.ID] {
			return fmt.Errorf("asset %q needs a unique id and a name", o.ID)
		}
		seen[o.ID] = true
		if city.City(o.City) == nil {
			return fmt.Errorf("asset %q: unknown city %q", o.ID, o.City)
		}
		if o.Cost <= 0 || o.Upkeep < 0 || o.UnlockCash < 0 || o.HeatFloor < 0 || o.HeatFloor > 100 || o.Pressure < 0 {
			return fmt.Errorf("asset %q: bad cost, upkeep, unlock_cash, heat_floor or pressure", o.ID)
		}
		known := false
		for _, e := range AssetEffects {
			known = known || e == o.Effect
		}
		if !known {
			return fmt.Errorf("asset %q: effect %q is not one of %v", o.ID, o.Effect, AssetEffects)
		}
		if effects[o.Effect] {
			return fmt.Errorf("asset %q: a second %s", o.ID, o.Effect)
		}
		effects[o.Effect] = true
		switch o.Effect {
		case AssetSupplier:
			if o.OwnRatio <= 0 || o.OwnRatio >= 1 {
				return fmt.Errorf("asset %q: own_ratio %v (0 up to 1)", o.ID, o.OwnRatio)
			}
		case AssetPort:
			if o.CapacityMul < 1 {
				return fmt.Errorf("asset %q: capacity_mul %v (at least 1)", o.ID, o.CapacityMul)
			}
		case AssetLab:
			if o.LabMul < 1 || o.LabQuality < 0 || o.LabQuality > 100 || o.LabCostMul <= 0 || o.LabCostMul > 1 {
				return fmt.Errorf("asset %q: bad lab_mul, lab_quality or lab_cost_mul", o.ID)
			}
		}
	}
	return nil
}
