package engine

import (
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
)

// TestRepeatedOfferRunsPast (#469): a faction's offer stops a
// fast-forward the first time; the same faction offering the same kind
// of deal again within OfferQuiet (the last let lapse or turned down)
// runs past, however often it asks; a different kind, another faction
// or an offer after a quiet spell stops. A load starts the memo over.
func TestRepeatedOfferRunsPast(t *testing.T) {
	t.Parallel()
	s, err := New(content.MustLoad())
	if err != nil {
		t.Fatal(err)
	}
	w := s.NewRun(3, game.Start{})
	stops := func(day int, faction, deal string) bool {
		ev := events.DealOffered{Day: day, Faction: faction, Rival: "Rosalind", Deal: deal}
		s.remember([]events.Event{ev})
		w.Dilemmas.Pending = nil
		return s.Stop([]events.Event{ev}, s.Alerts()).Kind == StopEvent
	}
	if !stops(10, "f2", game.DealTribute) {
		t.Fatal("the first offer did not stop")
	}
	for day := 12; day < 40; day += 2 {
		if stops(day, "f2", game.DealTribute) {
			t.Fatalf("the tribute asked again on day %d stopped", day)
		}
	}
	if !stops(40, "f2", game.DealTruce) {
		t.Fatal("a truce after the tributes did not stop")
	}
	if !stops(41, "f3", game.DealTribute) {
		t.Fatal("another faction's tribute did not stop")
	}
	if !stops(38+OfferQuiet+1, "f2", game.DealTribute) {
		t.Fatal("a tribute after a quiet spell did not stop")
	}
	s.Attach(w)
	if !stops(38+OfferQuiet+2, "f2", game.DealTribute) {
		t.Fatal("a tribute after a fresh attach did not stop")
	}
}
