package engine_test

import (
	"testing"

	"github.com/theclifmeister/kingpin/internal/game"
)

// TestViewCarriesWhatAFrontWashedToday (#588): view 23 carries what a
// front washed on the last day stepped, beside its lifetime washed, as
// the ledger's front pane shows them.
func TestViewCarriesWhatAFrontWashedToday(t *testing.T) {
	t.Parallel()
	s, w := freshSession(t)
	w.Fronts = append(w.Fronts, game.Front{ID: "laundromat", Name: "Laundromat", Washed: 4200, WashedToday: 350})
	v := s.View()
	if len(v.Fronts) != 1 {
		t.Fatalf("fronts: %+v", v.Fronts)
	}
	if f := v.Fronts[0]; f.WashedToday != 350 || f.Washed != 4200 {
		t.Errorf("the laundromat: %+v", f)
	}
}
