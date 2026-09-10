package harness

import (
	"fmt"
	"sort"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/sim"
)

// buying wraps a policy so it buys the given upgrades on fixed days, cash
// found for them, the way a scripted playthrough would.
func buying(cfg *content.Config, p Policy, on map[int]string) Policy {
	return func(w *game.World) {
		if id, ok := on[w.Day]; ok {
			Own(cfg, w, id)
			w.UpgradesToday = []string{id} // a real purchase, reported
		}
		p(w)
	}
}

// grant marks upgrades owned without buying them, so a test can measure
// one branch's effects without the prerequisites' (buying safehouse needs
// stash, and stash is more carry, more volume and more heat).
func grant(w *game.World, ids ...string) {
	for _, id := range ids {
		w.Upgrades[id] = true
	}
}

// A run that buys upgrades on fixed days replays identically from its seed.
func TestUpgradesAreDeterministic(t *testing.T) {
	cfg := content.MustLoad()
	on := map[int]string{3: "stash", 6: "burners", 9: "lawyer", 12: "supplier", 20: "lookouts"}
	run := func() Result {
		r, err := Run(cfg, 42, 60, buying(cfg, Managed(cfg, 50), on))
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
	a, b := run(), run()
	if len(a.Events) != len(b.Events) {
		t.Fatalf("event counts differ: %d vs %d", len(a.Events), len(b.Events))
	}
	bought := 0
	for i := range a.Events {
		if fmt.Sprintf("%#v", a.Events[i]) != fmt.Sprintf("%#v", b.Events[i]) {
			t.Fatalf("event %d differs:\n%#v\n%#v", i, a.Events[i], b.Events[i])
		}
		if _, ok := a.Events[i].(events.UpgradeBought); ok {
			bought++
		}
	}
	if bought != len(on) || len(a.World.Upgrades) != len(on) {
		t.Fatalf("%d purchases reported, %d owned, want %d", bought, len(a.World.Upgrades), len(on))
	}
	if a.World.Report == nil || len(a.World.Report.Upgrades) != 0 {
		t.Fatalf("day 60 report lists upgrades bought on other days: %v", a.World.Report.Upgrades)
	}
}

// Each effect pulls the right way: burners mean less heat for the same
// play, a stash is fifty more units, a supplier contact is a cheaper
// quote the next morning.
func TestUpgradeEffectsAreMonotone(t *testing.T) {
	cfg := content.MustLoad()
	for seed := uint64(1); seed <= 5; seed++ {
		plain, _ := Run(cfg, seed, 30, Trader(cfg, events.DialNormal))
		w := sim.NewWorld(cfg, seed)
		grant(w, "burners")
		burners, _ := RunFrom(cfg, w, 30, Trader(cfg, events.DialNormal))
		if burners.World.Heat.Value > plain.World.Heat.Value {
			t.Fatalf("seed %d: heat at day 30 is %.1f with burners, %.1f without", seed, burners.World.Heat.Value, plain.World.Heat.Value)
		}

		w = sim.NewWorld(cfg, seed)
		carry := w.Capacity()
		Own(cfg, w, "stash")
		if w.Capacity() != carry+50 {
			t.Fatalf("seed %d: stash took carry from %d to %d", seed, carry, w.Capacity())
		}

		w = sim.NewWorld(cfg, seed)
		before, _ := w.SupplierQuote("weed", 10)
		Own(cfg, w, "supplier")
		after, _ := w.SupplierQuote("weed", 10)
		if after >= before {
			t.Fatalf("seed %d: supplier contact quote %d -> %d on the day", seed, before, after)
		}
		day1, _ := RunFrom(cfg, w, 1, Idle)
		plain1, _ := Run(cfg, seed, 1, Idle)
		a, _ := day1.World.SupplierQuote("weed", 10)
		b, _ := plain1.World.SupplierQuote("weed", 10)
		if a >= b {
			t.Fatalf("seed %d: day-1 quote %d with a supplier contact, %d without", seed, a, b)
		}
	}
}

// Spending on the tree must pay: the upgraded player out-earns the
// managed one on median peak cash over the horizon, and is never punished
// for it. Twenty seeds, like the money curve: the rival's path diverges
// with any change in the player's (a different cash curve unlocks
// products on different days, which shifts every later RNG draw), so a
// handful of seeds can land the upgraded player next to an expansionist
// the managed one never met.
func TestUpgradedBeatsManaged(t *testing.T) {
	cfg := content.MustLoad()
	var up, man []int
	for seed := uint64(1); seed <= 20; seed++ {
		u, _ := Run(cfg, seed, Horizon, Upgraded(cfg, 50))
		if u.Over != nil {
			t.Fatalf("seed %d: upgraded trader ended on day %d: %s", seed, u.Days, u.Over.Cause)
		}
		if len(u.World.Upgrades) < 5 {
			t.Fatalf("seed %d: upgraded trader only bought %v", seed, u.World.Upgrades)
		}
		m, _ := Run(cfg, seed, Horizon, Managed(cfg, 50))
		up = append(up, u.PeakCash)
		man = append(man, m.PeakCash)
	}
	sort.Ints(up)
	sort.Ints(man)
	if up[len(up)/2] <= man[len(man)/2] {
		t.Fatalf("upgraded median peak %d, managed %d; the tree should pay for itself", up[len(up)/2], man[len(man)/2])
	}
}

// The Security branch softens the curve without removing it: an
// always-aggressive trader who owns all of it from day 1 lasts materially
// longer than one who owns none of it, and is still indicted.
func TestSecurityBranchSoftensAggressive(t *testing.T) {
	cfg := content.MustLoad()
	var plainDays, secDays []int
	for seed := uint64(1); seed <= 10; seed++ {
		plain, _ := Run(cfg, seed, Horizon, Trader(cfg, events.DialAggressive))
		w := sim.NewWorld(cfg, seed)
		for _, n := range cfg.Upgrades.Branch("security") {
			grant(w, n.ID)
		}
		sec, _ := RunFrom(cfg, w, Horizon, Trader(cfg, events.DialAggressive))
		if sec.Over == nil || (sec.Over.Cause != "indicted" && sec.Over.Cause != "arrested") {
			t.Fatalf("seed %d: aggressive trader with the Security branch still free after %d days (over=%v)", seed, sec.Days, sec.Over)
		}
		if sec.Days <= plain.Days {
			t.Fatalf("seed %d: with Security %d days, without %d; the branch should buy time", seed, sec.Days, plain.Days)
		}
		plainDays = append(plainDays, plain.Days)
		secDays = append(secDays, sec.Days)
	}
	sort.Ints(plainDays)
	sort.Ints(secDays)
	p, s := plainDays[len(plainDays)/2], secDays[len(secDays)/2]
	if float64(s) < 1.4*float64(p) {
		t.Fatalf("median days: %d with Security, %d without; want at least 1.4x", s, p)
	}
}

// The fall guy takes exactly one indictment, and only when owned: the
// aggressive trader who has one survives his first case, keeps trading,
// and is indicted for real later.
func TestFallGuyFiresOnce(t *testing.T) {
	cfg := content.MustLoad()
	for seed := uint64(1); seed <= 5; seed++ {
		plain, _ := Run(cfg, seed, Horizon, Trader(cfg, events.DialAggressive))
		for _, e := range plain.Events {
			if _, ok := e.(events.FallGuyBurned); ok {
				t.Fatalf("seed %d: the fall guy fired for a player who never bought one", seed)
			}
		}
		w := sim.NewWorld(cfg, seed)
		grant(w, "fallguy")
		res, _ := RunFrom(cfg, w, Horizon, Trader(cfg, events.DialAggressive))
		burned := 0
		var burnDay int
		for _, e := range res.Events {
			if ev, ok := e.(events.FallGuyBurned); ok {
				burned++
				burnDay = ev.Day
				if ev.CashLost <= 0 {
					t.Fatalf("seed %d: the fall guy cost nothing", seed)
				}
			}
		}
		if burned != 1 || !res.World.FallGuyUsed {
			t.Fatalf("seed %d: fall guy fired %d times (used=%v)", seed, burned, res.World.FallGuyUsed)
		}
		if res.Over == nil || res.Days <= burnDay || res.Days <= plain.Days {
			t.Fatalf("seed %d: with a fall guy burned on day %d the run went %d days (over=%v), plain %d", seed, burnDay, res.Days, res.Over, plain.Days)
		}
	}
}

// A retained lawyer lets the file go cold: a quiet player's evidence
// drops a point every evidence_decay_days, to zero; without the retainer
// it never moves. Neither touches #27: the rich hider is stung and never
// charged either way.
func TestRetainerLetsTheCaseGoCold(t *testing.T) {
	cfg := content.MustLoad()
	days := cfg.Upgrades.Upgrade("retainer").Effects.EvidenceDecayDays
	for seed := uint64(1); seed <= 3; seed++ {
		w := sim.NewWorld(cfg, seed)
		w.Heat.Evidence = 3
		w.Heat.EvidenceDay = 0
		w.Player.DirtyCash = 5_000_000
		grant(w, "retainer")
		res, _ := RunFrom(cfg, w, 3*days, Hide)
		if res.Over != nil || res.World.Heat.Evidence != 0 {
			t.Fatalf("seed %d: retained hider ended %v with %d evidence after %d days", seed, res.Over, res.World.Heat.Evidence, 3*days)
		}
		half, _ := RunFrom(cfg, func() *game.World {
			w := sim.NewWorld(cfg, seed)
			w.Heat.Evidence = 3
			grant(w, "retainer")
			return w
		}(), days+days/2, Hide)
		if half.World.Heat.Evidence != 2 {
			t.Fatalf("seed %d: %d evidence after %d days, want one point gone", seed, half.World.Heat.Evidence, days+days/2)
		}
		w = sim.NewWorld(cfg, seed)
		w.Heat.Evidence = 3
		w.Player.DirtyCash = 5_000_000
		res, _ = RunFrom(cfg, w, 3*days, Hide)
		if res.Over != nil || res.World.Heat.Evidence != 3 {
			t.Fatalf("seed %d: without a retainer the hider ended %v with %d evidence", seed, res.Over, res.World.Heat.Evidence)
		}
	}
}

// A lawyer on call thins the file: stings add nothing, raids one page, so
// the aggressive trader lasts longer and is still indicted.
func TestLawyerThinsTheFile(t *testing.T) {
	cfg := content.MustLoad()
	for seed := uint64(1); seed <= 5; seed++ {
		plain, _ := Run(cfg, seed, Horizon, Trader(cfg, events.DialAggressive))
		w := sim.NewWorld(cfg, seed)
		grant(w, "lawyer")
		res, _ := RunFrom(cfg, w, Horizon, Trader(cfg, events.DialAggressive))
		if res.Over == nil || res.Days <= plain.Days {
			t.Fatalf("seed %d: with a lawyer %d days (over=%v), without %d", seed, res.Days, res.Over, plain.Days)
		}
		for _, e := range res.Events {
			if ev, ok := e.(events.Enforcement); ok && ev.Level == "sting" && ev.Evidence != 0 {
				t.Fatalf("seed %d day %d: a sting added %d evidence past the lawyer", seed, ev.Day, ev.Evidence)
			}
		}
	}
}
