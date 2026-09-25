package ui

import (
	"strings"
	"testing"

	"github.com/theclifmeister/kingpin/internal/engine"
)

// TestPortAlertWords (#476): with the wholesaler's door open and the
// port untouched, the dashboard names the port's free corners, the
// dearest thing listed there, the wholesaler's share of street and
// where the road is (designer, the port's product, lists at the same
// line); o turns the map to the port.
func TestPortAlertWords(t *testing.T) {
	m := newTestModel(t, 120, 40)
	w := m.w
	var far string
	for _, cid := range w.CityOrder {
		if cid != w.Home().ID && w.WholesaleSupplier(cid) != nil {
			far = cid
		}
	}
	whole := w.WholesaleSupplier(far)
	w.Stats.PeakCash = whole.UnlockCash
	m.sess.EndDay() // the morning the door opens: the prices stamped, designer listed
	got := m.alertsOf(engine.AlertPort)
	if len(got) != 1 {
		t.Fatalf("port alerts %+v, want one", got)
	}
	text := stripANSI(got[0].text)
	for _, want := range []string{w.CityName(far) + " is untouched:", "free corner", "Designer $", whole.Name + " sells at", "% of street", "on the map screen (5)"} {
		if !strings.Contains(text, want) {
			t.Errorf("the port alert %q lacks %q", text, want)
		}
	}
	if !strings.Contains(got[0].why, w.CityName(far)) {
		t.Errorf("the stop line %q does not name the port", got[0].why)
	}
	pressAlert(t, m, engine.AlertPort)
	if m.screen != screenMap || m.shown().ID != far {
		t.Fatalf("o landed on screen %v turned to %s, not the map on %s", m.screen, m.shown().ID, far)
	}
}
