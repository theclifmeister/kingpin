package game

import (
	"errors"
	"fmt"

	"github.com/theclifmeister/kingpin/internal/format"
)

// The exits (#49, #195): how a run ends on the player's terms, and the
// one shape every ending is written in. Retire is the offshore
// account's exit (#195), Vanish the new identity's. Every other cause is
// detected by the sim that owns it, in its step, as a threshold on state
// it already reads, and written to Over through End the way these are
// (docs/endings.md).

var (
	// ErrNotQuiet means the retirement needs more quiet days first.
	ErrNotQuiet = errors.New("it is not quiet enough to walk away yet")
	// ErrNoIdentity means vanishing takes a new identity from the tree.
	ErrNoIdentity = errors.New("vanishing takes a new identity")
	// ErrNoReign means the crown takes the city: the reign is not on.
	ErrNoReign = errors.New("the city is not yours yet")
)

// End is the ending written for a cause on day: the day, the cause, the
// peak cash as it stands and who, and the score stamped on Stats as it
// stands the morning the run ends (Score). Every writer of Over goes
// through it, so the summary reads one shape whatever the cause.
func (w *World) End(cause string, day int, who string) *Ending {
	w.Stats.Score = w.Score()
	return &Ending{Day: day, Cause: cause, PeakCash: w.Stats.PeakCash, Who: who}
}

// Score is the run's score (#49, #195's ruling): the offshore account
// over one plus the bodies, and nothing else. The pile left behind is
// printed, never scored, so lying low k more days before an exit can
// never raise it (TestRetireeRetires); days are shown, never scored.
func (w *World) Score() int {
	return w.Offshore / (1 + w.Stats.Bodies)
}

// Retire ends the run as "retired": the player walks away on the
// account. It needs at least cash offshore (laundering.toml [offshore]
// retire_cash) and days quiet days in a row (retire_days, counted on
// QuietDays by the laundering sim), which the caller passes from the
// tuning as BuyFront takes its offer priced. The pile, the stock, the
// crew and the fronts are left behind; the account is what is scored.
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
	w.Over = w.End("retired", w.Day, "")
	return nil
}

// CanRetire reports whether Retire would take: the account at cash and
// the quiet days at days.
func (w *World) CanRetire(cash, days int) bool {
	return w.Over == nil && w.Offshore >= cash && w.QuietDays >= days
}

// Vanish ends the run as "vanished" (#49): the player leaves on the new
// identity the tree gave them (upgrades.toml identity, fx.Identities),
// on any morning, with whatever the account holds. It takes the
// identity and nothing else: the pile, the stock, the crew and the
// fronts are left behind as Retire leaves them.
func (w *World) Vanish(fx Effects) error {
	if w.Over != nil {
		return ErrGameOver
	}
	if fx.Identities <= 0 {
		return ErrNoIdentity
	}
	w.Over = w.End("vanished", w.Day, "")
	return nil
}

// CanVanish reports whether Vanish would take: an identity owned and
// the run not over.
func (w *World) CanVanish(fx Effects) bool { return w.Over == nil && fx.Identities > 0 }

// Crown ends the run as "kingpin" (#227): the player takes the crown
// while the reign holds (Reign, stamped by the rivals sim the morning
// the city became theirs for good and zeroed the morning it stops
// being), on any morning of it. It is the kingpin ending as it always
// read, on the player's say-so instead of the detector's; the score is
// the account as it stands, so playing the reign on is allowed and
// never rewarded.
func (w *World) Crown() error {
	if w.Over != nil {
		return ErrGameOver
	}
	if w.Reign <= 0 {
		return ErrNoReign
	}
	w.Over = w.End("kingpin", w.Day, "")
	return nil
}

// CanCrown reports whether Crown would take: the reign on and the run
// not over.
func (w *World) CanCrown() bool { return w.Over == nil && w.Reign > 0 }

// ReignDay is which day of the reign this is, counting the morning it
// began as day 1; 0 with no reign.
func (w *World) ReignDay() int { return w.ReignDayOn(w.Day) }

// ReignDayOn is ReignDay on a given day: a sim in the tick that stamps
// the reign reads it against the tick's day, since the clock has not
// moved the world's yet.
func (w *World) ReignDayOn(day int) int {
	if w.Reign <= 0 {
		return 0
	}
	return day - w.Reign + 1
}

// HomageDeals is the reign's tally (#227): how many factions pay you
// homage and what they pay a night between them.
func (w *World) HomageDeals() (crews, perDay int) {
	for _, r := range w.Rivals {
		if r == nil {
			continue
		}
		if d := w.DealWith(r.Faction(), DealHomage); d != nil {
			crews++
			perDay += d.Terms.PerDay
		}
	}
	return crews, perDay
}
