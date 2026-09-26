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

// The table's quiet moves are report lines (#506): a proposal nobody
// answered (no headline: it never reached the dice), a truce's last
// night ahead, a hit on the scouts that found nobody, a hit that
// landed, a leader succeeded and a cell gone its own way under a new
// name.
func TestTheTableSaysWhatItDid(t *testing.T) {
	cfg := content.MustLoad()
	n, err := news.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	w := sim.NewWorld(cfg, 4)
	w.Day = 4
	home, parent := w.Home().ID, w.Rival()
	tick := gametest.TickOn(w, 5,
		events.DealRefused{Day: 5, Rival: "Preacher", Faction: "f2", Deal: game.DealTruce, Terms: "a 15-day truce", Why: "their crew is finished: nobody was left to answer"},
		events.DealEnding{Day: 5, Rival: "Carmine", Faction: "f3", Deal: game.DealTruce, Until: 7},
		events.ScoutsMissed{Day: 5, Rival: "Mona", Faction: "f4", City: home, Why: "their scouts had gone home"},
		events.ScoutsHit{Day: 5, City: home, Rival: "Vito", Faction: "f5", Setback: 5, Arrive: 30},
		events.RivalLeaderArrested{Day: 5, Rival: "Deacon", Faction: "f6", City: home, Killed: true, Successor: "Mother"},
		events.RivalScouting{Day: 5, City: home, Rival: "Lark", Faction: "f7", Cell: parent.Faction(), Recruit: 10, Arrive: 20},
	)
	n.Step(w, tick)
	got := strings.Join(w.Report.Territory, "\n")
	for _, want := range []string{
		"Your proposal of a 15-day truce to Preacher went unanswered: their crew is finished",
		"The truce with Carmine holds one more night: from tomorrow they are free to push your corners.",
		"Your enforcers went after Mona's scouts and found nobody: their scouts had gone home.",
		"Your enforcers ran Vito's scouts out of",
		"Deacon is DEAD. Mother runs their crew now",
		parent.Leader + "'s crew split: a cell of it goes its own way under Lark.",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the report lacks %q:\n%s", want, got)
		}
	}
	for _, e := range tick.Events() {
		if h, ok := e.(events.Headline); ok && strings.Contains(h.Text, "Preacher") {
			t.Errorf("an unanswered proposal made the paper: %+v", h)
		}
	}
}

// A lieutenant's night says what they kept back and whom they took
// (#497): the stock a route or a contract earmarked, and the idle crew
// put to work in their city without an order.
func TestLieutenantSaysWhatTheyKeptAndWhomTheyTook(t *testing.T) {
	cfg := content.MustLoad()
	n, err := news.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	w := sim.NewWorld(cfg, 4)
	w.Day = 4
	product := w.Products[0]
	n.Step(w, gametest.TickOn(w, 5, events.LieutenantActed{Day: 5, ID: 9, Name: "Flaco", City: "bayport", CityName: "Bayport",
		Held: []events.Held{{Product: product, Units: 400, For: "Coast Road"}},
		Took: []events.Took{{Name: "Gato", Role: game.RoleRunner, Corner: "Wharf"}, {Name: "Nelly", Role: game.RoleEnforcer, Corner: "Wharf"}},
	}))
	got := strings.Join(w.Report.Crew, "\n")
	for _, want := range []string{
		"Flaco kept back 400 " + w.ProductName(product) + " for Coast Road: not theirs to sell.",
		"Flaco put idle crew to work in Bayport: Gato on Wharf, Nelly guarding Wharf. Nobody ordered it",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the report lacks %q:\n%s", want, got)
		}
	}
}
