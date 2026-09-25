package harness

import (
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/sim"
	"github.com/theclifmeister/kingpin/internal/sim/laundering"
)

// The nightly sweep offshore (#478, docs/laundering.md): the player's
// standing order on the account, moved by the laundering sim.

// TestSweepMovesTheCleanOverTheLine (#478): the laundered player with
// three lots of clean in hand and the sweep on from day 1, and a hand
// reservation of half a lot every tenth day. Every night the sweep moves the clean over the line it
// keeps (or over the night's upkeep), never more than the lot less what
// went by hand, so no night files a page; the account gains what moved
// less the fee; and the clean left is never under the line.
func TestSweepMovesTheCleanOverTheLine(t *testing.T) {
	t.Parallel()
	cfg := content.MustLoad()
	ld := laundering.New(cfg)
	lot := cfg.Laundering.Offshore.Lot
	_, sims, err := sim.Default(cfg)
	if err != nil {
		t.Fatal(err)
	}
	w := sim.NewWorld(cfg, 1)
	clock := game.NewClock(nil, sims...)
	play := Laundered(cfg, 40)
	const keep = 20_000
	nights, full, total := 0, 0, 0
	for day := 1; day <= 150 && w.Over == nil; day++ {
		play(w)
		if day == 1 {
			w.Player.CleanCash += 3 * lot // a windfall: the first nights move a full lot each
			if err := w.SetSweep(keep); err != nil {
				t.Fatal(err)
			}
		}
		if day%10 == 0 && w.Player.CleanCash > 0 {
			_ = w.Reserve(min(lot/2, w.Player.CleanCash))
		}
		byHand, offshore := w.Today.Reserved, w.Offshore
		var got *events.Reserved
		for _, e := range clock.EndDay(w) {
			if ev, ok := e.(events.Reserved); ok {
				got = &ev
			}
		}
		if got == nil {
			if byHand > 0 {
				t.Fatalf("day %d: %d reserved by hand and nothing moved", day, byHand)
			}
			continue
		}
		moved := got.Amount + got.Fee
		if got.Swept != moved-byHand {
			t.Fatalf("day %d: swept %d, want the move %d less the hand's %d", day, got.Swept, moved, byHand)
		}
		if got.Swept > lot-byHand {
			t.Fatalf("day %d: swept %d over the lot %d less the hand's %d", day, got.Swept, lot, byHand)
		}
		if got.Lots != 0 || w.Laundering.Structured.Lots != 0 {
			t.Fatalf("day %d: the sweep filed %d lots", day, got.Lots)
		}
		if w.Offshore-offshore != got.Amount || got.Fee != ld.Fee(w, moved) {
			t.Fatalf("day %d: the account gained %d, the event says %d (fee %d)", day, w.Offshore-offshore, got.Amount, got.Fee)
		}
		if got.Swept > 0 {
			nights++
			total += got.Swept
			if w.Player.CleanCash < keep {
				t.Fatalf("day %d: %d clean left under the line %d", day, w.Player.CleanCash, keep)
			}
			if got.Swept == lot-byHand {
				full++
			}
		}
	}
	if nights < 30 || full == 0 {
		t.Fatalf("the sweep moved on %d nights (%d a full lot): it did not run", nights, full)
	}
	t.Logf("swept %d over %d nights (%d a full lot); the account %d on day %d", total, nights, full, w.Offshore, w.Day)
}

// TestNoSweepIsTheOldRun (#478): the sweep is off unless the player
// turns it on, and a sweep turned on and off again before the night
// moves nothing: the run is byte-for-byte the run without it.
func TestNoSweepIsTheOldRun(t *testing.T) {
	t.Parallel()
	cfg := content.MustLoad()
	run := func(toggle bool) []string {
		_, sims, err := sim.Default(cfg)
		if err != nil {
			t.Fatal(err)
		}
		w := sim.NewWorld(cfg, 2)
		clock := game.NewClock(nil, sims...)
		play := Laundered(cfg, 40)
		var out []string
		for day := 1; day <= 120 && w.Over == nil; day++ {
			play(w)
			if toggle {
				_ = w.SetSweep(0)
				_ = w.StopSweep()
			}
			clock.EndDay(w)
			out = append(out, digest(w))
		}
		return out
	}
	plain, toggled := run(false), run(true)
	if len(plain) != len(toggled) {
		t.Fatalf("%d days against %d", len(plain), len(toggled))
	}
	for i := range plain {
		if plain[i] != toggled[i] {
			t.Fatalf("day %d: the run moved with the sweep off (%s, was %s)", i+1, toggled[i], plain[i])
		}
	}
}
