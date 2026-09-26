package ui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/game"
)

// TestFrontsOffSaleSayWhy (#525): a front that waits on the Dutchman's
// Book is named while the Book has never stood ("You own every front
// there is." only when you do), and once the Book is lost it stays on
// offer, locked, `needs` the Book, not dropped from the list.
func TestFrontsOffSaleSayWhy(t *testing.T) {
	m := newTestModel(t, 120, 40)
	w := m.w
	w.Stats.PeakCash = 1 << 40
	book := m.cfg.Assets.ByEffect(content.AssetSupplier)
	var gated []string
	for _, o := range m.rules.Laundering.Offers() {
		if o.Asset == book.ID {
			gated = append(gated, o.ID)
			continue
		}
		w.Fronts = append(w.Fronts, game.Front{ID: o.ID, Name: o.Name})
	}
	if len(gated) == 0 || len(m.frontRows()) != 0 {
		t.Fatalf("fixture: %d gated, %d on offer", len(gated), len(m.frontRows()))
	}
	if got := m.noFrontsOnOffer(); strings.Contains(got, "every front") || !strings.Contains(got, "waits on "+book.Name) {
		t.Errorf("with the bank unbought the ledger says %q", got)
	}
	w.Assets = append(w.Assets, game.Asset{ID: book.ID, Name: book.Name, Effect: book.Effect, City: book.City})
	w.LoseAsset(book.ID, w.Day, "seized")
	rows := m.frontRows()
	if len(rows) != len(gated) {
		t.Fatalf("with the book lost %d fronts on offer, want the %d that wait on it", len(rows), len(gated))
	}
	for _, r := range m.offerRows(rows, 200) {
		if st := fmt.Sprint(r[len(r)-1]); !strings.Contains(st, "needs "+book.Name) {
			t.Errorf("%v: status %q, want needs the book", r[0], st)
		}
	}
	for _, id := range gated {
		w.Fronts = append(w.Fronts, game.Front{ID: id})
	}
	if got := m.noFrontsOnOffer(); got != "You own every front there is." {
		t.Errorf("with every front owned the ledger says %q", got)
	}
}

// TestWalkAwayWaitsOnTonightsTransfer (#525): the night of a transfer
// over the lot, every way out that was open is closed on the walk away's
// page, naming the pages, retire and vanish alike.
func TestWalkAwayWaitsOnTonightsTransfer(t *testing.T) {
	m := newTestModel(t, 120, 40)
	w := m.w
	off := m.rules.Laundering.Offshore()
	w.Offshore, w.QuietDays = off.RetireCash, off.RetireDays
	w.Upgrades["identity"] = true
	w.Player.CleanCash = off.Lot * 3
	if err := m.sess.Reserve(off.Lot * 3); err != nil {
		t.Fatal(err)
	}
	if m.sess.PagesPending() <= 0 {
		t.Fatal("three lots file no page")
	}
	for _, r := range m.exitRows() {
		if r.name != "Retire" && r.name != "Vanish" {
			continue
		}
		if r.open || !strings.Contains(r.short, "today's transfer puts") {
			t.Errorf("%s the night of the transfer: open %v, short %q", r.name, r.open, r.short)
		}
	}
}
