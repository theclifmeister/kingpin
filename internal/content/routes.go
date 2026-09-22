package content

import (
	"fmt"

	"github.com/theclifmeister/kingpin/internal/events"
)

// RoutesConfig mirrors routes.toml: the edges between cities, the ship
// dial and what a seizure does. How the wholesaler sells is the
// connect's row in suppliers.toml (#72).
type RoutesConfig struct {
	Shipping ShippingTuning `toml:"shipping"`
	Dial     ShipDialTable  `toml:"dial"`
	Routes   []RouteConfig  `toml:"route"`
}

// ShippingTuning is what a seizure does to the world: heat in both cities
// on the route, a page in the DA's file if the shipment was sent fast, and
// a supply shock on the product in the city it was bound for.
type ShippingTuning struct {
	SeizureHeat     float64 `toml:"seizure_heat"`
	SeizureEvidence int     `toml:"seizure_evidence"`
	ShockFactor     float64 `toml:"shock_factor"`
	ShockDays       int     `toml:"shock_days"`
	RecordDays      int     `toml:"record_days"` // how long a seizure stays on the ledger
}

type ShipDialTable struct {
	Slow   ShipDialConfig `toml:"slow"`
	Normal ShipDialConfig `toml:"normal"`
	Fast   ShipDialConfig `toml:"fast"`
}

// ShipDialConfig scales a route: Days multiplies the days in transit,
// Risk the chance of interception on each of them.
type ShipDialConfig struct {
	Days float64 `toml:"days"`
	Risk float64 `toml:"risk"`
}

// RouteConfig is one edge of the route graph, usable both ways. Days is
// the time in transit at the normal dial, Capacity the most one shipment
// carries, Cost what every unit costs to send (dirty cash), Risk the
// chance per day in transit that the shipment is intercepted.
type RouteConfig struct {
	ID       string  `toml:"id"`
	Name     string  `toml:"name"`
	Mode     string  `toml:"mode"` // car, truck, boat, plane, tunnel
	From     string  `toml:"from"`
	To       string  `toml:"to"`
	Days     int     `toml:"days"`
	Capacity int     `toml:"capacity"`
	Cost     int     `toml:"cost"`
	Risk     float64 `toml:"risk"`
	Asset    string  `toml:"asset"` // the asset that opens the route (#48: the airstrip's plane, the tunnel); "" is a route that is always there
}

// Connects reports whether the route joins the two cities, either way.
func (r RouteConfig) Connects(a, b string) bool {
	return (r.From == a && r.To == b) || (r.From == b && r.To == a)
}

// Other is the city at the far end of the route from city, or "".
func (r RouteConfig) Other(city string) string {
	switch city {
	case r.From:
		return r.To
	case r.To:
		return r.From
	}
	return ""
}

// Route returns the route with id, or nil.
func (r RoutesConfig) Route(id string) *RouteConfig {
	for i := range r.Routes {
		if r.Routes[i].ID == id {
			return &r.Routes[i]
		}
	}
	return nil
}

// DialFor returns the tuning for a ship dial position.
func (r RoutesConfig) DialFor(d events.Ship) ShipDialConfig {
	switch d {
	case events.ShipSlow:
		return r.Dial.Slow
	case events.ShipFast:
		return r.Dial.Fast
	default:
		return r.Dial.Normal
	}
}

// validate checks the routes join cities that exist, the numbers make
// sense, and the dials are usable.
func (r RoutesConfig) validate(cities CityConfig) error {
	seen := map[string]bool{}
	for _, rt := range r.Routes {
		if rt.ID == "" || seen[rt.ID] {
			return fmt.Errorf("route %q missing or defined twice", rt.ID)
		}
		seen[rt.ID] = true
		if cities.City(rt.From) == nil || cities.City(rt.To) == nil || rt.From == rt.To {
			return fmt.Errorf("route %q joins %q and %q", rt.ID, rt.From, rt.To)
		}
		if rt.Days < 1 || rt.Capacity < 1 || rt.Cost < 0 || rt.Risk < 0 || rt.Risk > 1 || rt.Mode == "" {
			return fmt.Errorf("bad route %+v", rt)
		}
	}
	for _, d := range []ShipDialConfig{r.Dial.Slow, r.Dial.Normal, r.Dial.Fast} {
		if d.Days <= 0 || d.Risk < 0 {
			return fmt.Errorf("bad ship dial %+v", d)
		}
	}
	if r.Shipping.ShockDays < 0 || r.Shipping.ShockFactor < 0 {
		return fmt.Errorf("bad [shipping] table %+v", r.Shipping)
	}
	return nil
}

// validateAssets checks every route that names an asset names one the
// assets file has (#48).
func (r RoutesConfig) validateAssets(assets AssetsConfig) error {
	for _, rt := range r.Routes {
		if rt.Asset != "" && assets.Asset(rt.Asset) == nil {
			return fmt.Errorf("route %q opens with asset %q, which assets.toml does not have", rt.ID, rt.Asset)
		}
	}
	return nil
}
