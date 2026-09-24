package engine_test

import (
	"slices"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/engine"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/harness"
)

// TestEveryAlertHasAnAct (#352): every kind names at least one act, each
// on a screen, and every alert a played run raises carries one of its
// kind's. The TUI's TestEveryAlertHasATarget holds the names to its
// screens and dialogs.
func TestEveryAlertHasAnAct(t *testing.T) {
	t.Parallel()
	for _, k := range engine.AlertKinds() {
		acts := engine.ActsOf(k)
		if len(acts) == 0 {
			t.Errorf("%s names no act", k)
		}
		for _, a := range acts {
			if a.Screen == "" {
				t.Errorf("%s: an act on no screen %+v", k, a)
			}
		}
	}
	cfg := content.MustLoad()
	s, err := engine.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	w := s.NewRun(5, game.Start{})
	policy := harness.Trader(cfg, events.DialAggressive)
	kinds := map[engine.AlertKind]bool{}
	for d := 0; d < 80 && w.Over == nil; d++ {
		policy(w)
		s.EndDay()
		for _, a := range s.Alerts() {
			kinds[a.Kind] = true
			if !slices.Contains(engine.ActsOf(a.Kind), a.Act) {
				t.Fatalf("day %d: %s carries %+v, not one of %+v", w.Day, a.Kind, a.Act, engine.ActsOf(a.Kind))
			}
		}
	}
	if len(kinds) == 0 {
		t.Fatal("eighty aggressive days and not one alert")
	}
}

// TestUnpostedAlerts (#352): a runner or an enforcer off every post is
// an alert, once, that names a corner to put them on: a runner the
// corner you hold that nobody works, else a free one where you stand; an
// enforcer the worked corner nobody guards. Posted, in a house, in a
// cell or not a corner's role, nobody is.
func TestUnpostedAlerts(t *testing.T) {
	t.Parallel()
	s, w := crewRun(t)
	home := w.Home()
	for _, c := range w.Corners() {
		if c.Held() {
			w.Corner(c.ID).Hand(game.OwnerNone, "", w.Day) // the corner a run starts on
		}
	}
	w.Crew.Members = []game.CrewMember{
		{ID: 1, Name: "Vee", Role: game.RoleRunner, Loyalty: 60, Nerve: 50},
		{ID: 2, Name: "Bo", Role: game.RoleEnforcer, Loyalty: 60, Nerve: 50},
		{ID: 3, Name: "Lou", Role: game.RoleAccountant, Loyalty: 60, Nerve: 50},
		{ID: 4, Name: "Cell", Role: game.RoleRunner, Loyalty: 60, Nerve: 50, JailedUntil: w.Day + 5},
	}
	got := ofKind(s, engine.AlertUnposted)
	if len(got) != 2 || got[0].Member != 1 || got[1].Member != 2 {
		t.Fatalf("unposted %+v", got)
	}
	// Nothing held: the runner's is a free corner where you stand, the
	// enforcer has nowhere to guard and the act is their row.
	if a := got[0]; a.Act != engine.ActsOf(engine.AlertUnposted)[0] || a.City != home.ID || w.Corner(a.Corner) == nil || w.Corner(a.Corner).Owner != game.OwnerNone {
		t.Errorf("the runner with nothing held: %+v", a)
	}
	if a := got[1]; a.Act != engine.ActsOf(engine.AlertUnposted)[1] || a.Corner != "" {
		t.Errorf("the enforcer with nothing held: %+v", a)
	}

	// A corner held and unworked is the runner's; a worked one with
	// nobody guarding it the enforcer's.
	idle, worked := &home.Corners[2], &home.Corners[3]
	idle.Owner = game.OwnerPlayer
	worked.Owner, worked.Runner = game.OwnerPlayer, game.You
	got = ofKind(s, engine.AlertUnposted)
	if len(got) != 2 || got[0].Corner != idle.ID || got[1].Corner != worked.ID || got[1].Act.Mode != engine.ModePost {
		t.Fatalf("with corners held: %+v", got)
	}
	keys := map[string]bool{}
	for _, a := range s.Alerts() {
		if keys[a.Key] {
			t.Errorf("key %q twice", a.Key)
		}
		keys[a.Key] = true
	}

	// Posted, nobody is.
	idle.Runner, worked.Enforcer = 1, 2
	if got := ofKind(s, engine.AlertUnposted); len(got) != 0 {
		t.Errorf("everybody posted: %+v", got)
	}

	// A new alert, so a fast-forward stops on it once.
	before := s.Alerts()
	idle.Runner = 0
	st := s.Stop(nil, before)
	if st.Kind != engine.StopAlert || (st.Alert.Kind != engine.AlertUnposted && st.Alert.Kind != engine.AlertIdleCorner) {
		t.Fatalf("a runner off a post stopped %+v", st)
	}
	if st := s.Stop(nil, s.Alerts()); st.Kind == engine.StopAlert {
		t.Fatalf("stopped twice: %+v", st)
	}
}

// TestStashFullAlerts (#352): a city whose stash holds full_share of
// its capacity is an alert, once, on the ledger with the city; under it,
// or with nothing to hold anything, it is not.
func TestStashFullAlerts(t *testing.T) {
	t.Parallel()
	s, w := crewRun(t)
	share := content.MustLoad().Houses.Houses.FullShare
	if share <= 0 {
		t.Skip("no full_share in houses.toml")
	}
	here := w.Here().ID
	room := w.Capacity(here)
	product := w.Products[0]
	w.SetStock(here, product, int(share*float64(room))-1)
	if got := ofKind(s, engine.AlertStashFull); len(got) != 0 {
		t.Fatalf("under the share: %+v", got)
	}
	before := s.Alerts()
	w.SetStock(here, product, room)
	got := ofKind(s, engine.AlertStashFull)
	if len(got) != 1 || got[0].City != here || got[0].Count != room || got[0].Amount != room ||
		got[0].Act != (engine.Act{Screen: engine.ScreenLedger, Subject: engine.SubjectCity}) {
		t.Fatalf("full: %+v", got)
	}
	if st := s.Stop(nil, before); st.Kind != engine.StopAlert || st.Alert.Kind != engine.AlertStashFull {
		t.Errorf("a stash gone full stopped %+v", st)
	}
	if st := s.Stop(nil, s.Alerts()); st.Kind == engine.StopAlert && st.Alert.Kind == engine.AlertStashFull {
		t.Errorf("the full stash stopped twice: %+v", st)
	}
}
