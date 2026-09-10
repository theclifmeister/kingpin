package game

import (
	"errors"
	"os"
	"path/filepath"
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

func testWorld() *World {
	return NewWorld(99, "Testville", []StartingProduct{{ID: "a", Name: "A", Price: 10, Demand: 5}}, 500, 100)
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
	// A save from before the crew existed is refused, not migrated: runs
	// are roguelike. The message must tell the player what to do.
	w.SchemaVersion = SchemaVersion - 1
	if err := Save(w); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(); !errors.Is(err, ErrOldSchema) {
		t.Fatalf("expected ErrOldSchema, got %v", err)
	} else if !strings.Contains(err.Error(), "older version") {
		t.Fatalf("unreadable message: %v", err)
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
