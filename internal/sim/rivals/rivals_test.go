package rivals_test

import (
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/sim/rivals"
	"github.com/theclifmeister/kingpin/internal/sim/territory"
)

// world is a city with the player on the starting corner, a runner and
// two enforcers on the payroll, and a rival picked from the seed.
func world(t *testing.T, cfg *content.Config, seed uint64) (*game.World, *rivals.Sim) {
	t.Helper()
	w := game.NewWorld(seed, []game.StartingCity{{ID: cfg.City.Home().ID, Name: "Testville", Products: []game.StartingProduct{{ID: "weed", Name: "Weed", Price: 20, Demand: 60}}}}, 10_000, 100)
	territory.New(cfg.City).Seed(w)
	w.Crew.Members = []game.CrewMember{
		{ID: 1, Name: "Dre", Role: "runner", Skill: 60, Units: 120, Loyalty: 70, Nerve: 50},
		{ID: 2, Name: "Tank", Role: "enforcer", Skill: 50, Loyalty: 70, Nerve: 50},
		{ID: 3, Name: "Moose", Role: "enforcer", Skill: 80, Loyalty: 70, Nerve: 90},
	}
	w.Crew.NextID = 3
	s := rivals.New(cfg.Rivals, cfg.Names, cfg.Reputation.Effects, cfg.Law.Effects)
	s.Seed(w, game.RNGFor(seed, 0))
	return w, s
}

func step(w *game.World, s *rivals.Sim) []events.Event {
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

// The seed picks a leader, a personality and a connect; the rival arrives
// on schedule on the biggest free corner that does not border the player,
// and a save from before it existed gets one on load.
func TestSeedAndArrival(t *testing.T) {
	cfg := content.MustLoad()
	w, s := world(t, cfg, 3)
	r := w.Rival
	if r.Leader == "" || r.Personality == "" || r.Supplier < cfg.Rivals.Rivals.SupplierMin || r.Supplier > cfg.Rivals.Rivals.SupplierMax {
		t.Fatalf("seeded rival %+v", r)
	}
	if r.Cash != cfg.Rivals.Rivals.StartCash || r.Muscle != cfg.Rivals.Rivals.StartMuscle || r.Arrived != 0 {
		t.Fatalf("seeded rival %+v", r)
	}
	for w.Day < cfg.Rivals.Rivals.ArriveDay-1 {
		if k := kinds(step(w, s)); k["RivalMovedIn"] != 0 {
			t.Fatalf("day %d: arrived early", w.Day)
		}
	}
	evs := step(w, s)
	if k := kinds(evs); k["RivalMovedIn"] != 1 {
		t.Fatalf("day %d: %v", w.Day, k)
	}
	if w.Rival.Arrived != w.Day || w.RivalHeld() != 1 {
		t.Fatalf("after arriving: %+v holds %d", w.Rival, w.RivalHeld())
	}
	you := w.Corner(cfg.City.Territory.Start)
	var got *game.Corner
	for i := range w.Home().Corners {
		if c := &w.Home().Corners[i]; c.Owner == game.OwnerRival {
			got = c
		}
	}
	if got.Borders(*you) {
		t.Fatalf("arrived on %s, right next to you", got.ID)
	}
	for _, c := range w.Home().Corners {
		if c.Owner == game.OwnerNone && !c.Borders(*you) && c.Demand > got.Demand {
			t.Fatalf("arrived on %s (%.1f) with %s (%.1f) free and quiet", got.ID, got.Demand, c.ID, c.Demand)
		}
	}
	// Migration gives an old save a rival without moving anyone.
	old := game.NewWorld(1, []game.StartingCity{{ID: cfg.City.Home().ID, Name: "Testville"}}, 500, 100)
	s.Migrate(old)
	if old.Rival.Leader == "" || old.Rival.Arrived != 0 {
		t.Fatalf("migrated rival %+v", old.Rival)
	}
}

// Claims stop at the personality's cap, and every corner it takes has
// nobody standing on it and exactly one owner.
func TestClaimsRespectTheCap(t *testing.T) {
	cfg := content.MustLoad()
	for _, p := range content.Personalities {
		w, s := world(t, cfg, 11)
		w.Rival.Personality = p
		w.Rival.Cash = 10_000_000
		for w.Day < 200 {
			step(w, s)
			if n := w.RivalHeld(); n > s.MaxCorners(w) {
				// Pushes can carry an expansionist past its cap; claims cannot.
				if p != "expansionist" {
					t.Fatalf("%s day %d: holds %d corners, cap %d", p, w.Day, n, s.MaxCorners(w))
				}
			}
			for _, c := range w.Home().Corners {
				if c.Owner == game.OwnerRival && (c.Runner != 0 || c.Enforcer != 0) {
					t.Fatalf("%s day %d: %s is the rival's with %d/%d on it", p, w.Day, c.ID, c.Runner, c.Enforcer)
				}
			}
		}
		if w.RivalHeld() == 0 {
			t.Fatalf("%s: never held a corner", p)
		}
		if w.Rival.Cash < 0 || w.Rival.Muscle < 0 {
			t.Fatalf("%s: cash %d muscle %d", p, w.Rival.Cash, w.Rival.Muscle)
		}
	}
}

// A strike needs enforcers, draws heat scaled by the corner, is louder
// the harder it goes in, and when it lands the corner is yours with
// nobody on it and the rival holds a grudge.
func TestStrikes(t *testing.T) {
	cfg := content.MustLoad()
	w, s := world(t, cfg, 5)
	docks := w.Corner("docks")
	docks.Owner = game.OwnerRival
	w.Rival.Arrived = 1
	w.Rival.Muscle = 2
	if err := w.SendEnforcers("fourth", events.ForceHit); err == nil {
		t.Fatal("struck your own corner")
	}
	enforcers := w.Crew.Members[1:]
	w.Crew.Members = w.Crew.Members[:1]
	if err := w.SendEnforcers("docks", events.ForceHit); err != game.ErrNoEnforcers {
		t.Fatalf("struck with no enforcers: %v", err)
	}
	w.Crew.Members = append(w.Crew.Members, enforcers...)
	if got := s.Odds(w, events.ForceWarn); !(got > 0 && got < s.Odds(w, events.ForcePush) && s.Odds(w, events.ForcePush) < s.Odds(w, events.ForceHit)) {
		t.Fatalf("odds are not monotone in force: %.2f %.2f %.2f", got, s.Odds(w, events.ForcePush), s.Odds(w, events.ForceHit))
	}
	if s.StrikeHeat(docks, events.ForceHit) != cfg.Rivals.Force["hit"].Heat*docks.Heat {
		t.Fatalf("strike heat %.1f", s.StrikeHeat(docks, events.ForceHit))
	}
	taken, tries := false, 0
	for !taken && tries < 50 {
		if err := w.SendEnforcers("docks", events.ForceHit); err != nil {
			t.Fatal(err)
		}
		warBefore := w.Rival.War
		evs := step(w, s)
		w.Strike = nil // the clock clears it
		tries++
		var cs *events.CornerStruck
		for _, e := range evs {
			if ev, ok := e.(events.CornerStruck); ok {
				cs = &ev
			}
		}
		if cs == nil {
			t.Fatalf("day %d: no CornerStruck: %v", w.Day, kinds(evs))
		}
		if cs.Heat <= 0 || cs.Toll != cfg.Rivals.Force["hit"].Loyalty || cs.Force != events.ForceHit {
			t.Fatalf("strike event %+v", *cs)
		}
		if w.Rival.War <= warBefore && warBefore < 100 {
			t.Fatalf("day %d: war %.1f did not rise from %.1f", w.Day, w.Rival.War, warBefore)
		}
		taken = cs.Taken
		if taken {
			if !cs.Routed || docks.Owner != game.OwnerPlayer || docks.Runner != 0 || docks.Since != w.Day {
				t.Fatalf("taken: %+v corner %+v", *cs, *docks)
			}
			// The grudge may already have been paid back with a tip tonight.
			if w.Rival.Grudge+w.Rival.Tips != 1 || w.Rival.Routed != w.Day {
				t.Fatalf("after losing its last corner: %+v", w.Rival)
			}
		}
	}
	if !taken {
		t.Fatalf("%d hits with two enforcers against muscle 2 and never took the docks", tries)
	}
	if w.Stats.Strikes != tries || w.Stats.CornersWon != 1 {
		t.Fatalf("stats %+v", w.Stats)
	}
}

// Undercutting squeezes exactly the player corners that border the
// rival's, drags the price, and lifts when the border goes.
func TestUndercutFollowsTheBorder(t *testing.T) {
	cfg := content.MustLoad()
	w, s := world(t, cfg, 8)
	w.Rival.Arrived = 1
	w.Rival.Personality = "defensive"
	w.Rival.Muscle = 0                           // so it cannot push in this window
	w.Corner("docks").Owner = game.OwnerRival    // borders fourth, where you stand
	if err := w.Post("heights", 1); err != nil { // far from the docks
		t.Fatal(err)
	}
	price := w.Home().Market["weed"].Price
	evs := step(w, s)
	share := cfg.Rivals.Personality["defensive"].Undercut
	if f := w.Corner("fourth"); f.Squeeze != share || !w.Contested(*f) {
		t.Fatalf("fourth squeeze %.2f contested %v", f.Squeeze, w.Contested(*f))
	}
	if h := w.Corner("heights"); h.Squeeze != 0 {
		t.Fatalf("heights squeezed %.2f with no rival nearby", h.Squeeze)
	}
	if w.Home().Market["weed"].Price >= price {
		t.Fatalf("price %.2f did not drop from %.2f", w.Home().Market["weed"].Price, price)
	}
	found := false
	for _, e := range evs {
		if u, ok := e.(events.RivalUndercut); ok {
			found = true
			if len(u.Corners) != 1 || u.Corners[0] != "Fourth & Main" || u.Share != share {
				t.Fatalf("undercut event %+v", u)
			}
		}
	}
	if !found {
		t.Fatalf("no RivalUndercut: %v", kinds(evs))
	}
	// Share and demand shrink by the squeeze.
	if got, want := w.Corner("fourth").Share("weed"), 1.0*(1-share); got != want {
		t.Fatalf("fourth share %.3f, want %.3f", got, want)
	}
	// The rival driven off the docks lifts the squeeze.
	w.Corner("docks").Owner = game.OwnerNone
	step(w, s)
	if f := w.Corner("fourth"); f.Squeeze != 0 {
		t.Fatalf("fourth still squeezed %.2f after the rival left", f.Squeeze)
	}
}

// A grudge is paid back with a tip to the police, and a war loud enough
// ends in a crackdown that clears both sides and resets the meter.
func TestTipsAndCrackdown(t *testing.T) {
	cfg := content.MustLoad()
	w, s := world(t, cfg, 2)
	w.Rival.Arrived = 1
	w.Rival.Personality = "defensive"
	w.Rival.Grudge = 3
	w.Corner("docks").Owner = game.OwnerRival
	tips := 0
	for w.Day < 60 && tips < 3 {
		for _, e := range step(w, s) {
			if tp, ok := e.(events.RivalTippedPolice); ok {
				tips++
				if tp.Heat != cfg.Rivals.Rivals.TipHeat || tp.Rival != w.Rival.Leader {
					t.Fatalf("tip %+v", tp)
				}
			}
		}
	}
	if tips != 3 || w.Rival.Grudge != 0 || w.Rival.Tips != 3 {
		t.Fatalf("%d tips, rival %+v", tips, w.Rival)
	}
	if k := kinds(step(w, s)); k["RivalTippedPolice"] != 0 {
		t.Fatal("tipped with no grudge")
	}

	// A crackdown: contested corners on both sides go to none.
	w.Corner("depot").Owner = game.OwnerRival // borders fourth, where you stand
	w.Corner("railyard").Owner = game.OwnerRival
	if err := w.Post("projects", 1); err != nil { // borders depot
		t.Fatal(err)
	}
	if err := w.Post("projects", 2); err != nil {
		t.Fatal(err)
	}
	w.Rival.War = cfg.Rivals.Rivals.CrackdownThreshold
	muscle := w.Rival.Muscle
	evs := step(w, s)
	var we *events.WarEscalated
	lost := map[string]string{}
	for _, e := range evs {
		switch ev := e.(type) {
		case events.WarEscalated:
			we = &ev
		case events.CornerLost:
			lost[ev.Corner] = ev.Owner
		}
	}
	if we == nil || we.Stage != "crackdown" || we.Heat != cfg.Rivals.Rivals.CrackdownHeat || len(we.Lost) != 2*cfg.Rivals.Rivals.CrackdownCorners {
		t.Fatalf("crackdown %+v", we)
	}
	if lost["fourth"] != game.OwnerPlayer || lost["projects"] != game.OwnerPlayer || lost["depot"] != game.OwnerRival {
		t.Fatalf("lost %v", lost)
	}
	for id := range lost {
		if c := w.Corner(id); c.Owner != game.OwnerNone || c.Runner != 0 || c.Enforcer != 0 {
			t.Fatalf("%s after the crackdown: %+v", id, *c)
		}
	}
	if w.Rival.War != 0 || w.Rival.Muscle >= muscle {
		t.Fatalf("after the crackdown: %+v (muscle was %d)", w.Rival, muscle)
	}
	if w.PostOf(1) != nil || w.PostOf(2) != nil || w.PostOf(game.You) != nil {
		t.Fatal("somebody is still standing on cleared ground")
	}
}

// A loud city gets the rival's calls answered (#41): with a grudge to
// pay back, it tips the police more often at pressure 100 than at 0, and
// the pace it reads is the home city's, scaled by the law's knob.
func TestTipsRiseWithPressure(t *testing.T) {
	cfg := content.MustLoad()
	w, s := world(t, cfg, 1)
	if s.TipPace(w) != 1 {
		t.Fatalf("tip pace at pressure 0: %.2f", s.TipPace(w))
	}
	w.Home().Pressure = 100
	if want := 1 + cfg.Law.Effects.PressureTip; s.TipPace(w) != want {
		t.Fatalf("tip pace at pressure 100: %.2f, want %.2f", s.TipPace(w), want)
	}
	tips := func(pressure float64) int {
		n := 0
		for seed := uint64(1); seed <= 40; seed++ {
			w, s := world(t, cfg, seed)
			w.Rival.Arrived, w.Rival.Cash, w.Rival.Muscle = 1, 100_000, 3
			w.Rival.Personality = "defensive"
			w.Day = 10
			w.Home().Pressure = pressure
			w.Rival.Grudge = 5
			for d := 0; d < 10; d++ {
				w.Rival.Grudge = 5
				w.Home().Pressure = pressure
				n += kinds(step(w, s))["RivalTippedPolice"]
			}
		}
		return n
	}
	loud, quiet := tips(100), tips(0)
	t.Logf("tips over 40 seeds x 10 days: %d at pressure 100, %d at 0", loud, quiet)
	if loud <= quiet {
		t.Fatalf("pressure should make the calls land: %d at 100 vs %d at 0", loud, quiet)
	}
}

// max_share is a share of home's map (#60): on today's ten corners it
// reproduces the six, three, four and four corners the personalities
// set up on before it was a share, and a bigger map gives them more.
func TestMaxShareReproducesTheCaps(t *testing.T) {
	cfg := content.MustLoad()
	w, s := world(t, cfg, 1)
	want := map[string]int{"expansionist": 6, "defensive": 3, "opportunist": 4, "chaotic": 4}
	if n := len(w.Home().Corners); n != 10 {
		t.Fatalf("home has %d corners; the table below is for ten", n)
	}
	for p, n := range want {
		w.Rival.Personality = p
		if got := s.MaxCorners(w); got != n {
			t.Errorf("%s: max corners %d on ten, want %d", p, got, n)
		}
	}
	w.Home().Corners = append(w.Home().Corners, w.Home().Corners...)
	w.Rival.Personality = "expansionist"
	if got := s.MaxCorners(w); got != 12 {
		t.Errorf("expansionist: max corners %d on twenty, want 12", got)
	}
}
