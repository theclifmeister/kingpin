package news_test

import (
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/gametest"
	"github.com/theclifmeister/kingpin/internal/sim"
	"github.com/theclifmeister/kingpin/internal/sim/news"
)

// TestDebtIsItsOwnMoneyLine (#518): a debt paid on its day, or in part
// late, is booked on the flow's Debt paid line, never in Purchases (a
// playtest read "Purchases -$6,546" as a night's buys, $1,608 of it a
// debt), and like the purchases it is not the night's loss.
func TestDebtIsItsOwnMoneyLine(t *testing.T) {
	cfg := content.MustLoad()
	n, err := news.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	w := sim.NewWorld(cfg, 4)
	w.Day = 4
	home := w.Home().ID
	n.Step(w, gametest.TickOn(w, 5,
		events.DebtPaid{Day: 5, City: home, Name: "Cass", Amount: 900, Clean: 100},
		events.DebtLate{Day: 5, City: home, Name: "Ricky", Owed: 2_000, Paid: 500, Left: 1_500, Due: 10, What: "extended"},
	))
	f := w.Report.Flow
	if got := f.Line(game.FlowDebt); got != (game.Pools{Dirty: -1_300, Clean: -100}) {
		t.Fatalf("the debt line %+v, want -1,300 dirty and -100 clean", got)
	}
	if got := f.Line(game.FlowPurchases); got != (game.Pools{}) {
		t.Fatalf("the debts went in the purchases: %+v", got)
	}
	if f.Profit() != f.Net()+1_400 || game.FlowLabel(game.FlowDebt) != "Debt paid" {
		t.Fatalf("the profit %d on a net %d, the label %q", f.Profit(), f.Net(), game.FlowLabel(game.FlowDebt))
	}
}
