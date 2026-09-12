package crew_test

import (
	"reflect"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/sim/crew"
)

func world(t *testing.T, cfg *content.Config, cash int) (*game.World, *crew.Sim) {
	t.Helper()
	w := game.NewWorld(7, []game.StartingCity{{ID: "test", Name: "Testville", Products: []game.StartingProduct{{ID: "a", Name: "A", Price: 10, Demand: 5}}}}, cash, 100)
	s := crew.New(cfg.Crew, cfg.Names, cfg.Reputation.Effects, cfg.Upgrades)
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
		m, err := w.Hire(c.ID, s.MaxCrew(w))
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
	w.SetStock("test", "a", 10)
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
		if _, err := w.Hire(c.ID, s.MaxCrew(w)); err != nil {
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
	if _, err := w.Hire(c.ID, s.MaxCrew(w)); err != nil {
		t.Fatal(err)
	}
	evs := step(w, s)
	if w.Over == nil || w.Over.Cause != "broke" || kinds(evs)["GameOver"] != 1 {
		t.Fatalf("no cash, no stock, wages due: over=%v events=%v", w.Over, kinds(evs))
	}
}

// Accountants only come looking for work once there is a front to keep
// the books of, and when one skims it comes out of the wash, in clean
// cash, not the takings.
func TestAccountantsFollowFronts(t *testing.T) {
	cfg := content.MustLoad()
	w, s := world(t, cfg, 1_000_000)
	for i := 0; i < 40; i++ {
		step(w, s)
		for _, c := range w.Crew.Candidates {
			if c.Role == "accountant" {
				t.Fatalf("day %d: %s is an accountant looking for work with no front to keep", w.Day, c.Name)
			}
		}
	}
	w.Fronts = []game.Front{{ID: "laundromat", Name: "Laundromat", WashedToday: 10_000}}
	seen := false
	for i := 0; i < 60 && !seen; i++ {
		step(w, s)
		for _, c := range w.Crew.Candidates {
			if c.Role == "accountant" {
				seen = true
				if c.Units != 0 || c.Wage <= 0 {
					t.Fatalf("badly generated accountant %+v", c)
				}
			}
		}
	}
	if !seen {
		t.Fatal("no accountant came looking for work in 60 days with a front owned")
	}
	// A disloyal accountant skims the wash.
	w.Crew.Members = []game.CrewMember{{ID: 900, Name: "Books", Role: "accountant", Skill: 50, Loyalty: 20, Greed: 100, Wage: 100}}
	w.Player.CleanCash = 50_000
	dirty := w.Player.DirtyCash
	skimmed := false
	for i := 0; i < 20 && !skimmed; i++ {
		w.Crew.Members[0].Loyalty = 20
		for _, e := range step(w, s, events.PlayerSold{Day: w.Day + 1, Product: "a", Wanted: 10, Sold: 10, Revenue: 100_000}) {
			if sk, ok := e.(events.CrewSkimmed); ok {
				skimmed = true
				if sk.FromWash != sk.Amount || sk.FromWash <= 0 {
					t.Fatalf("accountant skim %+v should come from the wash alone", sk)
				}
				if w.Player.CleanCash != 50_000-sk.Amount {
					t.Fatalf("clean cash %d after a %d skim", w.Player.CleanCash, sk.Amount)
				}
			}
		}
		dirty -= s.Wages(w, w.Crew.Pay)
	}
	if !skimmed {
		t.Fatal("a loyalty-5 accountant never skimmed in 20 days")
	}
	if w.Player.DirtyCash < dirty {
		t.Fatalf("the accountant took from the takings: dirty %d, expected at least %d", w.Player.DirtyCash, dirty)
	}
}

// Only somebody under both the loyalty and the nerve line turns, the event
// is bookkeeping (no name in the ticker is the news sim's job), and an
// investigation names them with the odds the sim shows: at zero it names
// nobody and costs everyone a little loyalty, at one it names them and
// resets what the failures taught.
func TestTurningAndInvestigation(t *testing.T) {
	cfg := content.MustLoad()
	inf := cfg.Crew.Informant
	w, s := world(t, cfg, 1_000_000)
	w.SetPay(events.PayGenerous)
	w.Crew.Members = []game.CrewMember{
		{ID: 1, Name: "Rat", Role: "runner", Skill: 50, Loyalty: inf.Loyalty - 1, Greed: 0, Nerve: inf.Nerve - 1, Wage: 50},
		{ID: 2, Name: "Brave", Role: "runner", Skill: 50, Loyalty: inf.Loyalty - 1, Greed: 0, Nerve: inf.Nerve, Wage: 50},
		{ID: 3, Name: "Loyal", Role: "enforcer", Skill: 50, Loyalty: inf.Loyalty, Greed: 0, Nerve: 0, Wage: 50},
	}
	w.Crew.NextID = 3
	turned := 0
	for day := 0; day < 60 && turned == 0; day++ {
		turned += kinds(step(w, s))["CrewTurnedInformant"]
	}
	if turned != 1 || !w.Crew.Members[0].Informant || w.Crew.Members[1].Informant || w.Crew.Members[2].Informant || w.Stats.Informants != 1 {
		t.Fatalf("after 60 days: %d turned, roster %+v", turned, w.Crew.Members)
	}
	for day := 0; day < 10; day++ {
		if kinds(step(w, s))["CrewTurnedInformant"] != 0 {
			t.Fatal("an informant turned twice")
		}
	}

	// An investigation that cannot succeed.
	cfgZero := *cfg
	cfgZero.Crew.Informant.InvestigateBase, cfgZero.Crew.Informant.InvestigateSkill, cfgZero.Crew.Informant.InvestigateLearn = 0, 0, 0
	zero := crew.New(cfgZero.Crew, cfgZero.Names, cfgZero.Reputation.Effects, cfgZero.Upgrades)
	if got := zero.InvestigateOdds(w); got != 0 {
		t.Fatalf("odds with nothing to go on: %v", got)
	}
	loyal := map[int]float64{}
	for _, m := range w.Crew.Members {
		loyal[m.ID] = m.Loyalty
	}
	control := *w
	control.Crew.Members = append([]game.CrewMember(nil), w.Crew.Members...)
	control.Crew.Candidates = append([]game.CrewMember(nil), w.Crew.Candidates...)
	if err := w.Investigate(inf.InvestigateCost); err != nil {
		t.Fatal(err)
	}
	evs := step(w, zero)
	step(&control, zero)
	w.Investigation = nil
	var run events.InvestigationRun
	for _, e := range evs {
		if ev, ok := e.(events.InvestigationRun); ok {
			run = ev
		}
	}
	if run.Day == 0 || run.Found || run.Name != "" || run.Cost != inf.InvestigateCost {
		t.Fatalf("investigation that cannot succeed reported %+v", run)
	}
	if w.Crew.Investigated != 1 || w.Crew.Exposed != 0 || w.Stats.Investigations != 1 {
		t.Fatalf("after a failed investigation: %+v", w.Crew)
	}
	for i, m := range w.Crew.Members {
		want := control.Crew.Members[i].Loyalty - inf.InvestigateLoyalty
		if m.Loyalty > want+1e-9 || m.Loyalty < want-1e-9 {
			t.Fatalf("%s: loyalty %.1f after a failed investigation, control %.1f", m.Name, m.Loyalty, control.Crew.Members[i].Loyalty)
		}
	}
	// What the failure taught shows in the odds.
	if got, want := s.InvestigateOdds(w), inf.InvestigateBase+inf.InvestigateSkill*0.5+inf.InvestigateLearn; got < want-1e-9 || got > want+1e-9 {
		t.Fatalf("odds after one failure with a skill-50 enforcer: %v, want %v", got, want)
	}

	// And one that cannot fail.
	cfgSure := *cfg
	cfgSure.Crew.Informant.InvestigateBase = 1
	sure := crew.New(cfgSure.Crew, cfgSure.Names, cfgSure.Reputation.Effects, cfgSure.Upgrades)
	if err := w.Investigate(inf.InvestigateCost); err != nil {
		t.Fatal(err)
	}
	evs = step(w, sure)
	w.Investigation = nil
	for _, e := range evs {
		if ev, ok := e.(events.InvestigationRun); ok {
			run = ev
		}
	}
	if !run.Found || run.Name != "Rat" || w.Crew.Exposed != 1 || w.Crew.Investigated != 0 {
		t.Fatalf("investigation that cannot fail reported %+v, crew %+v", run, w.Crew)
	}

	// Firing the informant does not sour the rest.
	loyal = map[int]float64{}
	for _, m := range w.Crew.Members {
		loyal[m.ID] = m.Loyalty
	}
	if _, err := w.Fire(1); err != nil {
		t.Fatal(err)
	}
	evs = step(w, s)
	for _, e := range evs {
		if ev, ok := e.(events.CrewFired); ok && !ev.Informant {
			t.Fatalf("firing the informant reported as %+v", ev)
		}
	}
	for _, m := range w.Crew.Members {
		if m.Loyalty < loyal[m.ID] {
			t.Fatalf("%s lost loyalty (%.1f -> %.1f) over the informant being fired", m.Name, loyal[m.ID], m.Loyalty)
		}
	}
	if w.Crew.Informants() != 0 {
		t.Fatal("the informant is still on the payroll")
	}
}

// crewProbe reads every number the Crew branch can move off a world
// that owns the given nodes (granted, prerequisites ignored: a node is
// measured alone), so TestCrewNodesPullTheirWay can say a node moves
// the ones it names and nothing else. The static ones are the sim's
// public reads; the dice-driven ones (a skim, a turn) are counted over
// many steps on the same seed, and danger's cost is one step's loyalty
// drop for a member with nothing else pulling on them.
func crewProbe(t *testing.T, cfg *content.Config, ids ...string) map[string]float64 {
	t.Helper()
	fresh := func() (*game.World, *crew.Sim) {
		w := game.NewWorld(7, []game.StartingCity{{ID: "test", Name: "Testville", Products: []game.StartingProduct{{ID: "a", Name: "A", Price: 10, Demand: 5}}}}, 1_000_000, 100)
		for _, id := range ids {
			w.Upgrades[id] = true
		}
		s := crew.New(cfg.Crew, cfg.Names, cfg.Reputation.Effects, cfg.Upgrades)
		s.Seed(w, game.RNGFor(7, 0))
		return w, s
	}
	w, s := fresh()
	p := map[string]float64{}
	m := game.CrewMember{ID: 99, Name: "Probe", Role: "runner", Skill: 50, Loyalty: 50, Wage: 50}
	p["wage"] = float64(s.WageAt(w, m, events.PayFair))
	p["loyalty_loss"] = s.LoyaltyLoss(w)
	p["hire_fee"] = float64(s.HireFee(w, 50))
	p["max_crew"] = float64(s.MaxCrew(w))
	p["candidates"] = float64(s.Candidates(w))
	p["pool_days"] = float64(s.PoolDays(w))
	// The first faces of the pool are drawn the same whatever its size,
	// so a bigger pool is more people, not different ones.
	first := w.Crew.Candidates[:cfg.Crew.Crew.Candidates]
	skill, loyalty := 0, 0.0
	for _, c := range first {
		skill += c.Skill
		loyalty += c.Loyalty
	}
	p["pool_skill"] = float64(skill) / float64(len(first))
	p["pool_loyalty"] = loyalty / float64(len(first))

	// Danger: a sting yesterday, one member with no greed and no nerve,
	// nobody to shield them; what fair pay's drift leaves of the loss.
	w, s = fresh()
	w.Crew.Members = []game.CrewMember{{ID: 1, Name: "Probe", Role: "runner", Skill: 50, Loyalty: 50, Wage: 50, Greed: 0, Nerve: 0}}
	w.Heat.LastResponse = map[string]int{"sting": w.Day}
	step(w, s)
	p["danger_drop"] = 50 - w.Crew.Members[0].Loyalty

	// Skims: one member under the line, takings every night, loyalty
	// pinned so they neither climb over it nor walk; the count over a
	// hundred nights on one seed.
	w, s = fresh()
	w.Crew.Members = []game.CrewMember{{ID: 1, Name: "Probe", Role: "runner", Skill: 50, Loyalty: 20, Wage: 50, Greed: 50, Nerve: 100}}
	skims := 0
	for range 100 {
		w.Crew.Members[0].Loyalty = 20
		skims += kinds(step(w, s, events.PlayerSold{Day: w.Day + 1, Product: "a", Sold: 100, Revenue: 1000}))["CrewSkimmed"]
	}
	p["skims"] = float64(skims)

	// Turns: one member under both lines, the flag cleared every morning.
	w, s = fresh()
	w.Crew.Members = []game.CrewMember{{ID: 1, Name: "Probe", Role: "runner", Skill: 50, Loyalty: 15, Wage: 50, Greed: 50, Nerve: 10}}
	turns := 0
	for range 100 {
		w.Crew.Members[0].Loyalty, w.Crew.Members[0].Informant = 15, false
		turns += kinds(step(w, s, events.PlayerSold{Day: w.Day + 1, Product: "a", Sold: 100, Revenue: 1000}))["CrewTurnedInformant"]
	}
	p["turns"] = float64(turns)
	return p
}

// Every Crew node moves the number it names, the way it says, and no
// other (#118). The probe reads each number off the sim the way the
// crew screen does, so what the pane prints is what the dice use.
func TestCrewNodesPullTheirWay(t *testing.T) {
	cfg := content.MustLoad()
	base := crewProbe(t, cfg)
	if base["skims"] == 0 || base["turns"] == 0 || base["danger_drop"] <= 0 {
		t.Fatalf("the probe reads nothing to move: %v", base)
	}
	rows := []struct {
		id    string
		moves map[string]float64 // probe -> the factor (a multiplier) or, for a delta, base+delta
	}{
		{"word", map[string]float64{"candidates": base["candidates"] + 2, "pool_days": base["pool_days"] - 2}},
		{"payroll", map[string]float64{"wage": base["wage"] * 0.9}},
		{"family", map[string]float64{"loyalty_loss": base["loyalty_loss"] * 0.7, "danger_drop": -1}},
		{"hazard", map[string]float64{"danger_drop": -1}},
		{"room", map[string]float64{"max_crew": base["max_crew"] + 2}},
		{"room2", map[string]float64{"max_crew": base["max_crew"] + 2}},
		{"discipline", map[string]float64{"skims": -1}},
		{"training", map[string]float64{"pool_skill": base["pool_skill"] + 10}},
		{"vetting", map[string]float64{"turns": -1}},
		{"bonuses", map[string]float64{"hire_fee": base["hire_fee"] * 0.5, "pool_loyalty": base["pool_loyalty"] + 10}},
	}
	seen := map[string]bool{}
	for _, row := range rows {
		seen[row.id] = true
		got := crewProbe(t, cfg, row.id)
		for k, want := range row.moves {
			switch {
			case want < 0: // a chance: fewer, and not none
				if got[k] >= base[k] || got[k] == 0 {
					t.Errorf("%s: %s is %v with it, %v without; want fewer", row.id, k, got[k], base[k])
				}
			case abs(got[k]-want) > 0.5:
				t.Errorf("%s: %s is %v with it, want %v (base %v)", row.id, k, got[k], want, base[k])
			}
		}
		for k, v := range got {
			if _, named := row.moves[k]; !named && v != base[k] {
				t.Errorf("%s: moves %s (%v -> %v), which it does not name", row.id, k, base[k], v)
			}
		}
	}
	for _, n := range cfg.Upgrades.Branch("crew") {
		if !seen[n.ID] {
			t.Errorf("the Crew branch has %s, which the table does not", n.ID)
		}
	}
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

// factionWorld is the crew's test world with three corners at home, the
// player on the first and the rival on the second (#144).
func factionWorld(t *testing.T, cfg *content.Config) (*game.World, *crew.Sim) {
	t.Helper()
	w, s := world(t, cfg, 100_000)
	w.Home().Corners = []game.Corner{
		{ID: "home", City: "test", Name: "Home", Demand: 1, Heat: 1, Risk: 1, Owner: game.OwnerNone},
		{ID: "docks", City: "test", Name: "Docks", X: 1, Demand: 1, Heat: 1, Risk: 1, Owner: game.OwnerRival, Faction: game.FactionRival, Since: 1},
		{ID: "oldmill", City: "test", Name: "Old Mill", X: 2, Demand: 1, Heat: 1, Risk: 1, Owner: game.OwnerNone},
	}
	w.Rival = game.RivalState{ID: game.FactionRival, Leader: "Big Sal", Personality: "defensive", Cash: 1000, Muscle: 3, Arrived: 1}
	if err := w.Post("home", game.You); err != nil {
		t.Fatal(err)
	}
	return w, s
}

// A defector is a lead on the crew's own queue (#144): the crew sim
// names the faction on the event, writes nothing into the rival, and
// starts the queue afresh every step, so the rivals sim (which steps
// first) reads last night's and only last night's.
func TestDefectionQueuesALeadForTheRival(t *testing.T) {
	cfg := content.MustLoad()
	w, s := factionWorld(t, cfg)
	w.Crew.Members = []game.CrewMember{{ID: 901, Name: "Vee", Role: "runner", Skill: 50, Loyalty: cfg.Crew.Crew.QuitThreshold, Greed: 90, Nerve: 50, Units: 100, Wage: 50}}
	w.Crew.NextID = 901
	if err := w.Post("oldmill", 901); err != nil {
		t.Fatal(err)
	}
	rival := w.Rival
	evs := step(w, s)
	var defected *events.CrewDefected
	for _, e := range evs {
		if ev, ok := e.(events.CrewDefected); ok {
			defected = &ev
		}
	}
	if defected == nil || defected.Rival != "Big Sal" || defected.Faction != game.FactionRival || defected.Corner != "oldmill" {
		t.Fatalf("defection %+v, want one naming the faction and the corner", defected)
	}
	if len(w.Crew.Leads) != 1 || w.Crew.Leads[0] != (game.Lead{Name: "Vee", Corner: "oldmill"}) {
		t.Fatalf("leads %+v, want Vee's", w.Crew.Leads)
	}
	if !reflect.DeepEqual(w.Rival, rival) {
		t.Fatalf("the crew sim wrote into the rival:\n%+v\n%+v", w.Rival, rival)
	}
	if c := w.Corner("oldmill"); !c.Held() || c.Runner != 0 {
		t.Fatalf("the corner the night of the defection: %+v, want it held and unworked until the rival acts", c)
	}
	if step(w, s); w.Crew.Leads != nil {
		t.Fatalf("leads %+v the step after, want the queue empty", w.Crew.Leads)
	}
}

// A lieutenant's walk at home hands the corners to the rival tonight,
// naming the faction on the corners and the event, and books the flip
// with them: the one write into the rival outside its own sim, which
// TestRivalStateHasOneWriter lists (lieutenant.go says why).
func TestWalkHandsTheRivalTheCornersTonight(t *testing.T) {
	cfg := content.MustLoad()
	w, s := factionWorld(t, cfg)
	w.Crew.Members = []game.CrewMember{
		{ID: 901, Name: "Vee", Role: "runner", Skill: 50, Loyalty: 95, Greed: 5, Nerve: 90, Units: 100, Wage: 50},
		{ID: 902, Name: "Mo", Role: "lieutenant", Personality: "steady", Skill: 50, Loyalty: 0, Greed: 5, Nerve: 50, Wage: 50},
	}
	w.Crew.NextID = 902
	if err := w.Post("oldmill", 901); err != nil {
		t.Fatal(err)
	}
	if err := w.Assign(902, "test"); err != nil {
		t.Fatal(err)
	}
	evs := step(w, s)
	var walked *events.LieutenantWalked
	for _, e := range evs {
		if ev, ok := e.(events.LieutenantWalked); ok {
			walked = &ev
		}
	}
	if walked == nil || walked.Rival != "Big Sal" || walked.Faction != game.FactionRival || len(walked.Corners) != 1 || walked.Corners[0] != "Old Mill" {
		t.Fatalf("walk %+v, want Old Mill going to the rival, named", walked)
	}
	if c := w.Corner("oldmill"); c.Owner != game.OwnerRival || c.Faction != game.FactionRival || c.Runner != 0 || c.Since != w.Day {
		t.Fatalf("the corner the night of the walk: %+v", c)
	}
	if c := w.Corner("home"); c.Owner != game.OwnerPlayer || c.Faction != "" || c.Runner != game.You {
		t.Fatalf("the corner you stand on: %+v", c)
	}
	if r := w.Rival; r.Flips != 1 || r.LastFlip != w.Day || !r.Observed {
		t.Fatalf("the flip was not booked the night of the walk: %+v", r)
	}
	if len(w.Crew.Leads) != 0 {
		t.Fatalf("leads %+v after a walk, want none: the corners are already the rival's", w.Crew.Leads)
	}
}
