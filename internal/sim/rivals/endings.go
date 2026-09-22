package rivals

import (
	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
)

// The endings this sim owns (#49, rivals.toml [endings], docs/endings.md):
// kingpin, taken out and the table's betrayal. Each is a threshold on
// state the step already reads, checked after the table has stepped,
// with no dice of its own, and written to w.Over through World.End as
// the heat sim writes the indictment; a zero in the file boxes it, so a
// run on the file before the table is the run it was.

// endings runs the two read at the end of the step: kingpin and taken
// out. The betrayal is read where the deal breaks (betrayed).
func (s *Sim) endings(w *game.World, t *game.Tick) {
	if w.Over != nil {
		return
	}
	tun := s.cfg.Endings
	// Kingpin: Dominant() for dominant_days, holding the city: more than
	// kingpin_share of home's corners. The day it began is on the
	// table's own stamps (the last faction absorbed, fragmented or the
	// homage deal's Since), so no counter. The share is what makes it a
	// reign and not the weather: #44's rival_leader_killed empties a
	// table over a quiet trader on a corner in a year on a seed in six,
	// and a city nobody holds has no kingpin. Since #227 the detector
	// stamps the reign (World.Reign, the morning it begins, ReignBegan)
	// instead of ending the run: the crown is the player's to take from
	// the walk-away dialog (World.Crown) while it holds, and the morning
	// it stops holding (a faction set up again, the share fell) the
	// stamp zeroes (ReignBroken) and it can begin again. A run that
	// never reaches it is the run before.
	if tun.DominantDays > 0 {
		held, dominant := s.HoldsTheCity(w), w.Dominant()
		switch {
		case held && dominant && t.Day-s.DominantSince(w) >= tun.DominantDays:
			if w.Reign == 0 {
				w.Reign = t.Day
				crews, homage := w.HomageDeals()
				t.Emit(events.ReignBegan{Day: t.Day, City: w.Home().ID, Crews: crews, Homage: homage})
			}
		case w.Reign != 0:
			why := "a crew set up again"
			if !held {
				why = "the city slipped under the share"
			}
			w.Reign = 0
			t.Emit(events.ReignBroken{Day: t.Day, City: w.Home().ID, Why: why})
		}
	}
	// Taken out: a faction's push took the last corner you held
	// anywhere tonight, its war with you is open, and the enforcers on
	// the payroll, at work or laid up, are under taken_out_muscle:
	// nothing to hold and nobody to hold it with.
	if tun.TakenOutMuscle > 0 && w.Held() == 0 && w.Crew.OnPayroll(game.RoleEnforcer) < tun.TakenOutMuscle {
		for _, r := range w.Rivals {
			if r != nil && r.Alive() && r.LastFlip == t.Day && r.War >= s.cfg.Rivals.WarThreshold {
				w.Over = w.End(content.CauseTakenOut, t.Day, r.Leader)
				t.Emit(events.GameOver{Day: t.Day, Cause: content.CauseTakenOut})
				return
			}
		}
	}
}

// HoldsTheCity reports whether the player holds more than [endings]
// kingpin_share of home's corners: the kingpin's ground.
func (s *Sim) HoldsTheCity(w *game.World) bool {
	home := w.Home()
	return float64(w.HeldIn(home.ID)) > s.cfg.Endings.KingpinShare*float64(len(home.Corners))
}

// DominantSince is the day the city became yours: the latest of the
// days each faction at the table fell (Absorbed, Fragmented) or bowed
// (its homage deal's Since); 0 with no table. With Dominant() true it
// is the first day of the reign, and the kingpin ending reads the days
// since it.
func (s *Sim) DominantSince(w *game.World) int {
	since := 0
	for _, r := range w.Rivals {
		if r == nil {
			continue
		}
		since = max(since, r.Absorbed, r.Fragmented)
		if d := w.DealWith(r.Faction(), game.DealHomage); d != nil {
			since = max(since, d.Since)
		}
	}
	return since
}

// betrayed is the table's betrayal (#49): a faction has just broken a
// deal with you of its own accord (whim), and another faction's war
// with you is over betray_war, so the deal was what kept the second
// front quiet. A read on the table, no dice.
func (s *Sim) betrayed(w *game.World, t *game.Tick, r *game.RivalState) {
	line := s.cfg.Endings.BetrayWar
	if line <= 0 || w.Over != nil {
		return
	}
	for _, o := range w.Rivals {
		if o != nil && o != r && o.Alive() && o.War >= line {
			w.Over = w.End(content.CauseBetrayed, t.Day, r.Leader)
			t.Emit(events.GameOver{Day: t.Day, Cause: content.CauseBetrayed})
			return
		}
	}
}
