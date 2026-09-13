package rivals_test

import (
	"math"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
)

// TestDeedsPullTheirWay (#194, this sim's share): the deed to a block of
// yours cuts the rival's push pace on that corner by push_mul and no
// other's (PushPaceOn); the deed to a block of theirs cuts their defence
// of it by the same in the strike's odds (OddsOn) and no other corner's,
// so a strike on a deeded rival corner lands more often on the same
// dice; push_mul is never zero (the file refuses it), so a deed slows
// the rival and never stops it; the table boxed, nothing moves.
func TestDeedsPullTheirWay(t *testing.T) {
	cfg := duel()
	w, s := world(t, cfg, 3)
	tun := cfg.City.Deed
	if tun.PushMul <= 0 || tun.PushMul >= 1 {
		t.Fatalf("push_mul %v: a deed slows the rival and never stops it", tun.PushMul)
	}
	start := w.Corner(cfg.City.Territory.Start)
	docks := w.Corner("docks")
	r := w.Rival()
	r.Arrived, r.Muscle, r.Observed = 1, 6, true
	docks.Owner, docks.Faction, docks.Since = game.OwnerRival, r.Faction(), 1

	pace := s.PushPace(w)
	if got := s.PushPaceOn(w, start); got != pace || s.DeedMul(start) != 1 {
		t.Fatalf("pace on a corner with no deed %v, want %v", got, pace)
	}
	odds := s.Odds(w, r, events.ForcePush)
	if got := s.OddsOn(w, r, docks, events.ForcePush); got != odds {
		t.Fatalf("odds on a rival corner with no deed %v, want %v", got, odds)
	}
	w.Player.CleanCash = 1_000_000
	if err := w.BuyDeed(start.ID, 1000); err != nil {
		t.Fatal(err)
	}
	if got := s.PushPaceOn(w, start); math.Abs(got-pace*tun.PushMul) > 1e-9 {
		t.Fatalf("pace on the deeded corner %v, want %v x %v", got, pace, tun.PushMul)
	}
	if got := s.PushPaceOn(w, docks); got != pace {
		t.Fatalf("pace on the docks moved: %v", got)
	}
	if got := s.OddsOn(w, r, docks, events.ForcePush); got != odds {
		t.Fatalf("the deed on your block moved the odds on theirs: %v, was %v", got, odds)
	}
	// Their block: the defence cut, the odds up, on that corner alone.
	if err := w.BuyDeed(docks.ID, 1000); err != nil {
		t.Fatal(err)
	}
	fc := cfg.Rivals.ForceFor(events.ForcePush)
	attack := s.Strength(w) * fc.Attack
	want := fc.Flip * attack / (attack + s.Defence(w, r)*tun.PushMul)
	if got := s.OddsOn(w, r, docks, events.ForcePush); math.Abs(got-want) > 1e-9 || got <= odds {
		t.Fatalf("odds on the deeded rival corner %v, want %v (was %v)", got, want, odds)
	}
	if got := s.Odds(w, r, events.ForcePush); got != odds {
		t.Fatalf("the faction's odds moved: %v, was %v", got, odds)
	}
	// The strike rolls on it: over the seeds, never fewer corners taken
	// with the deed than without on the same dice (the roll is the
	// same, the line it has to beat lower).
	strikes := func(deed bool) int {
		taken := 0
		for seed := uint64(1); seed <= 40; seed++ {
			w, s := world(t, cfg, seed)
			r := w.Rival()
			r.Arrived, r.Muscle, r.Observed = 1, 8, true
			d := w.Corner("docks")
			d.Owner, d.Faction, d.Since = game.OwnerRival, r.Faction(), 1
			if deed {
				w.Player.CleanCash = 1000
				if err := w.BuyDeed(d.ID, 1000); err != nil {
					t.Fatal(err)
				}
			}
			if err := w.SendEnforcers(d.ID, events.ForcePush); err != nil {
				t.Fatal(err)
			}
			for _, e := range step(w, s) {
				if ev, ok := e.(events.CornerStruck); ok && ev.Taken {
					taken++
				}
			}
		}
		return taken
	}
	bare, deeded := strikes(false), strikes(true)
	if deeded < bare {
		t.Fatalf("strikes on a deeded rival corner landed %d times in 40, bare %d", deeded, bare)
	}
	t.Logf("push strikes landed: %d of 40 on a deeded rival corner, %d bare", deeded, bare)
	// The table boxed: no deed does anything.
	off := *cfg
	off.City.Deed = content.DeedTuning{}
	w2, s2 := world(t, &off, 3)
	w2.Player.CleanCash = 1000
	c := w2.Corner(cfg.City.Territory.Start)
	if err := w2.BuyDeed(c.ID, 1000); err != nil {
		t.Fatal(err)
	}
	if s2.DeedMul(c) != 1 || s2.PushPaceOn(w2, c) != s2.PushPace(w2) {
		t.Fatalf("a deed under the boxed table moved the pace: %v", s2.DeedMul(c))
	}
}
