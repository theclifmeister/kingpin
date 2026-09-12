// Package world deals the world's incidents (#44): a port strike, a
// hurricane, a snap election, dealt from a weighted table the way the
// news sim deals the dilemma deck (a gap, then rising odds, one roll a
// day, fixed for the seed) but landing on the world, not the player, so
// it steps first in the order and every sim reacts the same day. An
// incident writes only state a sim already reads (a route's closure,
// the market's shock, a city's pressure, the heat sim's federal window;
// game.ApplyIncident is the one place that knows the keys) and never
// into a sim; the two effects that need the law's dice, a new chief and
// a snap election, ride the Incident event and the law sim honours them
// the same tick. Its dice are Tick.Sub("incidents") and nothing else,
// so a run with the table boxed (cfg.Incidents.Table = nil, the
// harness's default) is byte-for-byte the run before the sim existed.
package world

import (
	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
)

// Sim is the world simulation.
type Sim struct {
	cfg    content.IncidentsConfig
	routes []content.RouteConfig
	names  content.NamesConfig
}

// New builds the world sim from the config, copying what it reads
// (#144): the table, the route graph (a closure names the routes of a
// mode) and the name pools an incident draws from.
func New(cfg *content.Config) *Sim {
	return &Sim{cfg: cfg.Incidents, routes: cfg.Routes.Routes, names: cfg.Names}
}

func (s *Sim) Name() string { return "world" }

// Table lists the rows the sim knows, in file order.
func (s *Sim) Table() []content.IncidentConfig { return s.cfg.Table }

// Step deals at most one incident: none for MinGap days after the last,
// then a chance rising each day so one is certain by MaxGap, if any is
// eligible. One roll a day whatever happens keeps the stream fixed for
// the seed; an empty table draws nothing at all.
func (s *Sim) Step(w *game.World, t *game.Tick) {
	if len(s.cfg.Table) == 0 || w.Over != nil {
		return
	}
	pace := s.cfg.Incidents
	rng := t.Sub("incidents")
	since := t.Day - w.Incidents.Last
	roll := rng.Float64()
	if since < pace.MinGap {
		return
	}
	if roll >= float64(since-pace.MinGap+1)/float64(pace.MaxGap-pace.MinGap+1) {
		return
	}
	// Weighted pick over what is eligible: rows retired by once, held
	// off by their own gap or not yet due are out, and a row about a
	// product the city has not listed waits until it has.
	type pick struct {
		inc   *content.IncidentConfig
		slots game.CardSlots
		w     float64
	}
	var picks []pick
	total := 0.0
	for i := range s.cfg.Table {
		inc := &s.cfg.Table[i]
		n := w.Incidents.Count[inc.ID]
		if inc.Once && n > 0 || t.Day < inc.MinDay {
			continue
		}
		if last, ok := w.Incidents.LastOf[inc.ID]; ok && t.Day-last < inc.MinGap {
			continue
		}
		sl, ok := game.Eligible(w, content.CardConfig{Trigger: inc.Trigger})
		if !ok {
			continue
		}
		city := s.city(w, sl)
		if !listed(w, city, inc.Effects.MarketShock) || !listed(w, city, inc.Effects.DemandShift) {
			continue
		}
		wt := inc.Weight
		if wt <= 0 {
			wt = 1
		}
		picks = append(picks, pick{inc, sl, wt})
		total += wt
	}
	if len(picks) == 0 {
		return
	}
	r := rng.Float64() * total
	chosen := picks[len(picks)-1]
	for _, p := range picks {
		if r < p.w {
			chosen = p
			break
		}
		r -= p.w
	}
	inc := chosen.inc
	at := game.IncidentTarget{City: s.city(w, chosen.slots)}
	for _, r := range s.routes {
		if inc.RouteMode != "" && r.Mode == inc.RouteMode && w.Cities[r.From] != nil && w.Cities[r.To] != nil {
			at.Routes = append(at.Routes, r.ID)
			if at.Route == "" {
				at.Route = r.Name
			}
		}
	}
	person := ""
	if pool := s.names.Pool(inc.Names); len(pool) > 0 {
		person = pool[rng.IntN(len(pool))]
	}
	product := inc.Effects.MarketShock.Product
	if product == "" {
		product = inc.Effects.DemandShift.Product
	}
	w.ApplyIncident(t.Day, inc.Effects, at)
	rec := game.Incident{Day: t.Day, ID: inc.ID, Name: inc.Name, City: at.City, Route: at.Route, Product: product, Person: person, Days: game.IncidentDays(inc.Effects)}
	w.RecordIncident(rec)
	t.Emit(events.Incident{
		Day: t.Day, ID: inc.ID, Name: inc.Name, City: at.City, Route: at.Route, Product: product, Person: person, Days: rec.Days,
		Chief: w.Law.Chief.Name, DA: w.Law.DA.Name, NewChief: inc.Effects.ChiefReplaced, Election: inc.Effects.ElectionCalled,
	})
}

// city is where an incident lands: the city its trigger named, else
// home. The world does not follow the player around.
func (s *Sim) city(w *game.World, sl game.CardSlots) string {
	if sl.CityID != "" {
		return sl.CityID
	}
	if h := w.Home(); h != nil {
		return h.ID
	}
	return ""
}

// listed reports whether a shock's product is on the city's market, or
// the shock is not set.
func listed(w *game.World, city string, sh content.ProductShock) bool {
	return !sh.Set() || w.Product(city, sh.Product) != nil
}
