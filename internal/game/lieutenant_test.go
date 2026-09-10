package game

import (
	"errors"
	"testing"

	"github.com/theclifmeister/kingpin/internal/events"
)

func lieutenantWorld() *World {
	w := NewWorld(1, []StartingCity{
		{ID: "home", Name: "Home", Products: []StartingProduct{{ID: "weed", Name: "Weed", Price: 20, Demand: 60}}},
		{ID: "hub", Name: "Hub", Products: []StartingProduct{{ID: "weed", Name: "Weed", Price: 26, Demand: 60}}},
	}, 1000, 100)
	w.Crew.Members = []CrewMember{
		{ID: 1, Name: "Dre", Role: "runner", Units: 100},
		{ID: 2, Name: "Marcus", Role: RoleLieutenant, Personality: "steady"},
		{ID: 3, Name: "Sly", Role: RoleLieutenant, Personality: "greedy"},
	}
	return w
}

// Assign checks the member, the role and the city, allows one lieutenant
// per city, and moving cities drops the standing orders of the old one;
// Unassign and Fire drop them too.
func TestAssignAndUnassign(t *testing.T) {
	w := lieutenantWorld()
	w.Day = 5
	for _, tc := range []struct {
		id   int
		city string
		err  error
	}{
		{9, "hub", ErrNoMember},
		{1, "hub", ErrNotLieutenant},
		{2, "nowhere", ErrNoCity},
		{2, "hub", nil},
		{3, "hub", ErrCityRun},
		{2, "hub", nil}, // again is a no-op
		{3, "home", nil},
	} {
		if err := w.Assign(tc.id, tc.city); !errors.Is(err, tc.err) {
			t.Fatalf("Assign(%d, %s) = %v, want %v", tc.id, tc.city, err, tc.err)
		}
	}
	m := w.Crew.Member(2)
	if m.City != "hub" || m.Assigned != 5 || w.Crew.Lieutenant("hub") != m || w.Crew.Lieutenants() != 2 {
		t.Fatalf("after assigning: %+v, %d running", *m, w.Crew.Lieutenants())
	}
	w.Delegate("hub", "weed", 10, events.DialQuiet)
	w.Delegate("home", "weed", 20, events.DialNormal)
	if o, ok := w.StandingOrder("hub", "weed"); !ok || o.Qty != 10 || o.Dial != events.DialQuiet || o.City != "hub" {
		t.Fatalf("standing order in the hub: %+v %v", o, ok)
	}
	// Moving Marcus home is refused while Sly runs it; unassigning Sly
	// drops home's order, and then Marcus can move, dropping the hub's.
	if err := w.Assign(2, "home"); !errors.Is(err, ErrCityRun) {
		t.Fatalf("moving onto a run city: %v", err)
	}
	if err := w.Unassign(3); err != nil {
		t.Fatal(err)
	}
	if _, ok := w.StandingOrder("home", "weed"); ok || w.Crew.Member(3).City != "" {
		t.Fatal("unassigning kept the standing order")
	}
	if err := w.Assign(2, "home"); err != nil {
		t.Fatal(err)
	}
	if _, ok := w.Delegated[OrderKey("hub", "weed")]; ok {
		t.Fatal("moving cities kept the old city's standing order")
	}
	if _, ok := w.StandingOrder("hub", "weed"); ok {
		t.Fatal("a standing order in a city nobody runs")
	}
	w.Delegate("home", "weed", 5, events.DialNormal)
	if _, err := w.Fire(2); err != nil {
		t.Fatal(err)
	}
	if _, ok := w.Delegated[OrderKey("home", "weed")]; ok {
		t.Fatal("firing the lieutenant kept their standing order")
	}
	if err := w.Unassign(1); !errors.Is(err, ErrNotLieutenant) {
		t.Fatalf("Unassign a runner: %v", err)
	}
	w.Over = &Ending{}
	if err := w.Assign(3, "hub"); !errors.Is(err, ErrGameOver) {
		t.Fatalf("Assign after the end: %v", err)
	}
}
