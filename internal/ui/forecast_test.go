package ui

import (
	"strings"
	"testing"

	"github.com/theclifmeister/kingpin/internal/game"
)

// TestForecastInTheGrammar (#397): a load due tonight that takes the
// pile past the cover is a dashboard alert pointing at the ledger, and
// the ledger's tonight line adds it up; without the load neither warns.
func TestForecastInTheGrammar(t *testing.T) {
	m := richModel(t, 100, 30)
	w := m.w
	line := m.rules.Heat.ExposureLine(w)
	w.Player.DirtyCash = line
	m.Update(key("1"))
	if alerts := stripANSI(strings.Join(m.alertLines(100, 14), "\n")); strings.Contains(alerts, "Tonight's") {
		t.Fatalf("no load and the alert warns:\n%s", alerts)
	}
	w.Exports.Loads = []game.ExportLoad{{ID: 1, Lane: "sea", Product: w.Products[0], Units: 1_000, Price: 5_000, Left: w.Day - 3, Lands: w.Day + 1}}
	fc := m.sess.Forecast()
	if fc.Loads != 1 || fc.Landings != 5_000_000 || fc.Pile != line+5_000_000-fc.Wages || fc.Heat <= 0 {
		t.Fatalf("the forecast: %+v (line %d)", fc, line)
	}
	if alerts := stripANSI(strings.Join(m.alertLines(100, 14), "\n")); !strings.Contains(alerts, "Tonight's 1 load land") || !strings.Contains(alerts, "before the wash.") {
		t.Fatalf("the alerts lack tonight's landing:\n%s", alerts)
	}
	m.Update(key("7"))
	view := stripANSI(m.View())
	if !strings.Contains(view, "tonight") || !strings.Contains(view, "at the count") || !strings.Contains(view, "past the cover") {
		t.Fatalf("the ledger lacks the tonight line:\n%s", view)
	}
	assertFits(t, m.View(), 100, 30, "ledger with the forecast")
}
