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
	w.Market["a"].Price = 10 + t.RNG.Float64()
	t.Emit(events.PriceMove{Day: t.Day, Product: "a", To: w.Market["a"].Price})
}

// testWorld is a one-product city with two corners; the player works the
// first one, the way a fresh run starts.
func testWorld() *World {
	w := NewWorld(99, "Testville", []StartingProduct{{ID: "a", Name: "A", Price: 10, Demand: 5}}, 500, 100)
	w.Territory.Corners = []Corner{
		{ID: "home", Name: "Home", Demand: 1, Heat: 1, Risk: 1, Owner: OwnerNone},
		{ID: "docks", Name: "Docks", X: 1, Demand: 1.5, Taste: map[string]float64{"a": 2}, Heat: 0.5, Risk: 2, Owner: OwnerNone},
	}
	if err := w.Post("home", You); err != nil {
		panic(err)
	}
	return w
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
	if a.Market["a"].Price != b2.Market["a"].Price {
		t.Fatalf("final price differs: %v vs %v", a.Market["a"].Price, b2.Market["a"].Price)
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
	w.Market["a"].SupplierPrice = 5
	if _, err := w.Buy("a", 200, 0.25); err == nil {
		t.Fatal("bought more than affordable")
	}
	if _, err := w.Buy("a", 101, 0); err == nil {
		t.Fatal("bought more than carry limit")
	}
	p, err := w.Buy("a", 20, 0.25)
	if err != nil || p.Cost != 100 || w.Player.DirtyCash != 400 || w.Player.Stock["a"] != 20 {
		t.Fatalf("buy: %v %+v cash=%d stock=%d", err, p, w.Player.DirtyCash, w.Player.Stock["a"])
	}
	if w.Market["a"].SupplierPrice <= 5 {
		t.Fatal("buying did not raise the supplier price")
	}
	if err := w.PlaceSell("a", 21, events.DialNormal); err == nil {
		t.Fatal("sold more than stock")
	}
	if err := w.PlaceSell("a", 20, events.DialQuiet); err != nil {
		t.Fatal(err)
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
	if w.Player.DirtyCash != 60 || w.Capacity() != 136 || len(w.Crew.Candidates) != 1 {
		t.Fatalf("after hire: cash %d capacity %d pool %d", w.Player.DirtyCash, w.Capacity(), len(w.Crew.Candidates))
	}
	w.SetPay(events.PayGenerous)
	w.Crew.Members[0].Loyalty = 33.5
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
	if got := w.Demand("a"); got != 5 {
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
	if got := w.Demand("a"); got != 5*(1+3) {
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
	if w.Worked() != 1 || w.Held() != 2 || w.Demand("a") != 5 {
		t.Fatalf("held %d worked %d demand %v", w.Held(), w.Worked(), w.Demand("a"))
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
	if err := w.Abandon("home"); err != nil || w.Demand("a") != 0 || w.Worked() != 0 {
		t.Fatalf("abandon home: %v demand %v", err, w.Demand("a"))
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
	if !reflect.DeepEqual(got.Territory, w.Territory) {
		t.Fatalf("corners did not round-trip:\n%+v\n%+v", got.Territory, w.Territory)
	}
}
