package game

import (
	"bytes"
	"encoding/gob"
	"math"
	"os"
	"testing"
)

// The file (#45): Learn keys a fact by subject and kind, latest wins,
// and drops the dead; a fact's confidence falls Stale a day, is clamped
// to 0..1 and forgets under Forget; Known answers Unknown, zero or false
// where the file holds nothing alive, and reads a band, a count and the
// books back; a lure is a planted fact until Expose names its author.
func TestFactsAgeAndTheFileReads(t *testing.T) {
	w := testWorld()
	w.Day = 10
	f := Fact{Subject: FactionRival, Kind: FactMuscle, Value: "4–6", Number: 5, Confidence: 0.6, Day: 10, Source: SourceSeen, Stale: 0.1, Forget: 0.25}
	w.Learn(f)
	if got := f.Now(10); got != 0.6 {
		t.Fatalf("now %.2f on the day", got)
	}
	if got := f.Now(12); math.Abs(got-0.4) > 1e-9 {
		t.Fatalf("now %.2f two days on", got)
	}
	if !f.Alive(13) || f.Alive(14) || f.Now(100) != 0 {
		t.Fatalf("alive on day 13 %v, day 14 %v, now on day 100 %.2f", f.Alive(13), f.Alive(14), f.Now(100))
	}
	if got := (Fact{Confidence: 3}).Now(0); got != 1 {
		t.Fatalf("clamped %.2f", got)
	}
	if lo, hi, _, ok := Known(w).Muscle(FactionRival); !ok || lo != 4 || hi != 6 {
		t.Fatalf("the band %d–%d %v", lo, hi, ok)
	}
	if word := Known(w).MuscleWord(FactionRival); word != "4–6" {
		t.Fatalf("the word %q", word)
	}
	if Known(w).Personality(FactionRival) != Unknown || Known(w).Chief() != Unknown || Known(w).MuscleWord("f2") != Unknown {
		t.Fatal("the file answers where it knows nothing")
	}
	// Latest wins on the same key; another key stands beside it.
	w.Learn(Fact{Subject: FactionRival, Kind: FactMuscle, Value: "5", Number: 5, Confidence: 1, Day: 11, Source: SourceBooks})
	w.Learn(Fact{Subject: FactionRival, Kind: FactPersonality, Value: "chaotic", Confidence: 1, Day: 11, Source: SourceSeen})
	w.Day = 11
	if len(w.Intel) != 2 || Known(w).MuscleWord(FactionRival) != "5" || Known(w).Personality(FactionRival) != "chaotic" {
		t.Fatalf("the file: %+v", w.Intel)
	}
	if lo, hi, _, _ := Known(w).Muscle(FactionRival); lo != 5 || hi != 5 {
		t.Fatalf("a count reads %d–%d", lo, hi)
	}
	// Facts is the live file, newest first; a dead fact is dropped by
	// the next Learn and read by nobody before it.
	w.Learn(Fact{Subject: "route", Kind: FactRisk, Value: "~2%/day", Number: 0.02, Confidence: 0.3, Day: 5, Source: SourceSeen, Stale: 0.1, Forget: 0.2})
	if facts := Known(w).Facts(); len(facts) != 2 || facts[0].Day != 11 {
		t.Fatalf("facts %+v", facts)
	}
	if _, ok := Known(w).Risk("route"); ok {
		t.Fatal("a dead fact read")
	}
	w.Learn(Fact{Subject: "x", Kind: "y", Confidence: 1, Day: 11})
	if len(w.Intel) != 3 {
		t.Fatalf("the dead fact was kept: %+v", w.Intel)
	}
	w.Unlearn("x", "y")
	if len(w.Intel) != 2 {
		t.Fatalf("unlearn: %+v", w.Intel)
	}
	// The books read back from the four facts, the day the oldest of
	// them was filed; a band from a push is not the books.
	w.LearnBooks("f2", Books{Day: 9, Cash: 1000, Income: 200, Muscle: 3, Wages: 150}, 0.05, 0.2)
	if b := Known(w).Books("f2"); b != (Books{Day: 9, Cash: 1000, Income: 200, Muscle: 3, Wages: 150}) || !b.Read() || b.Age(11) != 2 {
		t.Fatalf("the books %+v", b)
	}
	if b := Known(w).Books(FactionRival); b.Muscle != 5 || b.Cash != 0 {
		t.Fatalf("the rival's books %+v: the count was the scout's, the rest unread", b)
	}
	// A lure: planted, filed as a contact's word, named once it bites.
	w.Learn(Fact{Subject: "coast", Kind: FactRisk, Value: "~1%/day", Number: 0.01, Confidence: 0.9, Day: 11, Source: SourceContact, Stale: 0.05, Forget: 0.2, Planted: "f2"})
	if l := w.Lure("coast", FactRisk); l == nil || !l.Lure() || w.Lure("coast", FactMuscle) != nil {
		t.Fatalf("the lure %+v", l)
	}
	if fed := w.Expose("coast", FactRisk); fed != "f2" || w.Lure("coast", FactRisk) != nil {
		t.Fatalf("exposed %q, lure %v", fed, w.Lure("coast", FactRisk))
	}
	if f, _ := Known(w).Fact("coast", FactRisk); f.Source != "f2" || w.Expose("coast", FactRisk) != "" {
		t.Fatalf("after the bite %+v", f)
	}
	for _, c := range []struct{ n, w, lo, hi int }{{0, 3, 0, 2}, {4, 3, 3, 5}, {5, 3, 3, 5}, {6, 3, 6, 8}, {7, 1, 7, 7}, {9, 5, 5, 9}} {
		if lo, hi := MuscleBand(c.n, c.w); lo != c.lo || hi != c.hi {
			t.Errorf("band of %d in %d: %d–%d, want %d–%d", c.n, c.w, lo, hi, c.lo, c.hi)
		}
	}
}

// PayCop takes dirty cash at once, once a day; PlantSpy queues one
// member a night, never a lieutenant running a city, one in a cell or
// laid up, or one already under; a spy is not at work, not fit to
// drive, and Post and Guard refuse them.
func TestPayCopAndPlantSpy(t *testing.T) {
	w := testWorld()
	w.Rivals = []*RivalState{{ID: FactionRival, Leader: "Sal", Personality: "defensive", Arrived: 1, Muscle: 3}}
	w.Player.DirtyCash, w.Player.CleanCash = 4000, 100_000
	if err := w.PayCop(5000); err != ErrNoDirtyCash {
		t.Fatalf("a cop on clean cash: %v", err)
	}
	if err := w.PayCop(0); err != ErrBadAmount {
		t.Fatalf("nothing: %v", err)
	}
	if err := w.PayCop(3000); err != nil || w.Player.DirtyCash != 1000 || w.Today.Cop == nil || w.Today.Cop.Amount != 3000 || w.Stats.CopsPaid != 1 || w.Stats.CopCash != 3000 {
		t.Fatalf("paid: %v dirty %d cop %+v stats %+v", err, w.Player.DirtyCash, w.Today.Cop, w.Stats)
	}
	if err := w.PayCop(500); err != ErrCopPaid {
		t.Fatalf("twice: %v", err)
	}
	w.Crew.Members = []CrewMember{
		{ID: 1, Name: "A", Role: "runner", Skill: 50},
		{ID: 2, Name: "B", Role: "enforcer", Skill: 50, JailedUntil: 5},
		{ID: 3, Name: "C", Role: RoleLieutenant, Skill: 50, City: "test"},
		{ID: 4, Name: "D", Role: "runner", Skill: 50},
	}
	if err := w.PlantSpy("nobody", 1); err != ErrNoFaction {
		t.Fatalf("no faction: %v", err)
	}
	if err := w.PlantSpy("", 9); err != ErrNoMember {
		t.Fatalf("nobody: %v", err)
	}
	if err := w.PlantSpy("", 2); err != ErrJailed {
		t.Fatalf("a cell: %v", err)
	}
	if err := w.PlantSpy("", 3); err != ErrNotSpy {
		t.Fatalf("a lieutenant running a city: %v", err)
	}
	if err := w.PlantSpy("", 1); err != nil || w.Today.Spy == nil || w.Today.Spy.Member != 1 || w.Today.Spy.Faction != FactionRival {
		t.Fatalf("planted: %v %+v", err, w.Today.Spy)
	}
	if err := w.PlantSpy("", 4); err != ErrSpying {
		t.Fatalf("two a night: %v", err)
	}
	w.CancelSpy()
	if w.Today.Spy != nil {
		t.Fatal("not called off")
	}
	// Under: nothing works for you.
	m := w.Crew.Member(1)
	m.Undercover, m.UndercoverDay = FactionRival, 1
	if m.Working() || m.Fit(w.Day) || w.Crew.Role("runner") != 1 || w.Crew.OnPayroll("runner") != 2 || len(w.Crew.Spies()) != 1 {
		t.Fatalf("a spy at work: working %v fit %v runners %d of %d", m.Working(), m.Fit(w.Day), w.Crew.Role("runner"), w.Crew.OnPayroll("runner"))
	}
	if err := w.Post("home", 1); err != ErrUndercover {
		t.Fatalf("posting a spy: %v", err)
	}
	if err := w.PlantSpy("", 1); err != ErrUndercover {
		t.Fatalf("planting a spy twice: %v", err)
	}
	w.Rivals[0].Absorbed = 3
	if err := w.PlantSpy("", 4); err != ErrNoRival {
		t.Fatalf("a faction gone: %v", err)
	}
}

// v15World is what a pre-16 save carried for the books (#70): a
// snapshot on every faction.
type v15World struct {
	SchemaVersion int
	Seed          uint64
	Day           int
	Cities        map[string]*City
	CityOrder     []string
	Player        Player
	Upgrades      map[string]bool
	Rivals        []*struct {
		ID     string
		Leader string
		Muscle int
		Known  Books
	}
}

// A save from before intel (#45) files the books its scout read as
// facts on load (MigrateBooks, the 15 -> 16 step): the snapshot reads
// back through the file with its day, an unread faction files nothing,
// and the rival's muscle is nobody's business but the file's.
func TestSaveFilesTheBooksRead(t *testing.T) {
	t.Setenv("KINGPIN_HOME", t.TempDir())
	fresh := testWorld()
	old := v15World{SchemaVersion: 15, Seed: fresh.Seed, Day: 20, Cities: fresh.Cities, CityOrder: fresh.CityOrder, Player: fresh.Player, Upgrades: map[string]bool{}}
	old.Rivals = append(old.Rivals,
		&struct {
			ID     string
			Leader string
			Muscle int
			Known  Books
		}{FactionRival, "Big Sal", 7, Books{Day: 17, Cash: 48_000, Income: 12_000, Muscle: 5, Wages: 9_000}},
		&struct {
			ID     string
			Leader string
			Muscle int
			Known  Books
		}{"f2", "Ray", 2, Books{}})
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(old); err != nil {
		t.Fatal(err)
	}
	p, _ := SavePath(1)
	if err := os.WriteFile(p, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(1); err == nil {
		t.Fatal("a schema-15 save loaded without a migration")
	}
	got, err := Load(1, Migration{From: 15, Apply: MigrateBooks})
	if err != nil {
		t.Fatal(err)
	}
	if got.SchemaVersion != SchemaVersion || len(got.Intel) != 4 {
		t.Fatalf("schema %d, %d facts: %+v", got.SchemaVersion, len(got.Intel), got.Intel)
	}
	if b := Known(got).Books(FactionRival); b != (Books{Day: 17, Cash: 48_000, Income: 12_000, Muscle: 5, Wages: 9_000}) {
		t.Fatalf("the books read back %+v", b)
	}
	if b := Known(got).Books("f2"); b.Read() {
		t.Fatalf("unread books filed: %+v", b)
	}
	if got.Rival().Muscle != 7 || got.Faction("f2").Muscle != 2 {
		t.Fatalf("the truth moved: %d %d", got.Rival().Muscle, got.Faction("f2").Muscle)
	}
	// Loaded again on this schema the file is as it was.
	if err := Save(1, got); err != nil {
		t.Fatal(err)
	}
	again, err := Load(1)
	if err != nil {
		t.Fatal(err)
	}
	if len(again.Intel) != 4 || Known(again).Books(FactionRival) != Known(got).Books(FactionRival) {
		t.Fatalf("the file after a round trip: %+v", again.Intel)
	}
}
