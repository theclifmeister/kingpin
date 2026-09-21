package crew_test

import (
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/sim/crew"
)

// lifeWorld is world with crew.toml's [life] table in: a one-city
// world with a corner to stand on, the crew sim seeded, and every
// member and candidate given an age.
func lifeWorld(t *testing.T, cfg *content.Config, cash int) (*game.World, *crew.Sim) {
	t.Helper()
	w := game.NewWorld(7, []game.StartingCity{{ID: "test", Name: "Testville", Products: []game.StartingProduct{{ID: "a", Name: "A", Price: 10, Demand: 5}}}}, cash, 100)
	w.Home().Corners = []game.Corner{
		{ID: "c1", City: "test", Name: "First", Demand: 1, Heat: 1, Risk: 1, Owner: game.OwnerNone},
		{ID: "c2", City: "test", Name: "Second", Demand: 1, Heat: 1, Risk: 1, Owner: game.OwnerNone},
	}
	s := crew.New(cfg)
	s.Seed(w, game.RNGFor(7, 0))
	return w, s
}

// hire signs a member of a role by hand, aged and at loyalty 70, off
// the dice, so a test reads one thing.
func hire(w *game.World, role string, age int) *game.CrewMember {
	w.Crew.NextID++
	m := game.CrewMember{ID: w.Crew.NextID, Name: "M" + string(rune('A'+w.Crew.NextID%26)), Role: role, Skill: 50, Loyalty: 70, Greed: 20, Nerve: 60, Wage: 50, Fee: 100, Hired: w.Day, Age: age}
	if role == "runner" {
		m.Units = 100
	}
	w.Crew.Members = append(w.Crew.Members, m)
	return w.Crew.Member(m.ID)
}

// Every candidate and every member has an age in the range, off the
// life stream: the same seed's pool with the table boxed draws the
// same names, roles and skills with no age at all.
func TestAgesAreOffTheLifeStream(t *testing.T) {
	cfg := content.MustLoad()
	life := cfg.Crew.Life
	w, _ := lifeWorld(t, cfg, 500)
	boxed, _ := world(t, cfg, 500)
	if len(w.Crew.Candidates) != len(boxed.Crew.Candidates) {
		t.Fatalf("%d faces with life, %d without", len(w.Crew.Candidates), len(boxed.Crew.Candidates))
	}
	for i, c := range w.Crew.Candidates {
		b := boxed.Crew.Candidates[i]
		if c.Age < life.AgeMin || c.Age > life.AgeMax {
			t.Fatalf("%s is %d, want %d..%d", c.Name, c.Age, life.AgeMin, life.AgeMax)
		}
		if b.Age != 0 || c.Name != b.Name || c.Role != b.Role || c.Skill != b.Skill || c.Loyalty != b.Loyalty || c.Fee != b.Fee {
			t.Fatalf("the face moved with the ages on:\n%+v\n%+v", c, b)
		}
	}
	// A save from before ages (13 -> 14) gets them the same way.
	for i := range w.Crew.Candidates {
		w.Crew.Candidates[i].Age = 0
	}
	hire(w, "runner", 0)
	crew.New(cfg).MigrateAges(w)
	for _, c := range append(w.Crew.Candidates, w.Crew.Members...) {
		if c.Age < life.AgeMin || c.Age > life.AgeMax {
			t.Fatalf("after the migration %s is %d", c.Name, c.Age)
		}
	}
}

// Ageing: a member is a year older every year_days on the payroll,
// their skill grows a day to the cap (faster on generous pay), nerve
// falls past nerve_age, and the birthday at retire_age is the
// farewell: a loyal one recommends a kin, a sour one talks.
func TestAgeingAndRetirement(t *testing.T) {
	cfg := content.MustLoad()
	life := cfg.Crew.Life
	w, s := lifeWorld(t, cfg, 100_000)
	w.Player.DirtyCash = 100_000
	m := hire(w, "runner", life.NerveAge+1)
	m.Skill = 30
	w.SetPay(events.PayGenerous)
	id, nerve := m.ID, m.Nerve
	for d := 0; d < life.YearDays; d++ {
		w.Player.DirtyCash = 100_000
		step(w, s)
	}
	m = w.Crew.Member(id)
	if m == nil || m.Age != life.NerveAge+2 {
		t.Fatalf("after %d days: %+v", life.YearDays, m)
	}
	if m.Nerve != nerve-life.NerveLoss {
		t.Fatalf("nerve %d -> %d past nerve_age, want -%d", nerve, m.Nerve, life.NerveLoss)
	}
	want := 30 + int(life.SkillGrowth*cfg.Crew.Pay.Generous.Wage*float64(life.YearDays))
	if m.Skill < want-1 || m.Skill > want+1 {
		t.Fatalf("skill 30 -> %d in %d days on generous pay, want ~%d", m.Skill, life.YearDays, want)
	}
	// A skill-30 runner reaches the cap inside the horizon at generous
	// pay, and never past it.
	for d := 0; d < 200-life.YearDays; d++ {
		w.Player.DirtyCash = 100_000
		step(w, s)
		if m := w.Crew.Member(id); m != nil && m.Skill > life.SkillCap {
			t.Fatalf("skill %d over the cap %d", m.Skill, life.SkillCap)
		}
	}
	if m := w.Crew.Member(id); m != nil && m.Skill != life.SkillCap {
		t.Fatalf("skill %d after 200 days on generous pay, want the cap %d", m.Skill, life.SkillCap)
	}

	// Retirement: the birthday at retire_age. A loyal one recommends a
	// kin, who is in the pool at the discount; a sour one talks on the
	// way out.
	for _, tc := range []struct {
		loyalty float64
		kin     bool
		sour    bool
	}{{90, true, false}, {10, false, true}, {40, false, false}} {
		w, s := lifeWorld(t, cfg, 100_000)
		w.Player.DirtyCash = 100_000
		m := hire(w, "runner", life.RetireAge-1)
		m.Loyalty = tc.loyalty
		_ = w.Post("c1", m.ID)
		id := m.ID
		var retired *events.CrewRetired
		for d := 0; d < life.YearDays && retired == nil; d++ {
			w.Player.DirtyCash = 100_000
			for _, e := range step(w, s) {
				if ev, ok := e.(events.CrewRetired); ok {
					retired = &ev
				}
			}
		}
		if retired == nil || retired.ID != id || retired.Age != life.RetireAge {
			t.Fatalf("loyalty %.0f: no retirement in a year: %+v", tc.loyalty, retired)
		}
		if w.Crew.Member(id) != nil || w.Corner("c1").Runner != 0 || w.Stats.Retired != 1 {
			t.Fatalf("loyalty %.0f: the retiree is still about: %+v", tc.loyalty, w.Corner("c1"))
		}
		if (retired.Kin != "") != tc.kin || retired.Sour != tc.sour {
			t.Fatalf("loyalty %.0f: retired %+v, want kin %v sour %v", tc.loyalty, retired, tc.kin, tc.sour)
		}
		if tc.kin {
			found := false
			for _, c := range w.Crew.Candidates {
				if c.Name == retired.Kin && len(c.Kin) == 1 && c.Kin[0] == id {
					found = true
				}
			}
			if !found {
				t.Fatalf("the recommended kin %s is not in the pool: %+v", retired.Kin, w.Crew.Candidates)
			}
		}
	}
}

// Kin: hiring somebody can put their kin in the pool (an extra face at
// the discount, the usual faces untouched), and kin remember: firing
// one costs the other kin_loyalty and nobody else, paying one off lifts
// the other, an informant's firing costs nothing.
func TestKinRememberEachOther(t *testing.T) {
	cfg := content.MustLoad()
	life := cfg.Crew.Life
	w, s := lifeWorld(t, cfg, 100_000)
	w.Player.DirtyCash = 100_000
	// Hire the pool until a kin face turns up.
	var kin, of *game.CrewMember
	for d := 0; d < 40 && kin == nil; d++ {
		w.Player.DirtyCash = 100_000
		for _, c := range append([]game.CrewMember(nil), w.Crew.Candidates...) {
			if len(c.Kin) == 0 && len(w.Crew.Members) < 3 {
				_, _ = w.Hire(c.ID, 10)
			}
		}
		faces := 0
		for _, c := range w.Crew.Candidates {
			if len(c.Kin) == 0 {
				faces++
			}
		}
		step(w, s)
		for i := range w.Crew.Candidates {
			if c := &w.Crew.Candidates[i]; len(c.Kin) > 0 {
				kin = c
				of = w.Crew.Member(c.Kin[0])
			}
		}
		if n := 0; kin == nil {
			for _, c := range w.Crew.Candidates {
				if len(c.Kin) == 0 {
					n++
				}
			}
			if n != cfg.Crew.Crew.Candidates {
				t.Fatalf("day %d: %d usual faces, want %d", w.Day, n, cfg.Crew.Crew.Candidates)
			}
		}
	}
	if kin == nil || of == nil {
		t.Fatal("no kin came looking in 40 days of hiring")
	}
	if !of.IsKin(kin.ID) || kin.Fee > cfg.Crew.Crew.HireFeeBase+int(cfg.Crew.Crew.HireFeePerSkill*float64(kin.Skill)) {
		t.Fatalf("kin %+v of %+v: not linked or not at the discount", kin, of)
	}
	// Room for them.
	for _, m := range append([]game.CrewMember(nil), w.Crew.Members...) {
		if m.ID != of.ID {
			w.Crew.Members = removeMember(w.Crew.Members, m.ID)
		}
	}
	kid := kin.ID
	if _, err := w.Hire(kid, 10); err != nil {
		t.Fatal(err)
	}
	other := hire(w, "runner", 30)
	w.Crew.HiredToday = nil
	step(w, s)
	a, b, c := w.Crew.Member(of.ID), w.Crew.Member(kid), w.Crew.Member(other.ID)
	if a == nil || b == nil || c == nil {
		t.Fatalf("somebody walked: %+v", w.Crew.Members)
	}
	c.Greed, c.Nerve = b.Greed, b.Nerve // the same drift, so the difference is the kin's
	// A pay-off lifts the kin's loyalty by kin_loyalty on top of the
	// day's drift; the stranger's moves by the drift alone.
	wasB, wasC := b.Loyalty, c.Loyalty
	if _, err := w.PayOff(a.ID, 1, 0); err != nil {
		t.Fatal(err)
	}
	step(w, s)
	b, c = w.Crew.Member(kid), w.Crew.Member(other.ID)
	if d := (b.Loyalty - wasB) - (c.Loyalty - wasC); d < life.KinLoyalty-1e-9 || d > life.KinLoyalty+1e-9 {
		t.Fatalf("a pay-off moved the kin %.1f and the stranger %.1f, want %.0f apart", b.Loyalty-wasB, c.Loyalty-wasC, life.KinLoyalty)
	}
	// A firing costs the kin kin_loyalty more than the stranger.
	wasB, wasC = b.Loyalty, c.Loyalty
	if _, err := w.Fire(a.ID); err != nil {
		t.Fatal(err)
	}
	step(w, s)
	b, c = w.Crew.Member(kid), w.Crew.Member(other.ID)
	if b == nil || c == nil {
		t.Fatalf("somebody walked after the firing: %+v", w.Crew.Members)
	}
	if d := (c.Loyalty - wasC) - (b.Loyalty - wasB); d < life.KinLoyalty-1e-9 || d > life.KinLoyalty+1e-9 {
		t.Fatalf("a firing moved the kin %.1f and the stranger %.1f, want %.0f apart", b.Loyalty-wasB, c.Loyalty-wasC, life.KinLoyalty)
	}
	// An informant's firing costs the kin nothing beyond the drift.
	b.Kin = []int{c.ID}
	c.Kin = []int{b.ID}
	c.Informant = true
	d := hire(w, "runner", 30)
	b = w.Crew.Member(kid)
	d.Greed, d.Nerve = b.Greed, b.Nerve
	w.Crew.HiredToday = nil
	wasB, wasD := b.Loyalty, d.Loyalty
	if _, err := w.Fire(c.ID); err != nil {
		t.Fatal(err)
	}
	step(w, s)
	b, d = w.Crew.Member(kid), w.Crew.Member(d.ID)
	if diff := (b.Loyalty - wasB) - (d.Loyalty - wasD); diff < -1e-9 || diff > 1e-9 {
		t.Fatalf("an informant's firing moved the kin %.1f and the stranger %.1f", b.Loyalty-wasB, d.Loyalty-wasD)
	}
}

func removeMember(ms []game.CrewMember, id int) []game.CrewMember {
	for i, m := range ms {
		if m.ID == id {
			return append(ms[:i:i], ms[i+1:]...)
		}
	}
	return ms
}

// Arrests: the morning after a sweep each runner and enforcer who stood
// on a corner the police hit rolls arrest_chance, a raid that took
// stock jails the chemist, a seized shipment jails its driver with no
// dice; a jailed member is off the corner and Post refuses them; bail
// frees them tomorrow with loyalty up, and an unbailed release comes
// back under the informant line and rolls the turn at once.
func TestArrestsAndBail(t *testing.T) {
	cfg := content.MustLoad()
	sure := *cfg
	sure.Crew.Life.ArrestChance = 1
	life := sure.Crew.Life
	w, s := lifeWorld(t, &sure, 100_000)
	w.Player.DirtyCash = 100_000
	r := hire(w, "runner", 30)
	e := hire(w, "enforcer", 30)
	idle := hire(w, "runner", 30)
	chem := hire(w, game.RoleChemist, 30)
	drv := hire(w, game.RoleDriver, 30)
	w.Crew.HiredToday = nil
	_ = w.Post("c1", r.ID)
	_ = w.Post("c1", e.ID)
	// Last night's sweep, as the heat sim stamps it: the corner, the
	// two on it, and a raid that took stock.
	w.Heat.Sweep = game.Sweep{Day: w.Day + 1, City: "test", Level: content.Raid, Corners: []string{"c1"}, Crew: []int{r.ID, e.ID}, Units: 10}
	w.Day++ // the sweep was the night that brought today
	evs := step(w, s, events.ShipmentSeized{Day: w.Day + 1, Route: "r1", From: "test", Driver: drv.ID, DriverName: drv.Name})
	if k := kinds(evs); k["CrewArrested"] != 4 {
		t.Fatalf("arrests: %v", k)
	}
	for _, id := range []int{r.ID, e.ID, chem.ID, drv.ID} {
		m := w.Crew.Member(id)
		if !m.Jailed(w.Day) || m.JailedUntil != w.Day+life.JailDays || w.PostOf(id) != nil {
			t.Fatalf("%s: %+v, post %v", m.Name, m, w.PostOf(id))
		}
	}
	if m := w.Crew.Member(idle.ID); m.Jailed(w.Day) {
		t.Fatalf("the idle runner was arrested off no corner: %+v", m)
	}
	if w.Stats.Arrests != 4 || w.Crew.Chemist() != nil || w.Crew.Role("enforcer") != 0 || w.Crew.OnPayroll("enforcer") != 1 {
		t.Fatalf("arrests %d, chemist %v, enforcers at work %d on the payroll %d", w.Stats.Arrests, w.Crew.Chemist(), w.Crew.Role("enforcer"), w.Crew.OnPayroll("enforcer"))
	}
	if err := w.Post("c2", r.ID); err != game.ErrJailed {
		t.Fatalf("posting a jailed runner: %v", err)
	}
	// Bail: clean cash only, the whole of it.
	cost := s.BailCost(*r)
	w.Player.CleanCash = cost - 1
	if _, err := w.Bail(r.ID, cost); err == nil {
		t.Fatal("bail paid out of dirty cash")
	}
	w.Player.CleanCash = cost
	loyal := w.Crew.Member(r.ID).Loyalty
	if _, err := w.Bail(r.ID, cost); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Bail(r.ID, cost); err != game.ErrBailed {
		t.Fatalf("bail twice: %v", err)
	}
	if w.Player.CleanCash != 0 || w.Stats.Bails != 1 || w.Stats.BailCash != cost || !w.Crew.Member(r.ID).Bailed {
		t.Fatalf("after the bail: clean %d bails %d", w.Player.CleanCash, w.Stats.Bails)
	}
	evs = step(w, s)
	k := kinds(evs)
	if k["CrewBailed"] != 1 || k["CrewReleased"] != 1 {
		t.Fatalf("the morning after the bail: %v", k)
	}
	if m := w.Crew.Member(r.ID); m.Jailed(w.Day) || m.Bailed || m.Loyalty < loyal+life.BailLoyalty-2 {
		t.Fatalf("bailed: %+v (was %.0f)", m, loyal)
	}
	if err := w.Post("c2", r.ID); err != nil {
		t.Fatalf("posting the bailed runner: %v", err)
	}
	// The others sit it out: the day they are due, they come back
	// under the informant line, and with release_turn at 1 they talk.
	sure.Crew.Life.ReleaseTurn = 1
	s = crew.New(&sure)
	for w.Crew.Member(e.ID).Jailed(w.Day) {
		w.Player.DirtyCash = 100_000
		evs = step(w, s)
	}
	if kinds(evs)["CrewReleased"] != 3 {
		t.Fatalf("the release morning: %v", kinds(evs))
	}
	for _, id := range []int{e.ID, chem.ID, drv.ID} {
		m := w.Crew.Member(id)
		if m.Loyalty > cfg.Crew.Informant.Loyalty-life.JailLoyalty+1 || !m.Informant { // the day's drift on top
			t.Fatalf("%s came back at %.0f, informant %v", m.Name, m.Loyalty, m.Informant)
		}
	}
	if w.Crew.Chemist() == nil || w.Crew.Role("enforcer") != 1 {
		t.Fatal("the chemist and the enforcer are not back at work")
	}
}

// Shot: your enforcers going in and the guard on a corner the rival
// pushed are wounded or killed by the table's odds times the force's
// and the personality's multipliers; a death goes on Fallen and the
// count, both sides count, kin take it hardest, and with the table
// boxed nothing happens at all.
func TestShotOnTheCorner(t *testing.T) {
	cfg := content.MustLoad()
	kill := *cfg
	kill.Crew.Life.KillChance = 1
	w, s := lifeWorld(t, &kill, 100_000)
	w.Player.DirtyCash = 100_000
	w.Rival().Leader, w.Rival().Personality, w.Rival().Muscle = "Dutch", "chaotic", 5
	g := hire(w, "enforcer", 30)
	k := hire(w, "runner", 30)
	g = w.Crew.Member(g.ID) // the roster grew under the pointer
	g.Kin, k.Kin = []int{k.ID}, []int{g.ID}
	w.Crew.HiredToday = nil
	_ = w.Post("c1", g.ID)
	loyal, nerve := k.Loyalty, k.Nerve
	gid, kid := g.ID, k.ID // the roster shrinks under the pointers
	evs := step(w, s, events.RivalPushed{Day: w.Day + 1, Corner: "c1", Name: "First", Rival: "Dutch"})
	dead, theirs := 0, 0
	for _, e := range evs {
		if ev, ok := e.(events.CrewShot); ok && ev.Dead {
			if ev.Theirs {
				theirs++
			} else {
				dead++
				if ev.ID != gid || ev.Corner != "c1" || ev.City != "test" {
					t.Fatalf("shot: %+v", ev)
				}
			}
		}
	}
	if dead != 1 || theirs != 1 || w.Stats.Bodies != 2 || w.Stats.Fallen != 1 || len(w.Crew.Fallen) != 1 || w.Crew.Fallen[0].ID != gid {
		t.Fatalf("dead %d theirs %d bodies %d fallen %d %+v", dead, theirs, w.Stats.Bodies, w.Stats.Fallen, w.Crew.Fallen)
	}
	if w.Crew.Member(gid) != nil || w.Corner("c1").Enforcer != 0 {
		t.Fatalf("the dead are still on the roster or the corner: %+v", w.Corner("c1"))
	}
	kin := w.Crew.Member(kid)
	if kin.Loyalty > loyal-kill.Crew.Life.KinBodyLoyalty+1 || kin.Nerve != nerve+kill.Crew.Life.KinBodyNerve {
		t.Fatalf("the kin: loyalty %.0f -> %.0f, nerve %d -> %d", loyal, kin.Loyalty, nerve, kin.Nerve)
	}

	// A wound: off the corner for wound_days, then back. Nobody to shoot
	// on a push against an unguarded corner.
	wound := *cfg
	wound.Crew.Life.KillChance, wound.Crew.Life.WoundChance = 0, 1
	w, s = lifeWorld(t, &wound, 100_000)
	w.Player.DirtyCash = 100_000
	w.Rival().Leader, w.Rival().Personality = "Dutch", "defensive"
	g = hire(w, "enforcer", 30)
	w.Crew.HiredToday = nil
	_ = w.Post("c1", g.ID)
	evs = step(w, s, events.RivalPushed{Day: w.Day + 1, Corner: "c2", Name: "Second", Rival: "Dutch"})
	if kinds(evs)["CrewShot"] != 0 {
		t.Fatalf("a push on an unguarded corner shot somebody: %v", kinds(evs))
	}
	evs = step(w, s, events.CornerStruck{Day: w.Day + 1, Corner: "c2", Name: "Second", Rival: "Dutch", Force: events.ForceHit})
	if kinds(evs)["CrewShot"] != 1 || w.Stats.Wounded != 1 {
		t.Fatalf("the strike: %v, wounded %d", kinds(evs), w.Stats.Wounded)
	}
	m := w.Crew.Member(g.ID)
	if m == nil || !m.Wounded(w.Day) || m.WoundedUntil != w.Day+wound.Crew.Life.WoundDays || w.PostOf(g.ID) != nil {
		t.Fatalf("wounded: %+v", m)
	}
	if err := w.Post("c1", g.ID); err != game.ErrWounded {
		t.Fatalf("posting the wounded: %v", err)
	}
	for w.Crew.Member(g.ID).Wounded(w.Day) {
		w.Player.DirtyCash = 100_000
		evs = step(w, s)
	}
	if kinds(evs)["CrewRecovered"] != 1 || w.Crew.Member(g.ID).WoundedUntil != 0 {
		t.Fatalf("the recovery: %v", kinds(evs))
	}

	// The table boxed: the same strike and push move nobody.
	w, s = world(t, cfg, 100_000)
	w.Home().Corners = []game.Corner{{ID: "c1", City: "test", Name: "First", Demand: 1, Owner: game.OwnerNone}}
	g = hire(w, "enforcer", 0)
	_ = w.Post("c1", g.ID)
	w.Heat.Sweep = game.Sweep{Day: w.Day + 1, City: "test", Level: content.Raid, Corners: []string{"c1"}, Crew: []int{g.ID}, Units: 10}
	w.Day++
	evs = step(w, s, events.RivalPushed{Day: w.Day + 1, Corner: "c1", Rival: "Dutch"}, events.CornerStruck{Day: w.Day + 1, Corner: "c1", Force: events.ForceHit})
	k2 := kinds(evs)
	if k2["CrewShot"]+k2["CrewArrested"]+k2["CrewRetired"]+k2["KinLooking"] != 0 || w.Stats.Bodies+w.Stats.Arrests != 0 {
		t.Fatalf("with the table boxed: %v", k2)
	}
}

// The driver comes looking once a route has run, an extra face beside
// the usual ones, with a name from the drivers' list.
func TestDriverComesLookingOnceARouteHasRun(t *testing.T) {
	cfg := content.MustLoad()
	w, s := lifeWorld(t, cfg, 500)
	for d := 0; d < 3; d++ {
		step(w, s)
	}
	for _, c := range w.Crew.Candidates {
		if c.Role == game.RoleDriver {
			t.Fatalf("a driver in the pool before a route has run: %+v", c)
		}
	}
	w.Stats.Shipments = 1
	evs := step(w, s)
	drivers, faces := 0, 0
	for _, c := range w.Crew.Candidates {
		if c.Role == game.RoleDriver {
			drivers++
			found := false
			for _, n := range cfg.Names.Drivers {
				found = found || n == c.Name
			}
			if !found || c.Age == 0 || c.Wage <= 0 {
				t.Fatalf("driver %+v is not off the drivers' list", c)
			}
		} else if len(c.Kin) == 0 {
			faces++
		}
	}
	if drivers != 1 || faces != cfg.Crew.Crew.Candidates {
		t.Fatalf("%d drivers beside %d faces, want 1 beside %d", drivers, faces, cfg.Crew.Crew.Candidates)
	}
	if k := kinds(evs); k["Unlocked"] != 1 {
		t.Fatalf("the driver was not announced: %v", k)
	}
	if s.DriverCut(100) != cfg.Crew.Role[game.RoleDriver].DriverCut || s.DriverCut(50) != cfg.Crew.Role[game.RoleDriver].DriverCut/2 {
		t.Fatalf("driver cut at 100: %v, at 50: %v", s.DriverCut(100), s.DriverCut(50))
	}
}

// The bondsman (#230): with auto_bail owned and the clean cash to cover
// it, an arrest is bailed the night it lands (the whole of the role's
// bail, once, out tomorrow with bail_loyalty, the lawyer named on the
// money line); short of the cash the member sits as they always did,
// and the report says the account was short.
func TestBondsmanBailsTheMorningOf(t *testing.T) {
	cfg := content.MustLoad()
	sure := *cfg
	sure.Crew.Life.ArrestChance = 1
	life := sure.Crew.Life
	w, s := lifeWorld(t, &sure, 100_000)
	w.Upgrades["bondsman"] = true
	r := hire(w, "runner", 30)
	e := hire(w, "enforcer", 30)
	w.Crew.HiredToday = nil
	_ = w.Post("c1", r.ID)
	_ = w.Post("c1", e.ID)
	cost := s.BailCost(*r)
	w.Player.CleanCash = cost // the runner's bail and not a dollar more: the enforcer sits
	w.Heat.Sweep = game.Sweep{Day: w.Day + 1, City: "test", Level: content.Sting, Corners: []string{"c1"}, Crew: []int{r.ID, e.ID}}
	w.Day++
	loyal := w.Crew.Member(r.ID).Loyalty
	evs := step(w, s)
	k := kinds(evs)
	if k["CrewArrested"] != 2 || k["CrewBailed"] != 1 {
		t.Fatalf("the night of the arrests: %v", k)
	}
	for _, ev := range evs {
		switch ev := ev.(type) {
		case events.CrewBailed:
			if ev.ID != r.ID || ev.Cost != cost || ev.Who != "the lawyer" {
				t.Fatalf("the bondsman's bail: %+v", ev)
			}
		case events.CrewArrested:
			if ev.ID == r.ID && (!ev.Sprung || ev.Short) || ev.ID == e.ID && (ev.Sprung || !ev.Short) {
				t.Fatalf("the arrest's line: %+v", ev)
			}
		}
	}
	if m := w.Crew.Member(r.ID); !m.Bailed || m.JailedUntil != w.Day+1 || !m.Jailed(w.Day) {
		t.Fatalf("the runner after the bondsman: %+v", m)
	}
	if m := w.Crew.Member(e.ID); m.Bailed || m.JailedUntil != w.Day+life.JailDays {
		t.Fatalf("the enforcer with the account short: %+v", m)
	}
	if w.Player.CleanCash != 0 || w.Stats.Bails != 1 || w.Stats.BailCash != cost || w.Stats.Arrests != 2 {
		t.Fatalf("after the night: clean %d bails %d for %d, arrests %d", w.Player.CleanCash, w.Stats.Bails, w.Stats.BailCash, w.Stats.Arrests)
	}
	if _, err := w.Bail(r.ID, cost); err != game.ErrBailed {
		t.Fatalf("bailing by hand on top of the bondsman: %v", err)
	}
	// The morning: the runner walks with bail_loyalty, the enforcer sits.
	evs = step(w, s)
	if kinds(evs)["CrewReleased"] != 1 {
		t.Fatalf("the morning after: %v", kinds(evs))
	}
	if m := w.Crew.Member(r.ID); m.Jailed(w.Day) || m.Bailed || m.Loyalty < loyal+life.BailLoyalty-2 {
		t.Fatalf("released: %+v (was %.0f)", m, loyal)
	}
	if m := w.Crew.Member(e.ID); !m.Jailed(w.Day) {
		t.Fatalf("the enforcer walked with no bail down: %+v", m)
	}
	// Without the node the same night jails both and pays nothing.
	w, s = lifeWorld(t, &sure, 100_000)
	r = hire(w, "runner", 30)
	w.Crew.HiredToday = nil
	_ = w.Post("c1", r.ID)
	w.Player.CleanCash = 100_000
	w.Heat.Sweep = game.Sweep{Day: w.Day + 1, City: "test", Level: content.Sting, Corners: []string{"c1"}, Crew: []int{r.ID}}
	w.Day++
	evs = step(w, s)
	if k := kinds(evs); k["CrewArrested"] != 1 || k["CrewBailed"] != 0 {
		t.Fatalf("without the node: %v", k)
	}
	if m := w.Crew.Member(r.ID); m.Bailed || m.JailedUntil != w.Day+life.JailDays || w.Player.CleanCash != 100_000 || w.Stats.Bails != 0 {
		t.Fatalf("without the node: %+v, clean %d, bails %d", m, w.Player.CleanCash, w.Stats.Bails)
	}
	for _, ev := range evs {
		if ev, ok := ev.(events.CrewArrested); ok && (ev.Sprung || ev.Short) {
			t.Fatalf("the arrest's line names a bondsman nobody owns: %+v", ev)
		}
	}
}
