package laundering_test

import (
	"math"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/sim/laundering"
)

// levelProbe reads every number a level can move off the first front
// at the given level, with the world holding enough dirty cash to wash
// the front's full throughput: what it washes, earns, costs, its audit
// risk before the day (the ledger's, read at today's throughput), and
// the seizure and freeze of an audit.
func levelProbe(t *testing.T, cfg *content.Config, level int) map[string]float64 {
	t.Helper()
	fc := cfg.Laundering.Fronts[0]
	tun := cfg.Laundering.Laundering
	w := world(fc.Cost + tun.Float + fc.Throughput*100)
	s := laundering.New(cfg)
	if _, err := s.Buy(w, fc.ID); err != nil {
		t.Fatal(err)
	}
	w.Fronts[0].Level = level
	f := w.Fronts[0]
	return map[string]float64{
		"throughput": float64(s.Throughput(w, f)),
		"income":     float64(s.Income(f)),
		"upkeep":     float64(s.FrontUpkeep(w, f)),
		"audit_risk": s.AuditRisk(w, f),
		"next_cost":  float64(s.LevelCost(f, 1)),
		"float":      float64(s.Float(w)),
		"freeze":     float64(s.AuditFreezeDays(w)),
	}
}

// TestLevelsPullTheirWay (#192): a level moves a front's income,
// throughput, upkeep, audit risk and the next level's price by the
// file's numbers and nothing else; level zero reads the file exactly,
// so a run that never invests is the run before; and legit_ratio raises
// the risk of a big washer with a token level and not of one whose
// income covers its wash.
func TestLevelsPullTheirWay(t *testing.T) {
	cfg := content.MustLoad()
	fc := cfg.Laundering.Fronts[0]
	g := cfg.Laundering.Growth
	base := levelProbe(t, cfg, 0)
	if base["throughput"] != float64(fc.Throughput) || base["upkeep"] != float64(fc.Upkeep) || base["audit_risk"] != fc.AuditRisk || base["income"] != 0 || base["next_cost"] != float64(fc.LevelCost) {
		t.Fatalf("level 0 does not read the file: %v against %+v", base, fc)
	}
	near := func(got, want float64) bool { return math.Abs(got-want) <= 0.51 }
	for level := 1; level <= fc.MaxLevel; level++ {
		got := levelProbe(t, cfg, level)
		mul := math.Pow(fc.LevelMul, float64(level))
		income, cost := 0.0, 0.0
		for k := 0; k < level; k++ {
			income += math.Round(float64(fc.Income) * math.Pow(fc.LevelMul, float64(k)))
		}
		cost = math.Round(float64(fc.LevelCost) * mul)
		want := map[string]float64{
			"throughput": math.Round(float64(fc.Throughput) * mul),
			"income":     income,
			"upkeep":     math.Round(float64(fc.Upkeep) * mul),
			"next_cost":  cost,
			"float":      base["float"],
			"freeze":     base["freeze"],
		}
		for k, v := range want {
			if !near(got[k], v) {
				t.Errorf("level %d: %s is %v, want %v (base %v)", level, k, got[k], v, base[k])
			}
		}
		// The risk: audit_level a level, times the excess of the wash
		// (today's throughput, before the day) over legit_ratio times
		// the income.
		excess := math.Max(0, want["throughput"]/(g.LegitRatio*income)-1)
		risk := fc.AuditRisk * (1 + g.AuditLevel*float64(level)) * (1 + excess)
		if math.Abs(got["audit_risk"]-risk) > 1e-9 {
			t.Errorf("level %d: audit risk %v, want %v", level, got["audit_risk"], risk)
		}
		if got["audit_risk"] <= base["audit_risk"] {
			t.Errorf("level %d: audit risk %v is not over level 0's %v", level, got["audit_risk"], base["audit_risk"])
		}
	}
	// legit_ratio: a big washer with a token level is the one the audit
	// finds. The first front at level 1 with its throughput multiplied
	// (the greedy dial, the tree) reads a higher risk than the same
	// front at a level whose income covers the wash; and with the ratio
	// off the excess is nothing.
	big := *cfg
	big.Laundering.Fronts = append([]content.FrontConfig(nil), cfg.Laundering.Fronts...)
	big.Laundering.Fronts[0].Throughput = fc.Income * int(g.LegitRatio) * 4 // four times what level 1's income explains, before the level's own multiplier
	token := levelProbe(t, &big, 1)
	if want := big.Laundering.Fronts[0].AuditRisk * (1 + g.AuditLevel) * 4 * fc.LevelMul; math.Abs(token["audit_risk"]-want) > 1e-9 {
		t.Errorf("a big washer at level 1: risk %v, want %v (the wash is %.1f times what the income explains)", token["audit_risk"], want, 4*fc.LevelMul)
	}
	off := big
	off.Laundering.Growth.LegitRatio = 0
	if got := levelProbe(t, &off, 1)["audit_risk"]; math.Abs(got-big.Laundering.Fronts[0].AuditRisk*(1+g.AuditLevel)) > 1e-9 {
		t.Errorf("with legit_ratio off the excess still counts: %v", got)
	}
	covered := levelProbe(t, &big, fc.MaxLevel)
	if covered["audit_risk"]/token["audit_risk"] >= 1 {
		t.Errorf("a front whose income covers the wash (level %d, risk %v) reads no lower than the token level (%v)", fc.MaxLevel, covered["audit_risk"], token["audit_risk"])
	}
	// The whole ladder pays back inside tier 3 at every front: the
	// price of a level over what it earns.
	for _, fc := range cfg.Laundering.Fronts {
		if fc.MaxLevel == 0 {
			continue
		}
		if days := float64(fc.LevelCost) / float64(fc.Income); days > 120 {
			t.Errorf("%s: a level pays back in %.0f days, over tier 3's 120", fc.ID, days)
		}
	}
}

// The wash with a level (#192): the upkeep comes out of the pile first
// and the income lands after it, counted in Stats.Earned and the
// CashLaundered event; a front whose upkeep the pile cannot cover shuts
// and earns nothing that day, so a levelled front pays its own way only
// out of yesterday's income, kept; a frozen front earns nothing.
func TestLevelIncomeLandsAfterUpkeep(t *testing.T) {
	cfg := content.MustLoad()
	cfg.Laundering.Fronts[0].AuditRisk = 0
	s := laundering.New(cfg)
	fc := cfg.Laundering.Fronts[0]
	w := world(fc.Cost + cfg.Laundering.Laundering.Float) // nothing over the float: the wash moves nothing
	if _, err := s.Buy(w, fc.ID); err != nil {
		t.Fatal(err)
	}
	w.Fronts[0].Level = 2
	f := w.Fronts[0]
	inc, up := s.Income(f), s.FrontUpkeep(w, f)
	if inc <= up {
		t.Fatalf("level 2 earns %d against upkeep %d; the test wants a front that pays its own way", inc, up)
	}
	// Nothing clean in hand and nothing to wash: the upkeep is unpaid,
	// the place shuts and the income never lands.
	if k := kinds(step(w, s)); k["FrontFrozen"] != 1 || w.Player.CleanCash != 0 || w.Stats.Earned != 0 {
		t.Fatalf("with no reserve: %v clean %d earned %d", k, w.Player.CleanCash, w.Stats.Earned)
	}
	w.Fronts[0].FrozenUntil = 0
	w.Player.CleanCash = up
	evs := step(w, s)
	var cl events.CashLaundered
	for _, e := range evs {
		if c, ok := e.(events.CashLaundered); ok {
			cl = c
		}
	}
	if w.Player.CleanCash != inc || w.Stats.Earned != inc || cl.Earned != inc || cl.Upkeep != up || cl.Amount != 0 {
		t.Fatalf("after a day with the upkeep in hand: clean %d (want %d), earned %d, event %+v", w.Player.CleanCash, inc, w.Stats.Earned, cl)
	}
	if k := kinds(evs); k["FrontFrozen"] != 0 {
		t.Fatalf("the front shut with its upkeep in hand: %v", k)
	}
	if got := s.LegitIncome(w); got != inc-up {
		t.Fatalf("legit income %d, want %d", got, inc-up)
	}
	// From here yesterday's income covers today's upkeep: the front
	// pays its own way.
	step(w, s)
	if w.Player.CleanCash != inc-up+inc || w.Stats.Earned != 2*inc {
		t.Fatalf("the second day: clean %d earned %d", w.Player.CleanCash, w.Stats.Earned)
	}
	w.Fronts[0].FrozenUntil = w.Day + 10
	step(w, s)
	if w.Player.CleanCash != inc-up+inc || w.Stats.Earned != 2*inc {
		t.Fatalf("a shut front earned: clean %d earned %d", w.Player.CleanCash, w.Stats.Earned)
	}
}

// Invest (#192): whole levels at once, clean cash only, refused past the
// top, on a front not owned, and after the end; reported the morning
// after as FrontInvested, and at the growth table's headline level as
// FrontGrew, once, stamping Grew.
func TestInvestAndGrow(t *testing.T) {
	cfg := content.MustLoad()
	cfg.Laundering.Fronts[0].AuditRisk = 0
	s := laundering.New(cfg)
	fc := cfg.Laundering.Fronts[0]
	g := cfg.Laundering.Growth
	w := world(fc.Cost + cfg.Laundering.Laundering.Float)
	if _, err := s.Buy(w, fc.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.Invest(w, "casino", 1); err != game.ErrNoFront {
		t.Fatalf("a front not owned: %v", err)
	}
	if err := s.Invest(w, fc.ID, 1); err != game.ErrNoCleanCash {
		t.Fatalf("with no clean cash: %v", err)
	}
	f := w.Fronts[0]
	w.Player.CleanCash = s.LevelCost(f, 1) - 1
	if err := s.Invest(w, fc.ID, 1); err == nil || err == game.ErrNoCleanCash {
		t.Fatalf("a dollar short: %v", err)
	}
	if err := s.Invest(w, fc.ID, 0); err != game.ErrBadQuantity {
		t.Fatalf("no levels: %v", err)
	}
	w.Player.CleanCash = s.LevelCost(f, fc.MaxLevel+1)
	if err := s.Invest(w, fc.ID, fc.MaxLevel+1); err == nil {
		t.Fatal("past the top was taken")
	}
	dirty := w.Player.DirtyCash
	two := s.LevelCost(f, 2)
	if err := s.Invest(w, fc.ID, 2); err != nil {
		t.Fatal(err)
	}
	f = w.Fronts[0]
	if f.Level != 2 || f.Invested != two || w.Stats.Invested != two || w.Player.DirtyCash != dirty || w.InvestedToday(fc.ID) != two {
		t.Fatalf("after two levels: %+v stats %d dirty %d today %d", f, w.Stats.Invested, w.Player.DirtyCash, w.InvestedToday(fc.ID))
	}
	if w.NetWorth() != w.Cash()+fc.Cost+two {
		t.Fatalf("net worth %d does not count the levels at cost", w.NetWorth())
	}
	evs := step(w, s)
	var inv events.FrontInvested
	for _, e := range evs {
		if i, ok := e.(events.FrontInvested); ok {
			inv = i
		}
	}
	if inv.Levels != 2 || inv.Level != 2 || inv.Cost != two || inv.Income != s.Income(f) || kinds(evs)["FrontGrew"] != 0 {
		t.Fatalf("the morning after: %+v, events %v", inv, kinds(evs))
	}
	w.ClearToday(w.Day)
	// Up to the headline level: the paper, once.
	w.Player.CleanCash += s.LevelCost(w.Fronts[0], g.HeadlineLevel)
	if err := s.Invest(w, fc.ID, g.HeadlineLevel-2); err != nil {
		t.Fatal(err)
	}
	evs = step(w, s)
	if k := kinds(evs); k["FrontGrew"] != 1 || k["FrontInvested"] != 1 {
		t.Fatalf("at level %d: %v", g.HeadlineLevel, k)
	}
	if w.Fronts[0].Grew != w.Day {
		t.Fatalf("Grew %d, want %d", w.Fronts[0].Grew, w.Day)
	}
	w.ClearToday(w.Day)
	if k := kinds(step(w, s)); k["FrontGrew"] != 0 {
		t.Fatalf("the paper ran it twice: %v", k)
	}
	w.Over = &game.Ending{Day: w.Day, Cause: "arrested"}
	if err := s.Invest(w, fc.ID, 1); err != game.ErrGameOver {
		t.Fatalf("after the end: %v", err)
	}
}
