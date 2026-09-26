package engine_test

import (
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/engine"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/harness"
)

// TestInformantPagesAlert (#492): an informant on the payroll files a
// page with no sting, raid or investigation, and the morning after it
// is an alert naming the cause, on the crew screen where i is, with the
// file; it is a danger, it stops a fast-forward that morning and is
// gone the next.
func TestInformantPagesAlert(t *testing.T) {
	t.Parallel()
	cfg := content.MustLoad()
	s, w := crewRun(t)
	harness.Plant(cfg, w)
	var got []engine.Alert
	var st engine.Stop
	for night := 0; night < 3*cfg.Heat.Heat.InformantDays && len(got) == 0; night++ {
		w.SetLieLow(true)
		before := s.Alerts()
		evs := s.EndDay()
		answered(w)
		got = ofKind(s, engine.AlertPages)
		st = s.Stop(evs, before)
		for _, e := range evs {
			if en, ok := e.(events.Enforcement); ok && en.Evidence > 0 {
				t.Fatalf("a bust filed pages: %+v", en)
			}
		}
	}
	if len(got) != 1 {
		t.Fatalf("no pages alert in %d nights with an informant: %+v", 3*cfg.Heat.Heat.InformantDays, s.Alerts())
	}
	a := got[0]
	if a.Level != engine.PagesInformant || a.Have != cfg.Heat.Heat.InformantEvidence || a.Count != w.Heat.Evidence ||
		a.Amount <= 0 || a.Act != (engine.Act{Screen: engine.ScreenCrew}) || !a.Danger() || a.Notice() {
		t.Fatalf("the pages alert: %+v (file %d)", a, w.Heat.Evidence)
	}
	if st.Kind != engine.StopAlert || st.Alert.Kind != engine.AlertPages || !st.Danger() {
		t.Fatalf("the night's stop: %+v", st)
	}
	w.SetLieLow(true)
	s.EndDay()
	if got := ofKind(s, engine.AlertPages); len(got) != 0 && got[0].Key == a.Key {
		t.Fatalf("the same night's pages stood a second morning: %+v", got)
	}
}

// TestNoticesNeverStop (#504): the till, the float, a gate within reach
// and a full stash are notices; the file, a warrant, a task force, an
// informant and a member under the informant line are dangers, and a
// danger event stop is one too.
func TestNoticesNeverStop(t *testing.T) {
	t.Parallel()
	for _, k := range engine.AlertKinds() {
		a := engine.Alert{Kind: k}
		if a.Notice() && a.Danger() {
			t.Errorf("%s is both a notice and a danger", k)
		}
	}
	for _, k := range []engine.AlertKind{engine.AlertTill, engine.AlertFloat, engine.AlertGate, engine.AlertStashFull} {
		if !(engine.Alert{Kind: k}).Notice() {
			t.Errorf("%s is no notice", k)
		}
	}
	for _, a := range []engine.Alert{{Kind: engine.AlertFile}, {Kind: engine.AlertArrest}, {Kind: engine.AlertTaskForce}, {Kind: engine.AlertTalking},
		{Kind: engine.AlertPages}, {Kind: engine.AlertCrewLine, Cross: "under"}} {
		if !a.Danger() {
			t.Errorf("%+v is no danger", a)
		}
	}
	if (engine.Alert{Kind: engine.AlertCrewLine, Cross: "walk"}).Danger() {
		t.Error("a member near the walk is a danger")
	}
	for _, c := range []struct {
		e    events.Event
		want bool
	}{
		{events.WarrantSigned{}, true},
		{events.TaskForceFormed{}, true},
		{events.Enforcement{Level: content.Sting}, true},
		{events.ChiefReplaced{}, false},
	} {
		if got := (engine.Stop{Kind: engine.StopEvent, Event: c.e}).Danger(); got != c.want {
			t.Errorf("%T: danger %v, want %v", c.e, got, c.want)
		}
	}

	s, w := crewRun(t)
	before := s.Alerts()
	w.Player.DirtyCash = 0 // under the float, were a front owned
	for _, g := range s.GatesAhead() {
		w.Stats.PeakCash = g.Line - 1 // within reach
		break
	}
	for _, a := range s.Alerts() {
		if a.Notice() {
			if st := s.Stop(nil, before); st.Kind == engine.StopAlert && st.Alert.Notice() {
				t.Fatalf("a notice stopped: %+v", st)
			}
			return
		}
	}
	t.Fatalf("no notice raised: %+v", s.Alerts())
}

// TestQuietFastForwardStopsLittle (#504): a mid-game run (the
// distributor ninety days in: fronts, a route, standing orders) with the
// danger cleared, fast-forwarded thirty days on its routines. The stops
// are counted and pinned under a ceiling, and none is a notice: before
// #504 seed 7 stopped on the float too (13 stops, 2 neither a card nor
// a buyer). A playtest's F stopped about every 1.3 days mid-game on the
// till, the float, a full stash and "within reach".
func TestQuietFastForwardStopsLittle(t *testing.T) {
	t.Parallel()
	cfg := content.MustLoad()
	s, err := engine.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	for _, seed := range []uint64{7, 11} {
		w := s.NewRun(seed, game.Start{})
		var policy harness.Policy
		for _, p := range harness.Policies {
			if p.Name == "distributor" {
				policy = p.Make(cfg, harness.PolicyOpts{})
			}
		}
		for d := 0; d < 90 && w.Over == nil; d++ {
			policy(w)
			s.EndDay()
		}
		if w.Over != nil {
			t.Fatalf("seed %d: the fixture ended on day %d", seed, w.Day)
		}
		// No danger: no heat, no file, nobody talking, no warrant, no
		// investigation.
		for _, c := range w.Cities {
			c.Heat = 0
		}
		w.Heat.Evidence, w.Heat.Leaks, w.Heat.WarrantDay, w.Heat.Investigation = 0, 0, 0, game.Investigation{}
		for i := range w.Crew.Members {
			w.Crew.Members[i].Informant = false
		}
		answered(w)
		stops, other, ran := 0, 0, 0
		for ran < 30 && w.Over == nil {
			n, st, _ := s.FastForward(30-ran, nil)
			ran += n
			if st.Kind == engine.StopCap || st.Kind == engine.StopOver {
				break
			}
			stops++
			t.Logf("seed %d day %d: %s %s %T", seed, w.Day, st.Kind, st.Alert.Key, st.Event)
			if _, offer := st.Event.(events.ContractOffered); st.Kind != engine.StopCard && st.Kind != engine.StopStage && !offer {
				other++ // neither a card nor a buyer asking: both want an answer
			}
			if st.Kind == engine.StopAlert && st.Alert.Notice() {
				t.Errorf("seed %d day %d: stopped on a notice: %+v", seed, w.Day, st.Alert)
			}
			answered(w)
		}
		t.Logf("seed %d: %d stops in %d days, %d neither a card nor a buyer", seed, stops, ran, other)
		if stops > quietStops || other > quietOther {
			t.Errorf("seed %d: %d stops (%d neither a card nor a buyer) in %d quiet days, want %d (%d) at most", seed, stops, other, ran, quietStops, quietOther)
		}
	}
}

// quietStops is the most stops thirty quiet mid-game days may make
// (#504), and quietOther the most of them neither a card dealt nor a
// buyer asking, each of which wants an answer: the rest about one a
// fortnight, where a playtest stopped every 1.3 days on notices.
const (
	quietStops = 12
	quietOther = 1
)
