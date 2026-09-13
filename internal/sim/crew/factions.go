package crew

import (
	"math"

	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
)

// factions is the crew sim's one block for the table (#43), reading the
// rivals sim's events off the tick the way it reads CornerStruck's
// toll: a CrewPoached that landed drops the member and queues the lead
// the faction acts on next step (the defection of #13 with a faction
// named); one that did not dips their loyalty; and a
// RivalLeaderArrested puts the faction's muscle in the hiring pool as
// enforcers at the discount the event's faction sold them for, extra
// faces beside the pool's own (refill counts them out, as it counts
// the chemist), drawn off their own stream. It returns the ids of the
// members poached tonight, for the roster loop to drop. Nothing here
// writes into a faction.
func (s *Sim) factions(w *game.World, t *game.Tick, c *game.CrewState, fx game.Effects) map[int]bool {
	var gone map[int]bool
	for _, e := range t.Events() {
		switch ev := e.(type) {
		case events.CrewPoached:
			m := c.Member(ev.ID)
			if m == nil {
				continue
			}
			if ev.Stayed {
				m.Loyalty = math.Max(0, m.Loyalty-ev.Dip)
				continue
			}
			post := w.PostOf(m.ID)
			w.Recall(m.ID)
			lead := game.Lead{Name: m.Name, Faction: ev.Faction}
			if to := w.Faction(ev.Faction); post != nil && to != nil && post.City == w.CityOf(to).ID {
				lead.Corner = post.ID
			}
			c.Leads = append(c.Leads, lead)
			if gone == nil {
				gone = map[int]bool{}
			}
			gone[m.ID] = true
		case events.RivalLeaderArrested:
			rng := t.Sub("fragment")
			for i := 0; i < ev.Muscle; i++ {
				m := s.generate(w, rng, nil, fx)
				m.Role, m.Units, m.Personality = "enforcer", 0, ""
				m.Former = ev.Faction
				m.Fee = int(math.Round(float64(m.Fee) * s.fac.FragmentDiscount))
				c.Candidates = append(c.Candidates, m)
			}
		}
	}
	return gone
}

// former reports whether a candidate is a fragmented faction's muscle
// (#43): an extra face the pool's count leaves out.
func former(m game.CrewMember) bool { return m.Former != "" }
