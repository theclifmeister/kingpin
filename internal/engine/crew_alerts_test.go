package engine_test

import (
	"math"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/engine"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
)

// crewRun is a fresh run on a quiet morning with the pay at fair and
// the wages covered, for the crew trouble tests to put people on.
func crewRun(t *testing.T) (*engine.Session, *game.World) {
	t.Helper()
	s, err := engine.New(content.MustLoad())
	if err != nil {
		t.Fatal(err)
	}
	w := s.NewRun(3, game.Start{})
	w.Crew.Pay = events.PayFair
	w.Player.DirtyCash = 100000
	return s, w
}

// ofKind is the morning's alerts of one kind.
func ofKind(s *engine.Session, kind engine.AlertKind) []engine.Alert {
	var out []engine.Alert
	for _, a := range s.Alerts() {
		if a.Kind == kind {
			out = append(out, a)
		}
	}
	return out
}

// TestCrewTroubleAlerts (#345): a table of mornings and the crew
// trouble each raises, each once: a member within alert_margin over
// the skim line, a lieutenant over the flip line, a member over the
// walk; nobody where the line is further off and the drift is not
// falling; the skim while it is suspected; a corner held and unworked
// with the days before it drifts.
func TestCrewTroubleAlerts(t *testing.T) {
	t.Parallel()
	cfg := content.MustLoad()
	tun := cfg.Crew.Crew
	flip := cfg.Crew.Lieutenant.Flip
	for _, c := range []struct {
		name    string
		role    string
		loyalty float64
		cross   string // "" for no alert
		line    float64
	}{
		{"steady", game.RoleRunner, 60, "", 0},
		{"near the skim", game.RoleRunner, tun.SkimThreshold + 3, "skim", tun.SkimThreshold},
		{"under the skim, far from the walk", game.RoleRunner, tun.SkimThreshold - 1, "", 0},
		{"near the walk", game.RoleEnforcer, tun.QuitThreshold + 4, "walk", tun.QuitThreshold},
		{"a lieutenant near the flip", game.RoleLieutenant, flip + 2, "flip", flip},
		{"a lieutenant past the flip, near the walk", game.RoleLieutenant, tun.QuitThreshold + 1, "walk", tun.QuitThreshold},
	} {
		s, w := crewRun(t)
		w.Crew.Members = []game.CrewMember{{ID: 7, Name: "Deshawn", Role: c.role, Skill: 50, Loyalty: c.loyalty, Nerve: 50}}
		got := ofKind(s, engine.AlertCrewLine)
		if c.cross == "" {
			if len(got) != 0 {
				t.Errorf("%s: alerts %+v", c.name, got)
			}
			continue
		}
		if len(got) != 1 {
			t.Fatalf("%s: %d crew line alerts, want 1: %+v", c.name, len(got), got)
		}
		a := got[0]
		if a.Member != 7 || a.Cross != c.cross || a.Line != c.line || math.Abs(a.Gap-(c.loyalty-c.line)) > 1e-9 || a.Days != 0 {
			t.Errorf("%s: alert %+v", c.name, a)
		}
	}

	// Falling: a greedy member on stingy pay is an alert the night the
	// drift takes them over the walk, from outside the margin, and the
	// days say so.
	s, w := crewRun(t)
	w.Crew.Pay = events.PayStingy
	w.Player.DirtyCash = 0 // the wages come up short too
	w.Player.Reputation.Respect = 0
	w.Crew.Members = []game.CrewMember{{ID: 4, Name: "Tariq", Role: game.RoleRunner, Skill: 50, Loyalty: tun.QuitThreshold + tun.AlertMargin + 1, Greed: 100, Nerve: 50, Wage: 50}}
	if got := ofKind(s, engine.AlertCrewLine); len(got) != 1 || got[0].Cross != "walk" || got[0].Days != 1 {
		t.Errorf("falling over the walk tonight: %+v", got)
	}

	// The skim, while it is suspected and not after.
	s, w = crewRun(t)
	w.Day = 20
	w.Crew.LastSkim = 18
	if got := ofKind(s, engine.AlertSkim); len(got) != 1 || got[0].Day != 18 {
		t.Errorf("skim suspected: %+v", got)
	}
	w.Crew.LastSkim = w.Day - tun.SuspectDays
	if got := ofKind(s, engine.AlertSkim); len(got) != 0 {
		t.Errorf("skim long past: %+v", got)
	}

	// A corner held with nobody on it, and one worked.
	s, w = crewRun(t)
	home := w.Home()
	idle, worked := &home.Corners[0], &home.Corners[1]
	idle.Owner, idle.Idle = game.OwnerPlayer, 1
	worked.Owner, worked.Runner = game.OwnerPlayer, game.You
	got := ofKind(s, engine.AlertIdleCorner)
	if len(got) != 1 || got[0].Corner != idle.ID || got[0].City != home.ID || got[0].Days != cfg.City.Territory.DriftDays-1 {
		t.Errorf("idle corner: %+v", got)
	}

	// Each once: no two alerts share a key.
	s, w = crewRun(t)
	w.Crew.Members = []game.CrewMember{
		{ID: 1, Name: "A", Role: game.RoleRunner, Loyalty: tun.SkimThreshold + 1, Nerve: 50},
		{ID: 2, Name: "B", Role: game.RoleRunner, Loyalty: tun.QuitThreshold + 1, Nerve: 50},
	}
	w.Home().Corners[0].Owner = game.OwnerPlayer
	keys := map[string]bool{}
	for _, a := range s.Alerts() {
		if keys[a.Key] {
			t.Errorf("key %q twice", a.Key)
		}
		keys[a.Key] = true
	}
	if n := len(ofKind(s, engine.AlertCrewLine)); n != 2 {
		t.Errorf("%d crew line alerts, want 2", n)
	}
}

// TestCrewDriftIsTheNights (#345): the drift the crew line alert reads
// its days off is the move a quiet night makes.
func TestCrewDriftIsTheNights(t *testing.T) {
	t.Parallel()
	s, w := crewRun(t)
	w.Crew.Pay = events.PayStingy
	w.Crew.Members = []game.CrewMember{{ID: 9, Name: "Lou", Role: game.RoleAccountant, Skill: 40, Loyalty: 60, Greed: 70, Nerve: 30}}
	w.Crew.Members[0].Loyalty = content.MustLoad().Crew.Crew.SkimThreshold + 1
	before := w.Crew.Members[0].Loyalty
	got := ofKind(s, engine.AlertCrewLine)
	if len(got) != 1 {
		t.Fatalf("alerts %+v", got)
	}
	s.EndDay()
	m := w.Crew.Member(9)
	if m == nil {
		t.Fatal("Lou is gone")
	}
	moved := before - m.Loyalty
	if moved <= 0 {
		t.Fatalf("stingy and greedy and loyalty rose %v", -moved)
	}
	if days := int(math.Ceil(got[0].Gap / moved)); days != got[0].Days {
		t.Errorf("the alert said %d days at tonight's drift, the night moved %v (%d days)", got[0].Days, moved, days)
	}
}

// TestFastForwardStopsOnIdleCorner (#345): a corner that goes unworked
// is a new alert, so a fast-forward stops on it once; left alone it
// drifts back to the street, and that night stops too.
func TestFastForwardStopsOnIdleCorner(t *testing.T) {
	t.Parallel()
	s, w := crewRun(t)
	c := &w.Home().Corners[0]
	c.Owner, c.Runner = game.OwnerPlayer, game.You
	before := s.Alerts()
	c.Runner = 0
	st := s.Stop(nil, before)
	if st.Kind != engine.StopAlert || st.Alert.Kind != engine.AlertIdleCorner || st.Alert.Corner != c.ID {
		t.Fatalf("a corner gone idle stopped %+v", st)
	}
	if st := s.Stop(nil, s.Alerts()); st.Kind == engine.StopAlert && st.Alert.Kind == engine.AlertIdleCorner {
		t.Fatalf("the idle corner stopped twice: %+v", st)
	}
	if !engine.StopsOn(events.CornerLost{Reason: "idle", Owner: game.OwnerPlayer}) {
		t.Fatal("an idle corner lost does not stop")
	}
	if engine.StopsOn(events.CornerLost{Reason: "crackdown", Owner: game.OwnerRival}) {
		t.Fatal("a rival corner cleared stops")
	}

	// Left alone, a fast-forward never runs past the night it drifts:
	// whatever else stops it on the way, one stops there.
	id := c.ID
	for i := 0; ; i++ {
		ran, st, evs := s.FastForward(30, nil)
		if st.Kind == engine.StopCap || st.Kind == engine.StopOver || i > 30 {
			t.Fatalf("ran %d days and stopped %+v before the corner drifted", ran, st)
		}
		lost := false
		for _, e := range evs {
			if ev, ok := e.(events.CornerLost); ok && ev.Corner == id && ev.Reason == "idle" {
				lost = true
			}
		}
		if lost {
			break
		}
		if w.Corner(id).Owner != game.OwnerPlayer {
			t.Fatalf("the corner went on a day the fast-forward ran past (stopped %+v)", st)
		}
	}
}
