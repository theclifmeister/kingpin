package engine_test

import (
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/engine"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
)

// TestTillAlert (#459): a playtest sat at exactly $50,000 dirty for
// forty nights, the wash taking everything over the till, and could
// never save. Once the wash has left the pile at the till till_nights
// running, the dashboard says so and points at the careful dial; not a
// night sooner, not with the dial already careful, not with no front,
// and not once a night closed over the till.
func TestTillAlert(t *testing.T) {
	t.Parallel()
	cfg := content.MustLoad()
	need := cfg.Laundering.Laundering.TillNights
	if need <= 0 {
		t.Fatal("till_nights is off in the file")
	}
	night := func(closing int) game.CashFlow {
		return game.NewCashFlow(0, map[string]game.Pools{game.FlowLaundering: {Dirty: -4000, Clean: 4000}}, game.Pools{Dirty: closing})
	}
	for _, c := range []struct {
		name   string
		set    func(w *game.World, till int)
		nights int // 0: no alert
	}{
		{"at the till", func(w *game.World, till int) {}, need + 1},
		{"a night short", func(w *game.World, till int) { w.Flows = w.Flows[2:] }, 0},
		{"careful already", func(w *game.World, till int) { w.Laundering.Dial = events.LaunderCareful }, 0},
		{"the till raised", func(w *game.World, till int) { w.Laundering.Till = till + 1 }, 0}, // held on purpose (#496)
		{"no front", func(w *game.World, till int) { w.Fronts = nil }, 0},
		{"last night over the till", func(w *game.World, till int) { w.Flows[len(w.Flows)-1] = night(till + 1) }, 0},
		{"no wash last night", func(w *game.World, till int) {
			w.Flows[len(w.Flows)-1] = game.NewCashFlow(0, nil, game.Pools{Dirty: till})
		}, 0},
		{"an older night over the till", func(w *game.World, till int) { w.Flows[0] = night(till + 1) }, need},
	} {
		t.Run(c.name, func(t *testing.T) {
			s, w := crewRun(t)
			till := w.Float(cfg.Upgrades, cfg.Laundering.Laundering.Float)
			w.Fronts = append(w.Fronts, game.Front{ID: cfg.Laundering.Fronts[0].ID, Name: cfg.Laundering.Fronts[0].Name})
			w.Laundering.Dial = events.LaunderNormal
			w.Player.DirtyCash = till
			w.Flows = nil
			for range need + 1 {
				w.Flows = append(w.Flows, night(till))
			}
			c.set(w, till)
			got := ofKind(s, engine.AlertTill)
			if c.nights == 0 {
				if len(got) != 0 {
					t.Fatalf("a till alert: %+v", got)
				}
				return
			}
			if len(got) != 1 || got[0].Days != c.nights || got[0].Amount != till || got[0].Have != till || got[0].Key == "" {
				t.Fatalf("till alerts %+v, want one of %d nights at %d", got, c.nights, till)
			}
			if acts := engine.ActsOf(engine.AlertTill); len(acts) == 0 || acts[0].Screen != engine.ScreenLedger {
				t.Fatalf("the till alert opens %+v, not the ledger", acts)
			}
		})
	}
}
