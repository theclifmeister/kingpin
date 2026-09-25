package news_test

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/gametest"
	"github.com/theclifmeister/kingpin/internal/sim"
	"github.com/theclifmeister/kingpin/internal/sim/news"
)

// reportW is the most a report line may run to and be read whole in
// the morning report at 80 columns (#459): the modal's 76, its border
// and padding, and the section's indent. The modal cuts, never wraps.
const reportW = 70

// TestStarvedRoutineSaysWhy (#459): a route that sent nothing says why
// in the report's SHIPMENTS, and a supply contract out of cash says
// what the wash took the night before, each line whole at 80 columns.
func TestStarvedRoutineSaysWhy(t *testing.T) {
	cfg := content.MustLoad()
	for _, tc := range []struct {
		name  string
		ev    events.Event
		flows []game.CashFlow
		want  []string
	}{
		{"a route under the till", events.RouteIdle{Route: "interstate", Name: "Interstate", From: "bayport", To: "eastside", Why: events.IdleTill, Products: []string{"coke"}, Till: 50_000, Dirty: 48_000},
			nil, []string{"Interstate idle: $48,000 dirty, none over the $50,000 till"}},
		{"a route over the till, short of a lot", events.RouteIdle{Route: "channel", Name: "The Channel", From: "bayport", To: "eastside", Why: events.IdleTill, Products: []string{"coke"}, Till: 50_000, Dirty: 59_713},
			nil, []string{"The Channel idle: only $9,713 over the $50,000 till, short of a lot"}},
		{"a route with an empty stash", events.RouteIdle{Route: "interstate", Name: "Interstate", From: "bayport", To: "eastside", Why: events.IdleStock, Products: []string{"weed", "coke"}},
			nil, []string{"Interstate idle: no Weed or Coke in the Bayport stash"}},
		{"a route short of three", events.RouteIdle{Route: "coast", Name: "Coast Road", From: "bayport", To: "eastside", Why: events.IdleStock, Products: []string{"weed", "coke", "meth"}},
			nil, []string{"Coast Road idle: nothing it is short of in the Bayport stash"}},
		{"a contract after a wash", events.SupplyShort{City: "bayport", Product: "pills", Units: 3, Short: 140, Why: "cash"},
			[]game.CashFlow{game.NewCashFlow(20, map[string]game.Pools{game.FlowLaundering: {Dirty: -120_000, Clean: 120_000}}, game.Pools{Dirty: 50_000})},
			[]string{"Supply contract out of cash: 140 Pills short in Bayport.", "  the wash took $120,000 last night and left the till $50,000"}},
		{"a contract with no wash", events.SupplyShort{City: "bayport", Product: "coke", Units: 3, Short: 40, Why: "cash"},
			nil, []string{"Supply contract out of cash: 40 Coke short in Bayport."}},
	} {
		n, err := news.New(cfg)
		if err != nil {
			t.Fatal(err)
		}
		w := sim.NewWorld(cfg, 4)
		w.Day = 20
		w.Flows = tc.flows
		n.Step(w, gametest.TickOn(w, 21, tc.ev))
		got := append(append([]string(nil), w.Report.Sales...), w.Report.Shipments...)
		for _, want := range tc.want {
			found := false
			for _, l := range got {
				found = found || l == want
			}
			if !found {
				t.Errorf("%s: no line %q in %q", tc.name, want, got)
			}
			if n := utf8.RuneCountInString("  " + want); n > reportW {
				t.Errorf("%s: %q is %d wide, over the report's %d at 80 columns", tc.name, want, n, reportW)
			}
		}
		if strings.Contains(strings.Join(got, "\n"), "over the float") {
			t.Errorf("%s: the contract still says the float: %q", tc.name, got)
		}
	}
}
