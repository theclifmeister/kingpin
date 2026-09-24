package engine_test

import (
	"testing"

	"github.com/theclifmeister/kingpin/internal/engine"
	"github.com/theclifmeister/kingpin/internal/events"
)

// TestReignStopsOnce (#399): the fast-forward stops the morning the
// run's first reign begins, runs past a reign begun again, and stops
// on the reign breaking (which comes past the grace) and on going
// straight opening or lapsing (#398).
func TestReignStopsOnce(t *testing.T) {
	for _, c := range []struct {
		e    events.Event
		want bool
	}{
		{events.ReignBegan{}, true},
		{events.ReignBegan{Again: true}, false},
		{events.ReignBroken{}, true},
		{events.StraightOpened{}, true},
		{events.StraightLapsed{}, true},
	} {
		if got := engine.StopsOn(c.e); got != c.want {
			t.Errorf("StopsOn(%+v) = %v, want %v", c.e, got, c.want)
		}
	}
}
