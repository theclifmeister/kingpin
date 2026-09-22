package game

import (
	"errors"
	"fmt"
)

var (
	ErrGameOver       = errors.New("the run is over")
	ErrUnknownProduct = errors.New("unknown product")
	ErrBadQuantity    = errors.New("quantity must be positive")
	ErrBadRatio       = errors.New("the cut is more than the product takes") // #47: a ratio out of 0..cut_max, or one that adds nothing
	ErrNothingToCut   = errors.New("nothing here to cut")                    // #47: the stash here holds none of it
	ErrNoChemist      = errors.New("nobody on the payroll can cook")         // #47: a cook needs a chemist
	ErrNotCooked      = errors.New("that is not cooked, it is bought")       // #47: the product has no cook_cost
	ErrBatch          = errors.New("more than a batch")                      // #47: a cook order past the chemist's batch
	ErrCooking        = errors.New("a batch of that is already on today")    // #47: one cook order a product a city a day
	ErrNoCandidate    = errors.New("nobody by that name is looking for work")
	ErrJailed         = errors.New("they are in a cell")            // #46: a jailed member works nothing
	ErrWounded        = errors.New("they are laid up")              // #46: a wounded member works nothing
	ErrNotJailed      = errors.New("they are not in a cell")        // #46: nothing to bail
	ErrBailed         = errors.New("bail is already down for them") // #46: they walk tomorrow
	ErrNotDriver      = errors.New("only a driver rides a route")   // #46
	ErrNoMember       = errors.New("nobody by that name works for you")
	ErrCrewFull       = errors.New("the crew is as big as you can manage")
	ErrNoFront        = errors.New("no such front")
	ErrFrontOwned     = errors.New("you already own that front")
	ErrFrontMaxed     = errors.New("the place is as big as it gets")
	ErrNoCrew         = errors.New("nobody on the payroll to ask")
	ErrInvestigating  = errors.New("somebody is already asking around tonight")
	ErrNoCity         = errors.New("no such city")
	ErrNoRoute        = errors.New("no such route")
	ErrBadDial        = errors.New("no such dial position")
	ErrNotSupplied    = errors.New("the supplier here does not sell that; it comes in by the road")
	ErrNothingBought  = errors.New("nothing bought there today to return")
	ErrReturnGone     = errors.New("the units are no longer in the stash")
)

// Travel moves the player to another city at once. Product stays where it
// is: only the road moves it. Whatever corner you stood on is left with
// nobody on it and drifts unless a runner takes it.
func (w *World) Travel(city string) error {
	if w.Over != nil {
		return ErrGameOver
	}
	if w.Cities[city] == nil {
		return ErrNoCity
	}
	if city == w.Player.Location {
		return nil
	}
	w.Recall(You)
	w.Player.Location = city
	return nil
}

// SetLieLow toggles lying low for the day. Lying low cancels all orders,
// and the handoffs queued against the buyers' contracts (#71): it is
// everyone's day off.
func (w *World) SetLieLow(on bool) {
	w.Today.LieLow = on
	if on {
		w.Today.Orders = map[string]SellOrder{}
		w.Today.Deliveries = nil
	}
}

// dollars writes a whole-dollar figure with a $ for an error message.
func dollars(n int) string { return fmt.Sprintf("$%d", n) }

// spend takes cost from dirty cash first and clean cash for the rest, the
// way somebody paid off the books is paid. It reports whether there was
// enough between the two.
func (w *World) spend(cost int) bool {
	if cost > w.Cash() {
		return false
	}
	dirty := min(cost, w.Player.DirtyCash)
	w.Player.DirtyCash -= dirty
	w.Player.CleanCash -= cost - dirty
	return true
}
