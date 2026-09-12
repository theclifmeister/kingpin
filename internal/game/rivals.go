package game

import (
	"errors"
	"fmt"
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
	ErrNotRivals = errors.New("that corner is not the rival's")
	ErrBadUnits  = errors.New("units must be positive")
)

// Scout pays cost for a look at the rival's books tonight (#70): the
// rivals sim rolls whether it reads them, and a success stamps Known
// with today's numbers. One a day; paid up front, dirty then clean; the
// money is spent whatever the roll.
func (w *World) Scout(cost int) error {
	if w.Over != nil {
		return ErrGameOver
	}
	if w.Rival.Arrived == 0 {
		return ErrNoRival
	}
	if w.Today.Scouting != nil {
		return ErrScouting
	}
	if !w.spend(cost) {
		return fmt.Errorf("need $%d, only have $%d", cost, w.Cash())
	}
	w.Today.Scouting = &ScoutOrder{Cost: cost}
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
	if w.Rival.Arrived == 0 {
		return ErrNoRival
	}
	c := w.Corner(corner)
	if c == nil {
		return ErrNoCorner
	}
	if c.Owner != OwnerRival {
		return ErrNotRivals
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
func (w *World) BuyOff(units, cost int) error {
	if w.Over != nil {
		return ErrGameOver
	}
	if w.Rival.Arrived == 0 {
		return ErrNoRival
	}
	if units <= 0 {
		return ErrBadUnits
	}
	if w.Today.Poach != nil {
		return ErrPoaching
	}
	if cost > w.Player.DirtyCash {
		return fmt.Errorf("need $%d dirty, only have $%d", cost, w.Player.DirtyCash)
	}
	w.Player.DirtyCash -= cost
	w.Today.Poach = &PoachOrder{Units: units, Cost: cost}
	return nil
}

// CancelBuyOff calls tonight's order off and returns the money.
func (w *World) CancelBuyOff() {
	if w.Today.Poach != nil {
		w.Player.DirtyCash += w.Today.Poach.Cost
	}
	w.Today.Poach = nil
}
