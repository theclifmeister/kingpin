package harness

import (
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/sim"
)

// Crew life (#46): kin, ageing, arrests and getting shot.

// With crew.toml's [life] table boxed (harness.NoLife, cmd/balance
// -life off) a run is byte-for-byte the run before the feature: the
// life stream is drawn but nothing acts on it. The proof is the money
// curve's own numbers as main pinned them before #46 (t1-t4 with the
// incidents boxed, as every harness run has them), and a crewed run
// under the box that emits nothing of #46's and carries none of its
// state.
func TestNoLifeIsTheOldRun(t *testing.T) {
	cfg := NoLife(content.MustLoad())
	for _, row := range []struct {
		tier   int
		policy func(*content.Config) Policy
		want   int
	}{
		{1, func(c *content.Config) Policy { return Managed(c, 50) }, 84_930},
		{2, func(c *content.Config) Policy { return Crewed(c, 40) }, 605_294},
		{3, func(c *content.Config) Policy { return Boss(c, 40, "") }, 16_650_659},
		{4, func(c *content.Config) Policy { return Boss(c, 40, "") }, 83_811_847},
	} {
		if got := medianNetWorth(t, cfg, row.policy, tierDay(row.tier)); got != row.want {
			t.Errorf("tier %d with life boxed: median net worth %d on day %d, main's pre-#46 figure is %d", row.tier, got, tierDay(row.tier), row.want)
		}
	}
	for seed := uint64(1); seed <= 3; seed++ {
		res, err := Run(cfg, seed, 120, Crewed(cfg, 40))
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range res.Events {
			switch ev := e.(type) {
			case events.CrewArrested, events.CrewReleased, events.CrewBailed, events.CrewShot, events.CrewRecovered, events.CrewRetired, events.KinLooking:
				t.Fatalf("seed %d: %+v in a run with life boxed", seed, ev)
			}
		}
		w := res.World
		if s := w.Stats; s.Bodies+s.Fallen+s.Arrests+s.Bails+s.Wounded+s.Retired > 0 || len(w.Crew.Fallen) > 0 {
			t.Fatalf("seed %d: life in the stats with the table boxed: %+v", seed, s)
		}
		for _, m := range append(w.Crew.Members, w.Crew.Candidates...) {
			if m.Age != 0 || m.Growth != 0 || len(m.Kin) > 0 || m.JailedUntil+m.WoundedUntil != 0 {
				t.Fatalf("seed %d: %+v carries life with the table boxed", seed, m)
			}
		}
	}
}

// A jailed member sells nothing: on every morning of a crewed run with
// the police arresting everyone they find, nobody in a cell or laid up
// stands on a corner or in a house, so the served demand is the worked
// corners' as TestHeldDemandIsServed pins it; and the arrests land
// where the sweep was.
func TestJailedMembersWorkNothing(t *testing.T) {
	cfg := *content.MustLoad()
	cfg.Crew.Life.ArrestChance = 1
	arrests := 0
	for seed := uint64(1); seed <= 3; seed++ {
		crewed := Crewed(&cfg, 60) // hot enough for the stings
		res, err := Run(&cfg, seed, 120, func(w *game.World) {
			crewed(w)
			for _, m := range w.Crew.Members {
				if m.Fit(w.Day) {
					continue
				}
				if w.PostOf(m.ID) != nil || w.GuardOf(m.ID) != nil {
					t.Fatalf("seed %d day %d: %s is in a cell or laid up and on %v", seed, w.Day, m.Name, w.PostOf(m.ID))
				}
			}
		})
		if err != nil {
			t.Fatal(err)
		}
		var sweep events.Enforcement
		for _, e := range res.Events {
			switch ev := e.(type) {
			case events.Enforcement:
				if ev.Level == content.Sting || ev.Level == content.Raid {
					sweep = ev
				}
			case events.CrewArrested:
				arrests++
				if ev.Route == "" && ev.Corner != "" && (sweep.Day != ev.Day-1 || sweep.City != ev.City || !contains(sweep.Corners, ev.Corner)) {
					t.Fatalf("seed %d day %d: %s arrested on %s, the sweep was %+v", seed, ev.Day, ev.Name, ev.Corner, sweep)
				}
			}
		}
	}
	if arrests == 0 {
		t.Fatal("three hot crewed runs and nobody was arrested")
	}
}

func contains(xs []string, x string) bool {
	for _, y := range xs {
		if y == x {
			return true
		}
	}
	return false
}

// The war has bodies: a hit war has more of them at day 120 than the
// territory player who never sends anyone in, the count never
// decreases, both sides are in it, and a death is notoriety and
// pressure the next morning: the same seed with reputation.toml's and
// law.toml's body knobs zeroed reads lower on the morning of the first
// death and nowhere before it.
func TestWarHasBodies(t *testing.T) {
	cfg := content.MustLoad()
	warBodies, quietBodies := 0, 0
	for seed := uint64(1); seed <= 5; seed++ {
		last := 0
		war, err := Run(cfg, seed, 120, func(w *game.World) {
			if w.Stats.Bodies < last {
				t.Fatalf("seed %d day %d: bodies went %d -> %d", seed, w.Day, last, w.Stats.Bodies)
			}
			last = w.Stats.Bodies
		})
		if err != nil {
			t.Fatal(err)
		}
		_ = war
		war, _ = Run(cfg, seed, 120, Warlike(cfg, 40, 4, events.ForceHit))
		quiet, _ := Run(cfg, seed, 120, Territory(cfg, 40, 4))
		warBodies += war.World.Stats.Bodies
		quietBodies += quiet.World.Stats.Bodies
		yours, theirs := 0, 0
		for _, e := range war.Events {
			if ev, ok := e.(events.CrewShot); ok && ev.Dead {
				if ev.Theirs {
					theirs++
				} else {
					yours++
				}
			}
		}
		if yours+theirs != war.World.Stats.Bodies || yours != war.World.Stats.Fallen || yours != len(war.World.Crew.Fallen) {
			t.Fatalf("seed %d: %d of yours and %d of theirs dead, stats %d bodies %d fallen, %d on the list", seed, yours, theirs, war.World.Stats.Bodies, war.World.Stats.Fallen, len(war.World.Crew.Fallen))
		}
	}
	t.Logf("five hit wars: %d bodies; five territory runs: %d", warBodies, quietBodies)
	if warBodies <= quietBodies {
		t.Fatalf("the hit war has %d bodies at day 120, territory %d", warBodies, quietBodies)
	}

	// The morning of a death: notoriety and pressure higher than the
	// same morning with the body knobs off, and the same before it.
	deaf := *cfg
	deaf.Reputation.Notoriety.Body = 0
	deaf.Law.Pressure.Body = 0
	for seed := uint64(1); seed <= 5; seed++ {
		var noto, press []float64
		var notoDeaf, pressDeaf []float64
		home := cfg.City.Home().ID
		res, _ := Run(cfg, seed, 120, func(w *game.World) {
			noto = append(noto, w.Player.Reputation.Notoriety)
			press = append(press, w.Cities[home].Pressure)
			Warlike(cfg, 40, 4, events.ForceHit)(w)
		})
		_, _ = Run(&deaf, seed, 120, func(w *game.World) {
			notoDeaf = append(notoDeaf, w.Player.Reputation.Notoriety)
			pressDeaf = append(pressDeaf, w.Cities[home].Pressure)
			Warlike(&deaf, 40, 4, events.ForceHit)(w)
		})
		first := 0
		for _, e := range res.Events {
			if ev, ok := e.(events.CrewShot); ok && ev.Dead && ev.City == home {
				first = ev.Day
				break
			}
		}
		if first == 0 || first >= len(noto) {
			continue
		}
		for d := 1; d < first; d++ {
			if noto[d] != notoDeaf[d] || press[d] != pressDeaf[d] {
				t.Fatalf("seed %d day %d: the body knobs moved something before the first death on day %d", seed, d, first)
			}
		}
		if noto[first] <= notoDeaf[first] || press[first] <= pressDeaf[first] {
			t.Fatalf("seed %d: the morning after the first death (day %d) notoriety %.2f / pressure %.2f, with the knobs off %.2f / %.2f", seed, first, noto[first], press[first], notoDeaf[first], pressDeaf[first])
		}
		return // one seed with a death at home is the proof
	}
	t.Fatal("five hit wars and nobody died at home")
}

// The driver: on the same seeds the distributor with a driver on its
// route is seized less than the one without, the driver rides every
// shipment the route sends once assigned and fit, and a seized
// shipment jails its driver the same night.
func TestDriverCutsSeizures(t *testing.T) {
	cfg := content.MustLoad()
	with, without, driven, jailed := 0, 0, 0, 0
	for seed := uint64(1); seed <= 8; seed++ {
		a, _ := Run(cfg, seed, 150, Driven(cfg, 40))
		b, _ := Run(cfg, seed, 150, Distributor(cfg, 40))
		with += a.World.Stats.Seizures
		without += b.World.Stats.Seizures
		arrested := map[int]bool{}
		until := map[int]int{} // driver -> the day their cell opens, off the arrests
		for _, e := range a.Events {
			switch ev := e.(type) {
			case events.ShipmentSent:
				if ev.Driver != 0 {
					driven++
					if m := a.World.Crew.Member(ev.Driver); m != nil && m.Role != game.RoleDriver {
						t.Fatalf("seed %d day %d: %s drove a shipment as %s", seed, ev.Day, m.Name, m.Role)
					}
				}
			case events.ShipmentSeized:
				if ev.Driver != 0 && until[ev.Driver] <= ev.Day {
					arrested[ev.Day] = true // unless a seizure before put them in the cell already
				}
			case events.CrewArrested:
				until[ev.ID] = ev.Day + ev.Days
				if ev.Route != "" {
					if !arrested[ev.Day] {
						t.Fatalf("seed %d day %d: %s jailed off route %s with no seizure that night", seed, ev.Day, ev.Name, ev.Route)
					}
					jailed++
					delete(arrested, ev.Day)
				}
			}
		}
		if len(arrested) > 0 {
			t.Fatalf("seed %d: a driven shipment was seized on days %v and nobody went to a cell", seed, arrested)
		}
		t.Logf("seed %d: %d seizures with a driver, %d without; driven %d, jailed %d so far", seed, a.World.Stats.Seizures, b.World.Stats.Seizures, driven, jailed)
	}
	if driven == 0 {
		t.Fatal("the driven policy never put a driver on a shipment")
	}
	if with >= without {
		t.Fatalf("with a driver %d seizures over eight seeds, without %d: the driver cuts nothing", with, without)
	}
	// The cut is what the map shows: a skill-60 driver on a route takes
	// driver_cut x 0.6 off the day's risk, compounding with a bought
	// checkpoint's, and a jailed one takes nothing off.
	set, _, _ := sim.Default(cfg)
	w := sim.NewWorld(cfg, 1)
	r := cfg.Routes.Routes[0]
	base := set.Logistics.DayRisk(w, r, events.ShipNormal)
	w.Crew.NextID++
	w.Crew.Members = append(w.Crew.Members, game.CrewMember{ID: w.Crew.NextID, Name: "Wheels", Role: game.RoleDriver, Skill: 60, Loyalty: 60, Wage: 88})
	if err := w.SetRouteDriver(r.ID, w.Crew.NextID); err != nil {
		t.Fatal(err)
	}
	cut := cfg.Crew.Role[game.RoleDriver].DriverCut * 0.6
	if got := set.Logistics.DayRisk(w, r, events.ShipNormal); got < base*(1-cut)-1e-9 || got > base*(1-cut)+1e-9 {
		t.Fatalf("day risk %.4f with a skill-60 driver, base %.4f, want %.4f", got, base, base*(1-cut))
	}
	w.Crew.Members[len(w.Crew.Members)-1].JailedUntil = w.Day + 5
	if got := set.Logistics.DayRisk(w, r, events.ShipNormal); got != base {
		t.Fatalf("day risk %.4f with the driver in a cell, base %.4f", got, base)
	}
}
