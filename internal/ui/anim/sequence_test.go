package anim

import (
	"strings"
	"testing"
	"time"
)

// TestSequence: the steps play in order, each on its own clock, the
// last held past the end, and Done at the sum.
func TestSequence(t *testing.T) {
	a, b := NewText("aaa"), NewText("bbb")
	s := Sequence(
		Step{Still(a, ""), 100 * time.Millisecond},
		Step{Still(b, ""), 200 * time.Millisecond},
	)
	line := func(at time.Duration) string { return strings.TrimSpace(stripANSI(s.Frame(at, 10, 1)[0])) }
	for at, want := range map[time.Duration]string{0: "aaa", 99 * time.Millisecond: "aaa", 100 * time.Millisecond: "bbb", 299 * time.Millisecond: "bbb", time.Second: "bbb"} {
		if got := line(at); got != want {
			t.Errorf("at %v: %q, want %q", at, got, want)
		}
	}
	if s.Done(299*time.Millisecond) || !s.Done(300*time.Millisecond) {
		t.Error("Done is not at the sum of the steps")
	}
	if f := Sequence().Frame(0, 4, 2); len(f) != 2 || f[0] != "" || f[1] != "" {
		t.Errorf("an empty sequence drew %q", f)
	}
}
