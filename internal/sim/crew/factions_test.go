package crew_test

import (
	"reflect"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
)

// The crew sim's one block for the table (#43): a CrewPoached that
// landed drops the member and queues the lead for the faction named,
// a corner they stood on in the faction's city with it; one that did
// not dips their loyalty; and a RivalLeaderArrested puts the faction's
// muscle in the pool as enforcers at the discount, extra faces the
// pool's count leaves out. Nothing is written into a faction.
func TestFactionsBlock(t *testing.T) {
	cfg := content.MustLoad()
	w, s := world(t, cfg, 50_000)
	w.Rivals = []*game.RivalState{{ID: game.FactionRival, Leader: "Sal", Arrived: 1}, {ID: "f2", Leader: "Ray", Arrived: 1}}
	w.Home().Corners = []game.Corner{{ID: "a", City: "test", Name: "A", Demand: 1, Owner: game.OwnerPlayer}, {ID: "b", City: "test", Name: "B", Demand: 1, X: 1, Owner: game.OwnerPlayer}}
	w.Crew.Members = []game.CrewMember{
		{ID: 1, Name: "Dre", Role: "runner", Skill: 60, Units: 100, Loyalty: 80, Wage: 50},
		{ID: 2, Name: "Vee", Role: "runner", Skill: 60, Units: 100, Loyalty: 30, Wage: 50},
	}
	w.Crew.NextID = 2
	if err := w.Post("b", 2); err != nil {
		t.Fatal(err)
	}
	before := *w.Rivals[1]
	faces := len(w.Crew.Candidates)
	evs := step(w, s,
		events.CrewPoached{Day: w.Day + 1, ID: 2, Name: "Vee", Role: "runner", Rival: "Ray", Faction: "f2", Wages: 75},
		events.CrewPoached{Day: w.Day + 1, ID: 1, Name: "Dre", Role: "runner", Rival: "Ray", Faction: "f2", Wages: 75, Stayed: true, Dip: 4},
		events.RivalLeaderArrested{Day: w.Day + 1, Rival: "Sal", Faction: game.FactionRival, City: "test", Corners: 3, Muscle: 3},
	)
	if len(w.Crew.Members) != 1 || w.Crew.Members[0].ID != 1 {
		t.Fatalf("roster after the poach: %+v", w.Crew.Members)
	}
	if l := w.Crew.Members[0].Loyalty; l > 76.5 || l < 74 {
		t.Fatalf("the one who stayed has loyalty %.1f, want 80 less the dip of 4 and a night's drift", l)
	}
	if len(w.Crew.Leads) != 1 || w.Crew.Leads[0] != (game.Lead{Name: "Vee", Corner: "b", Faction: "f2"}) {
		t.Fatalf("leads %+v", w.Crew.Leads)
	}
	if c := w.Corner("b"); c.Runner != 0 {
		t.Fatalf("the poached runner still stands on b: %+v", *c)
	}
	if !reflect.DeepEqual(*w.Rivals[1], before) {
		t.Fatalf("the crew sim wrote into the faction: %+v", *w.Rivals[1])
	}
	former, plain := 0, 0
	for _, c := range w.Crew.Candidates {
		if c.Former == game.FactionRival {
			former++
			if c.Role != "enforcer" || c.Fee <= 0 || c.Units != 0 {
				t.Fatalf("a fragmented faction's head in the pool: %+v", c)
			}
			continue
		}
		plain++
	}
	if former != 3 || plain < faces {
		t.Fatalf("pool: %d former heads, %d faces (had %d); %v", former, plain, faces, kinds(evs))
	}
}
