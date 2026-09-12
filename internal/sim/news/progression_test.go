package news_test

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/harness"
	"github.com/theclifmeister/kingpin/internal/sim"
)

// reachedDays reads the TierReached events of a run: tier -> the day.
func reachedDays(res harness.Result) map[int]int {
	out := map[int]int{}
	for _, e := range res.Events {
		if ev, ok := e.(events.TierReached); ok {
			if _, dup := out[ev.Tier]; dup {
				panic(fmt.Sprintf("tier %d reached twice, day %d and %d", ev.Tier, out[ev.Tier], ev.Day))
			}
			out[ev.Tier] = ev.Day
		}
	}
	return out
}

// The tiers (#147) are entered in order and never lost: the boss enters
// every tier over five seeds, each after the one before, the idle and
// the lone managed player none past the first (the lines past the first
// keep the crew in them). The days are the gates': the tier-3 morning is
// the first the news sim reads the laundromat's line off the peak, the
// morning after the ledger first reads `open to you`, and the tier-4
// morning is the wholesaler's SupplierUnlocked, on the same seeds.
func TestTiersAreOrdered(t *testing.T) {
	cfg := content.MustLoad()
	if len(cfg.Progression.Tiers) != 4 {
		t.Fatalf("%d tiers in the file, want 4 (tier 5 arrives with #48)", len(cfg.Progression.Tiers))
	}
	laundromat := cfg.Laundering.Front("laundromat")
	if laundromat == nil {
		t.Fatal("no laundromat in laundering.toml")
	}
	for seed := uint64(1); seed <= 5; seed++ {
		open := -1 // the first morning the laundromat reads open to you
		policy := harness.Boss(cfg, 40, "")
		res, err := harness.Run(cfg, seed, 120, func(w *game.World) {
			if open < 0 && !(game.FrontOffer{UnlockCash: laundromat.UnlockCash}).Locked(w) {
				open = w.Day
			}
			policy(w)
		})
		if err != nil {
			t.Fatal(err)
		}
		w := res.World
		days := reachedDays(res)
		if w.Tier() != 4 || len(days) != 3 {
			t.Fatalf("seed %d: boss at tier %d on day %d, reached %v; want every tier entered", seed, w.Tier(), w.Day, days)
		}
		for n := 2; n <= 4; n++ {
			if days[n] != w.ReachedOn(n) {
				t.Fatalf("seed %d: tier %d reached on day %d by the event, %d on the world", seed, n, days[n], w.ReachedOn(n))
			}
			if n > 2 && days[n] <= days[n-1] {
				t.Fatalf("seed %d: tier %d on day %d, tier %d on day %d; tiers are entered in order, one a morning", seed, n-1, days[n-1], n, days[n])
			}
		}
		if open < 0 || days[3] != open+1 {
			t.Fatalf("seed %d: Territory reached on day %d; the laundromat read open to you on the morning of day %d, so the news sim reads its line the next morning", seed, days[3], open)
		}
		unlocked := -1
		for _, e := range res.Events {
			if ev, ok := e.(events.SupplierUnlocked); ok && ev.Supplier == "dutchman" {
				unlocked = ev.Day
			}
		}
		if unlocked < 0 || days[4] != unlocked {
			t.Fatalf("seed %d: Distribution reached on day %d, the Dutchman dealt from day %d; want the same morning", seed, days[4], unlocked)
		}
		t.Logf("seed %d: Crew d%d · Territory d%d (laundromat open d%d) · Distribution d%d (wholesaler d%d)", seed, days[2], days[3], open, days[4], unlocked)
	}
	for _, p := range []struct {
		name   string
		policy harness.Policy
	}{{"idle", harness.Idle}, {"managed", harness.Managed(cfg, 50)}} {
		res, err := harness.Run(cfg, 3, 120, p.policy)
		if err != nil {
			t.Fatal(err)
		}
		if res.World.Tier() != 1 || len(reachedDays(res)) != 0 || res.World.TierName(cfg.Progression) != "Corner" {
			t.Fatalf("%s: tier %d (%s) on day %d, reached %v; a player who never hires stays a Corner trader", p.name, res.World.Tier(), res.World.TierName(cfg.Progression), res.World.Day, reachedDays(res))
		}
	}
}

// A save on the morning a tier is entered reloads with the tier reached
// and the event not emitted again; the world runs on as the unsaved run
// does. A save from before the progression (Reached empty) catches up
// on its first mornings, one tier a morning, the highest last, and the
// world after N days is otherwise identical: the tier is read, never
// acted on.
func TestProgressionIsDeterministicAndSaves(t *testing.T) {
	t.Setenv("KINGPIN_HOME", t.TempDir())
	cfg := content.MustLoad()
	const seed, horizon = 2, 90
	straight, err := harness.Run(cfg, seed, horizon, harness.Boss(cfg, 40, ""))
	if err != nil {
		t.Fatal(err)
	}
	again, _ := harness.Run(cfg, seed, horizon, harness.Boss(cfg, 40, ""))
	if !reflect.DeepEqual(straight.World.Progression, again.World.Progression) || len(straight.Events) != len(again.Events) {
		t.Fatalf("two runs on seed %d differ: %v and %v", seed, straight.World.Progression, again.World.Progression)
	}
	days := reachedDays(straight)
	day3 := days[3]
	if day3 <= 0 || day3 >= horizon-10 {
		t.Fatalf("Territory reached on day %d; the test wants it inside the horizon", day3)
	}

	// Save on the morning Territory is entered: the tier is on the world
	// and the loaded run emits no second TierReached for it.
	part, _ := harness.Run(cfg, seed, day3, harness.Boss(cfg, 40, ""))
	if part.World.ReachedOn(3) != day3 {
		t.Fatalf("on the morning of day %d the world reads Territory on day %d", day3, part.World.ReachedOn(3))
	}
	if err := game.Save(1, part.World); err != nil {
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
	if !reflect.DeepEqual(loaded.Progression, part.World.Progression) {
		t.Fatalf("loaded %v, saved %v", loaded.Progression, part.World.Progression)
	}
	rest, _ := harness.RunFrom(cfg, loaded, horizon-day3, harness.Boss(cfg, 40, ""))
	for tier, d := range reachedDays(rest) {
		if tier <= 3 {
			t.Fatalf("after the save tier %d was reached again on day %d", tier, d)
		}
	}
	if !reflect.DeepEqual(rest.World.Progression, straight.World.Progression) || rest.World.NetWorth() != straight.World.NetWorth() {
		t.Fatalf("after the save %v and net worth %d, straight through %v and %d", rest.World.Progression, rest.World.NetWorth(), straight.World.Progression, straight.World.NetWorth())
	}

	// A pre-progression save: the same world with nothing reached
	// catches up one tier a morning, and is otherwise the same world.
	old, _ := harness.Run(cfg, seed, day3+5, harness.Boss(cfg, 40, ""))
	old.World.Progression = game.Progression{}
	if err := game.Save(2, old.World); err != nil {
		t.Fatal(err)
	}
	loaded, err = game.Load(2, set.Migrations()...)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Tier() != 1 || loaded.Progression.Reached != nil {
		t.Fatalf("a save with nothing reached loads at tier %d with %v", loaded.Tier(), loaded.Progression.Reached)
	}
	caught, _ := harness.RunFrom(cfg, loaded, horizon-day3-5, harness.Boss(cfg, 40, ""))
	got := reachedDays(caught)
	if got[2] != day3+6 || got[3] != day3+7 {
		t.Fatalf("the old save caught up %v from day %d; want Crew on the first morning and Territory on the second", got, day3+5)
	}
	if got[4] != days[4] {
		t.Fatalf("the old save reached Distribution on day %d, the straight run on day %d", got[4], days[4])
	}
	if caught.World.Tier() != straight.World.Tier() {
		t.Fatalf("the old save is at tier %d, the straight run at %d", caught.World.Tier(), straight.World.Tier())
	}
	a, b := *straight.World, *caught.World
	for _, w := range []*game.World{&a, &b} {
		w.Progression, w.Journal, w.Report = game.Progression{}, nil, nil
	}
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("the world after the old save's catch-up differs from the straight run beyond the tiers and the paper")
	}
}

// The tier stamp adds no dice: a run's events, its cards aside, are the
// pre-#147 run's, and the headline the tier makes picks its template off
// the progression's own stream. The stamp is one a morning: a world that
// holds every tier's trigger on day 0 enters them on days 1, 2 and 3.
func TestTiersOneAMorning(t *testing.T) {
	cfg := content.MustLoad()
	w := sim.NewWorld(cfg, 7)
	w.Player.DirtyCash = 600_000
	w.Stats.PeakCash = 600_000
	res, err := harness.RunFrom(cfg, w, 5, harness.Crewed(cfg, 40))
	if err != nil {
		t.Fatal(err)
	}
	days := reachedDays(res)
	first := days[2]
	if first < 1 || days[3] != first+1 || days[4] != first+2 {
		t.Fatalf("a world past every line reached %v; want one tier a morning", days)
	}
	// The morning's headline is one of the TierReached templates, in the
	// flavour's source.
	found := false
	for _, l := range res.World.Journal {
		if l.Source != "news" || l.Day != first {
			continue
		}
		for _, tmpl := range cfg.Headlines.Templates["TierReached"] {
			if lead, _, _ := strings.Cut(tmpl, "{{"); strings.HasPrefix(l.Text, lead) {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("no TierReached headline on day %d, the morning Crew was reached", first)
	}
}
