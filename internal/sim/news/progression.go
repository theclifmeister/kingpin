package news

import (
	"fmt"
	"strings"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/format"
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
		if _, ok := game.Eligible(w, content.CardConfig{Trigger: tier.Enter}); !ok {
			continue
		}
		w.Reach(n, t.Day)
		t.Emit(events.TierReached{Day: t.Day, Tier: n, Name: tier.Name})
		return n
	}
	return 0
}

// reignLine is the report's REIGN line (#227) while the city is yours:
// the day of the reign, the crews paying homage and what they paid
// tonight between them (the tick's TributePaid to you).
func reignLine(w *game.World, t *game.Tick) string {
	crews, _ := w.HomageDeals()
	paid := 0
	for _, e := range t.Events() {
		if ev, ok := e.(events.TributePaid); ok && ev.ToYou {
			paid += ev.Amount
		}
	}
	line := fmt.Sprintf("REIGN: day %d of the reign", w.ReignDayOn(t.Day))
	if crews > 0 {
		line += fmt.Sprintf(" · %s paying homage · %s a night", format.Plural(crews, "crew"), format.Money(paid))
	} else {
		line += " · every crew gone"
	}
	return line
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

// title is what the paper calls you (#233): the boss of the home city
// while it is yours in the kingpin's sense (World.Dominant with more
// than kingpin_share of home's corners held), a crew with anybody on
// the payroll, a dealer otherwise.
func (s *Sim) title(w *game.World) string {
	if s.boss(w) {
		return "the boss of " + w.Home().Name
	}
	if len(w.Crew.Members) > 0 {
		return "a crew"
	}
	return "a dealer"
}

// boss reports whether the paper calls you the boss (#233): the city
// yours in the kingpin's sense, read as the rivals sim reads it.
func (s *Sim) boss(w *game.World) bool {
	home := w.Home()
	return s.rcfg.KingpinShare > 0 && w.Dominant() && float64(w.HeldIn(home.ID)) > s.rcfg.KingpinShare*float64(len(home.Corners))
}

// swaggerLines are the report's opening under TIER (#233): what your
// name did last night, one line a thing, nothing on a night with
// nothing to say. The homage is on the reign's line while the reign
// holds, so it is said here only before it.
func swaggerLines(w *game.World, t *game.Tick) []string {
	var out []string
	homage, crews := 0, map[string]bool{}
	pushes, lost := map[string]int{}, map[string]int{}
	var order []string
	deterred := map[string]bool{}
	for _, e := range t.Events() {
		switch ev := e.(type) {
		case events.TributePaid:
			if ev.ToYou {
				homage += ev.Amount
				crews[ev.Faction] = true
			}
		case events.RivalPushed:
			if _, ok := pushes[ev.Rival]; !ok {
				order = append(order, ev.Rival)
			}
			pushes[ev.Rival]++
		case events.CrewShot:
			if ev.Theirs && ev.Dead && !ev.Strike {
				lost[ev.Rival]++
			}
		case events.ClaimDeterred:
			deterred[ev.City] = true
		}
	}
	if homage > 0 && w.Reign == 0 {
		who := "1 crew"
		if len(crews) > 1 {
			who = fmt.Sprintf("%d crews", len(crews)) // format.Plural reads crew as its own plural
		}
		out = append(out, fmt.Sprintf("%s paid homage last night: %s.", who, format.Money(homage)))
	}
	for _, rival := range order {
		line := fmt.Sprintf("%s's crew pushed on your front line %s and were held off", rival, times(pushes[rival]))
		if n := lost[rival]; n > 0 {
			line += fmt.Sprintf(", losing %s", format.Plural(n, "head"))
		}
		out = append(out, line+".")
	}
	for _, cid := range w.CityOrder {
		if deterred[cid] {
			out = append(out, fmt.Sprintf("Nobody set up on a free corner in %s: your name kept them out.", w.CityName(cid)))
		}
	}
	return out
}

// times is a count in words for a line: once, twice, 3 times.
func times(n int) string {
	switch n {
	case 1:
		return "once"
	case 2:
		return "twice"
	}
	return fmt.Sprintf("%d times", n)
}
