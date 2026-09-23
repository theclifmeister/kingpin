package ui

import (
	"testing"

	"github.com/theclifmeister/kingpin/internal/engine"
	"github.com/theclifmeister/kingpin/internal/events"
)

// TestEveryStopHasWords (#298): which events stop a fast-forward is the
// engine's (engine.StopsOn), the words are the TUI's (stopEvent). Every
// kind the engine stops on at its zero value has words here, so a kind
// added to the engine's list without a line in stopEvent fails by name
// instead of stopping on its bare kind.
func TestEveryStopHasWords(t *testing.T) {
	m := richModel(t, 80, 24)
	stops := 0
	for _, e := range events.All {
		if !engine.StopsOn(e) {
			if got := m.stopEvent(e); got != "" {
				t.Errorf("%s does not stop, yet stopEvent words it %q", e.Kind(), got)
			}
			continue
		}
		stops++
		if got := m.stopEvent(e); got == "" || got == e.Kind() {
			t.Errorf("the engine stops on %s and stopEvent has no words for it (%q)", e.Kind(), got)
		}
	}
	if stops < 30 {
		t.Fatalf("only %d kinds stop at their zero value; the list is wrong", stops)
	}
}
