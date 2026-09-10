package game

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/theclifmeister/kingpin/internal/events"
)

type counter struct{ n int }

func (c *counter) Name() string { return "counter" }
func (c *counter) Step(w *World, t *Tick) {
	c.n++
	// Use the RNG so determinism after reload is actually exercised.
	m := w.Home().Market["a"]
	m.Price = 10 + t.RNG.Float64()
	t.Emit(events.PriceMove{Day: t.Day, City: w.Home().ID, Product: "a", To: m.Price})
}

// testWorld is a one-product city with two corners; the player works the
// first one, the way a fresh run starts.
func testWorld() *World {
	w := NewWorld(99, []StartingCity{{ID: "test", Name: "Testville", HeatMul: 1, Products: []StartingProduct{{ID: "a", Name: "A", Price: 10, Demand: 5}}}}, 500, 100)
	w.Home().Corners = []Corner{
		{ID: "home", City: "test", Name: "Home", Demand: 1, Heat: 1, Risk: 1, Owner: OwnerNone},
		{ID: "docks", City: "test", Name: "Docks", X: 1, Demand: 1.5, Taste: map[string]float64{"a": 2}, Heat: 0.5, Risk: 2, Owner: OwnerNone},
	}
	if err := w.Post("home", You); err != nil {
		panic(err)
	}
	return w
}

// twoCityWorld is testWorld with a second city, Port, where a is cheap,
// one corner there and a route between the two.
func twoCityWorld() *World {
	w := testWorld()
	w.AddCity(StartingCity{ID: "port", Name: "Port", HeatMul: 0.5, Wholesale: true, Products: []StartingProduct{{ID: "a", Name: "A", Price: 4, Demand: 2}}})
	w.Cities["port"].Corners = []Corner{{ID: "wharf", City: "port", Name: "Wharf", Demand: 1, Heat: 1, Risk: 1, Owner: OwnerNone}}
	return w
}

// testShipment is a two-day car shipment between the test cities as the
// logistics sim would put it on the road on day 3, before Send gives it
// an id.
func testShipment(units int) Shipment {
	return Shipment{Route: "road", Mode: "car", From: "test", To: "port", Product: "a", Units: units, Dial: events.ShipNormal, Sent: 3, Arrives: 5, Cost: units * 2}
}

func TestSaveRoundTripIsDeterministic(t *testing.T) {
	t.Setenv("KINGPIN_HOME", t.TempDir())
	// Run A: 10 days straight.
	a := testWorld()
	ca := NewClock(nil, &counter{})
	var evA []events.Event
	for i := 0; i < 10; i++ {
		evA = append(evA, ca.EndDay(a)...)
	}
	// Run B: 5 days, save, load, 5 more.
	b := testWorld()
	cb := NewClock(nil, &counter{})
	var evB []events.Event
	for i := 0; i < 5; i++ {
		evB = append(evB, cb.EndDay(b)...)
	}
	if err := Save(b); err != nil {
		t.Fatal(err)
	}
	b2, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if b2.Day != 5 || b2.Seed != 99 || b2.Cash() != 500 {
		t.Fatalf("loaded world differs: day %d seed %d cash %d", b2.Day, b2.Seed, b2.Cash())
	}
	for i := 0; i < 5; i++ {
		evB = append(evB, cb.EndDay(b2)...)
	}
	if len(evA) != len(evB) {
		t.Fatalf("event counts differ: %d vs %d", len(evA), len(evB))
	}
	for i := range evA {
		if evA[i] != evB[i] {
			t.Fatalf("event %d differs after reload: %#v vs %#v", i, evA[i], evB[i])
		}
	}
	if a.Home().Market["a"].Price != b2.Home().Market["a"].Price {
		t.Fatalf("final price differs: %v vs %v", a.Home().Market["a"].Price, b2.Home().Market["a"].Price)
	}
}

func TestLoadErrors(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("KINGPIN_HOME", dir)
	if _, err := Load(); !errors.Is(err, ErrNoSave) {
		t.Fatalf("expected ErrNoSave, got %v", err)
	}
	p := filepath.Join(dir, "save.gob")
	if err := os.WriteFile(p, []byte("not a gob"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(); err == nil {
		t.Fatal("corrupt save loaded without error")
	}
	w := testWorld()
	w.SchemaVersion = SchemaVersion + 1
	if err := Save(w); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(); !errors.Is(err, ErrNewerSchema) {
		t.Fatalf("expected ErrNewerSchema, got %v", err)
	}
	if !HasSave() {
		t.Fatal("HasSave false after Save")
	}
	// An older save is upgraded one step at a time; with no path it is
	// refused with a message that says so.
	w.SchemaVersion = SchemaVersion - 1
	w.Day = 12
	if err := Save(w); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(); !errors.Is(err, ErrOldSchema) {
		t.Fatalf("expected ErrOldSchema, got %v", err)
	} else if !strings.Contains(err.Error(), "older version") {
		t.Fatalf("unreadable message: %v", err)
	}
	applied := 0
	up, err := Load(Migration{From: SchemaVersion - 1, Apply: func(w *World) {
		applied++
		if w.SchemaVersion != SchemaVersion-1 || w.Day != 12 {
			t.Fatalf("migration saw schema %d day %d", w.SchemaVersion, w.Day)
		}
	}})
	if err != nil || applied != 1 || up.SchemaVersion != SchemaVersion || up.Day != 12 {
		t.Fatalf("migrate: err %v applied %d schema %d day %d", err, applied, up.SchemaVersion, up.Day)
	}
	// A migration for the wrong version is no path at all.
	if _, err := Load(Migration{From: SchemaVersion - 2, Apply: func(*World) {}}); !errors.Is(err, ErrOldSchema) {
		t.Fatalf("expected ErrOldSchema with an unrelated migration, got %v", err)
	}
	if err := DeleteSave(); err != nil || HasSave() {
		t.Fatalf("delete failed: %v", err)
	}
}

func TestActions(t *testing.T) {
	w := testWorld()
	w.Home().Market["a"].SupplierPrice = 5
	if _, err := w.Buy("a", 200, 0.25); err == nil {
		t.Fatal("bought more than affordable")
	}
	if _, err := w.Buy("a", 101, 0); err == nil {
		t.Fatal("bought more than carry limit")
	}
	p, err := w.Buy("a", 20, 0.25)
	if err != nil || p.Cost != 100 || w.Player.DirtyCash != 400 || w.Stock("test", "a") != 20 {
		t.Fatalf("buy: %v %+v cash=%d stock=%d", err, p, w.Player.DirtyCash, w.Stock("test", "a"))
	}
	if w.Home().Market["a"].SupplierPrice <= 5 {
		t.Fatal("buying did not raise the supplier price")
	}
	if err := w.PlaceSell("test", "a", 21, events.DialNormal); err == nil {
		t.Fatal("sold more than stock")
	}
	if err := w.PlaceSell("nowhere", "a", 1, events.DialNormal); err != ErrNoCity {
		t.Fatalf("sold in a city that does not exist: %v", err)
	}
	if err := w.PlaceSell("test", "a", 20, events.DialQuiet); err != nil {
		t.Fatal(err)
	}
	if o, ok := w.Order("test", "a"); !ok || o.City != "test" || o.Qty != 20 {
		t.Fatalf("order: %+v %v", o, ok)
	}
	w.SetLieLow(true)
	if len(w.Orders) != 0 {
		t.Fatal("lying low should cancel orders")
	}
}

func TestSaveKeepsCrew(t *testing.T) {
	t.Setenv("KINGPIN_HOME", t.TempDir())
	w := testWorld()
	w.Crew.Candidates = []CrewMember{
		{ID: 1, Name: "Dre", Role: "runner", Skill: 60, Loyalty: 55, Greed: 40, Nerve: 70, Units: 36, Wage: 56, Fee: 340},
		{ID: 2, Name: "Tank", Role: "enforcer", Skill: 30, Loyalty: 60, Greed: 20, Nerve: 90, Wage: 45, Fee: 220},
	}
	w.Crew.NextID = 2
	w.Player.DirtyCash = 400
	if _, err := w.Hire(1, 6); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Hire(2, 6); err == nil {
		t.Fatal("hired with too little cash")
	}
	if w.Player.DirtyCash != 60 || w.Capacity("test") != 136 || len(w.Crew.Candidates) != 1 {
		t.Fatalf("after hire: cash %d capacity %d pool %d", w.Player.DirtyCash, w.Capacity("test"), len(w.Crew.Candidates))
	}
	w.SetPay(events.PayGenerous)
	w.Crew.Members[0].Loyalty = 33.5
	w.Crew.Members[0].Informant = true // the hidden flag rides along
	w.Crew.Exposed, w.Crew.Investigated = 1, 2
	w.Heat.LeakDay, w.Heat.Leaks = 4, 2
	if err := Save(w); err != nil {
		t.Fatal(err)
	}
	got, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.Crew.Pay != events.PayGenerous || len(got.Crew.Members) != 1 || got.Crew.Members[0] != w.Crew.Members[0] {
		t.Fatalf("crew did not round-trip: %+v", got.Crew)
	}
	if !got.Crew.Members[0].Informant || got.Crew.Informants() != 1 || got.Crew.Exposed != 1 || got.Crew.Investigated != 2 || got.Heat.LeakDay != 4 || got.Heat.Leaks != 2 {
		t.Fatalf("informant state did not round-trip: %+v %+v", got.Crew, got.Heat)
	}
	if got.Crew.Candidates[0] != w.Crew.Candidates[0] || got.Crew.NextID != 2 {
		t.Fatalf("pool did not round-trip: %+v", got.Crew)
	}
	if _, err := got.Fire(1); err != nil || len(got.Crew.Members) != 0 || len(got.Crew.FiredToday) != 1 {
		t.Fatalf("fire: %v %+v", err, got.Crew)
	}
	if _, err := got.Fire(1); err == nil {
		t.Fatal("fired someone twice")
	}
}

// An investigation is paid up front, one a night, and needs a crew to ask;
// a pay-off buys loyalty at once, capped at 100, from dirty cash first and
// clean for the rest. Both are cleared or reported by the clock.
func TestInvestigateAndPayOff(t *testing.T) {
	w := testWorld()
	if err := w.Investigate(100); err != ErrNoCrew {
		t.Fatalf("investigated an empty payroll: %v", err)
	}
	w.Crew.Members = []CrewMember{{ID: 1, Name: "Dre", Role: "runner", Loyalty: 90, Wage: 50}}
	w.Player.DirtyCash, w.Player.CleanCash = 300, 300
	if err := w.Investigate(1000); err == nil {
		t.Fatal("investigated with too little cash")
	}
	if err := w.Investigate(400); err != nil || w.Investigation == nil || w.Investigation.Cost != 400 {
		t.Fatalf("investigate: %v %+v", err, w.Investigation)
	}
	if w.Player.DirtyCash != 0 || w.Player.CleanCash != 200 {
		t.Fatalf("investigation took the wrong cash: dirty %d clean %d", w.Player.DirtyCash, w.Player.CleanCash)
	}
	if err := w.Investigate(1); err != ErrInvestigating {
		t.Fatalf("second investigation in a day: %v", err)
	}
	if _, err := w.PayOff(2, 100, 25); err != ErrNoMember {
		t.Fatalf("paid off a stranger: %v", err)
	}
	if _, err := w.PayOff(1, 500, 25); err == nil {
		t.Fatal("paid off with too little cash")
	}
	m, err := w.PayOff(1, 150, 25)
	if err != nil || m.Loyalty != 100 || w.Crew.Members[0].Loyalty != 100 || w.Player.CleanCash != 50 {
		t.Fatalf("pay off: %v %+v clean %d", err, m, w.Player.CleanCash)
	}
	if len(w.Crew.PaidOffToday) != 1 || w.Crew.PaidOffToday[0] != (Payoff{ID: 1, Name: "Dre", Cost: 150}) {
		t.Fatalf("pay off not recorded: %+v", w.Crew.PaidOffToday)
	}
	NewClock(nil, &counter{}).EndDay(w)
	if w.Investigation != nil || len(w.Crew.PaidOffToday) != 0 {
		t.Fatalf("scratch not cleared: %+v %+v", w.Investigation, w.Crew.PaidOffToday)
	}
}

// Posting claims a corner and moves people; recalling leaves it held but
// unworked; abandoning gives it back. Demand follows the worked corners.
func TestPostRecallAbandon(t *testing.T) {
	w := testWorld()
	w.Crew.Members = []CrewMember{
		{ID: 1, Name: "Dre", Role: "runner", Skill: 60, Units: 36},
		{ID: 2, Name: "Tank", Role: "enforcer", Skill: 30},
	}
	home, docks := w.Corner("home"), w.Corner("docks")
	if !home.Worked() || home.Runner != You || docks.Held() || w.Held() != 1 || w.Worked() != 1 {
		t.Fatalf("fresh world: home %+v docks %+v", *home, *docks)
	}
	if got := w.Demand("test", "a"); got != 5 {
		t.Fatalf("demand on the home corner = %v, want 5", got)
	}
	if err := w.Post("nowhere", 1); err != ErrNoCorner {
		t.Fatalf("post to a missing corner: %v", err)
	}
	if err := w.Post("docks", 99); err != ErrNoMember {
		t.Fatalf("post a stranger: %v", err)
	}
	w.Day = 4
	if err := w.Post("docks", 1); err != nil {
		t.Fatal(err)
	}
	if !docks.Worked() || docks.Runner != 1 || docks.Since != 4 || w.PostOf(1) != docks {
		t.Fatalf("after posting Dre: %+v", *docks)
	}
	// Docks has share 1.5 * taste 2 = 3 standard corners of product a.
	if got := w.Demand("test", "a"); got != 5*(1+3) {
		t.Fatalf("demand with both corners = %v, want 20", got)
	}
	if err := w.Post("docks", 2); err != nil || docks.Enforcer != 2 || docks.Runner != 1 {
		t.Fatalf("post an enforcer: %v %+v", err, *docks)
	}
	// Moving Dre home replaces you; the docks stay held but unworked.
	if err := w.Post("home", 1); err != nil {
		t.Fatal(err)
	}
	if home.Runner != 1 || docks.Runner != 0 || !docks.Held() || docks.Enforcer != 2 || w.PostOf(You) != nil {
		t.Fatalf("after moving Dre home: home %+v docks %+v", *home, *docks)
	}
	if w.Worked() != 1 || w.Held() != 2 || w.Demand("test", "a") != 5 {
		t.Fatalf("held %d worked %d demand %v", w.Held(), w.Worked(), w.Demand("test", "a"))
	}
	// Firing pulls them off; you can step back on.
	if _, err := w.Fire(1); err != nil || home.Runner != 0 || w.PostOf(1) != nil {
		t.Fatalf("fire: %v %+v", err, *home)
	}
	if err := w.Post("home", You); err != nil || home.Runner != You {
		t.Fatalf("step back on: %v %+v", err, *home)
	}
	if err := w.Abandon("docks"); err != nil || docks.Held() || docks.Enforcer != 0 || w.PostOf(2) != nil {
		t.Fatalf("abandon: %v %+v", err, *docks)
	}
	if err := w.Abandon("docks"); err == nil {
		t.Fatal("abandoned a corner twice")
	}
	docks.Owner = OwnerRival
	if err := w.Post("docks", You); err != ErrCornerTaken {
		t.Fatalf("post on a rival corner: %v", err)
	}
	if err := w.Abandon("home"); err != nil || w.Demand("test", "a") != 0 || w.Worked() != 0 {
		t.Fatalf("abandon home: %v demand %v", err, w.Demand("test", "a"))
	}
}

func TestSaveKeepsCorners(t *testing.T) {
	t.Setenv("KINGPIN_HOME", t.TempDir())
	w := testWorld()
	w.Crew.Members = []CrewMember{{ID: 1, Name: "Dre", Role: "runner"}, {ID: 2, Name: "Tank", Role: "enforcer"}}
	w.Day = 3
	if err := w.Post("docks", 1); err != nil {
		t.Fatal(err)
	}
	if err := w.Post("docks", 2); err != nil {
		t.Fatal(err)
	}
	w.Corner("home").Idle = 2
	if err := Save(w); err != nil {
		t.Fatal(err)
	}
	got, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Home().Corners, w.Home().Corners) {
		t.Fatalf("corners did not round-trip:\n%+v\n%+v", got.Home().Corners, w.Home().Corners)
	}
}

// Enforcers go against a rival corner only, one strike a day, and the
// clock clears it. Borders and contested corners follow the grid, and a
// squeeze cuts a corner's share.
func TestSendEnforcersAndBorders(t *testing.T) {
	w := testWorld()
	home, docks := w.Corner("home"), w.Corner("docks")
	if !home.Borders(*docks) || home.Borders(*home) {
		t.Fatalf("home (%d,%d) docks (%d,%d): borders %v", home.X, home.Y, docks.X, docks.Y, home.Borders(*docks))
	}
	if w.Contested(*home) || w.RivalHeld() != 0 {
		t.Fatal("contested with no rival")
	}
	if err := w.SendEnforcers("docks", events.ForceHit); err == nil {
		t.Fatal("sent enforcers at a free corner")
	}
	docks.Owner = OwnerRival
	if !w.Contested(*home) || !w.Contested(*docks) || w.RivalHeld() != 1 {
		t.Fatal("home and docks should contest each other")
	}
	if err := w.SendEnforcers("docks", events.ForceHit); err != ErrNoEnforcers {
		t.Fatalf("sent enforcers with none: %v", err)
	}
	w.Crew.Members = []CrewMember{{ID: 2, Name: "Tank", Role: "enforcer"}}
	if err := w.SendEnforcers("nowhere", events.ForcePush); err != ErrNoCorner {
		t.Fatalf("sent enforcers nowhere: %v", err)
	}
	if err := w.SendEnforcers("docks", events.ForceWarn); err != nil || w.Strike == nil || w.Strike.Force != events.ForceWarn {
		t.Fatalf("send: %v %+v", err, w.Strike)
	}
	if err := w.SendEnforcers("docks", events.ForceHit); err != nil || w.Strike.Force != events.ForceHit {
		t.Fatalf("sending again should replace: %v %+v", err, w.Strike)
	}
	w.CallOff()
	if w.Strike != nil {
		t.Fatal("call off")
	}
	if err := w.SendEnforcers("docks", events.ForcePush); err != nil {
		t.Fatal(err)
	}
	NewClock(nil, &counter{}).EndDay(w)
	if w.Strike != nil {
		t.Fatal("the clock did not clear the strike")
	}
	home.Squeeze = 0.25
	if got := home.Share("a"); got != 0.75 {
		t.Fatalf("squeezed share %v", got)
	}
	if got := w.Demand("test", "a"); got != 5*0.75 {
		t.Fatalf("squeezed demand %v", got)
	}
}

func TestSaveKeepsRival(t *testing.T) {
	t.Setenv("KINGPIN_HOME", t.TempDir())
	w := testWorld()
	w.Rival = RivalState{Leader: "Big Sal", Personality: "chaotic", Supplier: 0.8, Cash: 1234, Muscle: 3, Arrived: 2, Observed: true, Grudge: 1, War: 33.5, Claims: 2, Flips: 1, Tips: 1}
	w.Corner("docks").Owner = OwnerRival
	w.Corner("home").Squeeze = 0.2
	w.Stats.Strikes, w.Stats.CornersWon, w.Stats.CornersLost = 3, 1, 2
	if err := Save(w); err != nil {
		t.Fatal(err)
	}
	got, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Rival, w.Rival) || !reflect.DeepEqual(got.Home().Corners, w.Home().Corners) || got.Stats != w.Stats {
		t.Fatalf("rival did not round-trip:\n%+v\n%+v", got.Rival, w.Rival)
	}
}

// Fronts, their freezes and audits, and the launder dial survive a save.
func TestSaveKeepsFronts(t *testing.T) {
	t.Setenv("KINGPIN_HOME", t.TempDir())
	w := testWorld()
	w.Player.DirtyCash = 100_000
	w.Stats.PeakCash = 100_000
	w.Day = 6
	offer := FrontOffer{ID: "laundromat", Name: "Laundromat", Cost: 25_000, Throughput: 2_000, Upkeep: 150, AuditRisk: 0.01, UnlockCash: 25_000}
	f, err := w.BuyFront(offer)
	if err != nil || f.Bought != 6 || f.Cost != 25_000 || w.Player.DirtyCash != 75_000 || w.NetWorth() != 100_000 {
		t.Fatalf("buy: %v %+v cash %d worth %d", err, f, w.Player.DirtyCash, w.NetWorth())
	}
	if _, err := w.BuyFront(offer); err != ErrFrontOwned {
		t.Fatalf("bought twice: %v", err)
	}
	if _, err := w.BuyFront(FrontOffer{}); err != ErrNoFront {
		t.Fatalf("bought nothing: %v", err)
	}
	w.Fronts[0].FrozenUntil = 20
	w.Fronts[0].Washed = 12_000
	w.Fronts[0].WashedToday = 2_000
	w.Fronts[0].Audited = 6
	w.Fronts[0].AuditDial = events.LaunderGreedy
	w.SetLaunderDial(events.LaunderCareful)
	w.Player.CleanCash = 9_000
	if err := Save(w); err != nil {
		t.Fatal(err)
	}
	got, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Fronts, w.Fronts) || got.Laundering != w.Laundering || got.Player.CleanCash != 9_000 {
		t.Fatalf("laundering did not round-trip:\n%+v %+v\n%+v %+v", got.Fronts, got.Laundering, w.Fronts, w.Laundering)
	}
	if !got.Fronts[0].Frozen(19) || got.Fronts[0].Frozen(20) {
		t.Fatalf("freeze: %+v", got.Fronts[0])
	}
}

func TestSaveKeepsReputation(t *testing.T) {
	t.Setenv("KINGPIN_HOME", t.TempDir())
	w := testWorld()
	w.Player.Reputation = Reputation{Fear: 61.5, Respect: 12.25, Notoriety: 99}
	if err := Save(w); err != nil {
		t.Fatal(err)
	}
	got, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.Player.Reputation != w.Player.Reputation {
		t.Fatalf("reputation did not round-trip: %+v vs %+v", got.Player.Reputation, w.Player.Reputation)
	}
	for _, a := range Axes {
		if p := got.Player.Reputation.Axis(a); p == nil || *p != *w.Player.Reputation.Axis(a) {
			t.Fatalf("axis %s: %v", a, p)
		}
	}
	if got.Player.Reputation.Axis("charm") != nil {
		t.Fatal("an axis that does not exist")
	}
}

// Fund gives a city clean cash and only clean cash: dirty is refused
// however much of it there is, the amount is checked, and the gift is
// today's scratch for the law sim.
func TestFund(t *testing.T) {
	w := testWorld()
	w.Player.DirtyCash, w.Player.CleanCash = 1_000_000, 0
	if err := w.Fund("test", 100); err != ErrNoCleanCash {
		t.Fatalf("funded from dirty cash: %v", err)
	}
	w.Player.CleanCash = 500
	if err := w.Fund("test", 600); err == nil || err == ErrNoCleanCash {
		t.Fatalf("funded more than the clean cash: %v", err)
	}
	if err := w.Fund("nowhere", 100); err != ErrNoCity {
		t.Fatalf("funded a city that does not exist: %v", err)
	}
	if err := w.Fund("test", 0); err != ErrBadQuantity {
		t.Fatalf("funded nothing: %v", err)
	}
	if err := w.Fund("test", 300); err != nil {
		t.Fatal(err)
	}
	if err := w.Fund("test", 100); err != nil {
		t.Fatal(err)
	}
	if w.Player.CleanCash != 100 || w.Player.DirtyCash != 1_000_000 || w.Stats.Funded != 400 || w.FundedToday("test") != 400 || len(w.Funded) != 2 {
		t.Fatalf("after funding: clean %d dirty %d stats %d today %d %+v", w.Player.CleanCash, w.Player.DirtyCash, w.Stats.Funded, w.FundedToday("test"), w.Funded)
	}
	w.Over = &Ending{Day: 1, Cause: "test"}
	if err := w.Fund("test", 1); err != ErrGameOver {
		t.Fatalf("funded after the end: %v", err)
	}
}
