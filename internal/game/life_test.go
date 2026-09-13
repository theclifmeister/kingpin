package game

import (
	"testing"

	"github.com/theclifmeister/kingpin/internal/events"
)

// Crew life on the world (#46): bail is clean cash only and once, a
// driver rides one route and comes off it when fired, and a member in
// a cell or laid up works nothing.
func TestBailAndTheDriver(t *testing.T) {
	w := twoCityWorld()
	w.Crew.NextID = 2
	w.Crew.Members = []CrewMember{
		{ID: 1, Name: "R", Role: "runner", Skill: 50, Loyalty: 50, Units: 100, Wage: 50, JailedUntil: w.Day + 5},
		{ID: 2, Name: "D", Role: RoleDriver, Skill: 60, Loyalty: 50, Wage: 60},
	}
	w.Player.DirtyCash, w.Player.CleanCash = 100_000, 1_000
	if _, err := w.Bail(1, 1_500); err == nil {
		t.Fatal("bail came out of dirty cash")
	}
	if _, err := w.Bail(2, 100); err != ErrNotJailed {
		t.Fatalf("bail for a free member: %v", err)
	}
	w.Player.CleanCash = 1_500
	m, err := w.Bail(1, 1_500)
	if err != nil {
		t.Fatal(err)
	}
	if w.Player.CleanCash != 0 || w.Player.DirtyCash != 100_000 || !m.Bailed || m.JailedUntil != w.Day+1 || w.Stats.Bails != 1 || w.Stats.BailCash != 1_500 || len(w.Crew.BailedToday) != 1 {
		t.Fatalf("after the bail: clean %d dirty %d %+v stats %+v scratch %+v", w.Player.CleanCash, w.Player.DirtyCash, m, w.Stats, w.Crew.BailedToday)
	}
	if _, err := w.Bail(1, 0); err != ErrBailed {
		t.Fatalf("bail twice: %v", err)
	}
	if !m.Jailed(w.Day) || m.Fit(w.Day) || m.Working() {
		t.Fatalf("bailed today, still in the cell: %+v", m)
	}
	c := w.Home().Corners[0].ID
	if err := w.Post(c, 1); err != ErrJailed {
		t.Fatalf("posting from the cell: %v", err)
	}
	w.Crew.Members[1].WoundedUntil = w.Day + 2
	if err := w.Post(c, 2); err != ErrWounded {
		t.Fatalf("posting the wounded: %v", err)
	}
	w.Crew.Members[1].WoundedUntil = 0

	// The driver: one route at a time, only a driver, off it when fired.
	routes := []string{"r1", "r2"}
	if err := w.SetRouteDriver(routes[0], 1); err != ErrNotDriver {
		t.Fatalf("a runner on a route: %v", err)
	}
	if err := w.SetRouteDriver(routes[0], 2); err != nil {
		t.Fatal(err)
	}
	if w.DrivenRoute(2) != routes[0] || w.RouteDriver(routes[0], w.Day) == nil || w.RouteDriver(routes[0], w.Day).ID != 2 {
		t.Fatalf("driver on %s: %+v", routes[0], w.Routes)
	}
	if err := w.SetRouteDriver(routes[1], 2); err != nil {
		t.Fatal(err)
	}
	if w.Route(routes[0]).Driver != 0 || w.Route(routes[1]).Driver != 2 {
		t.Fatalf("a driver on two routes: %+v", w.Routes)
	}
	w.Crew.Members[1].JailedUntil = w.Day + 3
	if w.RouteDriver(routes[1], w.Day) != nil {
		t.Fatal("a jailed driver rides")
	}
	w.Crew.Members[1].JailedUntil = 0
	if err := w.SetRouteDriver(routes[1], 0); err != nil || w.Route(routes[1]).Driver != 0 {
		t.Fatalf("taking the driver off: %v %+v", err, w.Routes)
	}
	_ = w.SetRouteDriver(routes[1], 2)
	if _, err := w.Fire(2); err != nil {
		t.Fatal(err)
	}
	if w.Route(routes[1]).Driver != 0 || w.RouteDriver(routes[1], w.Day) != nil {
		t.Fatalf("the fired driver still rides: %+v", w.Routes)
	}
	// The clock clears the bail scratch with the rest.
	NewClock(nil).EndDay(w)
	if w.Crew.BailedToday != nil {
		t.Fatalf("bail scratch survived the day: %+v", w.Crew.BailedToday)
	}
	_ = events.PayFair
}

// The role count is the members at work: a jailed or wounded one is on
// the payroll and counted by OnPayroll, not by Role.
func TestRoleCountsTheMembersAtWork(t *testing.T) {
	c := CrewState{Members: []CrewMember{
		{ID: 1, Role: "enforcer"},
		{ID: 2, Role: "enforcer", JailedUntil: 9},
		{ID: 3, Role: "enforcer", WoundedUntil: 9},
		{ID: 4, Role: RoleChemist, Skill: 90, JailedUntil: 9},
		{ID: 5, Role: RoleChemist, Skill: 40},
	}}
	if c.Role("enforcer") != 1 || c.OnPayroll("enforcer") != 3 {
		t.Fatalf("enforcers at work %d, on the payroll %d", c.Role("enforcer"), c.OnPayroll("enforcer"))
	}
	if ch := c.Chemist(); ch == nil || ch.ID != 5 {
		t.Fatalf("the best chemist at work: %+v", ch)
	}
	if !c.Members[1].IsKin(3) == false || c.Members[0].IsKin(2) {
		t.Fatal("kin out of nothing")
	}
}
