package crew

import (
	"math"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
)

// investigate runs tonight's investigation, if one was paid for.
func (s *Sim) investigate(n *night) {
	w, t, c := n.w, n.t, n.c
	// The investigation: it names an informant with the odds the UI
	// showed, or nobody, and being asked costs everyone a little loyalty
	// either way when it comes up empty.
	if o := w.Today.Investigation; o != nil {
		ev := events.InvestigationRun{Day: t.Day, Cost: o.Cost}
		w.Stats.Investigations++
		if c.Informants() > 0 && t.RNG.Float64() < s.InvestigateOdds(w) {
			pick := t.RNG.IntN(c.Informants())
			for _, m := range c.Members {
				if !m.Informant {
					continue
				}
				if pick == 0 {
					ev.Found, ev.Name = true, m.Name
					c.Exposed = m.ID
				}
				pick--
			}
			c.Investigated = 0
		} else {
			n.asked = true
			c.Investigated++
		}
		t.Emit(ev)
	}
}

// drift moves every member's loyalty for the day.
func (s *Sim) drift(n *night) {
	w, t, c, tun, fx := n.w, n.t, n.c, n.tun, n.fx
	inf := s.cfg.Informant
	// Loyalty drift: pay, greed, danger, firings, unpaid wages, an
	// investigation that named nobody, and for the enforcers, the strike
	// they went on today: a toll from the rivals sim that the nervous feel
	// most and a win halves. A respected boss's crew feel every loss less.
	toll := 0.0
	hurt := 0
	for _, e := range t.Events() {
		switch ev := e.(type) {
		case events.CornerStruck:
			if ev.Taken {
				toll += ev.Toll / 2
			} else {
				toll += ev.Toll
			}
		case events.RivalBoosted:
			// A boost (#70) is the enforcers going in too: its toll by
			// nerve as a strike's, and a failure against real muscle
			// hurts the one with the least nerve.
			toll += ev.Toll
			hurt += ev.Hurt
		}
	}
	if hurt > 0 {
		var worst *game.CrewMember
		for i := range c.Members {
			m := &c.Members[i]
			if m.Role == game.RoleEnforcer && m.Working() && (worst == nil || m.Nerve < worst.Nerve) {
				worst = m
			}
		}
		if worst != nil {
			worst.Skill = max(1, worst.Skill-hurt)
		}
	}
	danger := false
	for _, level := range []string{content.Sting, content.Raid, content.TaskForce} {
		if d, ok := w.Heat.LastResponse[level]; ok && t.Day-d <= tun.DangerDays {
			danger = true
		}
	}
	shield := math.Pow(1-s.cfg.Role[game.RoleEnforcer].Protection, float64(n.enforcers))
	loss := s.loyaltyLoss(w, fx)
	base := s.cfg.PayFor(c.Pay).Loyalty
	base -= tun.FireLoyalty * float64(n.fired)
	if n.short > 0 {
		base -= tun.UnpaidLoyalty
	}
	if n.asked {
		base -= inf.InvestigateLoyalty
	}
	for i := range c.Members {
		m := &c.Members[i]
		d := base - tun.GreedDrift*float64(m.Greed)/100
		if danger {
			d -= tun.DangerLoyalty * fx.DangerLoyaltyMul * float64(100-m.Nerve) / 100 * shield
		}
		if m.Role == game.RoleEnforcer {
			d -= toll * float64(100-m.Nerve) / 100
		}
		if d < 0 {
			d *= loss
		}
		m.Loyalty = max(0, min(100, m.Loyalty+d))
	}
}

// quit sees off whoever walks or defects tonight, and whoever goes to a
// faction.
func (s *Sim) quit(n *night) {
	w, t, c, tun, fx := n.w, n.t, n.c, n.tun, n.fx
	// Quitting, or defecting: whoever walks leaves their corner
	// unworked, and while the rival holds ground in the city they go to
	// it instead, and walk it onto that corner (the rival sim acts on
	// the lead next step, off c.Leads: last night's are consumed by now,
	// the rival steps first, so tonight's start the queue afresh; #144)
	// if it is one the rival fights over: the rival lives at home, so a
	// corner in another city is just a corner left. A lieutenant running
	// a city walks with it.
	c.Leads = nil
	poached := s.factions(w, t, c, fx)
	kept := c.Members[:0]
	var gone []game.CrewMember // whoever walked or defected: their kin remember it (#46)
	for _, m := range c.Members {
		if poached[m.ID] {
			continue // gone to a faction tonight (#43)
		}
		if m.Loyalty > tun.QuitThreshold {
			kept = append(kept, m)
			continue
		}
		gone = append(gone, m)
		if m.Runs() {
			delete(n.acted, m.ID)
			s.walk(w, t, m)
			continue
		}
		post := w.PostOf(m.ID)
		w.Recall(m.ID)
		// The faction they go to (#43): the one holding most of the
		// city they stood in, or of home with no post; none holding
		// ground and they just quit.
		city := w.Home().ID
		if post != nil {
			city = post.City
		}
		to := w.StrongestFaction(city)
		if to == nil && city != w.Home().ID {
			to = w.StrongestFaction(w.Home().ID)
		}
		if to == nil {
			t.Emit(events.CrewQuit{Day: t.Day, Name: m.Name, Role: m.Role})
			continue
		}
		ev := events.CrewDefected{Day: t.Day, Name: m.Name, Role: m.Role, Rival: to.Leader, Faction: to.Faction()}
		lead := game.Lead{Name: m.Name, Faction: to.Faction()}
		if post != nil && post.City == w.CityOf(to).ID {
			ev.Corner, ev.CornerName = post.ID, post.Name
			lead.Corner = post.ID
		}
		c.Leads = append(c.Leads, lead)
		w.Stats.Defections++
		t.Emit(ev)
	}
	c.Members = kept
	for _, m := range gone {
		s.kinLoyalty(w, m, -s.cfg.Life.KinLoyalty)
	}
}
