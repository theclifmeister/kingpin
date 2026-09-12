package laundering_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/format"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/sim/laundering"
)

func world(cash int) *game.World {
	w := game.NewWorld(7, []game.StartingCity{{ID: "test", Name: "Testville", Products: []game.StartingProduct{{ID: "a", Name: "A", Price: 10, Demand: 5}}}}, cash, 100)
	w.Stats.PeakCash = cash
	w.Laundering.Dial = events.LaunderNormal
	return w
}

func step(w *game.World, s *laundering.Sim) []events.Event {
	t := &game.Tick{Day: w.Day + 1, RNG: game.RNGFor(w.Seed, w.Day+1)}
	s.Step(w, t)
	w.Day++
	return t.Events()
}

func kinds(evs []events.Event) map[string]int {
	out := map[string]int{}
	for _, e := range evs {
		out[e.Kind()]++
	}
	return out
}

// Every front: buying below its unlock, without the cash, or twice is
// refused with a readable error and leaves the world untouched; an unknown
// id is refused the same way.
func TestBuyFrontRefusals(t *testing.T) {
	cfg := content.MustLoad()
	s := laundering.New(cfg)
	if _, err := s.Buy(world(1_000_000_000), "casino"); err != game.ErrNoFront {
		t.Fatalf("unknown front: %v", err)
	}
	for _, fc := range cfg.Laundering.Fronts {
		cases := []struct {
			name string
			cash int
			peak int
			want string
		}{
			{"below unlock", fc.Cost * 2, fc.UnlockCash - 1, "nobody will sell"},
			{"without cash", fc.Cost - 1, fc.UnlockCash * 2, "only have"},
		}
		for _, c := range cases {
			w := world(c.cash)
			w.Stats.PeakCash = c.peak
			before := *w
			_, err := s.Buy(w, fc.ID)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("%s %s: err %v, want %q", fc.ID, c.name, err, c.want)
			}
			if !reflect.DeepEqual(before, *w) {
				t.Fatalf("%s %s: world changed: %+v -> %+v", fc.ID, c.name, before, *w)
			}
		}
		w := world(fc.Cost * 3)
		got, err := s.Buy(w, fc.ID)
		if err != nil || got.ID != fc.ID || got.Cost != fc.Cost || w.Player.DirtyCash != fc.Cost*2 || len(w.Fronts) != 1 {
			t.Fatalf("%s: buy %v %+v cash %d fronts %d", fc.ID, err, got, w.Player.DirtyCash, len(w.Fronts))
		}
		if w.NetWorth() != fc.Cost*3 {
			t.Fatalf("%s: net worth %d after buying, want the front counted at cost", fc.ID, w.NetWorth())
		}
		before := *w
		if _, err := s.Buy(w, fc.ID); err != game.ErrFrontOwned {
			t.Fatalf("%s twice: %v", fc.ID, err)
		}
		if !reflect.DeepEqual(before, *w) {
			t.Fatalf("%s twice: world changed", fc.ID)
		}
	}
	w := world(0)
	w.Over = &game.Ending{Day: 1, Cause: "arrested"}
	if _, err := s.Buy(w, cfg.Laundering.Fronts[0].ID); err != game.ErrGameOver {
		t.Fatalf("buy after the end: %v", err)
	}
}

// The wash moves dirty to clean up to the front's throughput, never below
// the float, pays upkeep in clean cash, and is reported the day after the
// purchase; a frozen front washes nothing.
func TestWashAndFloat(t *testing.T) {
	cfg := content.MustLoad()
	cfg.Laundering.Fronts[0].AuditRisk = 0
	s := laundering.New(cfg)
	tun := cfg.Laundering.Laundering
	fc := cfg.Laundering.Fronts[0]
	w := world(fc.Cost + tun.Float + fc.Throughput*3)
	if _, err := s.Buy(w, fc.ID); err != nil {
		t.Fatal(err)
	}
	evs := step(w, s)
	k := kinds(evs)
	if k["FrontBought"] != 1 || k["CashLaundered"] != 1 {
		t.Fatalf("day 1 events: %v", k)
	}
	f := w.Fronts[0]
	if f.WashedToday != fc.Throughput || w.Player.CleanCash != fc.Throughput-fc.Upkeep || w.Player.DirtyCash != tun.Float+fc.Throughput*2 {
		t.Fatalf("after one wash: front %+v clean %d dirty %d", f, w.Player.CleanCash, w.Player.DirtyCash)
	}
	if s.Capacity(w) != fc.Throughput || s.Upkeep(w) != fc.Upkeep {
		t.Fatalf("capacity %d upkeep %d", s.Capacity(w), s.Upkeep(w))
	}
	for i := 0; i < 5; i++ {
		step(w, s)
	}
	if w.Player.DirtyCash != tun.Float {
		t.Fatalf("the wash took the till to %d, the float is %d", w.Player.DirtyCash, tun.Float)
	}
	if w.Fronts[0].Washed != fc.Throughput*3 || w.Stats.Laundered != fc.Throughput*3 {
		t.Fatalf("lifetime washed %d, stats %d", w.Fronts[0].Washed, w.Stats.Laundered)
	}
	// Frozen: nothing moves, and the reopening day is honoured.
	w.Player.DirtyCash = tun.Float + 10*fc.Throughput
	w.Fronts[0].FrozenUntil = w.Day + 3
	clean := w.Player.CleanCash
	for i := 0; i < 2; i++ {
		if k := kinds(step(w, s)); k["CashLaundered"] != 0 || w.Fronts[0].WashedToday != 0 {
			t.Fatalf("frozen front washed on day %d: %v", w.Day, k)
		}
	}
	if w.Player.CleanCash != clean || s.Capacity(w) != fc.Throughput {
		t.Fatalf("frozen: clean %d -> %d, capacity tomorrow %d", clean, w.Player.CleanCash, s.Capacity(w))
	}
	if k := kinds(step(w, s)); k["CashLaundered"] != 1 {
		t.Fatalf("front did not reopen on day %d: %v", w.Day, k)
	}
}

// Upkeep the clean cash cannot cover shuts the front for a while.
func TestUnpaidUpkeepFreezes(t *testing.T) {
	cfg := content.MustLoad()
	s := laundering.New(cfg)
	tun := cfg.Laundering.Laundering
	fc := cfg.Laundering.Fronts[0]
	w := world(fc.Cost + tun.Float) // nothing over the float to wash
	if _, err := s.Buy(w, fc.ID); err != nil {
		t.Fatal(err)
	}
	evs := step(w, s)
	if k := kinds(evs); k["FrontFrozen"] != 1 || k["CashLaundered"] != 0 {
		t.Fatalf("events: %v", k)
	}
	if !w.Fronts[0].Frozen(w.Day) || w.Fronts[0].FrozenUntil != w.Day+tun.UpkeepFreezeDays || w.Player.CleanCash != 0 {
		t.Fatalf("after unpaid upkeep: %+v clean %d", w.Fronts[0], w.Player.CleanCash)
	}
}

// An audit freezes the front, seizes part of today's wash, records the
// dial it hit at, and is emitted; the dial scales throughput and risk.
func TestAuditAndDial(t *testing.T) {
	cfg := content.MustLoad()
	fc := cfg.Laundering.Fronts[0]
	cfg.Laundering.Fronts[0].AuditRisk = 1 // certain, at every dial
	cfg.Laundering.Dial.Careful.Risk = 1
	s := laundering.New(cfg)
	tun := cfg.Laundering.Laundering
	for _, d := range []events.Launder{events.LaunderCareful, events.LaunderNormal, events.LaunderGreedy} {
		w := world(fc.Cost + tun.Float + fc.Throughput*10)
		w.SetLaunderDial(d)
		if _, err := s.Buy(w, fc.ID); err != nil {
			t.Fatal(err)
		}
		want := int(float64(fc.Throughput)*s.Dial(d).Mul + 0.5)
		if got := s.Throughput(w, w.Fronts[0]); got != want {
			t.Fatalf("%s throughput %d, want %d", d, got, want)
		}
		evs := step(w, s)
		var audit *events.FrontAudited
		for _, e := range evs {
			if a, ok := e.(events.FrontAudited); ok {
				audit = &a
			}
		}
		f := w.Fronts[0]
		if audit == nil || audit.Dial != d || audit.Days != tun.AuditFreezeDays || f.Audited != w.Day || f.AuditDial != d {
			t.Fatalf("%s: audit %+v front %+v", d, audit, f)
		}
		seized := int(float64(want)*tun.AuditSeize + 0.5)
		if audit.Seized != seized || w.Player.CleanCash != want-fc.Upkeep-seized || w.Stats.Seized != seized {
			t.Fatalf("%s: seized %d (want %d), clean %d, stats %d", d, audit.Seized, seized, w.Player.CleanCash, w.Stats.Seized)
		}
		if !f.Frozen(w.Day+1) || f.Frozen(w.Day+tun.AuditFreezeDays) {
			t.Fatalf("%s: freeze %+v", d, f)
		}
	}
	// Risk follows the dial.
	w := world(fc.Cost * 2)
	fresh := content.MustLoad()
	s = laundering.New(fresh)
	if _, err := s.Buy(w, fc.ID); err != nil {
		t.Fatal(err)
	}
	var risks []float64
	for _, d := range []events.Launder{events.LaunderCareful, events.LaunderNormal, events.LaunderGreedy} {
		w.SetLaunderDial(d)
		risks = append(risks, s.AuditRisk(w, w.Fronts[0]))
	}
	if !(risks[0] < risks[1] && risks[1] < risks[2]) || risks[1] != fc.AuditRisk {
		t.Fatalf("audit risk by dial: %v", risks)
	}
}

// An accountant adds throughput to every front and cuts every front's
// audit risk, both by skill; two of them stack.
func TestAccountants(t *testing.T) {
	cfg := content.MustLoad()
	s := laundering.New(cfg)
	tun := cfg.Laundering.Laundering
	fc := cfg.Laundering.Fronts[0]
	w := world(fc.Cost * 2)
	if _, err := s.Buy(w, fc.ID); err != nil {
		t.Fatal(err)
	}
	f := w.Fronts[0]
	base, risk := s.Throughput(w, f), s.AuditRisk(w, f)
	w.Crew.Members = append(w.Crew.Members, game.CrewMember{ID: 1, Name: "Books", Role: "accountant", Skill: 50})
	if got := s.Throughput(w, f); got != base+int(tun.AccountantThroughput/2) {
		t.Fatalf("one skill-50 accountant: throughput %d, base %d", got, base)
	}
	if got := s.AuditRisk(w, f); got >= risk || got != risk*(1-tun.AccountantRiskCut/2) {
		t.Fatalf("one skill-50 accountant: risk %v, base %v", got, risk)
	}
	w.Crew.Members = append(w.Crew.Members, game.CrewMember{ID: 2, Name: "Ledger", Role: "accountant", Skill: 100})
	if got := s.Throughput(w, f); got != base+int(tun.AccountantThroughput*1.5) {
		t.Fatalf("two accountants: throughput %d, base %d", got, base)
	}
	if got := s.AuditRisk(w, f); got != risk*(1-tun.AccountantRiskCut/2)*(1-tun.AccountantRiskCut) {
		t.Fatalf("two accountants: risk %v, base %v", got, risk)
	}
	w.Crew.Members = append(w.Crew.Members, game.CrewMember{ID: 3, Name: "Dre", Role: "runner", Skill: 100})
	if got := s.Throughput(w, f); got != base+int(tun.AccountantThroughput*1.5) {
		t.Fatalf("a runner changed the throughput to %d", got)
	}
}

// Offers are priced from config, cheapest first, and a front the config
// no longer lists is inert rather than a crash.
func TestOffers(t *testing.T) {
	cfg := content.MustLoad()
	s := laundering.New(cfg)
	offers := s.Offers()
	if len(offers) != len(cfg.Laundering.Fronts) {
		t.Fatalf("%d offers for %d fronts", len(offers), len(cfg.Laundering.Fronts))
	}
	for i := 1; i < len(offers); i++ {
		if offers[i].Cost < offers[i-1].Cost {
			t.Fatalf("offers not sorted by cost: %+v", offers)
		}
	}
	w := world(100)
	w.Fronts = append(w.Fronts, game.Front{ID: "ghost", Name: "Ghost"})
	if k := kinds(step(w, s)); len(k) != 0 || s.Capacity(w) != 0 {
		t.Fatalf("a front the config does not know did something: %v", k)
	}
}

// An audit is the auditors questioning whoever keeps the books: the least
// loyal accountant under the informant line is turned (#13), one per
// audit, never twice, and never anybody loyal or in another role. The
// event is the crew sim's, so the heat sim treats it the same way.
func TestAuditFlipsTheAccountant(t *testing.T) {
	cfg := content.MustLoad()
	fc := cfg.Laundering.Fronts[0]
	cfg.Laundering.Fronts[0].AuditRisk = 1
	cfg.Laundering.Laundering.AccountantRiskCut = 0 // still certain with accountants about
	s := laundering.New(cfg)
	tun := cfg.Laundering.Laundering
	line := cfg.Crew.Informant.Loyalty
	w := world(fc.Cost + tun.Float + fc.Throughput*10)
	if _, err := s.Buy(w, fc.ID); err != nil {
		t.Fatal(err)
	}
	w.Crew.Members = []game.CrewMember{
		{ID: 1, Name: "Books", Role: "accountant", Skill: 50, Loyalty: line - 5, Nerve: 90},
		{ID: 2, Name: "Ledger", Role: "accountant", Skill: 50, Loyalty: line - 10, Nerve: 90},
		{ID: 3, Name: "Steady", Role: "accountant", Skill: 50, Loyalty: line, Nerve: 0},
		{ID: 4, Name: "Runs", Role: "runner", Skill: 50, Loyalty: 0, Nerve: 0},
	}
	evs := step(w, s)
	k := kinds(evs)
	if k["FrontAudited"] != 1 || k["CrewTurnedInformant"] != 1 {
		t.Fatalf("first audit: %v", k)
	}
	if !w.Crew.Members[1].Informant || w.Crew.Informants() != 1 || w.Stats.Informants != 1 {
		t.Fatalf("the least loyal accountant did not turn: %+v", w.Crew.Members)
	}
	for _, e := range evs {
		if ev, ok := e.(events.CrewTurnedInformant); ok && (ev.ID != 2 || ev.Name != "Ledger") {
			t.Fatalf("turned %+v", ev)
		}
	}
	// The next audit (the front is frozen; open another) turns the next
	// one down, and the loyal and the runner never.
	w.Fronts[0].FrozenUntil = 0
	evs = step(w, s)
	if kinds(evs)["CrewTurnedInformant"] != 1 || !w.Crew.Members[0].Informant || w.Crew.Informants() != 2 {
		t.Fatalf("second audit: %v, roster %+v", kinds(evs), w.Crew.Members)
	}
	w.Fronts[0].FrozenUntil = 0
	evs = step(w, s)
	if kinds(evs)["CrewTurnedInformant"] != 0 || w.Crew.Informants() != 2 {
		t.Fatalf("third audit: %v, roster %+v", kinds(evs), w.Crew.Members)
	}
}

// launderProbe reads every number the Laundering branch can move off a
// world owning the given nodes (granted, prerequisites ignored: a node
// is measured alone) with the first front bought: the sim's public
// reads, and what one certain audit seizes and shuts the front for.
func launderProbe(t *testing.T, cfg *content.Config, ids ...string) map[string]float64 {
	t.Helper()
	certain := *cfg
	certain.Laundering.Fronts = append([]content.FrontConfig(nil), cfg.Laundering.Fronts...)
	certain.Laundering.Fronts[0].AuditRisk = 1
	fc := cfg.Laundering.Fronts[0]
	tun := cfg.Laundering.Laundering
	w := world(fc.Cost + tun.Float + fc.Throughput*10)
	for _, id := range ids {
		w.Upgrades[id] = true
	}
	s := laundering.New(cfg)
	if _, err := s.Buy(w, fc.ID); err != nil {
		t.Fatal(err)
	}
	p := map[string]float64{}
	f := w.Fronts[0]
	p["throughput"] = float64(s.Throughput(w, f))
	p["audit_risk"] = s.AuditRisk(w, f)
	p["upkeep"] = float64(s.FrontUpkeep(w, f))
	p["float"] = float64(s.Float(w))
	p["freeze"] = float64(s.AuditFreezeDays(w))
	// One night with the audit certain: what it seizes of the wash.
	s = laundering.New(&certain)
	var audit events.FrontAudited
	for _, e := range step(w, s) {
		if a, ok := e.(events.FrontAudited); ok {
			audit = a
		}
	}
	if audit.Front == "" {
		t.Fatalf("no audit with the risk certain")
	}
	p["seized"] = float64(audit.Seized) / float64(w.Fronts[0].WashedToday)
	p["shut"] = float64(audit.Days)
	return p
}

// Every Laundering node moves the number it names, the way it says, and
// no other (#118). The probe reads each number off the sim the way the
// ledger does, so what the pane prints is what the dice use.
func TestLaunderingNodesPullTheirWay(t *testing.T) {
	cfg := content.MustLoad()
	base := launderProbe(t, cfg)
	rows := []struct {
		id    string
		moves map[string]float64
	}{
		{"books", map[string]float64{"throughput": base["throughput"] * 1.15}},
		{"shell", map[string]float64{"audit_risk": base["audit_risk"] * 0.7}},
		{"cashbiz", map[string]float64{"throughput": base["throughput"] * 1.25}},
		{"float", map[string]float64{"float": base["float"] * 0.5}},
		{"offshore", map[string]float64{"seized": base["seized"] * 0.5, "upkeep": base["upkeep"] * 0.8}},
		{"books2", map[string]float64{"freeze": base["freeze"] - 7, "shut": base["shut"] - 7}},
	}
	seen := map[string]bool{}
	for _, row := range rows {
		seen[row.id] = true
		got := launderProbe(t, cfg, row.id)
		for k, want := range row.moves {
			if d := got[k] - want; d > 0.51 || d < -0.51 {
				t.Errorf("%s: %s is %v with it, want %v (base %v)", row.id, k, got[k], want, base[k])
			}
		}
		for k, v := range got {
			if _, named := row.moves[k]; !named && v != base[k] {
				t.Errorf("%s: moves %s (%v -> %v), which it does not name", row.id, k, base[k], v)
			}
		}
	}
	for _, n := range cfg.Upgrades.Branch("laundering") {
		if !seen[n.ID] {
			t.Errorf("the Laundering branch has %s, which the table does not", n.ID)
		}
	}
	// The thinner float is the road's too: the wash and the budget read
	// one number off the world.
	w := world(1_000_000)
	w.Upgrades["float"] = true
	s := laundering.New(cfg)
	if got, want := s.Float(w), w.Float(cfg.Upgrades, cfg.Laundering.Laundering.Float); got != want || got != cfg.Laundering.Laundering.Float/2 {
		t.Fatalf("float %d, world %d, want half of %d", got, want, cfg.Laundering.Laundering.Float)
	}
	if got := s.Washable(w); got != 1_000_000-cfg.Laundering.Laundering.Float/2 {
		t.Fatalf("washable %d over a float of %d", got, s.Float(w))
	}
}

// A front's offer is announced the morning the ledger says it is open
// (#148): the laundering sim reads the line against the peak the clock
// is about to stamp, so the Unlocked fires in the tick the cash crosses
// it, once, with the offer's cost and line; a front already owned when
// its line is first read is stamped silently; and a run under every
// line fires none.
func TestFrontOpensTheMorningTheLedgerSays(t *testing.T) {
	cfg := content.MustLoad()
	s := laundering.New(cfg)
	first := s.Offers()[0]
	w := world(first.UnlockCash - 100)
	if evs := step(w, s); kinds(evs)["Unlocked"] != 0 || len(w.Laundering.Offered) != 0 {
		t.Fatalf("under the line: %v %v", kinds(evs), w.Laundering.Offered)
	}
	w.Player.DirtyCash = first.UnlockCash + 100 // the peak is stamped at the end of the day: not yet
	var got []events.Unlocked
	for _, e := range step(w, s) {
		if ev, ok := e.(events.Unlocked); ok {
			got = append(got, ev)
		}
	}
	if len(got) != 1 || got[0].Gate != "front" || got[0].ID != first.ID || got[0].Name != first.Name || got[0].Cost != first.Cost || got[0].Why != "peak cash "+format.Cash(first.UnlockCash) || got[0].City != "" {
		t.Fatalf("on the line: %+v", got)
	}
	if !w.Laundering.Offered[first.ID] {
		t.Fatalf("not stamped: %v", w.Laundering.Offered)
	}
	w.Stats.PeakCash = w.Player.DirtyCash
	if evs := step(w, s); kinds(evs)["Unlocked"] != 0 {
		t.Fatalf("announced twice: %v", kinds(evs))
	}
	// An old save: the front owned before the field existed.
	w = world(first.UnlockCash * 2)
	if _, err := s.Buy(w, first.ID); err != nil {
		t.Fatal(err)
	}
	if evs := step(w, s); kinds(evs)["Unlocked"] != 0 || !w.Laundering.Offered[first.ID] {
		t.Fatalf("an owned front: %v %v", kinds(evs), w.Laundering.Offered)
	}
}
