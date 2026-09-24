package engine

import "github.com/theclifmeister/kingpin/internal/game"

// Forecast is tonight's dirty pile as the police will count it (#397):
// the pile in hand, what the export loads due tonight pay on landing
// (the logistics sim steps before the count), less tonight's wages (the
// crew sim, before the count too), against the exposure line. The wash
// comes after the count (the laundering sim steps last), so it saves
// nothing tonight, and the forecast leaves it out; so it does tonight's
// street sales, which are the dice's. A read: it writes nothing.
type Forecast struct {
	Dirty    int     `json:"dirty"`    // the dirty cash in hand this morning
	Landings int     `json:"landings"` // what the loads due tonight pay if none is seized
	Loads    int     `json:"loads"`    // how many loads are due tonight
	Wages    int     `json:"wages"`    // tonight's wages at the pay dial
	Pile     int     `json:"pile"`     // Dirty + Landings - Wages, before tonight's sales
	Line     int     `json:"line"`     // the exposure line (heat.Sim.ExposureLine)
	Heat     float64 `json:"heat"`     // what that pile adds tonight where you are (heat.Sim.PileHeatOf)
}

// Forecast is tonight's pile (#397).
func (s *Session) Forecast() Forecast {
	return forecast(s.w, s.Rules())
}

func forecast(w *game.World, r Rules) Forecast {
	f := Forecast{Dirty: w.Player.DirtyCash, Wages: r.Crew.Wages(w, w.Crew.Pay), Line: r.Heat.ExposureLine(w)}
	tonight := w.Day + 1
	for _, l := range w.Exports.Loads {
		if l.Seized == 0 && l.Lands <= tonight {
			f.Landings += l.Revenue()
			f.Loads++
		}
	}
	f.Pile = f.Dirty + f.Landings - f.Wages
	f.Heat = r.Heat.PileHeatOf(w, f.Pile)
	return f
}
