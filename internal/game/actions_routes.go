package game

import (
	"math"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
)

// RouteSetting is the player's standing instruction for one route (#61):
// the dial it runs at and the stock the far city is kept at, per product,
// as units (Target) or as days of the far city's demand (Days, #115: the
// logistics sim sizes the units each morning from what the corners there
// serve, so the target follows the ground held). A product has one or
// the other: SetRouteDays clears the units and SetRouteTarget the days.
// The logistics sim reads it every day; the zero value is a route that is
// off, so a save from before the dial replays as it did, and a save from
// before the days target loads with Days nil, no schema bump.
type RouteSetting struct {
	Dial        events.RouteDial
	Target      map[string]int // product id -> units the route's destination is kept stocked to
	Days        map[string]int // product id -> days of the destination's demand it is kept stocked to
	ClosedUntil int            // the route is shut on every tick before this day (#44, an incident): nothing moves on it and nothing new is sent; 0 is open
	Bought      int            // the day the edge's checkpoint or customs deal runs out (#42; 0: none); World.Checkpoint says whether it is live
	Driver      int            // the crew member who rides every shipment on it (#46), 0 for nobody; persistent like the dial. World.RouteDriver says whether they can today
}

// Closed reports whether the route is shut on the tick that brings day:
// the logistics sim asks with the tick's day, the UI with tomorrow's.
func (rs RouteSetting) Closed(day int) bool { return day < rs.ClosedUntil }

// HasTargets is whether the route keeps anything anywhere: a route with
// none sends nothing however its dial stands.
func (rs RouteSetting) HasTargets() bool { return len(rs.Target) > 0 || len(rs.Days) > 0 }

// Route returns the setting for a route, off with no targets if it has
// never been set.
func (w *World) Route(id string) RouteSetting {
	if rs, ok := w.Routes[id]; ok {
		return rs
	}
	return RouteSetting{}
}

// SetRoute turns a route's dial. It is a persistent setting like the pay
// dial, not per-day scratch: the logistics sim runs the route at it every
// day until it is turned again. The targets are kept when it is turned
// off, so turning it back on picks up where it left off.
func (w *World) SetRoute(id string, d events.RouteDial) error {
	if w.Over != nil {
		return ErrGameOver
	}
	if id == "" {
		return ErrNoRoute
	}
	if d < events.RouteOff || d > events.RouteFast {
		return ErrBadDial
	}
	if w.Routes == nil {
		w.Routes = map[string]RouteSetting{}
	}
	rs := w.Routes[id]
	rs.Dial = d
	w.Routes[id] = rs
	return nil
}

// SetRouteTarget sets the stock a route keeps its destination at for a
// product, in units; zero clears it. The route sends the shortfall
// against it, buying by the lot at the source where a wholesaler deals.
// A days target for the product (SetRouteDays) is cleared: it has one or
// the other.
func (w *World) SetRouteTarget(id, product string, units int) error {
	return w.setRouteTarget(id, product, units, false)
}

// SetRouteDays sets the stock a route keeps its destination at for a
// product as days of that city's demand (#115): the logistics sim reads
// it each morning as days times World.Demand there, rounded up, so the
// target follows the corners held without being set again. Zero clears
// it, and a units target for the product goes with it.
func (w *World) SetRouteDays(id, product string, days int) error {
	return w.setRouteTarget(id, product, days, true)
}

// setRouteTarget is SetRouteTarget and SetRouteDays: the number goes in
// the one map and comes out of the other, and a map left empty is nil
// again so a cleared setting is the zero value it was.
func (w *World) setRouteTarget(id, product string, n int, days bool) error {
	if w.Over != nil {
		return ErrGameOver
	}
	if id == "" {
		return ErrNoRoute
	}
	if w.Home() == nil || w.Home().Market[product] == nil {
		return ErrUnknownProduct
	}
	if n < 0 {
		return ErrBadQuantity
	}
	if w.Routes == nil {
		w.Routes = map[string]RouteSetting{}
	}
	rs := w.Routes[id]
	set, other := &rs.Target, &rs.Days
	if days {
		set, other = &rs.Days, &rs.Target
	}
	delete(*other, product)
	if len(*other) == 0 {
		*other = nil
	}
	if n == 0 {
		delete(*set, product)
		if len(*set) == 0 {
			*set = nil
		}
	} else {
		if *set == nil {
			*set = map[string]int{}
		}
		(*set)[product] = n
	}
	w.Routes[id] = rs
	return nil
}

// Send is the logistics sim putting a shipment on the road (#61): the
// units leave the source stash and the fare leaves dirty cash now, and
// the sim lands or loses it. It is not a player action; the route dial
// is. The sim fills in the route, the days and the fare; the world gives
// it an id and keeps the count.
func (w *World) Send(s Shipment) Shipment {
	w.Player.DirtyCash -= s.Cost
	if s.Quality <= 0 {
		s.Quality = w.Quality(s.From, s.Product) // the lot carries the stash's quality (#47)
	}
	w.TakeStock(s.From, s.Product, s.Units)
	w.Logistics.NextID++
	s.ID = w.Logistics.NextID
	if s.Arrives <= s.Sent {
		s.Arrives = s.Sent + 1
	}
	w.Shipments = append(w.Shipments, s)
	w.Stats.Shipments++
	w.Stats.Shipped += s.Units
	return s
}

// SetRouteDriver puts the driver with id on a route (#46): they ride
// every shipment it sends from tomorrow, taking their cut off its risk,
// until they are moved or fired; 0 takes the route's driver off it. A
// driver rides one route: putting them on another takes them off the
// first. The setting persists like the dial.
func (w *World) SetRouteDriver(route string, id int) error {
	if w.Over != nil {
		return ErrGameOver
	}
	if route == "" {
		return ErrNoRoute
	}
	if id != 0 {
		m := w.Crew.Member(id)
		if m == nil {
			return ErrNoMember
		}
		if m.Role != RoleDriver {
			return ErrNotDriver
		}
		w.unseat(id)
	}
	if w.Routes == nil {
		w.Routes = map[string]RouteSetting{}
	}
	rs := w.Routes[route]
	rs.Driver = id
	w.Routes[route] = rs
	return nil
}

// unseat takes a member off whatever route they drive.
func (w *World) unseat(id int) {
	for rid, rs := range w.Routes {
		if rs.Driver == id {
			rs.Driver = 0
			w.Routes[rid] = rs
		}
	}
}

// RouteDriver is the driver riding a route's shipments on day, or nil:
// the route's driver if they are on the payroll and fit (a jailed or
// wounded one rides nothing, and the route runs without them).
func (w *World) RouteDriver(route string, day int) *CrewMember {
	m := w.Crew.Driver(w.Route(route).Driver)
	if m == nil || !m.Fit(day) {
		return nil
	}
	return m
}

// DrivenRoute is the route the member drives, "" for none.
func (w *World) DrivenRoute(id int) string {
	if id == 0 {
		return ""
	}
	for rid, rs := range w.Routes {
		if rs.Driver == id {
			return rid
		}
	}
	return ""
}

// RouteTarget is the units a route keeps its destination at for a
// product today: the units target as set, or a days target (#115) read
// as days times World.Demand at the far end this morning, rounded up
// (DaysTarget); no dice. Zero for a route with no target for the
// product. The logistics sim ships to it, and a lieutenant in the
// route's source city keeps back what it owes (#497).
func (w *World) RouteTarget(r content.RouteConfig, product string) int {
	rs := w.Route(r.ID)
	if d := rs.Days[product]; d > 0 {
		return w.DaysTarget(r.To, product, d)
	}
	return max(0, rs.Target[product])
}

// DaysTarget is the units that many days of a city's demand for a
// product are this morning, rounded up.
func (w *World) DaysTarget(city, product string, days int) int {
	if days <= 0 {
		return 0
	}
	// A hair under the product, so a share that multiplies to a whole
	// number is that number and not the one over it.
	return int(math.Ceil(float64(days)*w.Demand(city, product) - 1e-9))
}

// RouteShortfall is how many units of a product a route owes its
// destination today: the target less what is stashed there and what is
// already on the road to it.
func (w *World) RouteShortfall(r content.RouteConfig, product string) int {
	target := w.RouteTarget(r, product)
	if target <= 0 {
		return 0
	}
	return max(0, target-w.Stock(r.To, product)-w.Bound(r.To, product))
}
