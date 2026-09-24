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
	cut := max(0, min(int(math.Round(float64(revenue)*s.cfg.Captain.Cut)), w.Player.DirtyCash))
	w.Player.DirtyCash -= cut
	w.Stats.Cuts += cut
	return cut
}

// shadow is the world as a captain's night reads it where the world has
// not caught up yet: the loyalty a member will have by the captain's
// phase, the members gone by then, and the posts the night's earlier
// captains made. The night itself reads the world as it stands (an
// empty shadow); the day's preview (#353) fills one from the morning's
// certain moves and carries it from city to city, so the plan it reads
// is the plan the night runs.
type shadow struct {
	loy  map[int]float64
	gone map[int]bool
	post map[int]string // member -> the corner they stand on tonight, "" for none
}

func (sh *shadow) loyalty(m game.CrewMember) float64 {
	if l, ok := sh.loy[m.ID]; ok {
		return l
	}
	return m.Loyalty
}

func (sh *shadow) postOf(w *game.World, id int) *game.Corner {
	if cid, ok := sh.post[id]; ok {
		if cid == "" {
			return nil
		}
		return w.Corner(cid)
	}
	return w.PostOf(id)
}

// runnerOn is who works a corner tonight, as the shadow reads it.
func (sh *shadow) runnerOn(w *game.World, k *game.Corner) int {
	for id, cid := range sh.post {
		if cid == k.ID {
			if m := w.Crew.Member(id); m != nil && m.Role == game.RoleRunner {
				return id
			}
		}
	}
	if r := k.Runner; r != 0 && r != game.You {
		if cid, ok := sh.post[r]; ok && cid != k.ID {
			return 0 // moved off tonight
		}
	}
	return k.Runner
}

// plan is what a captain decides tonight, in the order they act on it:
// the suspected skimmers to pull, the idle runners to post and on which
// corner, and the members near their line to pay off, lowest first
// (the budget and the till are the caller's to apply). It writes
// nothing: captain carries it out on the world, the preview reads it.
type plan struct {
	pulled []int
	posts  []posting
	near   []int
}

// posting is one idle runner the plan puts on a corner.
type posting struct {
	member int
	corner string
}

func (s *Sim) plan(w *game.World, sh *shadow, cp game.CrewMember, city string, day, lastSkim int, d tonight, fx game.Effects) plan {
	tun := s.cfg.Crew
	var p plan
	c := w.Cities[city]
	if c == nil {
		return p
	}
	members := w.Crew.Members
	pulled := map[int]bool{}

	// 1. The suspected skimmers come off the corner.
	if lastSkim > 0 && day-lastSkim < tun.SuspectDays {
		for _, m := range members {
			if sh.gone[m.ID] || m.ID == cp.ID || sh.loyalty(m) >= tun.SkimThreshold {
				continue
			}
			if k := sh.postOf(w, m.ID); k != nil && k.City == city {
				pulled[m.ID] = true
				p.pulled = append(p.pulled, m.ID)
				sh.post[m.ID] = ""
			}
		}
	}

	// 2. The idle runners onto the held corners nobody works, biggest
	// first; a suspect is nobody to post. A held corner in a city is
	// always yours to post on, so nothing here is refused.
	for _, m := range members {
		if sh.gone[m.ID] || m.Role != game.RoleRunner || !m.Fit(day) || pulled[m.ID] || sh.loyalty(m) < tun.SkimThreshold || sh.postOf(w, m.ID) != nil {
			continue
		}
		var open []*game.Corner
		for i := range c.Corners {
			if k := &c.Corners[i]; k.Held() && sh.runnerOn(w, k) == 0 {
				open = append(open, k)
			}
		}
		sort.SliceStable(open, func(i, j int) bool { return open[i].Demand > open[j].Demand })
		if len(open) > 0 {
			p.posts = append(p.posts, posting{m.ID, open[0].ID})
			sh.post[m.ID] = open[0].ID
		}
	}

	// 3. The pay-offs, lowest first: whoever tonight's drift leaves
	// within margin of their next line down, the crew trouble alert's
	// reading (#345): the skim line, or the walk line once they are
	// under it.
	type cand struct {
		id  int
		loy float64
	}
	var near []cand
	for _, m := range members {
		if sh.gone[m.ID] || m.ID == cp.ID || m.Lieutenant() || s.cityIn(w, sh, m) != city {
			continue
		}
		m.Loyalty = sh.loyalty(m)
		line := tun.SkimThreshold
		if m.Loyalty < line {
			line = tun.QuitThreshold
		}
		if m.Loyalty+s.memberDrift(m, d.base, d.danger, d.shield, d.toll, d.loss, fx)-line <= s.cfg.Captain.Margin {
			near = append(near, cand{m.ID, m.Loyalty})
		}
	}
	sort.SliceStable(near, func(i, j int) bool { return near[i].loy < near[j].loy })
	for _, n := range near {
		p.near = append(p.near, n.id)
	}
	return p
}

// captain is one captain's night in their city, through the player's
// own actions (the plan's, in its order): the suspected skimmers come
// off their corners (World.Recall), the idle runners go on the held
// corners nobody works (World.Post), and the members near their line
// are paid off (World.PayOff at the pay-off's price and loyalty) while
// the budget lasts, their kin lifted as a pay-off by hand lifts them.
func (s *Sim) captain(n *night, cp *game.CrewMember, d tonight, ev *events.CaptainActed) {
	w, t, c := n.w, n.t, n.c
	sh := &shadow{post: map[int]string{}}
	p := s.plan(w, sh, *cp, ev.City, t.Day, c.LastSkim, d, n.fx)
	for _, id := range p.pulled {
		if m := c.Member(id); m != nil {
			w.Recall(id)
			ev.Pulled = append(ev.Pulled, m.Name)
		}
	}
	for _, ps := range p.posts {
		id := ps.member
		if w.Post(ps.corner, id) == nil {
			ev.Posted = append(ev.Posted, w.Corner(ps.corner).Name)
		}
	}
	for _, id := range p.near {
		m := c.Member(id)
		if m == nil {
			continue
		}
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

// cityIn is the city a member works in, for the captain's care: their
// corner's, the house they guard's, else home.
func (s *Sim) cityIn(w *game.World, sh *shadow, m game.CrewMember) string {
	if p := sh.postOf(w, m.ID); p != nil {
		return p.City
	}
	if _, moved := sh.post[m.ID]; moved {
		return w.Home().ID // pulled tonight: off the corner, and Recall takes them off a house too
	}
	if h := w.GuardOf(m.ID); h != nil {
		return h.City
	}
	return w.Home().ID
}

// CaptainNight is a captain's night as the day's preview reads it
// (#353): the city, the cut of its takings before the till caps it, the
// pay-offs in the order they would be made (each the member's price)
// and the budget they are made out of. A captain who is out tonight
// makes none and cuts nothing; one who stopped caring cuts and pays
// nobody.
type CaptainNight struct {
	City    string
	Cut     int
	Payoffs []int
	Budget  int
}

// CaptainNights is every captain's night tonight as the crew step will
// run it, off the world before the day ends (#353): revenue is what each
// city's street takes tonight (the captain's cut is of it), short
// whether tonight's wages will come up short. It reads the morning's
// certain moves the life phase makes before the captains act (the kin
// of today's firings and pay-offs, the cells that open, the retirements)
// and nothing that rolls: a trait shown tonight, a skim, an arrest or a
// strike is the preview's unknown. It writes nothing.
func (s *Sim) CaptainNights(w *game.World, revenue map[string]int, short bool) []CaptainNight {
	day := w.Day + 1
	fx := game.FoldEffects(w, s.tree)
	sh := s.morning(w, day)
	var out []CaptainNight
	var d *tonight
	for _, cid := range w.CityOrder {
		cp := w.Crew.Captain(cid)
		if cp == nil || sh.gone[cp.ID] || !cp.Fit(day) {
			continue
		}
		cn := CaptainNight{City: cid, Budget: cp.Budget, Cut: int(math.Round(float64(revenue[cid]) * s.cfg.Captain.Cut))}
		if sh.loyalty(*cp) >= s.cfg.Captain.Care {
			if d == nil {
				v := s.previewNight(w, day, short, sh, fx)
				d = &v
			}
			p := s.plan(w, sh, *cp, cid, day, w.Crew.LastSkim, *d, fx)
			for _, id := range p.near {
				if m := w.Crew.Member(id); m != nil {
					cn.Payoffs = append(cn.Payoffs, s.PayoffCost(*m))
				}
			}
		}
		out = append(out, cn)
	}
	return out
}

// previewNight is tonight's quiet reading as the preview sees it: the
// firings the rest hold against you, the enforcers at work by the skim
// (a cell that opens tonight counts), unpaid wages, and an
// investigation that can only name nobody.
func (s *Sim) previewNight(w *game.World, day int, short bool, sh *shadow, fx game.Effects) tonight {
	fired := 0
	for _, m := range w.Crew.FiredToday {
		if !m.Informant {
			fired++
		}
	}
	enforcers := 0
	for _, m := range w.Crew.Members {
		if m.Role == game.RoleEnforcer && !sh.gone[m.ID] && m.Undercover == "" && m.JailedUntil <= day && m.WoundedUntil <= day {
			enforcers++
		}
	}
	asked := w.Today.Investigation != nil && w.Crew.Informants() == 0
	return s.quietNight(w, w.Crew.Pay, day, fired, short, asked, enforcers, fx)
}

// morning is the shadow of the life phase's certain moves before the
// captains act: the kin of today's firings lose kin_loyalty and of
// today's pay-offs gain it, a cell that opens tonight sets the loyalty
// it sets, and a member whose birthday tonight is the farewell is gone.
func (s *Sim) morning(w *game.World, day int) *shadow {
	sh := &shadow{loy: map[int]float64{}, gone: map[int]bool{}, post: map[int]string{}}
	life := s.cfg.Life
	if !life.On() {
		return sh
	}
	kin := func(ids []int, d float64) {
		for _, id := range ids {
			if k := w.Crew.Member(id); k != nil {
				sh.loy[id] = max(0, min(100, sh.loyalty(*k)+d))
			}
		}
	}
	for _, m := range w.Crew.FiredToday {
		if !m.Informant {
			kin(m.Kin, -life.KinLoyalty)
		}
	}
	for _, p := range w.Crew.PaidOffToday {
		if m := w.Crew.Member(p.ID); m != nil {
			kin(m.Kin, life.KinLoyalty)
		}
	}
	inf := s.cfg.Informant
	for _, m := range w.Crew.Members {
		if m.JailedUntil > 0 && m.JailedUntil <= day {
			if m.Bailed {
				sh.loy[m.ID] = min(100, sh.loyalty(m)+life.BailLoyalty)
			} else {
				sh.loy[m.ID] = max(0, min(sh.loyalty(m), inf.Loyalty-life.JailLoyalty))
			}
		}
		if since := day - m.Hired; m.Age > 0 && since > 0 && since%life.YearDays == 0 && m.Age+1 >= life.RetireAge {
			sh.gone[m.ID] = true
		}
	}
	return sh
}
