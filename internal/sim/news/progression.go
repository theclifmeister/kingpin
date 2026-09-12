package news

import (
	"fmt"
	"strings"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
)

// tier stamps the tier the run enters this morning (#147) and emits
// TierReached, returning its number, or 0 when the run stays where it
// is. It walks every tier past the highest reached, in order, and
// enters the first whose trigger holds: one a morning at most, no dice.
// A tier once reached stays reached, and a save from before the
// progression catches up the same way, one morning a tier.
func (s *Sim) tier(w *game.World, t *game.Tick) int {
	for n := w.Tier() + 1; n <= len(s.pcfg.Tiers); n++ {
		tier := s.pcfg.Tiers[n-1]
		if _, ok := Eligible(w, content.CardConfig{Trigger: tier.Enter}); !ok {
			continue
		}
		w.Reach(n, t.Day)
		t.Emit(events.TierReached{Day: t.Day, Tier: n, Name: tier.Name})
		return n
	}
	return 0
}

// tierLines is the report's TIER section for tier n of total: the name
// and the blurb, what the stage opens (a line each, so none is cut) and
// what the next one takes.
func tierLines(n, total int, tier content.TierConfig) []string {
	lines := []string{fmt.Sprintf("%s, tier %d of %d. %s", strings.ToUpper(tier.Name), n, total, tier.Blurb)}
	for i, o := range tier.Opens {
		lead := "       "
		if i == 0 {
			lead = "Opens: "
		}
		lines = append(lines, lead+o)
	}
	if tier.Next != "" {
		lines = append(lines, "Next: "+tier.Next)
	}
	return lines
}
