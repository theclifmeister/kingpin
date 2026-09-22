package news

import (
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
)

// reporter is one morning's paper as Step writes it: the report, the
// headlines, what every line names by default, and the money the day's
// events moved, which the report's CASH BEFORE undoes. Each sim's events
// are written by its own method (report_<sim>.go); Step makes one a
// tick and hands it every event in the order they were emitted, so the
// templates draw off the streams in the order they always did.
type reporter struct {
	s     *Sim
	w     *game.World
	t     *game.Tick
	rep   *game.DayReport
	lines []game.Headline
	here  *game.City // where you stand: a line about anywhere else says where
	base  data       // what every line names unless it says otherwise

	// The money the day's events moved, summed for CASH BEFORE and the
	// MONEY lines: sales are already applied by market.
	soldRevenue, lostCash, spent, wages, skimmed, robbed, upgrades, upkeep int
	seized, paidOff, investigated, shipping, tribute, cuts, standingCut    int
	funded, backed, contracts, forfeits, repaid, rent, earned, invested    int
	cutting, cooking, reserved, deeds, deedRent, taxed                     int

	scouted, poached, boosted int // the books (#70): what a scout and a buy-off cost, less the refund, and what a boost took
	bribed, checkpoints       int // the bought law (#42): the envelopes and the deals on the road, paid up front

	routeCost  map[string]int // what each route cost today, lots and fares, by name
	routeOrder []string       // the routes in the order they first spent
}

// report writes one event into the morning, in the method of the sim
// that emits it, the day loop's order: an event is one sim's, so the
// first to take it is the only one that would.
func (r *reporter) report(e events.Event) {
	_ = r.reportMarket(e) || r.reportLogistics(e) || r.reportTerritory(e) || r.reportRivals(e) ||
		r.reportCrew(e) || r.reportHeat(e) || r.reportLaw(e) || r.reportLaundering(e) || r.reportReputation(e)
}

// add is a headline under source, its template picked off the home
// city's stream.
func (r *reporter) add(source, key string, d data) {
	list := r.s.tmpl[key]
	if len(list) == 0 {
		return
	}
	txt := render(list[r.t.RNG.IntN(len(list))], d)
	r.lines = append(r.lines, game.Headline{Day: r.t.Day, Source: source, Text: txt})
}

// addOff is add with the template picked off a side stream. The buyers'
// lines pick theirs off the deck's own stream (#71): the deck never
// touches the home city's dice, and its news must not either, or a run
// would deal its cards on different days with the deck in and out of the
// box. The connects' lines (#72) pick theirs off the connects' stream for
// the same reason: a run that never takes credit is the run it was.
func (r *reporter) addOff(stream, source, key string, d data) {
	list := r.s.tmpl[key]
	if len(list) == 0 {
		return
	}
	txt := render(list[r.t.Sub(stream).IntN(len(list))], d)
	r.lines = append(r.lines, game.Headline{Day: r.t.Day, Source: source, Text: txt})
}

func (r *reporter) addBuyers(key string, d data)    { r.addOff(game.StreamBuyers, "buyers", key, d) }
func (r *reporter) addSuppliers(key string, d data) { r.addOff(game.StreamSuppliers, "market", key, d) }

// addHouses: the houses' lines (#73) pick theirs off a side stream too:
// a run with no house is the run it was.
func (r *reporter) addHouses(source, key string, d data) {
	r.addOff(game.StreamHousesNews, source, key, d)
}

// addIntel: the intel lines (#45) pick theirs off the intel side stream:
// a run with no spy under and nobody feeding it is the run it was.
func (r *reporter) addIntel(source, key string, d data) {
	r.addOff(game.StreamIntelNews, source, key, d)
}

// crew names the faction a rival event is about (#43): the event's
// leader and their crew in the Leader and Faction slots, so a line about
// the second faction names the second faction.
func (r *reporter) crew(d data, leader string) data {
	if leader != "" {
		d.Leader, d.Faction = leader, leader+"'s crew"
	}
	return d
}

// in names a city for a line about somewhere other than where you are.
func (r *reporter) in(city string) string {
	if city == r.here.ID || r.w.Cities[city] == nil {
		return ""
	}
	return " in " + r.w.CityName(city)
}

// at is base for an event in a city.
func (r *reporter) at(city string) data {
	d := r.base
	if c := r.w.Cities[city]; c != nil {
		d.City = c.Name
	}
	return d
}

// cornerCity is the city a corner is in, or where you stand.
func (r *reporter) cornerCity(id string) string {
	if c := r.w.Corner(id); c != nil {
		return c.City
	}
	return r.here.ID
}

// charge puts a route's cost on the day's tab, keeping the order the
// routes first spent in.
func (r *reporter) charge(route string, cost int) {
	if _, ok := r.routeCost[route]; !ok {
		r.routeOrder = append(r.routeOrder, route)
	}
	r.routeCost[route] += cost
}
