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

// Intel (#45) at the harness level: the file is a layer the panels
// read, never a new truth, so a run that pays no cop and plants no spy
// is byte-for-byte the old run (the feed rolls on its own stream and a
// lie a policy never acts on moves nothing); the informed player, who
// buys the cop's word and lies low the night the police can move, is
// indicted later than the diplomat it is built on and earns no less;
// every fact's confidence is in 0..1 on every day, a fact forgets on
// its schedule, a spy is never at work; and a run with a cop paid, a
// spy under and a lie in play is deterministic and survives a save.

// TestNoIntelIsTheOldRun: with the feed on (the file's default) and no
// policy paying a cop, planting a spy or shipping on a road it was fed,
// the money curve's players read the same run on the file and under
// NoIntel (assertOldRun, as Run plays them: the net worth every day,
// the whole world every tenth and the last): the managed player to the
// tier-1 checkpoint, the crewed player to day 120 (past tier 2's) and
// the boss to tier 4's (past tier 3's), on the twenty seeds
// TestMoneyCurve takes its medians over, and nobody pays a cop, plants
// a spy or bites. Until #276 this pinned t1-t4's medians to the dollar
// (84,930 / 544,231 / 16,399,563 / 90,250,452), a copy of the money
// curve every balance PR had to edit, beside the crewed player's net
// worth and stats (the lures aside) on three seeds; the property is
// the box, and a feed that moves a number fails on the day it moves
// it, on any seed, where a median can hide it.
func TestNoIntelIsTheOldRun(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name   string
		policy func(*content.Config) Policy
		days   int
	}{
		{"managed", func(c *content.Config) Policy { return Managed(c, 50) }, tierDay(1)},
		{"crewed", func(c *content.Config) Policy { return Crewed(c, 40) }, max(tierDay(2), 120)},
		{"boss", func(c *content.Config) Policy { return Boss(c, 40, "") }, tierDay(4)},
	} {
		assertOldRun(t, oldRunCase{
			never:    "never used the file",
			box:      NoIntel,
			policies: map[string]func(*content.Config) Policy{row.name: row.policy},
			seeds:    20,
			days:     row.days,
			asRun:    true,
			every:    10,
			forbid: func(e events.Event) bool {
				switch e.(type) {
				case events.SpyPlanted, events.SpyFound, events.IntelFalse:
					return true
				}
				return false
			},
			after: func(t *testing.T, w *game.World, _ bool) {
				if s := w.Stats; s.CopsPaid+s.Spies+s.Bitten != 0 || len(w.Crew.Spies()) != 0 {
					t.Fatalf("paid a cop, planted a spy or bit: %+v", s)
				}
			},
		})
	}
}

// TestInformedOutlivesDiplomat: on the same seeds, both lying low at
// InformedTableHeat (over the sting line, so the diplomat is stung on
// the nights it sells), the informed player is indicted later (the
// median day, a survivor counting as the horizon) and holds at least as
// much at the horizon; it pays a cop, and the word it buys is the
// police's next move where it stands.
func TestInformedOutlivesDiplomat(t *testing.T) {
	t.Parallel()
	cfg := content.MustLoad()
	days := func(mk func(*content.Config, float64, int) Policy) (ends, worths []int, cops int) {
		for seed := uint64(1); seed <= InformedTableSeeds; seed++ {
			res, err := Run(cfg, seed, Horizon, mk(cfg, InformedTableHeat, 3))
			if err != nil {
				t.Fatal(err)
			}
			end := Horizon + 1
			if res.Over != nil {
				end = res.Over.Day
			}
			ends = append(ends, end)
			worths = append(worths, res.NetWorthAt(Horizon))
			cops += res.World.Stats.CopsPaid
			for _, f := range res.World.Intel {
				if f.Source == game.SourceCop && f.Kind == game.FactResponse && f.Subject != res.World.Player.Location {
					t.Fatalf("seed %d: the cop spoke of %s, the player stands in %s", seed, f.Subject, res.World.Player.Location)
				}
			}
		}
		sort.Ints(ends)
		sort.Ints(worths)
		return ends, worths, cops
	}
	dEnds, dWorths, dCops := days(Diplomat)
	iEnds, iWorths, iCops := days(Informed)
	mid := InformedTableSeeds / 2
	t.Logf("indicted on day %d (diplomat) against %d (informed), medians over %d seeds; net worth at day %d %d against %d; %d cops paid", dEnds[mid], iEnds[mid], InformedTableSeeds, Horizon, dWorths[mid], iWorths[mid], iCops)
	if dCops != 0 || iCops == 0 {
		t.Fatalf("the diplomat paid %d cops, the informed player %d", dCops, iCops)
	}
	if iEnds[mid] <= dEnds[mid] {
		t.Errorf("the informed player is indicted on day %d, the diplomat on day %d: the word bought no days", iEnds[mid], dEnds[mid])
	}
	if iWorths[mid] < dWorths[mid] {
		t.Errorf("the informed player holds %d at day %d, the diplomat %d: the word cost money", iWorths[mid], Horizon, dWorths[mid])
	}
}

// InformedTableSeeds and InformedTableHeat are the informed table's
// settings: the seeds and the lie-low line, over the sting line so the
// diplomat sells into stings.
const (
	InformedTableSeeds = 12
	InformedTableHeat  = 55
)

// spymaster is the informed player with a spy under and a road it was
// fed: it plants its most skilled runner with the nearest faction the
// first morning one is on the ground, and turns on the first road with
// a contact's word on it, so a run has a cop paid, a spy under and a
// lie in play.
func spymaster(cfg *content.Config) Policy {
	informed := Informed(cfg, 40, 3)
	return func(w *game.World) {
		informed(w)
		if w.Over != nil {
			return
		}
		if r := Nearest(w); r.Alive() && len(w.Crew.Spies()) == 0 && w.Today.Spy == nil {
			best := 0
			for _, m := range w.Crew.Members {
				if m.Role == "runner" && m.Working() && (best == 0 || m.Skill > w.Crew.Member(best).Skill) {
					best = m.ID
				}
			}
			if best != 0 {
				_ = w.PlantSpy(r.Faction(), best)
			}
		}
		for _, f := range game.Known(w).Facts() {
			if f.Kind == game.FactRisk && f.Source == game.SourceContact && !w.Route(f.Subject).Dial.On() {
				_ = w.SetRoute(f.Subject, events.RouteNormal)
				_ = w.SetRouteTarget(f.Subject, w.Products[0], 1000) // a shortfall the road sends at once
				w.SetStock(w.CityOrder[1], w.Products[0], 500)
			}
		}
	}
}

// TestIntelInvariants: over the spymaster on five seeds every live
// fact's confidence is in 0..1 on every day and its day never past
// today, a fact is gone once its confidence would fall under its
// forget line (and no sooner), a spy under is never at work, on a
// corner or a route, a lie about a road names one with its dial off
// the night it is fed, and every lie that bit names its author.
func TestIntelInvariants(t *testing.T) {
	t.Parallel()
	cfg := content.MustLoad()
	cfg.Intel.Intel.FeedChance = 1 // a lie every night a faction distrusts you
	tun := cfg.Intel.Intel
	for seed := uint64(1); seed <= 5; seed++ {
		w := sim.NewWorld(cfg, seed)
		for _, r := range w.Rivals {
			r.Trust = 0 // every faction distrusts you from the start: a lie a night once one is on the ground
		}
		_, sims, err := sim.Default(cfg)
		if err != nil {
			t.Fatal(err)
		}
		clock := game.NewClock(nil, sims...)
		policy := spymaster(cfg)
		var fed []game.Fact
		for day := 1; day <= 120 && w.Over == nil; day++ {
			policy(w)
			lures := map[string]game.Fact{}
			for _, f := range w.Intel {
				if f.Lure() && f.Alive(w.Day) {
					lures[f.Subject+"/"+f.Kind] = f
				}
			}
			evs := clock.EndDay(w)
			for _, f := range w.Intel {
				c := f.Now(w.Day)
				if c < 0 || c > 1 || f.Day > w.Day {
					t.Fatalf("seed %d day %d: %+v reads %.3f", seed, w.Day, f, c)
				}
			}
			for _, f := range game.Known(w).Facts() {
				if f.Now(w.Day) < f.Forget {
					t.Fatalf("seed %d day %d: a fact under its forget line is live: %+v", seed, w.Day, f)
				}
			}
			for _, m := range w.Crew.Members {
				if m.Undercover == "" {
					continue
				}
				if m.Working() || m.Fit(w.Day) || w.PostOf(m.ID) != nil || w.DrivenRoute(m.ID) != "" || w.Faction(m.Undercover) == nil {
					t.Fatalf("seed %d day %d: a spy at work: %+v", seed, w.Day, m)
				}
			}
			for _, e := range evs {
				switch ev := e.(type) {
				case events.IntelGained:
					if ev.Source == game.SourceContact {
						f, ok := game.Known(w).Fact(ev.Subject, ev.FactKind)
						if !ok {
							t.Fatalf("seed %d day %d: a lie fed and not filed: %+v", seed, w.Day, ev)
						}
						fed = append(fed, f)
						if f.Kind == game.FactRisk && f.Number != tun.FeedRisk {
							t.Fatalf("seed %d day %d: a road lie at %.3f", seed, w.Day, f.Number)
						}
					}
				case events.IntelFalse:
					// The lie that bit is the one planted that morning.
					// It bites on the night after the morning it was
					// acted on, so one that bit on its last live day is
					// forgotten by the time the night is over (seed 2
					// day 28 once investigations shipped on, #343, moved
					// the run): the file names the author while it holds
					// the lie, and the plant always does.
					f, ok := game.Known(w).Fact(ev.Subject, ev.FactKind)
					if !ok {
						f, ok = lures[ev.Subject+"/"+ev.FactKind], lures[ev.Subject+"/"+ev.FactKind].Planted != ""
						f.Source = f.Planted
					}
					if !ok || f.Source != ev.Faction || w.Faction(ev.Faction) == nil {
						t.Fatalf("seed %d day %d: a bite that names nobody: %+v %+v", seed, w.Day, ev, f)
					}
				}
			}
		}
		// A fact forgets on schedule: the oldest lie fed, if still in
		// the file, is under its life; one older than its life is gone.
		life := int((tun.FeedConfidence-tun.Forget)/tun.StaleRate) + 1
		for _, f := range fed {
			_, live := game.Known(w).Fact(f.Subject, f.Kind)
			if live && w.Day-f.Day > life {
				if g, _ := game.Known(w).Fact(f.Subject, f.Kind); g.Day == f.Day {
					t.Fatalf("seed %d: a lie fed on day %d is still live on day %d, %d days past its life", seed, f.Day, w.Day, life)
				}
			}
		}
		if len(fed) == 0 || w.Stats.Bitten == 0 {
			t.Fatalf("seed %d: %d lies fed, %d bit: the spymaster was never lured onto a road", seed, len(fed), w.Stats.Bitten)
		}
	}
}

// TestIntelIsDeterministicAndSaves: the spymaster on one seed twice is
// the same run event for event, with a cop paid, a spy reporting and a
// lie that bit in it; and the run saved on a morning with a spy under
// and a lie in the file replays the rest exactly after a load, the
// file, the spy and the lie intact.
func TestIntelIsDeterministicAndSaves(t *testing.T) {
	t.Setenv("KINGPIN_HOME", t.TempDir())
	// The duel (#43, harness.OneFaction): this pins a mechanism on a seed, and the table moves the seed's dice.
	cfg := OneFaction(content.MustLoad())
	cfg.Intel.Intel.FeedChance = 1
	policy := func() Policy { return spymaster(cfg) }
	play := func(days int) Result {
		w := sim.NewWorld(cfg, 6)
		w.Rival().Trust = 0 // it distrusts you from the start: a lie a night
		w.Player.DirtyCash = 200_000
		res, err := RunFrom(cfg, w, days, policy())
		if err != nil {
			t.Fatal(err)
		}
		return res
	}
	a, b := play(120), play(120)
	if len(a.Events) != len(b.Events) {
		t.Fatalf("event counts differ: %d vs %d", len(a.Events), len(b.Events))
	}
	for i := range a.Events {
		if fmt.Sprintf("%#v", a.Events[i]) != fmt.Sprintf("%#v", b.Events[i]) {
			t.Fatalf("event %d differs:\n%#v\n%#v", i, a.Events[i], b.Events[i])
		}
	}
	used := map[string]int{}
	for _, e := range a.Events {
		switch ev := e.(type) {
		case events.SpyPlanted, events.IntelFalse:
			used[e.Kind()]++
		case events.IntelGained:
			used[ev.Source]++
		}
	}
	if used["SpyPlanted"] == 0 || used[game.SourceContact] == 0 || used[game.SourceSpy] == 0 || used["IntelFalse"] == 0 {
		t.Fatalf("the spymaster did not use the file on seed 6: %v", used)
	}
	if a.World.Stats.CopsPaid == 0 || a.World.Stats.Reports == 0 {
		t.Fatalf("the spymaster paid %d cops and got %d reports on seed 6", a.World.Stats.CopsPaid, a.World.Stats.Reports)
	}
	// Save on a morning with a spy under and a lie live in the file (the
	// cop's word, bought at heat over InformedHeat, is in the file or
	// not by then; the save carries it either way).
	c := play(20)
	policy()(c.World)
	for (len(c.World.Crew.Spies()) == 0 || len(lures(c.World)) == 0) && c.World.Day < 110 {
		c = play(c.World.Day + 1)
		policy()(c.World)
	}
	if len(c.World.Crew.Spies()) == 0 || len(lures(c.World)) == 0 {
		t.Fatalf("between day 20 and 110 of seed 6 there was never a morning with a spy under and a lie live: spies %d lies %d", len(c.World.Crew.Spies()), len(lures(c.World)))
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
	if fmt.Sprintf("%+v", loaded.Intel) != fmt.Sprintf("%+v", c.World.Intel) || len(loaded.Crew.Spies()) != len(c.World.Crew.Spies()) || loaded.Crew.Spies()[0].UndercoverDay != c.World.Crew.Spies()[0].UndercoverDay {
		t.Fatalf("the file loaded as %+v, saved %+v; spies %+v against %+v", loaded.Intel, c.World.Intel, loaded.Crew.Spies(), c.World.Crew.Spies())
	}
	d, _ := RunFrom(cfg, loaded, 120-c.World.Day, policy())
	ref := play(c.World.Day)
	policy()(ref.World)
	e, _ := RunFrom(cfg, ref.World, 120-c.World.Day, policy())
	if len(d.Events) != len(e.Events) {
		t.Fatalf("after loading, %d events for the last %d days, want %d", len(d.Events), 120-c.World.Day, len(e.Events))
	}
	for i := range e.Events {
		if fmt.Sprintf("%#v", e.Events[i]) != fmt.Sprintf("%#v", d.Events[i]) {
			t.Fatalf("event %d after the save differs:\n%#v\n%#v", i, e.Events[i], d.Events[i])
		}
	}
}

// lures is the lies live in the file.
func lures(w *game.World) []game.Fact {
	var out []game.Fact
	for _, f := range game.Known(w).Facts() {
		if f.Lure() {
			out = append(out, f)
		}
	}
	return out
}
