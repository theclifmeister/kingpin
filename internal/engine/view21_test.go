package engine_test

import (
	"slices"
	"testing"

	"github.com/theclifmeister/kingpin/internal/engine"
	"github.com/theclifmeister/kingpin/internal/game"
)

// TestViewCarriesTheCrewsOddsAndEnds (#576): view 21 carries what
// Street Edition's crew tab reads: a member's kin, on the payroll and in
// the pool, the day the pool last turned over, your carry and what each
// city holds (World.Capacity).
func TestViewCarriesTheCrewsOddsAndEnds(t *testing.T) {
	t.Parallel()
	s, w := freshSession(t)
	w.Crew.Members = append(w.Crew.Members, game.CrewMember{ID: 901, Name: "Ana", Role: game.RoleRunner, Units: 10, Kin: []int{902}})
	w.Crew.Candidates = append(w.Crew.Candidates, game.CrewMember{ID: 902, Name: "Bo", Role: game.RoleRunner, Kin: []int{901}})
	w.Crew.PoolDay = 3
	v := s.View()
	if v.You.CarryLimit != w.Player.CarryLimit || v.You.PoolDay != 3 {
		t.Errorf("carry %d, pool day %d", v.You.CarryLimit, v.You.PoolDay)
	}
	for _, cid := range w.CityOrder {
		if v.You.Capacity[cid] != w.Capacity(cid) {
			t.Errorf("%s holds %d, want %d", cid, v.You.Capacity[cid], w.Capacity(cid))
		}
	}
	if here := w.Player.Location; v.You.Capacity[here] < v.You.CarryLimit+10 {
		t.Errorf("where you stand: %d, under your carry and Ana's", v.You.Capacity[here])
	}
	find := func(ms []engine.MemberView, id int) []int {
		for _, m := range ms {
			if m.ID == id {
				return m.Kin
			}
		}
		t.Fatalf("no member %d", id)
		return nil
	}
	if got := find(v.Crew, 901); !slices.Equal(got, []int{902}) {
		t.Errorf("Ana's kin %v", got)
	}
	if got := find(v.Pool, 902); !slices.Equal(got, []int{901}) {
		t.Errorf("Bo's kin %v", got)
	}
	// The view holds no slice of the world's.
	find(v.Crew, 901)[0] = 0
	if w.Crew.Members[len(w.Crew.Members)-1].Kin[0] != 902 {
		t.Error("the view shares Ana's kin with the world")
	}
}
