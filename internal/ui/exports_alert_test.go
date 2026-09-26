package ui

import (
	"strings"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/engine"
	"github.com/theclifmeister/kingpin/internal/game"
)

// TestExportsAlertWords (#505): at the Cartel stage with no load ever
// sent, the dashboard names the lanes abroad: the product with the best
// margin, its price abroad and its cost off the book, what a night
// carries and where the lane leaves from, then what to do (buy the book
// and set a load on the ledger; the load alone once the book is owned);
// o opens the ledger.
func TestExportsAlertWords(t *testing.T) {
	m := newTestModel(t, 120, 40)
	w := m.w
	for i, tr := range m.cfg.Progression.Tiers {
		if tr.ID == "cartel" {
			w.Reach(i+1, w.Day)
			w.SeeStage(i + 1)
		}
	}
	m.sess.EndDay() // the prices stamped
	got := m.alertsOf(engine.AlertExports)
	if len(got) != 1 {
		t.Fatalf("exports alerts %+v, want one", got)
	}
	book := m.cfg.Assets.ByEffect(content.AssetSupplier)
	lane := m.cfg.Exports.Lanes[0]
	text := stripANSI(got[0].text)
	for _, want := range []string{"The lanes abroad:", "a unit out of " + w.CityName(lane.City), "off the book", "units a night", "Buy " + book.Name, "t on a lane on the ledger screen (7)"} {
		if !strings.Contains(text, want) {
			t.Errorf("the exports alert %q lacks %q", text, want)
		}
	}
	if got[0].why != "the lanes abroad" {
		t.Errorf("the stop line is %q", got[0].why)
	}
	w.Assets = append(w.Assets, game.Asset{ID: book.ID, Name: book.Name, Effect: book.Effect, City: book.City})
	got = m.alertsOf(engine.AlertExports)
	if len(got) != 1 {
		t.Fatalf("exports alerts with the book %+v, want one", got)
	}
	if text := stripANSI(got[0].text); !strings.Contains(text, "The lanes abroad are open:") || !strings.Contains(text, "t on a lane sets a nightly load") || strings.Contains(text, "Buy ") {
		t.Errorf("with the book owned the alert reads %q", text)
	}
	pressAlert(t, m, engine.AlertExports)
	if m.screen != screenLedger {
		t.Fatalf("o landed on screen %v, not the ledger", m.screen)
	}
}

// TestCartelMorningPointsAtTheLanes (#525): the morning the Cartel is
// reached, before any load is ordered, the exports alert is in the
// alerts list and on the dashboard, and the stage's modal points at the
// lanes on the ledger, once: the stage is shown once.
func TestCartelMorningPointsAtTheLanes(t *testing.T) {
	m := newTestModel(t, 120, 40)
	w := m.w
	cartel := 0
	for i, tr := range m.cfg.Progression.Tiers {
		if tr.ID == "cartel" {
			cartel = i + 1
		}
	}
	for k := 2; k <= cartel; k++ {
		w.Reach(k, w.Day)
		if k < cartel {
			w.SeeStage(k)
		}
	}
	m.sess.EndDay()
	if len(w.Exports.Orders) != 0 {
		t.Fatal("an export order stands before the test sets one")
	}
	listed := false
	for _, a := range m.sess.Alerts() {
		listed = listed || a.Kind == engine.AlertExports
	}
	if !listed {
		t.Fatal("the exports alert is not in the alerts list on the Cartel's morning")
	}
	m.showStage()
	if m.mode != modeStage || m.stage != cartel {
		t.Fatalf("the Cartel's stage is not shown: mode %v stage %d", m.mode, m.stage)
	}
	if view := squash(stripANSI(m.View())); !strings.Contains(view, "the lanes are "+screenPointer(screenLedger)) {
		t.Errorf("the Cartel's stage does not point at the lanes:\n%s", view)
	}
	for m.mode != modePlay {
		m.Update(key("esc"))
	}
	m.Update(key("1"))
	if view := squash(stripANSI(m.View())); !strings.Contains(view, "The lanes abroad") {
		t.Errorf("the dashboard does not carry the exports alert:\n%s", view)
	}
	if m.w.StagePending() != 0 {
		t.Error("the stage is still pending after it was shown")
	}
}
