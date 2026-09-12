package game

import (
	"errors"
	"fmt"

	"github.com/theclifmeister/kingpin/internal/format"
)

// The exits (#49). Retire is the first of them (#195): the run ends on
// the player's terms, with the offshore account as the score.

var (
	// ErrNotQuiet means the retirement needs more quiet days first.
	ErrNotQuiet = errors.New("it is not quiet enough to walk away yet")
)

// Retire ends the run as "retired": the player walks away on the
// account. It needs at least cash offshore (laundering.toml [offshore]
// retire_cash) and days quiet days in a row (retire_days, counted on
// QuietDays by the laundering sim), which the caller passes from the
// tuning as BuyFront takes its offer priced. The pile, the stock, the
// crew and the fronts are left behind; the account is what is scored.
// #156's over scene plays its default for the cause until #49 gives
// retirement its own; #49's summary reads Offshore.
func (w *World) Retire(cash, days int) error {
	if w.Over != nil {
		return ErrGameOver
	}
	if w.Offshore < cash {
		return fmt.Errorf("the account holds %s, retiring takes %s: %s short", format.Money(w.Offshore), format.Money(cash), format.Money(cash-w.Offshore))
	}
	if w.QuietDays < days {
		return fmt.Errorf("%w: %s quiet, %s needed", ErrNotQuiet, format.Plural(w.QuietDays, "day"), format.Plural(days, "day"))
	}
	w.Over = &Ending{Day: w.Day, Cause: "retired", PeakCash: w.Stats.PeakCash}
	return nil
}

// CanRetire reports whether Retire would take: the account at cash and
// the quiet days at days.
func (w *World) CanRetire(cash, days int) bool {
	return w.Over == nil && w.Offshore >= cash && w.QuietDays >= days
}
