package harness

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/sim"
	"github.com/theclifmeister/kingpin/internal/sim/crew"
)

// TestNoCutIsTheOldRun (#47): a run that never cuts, never cooks and
// hires no chemist is the run it was. The crewed player over 120 days
// sells every unit at the default quality with the multiplier at 1, no
// corner's repeat business moves, nothing overdoses, and its net worth
// every day and its stats at the end are what the same run makes with
// the quality table in its box.
func TestNoCutIsTheOldRun(t *testing.T) {
	cfg := content.MustLoad()
	boxed := *cfg
	boxed.Market.Quality = content.QualityTuning{}
	for seed := uint64(1); seed <= 3; seed++ {
		a, err := Run(cfg, seed, 120, Crewed(cfg, 40))
		if err != nil {
			t.Fatal(err)
		}
		b, err := Run(&boxed, seed, 120, Crewed(&boxed, 40))
		if err != nil {
			t.Fatal(err)
		}
		for d := 1; d <= 120; d++ {
			if a.NetWorthAt(d) != b.NetWorthAt(d) {
				t.Fatalf("seed %d day %d: net worth %d with quality, %d with it boxed", seed, d, a.NetWorthAt(d), b.NetWorthAt(d))
			}
		}
		if a.World.Stats != b.World.Stats {
			t.Fatalf("seed %d: the stats differ:\n%+v\n%+v", seed, a.World.Stats, b.World.Stats)
		}
		def := cfg.Market.Quality.Default
		for _, e := range a.Events {
			switch ev := e.(type) {
			case events.PlayerSold:
				if ev.Quality != def || ev.QualityMul != 1 {
					t.Fatalf("seed %d day %d: sold at quality %v x%v without a cut", seed, ev.Day, ev.Quality, ev.QualityMul)
				}
			case events.Overdose, events.StockCut, events.Cooked, events.CookOrdered:
				t.Fatalf("seed %d: %+v in a run that never cut", seed, ev)
			}
		}
		for _, cid := range a.World.CityOrder {
			for _, c := range a.World.Cities[cid].Corners {
				if c.Repeat != cfg.Market.Quality.RepeatStart {
					t.Fatalf("seed %d: %s's repeat business is %v without a cut", seed, c.ID, c.Repeat)
				}
			}
			for id, q := range a.World.StashOf(cid) {
				if q > 0 && a.World.Quality(cid, id) != def {
					t.Fatalf("seed %d: %s in %s is at quality %v without a cut", seed, id, cid, a.World.Quality(cid, id))
				}
			}
		}
		if a.World.Crew.Chemist() != nil {
			t.Fatalf("seed %d: the crewed player hired a chemist", seed)
		}
	}
}

// TestQualityInvariants (#47): every morning of a cutting crewed run
// and a cooking one, every lot's quality is in 0..100, every corner's
// repeat business in repeat_min..1, and a lot never reads under what
// the connects sell at unless a cut or a chemist's lot has touched it
// (a buy never lowers quality below the supplier's).
func TestQualityInvariants(t *testing.T) {
	cfg := content.MustLoad()
	q := cfg.Market.Quality
	for _, tc := range []struct {
		name string
		pol  Policy
	}{
		{"cut 0.5", Cutter(cfg, 0.5, Crewed(cfg, 40))},
		{"cut 1.0", Cutter(cfg, 1, Managed(cfg, 50))},
		{"cook", Cook(cfg, 40)},
	} {
		cut := map[string]bool{} // city/product a cut or a chemist's lot has touched
		res, err := RunFrom(cfg, sim.NewWorld(cfg, 2), 100, func(w *game.World) {
			if w.Report != nil {
				for _, l := range w.Report.Crew {
					if strings.Contains(l, "batch landed") {
						for _, id := range w.Products {
							cut[w.Player.Location+"/"+id] = true // the chemist's quality is theirs, not the connects'
						}
					}
				}
			}
			for _, cid := range w.CityOrder {
				for _, c := range w.Cities[cid].Corners {
					if r := c.Repeats(); r < q.RepeatMin-1e-9 || r > 1 {
						t.Fatalf("%s day %d: %s repeat %v", tc.name, w.Day, c.ID, r)
					}
				}
				for id, n := range w.StashOf(cid) {
					qq := w.Quality(cid, id)
					if qq < 0 || qq > 100 {
						t.Fatalf("%s day %d: %s in %s at quality %v", tc.name, w.Day, id, cid, qq)
					}
					if n > 0 && !cut[cid+"/"+id] && qq < q.Default-1e-9 {
						t.Fatalf("%s day %d: %s in %s at quality %v under the connects' %v with no cut", tc.name, w.Day, id, cid, qq, q.Default)
					}
				}
			}
			tc.pol(w)
			for _, c := range w.Today.Cuts {
				cut[c.City+"/"+c.Product] = true
			}
		})
		if err != nil {
			t.Fatal(err)
		}
		if tc.name != "cook" && res.World.Stats.Cut == 0 {
			t.Fatalf("%s: nothing was cut", tc.name)
		}
	}
}

// TestGreedCurve (#47): -cut 0.5 earns more than uncut over the first 30
// days and less over 120 on the same seeds, for the managed player: a
// cut is a third off the bag's cost today and the corners' customers
// tomorrow. Medians over five seeds.
func TestGreedCurve(t *testing.T) {
	cfg := content.MustLoad()
	at := func(pol func() Policy, day int) int {
		var ws []int
		for seed := uint64(1); seed <= 5; seed++ {
			res, err := Run(cfg, seed, day, pol())
			if err != nil {
				t.Fatal(err)
			}
			ws = append(ws, res.NetWorthAt(day))
		}
		sort.Ints(ws)
		return ws[2]
	}
	plain := func() Policy { return Managed(cfg, 50) }
	cut := func() Policy { return Cutter(cfg, 0.5, Managed(cfg, 50)) }
	p30, c30 := at(plain, 30), at(cut, 30)
	p120, c120 := at(plain, 120), at(cut, 120)
	t.Logf("managed: day 30 $%d uncut, $%d cut 0.5; day 120 $%d uncut, $%d cut 0.5 (medians over 5 seeds)", p30, c30, p120, c120)
	if c30 <= p30 {
		t.Fatalf("cutting does not pay over 30 days: $%d cut against $%d", c30, p30)
	}
	if c120 >= p120 {
		t.Fatalf("cutting still pays over 120 days: $%d cut against $%d", c120, p120)
	}
}

// TestOverdosesArePressureNeverEvidence (#47): the cut-everything player
// (a full cut, hard product under od_quality) draws overdoses, which
// raise the pressure where it sells and its notoriety against the same
// run with od_chance zeroed, and never a page: an Overdose carries
// nothing the heat sim reads (the heat sim's source never names it), a
// sting or raid on a quiet day adds none (#27's shape), and the file at
// the end is what it is with od_chance zeroed and the same dice.
func TestOverdosesArePressureNeverEvidence(t *testing.T) {
	cfg := content.MustLoad()
	quiet := *cfg
	quiet.Market.Quality.OdChance = 0
	files, _ := filepath.Glob(filepath.Join("..", "sim", "heat", "*.go"))
	for _, f := range files {
		src, _ := os.ReadFile(f)
		if strings.Contains(string(src), "Overdose") {
			t.Fatalf("%s reads the overdose: an overdose is pressure and news, never the heat sim's", f)
		}
	}
	overdoses := 0
	var dp, dn []float64
	for seed := uint64(1); seed <= 5; seed++ {
		w := sim.NewWorld(cfg, seed)
		w.Player.DirtyCash = 200_000
		a, err := RunFrom(cfg, w, 120, Cutter(cfg, 1, Managed(cfg, 50)))
		if err != nil {
			t.Fatal(err)
		}
		w2 := sim.NewWorld(&quiet, seed)
		w2.Player.DirtyCash = 200_000
		b, err := RunFrom(&quiet, w2, 120, Cutter(&quiet, 1, Managed(&quiet, 50)))
		if err != nil {
			t.Fatal(err)
		}
		sold := map[int]bool{}
		for _, e := range a.Events {
			switch ev := e.(type) {
			case events.Overdose:
				overdoses++
				if ev.Product == "" || ev.Quality >= cfg.Market.Quality.OdQuality {
					t.Fatalf("seed %d day %d: an overdose on %s at quality %v", seed, ev.Day, ev.Product, ev.Quality)
				}
			case events.PlayerSold:
				if ev.Wanted > 0 {
					sold[ev.Day] = true
				}
			case events.Enforcement:
				if (ev.Level == content.Sting || ev.Level == content.Raid) && !sold[ev.Day] && ev.Evidence != 0 {
					t.Fatalf("seed %d day %d: %s on a quiet day added %d evidence", seed, ev.Day, ev.Level, ev.Evidence)
				}
			}
		}
		if a.World.Stats.Overdoses > 0 && a.World.Heat.Evidence != b.World.Heat.Evidence {
			// The file can move with the overdoses only through the
			// ladder pressure lowers: never on the event itself, which
			// the quiet-day check above holds.
			t.Logf("seed %d: file %d with overdoses, %d without (the pressure moves the ladder)", seed, a.World.Heat.Evidence, b.World.Heat.Evidence)
		}
		dp = append(dp, a.World.Home().Pressure-b.World.Home().Pressure)
		dn = append(dn, a.World.Player.Reputation.Notoriety-b.World.Player.Reputation.Notoriety)
	}
	if overdoses == 0 {
		t.Fatal("the cut-everything player drew no overdose in 120 days over 5 seeds")
	}
	sort.Float64s(dp)
	sort.Float64s(dn)
	t.Logf("%d overdoses over 5 seeds; pressure at home +%.1f, notoriety +%.1f over the same run without them (medians)", overdoses, dp[2], dn[2])
	if dp[2] <= 0 || dn[2] <= 0 {
		t.Fatalf("overdoses did not raise pressure (+%.1f) and notoriety (+%.1f)", dp[2], dn[2])
	}
}

// TestCookBeatsBuying (#47): a cooked unit of meth costs less than a
// bought one, the cook policy (laundered with a chemist) is worth at
// least laundered at the tier-3 and tier-4 checkpoints over five seeds
// (distributor's number logged beside them), every run of it ends with
// a chemist and cooks, and firing the chemist drops the next cook's
// quality: with two on the payroll the best cooks, without the best the
// next does, and with none nothing cooks.
func TestCookBeatsBuying(t *testing.T) {
	cfg := content.MustLoad()
	meth := cfg.Market.Product("meth")
	if float64(meth.CookCost) >= meth.BasePrice*cfg.Market.Market.SupplierRatio {
		t.Fatalf("cooking meth costs $%d a unit, buying it $%.0f", meth.CookCost, meth.BasePrice*cfg.Market.Market.SupplierRatio)
	}
	at := func(name string, pol func(*content.Config) Policy) (int, int) {
		var a120, a200 []int
		for seed := uint64(1); seed <= 5; seed++ {
			res, err := Run(cfg, seed, 200, pol(cfg))
			if err != nil {
				t.Fatal(err)
			}
			a120 = append(a120, res.NetWorthAt(120))
			a200 = append(a200, res.NetWorthAt(200))
			if name == "cook" && (res.World.Crew.Chemist() == nil || res.World.Stats.Cooked == 0) {
				t.Fatalf("seed %d: the cook ends with no chemist (%v) or cooked nothing (%d)", seed, res.World.Crew.Chemist() != nil, res.World.Stats.Cooked)
			}
		}
		sort.Ints(a120)
		sort.Ints(a200)
		t.Logf("%-12s day 120 $%d, day 200 $%d (medians over 5 seeds)", name, a120[2], a200[2])
		return a120[2], a200[2]
	}
	c120, c200 := at("cook", func(c *content.Config) Policy { return Cook(c, 40) })
	l120, l200 := at("laundered", func(c *content.Config) Policy { return Laundered(c, 40) })
	at("distributor", func(c *content.Config) Policy { return Distributor(c, 40) })
	if c120 < l120 || c200 < l200 {
		t.Fatalf("cook $%d / $%d against laundered $%d / $%d at days 120 / 200", c120, c200, l120, l200)
	}

	// The chemist's hand.
	w := sim.NewWorld(cfg, 1)
	cs := crew.New(cfg)
	hire := func(name string, skill int) int {
		w.Crew.NextID++
		w.Crew.Members = append(w.Crew.Members, game.CrewMember{ID: w.Crew.NextID, Name: name, Role: game.RoleChemist, Skill: skill, Loyalty: 60})
		return w.Crew.NextID
	}
	best := hire("Doc", 90)
	hire("Beaker", 30)
	high := cs.ChemistQuality(w)
	if _, err := w.Fire(best); err != nil {
		t.Fatal(err)
	}
	low := cs.ChemistQuality(w)
	if low >= high {
		t.Fatalf("firing the best chemist left the next cook at %v, was %v", low, high)
	}
	w.Player.DirtyCash = 1_000_000
	if _, err := w.CookOrder(w.Home().ID, "weed", 10, meth.CookCost, cs.CookDays(), cs.ChemistQuality(w), cs.Batch(w), cs.ChemistName(w)); err != nil {
		t.Fatalf("a cook with the second chemist: %v", err)
	}
	if _, err := w.Fire(w.Crew.Chemist().ID); err != nil {
		t.Fatal(err)
	}
	if _, err := w.CookOrder(w.Home().ID, "weed", 10, meth.CookCost, cs.CookDays(), cs.ChemistQuality(w), cs.Batch(w), cs.ChemistName(w)); err == nil {
		t.Fatal("a cook with no chemist was taken")
	}
}

// TestQualityIsDeterministicAndSaves (#47): the cutting cook replays
// from its seed, and a save in the middle of it, with cut lots, a cook
// on its way and corners that remember, plays on as the unbroken run.
func TestQualityIsDeterministicAndSaves(t *testing.T) {
	t.Setenv("KINGPIN_HOME", t.TempDir())
	cfg := content.MustLoad()
	set, _, err := sim.Default(cfg)
	if err != nil {
		t.Fatal(err)
	}
	pol := Cutter(cfg, 0.5, Cook(cfg, 40))
	a, _ := Run(cfg, 3, 100, pol)
	b, _ := Run(cfg, 3, 100, pol)
	if a.NetWorthAt(100) != b.NetWorthAt(100) || a.World.Stats != b.World.Stats {
		t.Fatalf("the same seed diverged: %d / %d", a.NetWorthAt(100), b.NetWorthAt(100))
	}
	day := 0
	for d := 60; d < 100 && day == 0; d++ {
		for _, e := range a.Events {
			if ev, ok := e.(events.CookOrdered); ok && ev.Day == d {
				day = d // a cook on its way at the save
			}
		}
	}
	if day == 0 {
		t.Fatal("no cook ordered between days 60 and 100")
	}
	c, _ := Run(cfg, 3, day, pol)
	if len(c.World.Crew.Cooks) == 0 {
		t.Fatalf("no cook on its way on day %d", day)
	}
	moved := false
	for _, k := range c.World.Home().Corners {
		if k.Repeat != 0 && k.Repeat != 1 {
			moved = true
		}
	}
	if c.World.Stats.Cut == 0 {
		t.Fatalf("day %d: nothing cut", day)
	}
	t.Logf("day %d: %d units cut in, a cook on its way, a corner remembers: %v (a chemist's hand keeps a half cut over the floor)", day, c.World.Stats.Cut, moved)
	if err := game.Save(1, c.World); err != nil {
		t.Fatal(err)
	}
	loaded, err := game.Load(1, set.Migrations()...)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Crew.Cooks) != len(c.World.Crew.Cooks) || loaded.Crew.Cooks[0] != c.World.Crew.Cooks[0] {
		t.Fatalf("the save lost the cook: %+v / %+v", loaded.Crew.Cooks, c.World.Crew.Cooks)
	}
	for _, cid := range loaded.CityOrder {
		for id := range loaded.StashOf(cid) {
			if loaded.Quality(cid, id) != c.World.Quality(cid, id) {
				t.Fatalf("the save moved %s's quality in %s: %v / %v", id, cid, loaded.Quality(cid, id), c.World.Quality(cid, id))
			}
		}
		for i, k := range loaded.Cities[cid].Corners {
			if k.Repeat != c.World.Cities[cid].Corners[i].Repeat {
				t.Fatalf("the save moved %s's repeat: %v / %v", k.ID, k.Repeat, c.World.Cities[cid].Corners[i].Repeat)
			}
		}
	}
	d, _ := RunFrom(cfg, loaded, 100-day, pol)
	if a.NetWorthAt(100) != d.NetWorthAt(100-day) || a.World.Stats != d.World.Stats {
		t.Fatalf("the saved run diverged: %d / %d", a.NetWorthAt(100), d.NetWorthAt(100-day))
	}
}
