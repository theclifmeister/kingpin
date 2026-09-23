package game

import (
	"errors"
)

// The books (#70): the player's moves against the rival's machine that
// are not a strike, a deal or a price war. Each queues intent for the
// rivals sim to resolve at the end of the day: a scout reads its books,
// a tip sends the police to one of its corners, a buy-off pays its
// muscle to go home. A boost, the enforcers going in for a corner's
// takings, is Boost in territory.go, since it shares the night's one
// strike order.

// ErrNoRival, the table's, is the refusal while nobody is contesting
// the city.
var (
	ErrScouting  = errors.New("somebody is already reading their books tonight")
	ErrTipped    = errors.New("you have already tipped the police tonight")
	ErrPoaching  = errors.New("you are already paying their people tonight")
	ErrNotRivals = errors.New("that corner is not a rival's")
	ErrBadUnits  = errors.New("units must be positive")
)

// Scout pays cost for a look at the rival's books tonight (#70): the
// rivals sim rolls whether it reads them, and a success stamps Known
// with today's numbers. One a day; paid up front, dirty then clean; the
// money is spent whatever the roll.
func (w *World) Scout(cost int) error { return w.ScoutFaction("", cost) }

// ScoutFaction is Scout with the faction named (#43).
func (w *World) ScoutFaction(faction string, cost int) error {
	if w.Over != nil {
		return ErrGameOver
	}
	r := w.Faction(faction)
	if r == nil {
		return ErrNoFaction
	}
	if !r.Alive() {
		return ErrNoRival
	}
	if w.Today.Scouting != nil {
		return ErrScouting
	}
	if !w.spend(cost) {
		return &ShortError{Need: cost, Have: w.Cash()}
	}
	w.Today.Scouting = &ScoutOrder{Cost: cost, Faction: faction}
	return nil
}

// CancelScout calls tonight's look off; the money stays spent, as an
// investigation's does.
func (w *World) CancelScout() { w.Today.Scouting = nil }

// Tip queues a tip to the police on a rival corner tonight (#70): free
// in cash. The rivals sim raises the rival's heat, costs its trust, holds
// its grudge, and past the notice line has the police take the corner.
// Under a truce or a tribute it is a betrayal. One a night.
func (w *World) Tip(corner string) error {
	if w.Over != nil {
		return ErrGameOver
	}
	c := w.Corner(corner)
	if c == nil {
		return ErrNoCorner
	}
	if c.Owner != OwnerRival {
		return ErrNotRivals
	}
	if r := w.Faction(c.Faction); r == nil || r.Arrived == 0 {
		return ErrNoRival
	}
	if w.Today.Tipoff != nil {
		return ErrTipped
	}
	w.Today.Tipoff = &TipOrder{Corner: corner}
	return nil
}

// CancelTip calls tonight's tip off.
func (w *World) CancelTip() { w.Today.Tipoff = nil }

// BuyOff pays cost to send units heads of the rival's muscle home
// tonight (#70), dirty cash only (the money changes hands on the
// street). The rivals sim rolls whether the order lands: landing, the
// heads leave the rival and never join you, what was paid for heads it
// did not have comes back; failing, the money is gone and the rival
// holds a grudge. One order a night.
func (w *World) BuyOff(units, cost int) error { return w.BuyOffFrom("", units, cost) }

// BuyOffFrom is BuyOff with the faction named (#43).
func (w *World) BuyOffFrom(faction string, units, cost int) error {
	if w.Over != nil {
		return ErrGameOver
	}
	r := w.Faction(faction)
	if r == nil {
		return ErrNoFaction
	}
	if !r.Alive() {
		return ErrNoRival
	}
	if units <= 0 {
		return ErrBadUnits
	}
	if w.Today.Poach != nil {
		return ErrPoaching
	}
	if err := w.payDirty(cost); err != nil {
		return err
	}
	w.Today.Poach = &PoachOrder{Units: units, Cost: cost, Faction: faction}
	return nil
}

// CancelBuyOff calls tonight's order off and returns the money.
func (w *World) CancelBuyOff() {
	if w.Today.Poach != nil {
		w.Player.DirtyCash += w.Today.Poach.Cost
	}
	w.Today.Poach = nil
}

// The war order (#229): enforcers as a standing order against one
// faction. DeclareWar names the faction; the rivals sim sends the
// hand's strike every night the hand leaves empty (Today.Strike nil),
// at rivals.toml [war] dial on the faction's corner nearest your front
// line, with the same roll, heat, toll and betrayal a hand's strike
// carries, and ends the war the night the faction folds, bows or has
// no corner left where you hold ground (WarEnded). It never ends the
// run: taken_out is the faction's to win, as now.

var (
	// ErrAtWar means a war is on already: one at a time.
	ErrAtWar = errors.New("one war at a time: call the other off first")
	// ErrNoWar means there is no war to call off.
	ErrNoWar = errors.New("there is no war on")
	// ErrNothingToTake means the faction holds no corner in a city you
	// hold ground in: nowhere for the enforcers to go.
	ErrNothingToTake = errors.New("they hold no corner in a city you hold ground in")
)

// DeclareWar puts the enforcers on a standing order against a faction:
// alive, with a corner in a city you hold ground in, and enforcers on
// the payroll to send. One war at a time.
func (w *World) DeclareWar(faction string) error {
	if w.Over != nil {
		return ErrGameOver
	}
	r := w.Faction(faction)
	if r == nil || !r.Alive() {
		return ErrNoFaction
	}
	if w.Crew.OnPayroll(RoleEnforcer) == 0 {
		return ErrNoEnforcers
	}
	if w.War != "" {
		return ErrAtWar
	}
	if !w.WarHasGround(r) {
		return ErrNothingToTake
	}
	w.War = r.Faction()
	return nil
}

// CallOffWar ends the war order.
func (w *World) CallOffWar() error {
	if w.Over != nil {
		return ErrGameOver
	}
	if w.War == "" {
		return ErrNoWar
	}
	w.War = ""
	return nil
}

// AtWarWith reports whether the war order stands against r.
func (w *World) AtWarWith(r *RivalState) bool {
	return r != nil && w.War != "" && w.War == r.Faction()
}

// WarHasGround reports whether the faction holds a corner in a city you
// hold ground in: somewhere the war can go.
func (w *World) WarHasGround(r *RivalState) bool {
	if r == nil {
		return false
	}
	for _, c := range w.Corners() {
		if c.Owner == OwnerRival && c.FactionID() == r.Faction() && w.HeldIn(c.City) > 0 {
			return true
		}
	}
	return false
}

// Hitting the scouts (#341): a faction moving on a city where you earn
// and nobody lives is telegraphed, and its scouts can be hit before it
// arrives.
var (
	ErrNotScouting = errors.New("they are not moving on a city")
	ErrScoutsHit   = errors.New("their scouts have been hit already")
)

// HitScouts sends the enforcers after the scouts of a faction moving on
// a city tonight (#341): the rivals sim sets it back setback_days and
// it holds a grudge. Once a faction; one a night; an enforcer on the
// payroll to send.
func (w *World) HitScouts(faction string) error {
	if w.Over != nil {
		return ErrGameOver
	}
	r := w.Faction(faction)
	if r == nil {
		return ErrNoFaction
	}
	if !r.Scouting() || r.Gone() {
		return ErrNotScouting
	}
	if r.ScoutsHit > 0 || w.Today.HitScouts != "" {
		return ErrScoutsHit
	}
	if w.Crew.OnPayroll(RoleEnforcer) == 0 {
		return ErrNoEnforcers
	}
	w.Today.HitScouts = r.Faction()
	return nil
}
