package game

import (
	"errors"
	"testing"
)

// TestShortErrorIsOneWording (#275): a purchase the player cannot cover
// is refused with a ShortError whatever it buys, written with
// format.Money in one wording, and takes nothing; payDirty and payClean
// take exactly the cost when it is covered, from their own pool alone.
func TestShortErrorIsOneWording(t *testing.T) {
	w := testWorld()
	w.Player.DirtyCash, w.Player.CleanCash = 1_000, 2_000
	err := w.payDirty(1_234)
	var short *ShortError
	if !errors.As(err, &short) || err.Error() != "need $1,234, only have $1,000 dirty" {
		t.Fatalf("dirty short: %v", err)
	}
	if err := w.payClean(2_001); err == nil || err.Error() != "need $2,001, only have $2,000 clean" {
		t.Fatalf("clean short: %v", err)
	}
	if w.Player.DirtyCash != 1_000 || w.Player.CleanCash != 2_000 {
		t.Fatalf("a refusal took cash: dirty %d clean %d", w.Player.DirtyCash, w.Player.CleanCash)
	}
	if err := w.payDirty(1_000); err != nil || w.Player.DirtyCash != 0 || w.Player.CleanCash != 2_000 {
		t.Fatalf("dirty pay: %v dirty %d clean %d", err, w.Player.DirtyCash, w.Player.CleanCash)
	}
	if err := w.payClean(500); err != nil || w.Player.CleanCash != 1_500 || w.Player.DirtyCash != 0 {
		t.Fatalf("clean pay: %v dirty %d clean %d", err, w.Player.DirtyCash, w.Player.CleanCash)
	}
	if got := (&ShortError{Need: 3_500_000, Have: 12}).Error(); got != "need $3,500,000, only have $12" {
		t.Fatalf("both pools: %q", got)
	}
	if _, err := w.BuyHouse(testHouse("h1", 30)); !errors.As(err, &short) || short.Need != 1_000 || short.Pool != "dirty" {
		t.Fatalf("a house with nothing dirty: %v", err)
	}
}
