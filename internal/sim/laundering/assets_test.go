package laundering_test

import (
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/sim/laundering"
)

// The assets' books (#48), the laundering sim's: an asset is bought with
// clean cash only once its line of peak clean cash is crossed, reported
// the morning after and billed its upkeep in clean cash every day
// (unpaid, it stands idle upkeep_freeze_days and its effect is off);
// the task force's AssetSeized and the police's TunnelFound take one
// off the books, and one found is gone for good while one seized can
// be bought again; and the door opening is announced once.
func TestAssetsAreCleanMoney(t *testing.T) {
	cfg := content.MustLoad()
	s := laundering.New(cfg)
	offers := s.AssetOffers()
	if len(offers) != len(cfg.Assets.Offers) || offers[0].Cost > offers[len(offers)-1].Cost {
		t.Fatalf("%d offers, cheapest first: %v", len(offers), offers)
	}
	o := offers[0]
	w := world(1_000_000_000)
	w.Player.CleanCash = o.Cost * 2
	if _, err := s.BuyAsset(w, "casino"); err != game.ErrNoAsset {
		t.Fatalf("unknown asset: %v", err)
	}
	if _, err := s.BuyAsset(w, o.ID); err == nil || w.HasAsset(o.ID) {
		t.Fatalf("bought under the line: %v", err)
	}
	w.Stats.PeakClean = o.UnlockCash
	w.Player.CleanCash = o.Cost - 1
	if _, err := s.BuyAsset(w, o.ID); err == nil || w.HasAsset(o.ID) {
		t.Fatalf("bought short: %v", err)
	}
	w.Player.CleanCash = 0
	if _, err := s.BuyAsset(w, o.ID); err != game.ErrNoCleanCash {
		t.Fatalf("bought with dirty cash: %v", err)
	}
	w.Player.CleanCash = o.Cost * 2
	a, err := s.BuyAsset(w, o.ID)
	if err != nil || !w.HasAsset(o.ID) || !w.AssetLive(o.ID) || w.Player.CleanCash != o.Cost || w.Player.DirtyCash != 1_000_000_000 || a.Upkeep != o.Upkeep || w.Stats.Assets != 1 || w.Stats.AssetCash != o.Cost || w.Today.AssetsBought[0] != o.ID {
		t.Fatalf("the buy: %v %+v clean %d assets %d", err, a, w.Player.CleanCash, w.Stats.Assets)
	}
	if _, err := s.BuyAsset(w, o.ID); err != game.ErrAssetOwned {
		t.Fatalf("bought twice: %v", err)
	}
	if w.NetWorth() != w.Cash()+o.Cost {
		t.Fatalf("net worth %d, want the cash and the asset at cost %d", w.NetWorth(), w.Cash()+o.Cost)
	}
	// The morning after: reported, and the upkeep billed.
	w.ClearToday(w.Day)
	evs := step(w, s)
	if kinds(evs)["AssetBought"] != 1 || w.Player.CleanCash != o.Cost-o.Upkeep || w.Stats.AssetUpkeep != o.Upkeep {
		t.Fatalf("the morning after: %v clean %d", kinds(evs), w.Player.CleanCash)
	}
	if kinds(step(w, s))["AssetBought"] != 0 {
		t.Fatal("reported twice")
	}
	// Unpaid: idle for the file's days, the effect off, then back.
	w.Player.CleanCash = 0
	evs = step(w, s)
	days := cfg.Assets.Assets.UpkeepFreezeDays
	if kinds(evs)["AssetFrozen"] != 1 || w.AssetLive(o.ID) || w.Asset(o.ID).FrozenUntil != w.Day+days {
		t.Fatalf("unpaid: %v live %v until %d (day %d)", kinds(evs), w.AssetLive(o.ID), w.Asset(o.ID).FrozenUntil, w.Day)
	}
	w.Player.CleanCash = o.Upkeep * 100
	for i := 1; i < days; i++ {
		if n := kinds(step(w, s))["AssetFrozen"]; n != 0 || w.Player.CleanCash != o.Upkeep*100 {
			t.Fatal("billed while idle")
		}
	}
	step(w, s) // the day it reopens, billed again
	if !w.AssetLive(o.ID) || w.Player.CleanCash != o.Upkeep*99 {
		t.Fatalf("after the freeze: live %v clean %d", w.AssetLive(o.ID), w.Player.CleanCash)
	}
	// Seized: off the books on the heat sim's event, and for sale again.
	tk := &game.Tick{Day: w.Day + 1, RNG: game.RNGFor(w.Seed, w.Day+1)}
	tk.Emit(events.AssetSeized{Day: tk.Day, Asset: o.ID, Name: o.Name, Cost: o.Cost})
	s.Step(w, tk)
	w.Day++
	if w.HasAsset(o.ID) || len(w.AssetsLost) != 1 || w.AssetsLost[0].Why != "seized" || w.Stats.AssetsLost != 1 || w.NetWorth() != w.Cash() {
		t.Fatalf("seized: %v lost %+v", w.Assets, w.AssetsLost)
	}
	w.Player.CleanCash = o.Cost * 2
	if _, err := s.BuyAsset(w, o.ID); err != nil || !w.HasAsset(o.ID) {
		t.Fatalf("bought again after the seizure: %v", err)
	}
	// Found (the tunnel): gone for good.
	tk = &game.Tick{Day: w.Day + 1, RNG: game.RNGFor(w.Seed, w.Day+1)}
	tk.Emit(events.TunnelFound{Day: tk.Day, Asset: o.ID, Name: o.Name})
	s.Step(w, tk)
	w.Day++
	if w.HasAsset(o.ID) || len(w.AssetsLost) != 2 || w.AssetsLost[1].Why != "found" {
		t.Fatalf("found: %v lost %+v", w.Assets, w.AssetsLost)
	}
	w.Player.CleanCash = o.Cost * 2
	if _, err := s.BuyAsset(w, o.ID); err == nil || w.HasAsset(o.ID) {
		t.Fatalf("bought again after it was found: %v", err)
	}
}

// The door: an asset's line crossed on peak clean cash is one Unlocked
// with gate "asset", once, the morning it is crossed, and never for an
// asset already owned.
func TestAssetDoorsOpenOnce(t *testing.T) {
	cfg := content.MustLoad()
	s := laundering.New(cfg)
	o := s.AssetOffers()[0]
	w := world(1_000_000)
	unlocked := func(evs []events.Event) int {
		n := 0
		for _, e := range evs {
			if ev, ok := e.(events.Unlocked); ok && ev.Gate == "asset" && ev.ID == o.ID {
				n++
			}
		}
		return n
	}
	if unlocked(step(w, s)) != 0 {
		t.Fatal("announced under the line")
	}
	w.Player.CleanCash = o.UnlockCash
	if unlocked(step(w, s)) != 1 {
		t.Fatal("not announced the morning the line is read")
	}
	if unlocked(step(w, s)) != 0 {
		t.Fatal("announced twice")
	}
}
