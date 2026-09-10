package crew_test

import (
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/sim/crew"
)

func world(t *testing.T, cfg *content.Config, cash int) (*game.World, *crew.Sim) {
	t.Helper()
	w := game.NewWorld(7, "Testville", []game.StartingProduct{{ID: "a", Name: "A", Price: 10, Demand: 5}}, cash, 100)
	s := crew.New(cfg.Crew, cfg.Names)
	s.Seed(w, game.RNGFor(7, 0))
	return w, s
}

func step(w *game.World, s *crew.Sim, extra ...events.Event) []events.Event {
	t := &game.Tick{Day: w.Day + 1, RNG: game.RNGFor(w.Seed, w.Day+1)}
	for _, e := range extra {
		t.Emit(e)
	}
	s.Step(w, t)
	w.Day++
	w.Crew.HiredToday, w.Crew.FiredToday = nil, nil
	return t.Events()
}

func kinds(evs []events.Event) map[string]int {
	out := map[string]int{}
	for _, e := range evs {
		out[e.Kind()]++
	}
	return out
}

func TestPoolIsSeededAndRotates(t *testing.T) {
	cfg := content.MustLoad()
	w, s := world(t, cfg, 500)
	tun := cfg.Crew.Crew
	if len(w.Crew.Candidates) != tun.Candidates {
		t.Fatalf("pool has %d candidates, want %d", len(w.Crew.Candidates), tun.Candidates)
	}
	seen := map[string]bool{}
	for _, c := range w.Crew.Candidates {
		if seen[c.Name] {
			t.Fatalf("duplicate name %q in pool", c.Name)
		}
		seen[c.Name] = true
		if c.Skill <= 0 || c.Wage <= 0 || c.Fee <= 0 || (c.Role == "runner") != (c.Units > 0) {
			t.Fatalf("badly generated candidate %+v", c)
		}
	}
	first := w.Crew.Candidates[0].ID
	for i := 0; i < tun.PoolDays-1; i++ {
		step(w, s)
		if w.Crew.Candidates[0].ID != first {
			t.Fatalf("pool rotated on day %d, before pool_days", w.Day)
		}
	}
	step(w, s)
	for _, c := range w.Crew.Candidates {
		if c.ID == first {
			t.Fatal("pool did not rotate on schedule")
		}
	}
}

func TestWagesFiringAndQuitting(t *testing.T) {
	cfg := content.MustLoad()
	w, s := world(t, cfg, 100_000)
	w.SetPay(events.PayFair)
	var ids []int
	for _, c := range append([]game.CrewMember(nil), w.Crew.Candidates...) {
		m, err := w.Hire(c.ID, s.MaxCrew())
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, m.ID)
	}
	bill := s.Wages(w, events.PayFair)
	if bill <= 0 || s.Wages(w, events.PayStingy) >= bill || s.Wages(w, events.PayGenerous) <= bill {
		t.Fatalf("wages not ordered by dial: %d/%d/%d", s.Wages(w, events.PayStingy), bill, s.Wages(w, events.PayGenerous))
	}
	before := w.Player.DirtyCash
	evs := step(w, s)
	k := kinds(evs)
	if k["CrewHired"] != len(ids) || k["CrewPaid"] != 1 {
		t.Fatalf("events after hiring: %v", k)
	}
	if w.Player.DirtyCash != before-bill || w.Stats.Wages != bill {
		t.Fatalf("wages: cash %d -> %d, bill %d", before, w.Player.DirtyCash, bill)
	}

	// Firing sours the rest by fire_loyalty on top of the day's drift.
	loyal := map[int]float64{}
	for _, m := range w.Crew.Members {
		loyal[m.ID] = m.Loyalty
	}
	if _, err := w.Fire(ids[0]); err != nil {
		t.Fatal(err)
	}
	control := *w
	control.Crew.Members = append([]game.CrewMember(nil), w.Crew.Members...)
	control.Crew.FiredToday = nil
	control.Crew.Candidates = append([]game.CrewMember(nil), w.Crew.Candidates...)
	evs = step(w, s)
	step(&control, s)
	if kinds(evs)["CrewFired"] != 1 {
		t.Fatalf("no CrewFired event: %v", kinds(evs))
	}
	for i, m := range w.Crew.Members {
		want := control.Crew.Members[i].Loyalty - cfg.Crew.Crew.FireLoyalty
		if m.Loyalty > want+1e-9 {
			t.Fatalf("%s: loyalty %.1f after a firing, control %.1f", m.Name, m.Loyalty, control.Crew.Members[i].Loyalty)
		}
	}

	// Unpaid wages hurt, and a member at the floor walks.
	w.Player.DirtyCash = 0
	w.Player.Stock["a"] = 10
	w.Crew.Members[0].Loyalty = cfg.Crew.Crew.QuitThreshold + 1
	evs = step(w, s)
	k = kinds(evs)
	if k["CrewQuit"] != 1 {
		t.Fatalf("expected one quit after unpaid wages: %v", k)
	}
	var paid events.CrewPaid
	for _, e := range evs {
		if p, ok := e.(events.CrewPaid); ok {
			paid = p
		}
	}
	if paid.Wages != 0 || paid.Short <= 0 {
		t.Fatalf("unpaid day reported as %+v", paid)
	}
}

func TestSkimTakesFromTakings(t *testing.T) {
	cfg := content.MustLoad()
	w, s := world(t, cfg, 10_000)
	for _, c := range append([]game.CrewMember(nil), w.Crew.Candidates...) {
		if _, err := w.Hire(c.ID, s.MaxCrew()); err != nil {
			t.Fatal(err)
		}
	}
	for i := range w.Crew.Members {
		w.Crew.Members[i].Loyalty = 0
	}
	skimmed := 0
	for day := 0; day < 30 && skimmed == 0; day++ {
		before := w.Player.DirtyCash
		evs := step(w, s, events.PlayerSold{Day: w.Day + 1, Product: "a", Sold: 100, Revenue: 1000})
		for _, e := range evs {
			if sk, ok := e.(events.CrewSkimmed); ok {
				skimmed = sk.Amount
				if sk.Amount <= 0 || sk.Amount > int(1000*cfg.Crew.Crew.SkimCap)+1 {
					t.Fatalf("skim of %d from $1000 takings", sk.Amount)
				}
				if w.Player.DirtyCash > before-sk.Amount {
					t.Fatalf("skim not deducted: %d -> %d, skim %d", before, w.Player.DirtyCash, sk.Amount)
				}
				if w.Crew.LastSkim != w.Day || w.Stats.Skimmed != sk.Amount {
					t.Fatalf("skim not recorded: last %d stats %d", w.Crew.LastSkim, w.Stats.Skimmed)
				}
			}
		}
		// Everyone is at the floor, so they walk on day one; pin them.
		for i := range w.Crew.Members {
			w.Crew.Members[i].Loyalty = 0
		}
		if len(w.Crew.Members) == 0 {
			break
		}
	}
	if skimmed == 0 {
		t.Fatal("a crew at zero loyalty never skimmed")
	}
}

func TestBrokeEndsTheRun(t *testing.T) {
	cfg := content.MustLoad()
	w, s := world(t, cfg, 500)
	c := w.Crew.Candidates[0]
	w.Player.DirtyCash = c.Fee
	if _, err := w.Hire(c.ID, s.MaxCrew()); err != nil {
		t.Fatal(err)
	}
	evs := step(w, s)
	if w.Over == nil || w.Over.Cause != "broke" || kinds(evs)["GameOver"] != 1 {
		t.Fatalf("no cash, no stock, wages due: over=%v events=%v", w.Over, kinds(evs))
	}
}
