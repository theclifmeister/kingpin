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

// The report's advice reads the setting as it stands (#537): the first
// seizure on a route already at slow does not say to turn the dial
// down, a standing order kept at the whole stash or raised since is not
// told to be raised, and a tribute a faction offers reads as a demand
// on you, apart from the homage it offers to pay.
func TestAdviceFitsTheSetting(t *testing.T) {
	cfg := content.MustLoad()
	report := func(setup func(w *game.World), ev events.Event) string {
		t.Helper()
		n, err := news.New(cfg)
		if err != nil {
			t.Fatal(err)
		}
		w := sim.NewWorld(cfg, 4)
		w.Day = 20
		setup(w)
		n.Step(w, gametest.TickOn(w, 21, ev))
		return strings.Join(append(append(append([]string(nil), w.Report.Sales...), w.Report.Shipments...), w.Report.Territory...), "\n")
	}
	first := func(w *game.World) { w.Stats.Seizures = 1 }
	seized := func(d events.Ship) events.Event {
		return events.ShipmentSeized{Route: "coast", Product: "coke", Units: 20, Mode: "car", From: "bayport", To: "eastside", Dial: d}
	}
	if got := report(first, seized(events.ShipSlow)); strings.Contains(got, "turned down") || !strings.Contains(got, "already on slow") {
		t.Errorf("a seizure at slow:\n%s", got)
	}
	if got := report(first, seized(events.ShipNormal)); !strings.Contains(got, "turned down") {
		t.Errorf("a seizure at normal:\n%s", got)
	}

	home := "eastside"
	sold := events.PlayerSold{City: home, Product: "weed", Sold: 5, Wanted: 5, Standing: true}
	stand := func(qty int, all bool) func(w *game.World) {
		return func(w *game.World) {
			w.SetStock(home, "weed", 40)
			q := qty
			if all {
				q = game.AllUnits
			}
			if err := w.PlaceStanding(home, "weed", q, events.DialNormal); err != nil {
				t.Fatal(err)
			}
		}
	}
	if got := report(stand(5, false), sold); !strings.Contains(got, "raise it") {
		t.Errorf("a standing order at 5 that sold out with 40 left is not told to rise:\n%s", got)
	}
	for name, setup := range map[string]func(*game.World){"kept at all": stand(0, true), "raised since": stand(30, false)} {
		if got := report(setup, sold); strings.Contains(got, "raise it") {
			t.Errorf("a standing order %s is told to rise:\n%s", name, got)
		}
	}

	offer := func(kind string) events.Event {
		d := game.Deal{Kind: kind, Terms: game.Terms{PerDay: 3000}}
		return events.DealOffered{Rival: "Reverend", Deal: kind, Terms: d.String(), Expires: 23}
	}
	if got := report(func(*game.World) {}, offer(game.DealTribute)); !strings.Contains(got, "Reverend demands tribute: you pay them $3,000 a day.") {
		t.Errorf("a tribute offered:\n%s", got)
	}
	if got := report(func(*game.World) {}, offer(game.DealHomage)); !strings.Contains(got, "Reverend offers homage: they pay you $3,000 a day.") {
		t.Errorf("a homage offered:\n%s", got)
	}
}
