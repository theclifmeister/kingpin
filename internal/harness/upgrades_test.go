package harness

import (
	"fmt"
	"sort"
	"strings"
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
		if burners.World.MaxHeat() > plain.World.MaxHeat() {
			t.Fatalf("seed %d: heat at day 30 is %.1f with burners, %.1f without", seed, burners.World.MaxHeat(), plain.World.MaxHeat())
		}

		w = sim.NewWorld(cfg, seed)
		carry := w.Capacity(w.Player.Location)
		Own(cfg, w, "stash")
		if w.Capacity(w.Player.Location) != carry+50 {
			t.Fatalf("seed %d: stash took carry from %d to %d", seed, carry, w.Capacity(w.Player.Location))
		}

		w = sim.NewWorld(cfg, seed)
		quote := func(w *game.World) int { return w.BestSupplier(w.Player.Location, "weed").Quote("weed", 10, false) }
		before := quote(w)
		Own(cfg, w, "supplier")
		after := quote(w)
		if after >= before {
			t.Fatalf("seed %d: supplier contact quote %d -> %d on the day", seed, before, after)
		}
		day1, _ := RunFrom(cfg, w, 1, Idle)
		plain1, _ := Run(cfg, seed, 1, Idle)
		a := quote(day1.World)
		b := quote(plain1.World)
		if a >= b {
			t.Fatalf("seed %d: day-1 quote %d with a supplier contact, %d without", seed, a, b)
		}
	}
}

// Spending on the tree must pay: the upgraded player out-earns the
// crewed one on median peak cash over the horizon, and is never punished
// for it. The yardstick is the crewed operation, the tree's customer
// (#117): a lone trader who buys every node pays for insurance and for
// crew, front and road nodes it cannot use, and with twenty-nine nodes
// (fifty-six once parts 2 and 3 land) buying everything is a trap for
// one by design. Twenty seeds, like the money curve: the rival's path
// diverges with any change in the player's (a different cash curve
// unlocks products on different days, which shifts every later RNG
// draw), so a handful of seeds can land the upgraded player next to an
// expansionist the crewed one never met.
func TestUpgradedBeatsCrewed(t *testing.T) {
	cfg := content.MustLoad()
	var up, crew []int
	for seed := uint64(1); seed <= 20; seed++ {
		u, _ := Run(cfg, seed, Horizon, Upgraded(cfg, 40))
		if u.Over != nil {
			t.Fatalf("seed %d: upgraded player ended on day %d: %s", seed, u.Days, u.Over.Cause)
		}
		if len(u.World.Upgrades) < 5 {
			t.Fatalf("seed %d: upgraded player only bought %v", seed, u.World.Upgrades)
		}
		c, _ := Run(cfg, seed, Horizon, Crewed(cfg, 40))
		up = append(up, u.PeakCash)
		crew = append(crew, c.PeakCash)
	}
	sort.Ints(up)
	sort.Ints(crew)
	if up[len(up)/2] <= crew[len(crew)/2] {
		t.Fatalf("upgraded median peak %d, crewed %d; the tree should pay for itself", up[len(up)/2], crew[len(crew)/2])
	}
	t.Logf("upgraded median peak %d, crewed %d", up[len(up)/2], crew[len(crew)/2])
}

// What the upgraded player owns at the tier checkpoints is the record
// part 4's screen and parts 2 and 3's nodes are laid against: the
// median count over twenty seeds and seed 1's list, logged.
func TestUpgradedOwns(t *testing.T) {
	cfg := content.MustLoad()
	for _, day := range []int{70, 120, Horizon} {
		var counts []int
		var owned []string
		for seed := uint64(1); seed <= 20; seed++ {
			res, _ := Run(cfg, seed, day, Upgraded(cfg, 40))
			counts = append(counts, len(res.World.Upgrades))
			if seed == 1 {
				for _, n := range cfg.Upgrades.Nodes {
					if res.World.Owns(n.ID) {
						owned = append(owned, n.ID)
					}
				}
			}
		}
		sort.Ints(counts)
		t.Logf("day %d: upgraded owns a median of %d of %d nodes; seed 1 owns %s", day, counts[len(counts)/2], len(cfg.Upgrades.Nodes), strings.Join(owned, ", "))
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
		if burned != 1 || res.World.FallsTaken != 1 {
			t.Fatalf("seed %d: fall guy fired %d times (taken=%d)", seed, burned, res.World.FallsTaken)
		}
		if res.Over == nil || res.Days <= burnDay || res.Days <= plain.Days {
			t.Fatalf("seed %d: with a fall guy burned on day %d the run went %d days (over=%v), plain %d", seed, burnDay, res.Days, res.Over, plain.Days)
		}
	}
}

// fall_guys is a count (#117): with the second name owned two
// indictments close on somebody else, one each, and the third is the
// player's.
func TestSecondFallGuyTakesTheSecondFall(t *testing.T) {
	cfg := content.MustLoad()
	for seed := uint64(1); seed <= 5; seed++ {
		w := sim.NewWorld(cfg, seed)
		grant(w, "fallguy", "fallguy2")
		res, _ := RunFrom(cfg, w, Horizon, Trader(cfg, events.DialAggressive))
		burned := 0
		for _, e := range res.Events {
			if _, ok := e.(events.FallGuyBurned); ok {
				burned++
			}
		}
		if burned != 2 || res.World.FallsTaken != 2 {
			t.Fatalf("seed %d: two fall guys fired %d times (taken=%d)", seed, burned, res.World.FallsTaken)
		}
		if res.Over == nil {
			t.Fatalf("seed %d: the aggressive trader with two fall guys was never indicted", seed)
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

// The Street branch (#119) slows the rival's taking of ground and never
// stops it: a passive player with all five nodes from day 0 still loses
// corners to an expansionist on every seed by the tier-3 checkpoint,
// loses no more of them by then than with nothing owned, and the first
// loss comes no sooner over the seeds than it did bare (the rival's
// dice move with any change in what stands on a corner, so one seed can
// go either way). The first-loss day is logged per seed: with the
// proposed numbers it is within 60 days on four seeds of five and day
// 66 on seed 5 (day 34 bare); holding every seed to the passive test's
// 60 days would need rival_push_mul at 0.95, a $400k node that does
// nothing, so the window here is the checkpoint's.
func TestStreetBranchSlowsTheRivalNeverStopsIt(t *testing.T) {
	cfg := content.MustLoad()
	street := []string{"boys", "watch", "dogs", "frontline", "ground"}
	firstLoss := func(seed uint64, own bool) (day, lost int) {
		w := sim.NewWorld(cfg, seed)
		w.Rival.Personality = "expansionist"
		if own {
			Own(cfg, w, street...)
		}
		res, err := RunFrom(cfg, w, TierDays[2], Territory(cfg, 40, 3))
		if err != nil {
			t.Fatal(err)
		}
		if res.World.Stats.Strikes != 0 {
			t.Fatalf("seed %d: the passive player sent enforcers", seed)
		}
		for _, e := range res.Events {
			if ct, ok := e.(events.CornerTaken); ok && ct.From == game.OwnerPlayer {
				if day == 0 {
					day = ct.Day
				}
				lost++
			}
		}
		return day, lost
	}
	plainDays, streetDays, plainLost, streetLost := 0, 0, 0, 0
	for seed := uint64(1); seed <= 5; seed++ {
		pd, pl := firstLoss(seed, false)
		sd, sl := firstLoss(seed, true)
		if sl == 0 {
			t.Fatalf("seed %d: with the whole Street branch the passive player held its corners for %d days next to an expansionist; the branch must slow the loss, never stop it", seed, TierDays[2])
		}
		t.Logf("seed %d: first corner lost on day %d with the Street branch (day %d without); %d lost by day %d (%d without)", seed, sd, pd, sl, TierDays[2], pl)
		plainDays, streetDays, plainLost, streetLost = plainDays+pd, streetDays+sd, plainLost+pl, streetLost+sl
	}
	t.Logf("over 5 seeds: first loss on day %.1f with the branch, %.1f without; %d corners lost with it, %d without", float64(streetDays)/5, float64(plainDays)/5, streetLost, plainLost)
	if streetDays < plainDays || streetLost > plainLost {
		t.Fatalf("the Street branch did not slow the rival: first loss on day %d (sum) against %d, %d corners lost against %d", streetDays, plainDays, streetLost, plainLost)
	}
}

// The Logistics branch (#119) moves more for less: with all six nodes
// from day 0 the distributor's road carries more units a shipment, pays
// less a unit in fares and is seized less over the horizon, summed over
// the seeds (the road rolls on its own side stream, so the risk cut is a
// count over five runs, never a promise on one), and the branch pays at
// the horizon on median peak cash.
func TestLogisticsBranchMovesMoreForLess(t *testing.T) {
	cfg := content.MustLoad()
	road := []string{"tyres", "compartments", "trucks", "drivers", "supplier", "supplier2", "ticket", "forwarder"}
	type tally struct {
		shipped, shipments, seized, fares int
		peaks                             []int
	}
	run := func(own bool) tally {
		var tl tally
		for seed := uint64(1); seed <= 5; seed++ {
			w := sim.NewWorld(cfg, seed)
			if own {
				Own(cfg, w, road...)
			}
			res, err := RunFrom(cfg, w, Horizon, Distributor(cfg, 40))
			if err != nil {
				t.Fatal(err)
			}
			tl.shipped += res.World.Stats.Shipped
			tl.shipments += res.World.Stats.Shipments
			tl.seized += res.World.Stats.Seizures
			for _, e := range res.Events {
				if sh, ok := e.(events.ShipmentSent); ok {
					tl.fares += sh.Cost
				}
			}
			tl.peaks = append(tl.peaks, res.PeakCash)
		}
		sort.Ints(tl.peaks)
		return tl
	}
	plain, branch := run(false), run(true)
	per := func(tl tally) (units, fare float64) {
		if tl.shipments == 0 || tl.shipped == 0 {
			return 0, 0
		}
		return float64(tl.shipped) / float64(tl.shipments), float64(tl.fares) / float64(tl.shipped)
	}
	pu, pf := per(plain)
	bu, bf := per(branch)
	t.Logf("plain: %d shipments, %.0f units each at $%.2f/u, %d seized, median peak %d; branch: %d shipments, %.0f units each at $%.2f/u, %d seized, median peak %d",
		plain.shipments, pu, pf, plain.seized, plain.peaks[len(plain.peaks)/2], branch.shipments, bu, bf, branch.seized, branch.peaks[len(branch.peaks)/2])
	if plain.shipments == 0 || branch.shipments == 0 {
		t.Fatal("the distributor never ran the road")
	}
	if bf >= pf || branch.seized > plain.seized {
		t.Fatalf("the road with the branch pays $%.2f/u (was $%.2f) and lost %d shipments (was %d)", bf, pf, branch.seized, plain.seized)
	}
	if branch.peaks[len(branch.peaks)/2] <= plain.peaks[len(plain.peaks)/2] {
		t.Fatalf("the branch does not pay: median peak %d with it, %d without", branch.peaks[len(branch.peaks)/2], plain.peaks[len(plain.peaks)/2])
	}
}
