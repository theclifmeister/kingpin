package game

import (
	"fmt"

	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/format"
)

// FrontOffer is a front as the laundering config prices it, handed to
// BuyFront by the caller so the world never needs the config.
type FrontOffer struct {
	ID         string
	Name       string
	Cost       int // dirty cash
	Throughput int // dirty cash washed per day at the normal dial
	Upkeep     int // clean cash per day
	AuditRisk  float64
	UnlockCash int // peak cash that puts it on offer
}

// Locked reports whether the offer is still gated behind peak cash.
func (o FrontOffer) Locked(w *World) bool { return w.Stats.PeakCash < o.UnlockCash }

// BuyFront buys a front with dirty cash. It applies immediately: the place
// opens tomorrow and the laundering sim reports the purchase at end of day.
// The offer must be on offer (peak cash past its unlock), not already
// owned, and affordable.
func (w *World) BuyFront(o FrontOffer) (Front, error) {
	if w.Over != nil {
		return Front{}, ErrGameOver
	}
	if o.ID == "" {
		return Front{}, ErrNoFront
	}
	if w.Front(o.ID) != nil {
		return Front{}, ErrFrontOwned
	}
	if o.Locked(w) {
		return Front{}, fmt.Errorf("nobody will sell you %s until you have moved $%d", o.Name, o.UnlockCash)
	}
	if o.Cost > w.Player.DirtyCash {
		return Front{}, fmt.Errorf("%s costs $%d, only have $%d dirty", o.Name, o.Cost, w.Player.DirtyCash)
	}
	w.Player.DirtyCash -= o.Cost
	f := Front{ID: o.ID, Name: o.Name, Cost: o.Cost, Bought: w.Day}
	w.Fronts = append(w.Fronts, f)
	return f, nil
}

// LevelOffer is a front's next levels as the laundering config prices
// them (#192), handed to Invest: which front, how many levels, what they
// cost between them in clean cash, and the level the front tops out at.
type LevelOffer struct {
	Front  string
	Levels int
	Cost   int // clean cash
	Max    int
}

// Invest buys a front's next levels with clean cash, whole levels at a
// time and at once: the front earns from tomorrow. Only clean cash pays
// (Fund's rule: dirty cash is refused however much of it there is), a
// front at its top takes no more, and the laundering sim reports the
// investment in the morning.
func (w *World) Invest(o LevelOffer) error {
	if w.Over != nil {
		return ErrGameOver
	}
	f := w.Front(o.Front)
	if f == nil {
		return ErrNoFront
	}
	if o.Levels <= 0 {
		return ErrBadQuantity
	}
	if o.Max <= 0 || f.Level >= o.Max {
		return ErrFrontMaxed
	}
	if f.Level+o.Levels > o.Max {
		return fmt.Errorf("%s takes %s more at most", f.Name, format.Plural(o.Max-f.Level, "level"))
	}
	if o.Cost > w.Player.CleanCash {
		if w.Player.CleanCash <= 0 {
			return ErrNoCleanCash
		}
		return fmt.Errorf("need $%d clean, only have $%d clean", o.Cost, w.Player.CleanCash)
	}
	w.Player.CleanCash -= o.Cost
	f.Level += o.Levels
	f.Invested += o.Cost
	w.Stats.Invested += o.Cost
	w.Today.Invested = append(w.Today.Invested, Investment{Front: o.Front, Levels: o.Levels, Cost: o.Cost})
	return nil
}

// InvestedToday is what the player has put into a front's levels today.
func (w *World) InvestedToday(id string) int {
	n := 0
	for _, inv := range w.Today.Invested {
		if inv.Front == id {
			n += inv.Cost
		}
	}
	return n
}

// SetLaunderDial sets the launder dial for every front. It persists until
// changed. A run that is over, or a dial off the three positions, is
// refused, as SetRoute refuses them (#281).
func (w *World) SetLaunderDial(d events.Launder) error {
	if w.Over != nil {
		return ErrGameOver
	}
	if d < events.LaunderCareful || d > events.LaunderGreedy {
		return ErrBadDial
	}
	w.Laundering.Dial = d
	return nil
}
