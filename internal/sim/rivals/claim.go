// Setting up on free ground: the arrival (step 1), the claim roll and
// its tell (step 4, #69), and the pick of the corner.

package rivals

import (
	"sort"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
)

// arrive is a faction's day before it has arrived (step 1): nothing
// before arrive_day; from it, the first day with a free corner to stand
// on and a seat left at the table's share, it sets up there. Either way
// it undercuts what it contests, and the rest of its step waits.
func (s *Sim) arrive(w *game.World, t *game.Tick, r *game.RivalState, rng game.Rand) {
	if t.Day < s.ArriveDay(w, r) {
		s.undercut(w, t, r)
		return
	}
	if c := s.pickFree(w, r, rng, t.Day, true); c != nil && !s.TableFull(w, r) {
		s.take(w, r, c, t.Day)
		r.Arrived = t.Day
		r.Claims++
		t.Emit(events.RivalMovedIn{Day: t.Day, Rival: r.Leader, Faction: r.Faction(), Corner: c.ID, Name: c.Name})
	}
	s.undercut(w, t, r)
}

// claim is today's claim roll (step 4): a free corner, by personality,
// up to what it wants, at the pace (#60): faster the more of the city
// you hold, slower the more you are feared, and never twice within
// claim_cooldown days unless your enforcers have been in within those
// days, when it grows as fast as it can (the roll is made either way,
// so the seed's dice stay put; its arrival is not a claim, so the
// second corner comes as it likes). The claim is telegraphed (#69): the
// corner it eyed yesterday is resolved first (resolveEyeing), then
// today's roll picks the next one and gives the tell. No new dice: the
// roll is the one made before, its corner taken a day later; and the
// cooldown runs from the tell, the day it chose, so the pace #60 set is
// the pace it keeps (a claim a day later is not a claim cycle a day
// longer), and a claim kept off a corner rests it too.
func (s *Sim) claim(w *game.World, t *game.Tick, r *game.RivalState, rng game.Rand, pc content.PersonalityConfig) {
	held := w.RivalHeldBy(r.Faction())
	if held >= s.MaxCorners(w, r) || (r.Routed != 0 && t.Day-r.Routed < s.cfg.Rivals.RegroupDays) {
		return
	}
	// One roll, as it always was; the fear's cut is read off it
	// either way (#233): a roll under the chance and over what fear
	// leaves it is a claim your name turned, reported and nothing
	// else.
	roll := rng.Float64()
	full := pc.ClaimChance * s.ClaimScale(w, r) / float64(s.Crowd(w, r))
	switch {
	case roll < full*s.ClaimPace(w):
		// The chest is asked after the roll (#139): a rival that cannot
		// pay for a corner today rolls all the same, so the seed's dice
		// do not move when its money binds. Nor does the table's cap
		// (#43): a table that holds its share of the city rolls and
		// sets up nowhere.
		if s.Rested(r, t.Day) && r.Cash >= s.ClaimCost(w, r) && !s.TableFull(w, r) {
			if c := s.pickFree(w, r, rng, t.Day, held == 0); c != nil {
				r.Eyeing, r.EyeingDay, r.LastClaim = c.ID, t.Day, t.Day
				t.Emit(events.RivalEyeing{Day: t.Day, Rival: r.Leader, Faction: r.Faction(), Corner: c.ID, Name: c.Name})
			}
		}
	case roll < full:
		t.Emit(events.ClaimDeterred{Day: t.Day, City: w.CityOf(r).ID, Rival: r.Leader, Faction: r.Faction()})
	}
}

// resolveEyeing is the claim the rival telegraphed yesterday (#69): it
// sets up on the corner it eyed, claim_cost spent, unless somebody got
// there first, when the claim fails, no cash is spent and it holds a
// grudge (RivalOutbid). A body on the corner is what a Post puts there:
// you, a runner or an enforcer, since a corner with an enforcer on it
// is held and it never walks onto held ground without a push. A corner
// it can no longer set up on for its own reasons (a split now covers
// it, it holds its share, it was routed, the corner is no longer free
// and not yours either, or its chest no longer covers the claim at
// today's prices, #139) is dropped without a word.
func (s *Sim) resolveEyeing(w *game.World, t *game.Tick, r *game.RivalState) {
	tun := s.cfg.Rivals
	if r.Eyeing == "" {
		return
	}
	id := r.Eyeing
	r.Eyeing, r.EyeingDay = "", 0
	c := w.Corner(id)
	if c == nil || c.City != s.city(w, r).ID || c.Owner == game.OwnerRival {
		return
	}
	if split := w.DealWith(r.Faction(), game.DealSplit); split != nil && split.Covers(c.ID) {
		return
	}
	if w.RivalHeldBy(r.Faction()) >= s.MaxCorners(w, r) || (r.Routed > 0 && t.Day-r.Routed < tun.RegroupDays) || r.Cash < s.ClaimCost(w, r) || s.TableFull(w, r) {
		return
	}
	if c.Held() {
		r.Grudge += tun.OutbidGrudge
		t.Emit(events.RivalOutbid{Day: t.Day, Rival: r.Leader, Faction: r.Faction(), Corner: c.ID, Name: c.Name})
		return
	}
	r.Cash -= s.ClaimCost(w, r)
	r.Claims++
	s.take(w, r, c, t.Day)
	t.Emit(events.CornerTaken{Day: t.Day, Corner: c.ID, Name: c.Name, Rival: r.Leader, Faction: r.Faction(), From: game.OwnerNone})
}

// Eyeing is the corner the rival at home sets up on next step, or nil:
// the tell (#69) the map marks and the panels name.
func (s *Sim) Eyeing(w *game.World) *game.Corner { return s.EyeingBy(w, w.Rival()) }

// EyeingBy is the corner a faction sets up on next step, or nil.
func (s *Sim) EyeingBy(w *game.World, r *game.RivalState) *game.Corner {
	if r == nil || r.Eyeing == "" {
		return nil
	}
	return w.Corner(r.Eyeing)
}

// pickFree chooses the free corner the faction sets up on. Arriving (or
// starting over) it takes the biggest one that does not border the player
// if there is one, so it grows toward you. After that its personality
// says: the biggest free corner anywhere, one next to its own, or any.
// Arriving, and for arrive_grace days after, a corner you have ever
// worked is not one it sets up on (#60).
func (s *Sim) pickFree(w *game.World, r *game.RivalState, rng game.Rand, day int, arriving bool) *game.Corner {
	var free, quiet, adjacent []*game.Corner
	ground := s.corners(w, r)
	split := w.DealWith(r.Faction(), game.DealSplit)
	grace := arriving || (r.Arrived > 0 && day-r.Arrived < s.cfg.Pace.ArriveGrace)
	for i := range ground {
		c := &ground[i]
		if c.Owner != game.OwnerNone || (split != nil && split.Covers(c.ID)) || (grace && c.Yours) {
			continue
		}
		free = append(free, c)
		next, own := false, false
		for _, o := range ground {
			if c.Borders(o) {
				next = next || o.Held()
				own = own || owns(o, r)
			}
		}
		if !next {
			quiet = append(quiet, c)
		}
		if own {
			adjacent = append(adjacent, c)
		}
	}
	if len(free) == 0 {
		return nil
	}
	pool := free
	grow := s.personality(r).Grow
	switch {
	case arriving:
		if len(quiet) > 0 {
			pool = quiet
		}
	case grow == "adjacent":
		if len(adjacent) == 0 {
			return nil
		}
		pool = adjacent
	}
	if grow == "random" {
		return pool[rng.IntN(len(pool))]
	}
	sort.SliceStable(pool, func(i, j int) bool { return pool[i].Demand > pool[j].Demand })
	return pool[0]
}
