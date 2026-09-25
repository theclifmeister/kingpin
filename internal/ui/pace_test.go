package ui

import (
	"strconv"
	"strings"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
)

// The hidden rules that set the pace have words (#472): the crown's
// "crews down" is spelled out a faction a line on the rivals screen and
// under the plan's step, in the rule's own terms and with its clock
// (rivals.Sim.Down); the dashboard opens the ambitions on `a`, and help
// lists it; WORDS defines the terms the screens use for it and the
// quiet day, at the file's own line.
func TestCrownSaysWhatKeepsACrew(t *testing.T) {
	m := richModel(t, 120, 40)
	r := m.w.Rival()
	for _, cid := range m.w.CityOrder {
		cs := m.w.Cities[cid].Corners
		for i := range cs {
			if cs[i].FactionID() == r.Faction() {
				cs[i].Owner, cs[i].Faction = game.OwnerNone, ""
			}
		}
	}
	r.Routed, r.LastTakenBy = m.w.Day-3, "somebody"
	absorb := m.cfg.Rivals.Factions.AbsorbDays
	want := "run out 3d ago; gone in " + strconv.Itoa(absorb-3) + "d unless it claims again"

	m.Update(key("8"))
	if view := stripANSI(m.View()); !strings.Contains(view, "for the crown: "+want) {
		t.Errorf("the rivals screen does not say what keeps %s off the count (%q):\n%s", r.Leader, want, view)
	}

	m.Update(key("1"))
	m.Update(key("a"))
	if m.mode != modeAmbitions {
		t.Fatalf("a on the dashboard: mode %v", m.mode)
	}
	for i, a := range m.ambitions() {
		if a.ID == content.AmbitionCity {
			m.amb.cursor = i
		}
	}
	if view := stripANSI(m.View()); !strings.Contains(view, r.Leader+": "+want) {
		t.Errorf("the crown's steps do not name %s and its clock:\n%s", r.Leader, view)
	}
	m.Update(key("esc"))

	// A faction on a corner has no clock, and says so.
	r.Routed = 0
	m.w.Home().Corners[0].Owner, m.w.Home().Corners[0].Faction = game.OwnerRival, r.Faction()
	if got := m.downWords(r); got != "holds 1 corner; no clock while it holds one" {
		t.Errorf("a faction on a corner reads %q", got)
	}

	help := stripANSI(strings.Join(m.helpLines(), "\n"))
	for _, want := range []string{"ambitions", "run out", "absorbed", "scattered", "gone", "quiet day", "score"} {
		if !strings.Contains(help, want) {
			t.Errorf("help does not carry %q", want)
		}
	}
	if strings.Contains(help, "{") {
		t.Errorf("a WORDS line kept its placeholder:\n%s", help)
	}
	if line := "all heat under " + strconv.Itoa(int(m.cfg.Laundering.Offshore.RetireHeat)); !strings.Contains(help, line) {
		t.Errorf("the quiet day does not read the file's line %q", line)
	}
}

// Meaning never rides on colour alone, and the small nits are fixed
// (#473): each journal line carries its source's tag, which the legend
// pairs with the name; the tree's cursor row keeps its state's glyph
// (TestUpgradesScreenInTheGrammar walks every node); the tab bar
// abbreviates at 100 columns; ALERTS keeps a line at 80x24 with the
// whole ladder listed; the seed page lists only keys that work on a
// seed; D turns the launder dial back; the map's arrows go to the cell
// nearest by column.
func TestNothingRidesOnColourAlone(t *testing.T) {
	seen := map[string]string{}
	for _, s := range journalSources {
		tag := strings.TrimSpace(sourceTag(s))
		if tag == "" {
			t.Errorf("%s has no tag", s)
		}
		if o, ok := seen[tag]; ok {
			t.Errorf("%s and %s share the tag %q", s, o, tag)
		}
		seen[tag] = s
	}
	m := richModel(t, 80, 24)
	m.Update(key("3"))
	main, _ := splitView(m)
	for _, l := range main[1:] {
		if l = strings.TrimSpace(l); l == "" {
			continue
		}
		f := strings.Fields(strings.TrimPrefix(l, "▸"))
		if len(f) < 2 || seen[f[1]] == "" {
			t.Errorf("a journal line carries no source tag: %q", l)
		}
	}
	legend := stripANSI(strings.Join(m.details()[len(m.details())-1].lines, "\n"))
	for tag, s := range seen {
		if !strings.Contains(legend, tag+" "+s) {
			t.Errorf("the legend does not pair %q with %s:\n%s", tag, s, legend)
		}
	}

	// The tab bar at 100 columns names its tabs.
	m = richModel(t, 100, 30)
	if top := stripANSI(strings.SplitN(m.View(), "\n", 2)[0]); !strings.Contains(top, "2Mkt") || !strings.Contains(top, "8Riv") {
		t.Errorf("the title bar at 100 columns: %q", top)
	}

	// ALERTS keeps a line at 80x24 with six products on the table.
	m = richModel(t, 80, 24)
	if len(m.w.Products) < 5 || len(m.alerts()) == 0 {
		t.Fatalf("the fixture lists %d products and %d alerts", len(m.w.Products), len(m.alerts()))
	}
	if p := panelNamed(panelsOf(stripANSI(m.View())), "ALERTS"); p == nil || len(p.lines) == 0 {
		t.Errorf("no ALERTS at 80x24:\n%s", stripANSI(m.View()))
	}
	assertFits(t, m.View(), 80, 24, "dashboard with ALERTS")

	// The seed page lists the digits, not the quantity shortcuts, and
	// they do nothing to it.
	m = pickerModel(t)
	m.Update(key("enter"))
	m.Update(key("1"))
	if m.nr.step != 1 {
		t.Fatalf("not on the seed page: step %d", m.nr.step)
	}
	var labels []string
	for _, b := range m.modeKeys(m.mode) {
		labels = append(labels, b.key+" "+b.label)
	}
	if got := strings.Join(labels, ", "); strings.Contains(got, "max") || strings.Contains(got, "half") || strings.Contains(got, "±") || !strings.Contains(got, "0-9 type a seed") {
		t.Errorf("the seed page lists %s", got)
	}
	for _, k := range []string{"4", "m", "up", "pgup", "h"} {
		m.Update(key(k))
	}
	if v := m.nr.seed.Value(); v != "4" {
		t.Errorf("the seed reads %q after 4 m up pgup h", v)
	}

	// D turns the launder dial back a notch: normal to careful, never
	// by way of greedy.
	m = richModel(t, 120, 40)
	m.w.Laundering.Dial = events.LaunderNormal
	m.Update(key("7"))
	m.Update(key("D"))
	if m.w.Laundering.Dial != events.LaunderCareful {
		t.Errorf("D from normal: %s", m.w.Laundering.Dial)
	}
	m.Update(key("d"))
	if m.w.Laundering.Dial != events.LaunderNormal {
		t.Errorf("d from careful: %s", m.w.Laundering.Dial)
	}

	// The map's arrows go to the cell in the next row nearest by column.
	m.Update(key("5"))
	for _, c := range []struct{ from, key, to string }{
		{"The Projects", "down", "Riverside"},
		{"The Strip", "up", "Bus Depot"},
		{"Precinct Row", "up", "Old Mill"},
		{"Fourth & Main", "down", "The Strip"},
	} {
		for i, x := range m.shown().Corners {
			if x.Name == c.from {
				m.mapCursor, m.onRoutes = i, false
			}
		}
		m.Update(key(c.key))
		if got := m.mapSelected(); got == nil || m.onRoutes || got.Name != c.to {
			t.Errorf("%s from %s: %v", c.key, c.from, got)
		}
	}
}
