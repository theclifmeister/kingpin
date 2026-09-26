package crew_test

import (
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/gametest"
	"github.com/theclifmeister/kingpin/internal/sim/crew"
)

// acted is the night's LieutenantActed, or fails.
func acted(t *testing.T, evs []events.Event) events.LieutenantActed {
	t.Helper()
	for _, e := range evs {
		if ev, ok := e.(events.LieutenantActed); ok {
			return ev
		}
	}
	t.Fatal("no LieutenantActed")
	return events.LieutenantActed{}
}

// A lieutenant does not sell what the owner earmarked for elsewhere
// (#497: Flaco sold the route's designer in Bayport at $927 when it was
// meant for Eastside at $3,200): a route running out of their city on
// its dial keeps back its shortfall at the far end, a contract you took
// there keeps back what it still owes, and the report says so. Off its
// dial the route earmarks nothing, and the whole stash is theirs.
func TestLieutenantKeepsWhatIsEarmarked(t *testing.T) {
	cfg := content.MustLoad()
	boxed := *cfg
	boxed.Crew.Life = content.LifeTuning{}
	boxed.Routes.Routes = []content.RouteConfig{{ID: "coast", Name: "Coast Road", Mode: "car", From: "test", To: "far", Days: 1, Capacity: 500}}
	w := game.NewWorld(7, []game.StartingCity{
		{ID: "test", Name: "Testville", Products: []game.StartingProduct{gametest.Weed}},
		{ID: "far", Name: "Farside", Products: []game.StartingProduct{gametest.Weed}},
	}, 100_000, 1000)
	s := crew.New(&boxed)
	s.Seed(w, game.RNGFor(7, 0))
	w.Crew.Members = []game.CrewMember{{ID: 902, Name: "Flaco", Role: "lieutenant", Personality: "violent", Skill: 50, Loyalty: 95, Greed: 5, Nerve: 90, Wage: 50}}
	w.Crew.NextID = 902
	if err := w.Assign(902, "test"); err != nil {
		t.Fatal(err)
	}
	weed := gametest.Weed.ID
	w.SetStock("test", weed, 100)
	w.SetStock("far", weed, 10)
	if err := w.SetRouteTarget("coast", weed, 70); err != nil {
		t.Fatal(err)
	}
	order := func() int {
		o, ok := w.DelegatedOrder("test", weed)
		if !ok {
			return 0
		}
		return o.Qty
	}

	// The dial off: nothing earmarked, the stash is theirs to sell.
	if ev := acted(t, step(w, s)); order() != 100 || len(ev.Held) != 0 {
		t.Fatalf("the route off: order %d, held %+v, want 100 and none", order(), ev.Held)
	}

	// On: the far end's 60 short stays back.
	if err := w.SetRoute("coast", events.RouteNormal); err != nil {
		t.Fatal(err)
	}
	ev := acted(t, step(w, s))
	if order() != 40 || len(ev.Held) != 1 || ev.Held[0] != (events.Held{Product: weed, Units: 60, For: "Coast Road"}) {
		t.Fatalf("the route on: order %d, held %+v, want 40 and 60 for Coast Road", order(), ev.Held)
	}

	// A contract here owes 25 more: kept back beside the route's.
	w.Contracts = append(w.Contracts, game.Contract{ID: 1, Name: "Marco", City: "test", Product: weed, Units: 30, Delivered: 5, Status: game.ContractAccepted, Due: w.Day + 5})
	ev = acted(t, step(w, s))
	if order() != 15 || len(ev.Held) != 1 || ev.Held[0].Units != 85 {
		t.Fatalf("the route and a contract: order %d, held %+v, want 15 and 85", order(), ev.Held)
	}

	// Everything earmarked: no order at all, and all 100 said.
	if err := w.SetRouteTarget("coast", weed, 500); err != nil {
		t.Fatal(err)
	}
	ev = acted(t, step(w, s))
	if order() != 0 || len(ev.Held) != 1 || ev.Held[0].Units != 100 {
		t.Fatalf("all of it earmarked: order %d, held %+v, want none and 100", order(), ev.Held)
	}
}

// Taking a city with every corner held keeps the posted crew where they
// are (#497: assigning Wally to a fully held Eastside pulled all seven
// runners off their corners): the robbed_off rule counts the stick-ups
// since the lieutenant took the city, never the ones before. A corner
// robbed twice on their watch still loses its crew.
func TestLieutenantKeepsThePostedCrew(t *testing.T) {
	cfg := content.MustLoad()
	w, s := factionWorld(t, cfg)
	off := cfg.Crew.Lieutenant.RobbedOff
	w.Crew.Members = []game.CrewMember{
		{ID: 901, Name: "Vee", Role: "runner", Skill: 50, Loyalty: 95, Greed: 5, Nerve: 90, Units: 100, Wage: 50},
		{ID: 902, Name: "Wally", Role: "lieutenant", Personality: "steady", Skill: 50, Loyalty: 95, Greed: 5, Nerve: 90, Wage: 50},
	}
	w.Crew.NextID = 902
	if err := w.Post("oldmill", 901); err != nil {
		t.Fatal(err)
	}
	w.Corner("oldmill").Robbed = off // stick-ups counted before the lieutenant came
	w.Day = 3
	if err := w.Assign(902, "test"); err != nil {
		t.Fatal(err)
	}
	ev := acted(t, step(w, s))
	if c := w.Corner("oldmill"); !c.Held() || c.Runner != 901 || len(ev.Dropped) != 0 {
		t.Fatalf("the night Wally took the city: %+v, dropped %v; want Vee still on it", *c, ev.Dropped)
	}

	// Robbed robbed_off times since they came: the old rule.
	c := w.Corner("oldmill")
	c.Robbed += off
	step(w, s)
	if c := w.Corner("oldmill"); c.Runner == 901 {
		t.Fatalf("a corner robbed %d times on Wally's watch kept its crew: %+v", off, *c)
	}
}

// A lieutenant who puts idle crew to work says whom (#497: Flaco moved
// Eastside's idle runners and enforcers to Bayport's Wharf unasked):
// the night's event names each one and the corner.
func TestLieutenantSaysWhomTheyTook(t *testing.T) {
	cfg := content.MustLoad()
	w, s := factionWorld(t, cfg)
	w.Crew.Members = []game.CrewMember{
		{ID: 901, Name: "Gato", Role: "runner", Skill: 50, Loyalty: 95, Greed: 5, Nerve: 90, Units: 100, Wage: 50},
		{ID: 902, Name: "Flaco", Role: "lieutenant", Personality: "violent", Skill: 50, Loyalty: 95, Greed: 5, Nerve: 90, Wage: 50},
		{ID: 903, Name: "Nelly", Role: "enforcer", Skill: 50, Loyalty: 95, Greed: 5, Nerve: 90, Wage: 50},
	}
	w.Crew.NextID = 903
	if err := w.Assign(902, "test"); err != nil {
		t.Fatal(err)
	}
	ev := acted(t, step(w, s))
	want := []events.Took{{Name: "Gato", Role: "runner", Corner: "Old Mill"}, {Name: "Nelly", Role: "enforcer", Corner: "Home"}}
	if len(ev.Took) != 2 || ev.Took[0] != want[0] || ev.Took[1] != want[1] {
		t.Fatalf("took %+v, want %+v", ev.Took, want)
	}
	if ev := acted(t, step(w, s)); len(ev.Took) != 0 {
		t.Fatalf("nobody idle, and they took %+v", ev.Took)
	}
}
