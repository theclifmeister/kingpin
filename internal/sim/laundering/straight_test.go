package laundering_test

import (
	"errors"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/sim/laundering"
)

// TestGoingStraightIsClaimed (#398): the businessman's streak no longer
// ends the run. The night it reaches legit_days the sim says so once
// (StraightOpened) and going straight is open; it stays open while the
// streak holds, a night that fails closes it (StraightLapsed) and
// zeroes the streak; GoStraight ends the run a businessman only while
// it is open.
func TestGoingStraightIsClaimed(t *testing.T) {
	cfg := content.MustLoad()
	s := laundering.New(cfg)
	days := cfg.Laundering.Businessman.LegitDays
	w := world(1_000_000)
	for _, fc := range cfg.Laundering.Fronts[:3] {
		w.Fronts = append(w.Fronts, game.Front{ID: fc.ID, Name: fc.Name, Cost: fc.Cost, Bought: 1, Level: fc.MaxLevel})
	}
	w.Player.CleanCash = 1_000_000
	home := w.Home()
	home.Goodwill, home.Pressure = 100, 0
	if err := s.GoStraight(w); !errors.Is(err, game.ErrNotStraight) {
		t.Fatalf("on day 0: %v", err)
	}
	opened := 0
	for night := 1; night <= days+5; night++ {
		w.Player.CleanCash = 1_000_000 // the upkeep never shuts a front
		k := kinds(step(w, s))
		opened += k["StraightOpened"]
		if w.Over != nil {
			t.Fatalf("night %d: the streak ended the run %s", night, w.Over.Cause)
		}
		if got := s.CanGoStraight(w); got != (night >= days) {
			t.Fatalf("night %d: open %v with %d legit days of %d", night, got, w.LegitDays, days)
		}
	}
	if opened != 1 {
		t.Fatalf("StraightOpened %d times, want once", opened)
	}
	home.Goodwill, home.Pressure = 0, 100 // the city turns
	if k := kinds(step(w, s)); k["StraightLapsed"] != 1 || w.LegitDays != 0 || s.CanGoStraight(w) {
		t.Fatalf("a failed night: %v, %d legit days, open %v", k, w.LegitDays, s.CanGoStraight(w))
	}
	if k := kinds(step(w, s)); k["StraightLapsed"] != 0 {
		t.Fatal("lapsed twice")
	}
	if err := s.GoStraight(w); !errors.Is(err, game.ErrNotStraight) || w.Over != nil {
		t.Fatalf("closed: %v, over %+v", err, w.Over)
	}
	w.LegitDays = days
	if err := s.GoStraight(w); err != nil || w.Over == nil || w.Over.Cause != content.CauseBusinessman || w.Over.Day != w.Day {
		t.Fatalf("open: %v, over %+v", err, w.Over)
	}
	if err := s.GoStraight(w); !errors.Is(err, game.ErrGameOver) {
		t.Fatalf("twice: %v", err)
	}
}
