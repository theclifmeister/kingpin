package ui

import (
	"strings"
	"testing"

	"github.com/theclifmeister/kingpin/internal/game"
)

// TestCutDialog (#47): t on the market opens the cut on the stash where
// you stand, the percent goes through numberField (blank is the most
// the product, the room and the till allow), enter cuts at once and
// the status, the table's qual column and the pane say what the lot
// became; the report next morning carries the cut. A lot at the
// default reads its figure in the column and nothing in the pane.
func TestCutDialog(t *testing.T) {
	m := richModel(t, 120, 40)
	w := m.w
	home := w.Player.Location
	weed := w.Products[0]
	w.SetStock(home, weed, 100-w.Stock(home, weed)+w.Street(home, weed)) // the lot at 100, houses counted
	w.SetQuality(home, weed, 50)
	w.Player.DirtyCash = 100000
	m.Update(key("2"))
	if v := stripANSI(m.View()); !strings.Contains(v, "qual") {
		t.Fatalf("the market table has no quality column:\n%s", v)
	}
	m.Update(key("t"))
	if m.mode != modeCut {
		t.Fatalf("t on the market: mode %v status %q", m.mode, m.status)
	}
	assertFits(t, m.View(), 120, 40, "the cut's product page")
	m.Update(key("enter"))
	if m.lab.step != 1 || m.lab.product != weed {
		t.Fatalf("the cut's number page: step %d product %q", m.lab.step, m.lab.product)
	}
	m.Update(key("5"))
	m.Update(key("0"))
	if v := stripANSI(m.View()); !strings.Contains(v, "After     150 units at quality 33") {
		t.Fatalf("the cut's preview:\n%s", v)
	}
	m.Update(key("enter"))
	if m.mode != modePlay {
		t.Fatalf("enter did not cut: mode %v err %q", m.mode, m.lab.err)
	}
	if l := w.Lot(home, weed); l.Units != 150 || l.Quality < 33.3 || l.Quality > 33.4 {
		t.Fatalf("the lot after the cut: %+v", l)
	}
	if !strings.Contains(m.status, "Cut 100 Weed into 150: quality 50 → 33") {
		t.Fatalf("the status: %q", m.status)
	}
	v := stripANSI(m.View())
	if !strings.Contains(v, "quality     33, sells at ×0.90") {
		t.Fatalf("the pane does not read the cut lot:\n%s", v)
	}
	// Cut what is not there: refused with the reason.
	for _, id := range w.Products {
		w.TakeStock(home, id, w.Stock(home, id))
	}
	m.Update(key("t"))
	if m.mode != modePlay || !strings.Contains(m.status, "Nothing here to cut") {
		t.Fatalf("t with an empty stash: mode %v status %q", m.mode, m.status)
	}
	// The report names the cut.
	w.SetStock(home, weed, 40)
	m.Update(key("t"))
	m.Update(key("enter"))
	m.Update(key("enter")) // blank: the most
	if m.mode != modePlay {
		t.Fatalf("a blank percent: mode %v err %q", m.mode, m.lab.err)
	}
	endDay(t, m)
	if rep := w.Report; rep == nil || !strings.Contains(strings.Join(rep.Sales, "\n"), "Cut 40 Weed into 80") {
		t.Fatalf("the report has no cut line: %+v", w.Report)
	}
}

// TestCookDialog (#47): o on the market is silent with no chemist and
// opens the cook with one; the product page lists what a chemist
// cooks, the units go through numberField (blank is a batch), enter
// queues the order and pays the precursors, and the lot lands after
// cook_days at the chemist's quality with a report line each end.
func TestCookDialog(t *testing.T) {
	m := richModel(t, 120, 40)
	w := m.w
	home := w.Player.Location
	if w.Product(home, "meth") == nil {
		t.Skip("the fixture has no meth")
	}
	w.Player.DirtyCash = 1000000
	m.Update(key("2"))
	m.Update(key("o"))
	if m.mode != modePlay {
		t.Fatalf("o with no chemist: mode %v", m.mode)
	}
	withChemist(m)
	m.Update(key("o"))
	if m.mode != modeCook {
		t.Fatalf("o with a chemist: mode %v status %q", m.mode, m.status)
	}
	if v := stripANSI(m.View()); !strings.Contains(v, "Meth") || strings.Contains(v, "Weed") {
		t.Fatalf("the cook's product page lists the wrong products:\n%s", v)
	}
	assertFits(t, m.View(), 120, 40, "the cook's product page")
	m.Update(key("enter"))
	m.Update(key("2"))
	m.Update(key("0"))
	if v := stripANSI(m.View()); !strings.Contains(v, "Cost      $6,000 for 20") {
		t.Fatalf("the cook's cost line:\n%s", v)
	}
	cash := w.Player.DirtyCash
	m.Update(key("enter"))
	if m.mode != modePlay {
		t.Fatalf("enter did not cook: mode %v err %q", m.mode, m.lab.err)
	}
	if len(w.Crew.Cooks) != 1 || w.Crew.Cooks[0].Units != 20 || w.Crew.Cooks[0].Quality != 78 || cash-w.Player.DirtyCash != 6000 {
		t.Fatalf("the cook: %+v, cash moved %d", w.Crew.Cooks, cash-w.Player.DirtyCash)
	}
	if !strings.Contains(m.status, "Doc is cooking 20 Meth at quality 78, ready in 3 days") {
		t.Fatalf("the status: %q", m.status)
	}
	if v := stripANSI(m.View()); !strings.Contains(v, "cooking     20 on the way") {
		m.cursor = 4
		if v := stripANSI(m.View()); !strings.Contains(v, "cooking     20 on the way") {
			t.Fatalf("the pane does not read the cook:\n%s", v)
		}
	}
	before := w.Stock(home, "meth")
	endDay(t, m)
	if !strings.Contains(strings.Join(w.Report.Crew, "\n"), "Doc is cooking 20 Meth") {
		t.Fatalf("the report has no cook line: %+v", w.Report.Crew)
	}
	for w.Stock(home, "meth") == before && w.Day < 10 {
		m.Update(key("enter"))
		endDay(t, m)
	}
	if got := w.Stock(home, "meth"); got != before+20 || w.Quality(home, "meth") == w.StreetQuality() {
		t.Fatalf("the lot did not land: stock %d (was %d), quality %v", got, before, w.Quality(home, "meth"))
	}
	if !strings.Contains(strings.Join(w.Report.Crew, "\n"), "Doc's batch landed") {
		t.Fatalf("the report has no landing line: %+v", w.Report.Crew)
	}
	_ = game.RoleChemist
}
