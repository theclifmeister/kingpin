// The war: the war order's target and its end (#229), the war getting
// louder (step 7) and the police's crackdown on a loud one.

package rivals

import (
	"math"
	"sort"

	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
)

// WarTarget is the corner the war order strikes tonight (#229): of the
// faction's corners in a city you hold ground in, one bordering a
// corner you hold before one that does not, the biggest first, ties in
// the map's order; nil with none, which ends the war. No dice.
func (s *Sim) WarTarget(w *game.World, r *game.RivalState) *game.Corner {
	var best *game.Corner
	front := false
	for _, cid := range w.CityOrder {
		city := w.Cities[cid]
		if w.HeldIn(cid) == 0 {
			continue
		}
		for i := range city.Corners {
			c := &city.Corners[i]
			if !owns(*c, r) {
				continue
			}
			line := false
			for _, o := range city.Corners {
				if o.Held() && c.Borders(o) {
					line = true
					break
				}
			}
			if best == nil || (line && !front) || (line == front && c.Demand > best.Demand) {
				best, front = c, line
			}
		}
	}
	return best
}

// war ends the war order the night it has nothing left to fight (#229):
// the faction gone, paying homage, or holding no corner in a city you
// hold ground in. A read, no dice; the run never ends here.
func (s *Sim) war(w *game.World, t *game.Tick) {
	if w.War == "" {
		return
	}
	r := w.Faction(w.War)
	why := ""
	switch {
	case r == nil || r.Gone():
		why = "they are no more"
	case w.DealWith(r.Faction(), game.DealHomage) != nil:
		why = "they pay you homage now"
	case s.WarTarget(w, r) == nil:
		why = "they hold no corner left in a city you hold ground in"
	}
	if why == "" {
		return
	}
	ev := events.WarEnded{Day: t.Day, Faction: w.War, Why: why}
	w.War = ""
	if r != nil {
		ev.Rival, ev.Faction = r.Leader, r.Faction()
	}
	t.Emit(ev)
}

// warOrder is the strike the war order (#229) sends on a faction tonight
// when the hand sent none: the dial's force on WarTarget; nil with no
// war on it, no enforcer on the crew (nobody at work, no order: the hand
// could send none either), the dial off or nothing left to hit. No dice.
func (s *Sim) warOrder(w *game.World, r *game.RivalState) *game.StrikeOrder {
	if !w.AtWarWith(r) || w.Crew.Role(game.RoleEnforcer) == 0 {
		return nil
	}
	force, on := s.cfg.War.Force()
	if !on {
		return nil
	}
	c := s.WarTarget(w, r)
	if c == nil {
		return nil
	}
	return &game.StrikeOrder{Corner: c.ID, Force: force, War: true}
}

// escalate is the war at the end of the night's fighting (step 7):
// crossing war_threshold tonight makes headlines; at crackdown_threshold
// the police clear both sides (crackdown); otherwise it fades a little.
func (s *Sim) escalate(w *game.World, t *game.Tick, r *game.RivalState, warBefore float64) {
	tun := s.cfg.Rivals
	if warBefore < tun.WarThreshold && r.War >= tun.WarThreshold {
		t.Emit(events.WarEscalated{Day: t.Day, Stage: events.StageOpen, War: r.War})
	}
	if r.War >= tun.CrackdownThreshold {
		s.crackdown(w, t, r)
	} else {
		r.War -= r.War * tun.WarDecay
	}
}

// crackdown is the police ending a loud war: each side loses corners,
// contested ones first (decided before anything is cleared, so both
// sides lose their front line), the faction loses muscle and the player
// draws heat.
func (s *Sim) crackdown(w *game.World, t *game.Tick, r *game.RivalState) {
	tun := s.cfg.Rivals
	ev := events.WarEscalated{Day: t.Day, Stage: events.StageCrackdown, War: r.War, Heat: tun.CrackdownHeat}
	var cleared []*game.Corner
	ground := s.corners(w, r)
	for _, owner := range []string{game.OwnerPlayer, game.OwnerRival} {
		var held []*game.Corner
		for i := range ground {
			if c := &ground[i]; c.Owner == owner && (owner != game.OwnerRival || owns(*c, r)) {
				held = append(held, c)
			}
		}
		sort.SliceStable(held, func(i, j int) bool {
			ci, cj := w.Contested(*held[i]), w.Contested(*held[j])
			if ci != cj {
				return ci
			}
			return held[i].Since > held[j].Since
		})
		cleared = append(cleared, held[:min(len(held), tun.CrackdownCorners)]...)
	}
	for _, c := range cleared {
		owner, faction := c.Owner, c.Faction
		c.Hand(game.OwnerNone, "", t.Day)
		ev.Lost = append(ev.Lost, c.Name)
		t.Emit(events.CornerLost{Day: t.Day, Corner: c.ID, Name: c.Name, Reason: "crackdown", Owner: owner, Faction: faction})
	}
	r.Muscle = int(math.Round(float64(r.Muscle) * (1 - tun.CrackdownMuscle)))
	r.War = 0
	if w.RivalHeldBy(r.Faction()) == 0 && r.Routed < t.Day {
		r.Routed = t.Day
		r.LastTakenBy = ""
	}
	t.Emit(ev)
}
