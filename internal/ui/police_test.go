package ui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/theclifmeister/kingpin/internal/game"
)

// policeDown walks the dashboard's arrows past the product table onto
// the police (#355).
func policeDown(m *Model) {
	m.Update(key("1"))
	for i := 0; i <= len(m.w.Products); i++ {
		m.Update(key("down"))
	}
}

// sectionText is a section's lines, plain.
func sectionText(s section) []string {
	var out []string
	for _, l := range s.lines {
		out = append(out, strings.TrimRight(stripANSI(l), " "))
	}
	return out
}

// hasLine reports whether a line of ls starts with prefix and holds
// every one of the parts after it.
func hasLine(ls []string, prefix string, parts ...string) bool {
	for _, l := range ls {
		if !strings.HasPrefix(l, prefix) {
			continue
		}
		ok := true
		for _, p := range parts {
			ok = ok && strings.Contains(l, p)
		}
		if ok {
			return true
		}
	}
	return false
}

// TestPoliceSectionAgreesWithTheSim (#355): for a table of worlds (cool
// and past the sting line, under pressure and bought goodwill, a pile
// under and over the line with the fronts' cover, a file a page from the
// line, the feds on the ladder) the POLICE section's lines are heat.Sim's:
// a row a rung at its line with what Rungs says it takes, the heat's
// distance to the first rung above it, the file at EvidenceArrest, and
// the pile against ExposureLine with the Cover itemised.
func TestPoliceSectionAgreesWithTheSim(t *testing.T) {
	cases := []struct {
		name                     string
		heat, pressure, goodwill float64
		dirty                    func(line int) int
		pages                    func(arrest int) int
	}{
		{"cool", 12, 0, 0, func(int) int { return 1000 }, func(int) int { return 0 }},
		{"past the sting", 60, 80, 0, func(l int) int { return l + 1 }, func(a int) int { return a - 1 }},
		{"goodwill", 45, 50, 60, func(l int) int { return l / 2 }, func(int) int { return 1 }},
		{"the feds", 90, 20, 10, func(int) int { return 30_000_000 }, func(int) int { return 0 }},
	}
	for _, c := range cases {
		m := richModel(t, 120, 40)
		w := m.w
		here := w.Here()
		here.Heat, here.Pressure, here.Goodwill = c.heat, c.pressure, c.goodwill
		w.Player.DirtyCash = c.dirty(m.rules.Heat.ExposureLine(w))
		arrest := m.rules.Heat.EvidenceArrest(w)
		w.Heat.Evidence = c.pages(arrest)
		sec := m.policeSection(here)
		ls := sectionText(sec)
		if want := "POLICE · " + strings.ToUpper(here.Name); sec.title != want {
			t.Fatalf("%s: titled %q, want %q", c.name, sec.title, want)
		}
		rungs := m.rules.Heat.Rungs(w, here)
		if c.name == "the feds" && rungs[len(rungs)-2].Level != "taskforce" {
			t.Fatalf("%s: the ladder has no feds: %+v", c.name, rungs)
		}
		next := ""
		for _, r := range rungs {
			label := fmt.Sprintf("%s %.0f", rungName(r.Level), r.Threshold)
			if !hasLine(ls, label, rungWords(r)[0]) {
				t.Errorf("%s: no row %q %q:\n%s", c.name, label, rungWords(r)[0], strings.Join(ls, "\n"))
			}
			if next == "" && here.Heat < r.Threshold {
				next = fmt.Sprintf("%s in %.0f", rungName(r.Level), r.Threshold-here.Heat)
			}
		}
		if !hasLine(ls, "heat", fmt.Sprintf("%.0f", here.Heat), next) {
			t.Errorf("%s: the heat row lacks %q:\n%s", c.name, next, strings.Join(ls, "\n"))
		}
		if !hasLine(ls, "file", fmt.Sprintf("%d/%d · %s to go", w.Heat.Evidence, arrest, plural(arrest-w.Heat.Evidence, "page"))) {
			t.Errorf("%s: the file row is not %d/%d:\n%s", c.name, w.Heat.Evidence, arrest, strings.Join(ls, "\n"))
		}
		if prose := strings.Join(ls, " "); !strings.Contains(prose, "a quiet day files nothing") {
			t.Errorf("%s: the file does not say a quiet day files nothing:\n%s", c.name, prose)
		}
		line, cover := m.rules.Heat.ExposureLine(w), m.rules.Heat.Cover(w)
		if !hasLine(ls, "dirty", cash(w.Player.DirtyCash)+" of "+cash(line)) {
			t.Errorf("%s: the pile's row is not %s of %s:\n%s", c.name, cash(w.Player.DirtyCash), cash(line), strings.Join(ls, "\n"))
		}
		if cover > 0 && !hasLine(ls, "", cash(line-cover)+" + "+cash(cover)+" fronts") {
			t.Errorf("%s: the cover is not itemised as %s + %s:\n%s", c.name, cash(line-cover), cash(cover), strings.Join(ls, "\n"))
		}
		over := hasLine(ls, "", "over: heat every day")
		if _, past := m.exposure(); over != past {
			t.Errorf("%s: over the line %v, the pane says %v", c.name, past, over)
		}
		if !hasLine(ls, "pressure", fmt.Sprintf("%.0f", here.Pressure)) || (c.goodwill > 0) != hasLine(ls, "pressure", "goodwill") {
			t.Errorf("%s: the pressure row:\n%s", c.name, strings.Join(ls, "\n"))
		}
	}
}

// The forecast is the file's (#355, docs/intel.md): no word, and the
// section says so; a cop's word, right or wrong, is what it shows,
// labelled an estimate with how sure it is today.
func TestPoliceForecastReadsTheFile(t *testing.T) {
	m := richModel(t, 120, 40)
	w := m.w
	here := w.Here()
	w.Unlearn(here.ID, game.FactResponse)
	if !hasLine(sectionText(m.policeSection(here)), "estimate", "no word from inside") {
		t.Fatalf("no word, and the section:\n%s", strings.Join(sectionText(m.policeSection(here)), "\n"))
	}
	here.Heat = 0 // the truth is the patrol; the word says a raid
	w.Learn(game.Fact{Subject: here.ID, Kind: game.FactResponse, Value: "raid", Number: float64(w.Day + 3), Confidence: 0.75, Day: w.Day, Source: game.SourceCop, Stale: 0.05, Forget: 0.2})
	ls := sectionText(m.policeSection(here))
	if !hasLine(ls, "estimate", fmt.Sprintf("raid from day %d", w.Day+3)) || !hasLine(ls, "", "a cop · 75% sure") {
		t.Fatalf("a cop's word, and the section:\n%s", strings.Join(ls, "\n"))
	}
}

// The dashboard's arrows go past the product table onto HEAT, CASH and
// LAW (#355): the three panels are marked, the pane (or the strip under
// 100 columns) leads with POLICE where you stand, up is the table
// again; the map's pane carries POLICE for the city shown, the other
// city's on `]`; every size fits.
func TestPoliceKeys(t *testing.T) {
	for _, sz := range [][2]int{{80, 24}, {100, 30}, {120, 40}} {
		m := richModel(t, sz[0], sz[1])
		here := strings.ToUpper(m.w.Here().Name)
		policeDown(m)
		if !m.onPolice {
			t.Fatalf("%dx%d: the arrows past the table are not on the police", sz[0], sz[1])
		}
		view := stripANSI(m.View())
		assertFits(t, m.View(), sz[0], sz[1], "the dashboard on the police")
		for _, p := range []string{"▸ HEAT", "▸ CASH", "▸ LAW"} {
			if !strings.Contains(view, p) {
				t.Errorf("%dx%d: %q is not marked:\n%s", sz[0], sz[1], p, view)
			}
		}
		if secs := m.details(); secs[0].title != "POLICE · "+here {
			t.Errorf("%dx%d: the pane leads with %q", sz[0], sz[1], secs[0].title)
		}
		if sz[0] < paneMinWidth {
			if !strings.Contains(stripANSI(stripLine(m)), "▸ POLICE · "+here) {
				t.Errorf("%dx%d: the strip is %q", sz[0], sz[1], stripANSI(stripLine(m)))
			}
			m.Update(key(" "))
			assertFits(t, m.View(), sz[0], sz[1], "the police overlay")
			m.Update(key("esc"))
		} else if pane := paneRender(m); !strings.Contains(pane, "POLICE · "+here) || !strings.Contains(pane, "estimate") {
			t.Errorf("%dx%d: the pane does not hold POLICE whole:\n%s", sz[0], sz[1], pane)
		}
		m.Update(key("up"))
		if m.onPolice || strings.Contains(stripANSI(m.View()), "▸ HEAT") {
			t.Fatalf("%dx%d: up from the police is not the table", sz[0], sz[1])
		}
		m.Update(key("down"))
		if !m.onPolice {
			t.Fatalf("%dx%d: down from the last product is not the police", sz[0], sz[1])
		}
		m.Update(key("5"))
		assertFits(t, m.View(), sz[0], sz[1], "the map")
		if secs := m.details(); secs[len(secs)-1].title != "POLICE · "+strings.ToUpper(m.shown().Name) {
			t.Errorf("%dx%d: the map's pane ends with %q", sz[0], sz[1], secs[len(secs)-1].title)
		}
		m.Update(key("]"))
		if secs := m.details(); secs[len(secs)-1].title != "POLICE · "+strings.ToUpper(m.shown().Name) || m.shown() == m.w.Here() {
			t.Errorf("%dx%d: `]` on the map: the pane ends with %q", sz[0], sz[1], secs[len(secs)-1].title)
		}
	}
}
