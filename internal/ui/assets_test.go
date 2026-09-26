package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbletea"
	"github.com/theclifmeister/kingpin/internal/content"
)

// The assets in the grammar (#48): the ledger carries no ASSETS block
// until the cartel is in view, then the offers with their lines in
// clean cash; the picker's third kind buys one with clean cash only;
// bought, the ledger's row reads standing, its pane says what it does,
// the map lists the route it opened and the gauge marks the task
// force's line; a task force announced is an alert. Everything fits at
// the three sizes.
func TestAssetsInTheGrammar(t *testing.T) {
	m := richModel(t, 100, 30)
	w := m.w
	ld := m.rules.Laundering
	offers := ld.AssetOffers()
	first := offers[0]
	m.Update(key("7"))
	if view := stripANSI(m.View()); strings.Contains(view, "ASSETS") {
		t.Fatalf("the ledger carries ASSETS with the cartel out of view:\n%s", view)
	}
	for _, r := range m.rules.Heat.Ladder(w, w.Here()) {
		if r.Level == content.TaskForce {
			t.Fatal("the gauge marks the task force with nothing owned")
		}
	}
	// The first line within reach: the block appears, locked.
	w.Stats.PeakClean = first.UnlockCash * 3 / 4
	view := stripANSI(m.View())
	if !strings.Contains(view, "ASSETS") || !strings.Contains(view, first.Name) || !strings.Contains(view, "clean to go") {
		t.Fatalf("the ledger lacks the locked offer:\n%s", view)
	}
	// Open, and the picker's asset page buys it with clean cash only.
	w.Stats.PeakClean = first.UnlockCash
	w.Player.CleanCash = first.Cost / 2
	w.Player.DirtyCash = first.Cost * 10
	m.Update(key("b"))
	m.Update(key("j"))
	m.Update(key("j"))
	m.Update(key("enter"))
	if m.mode != modeFront || m.front.kind != pickAsset || m.front.step != 1 {
		t.Fatalf("the picker is not on the asset page: mode %v kind %d step %d", m.mode, m.front.kind, m.front.step)
	}
	for _, sz := range [][2]int{{80, 24}, {100, 30}, {120, 40}} {
		m.Update(tea.WindowSizeMsg{Width: sz[0], Height: sz[1]})
		assertFits(t, m.View(), sz[0], sz[1], "asset page")
		if view := stripANSI(m.View()); !strings.Contains(view, "BUY AN ASSET") || !strings.Contains(view, first.Name) {
			t.Fatalf("%v: the asset page lacks the offer:\n%s", sz, view)
		}
	}
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m.Update(key("enter"))
	if m.mode != modePlay || len(w.Assets) != 0 || !strings.Contains(m.status, "clean") {
		t.Fatalf("bought with half the price in clean cash: assets %d status %q", len(w.Assets), m.status)
	}
	w.Player.CleanCash = first.Cost
	m.Update(key("b"))
	m.Update(key("3"))     // the asset row: a digit moves (#500)
	m.Update(key("enter")) // the offers
	m.Update(key("enter"))
	if m.mode != modePlay || len(w.Assets) != 1 || w.Assets[0].ID != first.ID || w.Player.CleanCash != 0 || w.Player.DirtyCash != first.Cost*10 {
		t.Fatalf("the buy: assets %v clean %d dirty %d status %q", w.Assets, w.Player.CleanCash, w.Player.DirtyCash, m.status)
	}
	if !strings.Contains(m.status, "Bought "+first.Name) {
		t.Fatalf("status %q", m.status)
	}
	// The ledger's row and pane, and the gauge.
	for _, sz := range [][2]int{{80, 24}, {100, 30}, {120, 40}} {
		m.Update(tea.WindowSizeMsg{Width: sz[0], Height: sz[1]})
		assertFrame(t, m, "ledger with an asset")
	}
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	view = stripANSI(m.View())
	if !strings.Contains(view, "1 asset owned") || !strings.Contains(view, "standing") {
		t.Fatalf("the ledger lacks the asset:\n%s", view)
	}
	for m.ledgerSelected().kind != ledgerAsset {
		m.Update(key("j"))
	}
	view = stripANSI(m.View())
	if !strings.Contains(view, strings.ToUpper(first.Name)) || !strings.Contains(view, "upkeep") || !strings.Contains(view, "forms at heat") {
		t.Fatalf("the pane lacks the asset's section:\n%s", view)
	}
	marked := false
	for _, r := range m.rules.Heat.Ladder(w, w.Here()) {
		marked = marked || r.Level == content.TaskForce
	}
	if !marked {
		t.Fatal("the gauge does not mark the task force with an asset owned")
	}
	m.Update(key("1"))
	if view := stripANSI(m.View()); !strings.Contains(view, "task force") {
		t.Fatalf("the dashboard's HEAT panel lacks the task force's line:\n%s", view)
	}
	// The route the asset opens is on the map from the day it is
	// bought (the sims read it tonight), and off it while the asset
	// stands idle.
	for _, r := range m.cfg.Routes.Routes {
		if r.Asset != first.ID {
			continue
		}
		m.Update(key("5"))
		if view := stripANSI(m.View()); !strings.Contains(view, r.Name) {
			t.Fatalf("the map lacks %s with its asset standing:\n%s", r.Name, view)
		}
		w.Assets[0].FrozenUntil = w.Day + 3
		if view := stripANSI(m.View()); strings.Contains(view, r.Name) {
			t.Fatalf("the map lists %s with its asset idle:\n%s", r.Name, view)
		}
		w.Assets[0].FrozenUntil = 0
	}
	// A task force announced this morning: the alert and the ledger's line.
	w.Heat.TaskForceDay = w.Day
	m.Update(key("1"))
	if view := stripANSI(m.View()); !strings.Contains(view, "task force formed") {
		t.Fatalf("no alert for the task force:\n%s", view)
	}
	m.Update(key("7"))
	if view := stripANSI(m.View()); !strings.Contains(view, "comes tonight") {
		t.Fatalf("the ledger does not warn of the task force:\n%s", view)
	}
}
