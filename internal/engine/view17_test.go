package engine_test

import (
	"testing"
)

// TestViewCarriesTheLabsNumbers (#557): view 17's `you.quality` is the
// quality of each lot held, absent where nothing is, and moves with a
// cut; `you.room` is each city's free stash, World.Free.
func TestViewCarriesTheLabsNumbers(t *testing.T) {
	t.Parallel()
	s, w := freshSession(t)
	home, id := w.CityOrder[0], w.Products[0]

	v := s.View()
	for _, cid := range w.CityOrder {
		if got, ok := v.You.Room[cid]; !ok || got != w.Free(cid) {
			t.Errorf("room in %s = %d (%v), want %d", cid, got, ok, w.Free(cid))
		}
		for pid := range v.You.Quality[cid] {
			if w.Stock(cid, pid) <= 0 {
				t.Errorf("quality of an empty lot: %s %s", cid, pid)
			}
		}
	}

	w.Player.DirtyCash += 100_000
	w.AddStock(home, id, 20, 80)
	w.SetQuality(home, id, 80)
	v = s.View()
	if got := v.You.Quality[home][id]; got != 80 {
		t.Fatalf("quality after stocking at 80: %v", got)
	}
	if v.You.Room[home] != w.Free(home) {
		t.Fatalf("room %d, want %d", v.You.Room[home], w.Free(home))
	}
	most := s.Rules().Market.CutMax(id)
	if most <= 0 {
		t.Skipf("%s cannot be cut", id)
	}
	rec, err := s.Cut(home, id, most)
	if err != nil {
		t.Fatal(err)
	}
	v = s.View()
	if got := v.You.Quality[home][id]; got != rec.To || got >= 80 {
		t.Fatalf("quality after the cut %v, the cut says %v", got, rec.To)
	}
	if v.You.Room[home] != w.Free(home) {
		t.Fatalf("room after the cut %d, want %d", v.You.Room[home], w.Free(home))
	}
}
