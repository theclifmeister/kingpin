package harness

import (
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/sim"
)

// The table (#43): three to six factions by seed, staggered in, fighting
// each other as well as you. The acceptance criteria live here; the
// arithmetic is in sim/rivals/factions_test.go.

// One faction is the old run (#43): under harness.OneFaction the rival
// at home rolls on the tick's stream alone and nothing of the table
// fires or is written, so the money curve reads main's own figures
// after #46 to the dollar (t1-t4, the incidents boxed as every harness
// run has them) and a run of every home policy emits none of the
// table's events.
func TestOneFactionIsTheOldRun(t *testing.T) {
	cfg := OneFaction(content.MustLoad())
	for _, row := range []struct {
		tier   int
		policy func(*content.Config) Policy
		want   int
	}{
		{1, func(c *content.Config) Policy { return Managed(c, 50) }, 84_930},
		{2, func(c *content.Config) Policy { return Crewed(c, 40) }, 595_556},
		{3, func(c *content.Config) Policy { return Boss(c, 40, "") }, 17_481_529},
		{4, func(c *content.Config) Policy { return Boss(c, 40, "") }, 94_304_076},
	} {
		if got := medianNetWorth(t, cfg, row.policy, tierDay(row.tier)); got != row.want {
			t.Errorf("tier %d in the duel: median net worth %d on day %d, main's figure after #46 is %d", row.tier, got, tierDay(row.tier), row.want)
		}
	}
	for name, policy := range map[string]Policy{
		"territory": Territory(cfg, 40, 4),
		"war":       Warlike(cfg, 40, 4, events.ForcePush),
		"tipster":   Tipster(cfg, 40),
		"diplomat":  Diplomat(cfg, 40, 3),
	} {
		res, err := Run(cfg, 3, Horizon, policy)
		if err != nil {
			t.Fatal(err)
		}
		w := res.World
		r := w.Rival()
		// The tipster's arrest is the player's own doing and lands in a
		// duel too (Fragmented); nothing else of the table does.
		if len(w.Rivals) != 1 || r.Home != "" || len(r.Grudges) != 0 || len(r.Trusts) != 0 || r.Ally != "" || r.Against != "" || r.Absorbed != 0 || r.LastTakenBy != "" || w.Stats.CrewPoached != 0 || w.Stats.Homage != 0 || w.Stats.Absorbed != 0 || (r.Fragmented != 0 && name != "tipster") {
			t.Fatalf("%s: the duel carries the table's state: %+v %+v", name, *r, w.Stats)
		}
		for _, e := range res.Events {
			switch e.(type) {
			case events.FactionPushed, events.RivalAbsorbed, events.CrewPoached, events.TrustSpread:
				t.Fatalf("%s: %s in a duel", name, e.Kind())
			}
			if d, ok := e.(events.DealOffered); ok && d.Deal == game.DealHomage {
				t.Fatalf("%s: a homage offered in a duel", name)
			}
		}
	}
}

// With four factions a rival-vs-rival push happens before day 120 on
// at least 80% of seeds (the issue asked for day 60 with every faction
// arriving on day 10; the table arrives arrive_gap days apart so the
// bands hold, and the acceptance moved with it), and a faction is
// absorbed on some seed within 200 days. No corner ever has two owners
// and every faction's books stay in range, on every day.
func TestFactionsContestBeforeDay120(t *testing.T) {
	cfg := Factions(content.MustLoad(), 4)
	pushed, absorbed := 0, 0
	const seeds = 10
	for seed := uint64(1); seed <= seeds; seed++ {
		first := 0
		res, err := RunFrom(cfg, sim.NewWorld(cfg, seed), Horizon, Territory(cfg, 40, 3))
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range res.Events {
			switch ev := e.(type) {
			case events.FactionPushed:
				if first == 0 {
					first = ev.Day
				}
				if ev.Faction == ev.Against || ev.Faction == "" || ev.Against == "" {
					t.Fatalf("seed %d: a faction pushed on itself: %+v", seed, ev)
				}
			case events.RivalAbsorbed:
				absorbed++
			}
		}
		if first > 0 && first < 120 {
			pushed++
		}
		if len(res.World.Rivals) != 4 {
			t.Fatalf("seed %d: %d factions", seed, len(res.World.Rivals))
		}
	}
	t.Logf("four factions: a rival-vs-rival push before day 120 on %d of %d seeds, %d absorptions in 200 days", pushed, seeds, absorbed)
	if pushed*10 < seeds*8 {
		t.Fatalf("a rival-vs-rival push before day 120 on only %d of %d seeds", pushed, seeds)
	}
	if absorbed == 0 {
		t.Fatal("no faction was absorbed on any seed within 200 days")
	}
}

// Poaching is the defection path with a faction named (#43): over the
// crewed player's runs a faction makes offers; one that lands drops the
// member from the roster the same night and queues the lead the faction
// acts on next step (the corner they stood on; sim/rivals pins the head
// it gains), one that does not leaves them on the payroll.
func TestPoachingIsTheDefectionPath(t *testing.T) {
	cfg := content.MustLoad()
	offers, landed := 0, 0
	for seed := uint64(1); seed <= 12; seed++ {
		w := sim.NewWorld(cfg, seed)
		_, sims, err := sim.Default(cfg)
		if err != nil {
			t.Fatal(err)
		}
		clock := game.NewClock(nil, sims...)
		policy := Crewed(cfg, 40)
		for day := 1; day <= Horizon && w.Over == nil; day++ {
			policy(w)
			for _, e := range clock.EndDay(w) {
				p, ok := e.(events.CrewPoached)
				if !ok {
					continue
				}
				offers++
				if p.Stayed {
					if m := w.Crew.Member(p.ID); m == nil {
						t.Fatalf("seed %d day %d: %s stayed and is gone", seed, w.Day, p.Name)
					}
					continue
				}
				landed++
				if w.Crew.Member(p.ID) != nil {
					t.Fatalf("seed %d day %d: %s took the offer and is still on the payroll", seed, w.Day, p.Name)
				}
				found := false
				for _, l := range w.Crew.Leads {
					found = found || (l.Name == p.Name && l.Faction == p.Faction)
				}
				if !found {
					t.Fatalf("seed %d day %d: no lead for %s: %+v", seed, w.Day, p.Name, w.Crew.Leads)
				}
			}
		}
	}
	t.Logf("crewed, twelve seeds: %d offers, %d took them", offers, landed)
	if offers == 0 || landed == 0 {
		t.Fatalf("%d offers and %d poached over twelve crewed runs", offers, landed)
	}
}

// Tipping a faction until its leader is arrested fragments it (#43):
// the tipster's tips take the leader on some seed, and within
// fragment_days its corners are the street's, its muscle is in your
// pool at the discount, and its city's prices spiked the morning after.
func TestTippingFragmentsTheFaction(t *testing.T) {
	cfg := content.MustLoad()
	f := cfg.Rivals.Factions
	fragmented := false
	for seed := uint64(1); seed <= 8 && !fragmented; seed++ {
		var arrested *events.RivalLeaderArrested
		var spiked, pooled bool
		res, err := Play(cfg, sim.NewWorld(cfg, seed), Horizon, func(w *game.World) {
			for _, r := range w.Rivals {
				if r.Fragmented == 0 {
					continue
				}
				if r.Muscle != 0 || !r.Gone() {
					t.Fatalf("seed %d day %d: the arrested faction is still in the game: %+v", seed, w.Day, *r)
				}
				if w.Day >= r.Fragmented+f.FragmentDays && w.RivalHeldBy(r.Faction()) != 0 {
					t.Fatalf("seed %d day %d: %d corners still theirs past fragment_days", seed, w.Day, w.RivalHeldBy(r.Faction()))
				}
				if r.Fragmented == w.Day { // the morning after: their people are in the pool
					for _, c := range w.Crew.Candidates {
						pooled = pooled || c.Former == r.Faction()
					}
				}
			}
			Tipster(cfg, 40)(w)
		}, Options{})
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range res.Events {
			switch ev := e.(type) {
			case events.RivalLeaderArrested:
				if arrested == nil {
					arrested = &ev
				}
			case events.PriceShock:
				if arrested != nil && ev.Day == arrested.Day+1 && ev.City == arrested.City && ev.Factor == f.ShockMul && ev.Days == f.ShockDays {
					spiked = true
				}
			}
		}
		if arrested == nil {
			continue
		}
		fragmented = true
		if arrested.Muscle > 0 && !pooled {
			t.Fatalf("seed %d: %d heads and none in the pool", seed, arrested.Muscle)
		}
		if !spiked {
			t.Fatalf("seed %d: no price spike in %s the morning after day %d", seed, arrested.City, arrested.Day)
		}
		t.Logf("seed %d: %s arrested on day %d with %d corners and %d heads", seed, arrested.Rival, arrested.Day, arrested.Corners, arrested.Muscle)
	}
	if !fragmented {
		t.Fatal("the tipster took no leader on eight seeds")
	}
}

// Dominant is false on every day of every policy over 200 days (#43):
// nobody wins the city by accident.
func TestNobodyIsDominantByAccident(t *testing.T) {
	cfg := content.MustLoad()
	policies := map[string]Policy{
		"idle": Idle, "hide": Hide, "quiet": Trader(cfg, events.DialQuiet), "aggressive": Trader(cfg, events.DialAggressive),
		"managed": Managed(cfg, 50), "crewed": Crewed(cfg, 40), "territory": Territory(cfg, 40, 4),
		"war-push": Warlike(cfg, 40, 4, events.ForcePush), "war-hit": Warlike(cfg, 60, 4, events.ForceHit),
		"diplomat": Diplomat(cfg, 40, 3), "laundered": Laundered(cfg, 40), "distributor": Distributor(cfg, 40),
		"boss": Boss(cfg, 40, ""), "saboteur": Saboteur(cfg, 40), "tipster": Tipster(cfg, 40), "pricewar": Pricewar(cfg, 40, 3, events.DialNormal),
	}
	for name, policy := range policies {
		for seed := uint64(1); seed <= 3; seed++ {
			_, err := RunFrom(cfg, sim.NewWorld(cfg, seed), Horizon, func(w *game.World) {
				if w.Dominant() {
					t.Fatalf("%s seed %d day %d: dominant by accident", name, seed, w.Day)
				}
				policy(w)
			})
			if err != nil {
				t.Fatal(err)
			}
		}
	}
}

// Dominant is true in a scripted scenario (#43): two of three factions
// gone, and the tipster taking the last one's leader.
func TestDominantScripted(t *testing.T) {
	cfg := Factions(content.MustLoad(), 3)
	for seed := uint64(1); seed <= 6; seed++ {
		w := sim.NewWorld(cfg, seed)
		w.Rivals[1].Absorbed, w.Rivals[1].Arrived = 1, 1
		w.Rivals[2].Fragmented, w.Rivals[2].Arrived = 1, 1
		w.Player.DirtyCash = 200_000
		day := 0
		res, err := RunFrom(cfg, w, Horizon, func(w *game.World) {
			if day == 0 && w.Dominant() {
				day = w.Day
			}
			Tipster(cfg, 40)(w)
		})
		if err != nil {
			t.Fatal(err)
		}
		if day > 0 {
			t.Logf("seed %d: dominant on day %d (%s %s)", seed, day, res.World.Rival().Leader, res.World.Stance(res.World.Rival(), 40))
			return
		}
	}
	t.Fatal("six seeds of a tipster against the last faction and never dominant")
}

// Six factions are deterministic and survive a save (#43): the same seed
// twice is the same run, and a run saved on a morning with the table
// dealt loads with every faction and plays on as the unsaved run did.
func TestSixFactionsAreDeterministicAndSave(t *testing.T) {
	t.Setenv("KINGPIN_HOME", t.TempDir())
	cfg := Factions(content.MustLoad(), 6)
	policy := func() Policy { return Territory(cfg, 40, 4) }
	play := func(days int) Result {
		res, err := RunFrom(cfg, sim.NewWorld(cfg, 2), days, policy())
		if err != nil {
			t.Fatal(err)
		}
		return res
	}
	a, b := play(150), play(150)
	if len(a.Events) != len(b.Events) || a.PeakCash != b.PeakCash {
		t.Fatalf("runs diverged: %d/%d events, peak %d/%d", len(a.Events), len(b.Events), a.PeakCash, b.PeakCash)
	}
	for i := range a.World.Rivals {
		x, y := *a.World.Rivals[i], *b.World.Rivals[i]
		if x.Leader != y.Leader || x.Cash != y.Cash || x.Muscle != y.Muscle || x.Trust != y.Trust || x.Arrived != y.Arrived {
			t.Fatalf("faction %d differs: %+v vs %+v", i, x, y)
		}
	}
	c := play(90)
	if len(c.World.Rivals) != 6 {
		t.Fatalf("%d factions", len(c.World.Rivals))
	}
	if err := game.Save(1, c.World); err != nil {
		t.Fatal(err)
	}
	set, _, err := sim.Default(cfg)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := game.Load(1, set.Migrations()...)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Rivals) != 6 {
		t.Fatalf("loaded %d factions", len(loaded.Rivals))
	}
	for i := range loaded.Rivals {
		x, y := *loaded.Rivals[i], *c.World.Rivals[i]
		if x.Leader != y.Leader || x.Cash != y.Cash || x.Muscle != y.Muscle || x.Trust != y.Trust || x.Arrived != y.Arrived || x.Home != y.Home || len(x.Deals) != len(y.Deals) || x.Absorbed != y.Absorbed || x.Fragmented != y.Fragmented || len(x.Grudges) != len(y.Grudges) {
			t.Fatalf("faction %d loaded as %+v, saved %+v", i, x, y)
		}
	}
	d, _ := RunFrom(cfg, loaded, 150-c.World.Day, policy())
	ref := play(c.World.Day)
	e, _ := RunFrom(cfg, ref.World, 150-c.World.Day, policy())
	if len(d.Events) != len(e.Events) || d.PeakCash != e.PeakCash || d.World.RivalHeld() != e.World.RivalHeld() {
		t.Fatalf("the save played on differently: %d/%d events, peak %d/%d, rivals hold %d/%d", len(d.Events), len(e.Events), d.PeakCash, e.PeakCash, d.World.RivalHeld(), e.World.RivalHeld())
	}
}

// The away knob (#43): with away = 1 a seat after the first lives in
// Bayport, and its corners are Bayport's.
func TestAwayFactionHoldsBayport(t *testing.T) {
	cfg := Factions(content.MustLoad(), 4)
	cfg.Rivals.Factions.Away = 1
	for seed := uint64(1); seed <= 4; seed++ {
		res, err := RunFrom(cfg, sim.NewWorld(cfg, seed), 120, Idle)
		if err != nil {
			t.Fatal(err)
		}
		w := res.World
		for _, r := range w.Rivals {
			if r.Home != w.CityOrder[1] {
				continue
			}
			if r.Arrived > 0 && w.HeldByIn(r.Faction(), r.Home) == 0 {
				t.Fatalf("seed %d: %s lives in %s, arrived day %d, and holds nothing there", seed, r.Leader, r.Home, r.Arrived)
			}
			if w.HeldByIn(r.Faction(), w.Home().ID) != 0 {
				t.Fatalf("seed %d: %s lives away and holds ground at home", seed, r.Leader)
			}
			if r.Arrived > 0 {
				return
			}
		}
	}
	t.Fatal("four seeds at away = 1 and no faction arrived in Bayport by day 120")
}
