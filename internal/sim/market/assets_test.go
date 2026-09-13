package market_test

import (
	"math"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/game"
)

// The supplier asset (#48): the wholesale connect in its city, once
// the asset is owned and standing, sells at own_ratio of street
// whatever the band, without limit, and a buy nudges nothing; every
// other connect reads as it did, and a cartel war abroad marks every
// connect's price up for its window.
func TestOwnedBookAndTheCartelWar(t *testing.T) {
	cfg := content.MustLoad()
	row := cfg.Assets.ByEffect(content.AssetSupplier)
	if row == nil {
		t.Fatal("no supplier asset in the file")
	}
	w, mk, clock := marketOnly(t, cfg, 3)
	w.Player.DirtyCash = 100_000_000
	w.Stats.PeakCash = 100_000_000
	clock.EndDay(w) // the doors open, the prices stamped
	sup := w.WholesaleSupplier(row.City)
	if sup == nil || sup.Owned || mk.Owned(w, sup) {
		t.Fatalf("the wholesaler in %s reads owned with nothing owned: %+v", row.City, sup)
	}
	before := map[string]float64{}
	for id, p := range sup.Price {
		before[id] = p
	}
	cap := sup.Cap
	w.Assets = []game.Asset{{ID: row.ID, Name: row.Name, City: row.City}}
	clock.EndDay(w)
	sup = w.WholesaleSupplier(row.City)
	if !sup.Owned || !mk.Owned(w, sup) || sup.Left() != game.OwnedCap || sup.Cap == cap {
		t.Fatalf("owned: %+v", sup)
	}
	for id, p := range sup.Price {
		if want := w.Product(row.City, id).Price * row.OwnRatio; math.Abs(p-want) > 1e-6 {
			t.Fatalf("%s at %.4f, want own_ratio's %.4f", id, p, want)
		}
	}
	for _, other := range w.SuppliersIn(row.City) {
		if other.ID != sup.ID && other.Owned {
			t.Fatalf("%s reads owned", other.Name)
		}
	}
	// A buy nudges nothing on the owned book, and takes the lot past
	// the old cap.
	id := w.Products[0]
	price := sup.Price[id]
	if _, err := w.Restock(row.City, id, cap/sup.Lot+1, cfg.Market.Market.BuyPricePressure); err != nil {
		t.Fatalf("a buy past the old cap: %v", err)
	}
	if sup.Price[id] != price {
		t.Fatalf("the owned book's price moved on a buy: %.4f -> %.4f", price, sup.Price[id])
	}
	// Idle for unpaid upkeep, the book is the wholesaler's again.
	w.Assets[0].FrozenUntil = w.Day + 5
	clock.EndDay(w)
	sup = w.WholesaleSupplier(row.City)
	if sup.Owned || sup.Left() == game.OwnedCap {
		t.Fatal("an idle asset's book is still yours")
	}
	// The cartel war (#48, an incident): every connect's price is mul
	// of itself for the window, then the file's again.
	w.Assets = nil
	clock.EndDay(w)
	base := map[string]float64{}
	for _, s := range w.Suppliers {
		for id, p := range s.Price {
			base[s.ID+"/"+id] = p / w.Product(s.City, id).Price
		}
	}
	w.ApplyIncident(w.Day+1, content.IncidentEffects{SupplierShock: content.TimedMul{Mul: 2, Days: 2}}, game.IncidentTarget{City: w.Home().ID})
	clock.EndDay(w)
	for _, s := range w.Suppliers {
		for id, p := range s.Price {
			if want := base[s.ID+"/"+id] * 2 * w.Product(s.City, id).Price; math.Abs(p-want) > 1e-6 {
				t.Fatalf("%s %s under the war: %.4f, want doubled %.4f", s.Name, id, p, want)
			}
		}
	}
	clock.EndDay(w)
	clock.EndDay(w)
	for _, s := range w.Suppliers {
		for id, p := range s.Price {
			if want := base[s.ID+"/"+id] * w.Product(s.City, id).Price; math.Abs(p-want) > 1e-6 {
				t.Fatalf("%s %s after the war: %.4f, want %.4f", s.Name, id, p, want)
			}
		}
	}
}
