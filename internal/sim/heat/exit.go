package heat

import (
	"math"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
)

// indict is the case. Every sting and raid goes in a file. A thick
// enough file is a case, and the case runs the exit plans (#49): a fall
// guy takes it, else a new identity makes it the vanished ending, else
// it is the end, in the city whose police answered tonight.
func (s *Sim) indict(d *day) {
	w, t, h := d.w, d.t, d.h
	if arrest := s.EvidenceArrest(w); w.Over == nil && arrest > 0 && h.Evidence >= arrest {
		if !s.takeFall(w, t, d.fx) {
			w.Over = w.End(s.exit(content.CauseIndicted, d.fx), t.Day, "")
			t.Emit(events.Enforcement{Day: t.Day, City: d.hot.ID, Level: content.Arrest, StockLost: map[string]int{}})
			t.Emit(events.GameOver{Day: t.Day, Cause: w.Over.Cause})
		}
	}
}

// exit is the cause the case ends the run with once the fall guys are
// spent (#49): the cause the police wrote (indicted, arrested), or
// vanished with a new identity from the tree (upgrades.toml identity,
// fx.Identities), which turns the end into an exit once, since the run
// is over either way. A read on the tree, no dice.
func (s *Sim) exit(cause string, fx game.Effects) string {
	if fx.Identities > 0 {
		return content.CauseVanished
	}
	return cause
}

// takeFall is the fall guy's one job: if the player owns one who has not
// taken his fall (fall_guys is a count, one fall each: World.FallGuyLeft),
// the case that would have ended the run closes on him instead. The file
// is wiped, heat drops to heat.toml fall_heat everywhere and fall_cash
// of the dirty cash and of the clean goes on making it stick. It reports
// whether he took it.
func (s *Sim) takeFall(w *game.World, t *game.Tick, fx game.Effects) bool {
	if !w.FallGuyLeft(fx) {
		return false
	}
	w.FallsTaken++
	w.Heat.Evidence = 0
	w.Heat.EvidenceDay = t.Day
	for _, c := range w.Cities {
		c.Heat = math.Min(c.Heat, s.cfg.Heat.FallHeat)
	}
	dirty, clean := s.fallShare(w.Player.DirtyCash), s.fallShare(w.Player.CleanCash)
	w.Player.DirtyCash -= dirty
	w.Player.CleanCash -= clean
	t.Emit(events.FallGuyBurned{Day: t.Day, CashLost: dirty + clean, Clean: clean})
	return true
}

// fallShare is fall_cash of a pile, truncated toward zero as the integer
// halving it replaced was (#275): a float64 holds every amount a run
// reaches exactly and multiplying by 0.5 is exact, so at 0.5 it is
// cash/2 to the dollar.
func (s *Sim) fallShare(cash int) int {
	return int(float64(cash) * s.cfg.Heat.FallCash)
}
