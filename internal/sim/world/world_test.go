package world_test

import (
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/sim"
	"github.com/theclifmeister/kingpin/internal/sim/world"
)

func tick(w *game.World, d int) *game.Tick {
	return &game.Tick{Day: d, RNG: game.RNGFor(w.Seed, d), Seed: w.Seed}
}

// step runs the sim one day and reports what it dealt.
func step(s *world.Sim, w *game.World, d int) (*events.Incident, *game.Tick) {
	t := tick(w, d)
	s.Step(w, t)
	w.Day = d
	for _, e := range t.Events() {
		if ev, ok := e.(events.Incident); ok {
			return &ev, t
		}
	}
	return nil, t
}

// The pacing is the deck's: none for min_gap days after the last, one
// by max_gap, at most one a day, and never with the table boxed.
func TestIncidentPacing(t *testing.T) {
	cfg := content.MustLoad()
	pace := cfg.Incidents.Incidents
	s := world.New(cfg)
	w := sim.NewWorld(cfg, 5)
	w.Player.DirtyCash = 300_000
	var days []int
	for d := 1; d <= 300; d++ {
		if ev, _ := step(s, w, d); ev != nil {
			if ev.Day != d {
				t.Fatalf("incident dated %d dealt on day %d", ev.Day, d)
			}
			days = append(days, d)
		}
	}
	if len(days) < 300/(pace.MaxGap+1) {
		t.Fatalf("only %d incidents in 300 days", len(days))
	}
	for i := 1; i < len(days); i++ {
		if gap := days[i] - days[i-1]; gap < pace.MinGap || gap > pace.MaxGap {
			t.Errorf("incidents on days %d and %d: gap %d outside %d..%d", days[i-1], days[i], gap, pace.MinGap, pace.MaxGap)
		}
	}
	if len(w.Incidents.Fired) != len(days) || w.Incidents.Last != days[len(days)-1] {
		t.Fatalf("recorded %d, last %d; dealt %v", len(w.Incidents.Fired), w.Incidents.Last, days)
	}
	t.Logf("%d incidents in 300 days: %v", len(days), days)

	boxed := *cfg
	boxed.Incidents.Table = nil
	s = world.New(&boxed)
	w = sim.NewWorld(cfg, 5)
	for d := 1; d <= 100; d++ {
		tk := tick(w, d)
		s.Step(w, tk)
		if len(tk.Events()) > 0 || w.Incidents.Last != 0 {
			t.Fatalf("day %d: the boxed table dealt %+v", d, tk.Events())
		}
	}
}

// The dice are the incidents' own: the home stream is untouched whether
// the table is in or out, and a once row fires once while a row with a
// gap waits it out.
func TestIncidentDiceAndGaps(t *testing.T) {
	cfg := content.MustLoad()
	s := world.New(cfg)
	w := sim.NewWorld(cfg, 9)
	for d := 1; d <= 60; d++ {
		tk := tick(w, d)
		want := game.RNGFor(w.Seed, d).Float64()
		s.Step(w, tk)
		w.Day = d
		if got := tk.RNG.Float64(); got != want {
			t.Fatalf("day %d: the world sim drew from the home stream", d)
		}
	}

	one := *cfg
	one.Incidents.Incidents = content.IncidentsTuning{MinGap: 1, MaxGap: 1}
	one.Incidents.Table = []content.IncidentConfig{
		{ID: "once", Name: "Once", Once: true, Report: ".", Effects: content.IncidentEffects{Pressure: 1}},
		{ID: "gapped", Name: "Gapped", MinGap: 10, Report: ".", Effects: content.IncidentEffects{Pressure: 1}},
	}
	s = world.New(&one)
	w = sim.NewWorld(&one, 9)
	count := map[string][]int{}
	for d := 1; d <= 60; d++ {
		if ev, _ := step(s, w, d); ev != nil {
			count[ev.ID] = append(count[ev.ID], d)
		}
	}
	if len(count["once"]) != 1 {
		t.Fatalf("the once row fired on days %v", count["once"])
	}
	g := count["gapped"]
	if len(g) < 3 {
		t.Fatalf("the gapped row fired on days %v", g)
	}
	for i := 1; i < len(g); i++ {
		if g[i]-g[i-1] < 10 {
			t.Fatalf("the gapped row fired on days %v: gap under its min_gap of 10", g)
		}
	}
}

// What an incident does lands where it says: the city it names, the
// routes of its mode, the market's own shock state, the federal window;
// and the event carries the slots the paper prints.
func TestIncidentEffectsLand(t *testing.T) {
	cfg := content.MustLoad()
	home := cfg.City.Home().ID
	port := ""
	for _, c := range cfg.City.Cities {
		if c.ID != home {
			port = c.ID
		}
	}
	only := func(inc content.IncidentConfig) *world.Sim {
		c := *cfg
		c.Incidents.Incidents = content.IncidentsTuning{MinGap: 1, MaxGap: 1}
		c.Incidents.Table = []content.IncidentConfig{inc}
		return world.New(&c)
	}
	w := sim.NewWorld(cfg, 2)
	s := only(content.IncidentConfig{ID: "strike", Name: "Strike", RouteMode: "boat", Names: "celebrities", Report: ".",
		Trigger: content.CardTrigger{City: port},
		Effects: content.IncidentEffects{RouteClosed: 4, MarketShock: content.ProductShock{Product: "coke", Mul: 1.6, Days: 10}, Pressure: 15, Heat: 5, Notoriety: 7, HeatDecay: content.TimedMul{Mul: 0.5, Days: 20}, ElectionCalled: 7}})
	before := w.Cities[port].Pressure
	ev, _ := step(s, w, 1)
	if ev == nil {
		t.Fatal("nothing fired on a certain day")
	}
	if ev.City != port || ev.Route == "" || ev.Product != "coke" || ev.Person == "" || ev.Days != 4 || ev.Election != 7 || ev.NewChief || ev.Chief != w.Law.Chief.Name || ev.DA != w.Law.DA.Name {
		t.Fatalf("event %+v", ev)
	}
	boats := 0
	for _, r := range cfg.Routes.Routes {
		rs := w.Route(r.ID)
		if r.Mode == "boat" {
			boats++
			if rs.ClosedUntil != 1+4 || !rs.Closed(1) || !rs.Closed(4) || rs.Closed(5) {
				t.Fatalf("boat route %s: closed until %d", r.ID, rs.ClosedUntil)
			}
			if r.Name != ev.Route && boats == 1 {
				t.Fatalf("the slot names %q, the first boat route is %q", ev.Route, r.Name)
			}
		} else if rs.ClosedUntil != 0 {
			t.Fatalf("%s route %s was shut by a boat closure", r.Mode, r.ID)
		}
	}
	if boats == 0 {
		t.Fatal("no boat route in the file")
	}
	m := w.Product(port, "coke")
	if m.ShockFactor != 1.6 || m.ShockDays != 11 || m.ShockSlump {
		t.Fatalf("the port's coke: factor %v days %d slump %v", m.ShockFactor, m.ShockDays, m.ShockSlump)
	}
	if h := w.Product(home, "coke"); h.ShockDays != 0 {
		t.Fatalf("the shock landed at home too: %+v", h)
	}
	if c := w.Cities[port]; c.Pressure != before+15 || c.Heat != 5 {
		t.Fatalf("the port: pressure %v (was %v) heat %v", c.Pressure, before, c.Heat)
	}
	if w.Cities[home].Pressure != 0 || w.Cities[home].Heat != 0 {
		t.Fatal("home moved on an incident in the port")
	}
	if w.Player.Reputation.Notoriety != 7 || w.Heat.FederalUntil != 21 || w.Heat.FederalDecay != 0.5 {
		t.Fatalf("notoriety %v federal until %d decay %v", w.Player.Reputation.Notoriety, w.Heat.FederalUntil, w.Heat.FederalDecay)
	}
	if len(w.Incidents.Fired) != 1 || w.Incidents.Count["strike"] != 1 || w.Incidents.LastOf["strike"] != 1 {
		t.Fatalf("recorded %+v", w.Incidents)
	}

	// A row about a product the city has not listed waits; one with no
	// city lands at home; a demand shift is the slump's shape.
	w = sim.NewWorld(cfg, 2)
	s = only(content.IncidentConfig{ID: "synth", Name: "Synth", Report: ".", Effects: content.IncidentEffects{DemandShift: content.ProductShock{Product: "designer", Mul: 0.5, Days: 60}}})
	if ev, _ := step(s, w, 1); ev != nil {
		t.Fatalf("fired on a product nobody has listed: %+v", ev)
	}
	s = only(content.IncidentConfig{ID: "synth", Name: "Synth", Report: ".", Effects: content.IncidentEffects{DemandShift: content.ProductShock{Product: "pills", Mul: 0.5, Days: 60}}})
	ev, _ = step(s, w, 2)
	if ev == nil || ev.City != home || ev.Product != "pills" || ev.Days != 60 {
		t.Fatalf("event %+v", ev)
	}
	if m := w.Product(home, "pills"); m.ShockFactor != 0.5 || m.ShockDays != 61 || !m.ShockSlump {
		t.Fatalf("home's pills: %+v", m)
	}
}
