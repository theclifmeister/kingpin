package crew_test

import (
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/game"
)

// TestCleanCashIsNotBroke (#395): the broke ending reads both piles. A
// run with no stock and no dirty cash but clean money on the books is
// short on the wages, not over: clean cash is drawn back into the
// dirty pile at a fee (World.CashOut). With neither pile it is broke,
// as it always was.
func TestCleanCashIsNotBroke(t *testing.T) {
	cfg := content.MustLoad()
	w, s := world(t, cfg, 0)
	for _, id := range w.Products {
		w.TakeStock(w.Here().ID, id, w.StockIn(w.Here().ID))
	}
	if w.TotalStock() != 0 {
		t.Fatalf("the fixture keeps %d units", w.TotalStock())
	}
	w.Crew.Members = append(w.Crew.Members, game.CrewMember{ID: 900, Name: "Runner", Role: game.RoleRunner, Skill: 60, Loyalty: 80, Nerve: 60, Wage: 60})
	w.Player.DirtyCash, w.Player.CleanCash = 0, 5_000_000
	step(w, s)
	if w.Over != nil {
		t.Fatalf("$5M clean ended the run %s", w.Over.Cause)
	}
	if w.Player.CleanCash != 5_000_000 {
		t.Fatalf("the wages took clean cash: %d left", w.Player.CleanCash)
	}
	w.Player.CleanCash = 0
	step(w, s)
	if w.Over == nil || w.Over.Cause != content.CauseBroke {
		t.Fatalf("no stock and no cash of either kind: over %+v, want broke", w.Over)
	}
}
