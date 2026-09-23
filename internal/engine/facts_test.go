package engine_test

import (
	"fmt"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/engine"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/harness"
)

// TestFastForwardIsTheDayLoop (#298): Session.FastForward is EndDay a
// day at a time weighed by Stop, nothing more. The same seed run both
// ways stops on the same day for the same reason, and after is called
// once a day with that day's events.
func TestFastForwardIsTheDayLoop(t *testing.T) {
	t.Parallel()
	cfg := content.MustLoad()
	policy := harness.Trader(cfg, events.DialNormal)
	for seed := uint64(1); seed <= 4; seed++ {
		a, err := engine.New(cfg)
		if err != nil {
			t.Fatal(err)
		}
		wa := a.NewRun(seed, game.Start{})
		policy(wa)
		days := 0
		ran, stop, last := a.FastForward(120, func(evs []events.Event) {
			days++
			if len(evs) == 0 {
				t.Errorf("seed %d: after was handed no day", seed)
			} else if _, ok := evs[len(evs)-1].(events.DayEnded); !ok {
				t.Errorf("seed %d: after was not handed a whole day", seed)
			}
		})
		if days != ran || ran < 1 {
			t.Fatalf("seed %d: ran %d days, after called %d times", seed, ran, days)
		}

		b, _ := engine.New(cfg)
		wb := b.NewRun(seed, game.Start{})
		policy(wb)
		var want engine.Stop
		var evs []events.Event
		n := 0
		for n < 120 {
			before := b.Alerts()
			evs = b.EndDay()
			n++
			if wb.Over != nil {
				want = engine.Stop{Kind: engine.StopOver}
				break
			}
			if want = b.Stop(evs, before); want.Kind != engine.StopNone {
				break
			}
		}
		if want.Kind == engine.StopNone {
			want.Kind = engine.StopCap
		}
		if n != ran || fmt.Sprintf("%#v", want) != fmt.Sprintf("%#v", stop) || len(evs) != len(last) {
			t.Fatalf("seed %d: FastForward ran %d and stopped on %#v; the day loop ran %d and stopped on %#v", seed, ran, stop, n, want)
		}
		if wa.Day != wb.Day {
			t.Fatalf("seed %d: day %d against %d", seed, wa.Day, wb.Day)
		}
	}
}

// TestStopsOnTheReadings (#298): an event whose fields say it needs
// nobody does not stop, the same kind that does stops.
func TestStopsOnTheReadings(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		e    events.Event
		want bool
	}{
		{events.Enforcement{Level: content.Patrol}, false},
		{events.Enforcement{Level: content.Raid}, true},
		{events.CornerStruck{War: true}, false},
		{events.CornerStruck{War: true, Taken: true}, true},
		{events.CornerStruck{}, true},
		{events.RivalBoosted{Taken: true}, false},
		{events.RivalBoosted{}, true},
		{events.CornerTaken{From: game.OwnerPlayer}, true},
		{events.CornerTaken{From: game.OwnerNone}, false},
		{events.CrewShot{Dead: true}, true},
		{events.CrewShot{Dead: true, Theirs: true}, false},
		{events.CrewShot{}, false},
		{events.PressureShifted{From: 1, To: 2}, true},
		{events.PressureShifted{From: 2, To: 1}, false},
		{events.ReputationShifted{From: 1, To: 2}, true},
		{events.ReputationShifted{From: 2, To: 1}, false},
		{events.PriceMove{}, false},
		{events.DayEnded{}, false},
	} {
		if got := engine.StopsOn(c.e); got != c.want {
			t.Errorf("StopsOn(%#v) = %v, want %v", c.e, got, c.want)
		}
	}
}

// TestPriceFacts (#298): the day's change, the range, the margin over
// the unit, and no unit where the city's supplier does not sell it.
func TestPriceFacts(t *testing.T) {
	t.Parallel()
	p := &game.ProductMarket{Price: 12, SupplierPrice: 8, History: []float64{9, 10, 12}}
	f := engine.Facts(p)
	if f.Unit != 8 || f.Delta != 20 || f.Lo != 9 || f.Hi != 12 || f.Margin != 50 {
		t.Fatalf("facts %+v", f)
	}
	if f := engine.FactsAt(p, 6); f.Unit != 6 || f.Margin != 100 {
		t.Fatalf("facts at 6 %+v", f)
	}
	p.NoSupply = true
	if f := engine.Facts(p); f.Unit != 0 || f.Margin != 0 {
		t.Fatalf("no supply %+v", f)
	}
}

// TestAlertsAreKeyedOnce (#298): the alerts' keys are unique on a
// morning, so a fast-forward's comparison names each once.
func TestAlertsAreKeyedOnce(t *testing.T) {
	t.Parallel()
	cfg := content.MustLoad()
	s, err := engine.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if s.Alerts() != nil {
		t.Fatal("alerts before a run")
	}
	w := s.NewRun(5, game.Start{})
	policy := harness.Trader(cfg, events.DialAggressive)
	seen := 0
	for d := 0; d < 80 && w.Over == nil; d++ {
		policy(w)
		s.EndDay()
		keys := map[string]bool{}
		for _, a := range s.Alerts() {
			if a.Key == "" || keys[a.Key] {
				t.Fatalf("day %d: alert %+v has an empty or repeated key", w.Day, a)
			}
			keys[a.Key] = true
			seen++
		}
	}
	if seen == 0 {
		t.Fatal("eighty aggressive days and not one alert")
	}
}
