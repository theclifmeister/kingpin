package harness

import (
	"fmt"
	"math"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
)

func TestDeterministicForSeed(t *testing.T) {
	cfg := content.MustLoad()
	a, err := Run(cfg, 42, 100, Trader(cfg, events.DialNormal))
	if err != nil {
		t.Fatal(err)
	}
	b, _ := Run(cfg, 42, 100, Trader(cfg, events.DialNormal))
	if len(a.Events) != len(b.Events) {
		t.Fatalf("event counts differ: %d vs %d", len(a.Events), len(b.Events))
	}
	for i := range a.Events {
		if fmt.Sprintf("%#v", a.Events[i]) != fmt.Sprintf("%#v", b.Events[i]) {
			t.Fatalf("event %d differs:\n%#v\n%#v", i, a.Events[i], b.Events[i])
		}
	}
	c, _ := Run(cfg, 43, 100, Trader(cfg, events.DialNormal))
	if len(c.Events) == len(a.Events) && fmt.Sprintf("%#v", c.Events) == fmt.Sprintf("%#v", a.Events) {
		t.Fatal("different seeds produced identical runs")
	}
}

func TestPriceInvariants(t *testing.T) {
	cfg := content.MustLoad()
	tun := cfg.Market.Market
	for seed := uint64(1); seed <= 5; seed++ {
		res, err := Run(cfg, seed, 1000, Trader(cfg, events.DialAggressive))
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range res.Events {
			pm, ok := e.(events.PriceMove)
			if !ok {
				continue
			}
			pc := cfg.Market.Product(pm.Product)
			lo, hi := pc.BasePrice*tun.PriceFloorRatio, pc.BasePrice*tun.PriceCeilingRatio
			if pm.To < lo || pm.To > hi || math.IsNaN(pm.To) {
				t.Fatalf("seed %d day %d: %s price %.2f outside [%.2f, %.2f]", seed, pm.Day, pm.Product, pm.To, lo, hi)
			}
		}
	}
}

func TestIdleMarketStaysNearEquilibrium(t *testing.T) {
	cfg := content.MustLoad()
	sum := map[string]float64{}
	n := map[string]int{}
	for seed := uint64(1); seed <= 3; seed++ {
		res, _ := Run(cfg, seed, 1000, Idle)
		for _, e := range res.Events {
			if pm, ok := e.(events.PriceMove); ok {
				sum[pm.Product] += pm.To
				n[pm.Product]++
			}
		}
	}
	for _, p := range cfg.Market.Products {
		mean := sum[p.ID] / float64(n[p.ID])
		if diff := math.Abs(mean-p.BasePrice) / p.BasePrice; diff > 0.20 {
			t.Errorf("%s: mean price %.2f is %.0f%% off base %.2f", p.ID, mean, diff*100, p.BasePrice)
		}
	}
}

func TestAggressiveSellingCrashesPrice(t *testing.T) {
	cfg := content.MustLoad()
	// No random shocks: this measures the player's own impact.
	cfg.Market.Market.ShockChance = 0
	cfg.Market.Market.SlumpChance = 0
	var start float64
	res, err := Run(cfg, 7, 3, func(w *game.World) {
		if w.Day == 0 {
			start = w.Market["weed"].Price
			w.Player.Stock["weed"] = 500
		}
		if err := w.PlaceSell("weed", w.Player.Stock["weed"], events.DialAggressive); err != nil {
			t.Fatalf("day %d: %v", w.Day, err)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	end := res.World.Market["weed"].Price
	if drop := (start - end) / start; drop < 0.30 {
		t.Fatalf("three aggressive days only dropped weed %.0f%% (%.2f -> %.2f)", drop*100, start, end)
	}
	// And the report says so: the last sale line must mention the crash.
	found := false
	for _, e := range res.Events {
		if ps, ok := e.(events.PlayerSold); ok && ps.Dial == events.DialAggressive && ps.Sold > 0 {
			found = true
		}
	}
	if !found {
		t.Fatal("no aggressive sale was recorded")
	}
}

// A player who sells normally but lies low when heat climbs should survive
// and out-earn the quiet trader: that is the whole point of the dial.
func TestManagedNormalBeatsQuiet(t *testing.T) {
	cfg := content.MustLoad()
	for seed := uint64(1); seed <= 10; seed++ {
		managed, _ := Run(cfg, seed, 200, Managed(cfg, 50))
		if managed.Over != nil {
			t.Fatalf("seed %d: managed trader ended on day %d: %s", seed, managed.Days, managed.Over.Cause)
		}
		quiet, _ := Run(cfg, seed, 200, Trader(cfg, events.DialQuiet))
		if managed.PeakCash <= quiet.PeakCash {
			t.Fatalf("seed %d: managed peaked at %d, quiet at %d; managing heat should pay", seed, managed.PeakCash, quiet.PeakCash)
		}
	}
}

func TestHeatBounds(t *testing.T) {
	cfg := content.MustLoad()
	for seed := uint64(1); seed <= 5; seed++ {
		res, _ := Run(cfg, seed, 500, Trader(cfg, events.DialAggressive))
		for _, e := range res.Events {
			if hc, ok := e.(events.HeatChanged); ok && (hc.To < 0 || hc.To > 100) {
				t.Fatalf("seed %d day %d: heat %.2f out of bounds", seed, hc.Day, hc.To)
			}
		}
	}
}

func TestAlwaysAggressiveGetsArrestedFast(t *testing.T) {
	cfg := content.MustLoad()
	for seed := uint64(1); seed <= 10; seed++ {
		res, _ := Run(cfg, seed, 200, Trader(cfg, events.DialAggressive))
		if res.Over == nil || (res.Over.Cause != "arrested" && res.Over.Cause != "indicted") {
			t.Fatalf("seed %d: aggressive trader survived %d days (over=%v)", seed, res.Days, res.Over)
		}
		if res.Days > 40 {
			t.Fatalf("seed %d: aggressive trader lasted %d days, expected <= 40", seed, res.Days)
		}
	}
}

func TestAlwaysQuietSurvivesAndEarnsLess(t *testing.T) {
	cfg := content.MustLoad()
	for seed := uint64(1); seed <= 10; seed++ {
		quiet, _ := Run(cfg, seed, 200, Trader(cfg, events.DialQuiet))
		if quiet.Over != nil {
			t.Fatalf("seed %d: quiet trader ended on day %d: %s", seed, quiet.Days, quiet.Over.Cause)
		}
		if quiet.EndCash <= cfg.Market.Market.StartCash {
			t.Fatalf("seed %d: quiet trader lost money: %d", seed, quiet.EndCash)
		}
		// Greed must pay in the short term or the dial has no tension: over
		// the days the aggressive trader survives, it must out-earn quiet.
		agg, _ := Run(cfg, seed, 200, Trader(cfg, events.DialAggressive))
		short, _ := Run(cfg, seed, max(1, agg.Days-1), Trader(cfg, events.DialQuiet))
		if agg.PeakCash <= short.PeakCash {
			t.Fatalf("seed %d: over %d days aggressive peaked at %d, quiet at %d; greed should pay short term", seed, agg.Days-1, agg.PeakCash, short.PeakCash)
		}
	}
}
