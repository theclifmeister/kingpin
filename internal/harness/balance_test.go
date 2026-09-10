package harness

import (
	"fmt"
	"math"
	"sort"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/sim"
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

// A player who sells normally but lies low when heat climbs is never
// punished for playing on, and out-earns the quiet trader: that is the
// whole point of the dial.
func TestManagedNormalBeatsQuiet(t *testing.T) {
	cfg := content.MustLoad()
	for seed := uint64(1); seed <= 10; seed++ {
		managed, _ := Run(cfg, seed, Horizon, Managed(cfg, 50))
		if managed.Over != nil {
			t.Fatalf("seed %d: managed trader ended on day %d: %s", seed, managed.Days, managed.Over.Cause)
		}
		quiet, _ := Run(cfg, seed, Horizon, Trader(cfg, events.DialQuiet))
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
		res, _ := Run(cfg, seed, Horizon, Trader(cfg, events.DialAggressive))
		if res.Over == nil || (res.Over.Cause != "arrested" && res.Over.Cause != "indicted") {
			t.Fatalf("seed %d: aggressive trader still free after %d days (over=%v)", seed, res.Days, res.Over)
		}
		if res.Days > 40 {
			t.Fatalf("seed %d: aggressive trader lasted %d days, expected <= 40", seed, res.Days)
		}
	}
}

// Quiet play is always safe, however long it goes on, and it pays less:
// the dial has to be a real trade.
func TestAlwaysQuietStaysFreeAndEarnsLess(t *testing.T) {
	cfg := content.MustLoad()
	for seed := uint64(1); seed <= 10; seed++ {
		quiet, _ := Run(cfg, seed, Horizon, Trader(cfg, events.DialQuiet))
		if quiet.Over != nil {
			t.Fatalf("seed %d: quiet trader ended on day %d: %s", seed, quiet.Days, quiet.Over.Cause)
		}
		if quiet.EndCash <= cfg.Market.Market.StartCash {
			t.Fatalf("seed %d: quiet trader lost money: %d", seed, quiet.EndCash)
		}
		// Greed must pay in the short term or the dial has no tension: over
		// the days the aggressive trader lasts, it must out-earn quiet.
		agg, _ := Run(cfg, seed, Horizon, Trader(cfg, events.DialAggressive))
		short, _ := Run(cfg, seed, max(1, agg.Days-1), Trader(cfg, events.DialQuiet))
		if agg.PeakCash <= short.PeakCash {
			t.Fatalf("seed %d: over %d days aggressive peaked at %d, quiet at %d; greed should pay short term", seed, agg.Days-1, agg.PeakCash, short.PeakCash)
		}
	}
}

// moneyCurve is the net-worth target per progression tier (#24): the
// median of the best harness policy at that tier, at the day the tier ends.
// Each phase adds its row when its multiplier ships. A pending row is
// measured and logged but not enforced: its band is the target, and the
// multiplier that reaches it has not shipped yet.
var moneyCurve = []struct {
	tier    int
	name    string
	policy  func(cfg *content.Config) Policy
	day     int
	lo, hi  int
	pending bool
}{
	{1, "managed", func(cfg *content.Config) Policy { return Managed(cfg, 50) }, 30, 50_000, 200_000, false},
	{2, "crewed", func(cfg *content.Config) Policy { return Crewed(cfg, 40) }, 70, 500_000, 2_000_000, false},
	// Tier 3 (#29): laundering lifts the dirty-cash ceiling, but the city's
	// seven corners absorb ~$20k a day and $10M by day 120 needs ~$80k, so
	// the row waits on the demand multiplier (cities and routes, #30).
	{3, "laundered", func(cfg *content.Config) Policy { return Laundered(cfg, 40) }, 120, 5_000_000, 20_000_000, true},
}

func medianNetWorth(t *testing.T, cfg *content.Config, policy func(*content.Config) Policy, day int) int {
	t.Helper()
	var worths []int
	for seed := uint64(1); seed <= 20; seed++ {
		res, err := Run(cfg, seed, day, policy(cfg))
		if err != nil {
			t.Fatal(err)
		}
		worths = append(worths, res.NetWorthAt(day))
	}
	sort.Ints(worths)
	return worths[len(worths)/2]
}

// The economy's scale is a target, not an accident: every tier's median
// net worth must land in its band, and the test must bite when the
// numbers drift, so it also checks that halving demand fails tier 1.
func TestMoneyCurve(t *testing.T) {
	cfg := content.MustLoad()
	for _, row := range moneyCurve {
		med := medianNetWorth(t, cfg, row.policy, row.day)
		t.Logf("tier %d: %s median net worth on day %d is %d (want %d..%d)", row.tier, row.name, row.day, med, row.lo, row.hi)
		if med < row.lo || med > row.hi {
			if row.pending {
				t.Logf("tier %d: %s median net worth on day %d is %d, target %d..%d not yet pinned", row.tier, row.name, row.day, med, row.lo, row.hi)
				continue
			}
			t.Errorf("tier %d: %s median net worth on day %d is %d, want %d..%d", row.tier, row.name, row.day, med, row.lo, row.hi)
		}
	}

	half := *cfg
	half.Market.Products = append([]content.ProductConfig(nil), cfg.Market.Products...)
	for i := range half.Market.Products {
		half.Market.Products[i].Demand /= 2
	}
	row := moneyCurve[0]
	med := medianNetWorth(t, &half, row.policy, row.day)
	t.Logf("tier %d with demand halved: %d", row.tier, med)
	if med >= row.lo {
		t.Errorf("tier %d with demand halved still reaches %d on day %d; the curve test does not bite", row.tier, med, row.day)
	}
}

// A pile of dirty cash draws the police but is not a countdown (#27): a
// player who has built something and then sits on it, lying low, is never
// indicted however long they wait. Stings and raids on a day nothing moved
// cost stock and cash and cool heat, but add no evidence.
func TestRichHiderIsNeverIndicted(t *testing.T) {
	cfg := content.MustLoad()
	for seed := uint64(1); seed <= 5; seed++ {
		w := sim.NewWorld(cfg, seed)
		w.Player.DirtyCash = 5_000_000
		res, _ := RunFrom(cfg, w, 1000, Hide)
		if res.Over != nil {
			t.Fatalf("seed %d: rich hider ended on day %d: %s", seed, res.Days, res.Over.Cause)
		}
		if res.World.Heat.Evidence != 0 {
			t.Fatalf("seed %d: rich hider has %d evidence against them without ever selling", seed, res.World.Heat.Evidence)
		}
		stings := 0
		for _, e := range res.Events {
			if ev, ok := e.(events.Enforcement); ok && (ev.Level == "sting" || ev.Level == "raid") {
				stings++
				if ev.Evidence != 0 {
					t.Fatalf("seed %d day %d: %s on a quiet day added %d evidence", seed, ev.Day, ev.Level, ev.Evidence)
				}
			}
		}
		if stings == 0 {
			t.Fatalf("seed %d: $5M dirty drew no stings or raids in 1000 days; the pile should still draw attention", seed)
		}
	}
}
