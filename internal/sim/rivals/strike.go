// The fighting on the street: the weight your enforcers bring and the
// faction's muscle meets them with, the odds the pickers show and the
// dice use, the player's strike (step 3), the defectors (3b) and the
// faction's pushes on your corners (5), and take, the one hand-over.

package rivals

import (
	"math"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
)

// Strength is the weight the crew's enforcers bring to a strike: each one
// counts 0.5 + skill/100, and one guarding a corner counts half of that,
// they are busy.
func (s *Sim) Strength(w *game.World) float64 {
	n := 0.0
	for _, m := range w.Crew.Members {
		if m.Role != game.RoleEnforcer || !m.Working() {
			continue // one in a cell or laid up (#46) goes on no strike
		}
		v := 0.5 + float64(m.Skill)/100
		if w.PostOf(m.ID) != nil {
			v /= 2
		}
		n += v
	}
	return n
}

// frontline is how many of a faction's corners border another side's,
// yours or another faction's; its muscle stands there.
func (s *Sim) frontline(w *game.World, r *game.RivalState) int {
	n := 0
	for _, c := range s.corners(w, r) {
		if owns(c, r) && w.Contested(c) {
			n++
		}
	}
	return n
}

// Defence is the muscle a faction puts on one corner when struck: its
// muscle spread over the front line, times its personality's defence.
func (s *Sim) Defence(w *game.World, r *game.RivalState) float64 {
	return s.DefenceAt(w, r, r.Muscle)
}

// DefenceAt is Defence with the muscle given (#45): the strike picker
// reads it at the band the file holds, the dice at the truth.
func (s *Sim) DefenceAt(w *game.World, r *game.RivalState, muscle int) float64 {
	return float64(muscle) / float64(max(1, s.frontline(w, r))) * s.personality(r).Defence
}

// Odds is the chance a strike at a force takes a corner of the faction
// today. The UI's strike picker shows it, so the estimate is what the
// dice use. A defensive faction standing with you against this one
// (#43) lends its muscle to the attack.
func (s *Sim) Odds(w *game.World, r *game.RivalState, force events.Force) float64 {
	return s.OddsOn(w, r, nil, force)
}

// OddsOn is Odds on one corner of the faction's: where you hold the
// deed to its block (#194) their defence of it is cut by push_mul, the
// businessman's way to a corner. The strike and the boost roll on it
// and the picker shows it; nil is Odds.
func (s *Sim) OddsOn(w *game.World, r *game.RivalState, c *game.Corner, force events.Force) float64 {
	return s.OddsOnAt(w, r, c, force, r.Muscle)
}

// OddsOnAt is OddsOn with the faction's muscle given (#45): what the
// strike picker shows for the band the file holds, the dice's shape on
// what you know.
func (s *Sim) OddsOnAt(w *game.World, r *game.RivalState, c *game.Corner, force events.Force, muscle int) float64 {
	fc := s.cfg.ForceFor(force)
	attack := (s.Strength(w) + s.AllyMuscle(w, game.FactionYou, r.Faction())) * fc.Attack
	if attack <= 0 {
		return 0
	}
	return fc.Flip * attack / (attack + s.DefenceAt(w, r, muscle)*s.DeedMul(c))
}

// StrikeHeat is what a strike on a corner at a force draws.
func (s *Sim) StrikeHeat(c *game.Corner, force events.Force) float64 {
	return s.cfg.ForceFor(force).Heat * c.Heat
}

// Guard is the weight of whoever stands on a player corner when it is
// pushed: an enforcer counts 1 + skill/50, you count 1.5, a runner 0.5,
// and the tree's guard_bonus (the front line) is a body on every one,
// so a corner it covers is never walked onto unopposed.
func (s *Sim) Guard(w *game.World, c *game.Corner) float64 {
	g := float64(s.Effects(w).GuardBonus)
	if m := w.Crew.Member(c.Enforcer); m != nil && c.Enforcer != 0 {
		g += 1 + float64(m.Skill)/50
	}
	switch {
	case c.Runner == game.You:
		g += 1.5
	case c.Runner != 0:
		g += 0.5
	}
	return g
}

// PushOdds is the chance one of a faction's pushes flips a player
// corner: its muscle on the front line against whoever is standing
// there.
func (s *Sim) PushOdds(w *game.World, r *game.RivalState, c *game.Corner) float64 {
	return s.PushOddsAt(w, r, c, r.Muscle)
}

// PushOddsAt is PushOdds with the faction's muscle given (#45), for the
// map's inspector at the band the file holds.
func (s *Sim) PushOddsAt(w *game.World, r *game.RivalState, c *game.Corner, muscle int) float64 {
	attack := float64(muscle) / float64(max(1, s.frontline(w, r)))
	defence := s.Guard(w, c)
	if attack <= 0 {
		return 0
	}
	return s.cfg.Rivals.PushFlip * attack / (attack + defence)
}

// struck resolves the player's strike on a faction tonight (step 3),
// before it moves, and reports whether it was a betrayal: a push or a
// hit under a deal is a betrayal of every deal (crossed). The war order
// (#229) is the hand's strike on a night the hand sent none (warOrder):
// the same order, the same roll on the same stream.
func (s *Sim) struck(w *game.World, t *game.Tick, r *game.RivalState, rng rand) bool {
	o := w.Today.Strike
	if o == nil {
		o = s.warOrder(w, r)
	}
	if o == nil {
		return false
	}
	c := w.Corner(o.Corner)
	if c == nil || !owns(*c, r) {
		return false
	}
	s.strike(w, t, r, rng, o, c)
	return s.crossed(w, t, r, o)
}

// defectors walks last night's defectors to this faction in (step 3b,
// off the crew sim's queue, #144): each one joins its muscle, and walks
// it onto the corner they ran if nobody stands there; if somebody does,
// it is a push like any other, with the defector's help counted in.
// Only a corner of the city it fights over: it never sets up elsewhere.
func (s *Sim) defectors(w *game.World, t *game.Tick, r *game.RivalState, rng rand) {
	for _, l := range w.Crew.Leads {
		if w.Faction(l.Faction) != r {
			continue
		}
		r.Muscle++
		r.Observed = true
		c := w.Corner(l.Corner)
		if c == nil || c.City != s.city(w, r).ID || !c.Held() || s.offLimits(w, r, c) {
			continue
		}
		if s.Guard(w, c) == 0 || rng.Float64() < s.PushOdds(w, r, c) {
			s.take(w, r, c, t.Day)
			r.Flips++
			r.LastFlip = t.Day
			w.Stats.CornersLost++
			t.Emit(events.CornerTaken{Day: t.Day, Corner: c.ID, Name: c.Name, Rival: r.Leader, Faction: r.Faction(), From: game.OwnerPlayer, Handed: l.Name})
			continue
		}
		r.War += s.cfg.Rivals.PushWar
		t.Emit(events.RivalPushed{Day: t.Day, Corner: c.ID, Name: c.Name, Rival: r.Leader, Faction: r.Faction()})
	}
}

// push is the faction's pushes on the player's corners it borders (step
// 5): at full pace while it wants more ground or has a grudge to pay
// back, at its personality's pace past that, and slower against a
// player it fears. Every push is noise; one that lands flips the corner
// and sends its people home. A deal keeps it off: every corner under a
// truce or a tribute, your side of the line under a split.
func (s *Sim) push(w *game.World, t *game.Tick, r *game.RivalState, rng rand, pc content.PersonalityConfig) {
	pace := s.PushPace(w)
	ground := s.corners(w, r)
	for i := range ground {
		c := &ground[i]
		if !c.Held() || !w.ContestedBy(*c, r.Faction()) || r.Muscle == 0 || s.offLimits(w, r, c) {
			continue
		}
		chance := pc.PushChance * pace * s.DeedMul(c) / float64(s.Crowd(w, r)) // the deed to the block (#194) slows it here
		if (w.RivalHeldBy(r.Faction()) >= s.MaxCorners(w, r) || !s.Rested(r, t.Day) || s.TableFull(w, r)) && r.Grudge == 0 {
			chance *= pc.PushPastCap // held at its cap, the table at its share (#43), or resting after a claim (#60): the slow pace, not a pause
		}
		if r.Personality == "opportunist" && (c.Enforcer == 0 || s.city(w, r).Heat > 50) {
			chance *= 2
		}
		chance *= 1 + 0.25*float64(min(r.Grudge, 4))
		if rng.Float64() >= chance {
			continue
		}
		r.Observed = true
		r.War += s.cfg.Rivals.PushWar
		s.sideWith(w, r, game.FactionYou)
		if rng.Float64() < s.PushOdds(w, r, c) {
			s.take(w, r, c, t.Day)
			r.Flips++
			r.LastFlip = t.Day
			w.Stats.CornersLost++
			t.Emit(events.CornerTaken{Day: t.Day, Corner: c.ID, Name: c.Name, Rival: r.Leader, Faction: r.Faction(), From: game.OwnerPlayer})
			continue
		}
		if c.Enforcer != 0 && rng.Float64() < 0.5 {
			r.Muscle-- // held off, and it cost them
		}
		t.Emit(events.RivalPushed{Day: t.Day, Corner: c.ID, Name: c.Name, Rival: r.Leader, Faction: r.Faction()})
	}
}

// strike resolves the player's enforcers going in on a corner of the
// faction, or for its takings (#70, boost).
func (s *Sim) strike(w *game.World, t *game.Tick, r *game.RivalState, rng rand, o *game.StrikeOrder, c *game.Corner) {
	if w.Crew.Role(game.RoleEnforcer) == 0 {
		return
	}
	if o.Boost {
		s.boost(w, t, r, rng, o, c)
		return
	}
	fc := s.cfg.ForceFor(o.Force)
	ev := events.CornerStruck{
		Day: t.Day, Corner: c.ID, Name: c.Name, Rival: r.Leader, Faction: r.Faction(), Force: o.Force,
		Heat: s.StrikeHeat(c, o.Force), Toll: fc.Loyalty, War: o.War,
	}
	w.Stats.Strikes++
	r.Observed = true
	r.LastStruck = t.Day
	r.War += fc.War
	r.Trust = math.Max(0, r.Trust-fc.Trust)
	if rng.Float64() < s.OddsOn(w, r, c, o.Force) {
		ev.Taken = true
		c.Hand(game.OwnerPlayer, "", t.Day)
		r.Grudge++
		r.LostToYou++
		r.LastTakenBy = ""
		w.Stats.CornersWon++
		if r.Muscle > 0 {
			r.Muscle--
		}
		if w.RivalHeldBy(r.Faction()) == 0 {
			ev.Routed = true
			r.Routed = t.Day
		}
	}
	t.Emit(ev)
}

// take hands a corner to a faction, naming it on the corner (#144),
// sending whoever was on it home.
func (s *Sim) take(w *game.World, r *game.RivalState, c *game.Corner, day int) {
	c.Hand(game.OwnerRival, r.Faction(), day)
}
