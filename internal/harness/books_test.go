package harness

import (
	"fmt"
	"sort"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/sim"
	"github.com/theclifmeister/kingpin/internal/sim/rivals"
)

// The books (#70): scouting, boosting, tipping and buying off muscle,
// the fourth answer to the rival beside the enforcers, the table and the
// price war.

func booksSim(cfg *content.Config) *rivals.Sim {
	return rivals.New(cfg)
}

// Invariants under the saboteur and the tipster, every day of five seeds
// against every temper: the rival's cash and muscle never negative and
// its heat in 0..100; a boost never changes a corner's owner (a raid, a
// crackdown or a price war the same night can, and each says so); the heads bought off never join the
// crew (the roster changes only by the hires, quits, firings and
// defections the night reported); a boost that landed put its cash in
// your pocket (the stats add up); Known is written by nothing but a
// scout that read the books and its day is never past today.
func TestBooksInvariants(t *testing.T) {
	cfg := content.MustLoad()
	for name, mk := range map[string]func() Policy{
		"saboteur": func() Policy { return Saboteur(cfg, 40) },
		"tipster":  func() Policy { return Tipster(cfg, 40) },
	} {
		for seed := uint64(1); seed <= 5; seed++ {
			known := game.Known{}
			crew := 0
			var last []events.Event
			boosted := 0
			check := func(w *game.World) {
				r := w.Rival
				if r.Cash < 0 || r.Muscle < 0 || r.Heat < 0 || r.Heat > 100 {
					t.Fatalf("%s seed %d day %d: rival cash %d muscle %d heat %.1f", name, seed, w.Day, r.Cash, r.Muscle, r.Heat)
				}
				if r.Known.Day > w.Day {
					t.Fatalf("%s seed %d day %d: the books read on day %d", name, seed, w.Day, r.Known.Day)
				}
				read, hired, gone := false, 0, 0
				cleared := map[string]bool{} // corners the night put back on the street: a raid, a crackdown, a price war
				for _, e := range last {
					switch ev := e.(type) {
					case events.RivalRaided:
						cleared[ev.Corner] = true
					case events.CornerLost:
						cleared[ev.Corner] = true
					case events.RivalAbandoned:
						cleared[ev.Corner] = true
					}
				}
				for _, e := range last {
					switch ev := e.(type) {
					case events.RivalScouted:
						read = read || ev.Read
					case events.CrewHired:
						hired++
					case events.CrewQuit, events.CrewFired, events.CrewDefected:
						gone++
					case events.RivalBoosted:
						if c := w.Corner(ev.Corner); c.Owner != game.OwnerRival && !cleared[ev.Corner] {
							t.Fatalf("%s seed %d day %d: %s is %s's the morning after a boost", name, seed, w.Day, ev.Name, c.Owner)
						}
						if ev.Taken {
							if ev.Cash <= 0 {
								t.Fatalf("%s seed %d day %d: a boost landed for nothing: %+v", name, seed, w.Day, ev)
							}
							boosted += ev.Cash
						} else if ev.Cash != 0 {
							t.Fatalf("%s seed %d day %d: a boost held off took %d", name, seed, w.Day, ev.Cash)
						}
					}
				}
				if r.Known != known && !read {
					t.Fatalf("%s seed %d day %d: the books moved %+v -> %+v with no scout reading them", name, seed, w.Day, known, r.Known)
				}
				if read && r.Known.Day != w.Day {
					t.Fatalf("%s seed %d day %d: read last night, stamped day %d", name, seed, w.Day, r.Known.Day)
				}
				known = r.Known
				if w.Day > 0 && len(w.Crew.Members) != crew+hired-gone {
					t.Fatalf("%s seed %d day %d: the roster went %d -> %d with %d hired and %d gone", name, seed, w.Day, crew, len(w.Crew.Members), hired, gone)
				}
				crew = len(w.Crew.Members)
				if w.Stats.Boosted != boosted {
					t.Fatalf("%s seed %d day %d: boosted %d in the stats, %d in the events", name, seed, w.Day, w.Stats.Boosted, boosted)
				}
			}
			policy := mk()
			w := sim.NewWorld(cfg, seed)
			clock := game.NewClock(nil, mustSims(t, cfg)...)
			for d := 0; d < 200 && w.Over == nil; d++ {
				check(w)
				policy(w)
				last = clock.EndDay(w)
			}
			check(w)
		}
	}
}

func mustSims(t *testing.T, cfg *content.Config) []game.Simulation {
	t.Helper()
	_, sims, err := sim.Default(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return sims
}

// The moves work and are priced: over five seeds the saboteur leaves the
// rival less muscle at day 120 than the crewed player, who never touches
// it, and the heat its fight adds (boosts, the rival's calls, the
// crackdowns) is under the push war's, both lying low at the same line.
// The rival's corners under each are logged: the war routs it where the
// saboteur bleeds it.
func TestSaboteurDrainsTheMuscleQuietly(t *testing.T) {
	cfg := content.MustLoad()
	type row struct {
		muscle, corners []int
		heat            []float64
	}
	rows := map[string]*row{}
	for name, policy := range map[string]Policy{
		"crewed":   Crewed(cfg, 40),
		"saboteur": Saboteur(cfg, 40),
		"war":      Warlike(cfg, 40, 0, events.ForcePush),
	} {
		r := &row{}
		for seed := uint64(1); seed <= 5; seed++ {
			res := pricewarRun(t, cfg, seed, 120, "", policy, nil)
			added := 0.0
			for _, e := range res.Events {
				switch ev := e.(type) {
				case events.CornerStruck:
					added += ev.Heat
				case events.RivalBoosted:
					added += ev.Heat
				case events.RivalTippedPolice:
					added += ev.Heat
				case events.WarEscalated:
					added += ev.Heat
				}
			}
			r.muscle = append(r.muscle, res.World.Rival.Muscle)
			r.corners = append(r.corners, res.World.RivalHeld())
			r.heat = append(r.heat, added)
		}
		sort.Ints(r.muscle)
		sort.Ints(r.corners)
		sort.Float64s(r.heat)
		rows[name] = r
		t.Logf("%s: rival muscle %v, corners %v, heat from the fight %.0f (medians over 5 seeds at day 120: muscle %d, corners %d, heat %.0f)", name, r.muscle, r.corners, r.heat, r.muscle[2], r.corners[2], r.heat[2])
	}
	if rows["saboteur"].muscle[2] >= rows["crewed"].muscle[2] {
		t.Fatalf("the saboteur left the rival %d muscle at the median, the crewed player %d", rows["saboteur"].muscle[2], rows["crewed"].muscle[2])
	}
	if rows["saboteur"].heat[2] >= rows["war"].heat[2] {
		t.Fatalf("the saboteur's fight added %.0f heat at the median, the push war's %.0f", rows["saboteur"].heat[2], rows["war"].heat[2])
	}
}

// Boosting is the fast drain and buying muscle the expensive one: on a
// rival whose chest is a night's wages, one night of boosting takes its
// till and costs you nothing in cash, a buy-off of the same heads takes
// more of them and costs cash for each; the buy-off removes more muscle
// than the boost's night and costs more per head.
func TestBuyOffRemovesMoreMuscleAtAPrice(t *testing.T) {
	cfg := content.MustLoad()
	sure := *cfg
	sure.Rivals.Poach.Odds = 1
	// The boost lands for certain: no defence on the corner and a hit
	// that always flips, so the night is the move's and not the dice's.
	sure.Rivals.Personality = map[string]content.PersonalityConfig{}
	for k, v := range cfg.Rivals.Personality {
		sure.Rivals.Personality[k] = v
	}
	pc := sure.Rivals.Personality["defensive"]
	pc.Defence = 0
	sure.Rivals.Personality["defensive"] = pc
	sure.Rivals.Force = map[string]content.ForceConfig{}
	for k, v := range cfg.Rivals.Force {
		sure.Rivals.Force[k] = v
	}
	hit := sure.Rivals.Force["hit"]
	hit.Flip = 1
	sure.Rivals.Force["hit"] = hit
	rv := booksSim(&sure)
	fixture := func() *game.World {
		w := sim.NewWorld(&sure, 4)
		w.Rival.Personality = "defensive"
		res, err := RunFrom(&sure, w, 40, Territory(&sure, 40, 3))
		if err != nil {
			t.Fatal(err)
		}
		w = res.World
		if w.RivalHeld() == 0 || min(rv.Want(w), rv.Afford(w)) < 3 {
			t.Fatalf("day 40 of seed 4: rival holds %d corners, wants %d and affords %d", w.RivalHeld(), rv.Want(w), rv.Afford(w))
		}
		w.Rival.Muscle = min(rv.Want(w), rv.Afford(w)) // at strength: the morning hires nobody, so the night's move is all that moves it
		w.Rival.Cash = rv.Wages(w)                     // a night's wages in the chest: a drain binds at once
		w.Player.DirtyCash = 4 * rv.MusclePrice(w)
		w.Home().Heat, w.Heat.Evidence = 0, 0 // the night is the move's, not the police's
		return w
	}
	// A night of boosting.
	boost := fixture()
	target := pickCorner(boost, func(c game.Corner) bool { return c.Owner == game.OwnerRival }, size)
	muscle, cash := boost.Rival.Muscle, boost.Player.DirtyCash
	if err := boost.Boost(target.ID, events.ForceHit); err != nil {
		t.Fatal(err)
	}
	clock := game.NewClock(nil, mustSims(t, &sure)...)
	evs := clock.EndDay(boost)
	var landed *events.RivalBoosted
	for _, e := range evs {
		if ev, ok := e.(events.RivalBoosted); ok && ev.Taken {
			landed = &ev
		}
	}
	if landed == nil {
		t.Fatalf("the boost at hit against no defence did not land: %v", evs)
	}
	boostGone := muscle - boost.Rival.Muscle
	boostCost := cash - boost.Player.DirtyCash // negative: the boost paid
	// The buy-off of three heads.
	buy := fixture()
	price := rv.MusclePrice(buy)
	muscle, cash = buy.Rival.Muscle, buy.Player.DirtyCash
	if err := buy.BuyOff(3, 3*price); err != nil {
		t.Fatal(err)
	}
	clock.EndDay(buy)
	buyGone := muscle - buy.Rival.Muscle
	buyCost := cash - buy.Player.DirtyCash
	t.Logf("a night of boosting: %d heads gone, %s to you; buying off three: %d heads gone for %s (%s a head)", boostGone, format(-boostCost), buyGone, format(buyCost), format(buyCost/max(1, buyGone)))
	if buyGone <= boostGone {
		t.Fatalf("the buy-off removed %d heads, the boost's night %d", buyGone, boostGone)
	}
	if buyGone < 3 || buyCost/buyGone <= boostCost/max(1, boostGone) {
		t.Fatalf("the buy-off cost %d a head, the boost %d", buyCost/max(1, buyGone), boostCost/max(1, boostGone))
	}
}

func format(n int) string { return fmt.Sprintf("$%d", n) }

// Tipping has teeth both ways: tips every night from the rival's arrival
// cost it a corner within stale_days of the first on every seed; the
// tipster ends day 30 with a thicker file than a saboteur that never
// tips; a tip under a truce breaks it; and a raid raises home's
// pressure.
func TestTipsHaveTeethBothWays(t *testing.T) {
	cfg := content.MustLoad()
	tp := cfg.Rivals.Tip
	stale := cfg.Rivals.Books.StaleDays
	for seed := uint64(1); seed <= 5; seed++ {
		first, raided := 0, 0
		for _, e := range pricewarRun(t, cfg, seed, 60, "", Tipster(cfg, 40), nil).Events {
			switch ev := e.(type) {
			case events.PoliceTipped:
				if first == 0 {
					first = ev.Day
				}
			case events.RivalRaided:
				if raided == 0 {
					raided = ev.Day
				}
			}
		}
		if first == 0 || raided == 0 || raided-first >= stale {
			t.Fatalf("seed %d: first tip on day %d, first raid on day %d, stale after %d", seed, first, raided, stale)
		}
	}
	var never, always []int
	for seed := uint64(1); seed <= 5; seed++ {
		never = append(never, pricewarRun(t, cfg, seed, 30, "", saboteur(cfg, 40, tipsNever), nil).World.Heat.Evidence)
		always = append(always, pricewarRun(t, cfg, seed, 30, "", Tipster(cfg, 40), nil).World.Heat.Evidence)
	}
	sort.Ints(never)
	sort.Ints(always)
	t.Logf("the file at day 30 over 5 seeds: never tipping %v, tipping every night %v", never, always)
	if always[2] <= never[2] {
		t.Fatalf("the tipster's file at the median is %d pages, the saboteur that never tips has %d", always[2], never[2])
	}
	// A tip under a truce is a betrayal, and the raid it brings is
	// pressure at home.
	w := sim.NewWorld(cfg, 3)
	w.Rival.Personality = "defensive"
	res, err := RunFrom(cfg, w, 40, Territory(cfg, 40, 3))
	if err != nil {
		t.Fatal(err)
	}
	w = res.World
	if w.RivalHeld() == 0 {
		t.Fatal("day 40 of seed 3: no rival corner")
	}
	w.Rival.Deals = []game.Deal{{Kind: game.DealTruce, Terms: game.Terms{Days: 30}, Since: w.Day, Until: w.Day + 30}}
	w.Rival.Heat = tp.PoliceNotice - tp.Heat // one tip from the line
	target := pickCorner(w, func(c game.Corner) bool { return c.Owner == game.OwnerRival }, size)
	betrayals, pressure := w.Stats.Betrayals, w.Home().Pressure
	if err := w.Tip(target.ID); err != nil {
		t.Fatal(err)
	}
	clock := game.NewClock(nil, mustSims(t, cfg)...)
	evs := clock.EndDay(w)
	var raid *events.RivalRaided
	for _, e := range evs {
		if ev, ok := e.(events.RivalRaided); ok {
			raid = &ev
		}
	}
	if raid == nil {
		t.Fatalf("a tip at the line brought no raid: %v", evs)
	}
	if w.Stats.Betrayals != betrayals+1 || w.Rival.Trust != cfg.Rivals.Diplomacy.BetrayalFloor || len(w.Rival.Deals) != 0 {
		t.Fatalf("a tip under a truce: betrayals %d -> %d, trust %.0f, deals %v", betrayals, w.Stats.Betrayals, w.Rival.Trust, w.Rival.Deals)
	}
	if w.Home().Pressure <= pressure {
		t.Fatalf("home's pressure %.1f -> %.1f on a raid", pressure, w.Home().Pressure)
	}
}

// Determinism and the save: the same seed plays the same twice with
// every move in use, and a run saved on a morning with each move queued
// (the books read, the rival's heat up) replays the rest of the run
// exactly.
func TestBooksAreDeterministicAndSave(t *testing.T) {
	t.Setenv("KINGPIN_HOME", t.TempDir())
	cfg := content.MustLoad()
	policy := func() Policy { return Saboteur(cfg, 40) }
	play := func(days int) Result {
		w := sim.NewWorld(cfg, 4)
		w.Rival.Personality = "opportunist"
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
		switch e.(type) {
		case events.RivalScouted, events.RivalBoosted, events.PoliceTipped, events.RivalMusclePoached:
			used[e.Kind()]++
		}
	}
	if len(used) < 4 {
		t.Fatalf("the saboteur did not use every move on seed 4: %v", used)
	}
	// Save on a morning with the books read and every move queued: the
	// scratch is per-day and the policy queues it again after the load;
	// the save carries the snapshot, the rival's heat and the scouting
	// count.
	c := play(40)
	policy()(c.World)
	for (c.World.Strike == nil || c.World.Scouting == nil && c.World.Tipoff == nil) && c.World.Day < 110 {
		c = play(c.World.Day + 1)
		policy()(c.World)
	}
	if c.World.Strike == nil || !c.World.Strike.Boost {
		t.Fatalf("between day 40 and 110 of seed 4 there was never a boost queued with another move: %+v", c.World.Strike)
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
	if loaded.Rival.Known != c.World.Rival.Known || loaded.Rival.Heat != c.World.Rival.Heat || loaded.Rival.Scouted != c.World.Rival.Scouted || loaded.Rival.LastRaid != c.World.Rival.LastRaid {
		t.Fatalf("the books loaded as %+v/%.1f/%d/%d, saved %+v/%.1f/%d/%d", loaded.Rival.Known, loaded.Rival.Heat, loaded.Rival.Scouted, loaded.Rival.LastRaid, c.World.Rival.Known, c.World.Rival.Heat, c.World.Rival.Scouted, c.World.Rival.LastRaid)
	}
	d, _ := RunFrom(cfg, loaded, 120-c.World.Day, policy())
	rest := a.Events[len(c.Events):]
	if len(d.Events) != len(rest) {
		t.Fatalf("after loading, %d events for the last %d days, want %d", len(d.Events), 120-c.World.Day, len(rest))
	}
	for i := range rest {
		if fmt.Sprintf("%#v", rest[i]) != fmt.Sprintf("%#v", d.Events[i]) {
			t.Fatalf("event %d after the save differs:\n%#v\n%#v", i, rest[i], d.Events[i])
		}
	}
}

// A run that never touches the books is the old run: the territory
// player's rival keeps no heat, its books are never read and nothing of
// the books is reported, so every pinned number stands (TestMoneyCurve
// is the guard).
func TestNoBooksIsTheOldRun(t *testing.T) {
	cfg := content.MustLoad()
	res := pricewarRun(t, cfg, 1, 120, "", Territory(cfg, 40, 3), func(w *game.World) {
		if w.Rival.Heat != 0 || w.Rival.Known.Read() || w.Rival.Scouted != 0 || w.Rival.LastRaid != 0 {
			t.Fatalf("day %d: the rival's books moved with nobody at them: %+v", w.Day, w.Rival)
		}
	})
	for _, e := range res.Events {
		switch e.(type) {
		case events.RivalScouted, events.RivalBoosted, events.PoliceTipped, events.RivalRaided, events.RivalMusclePoached:
			t.Fatalf("%+v in a run that never touched the books", e)
		}
	}
}
