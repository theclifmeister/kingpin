package game

import (
	"errors"
	"testing"
)

// TestCashOut (#395): clean cash drawn back into the dirty pile at
// once, less the fee the caller works out; refused with no clean cash,
// past it, at nothing, and on a run that is over; what it drew is the
// day's scratch and the run's stats.
func TestCashOut(t *testing.T) {
	w := testWorld()
	w.Player.DirtyCash, w.Player.CleanCash = 1_000, 0
	if err := w.CashOut(100, 10); !errors.Is(err, ErrNoCleanCash) {
		t.Fatalf("no clean cash: %v", err)
	}
	w.Player.CleanCash = 50_000
	if err := w.CashOut(0, 0); !errors.Is(err, ErrBadQuantity) {
		t.Fatalf("nothing: %v", err)
	}
	if err := w.CashOut(100, 200); !errors.Is(err, ErrBadQuantity) {
		t.Fatalf("a fee over the amount: %v", err)
	}
	var short *ShortError
	if err := w.CashOut(50_001, 5_000); !errors.As(err, &short) || w.Player.CleanCash != 50_000 || w.Player.DirtyCash != 1_000 {
		t.Fatalf("past the clean cash: %v, clean %d dirty %d", err, w.Player.CleanCash, w.Player.DirtyCash)
	}
	if err := w.CashOut(20_000, 2_000); err != nil {
		t.Fatal(err)
	}
	if err := w.CashOut(10_000, 1_000); err != nil {
		t.Fatal(err)
	}
	if w.Player.CleanCash != 20_000 || w.Player.DirtyCash != 28_000 {
		t.Fatalf("clean %d dirty %d, want 20,000 and 28,000", w.Player.CleanCash, w.Player.DirtyCash)
	}
	if w.Today.CashedOut != (CashOut{Amount: 30_000, Fee: 3_000}) || w.Stats.CashedOut != 30_000 || w.Stats.CashOutFees != 3_000 {
		t.Fatalf("today %+v, stats %d and %d", w.Today.CashedOut, w.Stats.CashedOut, w.Stats.CashOutFees)
	}
	w.Over = &Ending{}
	if err := w.CashOut(100, 10); !errors.Is(err, ErrGameOver) {
		t.Fatalf("a run that is over: %v", err)
	}
}
