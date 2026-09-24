package crew_test

import (
	"reflect"
	"slices"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/gametest"
	"github.com/theclifmeister/kingpin/internal/sim/crew"
)

// captainWorld is a home city with three held corners and four on the
// payroll: Cap, who can be captain; Idle, a runner with no corner;
// Skim, a runner under the skim line on the second corner while a skim
// is fresh; and Near, a greedy runner on the third whom tonight's
// drift takes within the captain's margin of the skim line.
func captainWorld(t *testing.T, cfg *content.Config) (*game.World, *crew.Sim, []*game.Corner) {
	t.Helper()
	w, s := world(t, cfg, 100_000)
	w.Crew.Members = []game.CrewMember{
		{ID: 101, Name: "Cap", Role: game.RoleEnforcer, Skill: 50, Loyalty: 80, Nerve: 90, Wage: 50},
		{ID: 102, Name: "Idle", Role: game.RoleRunner, Skill: 50, Loyalty: 60, Nerve: 90, Wage: 50, Units: 100},
		{ID: 103, Name: "Skim", Role: game.RoleRunner, Skill: 50, Loyalty: 20, Nerve: 90, Wage: 50, Units: 100},
		{ID: 104, Name: "Near", Role: game.RoleRunner, Skill: 50, Loyalty: 33, Greed: 100, Nerve: 90, Wage: 50, Units: 100},
	}
	w.Crew.NextID = 105
	w.Day = 5
	w.Crew.LastSkim = w.Day // money went missing this morning
	w.Home().Corners = []game.Corner{
		{ID: "a", City: "test", Name: "A", Demand: 1, Owner: game.OwnerPlayer},
		{ID: "b", City: "test", Name: "B", Demand: 2, X: 1, Owner: game.OwnerPlayer},
		{ID: "c", City: "test", Name: "C", Demand: 1, X: 2, Owner: game.OwnerPlayer},
	}
	var held []*game.Corner
	for i := range w.Home().Corners {
		held = append(held, &w.Home().Corners[i])
	}
	held[1].Runner, held[2].Runner = 103, 104
	return w, s, held
}

// TestCaptainIsTheHandsMoves (#346): a captain's night is the player's
// own actions taken for them. On one seed, a captain named at home pulls
// the suspected skimmer off their corner, posts the idle runner on the
// biggest held corner nobody works, and pays off the member near the
// line; the same three moves made by hand before the same night, with
// nobody named, leave the crew, the corners and the cash exactly where
// the captain's night left them.
func TestCaptainIsTheHandsMoves(t *testing.T) {
	cfg := content.MustLoad()
	cfg.Crew.Captain.Cut = 0 // the cut is the captain's alone; the hand keeps none
	budget := 10_000

	captained, s, _ := captainWorld(t, cfg)
	if err := captained.NameCaptain(101, captained.Home().ID, budget, 0, 0); err != nil {
		t.Fatal(err)
	}
	evs := gametest.StepUnseeded(captained, s).Events()
	var ev events.CaptainActed
	for _, e := range evs {
		if a, ok := e.(events.CaptainActed); ok {
			ev = a
		}
	}
	if !slices.Equal(ev.Pulled, []string{"Skim"}) || len(ev.Posted) != 1 || !slices.Equal(ev.Paid, []string{"Near"}) || ev.Spent != 60*50 {
		t.Fatalf("the captain's night: %+v", ev)
	}

	hand, s2, held := captainWorld(t, cfg)
	hand.Recall(103)
	open := held[0]
	if held[1].Demand > open.Demand {
		open = held[1]
	}
	if open.Name != ev.Posted[0] {
		t.Fatalf("the captain posted on %s, the biggest held corner nobody works is %s", ev.Posted[0], open.Name)
	}
	if err := hand.Post(open.ID, 102); err != nil {
		t.Fatal(err)
	}
	near := hand.Crew.Member(104)
	if _, err := hand.PayOff(104, s2.PayoffCost(*near), s2.PayoffLoyalty()); err != nil {
		t.Fatal(err)
	}
	gametest.StepUnseeded(hand, s2)

	cp := captained.Crew.Member(101)
	cp.Captain, cp.Budget = "", 0
	if !reflect.DeepEqual(captained.Crew.Members, hand.Crew.Members) {
		t.Fatalf("the crew after the captain's night:\n%+v\nby hand:\n%+v", captained.Crew.Members, hand.Crew.Members)
	}
	if !reflect.DeepEqual(captained.Home().Corners, hand.Home().Corners) {
		t.Fatal("the corners after the captain's night are not the hand's")
	}
	if !reflect.DeepEqual(captained.Player, hand.Player) {
		t.Fatalf("the cash after the captain's night %+v, by hand %+v", captained.Player, hand.Player)
	}
}

// A captain in a cell does nothing that night, one under the care line
// stops caring, and one named for nothing is refused: the stakes.
func TestCaptainHasStakes(t *testing.T) {
	cfg := content.MustLoad()
	w, s, _ := captainWorld(t, cfg)
	cp := cfg.Crew.Captain
	if err := w.NameCaptain(102, w.Home().ID, 0, cp.Loyalty, cp.Days); err != game.ErrNotTrusted {
		t.Fatalf("a new hire named captain: %v", err)
	}
	if err := w.NameCaptain(101, w.Home().ID, 10_000, 0, 0); err != nil {
		t.Fatal(err)
	}
	if err := w.NameCaptain(102, w.Home().ID, 10_000, 0, 0); err != game.ErrCaptained {
		t.Fatalf("a second captain at home: %v", err)
	}
	w.Crew.Member(101).JailedUntil = w.Day + 5
	for _, e := range gametest.StepUnseeded(w, s).Events() {
		if a, ok := e.(events.CaptainActed); ok && (!a.Absent || len(a.Posted)+len(a.Paid)+len(a.Pulled) > 0) {
			t.Fatalf("a jailed captain's night: %+v", a)
		}
	}
	if w.PostOf(102) != nil {
		t.Fatal("the idle runner was posted with the captain in a cell")
	}
	c := w.Crew.Member(101)
	c.JailedUntil, c.Loyalty = 0, cp.Care-1
	for _, e := range gametest.StepUnseeded(w, s).Events() {
		if a, ok := e.(events.CaptainActed); ok && (!a.Careless || len(a.Posted)+len(a.Paid)+len(a.Pulled) > 0) {
			t.Fatalf("a captain who stopped caring: %+v", a)
		}
	}
	if w.Crew.Member(101).Informant {
		t.Fatal("a careless captain flipped")
	}
}
