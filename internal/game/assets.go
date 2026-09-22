package game

import (
	"errors"
	"fmt"

	"github.com/theclifmeister/kingpin/internal/format"
)

// The assets (#48): the tier-5 purchases on the supply side, the way
// the fronts are on the money side. World.Assets is what is owned, in
// the order bought; World.AssetsLost is what the task force took or the
// tunnel the police found, for the record and the summary. An asset is
// bought with clean cash only (BuyAsset), gated on peak clean cash
// (Stats.PeakClean, the high-water mark the clock stamps beside
// PeakCash), costs clean upkeep the laundering sim collects, stands
// idle when the upkeep goes unpaid (FrozenUntil, a front's rule), and
// is read by the sim that owns the number it moves (AssetLive). Nothing
// here reads the config: the caller prices the offer, as BuyFront's
// does. The zero value is the run before: no bump.

var (
	ErrNoAsset    = errors.New("no such asset")
	ErrAssetOwned = errors.New("you already own that")
	ErrAssetGone  = errors.New("that is gone for good")
)

// Asset is one the player owns.
type Asset struct {
	ID          string
	Name        string
	Effect      string // the row's effect, for a reader that has no config (the ledger's kind column)
	City        string // the city it is in
	Cost        int    // clean cash paid
	Upkeep      int    // clean cash a day, as priced when bought
	Bought      int    // day bought
	FrozenUntil int    // idle until this day, the upkeep unpaid; 0 or past is standing
	Lost        int    // day it was lost (AssetsLost only)
	Why         string // how it was lost: seized, found (AssetsLost only)
}

// Frozen reports whether the asset stands idle on day.
func (a Asset) Frozen(day int) bool { return a.FrozenUntil > day }

// AssetOffer is an asset as the assets config prices it, handed to
// BuyAsset by the caller so the world never needs the config.
type AssetOffer struct {
	ID         string
	Name       string
	Effect     string
	City       string
	Cost       int // clean cash
	Upkeep     int // clean cash a day
	UnlockCash int // peak clean cash that puts it on offer
	HeatFloor  float64
}

// Locked reports whether the offer is still gated behind peak clean cash.
func (o AssetOffer) Locked(w *World) bool { return w.Stats.PeakClean < o.UnlockCash }

// Asset returns the owned asset with id, or nil.
func (w *World) Asset(id string) *Asset {
	return find(w.Assets, func(e *Asset) bool { return e.ID == id })
}

// HasAsset reports whether the asset is owned, standing or idle.
func (w *World) HasAsset(id string) bool { return w.Asset(id) != nil }

// AssetLive reports whether the asset is owned and standing tonight
// (not idle for unpaid upkeep): the test every sim's effect reads.
func (w *World) AssetLive(id string) bool {
	a := w.Asset(id)
	return a != nil && !a.Frozen(w.Day+1)
}

// AssetLost returns the record of a lost asset with id, the latest, or
// nil.
func (w *World) AssetLost(id string) *Asset {
	for i := len(w.AssetsLost) - 1; i >= 0; i-- {
		if w.AssetsLost[i].ID == id {
			return &w.AssetsLost[i]
		}
	}
	return nil
}

// BuyAsset buys an asset with clean cash, at once: it stands from
// tomorrow. Only clean cash pays (Fund's rule), the offer must be open
// (peak clean cash at its line), an asset already owned is refused, and
// one the police found (the tunnel) is gone for good; one the task
// force seized can be bought again at the price. The laundering sim
// reports it in the morning (AssetBought).
func (w *World) BuyAsset(o AssetOffer) (Asset, error) {
	if w.Over != nil {
		return Asset{}, ErrGameOver
	}
	if o.ID == "" {
		return Asset{}, ErrNoAsset
	}
	if w.HasAsset(o.ID) {
		return Asset{}, ErrAssetOwned
	}
	if lost := w.AssetLost(o.ID); lost != nil && lost.Why == "found" {
		return Asset{}, fmt.Errorf("%w: %s was found", ErrAssetGone, o.Name)
	}
	if o.Locked(w) {
		return Asset{}, fmt.Errorf("nobody will sell you %s until you have held %s clean", o.Name, format.Cash(o.UnlockCash))
	}
	if o.Cost > w.Player.CleanCash && w.Player.CleanCash <= 0 {
		return Asset{}, ErrNoCleanCash
	}
	if err := w.payClean(o.Cost); err != nil {
		return Asset{}, err
	}
	a := Asset{ID: o.ID, Name: o.Name, Effect: o.Effect, City: o.City, Cost: o.Cost, Upkeep: o.Upkeep, Bought: w.Day}
	w.Assets = append(w.Assets, a)
	w.Stats.Assets++
	w.Stats.AssetCash += o.Cost
	w.Today.AssetsBought = append(w.Today.AssetsBought, o.ID)
	return a, nil
}

// LoseAsset takes an owned asset off the books on day for a reason
// (seized by the task force, found by the police): it moves to the
// lost record and its effect is gone with it. The laundering sim, which
// owns the assets, is its one caller, on the heat sim's AssetSeized and
// the logistics sim's TunnelFound. It returns the asset lost, or nil
// for one not owned.
func (w *World) LoseAsset(id string, day int, why string) *Asset {
	for i := range w.Assets {
		if w.Assets[i].ID != id {
			continue
		}
		a := w.Assets[i]
		a.Lost, a.Why = day, why
		w.Assets = append(w.Assets[:i], w.Assets[i+1:]...)
		if len(w.Assets) == 0 {
			w.Assets = nil
		}
		w.AssetsLost = append(w.AssetsLost, a)
		w.Stats.AssetsLost++
		return &w.AssetsLost[len(w.AssetsLost)-1]
	}
	return nil
}

// AssetFloor is the highest heat floor of the assets owned and
// standing, from the offers priced: federal attention, what the heat
// sim never lets a city cool under while the asset is yours.
func (w *World) AssetFloor(offers []AssetOffer) float64 {
	floor := 0.0
	for _, o := range offers {
		if w.AssetLive(o.ID) {
			floor = max(floor, o.HeatFloor)
		}
	}
	return floor
}
