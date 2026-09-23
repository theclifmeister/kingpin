package harness

import (
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/engine"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/sim"
)

// flowNights plays every policy to Horizon on seed with the deck in play
// (every card answered with its first choice, the one that moves money)
// and the incidents dealt, and hands each night to check: the world as
// the night left it, the night's events and the piles the day opened on
// (the night before's close, before the morning's card and the day's
// actions). A night that ends the run is not checked: the loop stops
// short and the report is the night before's.
func flowNights(t *testing.T, seed uint64, check func(t *testing.T, name string, w *game.World, evs []events.Event, open game.Pools)) {
	cfg := content.MustLoad()
	for _, np := range Policies {
		t.Run(np.Name, func(t *testing.T) {
			t.Parallel()
			w := sim.NewWorld(cfg, seed)
			sess, err := engine.New(cfg)
			if err != nil {
				t.Fatal(err)
			}
			sess.Attach(w)
			policy := np.Make(cfg, DefaultPolicyOpts())
			open := game.Pools{Dirty: w.Player.DirtyCash, Clean: w.Player.CleanCash}
			for d := 0; d < Horizon && w.Over == nil; d++ {
				if w.Dilemmas.Pending != nil {
					_, _ = w.Choose(0)
				}
				policy(w)
				evs := sess.EndDay()
				if w.Over != nil {
					return
				}
				check(t, np.Name, w, evs, open)
				open = game.Pools{Dirty: w.Player.DirtyCash, Clean: w.Player.CleanCash}
			}
		})
	}
}

// TestCashFlowReconciles (#351): across every harness policy to day 200,
// the cards and the incidents in play, every night's flow opens on the
// piles the night before closed on and its lines carry it to the piles
// it closed on, dirty and clean each. The opening is worked back from
// the close, so a move the report misses (a card's cash, the assets'
// upkeep, a lieutenant's skim counted twice, an order whose faction was
// gone by night) shows as an opening that is not yesterday's close.
func TestCashFlowReconciles(t *testing.T) {
	t.Parallel()
	days := content.MustLoad().Headlines.Flow.Days
	flowNights(t, 3, func(t *testing.T, name string, w *game.World, _ []events.Event, open game.Pools) {
		f := w.Report.Flow
		if !f.Reconciles() {
			t.Fatalf("%s day %d: the flow does not reconcile: %+v + %+v != %+v", name, w.Day, f.Opening, f.Sum(), f.Closing)
		}
		if f.Opening != open {
			t.Fatalf("%s day %d: the flow opened on %+v, the night before closed on %+v; money lines %q", name, w.Day, f.Opening, open, w.Report.Money)
		}
		if f.Closing != (game.Pools{Dirty: w.Player.DirtyCash, Clean: w.Player.CleanCash}) {
			t.Fatalf("%s day %d: the flow closed on %+v, the piles are %d dirty, %d clean", name, w.Day, f.Closing, w.Player.DirtyCash, w.Player.CleanCash)
		}
		if w.Report.CashBefore != f.Opening.Total() || w.Report.CashAfter != f.Closing.Total() {
			t.Fatalf("%s day %d: CASH %d -> %d, the flow %d -> %d", name, w.Day, w.Report.CashBefore, w.Report.CashAfter, f.Opening.Total(), f.Closing.Total())
		}
		if n := len(w.Flows); n == 0 || n > days || w.Flows[n-1].Day != w.Day || w.Flows[n-1].Opening != f.Opening {
			t.Fatalf("%s day %d: the history holds %d flows, the last for day %d; want at most %d, the last tonight's", name, w.Day, n, w.Flows[max(n-1, 0)].Day, days)
		}
	})
}

// TestFlowIsTheReportersTotals (#351): the flow's lines are the night's
// events summed, read off the events themselves: the sales are the
// street's take and the buyers' less the cut the crew and the
// lieutenants kept, the losses every robbery, skim, seizure, audit and
// collection, the tax the free corners', and the wash's dirty side
// what went through the fronts.
func TestFlowIsTheReportersTotals(t *testing.T) {
	t.Parallel()
	flowNights(t, 5, func(t *testing.T, name string, w *game.World, evs []events.Event, _ game.Pools) {
		var sales, losses, tax, washed int
		for _, e := range evs {
			switch ev := e.(type) {
			case events.PlayerSold:
				sales += ev.Revenue - ev.Cut
			case events.ContractDelivered:
				sales += ev.Revenue
			case events.LieutenantActed:
				sales -= ev.Cut
			case events.CornerRobbed:
				losses -= ev.Cash
			case events.CrewSkimmed:
				losses -= ev.Amount
			case events.Enforcement:
				losses -= ev.CashLost
			case events.FallGuyBurned:
				losses -= ev.CashLost
			case events.FrontAudited:
				losses -= ev.Seized
			case events.ContractFailed:
				losses -= ev.Cash
			case events.Taxed:
				tax += ev.Amount
			case events.CashLaundered:
				washed += ev.Amount
			}
		}
		f := w.Report.Flow
		for _, c := range []struct {
			cat  string
			got  int
			want int
		}{
			{game.FlowSales, f.Line(game.FlowSales).Total(), sales},
			{game.FlowLosses, f.Line(game.FlowLosses).Total(), losses},
			{game.FlowTax, f.Line(game.FlowTax).Total(), tax},
			{"laundering, dirty", f.Line(game.FlowLaundering).Dirty, -washed},
		} {
			if c.got != c.want {
				t.Fatalf("%s day %d: %s %d in the flow, %d off the night's events", name, w.Day, c.cat, c.got, c.want)
			}
		}
		if len(f.Lines) != len(game.FlowCats) {
			t.Fatalf("%s day %d: %d lines, want one a category (%d)", name, w.Day, len(f.Lines), len(game.FlowCats))
		}
	})
}
