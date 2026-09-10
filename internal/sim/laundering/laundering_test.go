package laundering_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/sim/laundering"
)

func world(cash int) *game.World {
	w := game.NewWorld(7, "Testville", []game.StartingProduct{{ID: "a", Name: "A", Price: 10, Demand: 5}}, cash, 100)
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
	s := laundering.New(cfg.Laundering)
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
	s := laundering.New(cfg.Laundering)
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
	s := laundering.New(cfg.Laundering)
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
	s := laundering.New(cfg.Laundering)
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
	s = laundering.New(content.MustLoad().Laundering)
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
	s := laundering.New(cfg.Laundering)
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
	s := laundering.New(cfg.Laundering)
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
