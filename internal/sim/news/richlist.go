package news

import (
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
)

// richList stamps the rich list's next line (#392) if net worth is over
// it this morning, one a morning like the tiers, and emits RichListed.
// It returns the event, or false. No dice.
func (s *Sim) richList(w *game.World, t *game.Tick) (events.RichListed, bool) {
	rl := s.cfg.RichList
	if w.Progression.Rich >= len(rl.Lines) {
		return events.RichListed{}, false
	}
	line := rl.Lines[w.Progression.Rich]
	worth := w.NetWorth()
	if worth < line {
		return events.RichListed{}, false
	}
	w.Progression.Rich++
	ev := events.RichListed{Day: t.Day, Line: line, NetWorth: worth, Rank: rl.Rank(worth)}
	t.Emit(ev)
	return ev, true
}
