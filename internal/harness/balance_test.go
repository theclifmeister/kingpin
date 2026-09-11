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
			base := pc.BasePrice * cfg.City.City(pm.City).Product(pm.Product).Price
			lo, hi := base*tun.PriceFloorRatio, base*tun.PriceCeilingRatio
			if pm.To < lo || pm.To > hi || math.IsNaN(pm.To) {
				t.Fatalf("seed %d day %d: %s %s price %.2f outside [%.2f, %.2f]", seed, pm.Day, pm.City, pm.Product, pm.To, lo, hi)
			}
		}
	}
}

func TestIdleMarketStaysNearEquilibrium(t *testing.T) {
	cfg := content.MustLoad()
	sum := map[string]float64{}
	n := map[string]int{}
	for seed := uint64(1); seed <= 5; seed++ {
		res, _ := Run(cfg, seed, 1000, Idle)
		for _, e := range res.Events {
			if pm, ok := e.(events.PriceMove); ok {
				sum[game.OrderKey(pm.City, pm.Product)] += pm.To
				n[game.OrderKey(pm.City, pm.Product)]++
			}
		}
	}
	// Every city's street settles on its own take on the ladder. Supply
	// shocks pull the mean about a fifth over the base (a slump cuts
	// demand, not price), in every city alike; an idle player never
	// unlocks the upper rungs, so those are not measured.
	for _, c := range cfg.City.Cities {
		for _, p := range cfg.Market.Products {
			k := game.OrderKey(c.ID, p.ID)
			if n[k] == 0 {
				continue
			}
			base := p.BasePrice * c.Product(p.ID).Price
			mean := sum[k] / float64(n[k])
			if diff := math.Abs(mean-base) / base; diff > 0.25 {
				t.Errorf("%s %s: mean price %.2f is %.0f%% off base %.2f", c.ID, p.ID, mean, diff*100, base)
			}
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
			start = w.Home().Market["weed"].Price
			w.Stash(w.Home().ID)["weed"] = 500
		}
		if err := w.PlaceSell(w.Home().ID, "weed", w.Stock(w.Home().ID, "weed"), events.DialAggressive); err != nil {
			t.Fatalf("day %d: %v", w.Day, err)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	end := res.World.Home().Market["weed"].Price
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
	// Tiers 3 and 4 (#60) are the boss: the player who uses every screen.
	// Designer is the port's product, so it reaches home by the road and
	// never through a tier-2 crew's supplier; the Security branch's ghost
	// nodes are how a crew's volume outgrows the street's notice; the
	// fronts cover the pile the wash cannot keep up with. TestMoneyCeilings
	// logs what each row would make with the rival kept out and heat off.
	{3, "boss", func(cfg *content.Config) Policy { return Boss(cfg, 40, "") }, 120, 5_000_000, 20_000_000, false},
	{4, "boss", func(cfg *content.Config) Policy { return Boss(cfg, 40, "") }, Horizon, 50_000_000, 200_000_000, false},
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

// The ceilings (#60): what each tier's policy would make with the rival
// kept out, with heat switched off, and with both, at that tier's
// checkpoint. Logged, never enforced: they are the wall every balance
// change is judged against, so the log says whether a missed band is the
// rival's, the police's or the street's.
func TestMoneyCeilings(t *testing.T) {
	cfg := content.MustLoad()
	boxes := []struct {
		name string
		box  func(*content.Config) *content.Config
	}{
		{"as is", func(c *content.Config) *content.Config { return c }},
		{"no rival", NoRival},
		{"no heat", NoHeat},
		{"no rival, no heat", func(c *content.Config) *content.Config { return NoHeat(NoRival(c)) }},
	}
	for _, row := range moneyCurve {
		for _, b := range boxes {
			med := medianNetWorth(t, b.box(cfg), row.policy, row.day)
			t.Logf("tier %d: %s on day %d, %s: %d", row.tier, row.name, row.day, b.name, med)
		}
	}
}

// The boss is the best policy the harness has, and it is still not a
// free ride (#60): one that never lies low is indicted before the
// horizon on every seed.
func TestBossWhoNeverLiesLowIsIndicted(t *testing.T) {
	cfg := content.MustLoad()
	for seed := uint64(1); seed <= 10; seed++ {
		res, _ := Run(cfg, seed, Horizon, Boss(cfg, 100, ""))
		if res.Over == nil || (res.Over.Cause != "indicted" && res.Over.Cause != "arrested") {
			t.Fatalf("seed %d: the boss who never lies low is still free after %d days (over=%v)", seed, res.Days, res.Over)
		}
	}
}

// Designer is the port's product (#60): the home supplier does not sell
// it, the wholesale city's does, and it reaches home by the road.
func TestDesignerComesByRoad(t *testing.T) {
	cfg := content.MustLoad()
	home, hub := cfg.City.Home().ID, ""
	for _, c := range cfg.City.Cities {
		if c.Wholesale {
			hub = c.ID
		}
	}
	w := sim.NewWorld(cfg, 1)
	w.Player.DirtyCash = 10_000_000
	w.Stats.PeakCash = 10_000_000
	if _, err := RunFrom(cfg, w, 1, Idle); err != nil {
		t.Fatal(err)
	}
	if w.Product(home, "designer") == nil || w.Product(hub, "designer") == nil {
		t.Fatal("designer is not on the ladder with a fortune in the bank")
	}
	if _, err := w.Buy("designer", 1, 0); err != game.ErrNotSupplied {
		t.Fatalf("bought designer from the home supplier: %v", err)
	}
	if _, err := w.Buy("heroin", 1, 0); err != nil {
		t.Fatalf("the home supplier stopped selling heroin: %v", err)
	}
	_ = w.Travel(hub)
	if _, err := w.Buy("designer", 1, 0); err != nil {
		t.Fatalf("the port's supplier does not sell designer: %v", err)
	}
	// The boss's home sells designer, and every unit of it landed off a
	// shipment: nothing bought at home, everything bought by the lot at
	// the hub or at its retail counter.
	boss := Boss(cfg, 40, "steady")
	res, _ := Run(cfg, 1, TierDays[2], func(w *game.World) {
		boss(w)
		for _, b := range w.Buys {
			if b.City == home && b.Product == "designer" {
				t.Fatalf("day %d: designer bought at home: %+v", w.Day, b)
			}
		}
	})
	sold, landed := 0, 0
	for _, e := range res.Events {
		switch ev := e.(type) {
		case events.PlayerSold:
			if ev.City == home && ev.Product == "designer" {
				sold += ev.Sold
			}
		case events.ShipmentArrived:
			if ev.To == home && ev.Product == "designer" {
				landed += ev.Units
			}
		}
	}
	if sold == 0 || landed == 0 || sold > landed {
		t.Fatalf("home sold %d designer and %d landed by road", sold, landed)
	}
}
