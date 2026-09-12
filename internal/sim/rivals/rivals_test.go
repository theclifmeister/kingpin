package rivals_test

import (
	"math"
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
	territory.New(cfg).Seed(w)
	w.Crew.Members = []game.CrewMember{
		{ID: 1, Name: "Dre", Role: "runner", Skill: 60, Units: 120, Loyalty: 70, Nerve: 50},
		{ID: 2, Name: "Tank", Role: "enforcer", Skill: 50, Loyalty: 70, Nerve: 50},
		{ID: 3, Name: "Moose", Role: "enforcer", Skill: 80, Loyalty: 70, Nerve: 90},
	}
	w.Crew.NextID = 3
	s := rivals.New(cfg)
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
	if r.Cash != int(math.Round(cfg.Rivals.Rivals.StartCash*s.CornerDay(w))) || r.Muscle != cfg.Rivals.Rivals.StartMuscle || r.Arrived != 0 {
		t.Fatalf("seeded rival %+v (a corner-day is %.0f)", r, s.CornerDay(w))
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
	w.Rival.Muscle = 8 // dug in for the war (#139: on one corner its take kept two)
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

// The Street branch's rival nodes (#119): each moves the one number it
// names and nothing else. The front line is a body on every contested
// corner in Guard, so the push odds the map shows fall on a bare corner
// and a guarded one alike and a corner with nobody on it is no longer
// walked onto; held ground cuts the push pace beside fear's cut and
// leaves the guard alone; neither touches the claim pace or the strike
// odds.
func TestRivalNodesMoveTheirNumbers(t *testing.T) {
	cfg := content.MustLoad()
	own := func(ids ...string) (*game.World, *rivals.Sim) {
		w, s := world(t, cfg, 5)
		w.Upgrades = map[string]bool{}
		for _, id := range ids {
			if cfg.Upgrades.Upgrade(id) == nil {
				t.Fatalf("no node %s", id)
			}
			w.Upgrades[id] = true
		}
		w.Rival.Arrived, w.Rival.Muscle = 1, 4
		w.Corner("docks").Owner = game.OwnerRival
		w.Corner("railyard").Owner = game.OwnerPlayer // bare, and borders the docks
		if !w.Contested(*w.Corner("railyard")) {
			t.Fatal("the rail yard does not border the docks")
		}
		return w, s
	}
	plain, s := own()
	bare := plain.Corner("railyard")
	you := plain.Corner(cfg.City.Territory.Start)
	baseBare, baseYou := s.Guard(plain, bare), s.Guard(plain, you)
	if baseBare != 0 || baseYou != 1.5 {
		t.Fatalf("bare guard %.1f, yours %.1f", baseBare, baseYou)
	}
	basePace, baseClaim, baseOdds := s.PushPace(plain), s.ClaimPace(plain), s.Odds(plain, events.ForcePush)
	cases := []struct {
		nodes []string
		guard float64 // added to every corner's guard
		pace  float64 // on the push pace
	}{
		{[]string{"frontline"}, 1, 1},
		{[]string{"ground"}, 0, 0.7},
		{[]string{"frontline", "ground"}, 1, 0.7},
	}
	for _, tc := range cases {
		w, s := own(tc.nodes...)
		b, y := w.Corner("railyard"), w.Corner(cfg.City.Territory.Start)
		if got := s.Guard(w, b); got != baseBare+tc.guard {
			t.Errorf("%v: bare guard %.1f, want %.1f", tc.nodes, got, baseBare+tc.guard)
		}
		if got := s.Guard(w, y); got != baseYou+tc.guard {
			t.Errorf("%v: your guard %.1f, want %.1f", tc.nodes, got, baseYou+tc.guard)
		}
		if got, want := s.PushPace(w), basePace*tc.pace; math.Abs(got-want) > 1e-12 {
			t.Errorf("%v: push pace %.3f, want %.3f", tc.nodes, got, want)
		}
		if s.ClaimPace(w) != baseClaim || s.Odds(w, events.ForcePush) != baseOdds {
			t.Errorf("%v: claim pace %.3f (was %.3f), strike odds %.3f (was %.3f)", tc.nodes, s.ClaimPace(w), baseClaim, s.Odds(w, events.ForcePush), baseOdds)
		}
		// The odds the map shows are the guard's: lower on every corner
		// with the front line, the same without it.
		if tc.guard > 0 && s.PushOdds(w, b) >= s.PushOdds(plain, bare) {
			t.Errorf("%v: push odds on a bare corner %.3f, %.3f without", tc.nodes, s.PushOdds(w, b), s.PushOdds(plain, bare))
		}
		if tc.guard == 0 && s.PushOdds(w, b) != s.PushOdds(plain, bare) {
			t.Errorf("%v: push odds on a bare corner moved to %.3f from %.3f", tc.nodes, s.PushOdds(w, b), s.PushOdds(plain, bare))
		}
	}
	// A defector's lead onto a bare corner walks straight on without the
	// front line, and has to push with it: the corner is no longer
	// unopposed, so the rival rolls the odds the map shows.
	took := func(nodes ...string) bool {
		w, s := own(nodes...)
		w.Crew.Leads = []game.Lead{{Corner: "railyard", Name: "Dre"}}
		evs := step(w, s)
		for _, e := range evs {
			if ct, ok := e.(events.CornerTaken); ok && ct.Corner == "railyard" && ct.Handed == "Dre" {
				return true
			}
		}
		return false
	}
	if !took() {
		t.Fatal("a defector's lead onto a bare corner was not walked onto")
	}
	if took("frontline") {
		t.Log("the front line's push on seed 5 landed; the roll was made either way")
	}
}

// eager is a config in which the rival claims every day it can: the
// claim roll is certain, the pace flat and the cooldown gone, so a
// test reads the tell and its answer on a known day.
func eager(cfg *content.Config) *content.Config {
	boxed := *cfg
	boxed.Rivals.Personality = map[string]content.PersonalityConfig{}
	for k, p := range cfg.Rivals.Personality {
		p.ClaimChance = 1
		boxed.Rivals.Personality[k] = p
	}
	boxed.Rivals.Pace = content.PaceTuning{}
	return &boxed
}

// settled is a world with the rival in town on the last corner, rich
// enough to claim and with muscle enough that it recruits nobody, so
// its cash moves by income and wages alone.
func settled(t *testing.T, cfg *content.Config, seed uint64, personality string) (*game.World, *rivals.Sim) {
	t.Helper()
	w, s := world(t, cfg, seed)
	w.Rival.Personality = personality
	w.Rival.Arrived, w.Rival.Cash, w.Rival.Muscle, w.Rival.Observed = 1, 200_000, 30, true
	c := &w.Home().Corners[len(w.Home().Corners)-1]
	c.Owner, c.Since = game.OwnerRival, 1
	w.Day = 1
	return w, s
}

// The claim is telegraphed (#69): the day the roll succeeds the rival
// eyes a free corner and says so, spending nothing, the cooldown
// running from that day; the next step it sets up on it, claim_cost
// spent and the claim counted, and the tell is cleared.
func TestTellPrecedesTheClaim(t *testing.T) {
	cfg := eager(content.MustLoad())
	for _, p := range content.Personalities {
		w, s := settled(t, cfg, 3, p)
		held, claims := w.RivalHeld(), w.Rival.Claims
		evs := step(w, s)
		k := kinds(evs)
		if k["RivalEyeing"] != 1 || k["CornerTaken"] != 0 {
			t.Fatalf("%s: the first step: %v", p, k)
		}
		if w.Rival.Eyeing == "" || w.Rival.EyeingDay != w.Day || w.Rival.LastClaim != w.Day || w.RivalHeld() != held || w.Rival.Claims != claims {
			t.Fatalf("%s: after the tell: %+v holds %d", p, w.Rival, w.RivalHeld())
		}
		eyed := w.Corner(w.Rival.Eyeing)
		if eyed == nil || eyed.Owner != game.OwnerNone {
			t.Fatalf("%s: eyeing %q, owner %q", p, w.Rival.Eyeing, eyed.Owner)
		}
		for _, e := range evs {
			if ev, ok := e.(events.RivalEyeing); ok && (ev.Corner != eyed.ID || ev.Name != eyed.Name || ev.Rival != w.Rival.Leader) {
				t.Fatalf("%s: the tell names %+v, eyeing %s", p, ev, eyed.ID)
			}
		}
		cash := w.Rival.Cash + s.Income(w) - s.Wages(w)
		k = kinds(step(w, s))
		if k["CornerTaken"] != 1 || k["RivalOutbid"] != 0 {
			t.Fatalf("%s: the second step: %v", p, k)
		}
		if eyed.Owner != game.OwnerRival || w.RivalHeld() != held+1 || w.Rival.Claims != claims+1 {
			t.Fatalf("%s: after the claim: %s is %s's, %+v", p, eyed.ID, eyed.Owner, w.Rival)
		}
		if w.Rival.Cash != cash-s.ClaimCost(w) {
			t.Fatalf("%s: cash %d after the claim, want %d", p, w.Rival.Cash, cash-s.ClaimCost(w))
		}
		// The next tell is given the same step the claim lands only if
		// the pace lets it: with none, the eager rival eyes again at once.
		if w.Rival.Eyeing == "" {
			t.Fatalf("%s: the eager rival did not eye the next corner", p)
		}
	}
}

// The tell is answerable (#69): somebody posted on the eyed corner
// before the rival comes makes the claim fail, no cash spent, no claim
// counted, the grudge up by outbid_grudge (or a call made on it that
// night), the corner still yours; and
// the next tell names another corner. An enforcer on it counts: the
// corner is held, and the rival never walks onto held ground.
func TestPostingOnTheEyedCornerOutbidsTheClaim(t *testing.T) {
	cfg := eager(content.MustLoad())
	tun := cfg.Rivals.Rivals
	for _, who := range []int{game.You, 1, 2} {
		w, s := settled(t, cfg, 5, "expansionist")
		step(w, s)
		eyed := w.Corner(w.Rival.Eyeing)
		if eyed == nil {
			t.Fatal("no tell")
		}
		if err := w.Post(eyed.ID, who); err != nil {
			t.Fatal(err)
		}
		held, claims, grudge := w.RivalHeld(), w.Rival.Claims, w.Rival.Grudge
		cash := w.Rival.Cash + s.Income(w) - s.Wages(w)
		evs := step(w, s)
		k := kinds(evs)
		if k["RivalOutbid"] != 1 || k["CornerTaken"] != 0 {
			t.Fatalf("posting %d: %v", who, k)
		}
		if eyed.Owner != game.OwnerPlayer || w.RivalHeld() != held || w.Rival.Claims != claims {
			t.Fatalf("posting %d: %s is %s's, rival holds %d, claims %d", who, eyed.ID, eyed.Owner, w.RivalHeld(), w.Rival.Claims)
		}
		if w.Rival.Cash != cash {
			t.Fatalf("posting %d: cash %d, want %d unspent", who, w.Rival.Cash, cash)
		}
		// The grudge is held, or paid back the same night with a call.
		if paid := k["RivalTippedPolice"]; w.Rival.Grudge+paid != grudge+tun.OutbidGrudge || paid > 1 {
			t.Fatalf("posting %d: grudge %d, was %d, %d calls", who, w.Rival.Grudge, grudge, paid)
		}
		if w.Rival.Eyeing == "" || w.Rival.Eyeing == eyed.ID {
			t.Fatalf("posting %d: the next tell is %q", who, w.Rival.Eyeing)
		}
	}
}

// A tell the rival can no longer act on is dropped without a word: a
// split sealed overnight that covers the corner, or its share reached.
func TestStaleTellIsDropped(t *testing.T) {
	cfg := eager(content.MustLoad())
	w, s := settled(t, cfg, 7, "expansionist")
	step(w, s)
	eyed := w.Rival.Eyeing
	w.Rival.Deals = []game.Deal{{Kind: game.DealSplit, Terms: game.Terms{Corners: []string{eyed}}, Since: w.Day}}
	held, cash, wages := w.RivalHeld(), w.Rival.Cash, s.Wages(w)
	k := kinds(step(w, s))
	if k["CornerTaken"] != 0 || k["RivalOutbid"] != 0 {
		t.Fatalf("under the split: %v", k)
	}
	if w.Corner(eyed).Owner != game.OwnerNone || w.RivalHeld() != held || w.Rival.Cash < cash-wages {
		t.Fatalf("under the split: %s is %s's, holds %d, cash %d -> %d", eyed, w.Corner(eyed).Owner, w.RivalHeld(), cash, w.Rival.Cash)
	}
	if w.Rival.Eyeing == eyed {
		t.Fatalf("the split corner is eyed again")
	}

	w, s = settled(t, cfg, 7, "defensive")
	step(w, s)
	eyed = w.Rival.Eyeing
	for i := range w.Home().Corners {
		if c := &w.Home().Corners[i]; c.Owner == game.OwnerNone && c.ID != eyed && w.RivalHeld() < s.MaxCorners(w) {
			c.Owner = game.OwnerRival
		}
	}
	k = kinds(step(w, s))
	if k["CornerTaken"] != 0 || w.Corner(eyed).Owner != game.OwnerNone || w.RivalHeld() > s.MaxCorners(w) {
		t.Fatalf("at its share: %v, %s is %s's, holds %d of %d", k, eyed, w.Corner(eyed).Owner, w.RivalHeld(), s.MaxCorners(w))
	}
}
