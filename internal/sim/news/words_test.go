package news_test

import (
	"strings"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/gametest"
	"github.com/theclifmeister/kingpin/internal/sim"
	"github.com/theclifmeister/kingpin/internal/sim/news"
)

// The report's words match what happened (#465): a sting that took
// cash and no stock says the cash (it read `STING: lost and $1,486`),
// one that took both says both, and an order taken and handed over on
// one day reads taken first (the market hands the lot over before it
// settles yesterday's acceptances).
func TestReportWordsMatchTheNight(t *testing.T) {
	cfg := content.MustLoad()
	n, err := news.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	w := sim.NewWorld(cfg, 4)
	w.Day = 4
	home := w.Home().ID
	n.Step(w, gametest.TickOn(w, 5,
		events.Enforcement{Day: 5, City: home, Level: content.Sting, CashLost: 1_486},
	))
	heat := strings.Join(w.Report.Heat, "\n")
	if !strings.Contains(heat, "STING: lost $1,486") || strings.Contains(heat, "lost and") {
		t.Errorf("a sting that took cash alone:\n%s", heat)
	}
	w.Day = 5
	n.Step(w, gametest.TickOn(w, 6,
		events.Enforcement{Day: 6, City: home, Level: content.Sting, StockLost: map[string]int{"weed": 2}, CashLost: 1_100},
	))
	if heat := strings.Join(w.Report.Heat, "\n"); !strings.Contains(heat, "STING: lost 2 Weed and $1,100") {
		t.Errorf("a sting that took both:\n%s", heat)
	}

	w.Day = 6
	n.Step(w, gametest.TickOn(w, 7,
		events.ContractDelivered{Day: 7, ID: 3, Name: "Marco", City: home, Product: "weed", Units: 27, Owed: 3, Total: 30, Price: 20, Street: 20, Signed: 20, Revenue: 540},
		events.ContractAccepted{Day: 7, ID: 3, Name: "Marco", City: home, Product: "weed", Units: 30, Due: 12},
	))
	took, handed := -1, -1
	for i, l := range w.Report.Sales {
		switch {
		case strings.HasPrefix(l, "You took the order from Marco"):
			took = i
		case strings.HasPrefix(l, "Handed 27 Weed to Marco"):
			handed = i
		}
	}
	if took < 0 || handed < 0 || took > handed {
		t.Errorf("the order reads taken at %d and handed at %d:\n%s", took, handed, strings.Join(w.Report.Sales, "\n"))
	}
}
