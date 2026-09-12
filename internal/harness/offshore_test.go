package harness

import (
	"sort"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/sim"
	"github.com/theclifmeister/kingpin/internal/sim/laundering"
)

// noOffshore is the config with the [offshore] table taken off: the
// file before #195, for the runs that pin the old run.
func noOffshore(cfg *content.Config) *content.Config {
	off := *cfg
	off.Laundering.Offshore = content.OffshoreConfig{}
	return &off
}

// TestNoReserveIsTheOldRun (#195): a run that never reserves is
// byte-for-byte the run before the account existed. The laundered
// player plays 120 days on the file and on the file with the table
// taken off, hashed daily with the quiet-day count set aside (the one
// number the table moves on its own: a count nothing but Retire reads),
// and nothing else differs; no Reserved goes out and the account stays
// empty.
func TestNoReserveIsTheOldRun(t *testing.T) {
	cfg := content.MustLoad()
	off := noOffshore(cfg)
	for seed := uint64(1); seed <= 3; seed++ {
		var with, without []string
		for i, c := range []*content.Config{cfg, off} {
			w := sim.NewWorld(c, seed)
			_, sims, err := sim.Default(c)
			if err != nil {
				t.Fatal(err)
			}
			clock := game.NewClock(nil, sims...)
			policy := Laundered(c, 40)
			var ds []string
			for day := 1; day <= 120 && w.Over == nil; day++ {
				policy(w)
				for _, e := range clock.EndDay(w) {
					if _, ok := e.(events.Reserved); ok {
						t.Fatalf("seed %d day %d: %+v in a run that never reserved", seed, day, e)
					}
				}
				quiet := w.QuietDays
				w.QuietDays = 0
				ds = append(ds, digest(w))
				w.QuietDays = quiet
			}
			if w.Offshore != 0 || w.Stats.Reserved != 0 || w.Stats.Fees != 0 || w.Laundering.Structured != (game.Structuring{}) {
				t.Fatalf("seed %d: the account moved with nobody reserving: %d, stats %d/%d, %+v", seed, w.Offshore, w.Stats.Reserved, w.Stats.Fees, w.Laundering.Structured)
			}
			if i == 0 {
				with = ds
			} else {
				without = ds
			}
		}
		for day := range with {
			if with[day] != without[day] {
				t.Fatalf("seed %d: the world moved on day %d with the account in the file and nobody reserving", seed, day+1)
			}
		}
	}
}

// reservingHider is a hider with clean cash who moves amount offshore
// on the days pick says, on a world with dirty cash to draw the
// police.
func reservingHider(t *testing.T, cfg *content.Config, seed uint64, days int, clean int, amount func(day int) int) Result {
	t.Helper()
	w := sim.NewWorld(cfg, seed)
	w.Player.DirtyCash = 5_000_000
	w.Player.CleanCash = clean
	res, err := RunFrom(cfg, w, days, func(w *game.World) {
		Hide(w)
		if amt := amount(w.Day); amt > 0 {
			_ = w.Reserve(min(amt, w.Player.CleanCash))
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	return res
}

// TestStructuringFilesPages (#195): a lot a day for a hundred days files
// nothing and moves the money less the fee; twice the lot on one day
// files structure_evidence the morning after, once; a run that reserves
// under the lot every day is a quiet one, and the rich hider who does
// (TestRichHiderIsNeverIndicted's, with the account) is never indicted
// however long they wait.
func TestStructuringFilesPages(t *testing.T) {
	cfg := content.MustLoad()
	off := cfg.Laundering.Offshore
	pages := cfg.Heat.Heat.StructureEvidence
	if off.Lot <= 0 || pages <= 0 {
		t.Fatal("no lot or no pages in the file")
	}
	ld := laundering.New(cfg)
	quiet := reservingHider(t, cfg, 1, 100, off.Lot*100, func(int) int { return off.Lot })
	if quiet.World.Heat.Evidence != 0 || quiet.World.Offshore != 100*(off.Lot-ld.Fee(off.Lot)) || quiet.World.Stats.Fees != 100*ld.Fee(off.Lot) {
		t.Fatalf("a lot a day for a hundred days: evidence %d, offshore %d, fees %d", quiet.World.Heat.Evidence, quiet.World.Offshore, quiet.World.Stats.Fees)
	}
	for _, e := range quiet.Events {
		if ev, ok := e.(events.Reserved); ok && ev.Lots != 0 {
			t.Fatalf("day %d: %d lots over the line on a lot", ev.Day, ev.Lots)
		}
	}
	lump := reservingHider(t, cfg, 1, 3, off.Lot*2, func(day int) int {
		if day == 0 {
			return off.Lot * 2
		}
		return 0
	})
	var filed []int
	var evs []events.Reserved
	for _, e := range lump.Events {
		switch ev := e.(type) {
		case events.Reserved:
			evs = append(evs, ev)
		case events.HeatChanged:
			for _, r := range ev.Reasons {
				if len(r) > 0 && r[0] == 'm' && ev.City == lump.World.Player.Location {
					filed = append(filed, ev.Day)
				}
			}
		}
	}
	if len(evs) != 1 || evs[0].Day != 1 || evs[0].Lots != 1 || lump.World.Heat.Evidence != pages || lump.World.Heat.EvidenceDay != 2 {
		t.Fatalf("twice the lot on day 0: reserved %+v, evidence %d on day %d (want %d on day 2)", evs, lump.World.Heat.Evidence, lump.World.Heat.EvidenceDay, pages)
	}
	if len(filed) != 1 || filed[0] != 2 {
		t.Fatalf("the file grew on days %v, want the morning after the move only", filed)
	}
	for seed := uint64(1); seed <= 5; seed++ {
		res := reservingHider(t, cfg, seed, 1000, off.Lot*1000, func(int) int { return off.Lot })
		if res.Over != nil || res.World.Heat.Evidence != 0 {
			t.Fatalf("seed %d: the reserving hider ended on day %d (%v) with %d evidence", seed, res.Days, res.Over, res.World.Heat.Evidence)
		}
	}
}

// TestTheAccountIsSafe (#195): an indictment with a fall guy owned
// halves the pile and leaves the account whole; an audit takes from
// the pile and never the account; and the account counts in net worth.
func TestTheAccountIsSafe(t *testing.T) {
	cfg := content.MustLoad()
	for seed := uint64(1); seed <= 3; seed++ {
		w := sim.NewWorld(cfg, seed)
		grant(w, "fallguy")
		w.Offshore = 2_000_000
		w.Player.CleanCash = 400_000
		res, _ := RunFrom(cfg, w, Horizon, Trader(cfg, events.DialAggressive))
		burned := false
		for _, e := range res.Events {
			if _, ok := e.(events.FallGuyBurned); ok {
				burned = true
			}
		}
		if !burned {
			t.Fatalf("seed %d: no fall guy burned", seed)
		}
		if res.World.Offshore != 2_000_000 {
			t.Fatalf("seed %d: the fall guy took from the account: %d", seed, res.World.Offshore)
		}
	}
	// A certain audit: the pile pays, the account does not.
	certain := *cfg
	certain.Laundering.Fronts = append([]content.FrontConfig(nil), cfg.Laundering.Fronts...)
	certain.Laundering.Fronts[0].AuditRisk = 1
	ld := laundering.New(&certain)
	w := sim.NewWorld(&certain, 1)
	fc := certain.Laundering.Fronts[0]
	w.Player.DirtyCash = fc.Cost + 1_000_000
	w.Stats.PeakCash = fc.UnlockCash
	if _, err := ld.Buy(w, fc.ID); err != nil {
		t.Fatal(err)
	}
	w.Offshore = 500_000
	w.Player.CleanCash = 100_000
	res, _ := RunFrom(&certain, w, 3, Hide)
	audited := 0
	for _, e := range res.Events {
		if ev, ok := e.(events.FrontAudited); ok && ev.Seized > 0 {
			audited++
		}
	}
	if audited == 0 || res.World.Offshore != 500_000 || res.World.Stats.Seized == 0 {
		t.Fatalf("audits %d, offshore %d, seized %d", audited, res.World.Offshore, res.World.Stats.Seized)
	}
	if got := w.NetWorth(); got < w.Offshore+w.Cash() {
		t.Fatalf("net worth %d does not count the account", got)
	}
}

// TestRetireeRetires (#195): the retiree ends retired on most of fifty
// seeds, its score the account at exit, and once it could retire, lying
// low k more days before retiring scores the same or lower (#49's
// no-day-cap rule: playing on is not rewarded), in a table.
func TestRetireeRetires(t *testing.T) {
	cfg := content.MustLoad()
	ld := laundering.New(cfg)
	retired := 0
	var scores, days []int
	for seed := uint64(1); seed <= 50; seed++ {
		res, err := Run(cfg, seed, Horizon, Retiree(cfg, 40))
		if err != nil {
			t.Fatal(err)
		}
		if res.Over != nil && res.Over.Cause == "retired" {
			retired++
			scores = append(scores, res.World.Offshore)
			days = append(days, res.Days)
			if res.World.Offshore < cfg.Laundering.Offshore.RetireCash || res.World.QuietDays < cfg.Laundering.Offshore.RetireDays {
				t.Fatalf("seed %d: retired on %d offshore and %d quiet days", seed, res.World.Offshore, res.World.QuietDays)
			}
		} else if res.Over != nil {
			// The laundered player it is built on ends the same way on
			// this seed (seed 29 is indicted on day 80 before it has
			// anything to retire on): not the account's doing.
			t.Logf("seed %d: the retiree ended %s on day %d", seed, res.Over.Cause, res.Days)
		}
	}
	sort.Ints(scores)
	sort.Ints(days)
	t.Logf("retiree: %d of 50 retired by day %d, scoring %d (median, %d..%d), on day %d (median)", retired, Horizon, scores[len(scores)/2], scores[0], scores[len(scores)-1], days[len(days)/2])
	if retired <= 25 {
		t.Errorf("the retiree retired on %d of 50 seeds; most should", retired)
	}
	// The table: on one seed, once retiring is open, k more days lying
	// low with the corners recalled, then retire.
	for _, k := range []int{0, 10, 30, 60} {
		w := sim.NewWorld(cfg, 1)
		policy := Retiree(cfg, 40)
		_, sims, err := sim.Default(cfg)
		if err != nil {
			t.Fatal(err)
		}
		clock := game.NewClock(nil, sims...)
		waited := -1
		for w.Over == nil && w.Day < 2*Horizon {
			if w.Day >= RetireAfter && ld.CanRetire(w) {
				if waited < 0 {
					waited = 0
				}
				if waited >= k {
					_ = ld.Retire(w)
					break
				}
				waited++
				for _, m := range w.Crew.Members {
					w.Recall(m.ID)
				}
				w.SetLieLow(true)
			} else {
				policy(w)
			}
			clock.EndDay(w)
		}
		if w.Over == nil || w.Over.Cause != "retired" {
			t.Fatalf("k=%d: %+v", k, w.Over)
		}
		t.Logf("k=%d: retired on day %d scoring %d", k, w.Day, w.Offshore)
		if k == 0 {
			continue
		}
		if base := scoreAt(t, cfg, 1, 0); w.Offshore > base {
			t.Errorf("lying low %d more days scored %d, over %d at once", k, w.Offshore, base)
		}
	}
}

// scoreAt is the retiree's score on seed retiring k days after it
// first could.
func scoreAt(t *testing.T, cfg *content.Config, seed uint64, k int) int {
	t.Helper()
	ld := laundering.New(cfg)
	w := sim.NewWorld(cfg, seed)
	policy := Retiree(cfg, 40)
	_, sims, err := sim.Default(cfg)
	if err != nil {
		t.Fatal(err)
	}
	clock := game.NewClock(nil, sims...)
	waited := -1
	for w.Over == nil && w.Day < 2*Horizon {
		if w.Day >= RetireAfter && ld.CanRetire(w) {
			if waited < 0 {
				waited = 0
			}
			if waited >= k {
				_ = ld.Retire(w)
				break
			}
			waited++
			for _, m := range w.Crew.Members {
				w.Recall(m.ID)
			}
			w.SetLieLow(true)
		} else {
			policy(w)
		}
		clock.EndDay(w)
	}
	return w.Offshore
}

// TestRetireIsATierFourExit (#195, the sizing): the laundered player's
// clean cash less the fee reaches retire_cash by tier 4's checkpoint
// and not by tier 3's, on the median of ten seeds; and the boss moving
// a lot a day holds under a third of its old $30M in the pile at day
// 200 (TestBossPileDrains reads the pile).
func TestRetireIsATierFourExit(t *testing.T) {
	cfg := content.MustLoad()
	off := cfg.Laundering.Offshore
	ld := laundering.New(cfg)
	reach := func(days int) int {
		var got []int
		for seed := uint64(1); seed <= 10; seed++ {
			res, err := Run(cfg, seed, days, Laundered(cfg, 40))
			if err != nil {
				t.Fatal(err)
			}
			clean := res.World.Player.CleanCash
			got = append(got, clean-ld.Fee(clean))
		}
		sort.Ints(got)
		return got[len(got)/2]
	}
	t3, t4 := reach(tierDay(3)), reach(tierDay(4))
	t.Logf("laundered's clean less the fee: %d at day %d, %d at day %d; retiring takes %d", t3, tierDay(3), t4, tierDay(4), off.RetireCash)
	if t3 >= off.RetireCash {
		t.Errorf("retire_cash %d is reachable at tier 3 (%d at day %d)", off.RetireCash, t3, tierDay(3))
	}
	if t4 < off.RetireCash {
		t.Errorf("retire_cash %d is not reachable at tier 4 (%d at day %d)", off.RetireCash, t4, tierDay(4))
	}
}

// TestTheAccountSurvivesASave (#195): the account, the quiet days and
// the last move's record come back from a save, and the run plays on
// as one that was not saved.
func TestTheAccountSurvivesASave(t *testing.T) {
	t.Setenv("KINGPIN_HOME", t.TempDir())
	cfg := content.MustLoad()
	_, sims, err := sim.Default(cfg)
	if err != nil {
		t.Fatal(err)
	}
	lot := cfg.Laundering.Offshore.Lot
	build := func() *game.World {
		w := sim.NewWorld(cfg, 5)
		w.Player.DirtyCash = 1_000_000
		w.Player.CleanCash = lot * 20
		return w
	}
	a, b := build(), build()
	ca, cb := game.NewClock(nil, sims...), game.NewClock(nil, sims...)
	for i := 0; i < 10; i++ {
		_ = a.Reserve(lot * 2)
		_ = b.Reserve(lot * 2)
		ca.EndDay(a)
		cb.EndDay(b)
	}
	if err := game.Save(1, b); err != nil {
		t.Fatal(err)
	}
	b2, err := game.Load(1)
	if err != nil {
		t.Fatal(err)
	}
	if b2.Offshore != b.Offshore || b2.Offshore == 0 || b2.QuietDays != b.QuietDays || b2.Laundering.Structured != b.Laundering.Structured || b2.Stats.Reserved != b.Stats.Reserved {
		t.Fatalf("loaded %d/%d/%+v, saved %d/%d/%+v", b2.Offshore, b2.QuietDays, b2.Laundering.Structured, b.Offshore, b.QuietDays, b.Laundering.Structured)
	}
	for i := 0; i < 10; i++ {
		ca.EndDay(a)
		cb.EndDay(b2)
	}
	if digest(a) != digest(b2) {
		t.Fatalf("the run diverged after the save: offshore %d / %d, evidence %d / %d", a.Offshore, b2.Offshore, a.Heat.Evidence, b2.Heat.Evidence)
	}
}
