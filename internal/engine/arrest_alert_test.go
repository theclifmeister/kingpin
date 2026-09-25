package engine_test

import (
	"strings"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/engine"
	"github.com/theclifmeister/kingpin/internal/events"
)

// TestArrestAlert (#475): a warrant out this morning is the loudest
// alert, with the city, its heat, the arrest line and the night it is
// served; it is keyed by the night it was signed, so a fast-forward
// stops once on it; WarrantSigned stops one too. The heat alert is
// keyed by the highest rung met under the arrest, so a fast-forward
// stops again as the heat crosses the sting line.
func TestArrestAlert(t *testing.T) {
	t.Parallel()
	s, w := crewRun(t)
	if got := ofKind(s, engine.AlertArrest); len(got) != 0 {
		t.Fatalf("an arrest alert with no warrant: %+v", got)
	}
	before := s.Alerts()
	w.Day = 12
	w.Heat.WarrantDay = w.Day
	w.Here().Heat = 96
	got := s.Alerts()
	if len(got) == 0 || got[0].Kind != engine.AlertArrest {
		t.Fatalf("the warrant is not the loudest alert: %+v", got)
	}
	a := got[0]
	if a.City != w.Here().ID || a.Heat != 96 || a.Line <= 0 || a.Due != w.Day+1 || a.Days != 1 || a.Key != "warrant signed 12" || a.Act.Screen != engine.ScreenDashboard {
		t.Fatalf("the arrest alert: %+v", a)
	}
	if st := s.Stop(nil, before); st.Kind != engine.StopAlert || st.Alert.Kind != engine.AlertArrest {
		t.Fatalf("a warrant out does not stop a fast-forward: %+v", st)
	}
	if st := s.Stop(nil, s.Alerts()); st.Kind == engine.StopAlert && st.Alert.Kind == engine.AlertArrest {
		t.Fatal("the same warrant stopped twice")
	}
	if !engine.StopsOn(events.WarrantSigned{}) {
		t.Fatal("WarrantSigned does not stop a fast-forward")
	}

	// The heat alert by rung: the patrol's key as before, the sting's
	// once the heat is over its line.
	w.Heat.WarrantDay = 0
	cfg := content.MustLoad()
	line := func(level string) float64 {
		for _, r := range cfg.Heat.Responses {
			if r.Level == level {
				return r.Threshold
			}
		}
		t.Fatalf("no rung %s", level)
		return 0
	}
	w.Here().Heat = line(content.Patrol) + 1
	heat := ofKind(s, engine.AlertHeat)
	if len(heat) != 1 || heat[0].Level != content.Patrol || !strings.HasSuffix(heat[0].Key, "over the patrol line") {
		t.Fatalf("over the patrol line: %+v", heat)
	}
	w.Here().Heat = line(content.Sting) + 1
	heat = ofKind(s, engine.AlertHeat)
	if len(heat) != 1 || heat[0].Level != content.Sting || !strings.HasSuffix(heat[0].Key, "over the sting line") {
		t.Fatalf("over the sting line: %+v", heat)
	}
}
