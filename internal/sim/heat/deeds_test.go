package heat_test

import (
	"math"
	"strings"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/sim/heat"
)

// TestDeedsPullTheirWay (#194, this sim's share): a house on a block
// whose deed is yours weighs raid_mul of its block's heat in the raid's
// roll and a house on any other block what it did; and the morning
// after the DA seized a deed (w.Law.Forfeited, the law's night read as
// Backfired is) the file grows by forfeit_evidence pages where you are,
// once, whatever was sold, and never on the day itself.
func TestDeedsPullTheirWay(t *testing.T) {
	cfg := content.MustLoad()
	s := heat.New(cfg)
	tun := cfg.City.Deed
	w := world(t, cfg)
	home := w.Home().ID
	w.Player.DirtyCash = 100_000
	for _, id := range []string{"a", "b"} {
		if _, err := w.BuyHouse(game.HouseOffer{ID: id, Name: "H" + id, City: home, Corner: w.Home().Corners[0].ID, Capacity: 100, Price: 1}); err != nil {
			t.Fatal(err)
		}
	}
	w.House("b").Corner = w.Home().Corners[1].ID
	a, b := w.House("a"), w.House("b")
	wa, wb := s.RaidWeight(w, a), s.RaidWeight(w, b)
	if wa != w.Home().Corners[0].Heat || wb != w.Home().Corners[1].Heat {
		t.Fatalf("weights before the deed %v %v, want the blocks' heat", wa, wb)
	}
	w.Player.CleanCash = 1000
	if err := w.BuyDeed(w.Home().Corners[0].ID, 1000); err != nil {
		t.Fatal(err)
	}
	if got := s.RaidWeight(w, a); math.Abs(got-wa*tun.RaidMul) > 1e-9 {
		t.Fatalf("the house on the deeded block weighs %v, want %v x %v", got, wa, tun.RaidMul)
	}
	if got := s.RaidWeight(w, b); got != wb {
		t.Fatalf("the house on the other block moved: %v, was %v", got, wb)
	}
	off := *cfg
	off.City.Deed = content.DeedTuning{}
	if got := heat.New(&off).RaidWeight(w, a); got != wa {
		t.Fatalf("under the boxed table the deed weighs: %v, was %v", got, wa)
	}

	// The forfeiture's pages, the morning after.
	w = world(t, cfg)
	w.Law.Forfeited = w.Day + 1
	step(w, s)
	if w.Heat.Evidence != 0 {
		t.Fatalf("the morning of the forfeiture itself: evidence %d", w.Heat.Evidence)
	}
	tk := step(w, s)
	if w.Heat.Evidence != tun.ForfeitEvidence || w.Heat.EvidenceDay != w.Day {
		t.Fatalf("the morning after: evidence %d, want %d", w.Heat.Evidence, tun.ForfeitEvidence)
	}
	found := false
	for _, e := range tk.Events() {
		if ev, ok := e.(events.HeatChanged); ok && ev.City == w.Player.Location {
			for _, r := range ev.Reasons {
				found = found || strings.Contains(r, "forfeiture")
			}
		}
	}
	if !found {
		t.Fatal("the reasons do not name the forfeiture")
	}
	step(w, s)
	if w.Heat.Evidence != tun.ForfeitEvidence {
		t.Fatalf("a forfeiture filed twice: %d", w.Heat.Evidence)
	}
	if s.ForfeitEvidence() != tun.ForfeitEvidence {
		t.Fatalf("ForfeitEvidence %d, the file says %d", s.ForfeitEvidence(), tun.ForfeitEvidence)
	}
}
