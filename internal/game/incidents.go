package game

import (
	"math"

	"github.com/theclifmeister/kingpin/internal/content"
)

// IncidentState is the world's incidents (#44) as they stand in this
// run: when the last one fired (the table's pacing), how often each has
// (Once retires a row, MinGap holds it off), and the record of every
// one for the run summary. The zero value is the pre-#44 state, so no
// schema bump.
type IncidentState struct {
	Last   int            // day the last incident fired; paces the next
	Count  map[string]int // incident id -> times fired this run
	LastOf map[string]int // incident id -> day it last fired
	Fired  []Incident     // every incident, oldest first
}

// Incident is one that fired: the row's id and name, the day, the city
// it landed in, the route it shut (a name; "" none), the product it
// moved ("" none), who it named ("" nobody) and the days its longest
// timed effect runs for.
type Incident struct {
	Day     int
	ID      string
	Name    string
	City    string // city id
	Route   string // route name
	Product string // product id
	Person  string
	Days    int
}

// IncidentTarget is where an incident lands, resolved by the world sim
// before it applies: the city (an id; the trigger's or home), the
// routes a closure shuts (ids) and the first of them by name.
type IncidentTarget struct {
	City   string
	Routes []string
	Route  string
}

// ApplyIncident is the one function that knows what every incident
// effect does to the world (#44), the way applyEffect is for a card's.
// It writes only state a sim already reads: a route's closure
// (RouteSetting.ClosedUntil), the market's own shock state, a city's
// pressure and heat, the notoriety axis and the heat sim's federal
// window. The two effects that need another sim's dice and tuning, a
// new chief and a snap election, ride the Incident event instead and
// the law sim honours them the same tick. day is the tick's day; a
// closure of N shuts the routes for the ticks day .. day+N-1, and a
// shock of N days holds the market's countdown for N ticks counting
// this one (the market steps next and takes one off before it prices).
func (w *World) ApplyIncident(day int, e content.IncidentEffects, at IncidentTarget) {
	if e.RouteClosed > 0 {
		if w.Routes == nil {
			w.Routes = map[string]RouteSetting{}
		}
		for _, id := range at.Routes {
			rs := w.Routes[id]
			rs.ClosedUntil = max(rs.ClosedUntil, day+e.RouteClosed)
			w.Routes[id] = rs
		}
	}
	city := w.Cities[at.City]
	if city == nil {
		city = w.Home()
	}
	if city != nil {
		if sh := e.MarketShock; sh.Set() {
			if m := city.Market[sh.Product]; m != nil {
				m.ShockFactor, m.ShockDays, m.ShockSlump = sh.Mul, sh.Days+1, false
			}
		}
		if sh := e.DemandShift; sh.Set() {
			if m := city.Market[sh.Product]; m != nil {
				m.ShockFactor, m.ShockDays, m.ShockSlump = sh.Mul, sh.Days+1, true
			}
		}
		if e.Pressure != 0 {
			city.Pressure = clamp(city.Pressure + e.Pressure)
		}
		if e.Heat != 0 {
			city.Heat = clamp(city.Heat + e.Heat)
			w.Heat.Peak = math.Max(w.Heat.Peak, city.Heat)
		}
	}
	if e.Notoriety != 0 {
		w.Player.Reputation.Notoriety = clamp(w.Player.Reputation.Notoriety + e.Notoriety)
	}
	if d := e.HeatDecay; d.Set() {
		w.Heat.FederalUntil = day + d.Days
		w.Heat.FederalDecay = d.Mul
	}
}

// IncidentDays is what the Days slot reads: the days of the row's first
// timed effect in a fixed order, the closure, then the price shock, the
// demand shift, the federal window and the snap election, so a row's
// text knows which it names.
func IncidentDays(e content.IncidentEffects) int {
	for _, d := range []int{e.RouteClosed, e.MarketShock.Days, e.DemandShift.Days, e.HeatDecay.Days, e.ElectionCalled} {
		if d > 0 {
			return d
		}
	}
	return 0
}

// RecordIncident books one that fired: the pacing, the counts and the
// record. The world sim is its one caller.
func (w *World) RecordIncident(inc Incident) {
	if w.Incidents.Count == nil {
		w.Incidents.Count = map[string]int{}
	}
	if w.Incidents.LastOf == nil {
		w.Incidents.LastOf = map[string]int{}
	}
	w.Incidents.Last = inc.Day
	w.Incidents.Count[inc.ID]++
	w.Incidents.LastOf[inc.ID] = inc.Day
	w.Incidents.Fired = append(w.Incidents.Fired, inc)
}

// RouteClosed reports whether a route is shut for tonight's tick: what
// the routes screen and the map read.
func (w *World) RouteClosed(id string) bool { return w.Route(id).Closed(w.Day + 1) }
