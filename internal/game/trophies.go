package game

import (
	"errors"
	"fmt"

	"github.com/theclifmeister/kingpin/internal/format"
)

var (
	ErrNoTrophy    = errors.New("no such trophy")
	ErrTrophyOwned = errors.New("you already own that")
)

// Trophy is one bought (#392): clean cash spent to be seen spending
// it. It counts in net worth at cost while it is yours; the reputation
// sim reads the owned rows every day, and the task force can take one.
type Trophy struct {
	ID     string
	Name   string
	Cost   int    // clean cash paid
	Bought int    // day bought
	Lost   int    // day it was taken (TrophiesLost only)
	Why    string // how it was lost: seized (TrophiesLost only)
}

// TrophyOffer is a trophy as the file prices it, handed to BuyTrophy.
type TrophyOffer struct {
	ID         string
	Name       string
	Cost       int // clean cash
	UnlockCash int // peak clean cash that puts it on offer
}

// Locked reports whether the offer is still gated behind peak clean cash.
func (o TrophyOffer) Locked(w *World) bool { return w.Stats.PeakClean < o.UnlockCash }

// Trophy returns the owned trophy with id, or nil.
func (w *World) Trophy(id string) *Trophy {
	return find(w.Trophies, func(e *Trophy) bool { return e.ID == id })
}

// BuyTrophy buys a trophy with clean cash, at once. Only clean cash
// pays, the offer must be open (peak clean cash at its line), and one
// already owned is refused; one the task force took can be bought
// again at the price. The laundering sim reports it in the morning
// (TrophyBought).
func (w *World) BuyTrophy(o TrophyOffer) (Trophy, error) {
	if w.Over != nil {
		return Trophy{}, ErrGameOver
	}
	if o.ID == "" {
		return Trophy{}, ErrNoTrophy
	}
	if w.Trophy(o.ID) != nil {
		return Trophy{}, ErrTrophyOwned
	}
	if o.Locked(w) {
		return Trophy{}, fmt.Errorf("nobody sells %s to somebody who has not held %s clean", o.Name, format.Cash(o.UnlockCash))
	}
	if o.Cost > w.Player.CleanCash && w.Player.CleanCash <= 0 {
		return Trophy{}, ErrNoCleanCash
	}
	if err := w.payClean(o.Cost); err != nil {
		return Trophy{}, err
	}
	t := Trophy{ID: o.ID, Name: o.Name, Cost: o.Cost, Bought: w.Day}
	w.Trophies = append(w.Trophies, t)
	w.Stats.Trophies++
	w.Stats.TrophyCash += o.Cost
	return t, nil
}

// LoseTrophy takes an owned trophy off the books on day for a reason:
// it moves to the lost record. The laundering sim, which owns the
// trophies (they are clean money), is its one caller, on the heat
// sim's TrophySeized. It returns the trophy lost, or nil.
func (w *World) LoseTrophy(id string, day int, why string) *Trophy {
	for i := range w.Trophies {
		if w.Trophies[i].ID != id {
			continue
		}
		t := w.Trophies[i]
		t.Lost, t.Why = day, why
		w.Trophies = append(w.Trophies[:i], w.Trophies[i+1:]...)
		if len(w.Trophies) == 0 {
			w.Trophies = nil
		}
		w.TrophiesLost = append(w.TrophiesLost, t)
		w.Stats.TrophiesLost++
		return &w.TrophiesLost[len(w.TrophiesLost)-1]
	}
	return nil
}
