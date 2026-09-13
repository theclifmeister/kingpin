package game

import (
	"bytes"
	"encoding/gob"
	"os"
	"testing"
)

// v14World is what a pre-15 save carried for its one rival (#43): the
// faction on World itself.
type v14World struct {
	SchemaVersion int
	Seed          uint64
	Day           int
	Cities        map[string]*City
	CityOrder     []string
	Player        Player
	Upgrades      map[string]bool
	Rival         RivalState
}

// A save from before the table (#43) seats its one rival as the first
// faction on load (SeatRival, the 14 -> 15 step's first half): its
// leader, its corners, its deals and its books as they were, and its
// corners still name it.
func TestSaveSeatsTheOneRival(t *testing.T) {
	t.Setenv("KINGPIN_HOME", t.TempDir())
	fresh := testWorld()
	rival := RivalState{ID: FactionRival, Leader: "Big Sal", Personality: "expansionist", Cash: 4200, Muscle: 5, Arrived: 3, Trust: 35, Deals: []Deal{{Kind: DealTruce, Terms: Terms{Days: 20}, Since: 5, Until: 25}}, Heat: 12}
	fresh.Home().Corners[1].Owner = OwnerRival
	old := v14World{SchemaVersion: 14, Seed: fresh.Seed, Day: 6, Cities: fresh.Cities, CityOrder: fresh.CityOrder, Player: fresh.Player, Upgrades: map[string]bool{}, Rival: rival}
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(old); err != nil {
		t.Fatal(err)
	}
	p, _ := SavePath(1)
	if err := os.WriteFile(p, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(1); err == nil {
		t.Fatal("a schema-14 save loaded without a migration")
	}
	got, err := Load(1, Migration{From: 14, Apply: (*World).SeatRival})
	if err != nil {
		t.Fatal(err)
	}
	if got.SchemaVersion != SchemaVersion || len(got.Rivals) != 1 {
		t.Fatalf("schema %d, %d factions", got.SchemaVersion, len(got.Rivals))
	}
	r := got.Rival()
	if r.Leader != "Big Sal" || r.Cash != 4200 || r.Muscle != 5 || r.Arrived != 3 || r.Trust != 35 || r.Heat != 12 || len(r.Deals) != 1 || r.Faction() != FactionRival {
		t.Fatalf("the seated rival: %+v", *r)
	}
	if c := got.Home().Corners[1]; c.Owner != OwnerRival || c.FactionID() != FactionRival || got.RivalHeldBy(FactionRival) != 1 {
		t.Fatalf("its corner: %+v", c)
	}
	if got.Faction("") != r || got.Faction(FactionRival) != r || got.Faction("f2") != nil || got.FactionIndex(FactionRival) != 0 {
		t.Fatal("Faction does not resolve the seated rival")
	}
}

// The table's helpers on the world (#43): who holds what, who borders
// whom, who is strongest where, and Dominant's rule.
func TestFactionHelpers(t *testing.T) {
	w := testWorld()
	w.Rivals = []*RivalState{{ID: FactionRival, Leader: "Sal", Arrived: 1}, {ID: "f2", Leader: "Ray", Arrived: 1}, {ID: "f3", Leader: "Vic"}}
	w.Home().Corners = append(w.Home().Corners, Corner{ID: "far", City: "test", Name: "Far", Demand: 1, Heat: 1, Risk: 1, Owner: OwnerNone})
	cs := w.Home().Corners
	// home (0,0) corner (1,0) far (2,0)? testWorld lays them out; use
	// the ids it gives and set the borders by hand.
	for i := range cs {
		cs[i].X, cs[i].Y = i, 0
	}
	cs[0].Owner = OwnerPlayer
	cs[1].Owner, cs[1].Faction = OwnerRival, FactionRival
	cs[2].Owner, cs[2].Faction = OwnerRival, "f2"
	if w.RivalHeld() != 2 || w.RivalHeldBy(FactionRival) != 1 || w.RivalHeldBy("f2") != 1 || w.HeldByIn("f2", "test") != 1 {
		t.Fatalf("counts %d %d %d", w.RivalHeld(), w.RivalHeldBy(FactionRival), w.RivalHeldBy("f2"))
	}
	if !w.Contested(cs[0]) || !w.Contested(cs[1]) || !w.Contested(cs[2]) {
		t.Fatal("every corner borders another side")
	}
	if !w.ContestedBy(cs[0], FactionRival) || w.ContestedBy(cs[0], "f2") || !w.ContestedBy(cs[2], FactionRival) || !w.ContestedBy(cs[1], "f2") || w.ContestedBy(cs[1], FactionRival) {
		t.Fatal("ContestedBy is wrong about who borders whom")
	}
	if s := w.StrongestFaction("test"); s == nil || s.ID != FactionRival {
		t.Fatalf("strongest %+v", s)
	}
	cs[0].Owner, cs[0].Faction = OwnerRival, "f2"
	if s := w.StrongestFaction("test"); s.ID != "f2" {
		t.Fatalf("strongest %+v", s)
	}
	if w.FactionName("f2") != "Ray's crew" || w.FactionName("nobody") != "the rival" || w.Stance(w.Rivals[2], 40) != "not yet" {
		t.Fatal("names and stances")
	}
	// Dominant: not with a faction on the ground and no deal; not with
	// none arrived; yes with every arrived one gone or paying.
	if w.Dominant() {
		t.Fatal("dominant with two on the ground")
	}
	w.Rivals[0].Absorbed = 5
	w.Rivals[1].Deals = []Deal{{Kind: DealHomage, Terms: Terms{PerDay: 100}, Since: 1}}
	if w.Dominant() {
		t.Fatal("dominant with one still in the wings")
	}
	w.Rivals[2].Arrived, w.Rivals[2].Fragmented = 4, 9
	if !w.Dominant() {
		t.Fatal("not dominant with two gone and one paying")
	}
	w.Rivals[2].Fragmented = 0
	w.Rivals[2].Arrived = 0
	w.Rivals[1].Deals = nil
	if w.Dominant() {
		t.Fatal("dominant with one on the ground paying nothing")
	}
	w.Rivals[1].Arrived = 0
	w.Rivals[0].Absorbed = 0
	w.Rivals[0].Arrived = 0
	if w.Dominant() {
		t.Fatal("dominant with nobody arrived")
	}
	// A world built by hand with no factions gets the rival at home on
	// demand, and a deal with nobody is no deal.
	empty := testWorld()
	if empty.Rival() == nil || len(empty.Rivals) != 1 || empty.Rival().Faction() != FactionRival || empty.DealWith("f9", DealTruce) != nil || empty.Dominant() {
		t.Fatal("the empty world")
	}
}
