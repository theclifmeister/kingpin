// The table follows the money (#341): a city where the player earns and
// no faction lives draws one, telegraphed in stages.

package rivals

import (
	"fmt"
	"slices"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
)

// Expansion is earned by the player's take, not by the calendar. The
// rivals sim keeps a window on the take in every city away from home
// (World.Takes, off the tick's PlayerSold, no dice), and a city where
// no faction lives whose window crosses take_min draws one:
//
//  1. the scouts (RivalScouting, an intel fact, the map's mark): a seat
//     still in the wings, the last to come, or, once every seat has
//     arrived, a cell of the strongest faction at home (Cell), which
//     takes start_muscle of its heads, never its last;
//  2. the recruiting, scout_days later (RivalRecruiting): bite of the
//     hiring pool's best faces (the crew sim drops them), your crew in
//     the city poachable at the table's poach_chance and poach_line,
//     and a tribute offered for a peace through the usual DealOffered;
//  3. the arrival, arrive_days after that: the arrive step seats it
//     with Home the city under arrive_grace, and from then on it is a
//     faction like any other.
//
// The player's answers: grab the city's corners first (the table's
// share and arrive_grace leave it less room), hit the scouts (once, a
// setback of setback_days and a grudge), take the tribute (it arrives
// at peace), let the take fall back under the line before the
// recruiting (the scouts go home: RivalWithdrew, and the window starts
// over), or leave (once it has recruited it comes anyway).
//
// It needs a table (the duel never expands), and its dice are the
// expansion stream's (a cell's leader, the recruiters' poach), so a run
// whose take away from home never crosses take_min is byte-for-byte the
// run before it (TestNoExpansionIsTheOldRun).

// Take is the player's take in a city over the window: the revenue of
// the last window_days days (#341). Zero for a city nobody sold in.
func (s *Sim) Take(w *game.World, city string) int {
	n := 0
	for _, v := range w.Takes[city] {
		n += v
	}
	return n
}

// Expansion exposes the expansion's tuning (#341) for the UI and the
// harness.
func (s *Sim) Expansion() content.ExpansionTuning { return s.cfg.Expansion }

// window writes tonight's take into every away city's window: the slot
// for today (the day modulo the window) holds what sold there tonight.
// A city gets a row the first night anything sells there.
func (s *Sim) window(w *game.World, t *game.Tick) {
	n := s.cfg.Expansion.WindowDays
	if n < 1 {
		return
	}
	home := w.Home().ID
	sold := map[string]int{}
	for _, e := range t.Events() {
		if ev, ok := e.(events.PlayerSold); ok && ev.City != home && ev.Revenue > 0 {
			sold[ev.City] += ev.Revenue
		}
	}
	for _, cid := range w.CityOrder {
		row, ok := w.Takes[cid]
		if !ok && sold[cid] == 0 {
			continue
		}
		if len(row) != n {
			row = make([]int, n)
		}
		row[t.Day%n] = sold[cid]
		if w.Takes == nil {
			w.Takes = map[string][]int{}
		}
		w.Takes[cid] = row
	}
}

// expand is the expansion's night (#341), before the factions step: the
// window, the factions already on their way (a hit on the scouts, the
// scouts going home, the recruiting), then a city over the line with
// nobody living there drawing one.
func (s *Sim) expand(w *game.World, t *game.Tick) {
	if len(w.Rivals) < 2 {
		return // the duel never expands
	}
	s.window(w, t)
	e := s.cfg.Expansion
	if !e.Enabled {
		return
	}
	var home []*game.RivalState
	for _, r := range w.Rivals {
		if r != nil && r.Scouting() && !r.Gone() && s.scouting(w, t, r) {
			home = append(home, r)
		}
	}
	// A cell whose scouts went home rejoins the faction it split off:
	// it never arrived, so nothing else names it.
	if len(home) > 0 {
		w.Rivals = slices.DeleteFunc(w.Rivals, func(r *game.RivalState) bool {
			return slices.Contains(home, r) && r.Cell != ""
		})
	}
	for _, cid := range w.CityOrder {
		if cid == w.Home().ID || s.Take(w, cid) < e.TakeMin || s.lived(w, cid) {
			continue
		}
		s.sendScouts(w, t, cid)
	}
}

// lived reports whether a faction lives in a city, or is on its way
// there: no second faction is drawn to it.
func (s *Sim) lived(w *game.World, city string) bool {
	for _, r := range w.Rivals {
		if r != nil && !r.Gone() && w.CityOf(r).ID == city {
			return true
		}
	}
	return false
}

// sendScouts is the first stage: the faction that moves on city, a seat
// still in the wings (the last to come) or a cell of the strongest
// faction at home, its home moved there, its scouts seen.
func (s *Sim) sendScouts(w *game.World, t *game.Tick, city string) {
	var r *game.RivalState
	for i := len(w.Rivals) - 1; i > 0; i-- {
		o := w.Rivals[i]
		if o != nil && o.Arrived == 0 && !o.Gone() && o.Home == "" && o.ScoutingCity == "" && o.Leader != "" {
			r = o
			break
		}
	}
	if r == nil {
		parent := w.StrongestFaction(w.Home().ID)
		if parent == nil || !parent.Alive() {
			return
		}
		r = s.split(w, t, parent)
	}
	r.Home, r.ScoutingCity, r.ScoutDay = city, city, t.Day
	if r.Cell != "" {
		r.Cash = s.cost(w, r, s.cfg.Rivals.StartCash) // the chest a seat arrives with, at the city's prices
	}
	w.Stats.Moves++
	s.fileScouts(w, t, r)
	t.Emit(events.RivalScouting{Day: t.Day, City: city, Rival: r.Leader, Faction: r.Faction(), Cell: r.Cell, Recruit: t.Day + s.cfg.Expansion.ScoutDays, Arrive: s.ArriveDay(w, r)})
}

// split is a cell of parent going its own way (#341): a new faction at
// the end of the table with a leader nobody else has (off the expansion
// stream), the parent's temper, connect and trust in you, and
// start_muscle heads, as many of them the parent's as it can spare
// without its last.
func (s *Sim) split(w *game.World, t *game.Tick, parent *game.RivalState) *game.RivalState {
	rng := t.Sub(game.StreamExpansion)
	n := len(w.Rivals) + 1
	for w.FactionIndex(fmt.Sprintf("f%d", n)) >= 0 {
		n++
	}
	var used, free []string
	for _, o := range w.Rivals {
		if o != nil {
			used = append(used, o.Leader)
		}
	}
	for _, name := range s.names {
		if !slices.Contains(used, name) {
			free = append(free, name)
		}
	}
	r := &game.RivalState{ID: fmt.Sprintf("f%d", n), Leader: fmt.Sprintf("Faction %d", n), Personality: parent.Personality, Supplier: parent.Supplier, Trust: parent.Trust, Cell: parent.Faction()}
	if len(free) > 0 {
		r.Leader = free[rng.IntN(len(free))]
	}
	tun := s.cfg.Rivals
	parent.Muscle -= min(tun.StartMuscle, max(0, parent.Muscle-1))
	r.Muscle = tun.StartMuscle
	w.Rivals = append(w.Rivals, r)
	return r
}

// fileScouts files what you know of the move: the city and the day it
// arrives, seen, held until it arrives or goes home.
func (s *Sim) fileScouts(w *game.World, t *game.Tick, r *game.RivalState) {
	f := game.Fact{Subject: r.Faction(), Kind: game.FactScout, Value: r.ScoutingCity, Number: float64(s.ArriveDay(w, r)), Confidence: 1, Day: t.Day, Source: game.SourceSeen, Forget: s.intel.Forget}
	w.Learn(f)
	t.Emit(events.IntelGained{Day: t.Day, Subject: f.Subject, FactKind: f.Kind, Value: f.Value, Confidence: f.Confidence, Source: f.Source, Name: r.Leader})
}

// scouting is one faction's night on its way (#341): the offers it made
// lapse or are sealed, a hit on its scouts lands, its scouts go home if
// the take fell back before it recruited (it reports true), and the
// recruiting begins scout_days after the scouts came, and then goes on
// each night until it arrives.
func (s *Sim) scouting(w *game.World, t *game.Tick, r *game.RivalState) bool {
	e := s.cfg.Expansion
	id, city := r.Faction(), r.ScoutingCity
	w.Offers = slices.DeleteFunc(w.Offers, func(o game.Offer) bool { return o.With() == id && t.Day > o.Expires })
	for _, o := range w.Today.Accepted {
		if o.With() == id && w.DealWith(id, o.Deal.Kind) == nil {
			d := o.Deal
			d.Offered = true
			s.seal(w, t, r, d)
		}
	}
	// A tribute taken on the way is paid from the night it is sealed, as
	// table pays it once it is on the ground: missed, it is a betrayal
	// and a phone call.
	if d := w.DealWith(id, game.DealTribute); d != nil && d.Since < t.Day {
		if w.Player.DirtyCash < d.Terms.PerDay {
			s.betray(w, t, r, *d, "the tribute went unpaid")
			r.Tips++
			t.Emit(events.RivalTippedPolice{Day: t.Day, Rival: r.Leader, Faction: id, Heat: s.cfg.Rivals.TipHeat})
		} else {
			w.Player.DirtyCash -= d.Terms.PerDay
			r.Cash += d.Terms.PerDay
			w.Stats.Tribute += d.Terms.PerDay
			t.Emit(events.TributePaid{Day: t.Day, Rival: r.Leader, Faction: id, Amount: d.Terms.PerDay})
		}
	}
	if w.Today.HitScouts == id && r.ScoutsHit == 0 {
		r.ScoutsHit = t.Day
		r.ScoutDay += e.SetbackDays
		r.Grudge++
		r.Observed = true
		if s.breakAll(w, t, r, "your enforcers hit their scouts") {
			r.Tips++
			t.Emit(events.RivalTippedPolice{Day: t.Day, Rival: r.Leader, Faction: id, Heat: s.cfg.Rivals.TipHeat})
		}
		s.fileScouts(w, t, r)
		t.Emit(events.ScoutsHit{Day: t.Day, City: city, Rival: r.Leader, Faction: id, Setback: e.SetbackDays, Arrive: s.ArriveDay(w, r)})
	}
	if r.Recruited == 0 && s.Take(w, city) < e.TakeMin {
		s.withdraw(w, t, r)
		return true
	}
	if r.Recruited == 0 && t.Day >= r.ScoutDay+e.ScoutDays {
		r.Recruited = t.Day
		t.Emit(events.RivalRecruiting{Day: t.Day, City: city, Rival: r.Leader, Faction: id, Bite: e.Bite, Arrive: s.ArriveDay(w, r)})
		if !s.offering(w, id) && w.DealWith(id, game.DealTribute) == nil && w.HeldIn(city) > 0 {
			s.putOffer(w, t, r, game.Deal{Kind: game.DealTribute, Terms: game.Terms{PerDay: s.cut(w, r, s.cfg.Diplomacy.TributeCuts[1])}})
		}
	}
	if r.Recruited > 0 {
		s.recruit(w, t, r)
	}
	return false
}

// withdraw is the scouts going home (#341): the take fell back under
// the line before the recruiting. A seat goes back to the wings at
// home, to arrive on its own day; a cell rejoins the faction it split
// off (the caller drops it from the table), its heads with it. The
// city's window starts over, so the next move there waits on a fresh
// window over the line.
func (s *Sim) withdraw(w *game.World, t *game.Tick, r *game.RivalState) {
	city := r.ScoutingCity
	t.Emit(events.RivalWithdrew{Day: t.Day, City: city, Rival: r.Leader, Faction: r.Faction()})
	w.Stats.Withdrew++
	w.Unlearn(r.Faction(), game.FactScout)
	w.Offers = slices.DeleteFunc(w.Offers, func(o game.Offer) bool { return o.With() == r.Faction() })
	delete(w.Takes, city)
	r.Home, r.ScoutingCity, r.ScoutDay, r.Recruited, r.ScoutsHit = "", "", 0, 0, 0
	if r.Cell != "" {
		if p := w.Faction(r.Cell); p != nil && !p.Gone() {
			p.Muscle += r.Muscle
		}
		r.Muscle = 0
	}
}

// recruit is the recruiters working on your crew in the city (#341):
// at the table's poach_chance a night, off the expansion stream (the
// roll made first, whoever is there), the least loyal member posted in
// the city is offered poach_mul of their wage; under poach_line they
// go, over it they stay a little less loyal. The crew sim reads the
// CrewPoached as it reads the table's.
func (s *Sim) recruit(w *game.World, t *game.Tick, r *game.RivalState) {
	f := s.cfg.Factions
	if f.PoachChance <= 0 || t.Sub(game.StreamExpansion).Float64() >= f.PoachChance {
		return
	}
	pick := -1
	for i, m := range w.Crew.Members {
		if m.Runs() {
			continue
		}
		if p := w.PostOf(m.ID); p == nil || p.City != r.ScoutingCity {
			continue
		}
		if pick < 0 || m.Loyalty < w.Crew.Members[pick].Loyalty {
			pick = i
		}
	}
	if pick < 0 {
		return
	}
	m := w.Crew.Members[pick]
	ev := events.CrewPoached{Day: t.Day, ID: m.ID, Name: m.Name, Role: m.Role, Rival: r.Leader, Faction: r.Faction(), Wages: s.PoachOffer(m)}
	if m.Loyalty >= f.PoachLine {
		ev.Stayed, ev.Dip = true, f.PoachDip
	} else {
		w.Stats.CrewPoached++
	}
	t.Emit(ev)
}

// landed closes the expansion the night a faction arrives in the city
// it moved on (#341): counted, its move no longer news.
func (s *Sim) landed(w *game.World, t *game.Tick) {
	for _, r := range w.Rivals {
		if r == nil || r.ScoutingCity == "" || r.Arrived != t.Day {
			continue
		}
		w.Stats.Expanded++
		w.Unlearn(r.Faction(), game.FactScout)
		r.ScoutingCity, r.ScoutDay, r.Recruited, r.ScoutsHit = "", 0, 0, 0
	}
}

// forceIn is a faction due in a city where you left it no free corner
// (#341), arrive_grace days late: one push a night on your least
// guarded corner there, on its own stream, at the push odds (its whole
// muscle, no front line yet). Landing, it has arrived there (the corner
// is a corner taken off you, and its move-in); held off, it may lose a
// head and comes back tomorrow. At peace with you it waits: you pay it
// to stay out.
func (s *Sim) forceIn(w *game.World, t *game.Tick, r *game.RivalState, rng game.Rand) {
	if w.AtPeaceWith(r.Faction()) || r.Muscle == 0 {
		return
	}
	var c *game.Corner
	ground := s.corners(w, r)
	for i := range ground {
		k := &ground[i]
		if k.Owner != game.OwnerPlayer || s.offLimits(w, r, k) {
			continue
		}
		if c == nil || s.Guard(w, k) < s.Guard(w, c) {
			c = k
		}
	}
	if c == nil {
		return
	}
	r.Observed = true
	r.War += s.cfg.Rivals.PushWar
	if rng.Float64() < s.PushOdds(w, r, c) {
		s.take(w, r, c, t.Day)
		r.Arrived = t.Day
		r.Flips++
		r.LastFlip = t.Day
		w.Stats.CornersLost++
		t.Emit(events.CornerTaken{Day: t.Day, Corner: c.ID, Name: c.Name, Rival: r.Leader, Faction: r.Faction(), From: game.OwnerPlayer})
		t.Emit(events.RivalMovedIn{Day: t.Day, Rival: r.Leader, Faction: r.Faction(), Corner: c.ID, Name: c.Name})
		return
	}
	if c.Enforcer != 0 && rng.Float64() < 0.5 {
		r.Muscle-- // held off, and it cost them
	}
	t.Emit(events.RivalPushed{Day: t.Day, Corner: c.ID, Name: c.Name, Rival: r.Leader, Faction: r.Faction()})
}

// late reports whether a faction on its way is arrive_grace days past
// the day it was due (#341): a city whose free corners you have all
// worked once is not a city it waits on for ever.
func (s *Sim) late(w *game.World, r *game.RivalState, day int) bool {
	return r.Scouting() && day >= s.ArriveDay(w, r)+s.cfg.Pace.ArriveGrace
}
