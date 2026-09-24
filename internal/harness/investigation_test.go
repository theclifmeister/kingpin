package harness

import (
	"sort"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
)

// Targeted investigations (#343, heat.toml [investigation], on in the
// file, switched with Investigations): the police name an
// operation before the sting, and the player suspends it, moves it or
// lets them have it.

// With investigations off, every policy plays the run the numbers in
// the table cannot touch: the table zeroed is the same run, day by day
// to the horizon, no investigation opens or closes, and nothing is
// tallied. Off is the blind sting, the run before the feature (the
// digest's history in TestSeedDigest says so of the file's numbers).
func TestNoInvestigationIsTheOldRun(t *testing.T) {
	t.Parallel()
	policies := map[string]func(*content.Config) Policy{}
	for _, p := range Policies {
		policies[p.Name] = func(c *content.Config) Policy { return p.Make(c, DefaultPolicyOpts()) }
	}
	assertOldRun(t, oldRunCase{
		never: "played with investigations off",
		file:  func(cfg *content.Config) *content.Config { return Investigations(cfg, false) },
		box: func(cfg *content.Config) *content.Config {
			boxed := *cfg
			boxed.Heat.Investigation = content.InvestigationTuning{}
			return &boxed
		},
		policies: policies,
		seeds:    1,
		days:     Horizon,
		every:    10,
		forbid: func(e events.Event) bool {
			switch e.(type) {
			case events.InvestigationOpened, events.InvestigationClosed:
				return true
			}
			return false
		},
		after: func(t *testing.T, w *game.World, _ bool) {
			if w.Heat.Trail != nil || w.Heat.Investigation != (game.Investigation{}) {
				t.Fatalf("off, the trail is %v and the investigation %+v", w.Heat.Trail, w.Heat.Investigation)
			}
		},
	})
}

// medianWorth is the median net worth on day over seeds 1..n.
func medianWorth(t *testing.T, cfg *content.Config, policy func(*content.Config) Policy, n uint64, day int) int {
	t.Helper()
	var worths []int
	for seed := uint64(1); seed <= n; seed++ {
		res, err := Run(cfg, seed, day, policy(cfg))
		if err != nil {
			t.Fatal(err)
		}
		worths = append(worths, res.NetWorthAt(day))
	}
	sort.Ints(worths)
	return worths[len(worths)/2]
}

// The issue's acceptance, in numbers: lying low is no longer the best
// answer to every warning. The surgeon, who answers each investigation
// by walking off the named corner and never lies low, ends the
// Distribution tier with more net worth than the managed player who
// lies low at 50, on the median of twenty seeds; and a player who
// ignores every warning (normal: never lies low, never moves) does no
// better with investigations on than with the blind sting.
func TestSurgeonBeatsManaged(t *testing.T) {
	t.Parallel()
	cfg := Investigations(content.MustLoad(), true)
	day := TierDays[3]
	surgeon := medianWorth(t, cfg, func(c *content.Config) Policy { return Surgeon(c, SurgeonLine) }, 20, day)
	managed := medianWorth(t, cfg, func(c *content.Config) Policy { return Managed(c, 50) }, 20, day)
	t.Logf("day %d medians: surgeon %d, managed %d", day, surgeon, managed)
	if surgeon <= managed {
		t.Errorf("the surgeon ends day %d on %d, managed on %d: answering the investigation should beat lying low", day, surgeon, managed)
	}
	normal := func(c *content.Config) Policy { return Trader(c, events.DialNormal) }
	on := medianWorth(t, Investigations(cfg, true), normal, 20, day)
	off := medianWorth(t, Investigations(cfg, false), normal, 20, day)
	t.Logf("day %d medians for the player who ignores warnings: %d with investigations, %d with the blind sting", day, on, off)
	if on > off {
		t.Errorf("ignoring every warning ends day %d on %d with investigations, %d without: it should do no better", day, on, off)
	}
}

// The surgeon meets investigations and answers them: on a pinned seed
// investigations open and every one that lands misses, so no page is
// filed by one; and it never lies low.
func TestSurgeonAnswersEveryInvestigation(t *testing.T) {
	t.Parallel()
	cfg := Investigations(content.MustLoad(), true)
	opened, hits := 0, 0
	for seed := uint64(1); seed <= 5; seed++ {
		surgeon := Surgeon(cfg, SurgeonLine)
		res, err := Run(cfg, seed, TierDays[3], func(w *game.World) {
			surgeon(w)
			if w.Today.LieLow {
				t.Fatalf("seed %d day %d: the surgeon lay low", seed, w.Day)
			}
		})
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range res.Events {
			switch ev := e.(type) {
			case events.InvestigationOpened:
				opened++
			case events.InvestigationClosed:
				if ev.Hit {
					hits++
				}
			}
		}
	}
	t.Logf("five seeds: %d investigations opened, %d hit", opened, hits)
	if opened == 0 || hits != 0 {
		t.Fatalf("five seeds: %d investigations opened, %d hit; want some, and none hitting", opened, hits)
	}
}
