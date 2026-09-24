package harness

import (
	"sort"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/sim"
)

// NoTax returns a copy of cfg with the tax boxed (#231): cut at 0, so
// no free corner pays.
func NoTax(cfg *content.Config) *content.Config {
	boxed := *cfg
	boxed.City.Tax.Cut = 0
	return &boxed
}

// TestNoTaxIsTheOldRun (#231): a run in which the share never holds
// is byte-for-byte the run before the tax existed. The boss, the
// laundered and the crewed players are hashed daily on the file and on
// the file with cut at 0 (assertOldRun: one seed to the tier-4
// checkpoint), identical, nothing taxed; the jitter rolls on the tax's
// own stream, so the home stream never moves.
func TestNoTaxIsTheOldRun(t *testing.T) {
	t.Parallel()
	assertOldRun(t, oldRunCase{
		never: "never held the share",
		seeds: 1,
		days:  Horizon,
		box:   NoTax,
		policies: map[string]func(*content.Config) Policy{
			"boss":      func(c *content.Config) Policy { return Boss(c, 40, "") },
			"laundered": func(c *content.Config) Policy { return Laundered(c, 40) },
			"crewed":    func(c *content.Config) Policy { return Crewed(c, 40) },
		},
		forbid: func(e events.Event) bool {
			_, ok := e.(events.Taxed)
			return ok
		},
		after: func(t *testing.T, w *game.World, _ bool) {
			if w.Stats.Taxed != 0 {
				t.Fatalf("taxed %d with the share never held", w.Stats.Taxed)
			}
		},
	})
}

// TestTaxAtTierFive (#231): the tier-5 bands re-read with the tax in
// the file, the boss and the cartel on ten seeds at day 300, the
// medians logged with what the tax paid: the boss in $100M-$1B (#205),
// the cartel in $1B-$5B since its lanes abroad (#391). The tax rewards
// holding the city, which the boss on a seed in fifty does by day 211
// with the weather on and the harness boxes.
func TestTaxAtTierFive(t *testing.T) {
	t.Parallel()
	cfg := content.MustLoad()
	day := tierDay(5)
	for _, row := range []struct {
		name   string
		policy func(*content.Config) Policy
		lo, hi int
	}{
		{"boss", func(c *content.Config) Policy { return Boss(c, 40, "") }, 100_000_000, 1_000_000_000},
		// The cartel's lanes abroad (#391) are its band: $1B-$5B.
		{"cartel", func(c *content.Config) Policy { return Cartel(c, 40) }, 1_000_000_000, 5_000_000_000},
	} {
		var worths, taxed []int
		for seed := uint64(1); seed <= 10; seed++ {
			res, err := Run(cfg, seed, day, row.policy(cfg))
			if err != nil {
				t.Fatal(err)
			}
			worths = append(worths, res.NetWorthAt(day))
			taxed = append(taxed, res.World.Stats.Taxed)
		}
		sort.Ints(worths)
		sort.Ints(taxed)
		t.Logf("tier 5: %s median net worth on day %d is %d (%d..%d), taxed %d (median) over ten seeds", row.name, day, worths[5], worths[0], worths[9], taxed[5])
		if worths[5] < row.lo || worths[5] > row.hi {
			t.Errorf("tier 5: %s median net worth on day %d is %d, want %d..%d", row.name, day, worths[5], row.lo, row.hi)
		}
	}
}

// TestTaxPaysTheReign (#231): on the kingpin scenario (every faction
// fallen, six of home's ten corners held with a runner on each, the
// crown never taken) the four free corners pay every night: Taxed each
// morning, the cash in the till, no heat and no page for it, and the
// corner a faction would set up on left out.
func TestTaxPaysTheReign(t *testing.T) {
	t.Parallel()
	var sc scenario
	for _, s := range scenarios() {
		if s.cause == content.CauseKingpin {
			sc = s
		}
	}
	cfg := sc.cfg(content.MustLoad())
	w := sim.NewWorld(cfg, 1)
	sc.world(cfg, w)
	nights, corners := 0, 0
	res, err := RunFrom(cfg, w, 30, func(w *game.World) {
		if w.Heat.Evidence != 0 {
			t.Fatalf("day %d: a page in the file with nothing sold", w.Day)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range res.Events {
		if ev, ok := e.(events.Taxed); ok {
			nights++
			corners = ev.Corners
			if ev.City != w.Home().ID || ev.Amount <= 0 {
				t.Fatalf("%+v", ev)
			}
		}
	}
	if nights != 30 || corners != 4 || res.World.Stats.Taxed <= 0 {
		t.Fatalf("taxed on %d nights, %d corners the last, %d in all", nights, corners, res.World.Stats.Taxed)
	}
	t.Logf("the reign's tax: %d over 30 nights off %d free corners", res.World.Stats.Taxed, corners)
}
