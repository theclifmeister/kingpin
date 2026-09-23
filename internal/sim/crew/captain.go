package crew

import (
	"math"
	"sort"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
)

// The captain (#346): a trusted member named to run crew care for a
// city (World.NameCaptain, one a city). Modelled on the lieutenant's
// delegate, everything a captain does is the player's own actions,
// taken for them each night after the wages and the investigation and
// before the drift: nothing new is simulated, only who decides, and
// nothing here rolls dice. A run where nobody is named is the run before
// the feature: the phase finds no captain and returns.

// Captaincy exposes the captain's tuning the UI explains itself with.
func (s *Sim) Captaincy() content.CaptainTuning { return s.cfg.Captain }

// CanCaptain is whether m could be named captain today: on the payroll
// long enough and loyal enough, at work, and not a lieutenant (who runs
// a city, and is nobody's captain). It is World.NameCaptain's rule on
// the tuning, for the crew screen to say before it asks.
func (s *Sim) CanCaptain(w *game.World, m game.CrewMember) bool {
	cp := s.cfg.Captain
	return !m.Lieutenant() && m.Working() && m.Loyalty >= cp.Loyalty && w.Day-m.Hired >= cp.Days
}

// captains runs each captain's night, city by city in the map's order,
// and files what they did.
func (s *Sim) captains(n *night) {
	w, t, c := n.w, n.t, n.c
	var d *tonight
	for _, cid := range w.CityOrder {
		cp := c.Captain(cid)
		if cp == nil {
			continue
		}
		ev := events.CaptainActed{Day: t.Day, ID: cp.ID, Name: cp.Name, City: cid, CityName: w.CityName(cid)}
		switch {
		case !cp.Fit(t.Day):
			ev.Absent = true // arrested, shot or under: the city goes without tonight
		case cp.Loyalty < s.cfg.Captain.Care:
			ev.Careless = true
			ev.Cut = s.captainCut(w, t, cid)
		default:
			ev.Cut = s.captainCut(w, t, cid)
			if d == nil {
				v := s.tonight(n)
				d = &v
			}
			s.captain(n, cp, *d, &ev)
		}
		t.Emit(ev)
	}
}

// captainCut is the captain's cut of their city's takings today, off
// dirty cash as the lieutenant's is.
func (s *Sim) captainCut(w *game.World, t *game.Tick, city string) int {
	revenue := 0
	for _, e := range t.Events() {
		if ps, ok := e.(events.PlayerSold); ok && ps.City == city {
			revenue += ps.Revenue
		}
	}
	cut := min(int(math.Round(float64(revenue)*s.cfg.Captain.Cut)), w.Player.DirtyCash)
	cut = max(0, cut)
	w.Player.DirtyCash -= cut
	w.Stats.Cuts += cut
	return cut
}

// captain is one captain's night in their city, through the player's
// own actions and in this order: a suspected skimmer (under the skim
// line while the skim is fresh, the crew screen's warning) comes off
// their corner there (World.Recall); the idle runners at work go on the
// held corners there nobody works, biggest first, before they drift
// (World.Post); and a member there whom tonight's drift leaves within
// margin of the quit line is paid off (World.PayOff at the pay-off's
// price and loyalty), the lowest first, while the budget lasts. The
// kin of one paid off are lifted as a pay-off by hand lifts them.
func (s *Sim) captain(n *night, cp *game.CrewMember, d tonight, ev *events.CaptainActed) {
	w, t, c, fx := n.w, n.t, n.c, n.fx
	tun := s.cfg.Crew
	city := w.Cities[ev.City]
	if city == nil {
		return
	}

	// 1. The suspected skimmers come off the corner.
	pulled := map[int]bool{}
	if c.LastSkim > 0 && t.Day-c.LastSkim < tun.SuspectDays {
		for _, m := range c.Members {
			if m.ID == cp.ID || m.Loyalty >= tun.SkimThreshold {
				continue
			}
			if p := w.PostOf(m.ID); p != nil && p.City == ev.City {
				w.Recall(m.ID)
				pulled[m.ID] = true
				ev.Pulled = append(ev.Pulled, m.Name)
			}
		}
	}

	// 2. The idle runners onto the held corners nobody works, biggest
	// first; a suspect is nobody to post.
	for _, m := range c.Members {
		if m.Role != game.RoleRunner || !m.Fit(t.Day) || pulled[m.ID] || m.Loyalty < tun.SkimThreshold || w.PostOf(m.ID) != nil {
			continue
		}
		var open []*game.Corner
		for i := range city.Corners {
			if k := &city.Corners[i]; k.Held() && k.Runner == 0 {
				open = append(open, k)
			}
		}
		sort.SliceStable(open, func(i, j int) bool { return open[i].Demand > open[j].Demand })
		for _, k := range open {
			if w.Post(k.ID, m.ID) == nil {
				ev.Posted = append(ev.Posted, k.Name)
				break
			}
		}
	}

	// 3. The pay-offs, lowest first, while the budget lasts: whoever
	// tonight's drift leaves within margin of their next line down, the
	// crew trouble alert's reading (#345): the skim line, or the walk
	// line once they are under it.
	var near []*game.CrewMember
	for i := range c.Members {
		m := &c.Members[i]
		if m.ID == cp.ID || m.Lieutenant() || s.cityOf(w, *m) != ev.City {
			continue
		}
		line := tun.SkimThreshold
		if m.Loyalty < line {
			line = tun.QuitThreshold
		}
		if m.Loyalty+s.memberDrift(*m, d.base, d.danger, d.shield, d.toll, d.loss, fx)-line <= s.cfg.Captain.Margin {
			near = append(near, m)
		}
	}
	sort.SliceStable(near, func(i, j int) bool { return near[i].Loyalty < near[j].Loyalty })
	for _, m := range near {
		cost := s.PayoffCost(*m)
		if ev.Spent+cost > cp.Budget {
			continue
		}
		if _, err := w.PayOff(m.ID, cost, s.PayoffLoyalty()); err != nil {
			continue
		}
		ev.Spent += cost
		ev.SpentClean += c.PaidOffToday[len(c.PaidOffToday)-1].Clean
		ev.Paid = append(ev.Paid, m.Name)
		if s.cfg.Life.On() {
			s.kinLoyalty(w, *m, s.cfg.Life.KinLoyalty)
		}
	}
}

// cityOf is the city a member works in, for the captain's care: their
// corner's, the house they guard's, else home.
func (s *Sim) cityOf(w *game.World, m game.CrewMember) string {
	if p := w.PostOf(m.ID); p != nil {
		return p.City
	}
	if h := w.GuardOf(m.ID); h != nil {
		return h.City
	}
	return w.Home().ID
}
