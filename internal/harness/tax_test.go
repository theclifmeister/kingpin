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
// laundered and the crewed players are hashed daily to the tier-4
// checkpoint on the file and on the file with cut at 0, one seed each,
// identical, nothing taxed; the jitter rolls on the tax's own stream,
// so the home stream never moves.
func TestNoTaxIsTheOldRun(t *testing.T) {
	t.Parallel()
	cfg := content.MustLoad()
	off := NoTax(cfg)
	for name, policy := range map[string]func(*content.Config) Policy{
		"boss":      func(c *content.Config) Policy { return Boss(c, 40, "") },
		"laundered": func(c *content.Config) Policy { return Laundered(c, 40) },
		"crewed":    func(c *content.Config) Policy { return Crewed(c, 40) },
	} {
		name, policy := name, policy
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			var with, without []string
			for i, c := range []*content.Config{cfg, off} {
				w := sim.NewWorld(c, 1)
				_, sims, err := sim.Default(c)
				if err != nil {
					t.Fatal(err)
				}
				clock := game.NewClock(nil, sims...)
				p := policy(c)
				var ds []string
				for day := 1; day <= Horizon && w.Over == nil; day++ {
					p(w)
					for _, e := range clock.EndDay(w) {
						if ev, ok := e.(events.Taxed); ok {
							t.Fatalf("%s day %d: %+v in a run that never held the share", name, day, ev)
						}
					}
					ds = append(ds, digest(w))
				}
				if w.Stats.Taxed != 0 {
					t.Fatalf("%s: taxed %d with the share never held", name, w.Stats.Taxed)
				}
				if i == 0 {
					with = ds
				} else {
					without = ds
				}
			}
			for day := range with {
				if day >= len(without) || with[day] != without[day] {
					t.Fatalf("%s: the world moved on day %d with the tax in the file and the share never held", name, day+1)
				}
			}
		})
	}
}

// TestTaxAtTierFive (#231): the tier-5 band ($100M-$1B, #205) re-read
// with the tax in the file, the boss and the cartel on ten seeds at day
// 300, the medians logged with what the tax paid; neither leaves the
// band. The goal #223 set, cartel over boss on the median, is logged,
// not pinned: the tax rewards holding the city, which the boss on a
// seed in fifty does by day 211 with the weather on and the harness
// boxes.
func TestTaxAtTierFive(t *testing.T) {
	t.Parallel()
	cfg := content.MustLoad()
	day := tierDay(5)
	for _, row := range []struct {
		name   string
		policy func(*content.Config) Policy
	}{
		{"boss", func(c *content.Config) Policy { return Boss(c, 40, "") }},
		{"cartel", func(c *content.Config) Policy { return Cartel(c, 40) }},
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
		if worths[5] < 100_000_000 || worths[5] > 1_000_000_000 {
			t.Errorf("tier 5: %s median net worth on day %d is %d, want $100M..$1B", row.name, day, worths[5])
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
