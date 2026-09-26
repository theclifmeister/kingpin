package rivals_test

import (
	"testing"

	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
)

// Every proposal gets an answer line (#506): one the dice answer is an
// acceptance or a refusal with no Why, the only kind with a headline;
// one put to a faction finished before it could answer is a refusal
// with a Why, counted nowhere, rather than nothing at all.
func TestEveryProposalIsAnswered(t *testing.T) {
	cfg := duel()
	w, s := arrived(t, cfg, 4, "defensive")
	if err := w.Propose(game.DealTruce, game.Terms{Days: cfg.Rivals.Diplomacy.TruceDays[0]}); err != nil {
		t.Fatal(err)
	}
	evs := step(w, s)
	if k := kinds(evs); k["DealAccepted"]+k["DealRefused"] != 1 {
		t.Fatalf("a proposal the dice answer: %v", k)
	}
	if ev, ok := first[events.DealRefused](evs); ok && ev.Why != "" {
		t.Fatalf("a refusal on the dice carries a Why: %+v", ev)
	}

	w, s = arrived(t, cfg, 4, "defensive")
	if err := w.Propose(game.DealTruce, game.Terms{Days: cfg.Rivals.Diplomacy.TruceDays[0]}); err != nil {
		t.Fatal(err)
	}
	w.Rival().Fragmented = w.Day // finished before the night it would answer
	refused := w.Stats.DealsRefused
	evs = step(w, s)
	ev, ok := first[events.DealRefused](evs)
	if !ok || ev.Why == "" || ev.Faction != w.Rival().Faction() || ev.Deal != game.DealTruce || ev.Rival != w.Rival().Leader {
		t.Fatalf("a proposal nobody was left to answer: %+v (%v)", ev, kinds(evs))
	}
	if w.Stats.DealsRefused != refused {
		t.Fatalf("an unanswered proposal counted as refused: %d, was %d", w.Stats.DealsRefused, refused)
	}

	// No proposal, no line.
	w, s = arrived(t, cfg, 4, "defensive")
	if _, ok := first[events.DealRefused](step(w, s)); ok {
		t.Fatal("a refusal with nothing proposed")
	}
}

// A truce's end is said a day ahead (#506): the night before its last,
// DealEnding; on its last, DealEnded, as ever, and no second warning.
func TestTruceEndIsSaidADayAhead(t *testing.T) {
	cfg := duel()
	w, s := arrived(t, cfg, 4, "defensive")
	w.Rival().Deals = []game.Deal{{Kind: game.DealTruce, Terms: game.Terms{Days: 4}, Since: w.Day, Until: w.Day + 4}}
	until := w.Day + 4
	var ending, ended int
	for w.Day < until && len(w.Rival().Deals) > 0 {
		evs := step(w, s)
		if ev, ok := first[events.DealEnding](evs); ok {
			if ending != 0 || ev.Until != until || ev.Deal != game.DealTruce || ev.Rival != w.Rival().Leader {
				t.Fatalf("day %d: the warning %+v (last on day %d)", w.Day, ev, ending)
			}
			ending = w.Day
		}
		if _, ok := first[events.DealEnded](evs); ok {
			ended = w.Day
		}
	}
	if ended == 0 || ending != ended-1 || ended != until-1 {
		t.Fatalf("warned on day %d, ended on day %d, the truce held until %d", ending, ended, until)
	}
}

// A hit on the scouts has its word in the morning (#506): landed it is
// ScoutsHit; a faction gone before the enforcers got there is
// ScoutsMissed, saying why, and nothing is set back.
func TestHitOnTheScoutsIsReported(t *testing.T) {
	w, s, cfg := expansionWorld(t, 4, 3)
	e := cfg.Rivals.Expansion
	daily := e.TakeMin/e.WindowDays + 1
	for day := 1; day <= e.WindowDays; day++ {
		night(w, s, daily)
	}
	r := w.Rivals[3]
	if !r.Scouting() {
		t.Fatalf("not scouting: %+v", *r)
	}
	w.Crew.Members = append(w.Crew.Members, game.CrewMember{ID: 9, Name: "Tank", Role: game.RoleEnforcer, Skill: 50, Loyalty: 70, Nerve: 50})
	if err := w.HitScouts(r.Faction()); err != nil {
		t.Fatal(err)
	}
	r.Fragmented = w.Day // gone before the night
	evs := night(w, s, daily)
	if _, ok := first[events.ScoutsHit](evs); ok {
		t.Fatal("a gone faction's scouts were hit")
	}
	ev, ok := first[events.ScoutsMissed](evs)
	if !ok || ev.Faction != r.Faction() || ev.Rival != r.Leader || ev.Why == "" {
		t.Fatalf("the hit that found nobody: %+v (%v)", ev, kinds(evs))
	}
	if r.ScoutsHit != 0 || r.Grudge != 0 {
		t.Fatalf("a miss set them back or cost a grudge: %+v", *r)
	}
	// With no hit ordered, no word.
	if _, ok := first[events.ScoutsMissed](night(w, s, daily)); ok {
		t.Fatal("a miss with no hit ordered")
	}
}
