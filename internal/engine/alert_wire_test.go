package engine_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/engine"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/harness"
)

// TestAlertsCarryTheirClass (#549): every alert on the wire carries
// Danger() and Notice() as its danger and notice, both always sent, so
// a front end outside Go styles a danger without working the rule out;
// over sixty nights of the harness's boss and of an aggressive trader
// every alert agrees, the preview's too, and a danger and a notice
// both come.
func TestAlertsCarryTheirClass(t *testing.T) {
	t.Parallel()
	cfg := content.MustLoad()
	var dangers, notices int
	for _, policy := range []harness.Policy{harness.Boss(cfg, 40, ""), harness.Trader(cfg, events.DialAggressive)} {
		s, err := engine.New(cfg)
		if err != nil {
			t.Fatal(err)
		}
		w := s.NewRun(7, game.Start{})
		for night := 0; night < 60 && w.Over == nil; night++ {
			for _, a := range s.View().Alerts {
				if a.IsDanger != a.Danger() || a.IsNotice != a.Notice() {
					t.Fatalf("night %d: %s carries danger %v notice %v, the rule says %v %v", night, a.Kind, a.IsDanger, a.IsNotice, a.Danger(), a.Notice())
				}
				if a.IsDanger {
					dangers++
				}
				if a.IsNotice {
					notices++
				}
			}
			if p := s.Preview(); p != nil {
				for i, a := range p.Alerts {
					if a.IsDanger != a.Danger() || a.IsNotice != a.Notice() {
						t.Fatalf("night %d: the preview's alert %d, %s, is unclassed", night, i, a.Kind)
					}
				}
			}
			if w.Dilemmas.Pending != nil {
				_, _ = s.Choose(0)
			}
			policy(w)
			s.EndDay()
		}
	}
	if dangers == 0 || notices == 0 {
		t.Errorf("sixty nights raised %d dangers and %d notices: the test proves nothing", dangers, notices)
	}
	raw, err := json.Marshal(engine.Alert{Kind: engine.AlertPlan})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"danger":false`) || !strings.Contains(string(raw), `"notice":false`) {
		t.Errorf("a plain alert leaves its class off the wire: %s", raw)
	}
}

// TestReignAlertSlips (#549): a reign under the share carries the
// mornings it has left, the TUI's "it breaks in N mornings" (#399),
// and none while it holds.
func TestReignAlertSlips(t *testing.T) {
	t.Parallel()
	cfg := content.MustLoad()
	s, w := crewRun(t)
	w.Day, w.Reign = 10, 5
	grace := cfg.Rivals.Endings.ReignGrace
	for slip, want := range map[int]int{0: 0, 1: grace, grace: 1} {
		w.ReignSlip = slip
		got := ofKind(s, engine.AlertReign)
		if len(got) != 1 || got[0].Slip != want {
			t.Errorf("slip %d of grace %d: the reign alert %+v, want slip %d", slip, grace, got, want)
		}
	}
}
