package harness

import (
	"fmt"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/sim"
)

// oldRunCase is one TestNo*IsTheOldRun (#276): a feature in the file
// that no policy of the case uses is byte-for-byte the run before the
// feature existed. Every policy plays every seed twice, on the file and
// on box(file), the whole world hashed (digest, TestSeedDigest's walk)
// at the end of every day, and the two lists must be the same to the
// last day; an event forbid names fails the run the day it goes out,
// and after reads each run's world at the end. Until #276 the pattern
// was written out eight times and had drifted (one seed or three, 120
// days or Horizon, subtests or not, a message naming the seed or not);
// the helper is the one copy, and a case says only what differs.
type oldRunCase struct {
	// never is what no policy of the case does, for the messages:
	// "never bought the bondsman".
	never string
	// box is the file with the feature taken off: a copy, never the
	// original (TestHarnessTestsShareNothing).
	box func(*content.Config) *content.Config
	// policies are the players, each built on the config it plays.
	policies map[string]func(*content.Config) Policy
	// seeds are played 1..seeds, days deep (a run that ends sooner is
	// compared to its end, and the other run must end the same day).
	// Each case keeps what its copy played before #276: unifying all
	// eight on three seeds to the tier-4 checkpoint doubled their CPU
	// (33.6 s to 66.5 s), for runs the digest already reads day by day.
	seeds uint64
	days  int
	// every, if over one, takes the digest every every-th day and on
	// the last day played, and reads the net worth alone on the days
	// between: the digest's reflective walk is most of what a run
	// costs (20 s of 26 s CPU for TestNoIntelIsTheOldRun's twenty
	// seeds hashed daily, 14,000 digests). A moved net worth still
	// fails on its day, any other moved field within every days of it.
	// A case with until reads daily.
	every int
	// forbid, if set, is true of an event no run of the case may emit,
	// on the file or the box.
	forbid func(events.Event) bool
	// scrub, if set, zeroes the fields the feature legitimately moves
	// on its own in a run that never uses it (a count nothing reads, the
	// morning report's text) before the day's digest, and returns what
	// puts them back; it is called on both runs, and boxed says which.
	scrub func(t *testing.T, w *game.World, boxed bool) (restore func())
	// after, if set, reads each run's world at the end.
	after func(t *testing.T, w *game.World, boxed bool)
	// until, if set, ends the comparison on the first day it is true of
	// the file's run (that day not compared): a feature that is on the
	// ground in every run from some day on (the table's second seat,
	// #43) is the old run up to that day and no further. The box then
	// plays the days the file did.
	until func(w *game.World, today []events.Event) bool
	// asRun plays both runs as harness.Run does, the deck and the
	// incident table boxed (#44): the conditions the money curve is read
	// under. Unset, the raw file plays, the deck dealt and never
	// answered and the weather on, as the eight copies before #276 did.
	asRun bool
}

// assertOldRun plays c: one parallel subtest a policy and a seed, named
// "policy/seed1", each playing the file and the box and failing on the
// first day the digests differ.
func assertOldRun(t *testing.T, c oldRunCase) {
	t.Helper()
	cfg := content.MustLoad()
	off := c.box(cfg)
	if c.seeds == 0 || c.days == 0 {
		t.Fatal("an old-run case plays no seed or no day")
	}
	if c.asRun {
		cfg, off = asRun(cfg), asRun(off)
	}
	for name, policy := range c.policies {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			for seed := uint64(1); seed <= c.seeds; seed++ {
				t.Run(fmt.Sprintf("seed%d", seed), func(t *testing.T) {
					t.Parallel()
					with, cut := oldRunDigests(t, c, cfg, policy, seed, c.days, false)
					days := c.days
					if cut {
						days = len(with)
						t.Logf("compared %d days, to the day before the feature was on the ground", days)
					}
					without, _ := oldRunDigests(t, c, off, policy, seed, days, true)
					if len(with) != len(without) {
						t.Fatalf("the file plays %d days, the box %d, in a run that %s", len(with), len(without), c.never)
					}
					for day := range with {
						if with[day] != without[day] {
							t.Fatalf("the world moved on day %d between the file and the box, in a run that %s", day+1, c.never)
						}
					}
				})
			}
		})
	}
}

// asRun is cfg with the deck and the incident table boxed, as Play
// boxes them for Run: a copy.
func asRun(cfg *content.Config) *content.Config {
	boxed := *cfg
	boxed.Dilemmas.Cards = nil
	boxed.Incidents.Table = nil
	return &boxed
}

// oldRunDigests plays one run of c on cfg and returns its daily digests,
// and whether c.until cut the file's run short.
func oldRunDigests(t *testing.T, c oldRunCase, cfg *content.Config, policy func(*content.Config) Policy, seed uint64, days int, boxed bool) ([]string, bool) {
	t.Helper()
	w := sim.NewWorld(cfg, seed)
	_, sims, err := sim.Default(cfg)
	if err != nil {
		t.Fatal(err)
	}
	clock := game.NewClock(nil, sims...)
	p := policy(cfg)
	which := "the file"
	if boxed {
		which = "the box"
	}
	var ds []string
	for day := 1; day <= days && w.Over == nil; day++ {
		p(w)
		today := clock.EndDay(w)
		for _, e := range today {
			if c.forbid != nil && c.forbid(e) {
				t.Fatalf("%s, day %d: %+v in a run that %s", which, day, e, c.never)
			}
		}
		if !boxed && c.until != nil && c.until(w, today) {
			return ds, true
		}
		switch {
		case c.every > 1 && c.until == nil && day%c.every != 0 && day != days && w.Over == nil:
			ds = append(ds, fmt.Sprintf("worth %d", w.NetWorth()))
		case c.scrub != nil:
			restore := c.scrub(t, w, boxed)
			ds = append(ds, digest(w))
			restore()
		default:
			ds = append(ds, digest(w))
		}
	}
	if c.after != nil {
		c.after(t, w, boxed)
	}
	return ds, false
}
