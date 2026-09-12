package ui

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/theclifmeister/kingpin/internal/game"
)

// Unlocks are announced (#148): F stops on a gate crossed and on the
// rival moving in, the dashboard carries the nearest gate ahead while
// it is within reach, keyed so F stops once for it, and the screens
// name the line before it fires.

// laundromat is the first front on offer and its line.
func laundromat(t *testing.T, m *Model) game.FrontOffer {
	t.Helper()
	rows := m.frontRows()
	if len(rows) == 0 || rows[0].ID != "laundromat" {
		t.Fatalf("the first offer is %+v", rows)
	}
	return rows[0]
}

// unlockAlert is the dashboard's alert for a gate, or nil.
func unlockAlert(m *Model, key string) *alert {
	for _, a := range m.alerts() {
		if a.key == key {
			return &a
		}
	}
	return nil
}

// F stops on the morning the laundromat opens, with the report opening
// on `the Laundromat is open to you`, and on the rival moving in.
func TestFastForwardStopsOnAnUnlock(t *testing.T) {
	m := newTestModel(t, 80, 24)
	m.startRun(5) // a seed whose first week is quiet
	o := laundromat(t, m)
	// Past the ladder's first two lines already, so the products are
	// listed and announced on the first day and not on the front's.
	m.w.Player.DirtyCash = o.UnlockCash - 1_000
	m.w.Stats.PeakCash = m.w.Player.DirtyCash
	fast(t, m, 1)
	closeMorning(t, m)
	if o.Locked(m.w) != true || m.w.Laundering.Offered["laundromat"] {
		t.Fatalf("the laundromat is open at peak %d: %+v", m.w.Stats.PeakCash, m.w.Laundering)
	}
	m.w.Player.DirtyCash = o.UnlockCash + 1_000 // the clock stamps the peak tonight
	day := m.w.Day
	fast(t, m, 30)
	if m.w.Day != day+1 {
		t.Fatalf("F ran from day %d to %d: %q", day, m.w.Day, m.fastStop)
	}
	if got := reportLine(t, m); got != "Stopped after 1 day: the Laundromat is open to you." {
		t.Fatalf("the report opens with %q", got)
	}
	if !m.w.Laundering.Offered["laundromat"] || o.Locked(m.w) {
		t.Fatalf("the laundromat after the stop: locked %v offered %v", o.Locked(m.w), m.w.Laundering.Offered)
	}
	view := stripANSI(m.View())
	if !strings.Contains(view, "UNLOCKED") || !strings.Contains(view, "The Laundromat is open to you on the ledger screen (7): $25K.") {
		t.Fatalf("the report has no UNLOCKED section for it:\n%s", view)
	}
	if i, j := strings.Index(view, "UNLOCKED"), strings.Index(view, "PRICES"); j > 0 && i > j {
		t.Fatalf("UNLOCKED is not the first section:\n%s", view)
	}
	assertFits(t, m.View(), 80, 24, "report on the unlock")
	closeMorning(t, m)
	// Not again: the next F stops for something else.
	fast(t, m, 30)
	if strings.Contains(m.fastStop, "Laundromat") {
		t.Fatalf("stopped again for the laundromat: %q", m.fastStop)
	}

	// The rival moving in.
	m = newTestModel(t, 80, 24)
	m.startRun(5)
	arrived := 0
	for m.w.Day < 40 && arrived == 0 {
		before := m.w.Rival.Arrived
		fast(t, m, 30)
		if before == 0 && m.w.Rival.Arrived != 0 {
			arrived = m.w.Day
			want := fmt.Sprintf(": %s moved in on ", m.w.Rival.Leader)
			if !strings.Contains(m.fastStop, want) || m.w.Rival.Arrived != m.w.Day {
				t.Fatalf("the rival moved in on day %d and F stopped with %q", m.w.Rival.Arrived, m.fastStop)
			}
		}
		closeMorning(t, m)
	}
	if arrived == 0 {
		t.Fatalf("the rival never moved in by day %d", m.w.Day)
	}
}

// The nearest gate ahead is an alert while it is within reach, under
// unlockNear of its line to go: it appears the morning the peak crosses
// half the line, stops F once (keyed per gate), and goes the morning
// the gate fires, when the Unlocked stops F instead.
func TestUnlockAlerts(t *testing.T) {
	m := newTestModel(t, 80, 24)
	m.startRun(5)
	o := laundromat(t, m)
	key := "unlock:front:laundromat"
	half := int(float64(o.UnlockCash) * unlockNear)
	m.w.Player.DirtyCash = half - 500
	m.w.Stats.PeakCash = m.w.Player.DirtyCash
	if a := unlockAlert(m, key); a != nil {
		t.Fatalf("an alert at %s to go: %+v", cash(o.UnlockCash-m.w.Stats.PeakCash), a)
	}
	m.w.Player.DirtyCash = half + 500 // within reach once the clock stamps it
	day := m.w.Day
	fast(t, m, 30)
	if m.w.Day != day+1 || m.fastStop != "Stopped after 1 day: the Laundromat within reach." {
		t.Fatalf("day %d -> %d: %q", day, m.w.Day, m.fastStop)
	}
	a := unlockAlert(m, key)
	if a == nil {
		t.Fatalf("no alert at %s to go: %+v", cash(o.UnlockCash-m.w.Stats.PeakCash), m.alerts())
	}
	if want := fmt.Sprintf("The Laundromat opens at %s peak: %s to go.", cash(o.UnlockCash), cash(o.UnlockCash-m.w.Stats.PeakCash)); stripANSI(a.text) != want {
		t.Fatalf("alert %q, want %q", stripANSI(a.text), want)
	}
	closeMorning(t, m)
	if !strings.Contains(stripANSI(m.View()), "The Laundromat opens at") {
		t.Fatalf("the dashboard does not carry the alert:\n%s", stripANSI(m.View()))
	}
	// Keyed: the same alert does not stop the next F.
	fast(t, m, 30)
	if strings.Contains(m.fastStop, "Laundromat") {
		t.Fatalf("stopped again for the alert: %q", m.fastStop)
	}
	closeMorning(t, m)
	// The gate fires: the alert goes and the unlock stops F.
	m.w.Player.DirtyCash = o.UnlockCash + 1
	fast(t, m, 30)
	if m.fastStop != "Stopped after 1 day: the Laundromat is open to you." {
		t.Fatalf("on the morning it opened: %q", m.fastStop)
	}
	if a := unlockAlert(m, key); a != nil {
		t.Fatalf("the alert is still up after the gate fired: %+v", a)
	}
	closeMorning(t, m)
	// The next gate ahead is the pool connect's and the car wash's
	// line; neither is within reach at the laundromat's, so nothing.
	for _, a := range m.alerts() {
		if strings.HasPrefix(a.key, "unlock:") {
			t.Fatalf("an alert for a gate not within reach: %+v", a)
		}
	}
}

// The line named before it fires: the market pane's NOTES name the next
// product on the ladder and what it takes, the ledger's ON OFFER status
// reads the distance, and the crew screen's pool title says what the
// lieutenants wait on until they come.
func TestUnlockLinesBeforeTheyFire(t *testing.T) {
	m := newTestModel(t, 120, 40)
	m.startRun(5)
	m.w.Stats.PeakCash = 1_800
	o := laundromat(t, m)
	m.Update(key("2"))
	if pane := strings.ReplaceAll(paneText(m), "\n", " "); !strings.Contains(pane, "Heroin lists at $3,000 peak cash ($1,200 to go).") {
		t.Fatalf("the market's NOTES do not name the next product:\n%s", pane)
	}
	m.Update(key("7"))
	if view := stripANSI(m.View()); !strings.Contains(view, cash(o.UnlockCash-1_800)+" to go") {
		t.Fatalf("the ledger's status is not the distance:\n%s", view)
	}
	m.Update(tea.WindowSizeMsg{Width: 160, Height: 40})
	if view := stripANSI(m.View()); !strings.Contains(view, "locked · "+cash(o.UnlockCash-1_800)+" to go") {
		t.Fatalf("the ledger's status at 160 columns:\n%s", view)
	}
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m.Update(key("4"))
	m.Update(tea.WindowSizeMsg{Width: 160, Height: 40})
	if view := stripANSI(m.View()); !strings.Contains(view, "lieutenants come looking once you hold corners in two cities") {
		t.Fatalf("the pool title does not say what the lieutenants wait on:\n%s", view)
	}
	for _, sz := range [][2]int{{120, 40}, {80, 24}} {
		m.Update(tea.WindowSizeMsg{Width: sz[0], Height: sz[1]})
		if view := stripANSI(m.View()); !strings.Contains(view, "lieutenants once you hold two cities") {
			t.Fatalf("the pool title at %d columns:\n%s", sz[0], view)
		}
	}
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	// Once the ladder is listed here, no note; once lieutenants come,
	// no line.
	m.w.Stats.PeakCash = 1_000_000
	for _, cid := range m.w.CityOrder {
		m.w.Cities[cid].Corners[0].Owner = game.OwnerPlayer
	}
	m.Update(key("n"))
	closeMorning(t, m)
	m.Update(key("2"))
	if pane := paneText(m); strings.Contains(pane, "lists at") {
		t.Fatalf("a next product with the ladder listed:\n%s", pane)
	}
	m.Update(key("4"))
	if view := stripANSI(m.View()); strings.Contains(view, "lieutenants once") || strings.Contains(view, "lieutenants come") {
		t.Fatalf("the pool title still names the lieutenants' line:\n%s", view)
	}
}
