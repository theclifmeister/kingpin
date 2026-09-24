package laundering_test

import (
	"strings"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/gametest"
	"github.com/theclifmeister/kingpin/internal/sim/laundering"
)

// The trophies (#392): clean cash only, refused under the line, short
// or owned; bought, it counts in net worth at cost, is reported the
// morning after, and the task force's TrophySeized takes it off the
// books to the lost record, after which it is for sale again.
func TestTrophiesAreCleanMoney(t *testing.T) {
	cfg := content.MustLoad()
	s := laundering.New(cfg)
	o := s.TrophyOffers()[0]
	w := world(0)
	w.Player.CleanCash = o.Cost
	if _, err := s.BuyTrophy(w, o.ID); err == nil || !strings.Contains(err.Error(), "held") {
		t.Fatalf("under the line: %v", err)
	}
	w.Stats.PeakClean = o.UnlockCash
	w.Player.CleanCash = o.Cost - 1
	w.Player.DirtyCash = o.Cost * 2 // under the rot line
	if _, err := s.BuyTrophy(w, o.ID); err == nil || !strings.Contains(err.Error(), "clean") {
		t.Fatalf("short of clean with dirty to spare: %v", err)
	}
	if _, err := s.BuyTrophy(w, "nothing"); err != game.ErrNoTrophy {
		t.Fatalf("an unknown trophy: %v", err)
	}
	w.Player.CleanCash = o.Cost
	before := w.NetWorth()
	tr, err := s.BuyTrophy(w, o.ID)
	if err != nil || w.Player.CleanCash != 0 || w.NetWorth() != before || w.Stats.TrophyCash != o.Cost {
		t.Fatalf("bought %+v %v: clean %d, net worth %d (was %d)", tr, err, w.Player.CleanCash, w.NetWorth(), before)
	}
	if _, err := s.BuyTrophy(w, o.ID); err != game.ErrTrophyOwned {
		t.Fatalf("twice: %v", err)
	}
	if k := kinds(step(w, s)); k["TrophyBought"] != 1 {
		t.Fatalf("the morning after: %v", k)
	}
	gametest.StepUnseeded(w, s, events.TrophySeized{Day: w.Day + 1, Trophy: o.ID, Name: o.Name, Cost: o.Cost})
	if len(w.Trophies) != 0 || len(w.TrophiesLost) != 1 || w.TrophiesLost[0].Why != "seized" || w.NetWorth() != before-o.Cost {
		t.Fatalf("after the seizure: owned %v lost %v net worth %d", w.Trophies, w.TrophiesLost, w.NetWorth())
	}
	w.Player.CleanCash = o.Cost
	if _, err := s.BuyTrophy(w, o.ID); err != nil {
		t.Fatalf("for sale again after the seizure: %v", err)
	}
}

// The rot (#392): nothing at or under the line; over it, rot of what
// is over, off the dirty pile after the wash, reported and counted.
func TestDirtyPileRots(t *testing.T) {
	cfg := content.MustLoad()
	s := laundering.New(cfg)
	line := cfg.Laundering.Laundering.RotLine
	if line <= 0 || s.Rot(line) != 0 || s.Rot(line-1) != 0 {
		t.Fatalf("rot at the line: %d", s.Rot(line))
	}
	w := world(line)
	if k := kinds(step(w, s)); k["CashRotted"] != 0 || w.Stats.Rotted != 0 || w.Player.DirtyCash != line {
		t.Fatalf("a pile at the line rotted: %v %d", k, w.Stats.Rotted)
	}
	over := 100_000_000
	w = world(line + over)
	want := int(float64(over) * cfg.Laundering.Laundering.Rot)
	if got := s.Rot(line + over); got != want {
		t.Fatalf("rot %d, want %d", got, want)
	}
	if k := kinds(step(w, s)); k["CashRotted"] != 1 || w.Stats.Rotted != want || w.Player.DirtyCash != line+over-want {
		t.Fatalf("rotted %v %d, pile %d", k, w.Stats.Rotted, w.Player.DirtyCash)
	}
}
