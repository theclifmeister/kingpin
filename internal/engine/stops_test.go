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

// TestShortfallStopsOnce (#469): a standing order short every night
// stops the fast-forward the morning it starts and never again while it
// goes on, so F 30 runs on to a real event; one a landing covered the
// same night does not stop at all, and a supply contract short of room
// never does.
func TestShortfallStopsOnce(t *testing.T) {
	t.Parallel()
	s, w := crewRun(t)
	city, product := w.Player.Location, w.Products[0]
	w.SetStock(city, product, 5)
	w.Standing = map[string]game.SellOrder{game.OrderKey(city, product): {City: city, Product: product, Qty: 10_000, Dial: events.DialNormal}}

	// The first night is short: it stops, the second does not.
	short := func(evs []events.Event) []events.Event {
		var out []events.Event
		for _, e := range evs {
			if ev, ok := e.(events.StandingShort); ok && ev.City == city && ev.Product == product {
				out = append(out, e)
			}
		}
		return out
	}
	first := short(s.EndDay())
	if len(first) != 1 {
		t.Fatalf("the first night was not short: %v", first)
	}
	if st := s.Stop(first, s.Alerts()); st.Kind != engine.StopEvent {
		t.Fatalf("the shortfall's first morning did not stop: %+v", st)
	}
	again := short(s.EndDay())
	if len(again) != 1 {
		t.Fatalf("the second night was not short: %v", again)
	}
	if st := s.Stop(again, s.Alerts()); st.Kind == engine.StopEvent {
		t.Fatalf("the shortfall stopped a second morning: %+v", st)
	}

	// F 30, again and again: the order is short every night and never
	// the reason the run stopped.
	for i := 0; i < 6 && w.Over == nil; i++ {
		ran, st, evs := s.FastForward(30, nil)
		t.Logf("F 30 ran %d days, stopped %s %v", ran, st.Kind, st.Event)
		if len(short(evs)) == 0 && st.Kind != engine.StopOver {
			t.Fatalf("the order was not short the night the run stopped (%+v)", st)
		}
		if ev, ok := st.Event.(events.StandingShort); st.Kind == engine.StopEvent && ok {
			t.Fatalf("F stopped on the standing order again: %+v", ev)
		}
		answered(w)
	}

	// A shortfall a landing covered the same night (the stash holds the
	// order by the morning) does not stop, even the first time.
	other := w.Products[1]
	w.SetStock(city, other, 50)
	covered := []events.Event{events.StandingShort{City: city, Product: other, Units: 40, Stock: 0}}
	if st := s.Stop(covered, s.Alerts()); st.Kind == engine.StopEvent {
		t.Fatalf("a shortfall the night covered stopped: %+v", st)
	}
	w.SetStock(city, other, 0)
	if st := s.Stop(covered, s.Alerts()); st.Kind != engine.StopEvent {
		t.Fatalf("a new shortfall did not stop: %+v", st)
	}
	room := []events.Event{events.SupplyShort{City: city, Product: other, Units: 0, Short: 40, Why: "room"}}
	if st := s.Stop(room, s.Alerts()); st.Kind == engine.StopEvent {
		t.Fatalf("a supply contract short of room stopped: %+v", st)
	}
}

// answered is the morning dealt with: the card answered, the stages
// seen, so Stop weighs the events.
func answered(w *game.World) {
	w.Dilemmas.Pending = nil
	for t := range w.Progression.Reached {
		if w.Progression.Seen == nil {
			w.Progression.Seen = map[int]bool{}
		}
		w.Progression.Seen[t] = true
	}
}

// TestLostCornersStop (#469): a corner of yours lost for any reason
// stops, the police's crackdown too.
func TestLostCornersStop(t *testing.T) {
	for _, why := range []string{"idle", "crackdown"} {
		if !engine.StopsOn(events.CornerLost{Reason: why, Owner: game.OwnerPlayer}) {
			t.Errorf("a corner of yours lost to %s does not stop", why)
		}
	}
	if !engine.StopsOn(events.CornerTaken{From: game.OwnerPlayer}) {
		t.Error("a corner taken off you does not stop")
	}
}

// TestLastCornerAlert (#471): the morning your last corner in a city
// goes, an alert says so, naming a free corner there to post on, and a
// fast-forward stops on it once; while you hold one, or never held
// one and have nothing there to sell, there is none.
func TestLastCornerAlert(t *testing.T) {
	t.Parallel()
	s, w := crewRun(t)
	home := w.Home()
	for i := range home.Corners {
		if home.Corners[i].Held() {
			home.Corners[i].Hand(game.OwnerNone, "", w.Day)
		}
	}
	c := &home.Corners[0]
	c.Hand(game.OwnerPlayer, "", w.Day)
	c.Runner, c.Yours = game.You, true
	if got := ofKind(s, engine.AlertNoCorner); len(got) != 0 {
		t.Fatalf("an alert while you hold a corner: %+v", got)
	}
	before := s.Alerts()
	c.Hand(game.OwnerRival, "", w.Day)
	st := s.Stop([]events.Event{events.CornerTaken{Corner: c.ID, Name: c.Name, From: game.OwnerPlayer}}, before)
	if st.Kind != engine.StopAlert || st.Alert.Kind != engine.AlertNoCorner || st.Alert.City != home.ID {
		t.Fatalf("the last corner gone stopped %+v", st)
	}
	free := w.Corner(st.Alert.Corner)
	if free == nil || free.City != home.ID || free.Owner != game.OwnerNone || st.Alert.Act.Mode != engine.ModePost {
		t.Fatalf("the alert names no free corner to post on: %+v", st.Alert)
	}
	if st := s.Stop(nil, s.Alerts()); st.Kind == engine.StopAlert && st.Alert.Kind == engine.AlertNoCorner {
		t.Fatalf("the alert stopped twice: %+v", st)
	}
	// Somewhere you never held a corner and have nothing to sell is not
	// a corner lost.
	for _, a := range ofKind(s, engine.AlertNoCorner) {
		if a.City != home.ID {
			t.Errorf("an alert where you never held a corner: %+v", a)
		}
	}
}
