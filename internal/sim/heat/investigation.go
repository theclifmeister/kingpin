package heat

import (
	"math"
	"sort"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
)

// Targeted investigations (#343, heat.toml [investigation]). With the
// feature on the sim keeps a tally of the heat it already computes by
// the source the police can put a name to (HeatState.Trail): a street
// sale's heat split over the corners it moved on, a buyer's handoff by
// its product, a move by the house the stock went into. The night the
// hottest city's police would send the sting, they open an
// investigation on the biggest source in that city instead (an argmax:
// no dice), and lead_days nights later the sting lands on that target
// alone. Off, nothing here runs and the sting is the blind one.

// Investigating reports whether investigations open at all.
func (s *Sim) Investigating() bool { return s.cfg.Investigation.On() }

// InvestigationTuning is the file's [investigation], for the UI.
func (s *Sim) InvestigationTuning() content.InvestigationTuning { return s.cfg.Investigation }

// lead puts v of today's heat on a source.
func (d *day) lead(city, kind, target string, v float64) {
	if v <= 0 || d.leads == nil {
		return
	}
	d.leads[game.LeadKey(city, kind, target)] += v
}

// leadSale splits a street sale's heat over the corners it moved on in
// the proportions CornerWeight weighs them: each worked corner's share
// of the product times its heat, at the crew discount for a runner's
// and your notoriety for yours.
func (s *Sim) leadSale(d *day, city, product string, v float64) {
	if d.leads == nil || v <= 0 {
		return
	}
	d.sold[city+"/"+product] = true
	c0 := d.w.City(city)
	if c0 == nil {
		return
	}
	personal, crew := s.PersonalHeat(d.w), s.CrewHeat(d.w)
	type part struct {
		id string
		wt float64
	}
	var parts []part
	total := 0.0
	for _, c := range c0.Corners {
		if !c.Worked() {
			continue
		}
		unit := c.Heat
		if c.Runner != game.You {
			unit *= crew
		} else {
			unit *= personal
		}
		if wt := c.Share(product) * unit; wt > 0 {
			parts = append(parts, part{c.ID, wt})
			total += wt
		}
	}
	for _, p := range parts {
		d.lead(city, game.LeadCorner, p.id, v*p.wt/total)
	}
}

// trail folds today's leads into the window: every tally forgets
// 1/window_days of itself, today's heat goes on top, and a tally under
// a hundredth of a point is dropped.
func (s *Sim) trail(d *day) {
	if d.leads == nil {
		return
	}
	h := d.h
	if h.Trail == nil {
		h.Trail = map[string]float64{}
	}
	keep := 1 - 1/float64(s.cfg.Investigation.WindowDays)
	for k, v := range h.Trail {
		if v *= keep; v < 0.01 {
			delete(h.Trail, k)
		} else {
			h.Trail[k] = v
		}
	}
	for k, v := range d.leads {
		h.Trail[k] += v
	}
}

// Lead is a source of a city's heat in the window, for the UI and the
// opening: its kind, target and tally.
type Lead struct {
	Kind   string
	Target string
	Heat   float64
}

// Leads is a city's sources in the window, biggest first; a tie goes to
// the kind first in game.LeadKinds (a corner before a product before a
// house), then the target's id. The first is the one an investigation
// names.
func Leads(w *game.World, city string) []Lead {
	var out []Lead
	for k, v := range w.Heat.Trail {
		if c, kind, target := game.SplitLeadKey(k); c == city && v > 0 {
			out = append(out, Lead{kind, target, v})
		}
	}
	rank := func(kind string) int {
		for i, k := range game.LeadKinds {
			if k == kind {
				return i
			}
		}
		return len(game.LeadKinds)
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Heat != b.Heat {
			return a.Heat > b.Heat
		}
		if rank(a.Kind) != rank(b.Kind) {
			return rank(a.Kind) < rank(b.Kind)
		}
		return a.Target < b.Target
	})
	return out
}

// open is the night the sting would come to city: the police open an
// investigation on its biggest source instead, and say so. False with
// nothing to name: the sting is the blind one.
func (s *Sim) open(d *day, city *game.City) bool {
	leads := Leads(d.w, city.ID)
	if len(leads) == 0 {
		return false
	}
	top := leads[0]
	due := d.t.Day + s.cfg.Investigation.LeadDays
	d.h.Investigation = game.Investigation{City: city.ID, Kind: top.Kind, Target: top.Target, Opened: d.t.Day, Due: due}
	d.t.Emit(events.InvestigationOpened{Day: d.t.Day, City: city.ID, Lead: top.Kind, Target: top.Target, Name: d.w.LeadName(top.Kind, top.Target), Due: due})
	return true
}

// strike is the night an investigation lands: the sting on its target
// alone. A corner worked tonight loses the sting's share of the street's
// stock and the pile, and its runner and enforcer are swept; a product
// loses its whole stock on the street in the city; a house is raided at
// the raid's share. It files the sting's pages only if the target was in
// use tonight (a corner sold on, a product sold or handed over, a house
// holding stock on a night you dealt in its city): only dealing builds
// a case (#27), the investigation's evidence where the file sets it. A
// hit that found anything takes closed_heat_drop off the city's heat
// (each repeat less, as fire's) and goes out as the sting's
// Enforcement; a miss takes nothing, files nothing and cools nothing.
func (s *Sim) strike(d *day, sting content.ResponseConfig) {
	w, t, h, fx := d.w, d.t, d.h, d.fx
	inv := h.Investigation
	h.Investigation = game.Investigation{}
	city := w.Cities[inv.City]
	closed := events.InvestigationClosed{Day: t.Day, City: inv.City, Lead: inv.Kind, Target: inv.Target, Name: w.LeadName(inv.Kind, inv.Target)}
	if city == nil {
		t.Emit(closed)
		return
	}
	ev := events.Enforcement{Day: t.Day, City: city.ID, Level: sting.Level, StockLost: map[string]int{}}
	inUse := false
	switch inv.Kind {
	case game.LeadCorner:
		c := w.Corner(inv.Target)
		if c == nil || c.City != city.ID || !c.Worked() || d.leads[game.LeadKey(city.ID, game.LeadCorner, c.ID)] <= 0 {
			break
		}
		inUse = true
		loss := sting.StockLoss * fx.StingStockMul
		for id, q := range w.StreetOf(city.ID) {
			if lost := w.TakeStreet(city.ID, id, int(math.Round(float64(q)*loss))); lost > 0 {
				ev.StockLost[id] = lost
			}
		}
		ev.CashLost = int(math.Round(float64(w.Player.DirtyCash) * sting.CashLoss))
		w.Player.DirtyCash -= ev.CashLost
		sweep := game.Sweep{Day: t.Day, City: city.ID, Level: sting.Level, Corners: []string{c.ID}}
		for _, id := range []int{c.Runner, c.Enforcer} {
			if id > 0 {
				sweep.Crew = append(sweep.Crew, id)
			}
		}
		for _, n := range ev.StockLost {
			sweep.Units += n
		}
		ev.Corners = sweep.Corners
		w.Heat.Sweep = sweep
	case game.LeadProduct:
		inUse = d.sold[city.ID+"/"+inv.Target]
		if lost := w.TakeStreet(city.ID, inv.Target, w.StreetOf(city.ID)[inv.Target]); lost > 0 {
			ev.StockLost[inv.Target] = lost
		}
	case game.LeadHouse:
		house := w.House(inv.Target)
		if house == nil || house.City != city.ID || house.Units() == 0 {
			break
		}
		inUse = d.attempted[city.ID]
		loss := sting.StockLoss
		if r := s.rung(content.Raid); r != nil {
			loss = r.StockLoss * fx.RaidLossMul
		}
		ev.House, ev.HouseName = house.ID, house.Name
		for _, id := range w.SortedProducts() {
			if lost := w.TakeFromHouse(house.ID, id, int(math.Round(float64(house.Stock[id])*loss))); lost > 0 {
				ev.StockLost[id] = lost
				w.Stats.HouseUnits += lost
			}
		}
		house.Raided++
		t.Emit(events.HouseRaided{Day: t.Day, House: house.ID, Name: house.Name, City: city.ID, Level: sting.Level, StockLost: ev.StockLost})
		if !house.Known && len(ev.StockLost) > 0 {
			house.Known = true
			t.Emit(events.HouseCompromised{Day: t.Day, House: house.ID, Name: house.Name, City: city.ID, Why: "bust"})
		}
	}
	units := 0
	for _, n := range ev.StockLost {
		units += n
	}
	closed.Hit = inUse || units > 0
	if !closed.Hit {
		// Nothing there: the police found nothing and cool nothing.
		// The city stays as hot as the rest of the operation keeps it.
		t.Emit(closed)
		return
	}
	h.Responses[sting.Level]++
	w.Stats.Stings++
	if units > 0 {
		h.Busts = append(h.Busts, game.Bust{Day: t.Day, City: city.ID, Level: sting.Level, Units: units})
		kept := h.Busts[:0]
		for _, b := range h.Busts {
			if t.Day-b.Day <= s.cfg.Heat.BustDays {
				kept = append(kept, b)
			}
		}
		h.Busts = kept
	}
	if inUse {
		pages := sting.Evidence
		if s.cfg.Investigation.Evidence > 0 {
			pages = s.cfg.Investigation.Evidence
		}
		ev.Evidence = max(0, pages-fx.EvidenceCut)
		if ev.Evidence > 0 {
			h.Evidence += ev.Evidence
			h.EvidenceDay = t.Day
		}
	}
	closed.Evidence = ev.Evidence
	// Each repeat cools less, as every rung's does (fire): the first
	// corner fed to them buys the most.
	city.Heat -= s.cfg.Investigation.ClosedHeatDrop / float64(max(1, h.Responses[sting.Level]))
	t.Emit(ev)
	t.Emit(closed)
}
