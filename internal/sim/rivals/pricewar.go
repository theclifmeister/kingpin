// The price war (#68) from the faction's side: its undercut on the
// corners of yours it contests, and its answer to yours on its own.

package rivals

import (
	"math"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
)

// Pricewar exposes the price war's tuning for the UI and the harness.
func (s *Sim) Pricewar() content.PricewarTuning { return s.cfg.Pricewar }

// undercut marks the player's corners the faction contests as squeezed
// and drags the street price by the share of worked demand that is
// contested. The squeeze on the factions' own corners is the market
// sim's (#68, the player's price war) and is left alone; the player's
// corners' squeeze is zeroed once a step before any faction writes it,
// and two factions on one corner leave the deeper cut.
func (s *Sim) undercut(w *game.World, t *game.Tick, r *game.RivalState) {
	tun := s.cfg.Rivals
	share := s.personality(r).Undercut
	var names []string
	total, squeezed := 0.0, 0.0
	ground := s.corners(w, r)
	for i := range ground {
		c := &ground[i]
		if c.Owner == game.OwnerRival {
			continue
		}
		if !c.Held() || !w.ContestedBy(*c, r.Faction()) || s.offLimits(w, r, c) {
			continue
		}
		c.Squeeze = math.Max(c.Squeeze, share)
		names = append(names, c.Name)
		if c.Worked() {
			squeezed += c.Demand
		}
	}
	for _, c := range ground {
		if c.Worked() {
			total += c.Demand
		}
	}
	if len(names) == 0 {
		return
	}
	if total > 0 && squeezed > 0 {
		connect := (1 - r.Supplier) / math.Max(0.01, 1-tun.SupplierMin) // 1 for the best connect it can have
		drag := tun.UndercutPrice * connect * squeezed / total
		for _, m := range s.city(w, r).Market {
			m.Price *= 1 - drag
		}
	}
	t.Emit(events.RivalUndercut{Day: t.Day, Rival: r.Leader, Faction: r.Faction(), Corners: names, Share: share})
}

// pricewar is the faction's side of the player's price war (#68). Every
// corner of its the market squeezed today (Corner.Squeeze, the share of
// its trade the player's orders next door took) is a day starved
// (Corner.Starved, StarvedDay); a rest of more than pricewar_days days
// forgets the count, so a war fought every other day still bites, only
// later. An undercut day costs war, once for the day whatever the
// corners. A corner starved pricewar_days days gets an answer by
// personality, and the answer, when it comes, holds a grudge if the
// tuning says (so tip_chance applies: a grudge a day made the price war
// hotter than a hit war, fourteen calls in 120 days): a defensive or an
// expansionist
// rival pushes on the player corner doing the cutting (the biggest
// worked one next door) at push_chance times pricewar_push, the usual
// push odds deciding whether it takes it; an opportunist abandons the
// corner (RivalAbandoned) and sets up elsewhere as the claim step lets
// it; a chaotic one rolls between the two. The count starts over after
// an answer. Nothing here rolls dice on a corner nobody undercut, so a
// run that never does is the old run.
func (s *Sim) pricewar(w *game.World, t *game.Tick, r *game.RivalState, rng rand) {
	tun := s.cfg.Pricewar
	ground := s.corners(w, r)
	starved := false
	for i := range ground {
		c := &ground[i]
		if !owns(*c, r) || c.Squeeze <= 0 {
			continue
		}
		if t.Day-c.StarvedDay > tun.PricewarDays {
			c.Starved = 0
		}
		c.Starved++
		c.StarvedDay = t.Day
		starved = true
	}
	if !starved {
		return
	}
	r.Observed = true
	r.War += tun.War
	pc := s.personality(r)
	for i := range ground {
		c := &ground[i]
		if !owns(*c, r) || tun.PricewarDays <= 0 || c.Starved < tun.PricewarDays {
			continue
		}
		abandon := false
		switch r.Personality {
		case "opportunist":
			abandon = true
		case "chaotic":
			abandon = rng.Float64() < 0.5
		}
		if abandon {
			if tun.Grudge {
				r.Grudge++
			}
			c.Hand(game.OwnerNone, "", t.Day)
			t.Emit(events.RivalAbandoned{Day: t.Day, Rival: r.Leader, Faction: r.Faction(), Corner: c.ID, Name: c.Name, Reason: "pricewar"})
			if w.RivalHeldBy(r.Faction()) == 0 && r.Routed < t.Day {
				r.Routed = t.Day
				r.LastTakenBy = ""
			}
			continue
		}
		// The push: on the biggest corner of yours working it cheap.
		target := s.cutter(w, r, *c)
		if target == nil || r.Muscle == 0 || s.offLimits(w, r, target) {
			continue
		}
		if rng.Float64() >= pc.PushChance*tun.PricewarPush*s.PushPaceOn(w, target) {
			continue
		}
		if tun.Grudge {
			r.Grudge++
		}
		c.Starved = 0
		r.War += s.cfg.Rivals.PushWar
		if rng.Float64() < s.PushOdds(w, r, target) {
			s.take(w, r, target, t.Day)
			r.Flips++
			r.LastFlip = t.Day
			w.Stats.CornersLost++
			t.Emit(events.CornerTaken{Day: t.Day, Corner: target.ID, Name: target.Name, Rival: r.Leader, Faction: r.Faction(), From: game.OwnerPlayer, Pricewar: true})
			continue
		}
		if target.Enforcer != 0 && rng.Float64() < 0.5 {
			r.Muscle--
		}
		t.Emit(events.RivalPushed{Day: t.Day, Corner: target.ID, Name: target.Name, Rival: r.Leader, Faction: r.Faction(), Pricewar: true})
	}
}

// cutter is the player's corner doing the cutting on a rival corner: the
// biggest worked one next door, or nil.
func (s *Sim) cutter(w *game.World, r *game.RivalState, c game.Corner) *game.Corner {
	ground := s.corners(w, r)
	var best *game.Corner
	for i := range ground {
		o := &ground[i]
		if !o.Worked() || !c.Borders(*o) {
			continue
		}
		if best == nil || o.Demand > best.Demand {
			best = o
		}
	}
	return best
}
