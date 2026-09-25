package game

import (
	"errors"
	"fmt"
	"math"
	"sort"
)

// Intel (#45): what the player knows against what is true. World.Intel
// is a file of Facts, one per (Subject, Kind), latest wins, written by
// the sims that own the truth through their own rule (a push reveals a
// faction's muscle within a band, a raid the chief, a seizure a route's
// risk, the scout the books, #43's Observed personality; a cop paid,
// a spy under, a faction feeding you lies) and read by the UI's rival,
// law and route panels through Known, never off the truth. The harness
// policies keep reading the world: they tune the sims, they are not
// players.
//
// A fact ages: its confidence falls Stale a day from the day it was
// learnt and it is forgotten under Forget (Now, Alive), both stamped on
// the fact by its writer from intel.toml, so nothing steps the file and
// a save reads the same on any build. A planted fact carries the
// faction that fed it in Planted, which the sims read to make it bite
// and the screen never does; until it bites its Source is SourceContact
// ("a contact says"), after it the faction's id.

// Fact kinds. A subject is a faction id, a city id (the police there),
// a route id or SubjectChief.
const (
	FactPersonality = "personality" // a faction's temper, or the chief's under SubjectChief
	FactMuscle      = "muscle"      // heads: Number, or a band in Value
	FactCash        = "cash"        // the books (#70): the chest
	FactIncome      = "income"      // ... the take a day
	FactWages       = "wages"       // ... the wage bill a day
	FactMove        = "move"        // the corner a faction moves on next: Value the corner id
	FactStash       = "stash"       // the corner whose till is fattest: Value the corner id
	FactResponse    = "response"    // the police's next rung in a city: Value the level, Number the first day it can fire
	FactRisk        = "risk"        // a route's risk a day in transit: Number
	FactScout       = "scout"       // a faction moving on a city (#341): Value the city id, Number the day it arrives
)

// SubjectChief is the chief's subject: one chief at a time, the law
// sim's.
const SubjectChief = "chief"

// Sources: how a fact was learnt, what the screen says beside it.
const (
	SourceSeen    = "seen"    // observation: something that happened to you
	SourceBooks   = "books"   // a scout that read the books (#70)
	SourceCop     = "cop"     // a cop paid
	SourceSpy     = "spy"     // a crew member under
	SourceContact = "contact" // "a contact says": a planted fact that has not bitten yet
)

// Unknown is what a panel shows where the file has no fact.
const Unknown = "?"

// Fact is one thing the player knows, or thinks they do.
type Fact struct {
	Subject    string
	Kind       string
	Value      string  // the fact in words: "expansionist", "4–6", a corner id, a level
	Number     float64 // the fact as a number where it has one: heads, dollars, a risk a day, a day
	Confidence float64 // 0..1 the day it was learnt
	Day        int     // the day it was learnt
	Source     string  // SourceSeen, SourceBooks, SourceCop, SourceSpy, SourceContact, or a faction id once a plant has bitten
	Stale      float64 // what the confidence loses a day; 0 never fades
	Forget     float64 // the confidence under which the fact is dropped
	Planted    string  // the faction that fed it, "" for a true one; the sims read it, the screen never does
}

// Now is the fact's confidence on day: what it was learnt at, less
// Stale for every day since, in 0..1.
func (f Fact) Now(day int) float64 {
	age := max(0, day-f.Day)
	return max(0, min(1, f.Confidence-f.Stale*float64(age)))
}

// Alive reports whether the fact still holds on day: over its Forget
// line, and over zero.
func (f Fact) Alive(day int) bool {
	c := f.Now(day)
	return c > 0 && c >= f.Forget
}

// Age is how many days old the fact is on day.
func (f Fact) Age(day int) int { return max(0, day-f.Day) }

// Lure reports whether the fact is a plant still waiting to bite: fed
// by a faction and not yet found out.
func (f Fact) Lure() bool { return f.Planted != "" && f.Source == SourceContact }

// Learn files a fact: it replaces the one with the same subject and kind,
// drops every fact dead on the fact's day, and clamps the confidence.
func (w *World) Learn(f Fact) {
	f.Confidence = max(0, min(1, f.Confidence))
	kept := w.Intel[:0]
	for _, o := range w.Intel {
		if o.Subject == f.Subject && o.Kind == f.Kind {
			continue
		}
		if !o.Alive(f.Day) {
			continue
		}
		kept = append(kept, o)
	}
	w.Intel = append(kept, f)
}

// Unlearn drops the fact with the subject and kind, if any: a chief
// replaced is a chief you know nothing about.
func (w *World) Unlearn(subject, kind string) {
	kept := w.Intel[:0]
	for _, o := range w.Intel {
		if o.Subject == subject && o.Kind == kind {
			continue
		}
		kept = append(kept, o)
	}
	w.Intel = kept
}

// Expose marks a planted fact as bitten: its source becomes the faction
// that fed it, for the screen to say so. It returns the faction id, or
// "" when the fact is no lure.
func (w *World) Expose(subject, kind string) string {
	for i := range w.Intel {
		f := &w.Intel[i]
		if f.Subject == subject && f.Kind == kind && f.Lure() {
			f.Source = f.Planted
			return f.Planted
		}
	}
	return ""
}

// Lure is the planted fact waiting to bite on the subject and kind, or
// nil.
func (w *World) Lure(subject, kind string) *Fact {
	for i := range w.Intel {
		f := &w.Intel[i]
		if f.Subject == subject && f.Kind == kind && f.Lure() && f.Alive(w.Day) {
			return f
		}
	}
	return nil
}

// Knowledge is the file read on a day: what the panels show. Known(w)
// builds it; every accessor answers with Unknown, a zero or false where
// the file has nothing alive.
type Knowledge struct {
	w   *World
	day int
}

// Known is the file as it reads today.
func Known(w *World) Knowledge { return Knowledge{w: w, day: w.Day} }

// Fact is the live fact on the subject and kind.
func (k Knowledge) Fact(subject, kind string) (Fact, bool) {
	for _, f := range k.w.Intel {
		if f.Subject == subject && f.Kind == kind && f.Alive(k.day) {
			return f, true
		}
	}
	return Fact{}, false
}

// Facts is every live fact, the newest first, then by subject and kind:
// the intel screen's rows.
func (k Knowledge) Facts() []Fact {
	var out []Fact
	for _, f := range k.w.Intel {
		if f.Alive(k.day) {
			out = append(out, f)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Day != out[j].Day {
			return out[i].Day > out[j].Day
		}
		if out[i].Subject != out[j].Subject {
			return out[i].Subject < out[j].Subject
		}
		return out[i].Kind < out[j].Kind
	})
	return out
}

// Personality is the faction's temper as known, or Unknown.
func (k Knowledge) Personality(faction string) string {
	if f, ok := k.Fact(faction, FactPersonality); ok {
		return f.Value
	}
	return Unknown
}

// Chief is the chief's temper as known, or Unknown.
func (k Knowledge) Chief() string { return k.Personality(SubjectChief) }

// Muscle is the faction's heads as known: a band (lo..hi; equal for an
// exact read) and the fact, or ok false.
func (k Knowledge) Muscle(faction string) (lo, hi int, f Fact, ok bool) {
	f, ok = k.Fact(faction, FactMuscle)
	if !ok {
		return 0, 0, f, false
	}
	lo, hi = f.Band()
	return lo, hi, f, true
}

// MuscleWord is the heads as the screen prints them: "5", "4–6" or
// Unknown.
func (k Knowledge) MuscleWord(faction string) string {
	f, ok := k.Fact(faction, FactMuscle)
	if !ok {
		return Unknown
	}
	return f.Value
}

// Band is a muscle fact's band: the Value "lo–hi", or Number twice.
func (f Fact) Band() (lo, hi int) {
	if n, _ := fmt.Sscanf(f.Value, "%d–%d", &lo, &hi); n == 2 {
		return lo, hi
	}
	n := int(math.Round(f.Number))
	return n, n
}

// Books is the faction's books as last read (#70): the muscle, the
// chest, the take and the wage bill, their day the day of the last
// read. Read() says whether any of it is known.
type Books struct {
	Day    int
	Cash   int
	Income int
	Muscle int
	Wages  int
}

// Read reports whether the books have ever been read.
func (b Books) Read() bool { return b.Day > 0 }

// Age is how many days old the read is on day.
func (b Books) Age(day int) int { return day - b.Day }

// LearnBooks files a read of the faction's books (#70): the four facts
// at full confidence on day, fading at stale to forget.
func (w *World) LearnBooks(faction string, b Books, stale, forget float64) {
	for _, f := range []struct {
		kind string
		n    int
	}{{FactMuscle, b.Muscle}, {FactCash, b.Cash}, {FactIncome, b.Income}, {FactWages, b.Wages}} {
		w.Learn(Fact{Subject: faction, Kind: f.kind, Value: fmt.Sprint(f.n), Number: float64(f.n), Confidence: 1, Day: b.Day, Source: SourceBooks, Stale: stale, Forget: forget})
	}
}

// Books is the faction's books as the file holds them: the facts the
// scout writes (cash, income, wages, muscle), stamped with the day of
// the oldest of them so the age reads the read's.
func (k Knowledge) Books(faction string) Books {
	var b Books
	take := func(kind string, into *int) bool {
		f, ok := k.Fact(faction, kind)
		if !ok || f.Source != SourceBooks {
			return false
		}
		*into = int(math.Round(f.Number))
		if b.Day == 0 || f.Day < b.Day {
			b.Day = f.Day
		}
		return true
	}
	take(FactCash, &b.Cash)
	take(FactIncome, &b.Income)
	take(FactWages, &b.Wages)
	take(FactMuscle, &b.Muscle)
	return b
}

// Response is the police's next rung in the city as known: the level
// and the first day it can fire.
func (k Knowledge) Response(city string) (level string, day int, ok bool) {
	f, ok := k.Fact(city, FactResponse)
	if !ok {
		return "", 0, false
	}
	return f.Value, int(math.Round(f.Number)), true
}

// Risk is the route's risk a day in transit as known.
func (k Knowledge) Risk(route string) (float64, bool) {
	f, ok := k.Fact(route, FactRisk)
	if !ok {
		return 0, false
	}
	return f.Number, true
}

// Move is the corner the faction moves on next, as known.
func (k Knowledge) Move(faction string) (corner string, ok bool) {
	f, ok := k.Fact(faction, FactMove)
	if !ok {
		return "", false
	}
	return f.Value, true
}

// Stash is the corner whose till the faction keeps fattest, as known.
func (k Knowledge) Stash(faction string) (corner string, ok bool) {
	f, ok := k.Fact(faction, FactStash)
	if !ok {
		return "", false
	}
	return f.Value, true
}

// MuscleBand is the band a muscle of n heads is seen within: the band
// of width w it falls in, "lo–hi" (a width of 1 or less is the exact
// count).
func MuscleBand(n, w int) (lo, hi int) {
	if w <= 1 {
		return n, n
	}
	lo = (n / w) * w
	return lo, lo + w - 1
}

// The player's moves (#45): a cop paid, a spy planted. Each queues on
// the day's scratch for the sims to resolve tonight.

var (
	ErrCopPaid    = errors.New("you have already paid a cop today")
	ErrSpying     = errors.New("somebody is already going under tonight")
	ErrUndercover = errors.New("they are under") // a spy works nothing for you
	ErrNotSpy     = errors.New("a lieutenant running a city cannot go under")
	ErrNoIntel    = errors.New("there is nothing to know yet")
)

// CopOrder is a cop paid today: what was in the envelope. The heat sim
// writes the police's next rung off it and the law sim the chief's
// temper, each at cop_accuracy scaled by the amount against cop_price.
type CopOrder struct {
	Amount int
}

// SpyOrder is a crew member going under tonight (#45): whose crew and
// who.
type SpyOrder struct {
	Faction string
	Member  int
}

// PayCop pays a cop amount in dirty cash for a word on the police
// (#45): what the chief is like and when the police here can next
// move, at cop_accuracy for the full price and less for less. The money
// is gone at once; one cop a day.
func (w *World) PayCop(amount int) error {
	if w.Over != nil {
		return ErrGameOver
	}
	if amount <= 0 {
		return ErrBadAmount
	}
	if w.Today.Cop != nil {
		return ErrCopPaid
	}
	if amount > w.Player.DirtyCash {
		return ErrNoDirtyCash
	}
	w.Player.DirtyCash -= amount
	w.Stats.CopsPaid++
	w.Stats.CopCash += amount
	w.Today.Cop = &CopOrder{Amount: amount}
	return nil
}

// PlantSpy sends the member with id under with the faction tonight
// (#45): from tomorrow they stand on no corner and sell nothing (Working
// is false, Post and Guard refuse them), and every spy_days the crew
// sim files what they saw, until they are found. A lieutenant running a
// city stays where they are. One a night.
func (w *World) PlantSpy(faction string, id int) error {
	if w.Over != nil {
		return ErrGameOver
	}
	r := w.Faction(faction)
	if r == nil {
		return ErrNoFaction
	}
	if !r.Alive() {
		return ErrNoRival
	}
	m := w.Crew.Member(id)
	if m == nil {
		return ErrNoMember
	}
	if m.Undercover != "" {
		return ErrUndercover
	}
	if m.Jailed(w.Day) {
		return ErrJailed
	}
	if m.Wounded(w.Day) {
		return ErrWounded
	}
	if m.Runs() {
		return ErrNotSpy
	}
	if w.Today.Spy != nil {
		return ErrSpying
	}
	w.Today.Spy = &SpyOrder{Faction: r.Faction(), Member: id}
	return nil
}

// CancelSpy calls tonight's plant off.
func (w *World) CancelSpy() { w.Today.Spy = nil }

// Spies is who is under, in roster order.
func (c CrewState) Spies() []CrewMember {
	var out []CrewMember
	for _, m := range c.Members {
		if m.Undercover != "" {
			out = append(out, m)
		}
	}
	return out
}

// MigrateBooks is the 15 -> 16 step (#45): the books a pre-16 save
// carried on Rival.Known become facts in the file, the scout's four,
// at full confidence on the day they were read (they fade from there
// as a fresh read would). Load reads the old field off the stream a
// second time (v15); a save with no read leaves the file as it was.
func MigrateBooks(w *World) {
	if w.books == nil {
		return
	}
	reads := w.books.Rivals
	if len(reads) == 0 && w.books.Rival.Known.Day > 0 {
		reads = append(reads, &struct {
			ID    string
			Known Books
		}{FactionRival, w.books.Rival.Known})
	}
	for _, k := range reads {
		if k == nil || k.Known.Day == 0 {
			continue
		}
		id := k.ID
		if id == "" {
			id = FactionRival
		}
		w.LearnBooks(id, k.Known, BooksStale, BooksForget)
	}
	w.books = nil
}

// BooksStale and BooksForget are what a migrated read fades at: the
// file's stale_rate and forget as they stood when the step was written
// (game never reads content, and a migration runs once).
const (
	BooksStale  = 0.05
	BooksForget = 0.2
)

// v15 is what a pre-16 save carried for the books (#70): a snapshot on
// every faction (and on the one rival of a pre-15 save), read off the
// stream a second time for MigrateBooks. The schema is read beside
// them so gob has a field to match on a save with neither.
type v15 struct {
	SchemaVersion int
	Rival         struct{ Known Books }
	Rivals        []*struct {
		ID    string
		Known Books
	}
}
