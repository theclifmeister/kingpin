package ui

import (
	"strings"
	"testing"

	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
)

// The map screen's u (#68): a price war is queued on a rival corner
// next to one you work, at a dial the picker chooses, and can be called
// off; it is refused from other screens, on your own corners, on a
// rival corner none of your worked corners borders, and under a truce.
// The map marks the corner $ and the inspector carries the row.
func TestUndercutKeys(t *testing.T) {
	m := newTestModel(t, 120, 40)
	m.Update(key("u"))
	if m.mode != modePlay || !strings.Contains(m.status, "map") {
		t.Fatalf("u on the dashboard: mode %v status %q", m.mode, m.status)
	}
	m.Update(key("5"))
	m.mapCursor = m.yourCorner()
	m.Update(key("u"))
	if m.mode != modePlay || !strings.Contains(m.status, "rival holds") {
		t.Fatalf("u on your own corner: mode %v status %q", m.mode, m.status)
	}
	// The rival on The Heights, far from you on Fourth & Main.
	heights := m.w.Corner("heights")
	heights.Owner = game.OwnerRival
	m.w.Rival.Arrived, m.w.Rival.Muscle, m.w.Rival.Leader = 1, 3, "Vasquez"
	at := func(id string) {
		for i, c := range m.shown().Corners {
			if c.ID == id {
				m.mapCursor = i
			}
		}
	}
	at("heights")
	m.Update(key("u"))
	if m.mode != modePlay || !strings.Contains(m.status, "next to it") {
		t.Fatalf("u on a rival corner with none of yours next door: mode %v status %q", m.mode, m.status)
	}
	if text := paneText(m); !strings.Contains(text, "Work a corner next door") {
		t.Fatalf("the inspector does not say how to undercut it:\n%s", text)
	}
	// The Docks border Fourth & Main, where you stand.
	docks := m.w.Corner("docks")
	docks.Owner = game.OwnerRival
	at("docks")
	if text := paneText(m); !strings.Contains(text, "u  undercut: takes ~") {
		t.Fatalf("the inspector does not offer the undercut:\n%s", text)
	}
	m.Update(key("u"))
	if m.mode != modeUndercut || len(m.undercutRows()) != 3 {
		t.Fatalf("u next door: mode %v rows %v status %q", m.mode, m.undercutRows(), m.status)
	}
	view := stripANSI(m.View())
	for _, s := range []string{"UNDERCUT", "quiet", "normal", "aggressive", "takes", "units/day", "price", "they lose", "-10%", "Vasquez"} {
		if !strings.Contains(view, s) {
			t.Errorf("the picker does not show %q:\n%s", s, view)
		}
	}
	m.Update(key("3")) // aggressive
	if d, ok := m.w.Undercutting("docks"); m.mode != modePlay || !ok || d != events.DialAggressive {
		t.Fatalf("after picking aggressive: mode %v undercuts %v status %q", m.mode, m.w.Undercuts, m.status)
	}
	if !strings.Contains(m.status, "Undercutting The Docks tonight at aggressive") {
		t.Fatalf("status: %q", m.status)
	}
	view = stripANSI(m.View())
	if !strings.Contains(view, "$ THE DOCKS") || !strings.Contains(view, "$ undercut aggr.") {
		t.Fatalf("the map does not mark the corner under the price war:\n%s", view)
	}
	if text := paneText(m); !strings.Contains(text, "undercut    aggressive · takes ~") {
		t.Fatalf("the inspector does not carry the undercut row:\n%s", text)
	}
	// Reopened, the picker opens on the dial queued and offers stop.
	m.Update(key("u"))
	if rows := m.undercutRows(); len(rows) != 4 || m.undercutCursor != 2 {
		t.Fatalf("picker with an undercut queued: rows %v cursor %d", rows, m.undercutCursor)
	}
	m.Update(key("4")) // stop
	if _, ok := m.w.Undercutting("docks"); ok || !strings.Contains(m.status, "Called off") {
		t.Fatalf("stop did not call it off: %v %q", m.w.Undercuts, m.status)
	}
	// Under a truce it is refused, and the market moves nothing.
	m.w.Rival.Deals = []game.Deal{{Kind: game.DealTruce, Terms: game.Terms{Days: 10}, Since: m.w.Day, Until: m.w.Day + 10}}
	m.Update(key("u"))
	if m.mode != modePlay || !strings.Contains(m.status, "Can't undercut: a truce or a tribute holds") {
		t.Fatalf("u under a truce: mode %v status %q", m.mode, m.status)
	}
	m.w.Rival.Deals = nil
	// Queued, sold and reported: the SALES section carries the corner
	// and the rival corner wakes up squeezed.
	m.Update(key("u"))
	m.Update(key("2"))
	m.w.Stash(m.w.Player.Location)[m.w.Products[0]] = 200
	m.Update(key("1"))
	m.Update(key("s"))
	m.Update(key("enter"))
	m.Update(key("enter"))
	m.Update(key("enter"))
	m.Update(key("enter"))
	m.Update(key("esc"))
	endDay(t, m)
	report := strings.Join(m.w.Report.Sales, "\n")
	if !strings.Contains(report, "Undercut Vasquez on The Docks") {
		t.Fatalf("the report does not carry the undercut:\n%s", report)
	}
	if docks.Squeeze <= 0 || docks.Starved != 1 {
		t.Fatalf("the rival corner is not squeezed the morning after: squeeze %.2f starved %d", docks.Squeeze, docks.Starved)
	}
	if m.w.Undercuts != nil {
		t.Fatalf("the clock did not clear the undercuts: %v", m.w.Undercuts)
	}
	m.Update(key("enter"))
	m.Update(key("5"))
	at("docks")
	if text := paneText(m); !strings.Contains(text, "squeezed") || !strings.Contains(text, "by you, 1 day") {
		t.Fatalf("the inspector does not say what last night took:\n%s", text)
	}
}
