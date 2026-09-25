package engine_test

import (
	"testing"

	"github.com/theclifmeister/kingpin/internal/engine"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
)

// TestReignStopsOnce (#399): the fast-forward stops the morning the
// run's first reign begins, runs past a reign begun again, and stops
// on the reign breaking (which comes past the grace) and on going
// straight opening or lapsing (#398).
func TestReignStopsOnce(t *testing.T) {
	for _, c := range []struct {
		e    events.Event
		want bool
	}{
		{events.ReignBegan{}, true},
		{events.ReignBegan{Again: true}, false},
		{events.ReignBroken{}, true},
		{events.StraightOpened{}, true},
		{events.StraightLapsed{}, true},
	} {
		if got := engine.StopsOn(c.e); got != c.want {
			t.Errorf("StopsOn(%+v) = %v, want %v", c.e, got, c.want)
		}
	}
}

// TestOfferStopsWhereYouCanAnswerIt (#442): a buyer's offer stops the
// fast-forward in a city you stand in, work a corner in or hold stock
// in, and runs past one anywhere else; other events stop as StopsOn says.
func TestOfferStopsWhereYouCanAnswerIt(t *testing.T) {
	t.Parallel()
	s, w := crewRun(t)
	home, away := w.Player.Location, ""
	for _, id := range w.CityOrder {
		if id != home {
			away = id
			break
		}
	}
	for _, c := range w.Corners() {
		if c.City == away && c.Held() {
			w.Corner(c.ID).Hand(game.OwnerNone, "", w.Day)
		}
	}
	for _, id := range w.Products {
		w.SetStock(away, id, 0)
	}
	offer := func(city string) []events.Event {
		return []events.Event{events.ContractOffered{City: city, Product: w.Products[0], Units: 10}}
	}
	before := s.Alerts()
	if st := s.Stop(offer(home), before); st.Kind != engine.StopEvent {
		t.Fatalf("an offer where you stand did not stop: %+v", st)
	}
	if st := s.Stop(offer(away), before); st.Kind == engine.StopEvent {
		t.Fatalf("an offer where you have nothing stopped: %+v", st)
	}
	w.SetStock(away, w.Products[0], 5)
	if st := s.Stop(offer(away), before); st.Kind != engine.StopEvent {
		t.Fatalf("an offer where you hold stock did not stop: %+v", st)
	}
	w.SetStock(away, w.Products[0], 0)
	corner := &w.Cities[away].Corners[0]
	corner.Owner, corner.Runner = game.OwnerPlayer, game.You
	if st := s.Stop(offer(away), s.Alerts()); st.Kind != engine.StopEvent {
		t.Fatalf("an offer where you work a corner did not stop: %+v", st)
	}
	if st := s.Stop([]events.Event{events.ChiefReplaced{}}, s.Alerts()); st.Kind != engine.StopEvent {
		t.Fatalf("another stopping event ran past: %+v", st)
	}
}
