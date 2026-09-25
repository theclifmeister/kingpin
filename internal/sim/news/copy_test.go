package news_test

import (
	"strings"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/gametest"
	"github.com/theclifmeister/kingpin/internal/sim"
	"github.com/theclifmeister/kingpin/internal/sim/news"
)

// The report's copy nits (#473): a buyer is a phrase ("a foreman at the
// docks"), so its order and its offer are never a possessive; and a
// role the character started with (the Cook's chemist) opens as more
// of them, not as news.
func TestReportLinesReadAsSentences(t *testing.T) {
	cfg := content.MustLoad()
	n, err := news.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	w := sim.NewWorld(cfg, 4)
	w.Day = 4
	home, product := w.Home().ID, w.Products[0]
	n.Step(w, gametest.TickOn(w, 5,
		events.ContractAccepted{Day: 5, Name: "a foreman at the docks", City: home, Product: product, Units: 30, Due: 12},
		events.ContractExpired{Day: 5, Name: "a foreman at the docks", City: home, Product: product, Units: 20},
		events.Unlocked{Day: 5, Gate: "role", ID: game.RoleChemist, Name: "Chemists", Why: "Meth on the ladder"},
	))
	got := strings.Join(append(append([]string{}, w.Report.Sales...), w.Report.Unlocked...), "\n")
	for _, want := range []string{"You took the order from a foreman at the docks: 30", "The offer from a foreman at the docks lapsed: 20", "Chemists want work on the crew screen (4): Meth on the ladder."} {
		if !strings.Contains(got, want) {
			t.Errorf("the report lacks %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "docks's") {
		t.Errorf("a phrase took a possessive:\n%s", got)
	}

	w.Crew.Members = append(w.Crew.Members, game.CrewMember{ID: 99, Name: "Walt", Role: game.RoleChemist, Loyalty: 70})
	w.Day = 5
	n.Step(w, gametest.TickOn(w, 6, events.Unlocked{Day: 6, Gate: "role", ID: game.RoleChemist, Name: "Chemists", Why: "Meth on the ladder"}))
	if got := strings.Join(w.Report.Unlocked, "\n"); !strings.Contains(got, "More chemists want work on the crew screen (4)") {
		t.Errorf("with a chemist on the payroll the line reads:\n%s", got)
	}
}
