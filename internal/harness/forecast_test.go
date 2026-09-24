package harness

import (
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/engine"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/sim"
)

// TestForecastMatchesTheNight (#397): the cartel played to day 400 on
// two seeds, the forecast read each morning after the policy, against
// what the night did. The loads due tonight pay what the forecast said
// on every night none was seized, and the wages it read are the bill
// the crew sim paid (paid and short) on every night the roster did not
// change under it. The forecast writes nothing (the digest's walk).
func TestForecastMatchesTheNight(t *testing.T) {
	t.Parallel()
	cfg := content.MustLoad()
	for _, seed := range []uint64{1, 3} {
		boxed := *cfg
		boxed.Dilemmas.Cards, boxed.Incidents.Table = nil, nil
		sess, err := engine.New(&boxed)
		if err != nil {
			t.Fatal(err)
		}
		w := sim.NewWorld(&boxed, seed)
		sess.Attach(w)
		policy := Cartel(&boxed, 40)
		landed, wagesChecked := 0, 0
		for d := 0; d < 400 && w.Over == nil; d++ {
			policy(w)
			before := digest(w)
			fc := sess.Forecast()
			if digest(w) != before {
				t.Fatalf("seed %d day %d: the forecast wrote the world", seed, w.Day)
			}
			roster := len(w.Crew.Members)
			evs := sess.EndDay()
			got, seized, paid, changed := 0, false, -1, false
			for _, e := range evs {
				switch ev := e.(type) {
				case events.ExportLanded:
					got += ev.Revenue
				case events.ExportSeized:
					seized = true
				case events.CrewPaid:
					paid = ev.Wages + ev.Short
				case events.CrewHired, events.CrewFired, events.CrewQuit, events.CrewDefected, events.CrewArrested, events.CrewShot, events.CrewRetired, events.CrewReleased, events.CrewBailed, events.CrewRecovered:
					changed = true
				}
			}
			if !seized && got != fc.Landings {
				t.Fatalf("seed %d day %d: %d loads landed for %d, the forecast said %d (%d loads)", seed, w.Day, fc.Loads, got, fc.Landings, fc.Loads)
			}
			if fc.Loads > 0 && !seized {
				landed++
			}
			if paid >= 0 && !changed && roster > 0 {
				if paid != fc.Wages {
					t.Fatalf("seed %d day %d: the crew was paid a bill of %d, the forecast said %d", seed, w.Day, paid, fc.Wages)
				}
				wagesChecked++
			}
		}
		if landed == 0 || wagesChecked == 0 {
			t.Fatalf("seed %d: %d nights of landings and %d of wages checked: the test reads nothing", seed, landed, wagesChecked)
		}
		t.Logf("seed %d: %d nights of landings and %d of wages matched", seed, landed, wagesChecked)
	}
}
