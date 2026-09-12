package ui

import (
	"regexp"
	"strconv"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// The map in the grammar (#85): MAIN is the title line, the grid and
// the routes; the corner inspector and the route detail are the pane.

// paneText is the pane's sections rendered plain, one line each, for
// asserting what the inspector carries.
func paneText(m *Model) string {
	var ls []string
	for _, s := range m.details() {
		ls = append(ls, s.title)
		for _, l := range s.lines {
			ls = append(ls, stripANSI(l))
		}
	}
	return strings.Join(ls, "\n")
}

// mapFacts is what the corner inspector must say about a corner in
// each state: the old inspector's facts, the hint as key rows.
func mapFacts(t *testing.T, m *Model, c *game.Corner, text, where string) {
	t.Helper()
	w := m.w
	var want []string
	switch {
	case c.Held():
		want = append(want, "yours since day", "robbery", "%/day", "runner", "enforcer")
		if c.Worked() {
			want = append(want, "move a runner here", "post an enforcer", "abandon the corner")
		} else {
			want = append(want, "post a runner here", "post an enforcer", "abandon the corner")
		}
		if w.Contested(*c) {
			want = append(want, "push flips")
		}
	case c.Owner == game.OwnerRival:
		want = append(want, w.Rival.Leader+"'s since day", "holds", "push takes it", "hit ~")
	case m.eyed(c):
		// The tell (#69): the rival may be eyeing the fixture's free
		// corner on some seeds; the hint is then to keep them off.
		want = append(want, "free", "post a runner to keep them off")
	default:
		want = append(want, "free", "post a runner to claim it")
	}
	if c.Squeeze > 0 {
		want = append(want, "undercut", w.Rival.Leader)
	}
	want = append(want, "size", "heat", "risk", "demand")
	for _, id := range w.Products {
		want = append(want, w.ProductName(id)+" ~")
	}
	for _, s := range want {
		if !strings.Contains(text, s) {
			t.Errorf("%s: %s: the inspector does not say %q:\n%s", where, c.Name, s, text)
		}
	}
}

// Every fact the old inspector printed is in the pane at 120 (the
// pane's first section is the corner's name) and in the overlay space
// opens at 80: owner and since, size, heat and risk, robbery, undercut,
// push odds, demand per product, runner, enforcer and the hint, for a
// worked corner, one held with nobody on it, the rival's and a free
// one. At 80x24 the overlay's first screen holds only the top of the
// inspector (the shared overlay decides whether the rest is cut or
// scrolled to), so the whole of it is read at 80x36.
func TestMapInspectorInPane(t *testing.T) {
	for _, sz := range [][2]int{{120, 40}, {80, 36}, {80, 24}} {
		m := richModel(t, sz[0], sz[1])
		m.Update(key("5"))
		home := m.w.Home()
		home.Corners[1].Squeeze = 0.1
		home.Corners[1].Enforcer = 0
		rival, worked, free := -1, -1, -1
		var idle int = -1
		for i := range home.Corners {
			c := &home.Corners[i]
			switch {
			case c.Owner == game.OwnerRival && rival < 0:
				rival = i
			case c.Worked() && c.Squeeze > 0 && worked < 0:
				worked = i
			case !c.Held() && c.Owner != game.OwnerRival && free < 0:
				free = i
			case !c.Held() && c.Owner != game.OwnerRival && idle < 0:
				c.Owner, c.Since = game.OwnerPlayer, m.w.Day
				idle = i
			}
		}
		if rival < 0 || worked < 0 || free < 0 || idle < 0 {
			t.Fatalf("fixture: rival %d worked %d free %d idle %d", rival, worked, free, idle)
		}
		if !m.w.Contested(home.Corners[worked]) {
			t.Fatalf("fixture: %s is not contested", home.Corners[worked].Name)
		}
		for _, i := range []int{worked, idle, rival, free} {
			m.mapCursor, m.onRoutes = i, false
			c := &home.Corners[i]
			secs := m.details()
			if len(secs) == 0 || secs[0].title != strings.ToUpper(c.Name) {
				t.Fatalf("%dx%d: the pane's first section is not %s: %v", sz[0], sz[1], c.Name, secs)
			}
			view := stripANSI(m.View())
			switch {
			case sz[0] >= paneMinWidth:
				mapFacts(t, m, c, view, "the pane")
			default:
				if !strings.Contains(strings.Split(view, "\n")[sz[1]-2], "▸ "+strings.ToUpper(c.Name)) {
					t.Errorf("%dx%d: the strip does not name %s:\n%s", sz[0], sz[1], c.Name, view)
				}
				m.Update(key(" "))
				if m.mode != modeDetails {
					t.Fatalf("space: mode %v", m.mode)
				}
				overlay := stripANSI(m.View())
				assertFits(t, m.View(), sz[0], sz[1], "map overlay")
				if sz[1] >= 36 {
					mapFacts(t, m, c, overlay, "the overlay")
				} else if !strings.Contains(overlay, strings.ToUpper(c.Name)) || !strings.Contains(overlay, "size") {
					t.Errorf("%dx%d: the overlay does not open on %s:\n%s", sz[0], sz[1], c.Name, overlay)
				}
				m.Update(key("esc"))
			}
			mapFacts(t, m, c, paneText(m), "the sections")
		}
		// The rival's count is the inspector's, not the title's.
		m.mapCursor = rival
		if text := paneText(m); !strings.Contains(text, plural(m.w.RivalHeld(), "corner")) {
			t.Errorf("the rival's corner does not say how many they hold:\n%s", text)
		}
		if title := strings.Split(stripANSI(m.View()), "\n")[1]; !strings.Contains(title, "held ·") || !strings.Contains(title, "/day free") || strings.Contains(title, m.w.Rival.Leader) {
			t.Errorf("%dx%d: the title line: %q", sz[0], sz[1], title)
		}
	}
}

// The route detail is the pane's while the cursor is on the routes: the
// edge, the dial, the terms at it, the target, what is on the road and
// what r and R do; the route's row is ▸ and Selected, and the chosen
// corner's name cell stays Selected.
func TestMapRouteInPane(t *testing.T) {
	// Styles render plain with no terminal; the selection is read off
	// the escapes, so the profile is true colour for this test.
	profile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(0) // termenv.TrueColor
	defer lipgloss.SetColorProfile(profile)
	m := richModel(t, 120, 40)
	m.Update(key("5"))
	m.mapCursor = 1
	for !m.onRoutes {
		m.Update(key("j"))
	}
	sel := m.mapSelected() // the corner the walk down left the cursor on
	r := m.selectedRoute()
	secs := m.details()
	if len(secs) == 0 || secs[0].title != strings.ToUpper(r.Name) {
		t.Fatalf("the pane's first section is not %s: %v", r.Name, secs)
	}
	text := paneText(m)
	for _, s := range []string{
		m.w.CityName(r.From) + " ", "▶ " + m.w.CityName(r.To), "dial", "[normal]", "days", "capacity", "fare", "/u", "seized",
		"target", "120 " + m.w.ProductName(m.w.Products[0]), "on the road", "60 " + m.w.ProductName(m.w.Products[0]) + ", ",
		"r  turn the dial", "R  set a target",
	} {
		if !strings.Contains(text, s) {
			t.Errorf("the route detail does not say %q:\n%s", s, text)
		}
	}
	// Selection: the route row is ▸ and drawn Selected across; the
	// corner's name cell is Selected too, on the routes or off them.
	view := m.View()
	selectedOpen := theme.Selected.Render("x")
	selectedOpen = selectedOpen[:strings.Index(selectedOpen, "x")]
	if selectedOpen == "" {
		t.Fatal("Selected renders plain")
	}
	// selected reports whether s opens a Selected cell: the route's name
	// at the start of its row, a corner's after its mark.
	selected := func(s string) bool {
		for _, mark := range []string{"", "· ", "▪ ", "▴ "} {
			if strings.Contains(view, selectedOpen+mark+s) {
				return true
			}
		}
		return false
	}
	if !selected(r.Name) || !strings.Contains(stripANSI(view), "▸ "+r.Name) {
		t.Errorf("the route under the cursor is not ▸ and Selected:\n%s", view)
	}
	if !selected(strings.ToUpper(sel.Name)) {
		t.Errorf("the corner's name cell is not Selected while the cursor is on the routes:\n%s", view)
	}
	m.Update(key("k"))
	for m.onRoutes {
		m.Update(key("k"))
	}
	view = m.View()
	if !selected(strings.ToUpper(sel.Name)) {
		t.Errorf("the corner's name cell is not Selected:\n%s", view)
	}
	if selected(r.Name) || !strings.Contains(stripANSI(view), "▸ "+r.Name) {
		t.Errorf("off the routes the route row is Selected, or lost its ▸:\n%s", view)
	}
	if underlined.MatchString(view) {
		t.Errorf("an underlined cell on the map:\n%s", view)
	}
}

// underlined matches an SGR sequence that turns underline on.
var underlined = regexp.MustCompile(`\x1b\[(\d+;)*4(;\d+)*m`)

// checkMap is what TestRendersAtCommonSizes asks of the map: every
// route line is rendered whole, at 80 the strip names the selected
// corner (or the route when the cursor is on the routes) and from 100
// the pane's first section does, and a shipment in flight shows on its
// route.
func checkMap(t *testing.T, m *Model, view, what string) {
	t.Helper()
	if m.mode != modePlay || m.screen != screenMap {
		return
	}
	plain := stripANSI(view)
	rows := strings.Split(plain, "\n")
	for _, r := range m.mapRoutes() {
		if !strings.Contains(plain, r.Name) {
			t.Errorf("%dx%d %s: the route %s is not listed:\n%s", m.width, m.height, what, r.Name, plain)
		}
		if n := m.unitsOn(r.ID); n > 0 && !strings.Contains(plain, "▪"+strconv.Itoa(n)) {
			t.Errorf("%dx%d %s: the shipment on %s is not shown:\n%s", m.width, m.height, what, r.Name, plain)
		}
	}
	if !strings.Contains(rows[1], "held ·") {
		t.Errorf("%dx%d %s: row 1 is not the title line: %q", m.width, m.height, what, rows[1])
	}
	name := ""
	if m.onRoutes {
		if r := m.selectedRoute(); r != nil {
			name = strings.ToUpper(r.Name)
		}
	} else if c := m.mapSelected(); c != nil {
		name = strings.ToUpper(c.Name)
	}
	if name == "" {
		return
	}
	secs := m.details()
	if len(secs) == 0 || secs[0].title != name {
		t.Errorf("%dx%d %s: the pane's first section is not %s: %v", m.width, m.height, what, name, secs)
	}
	if m.width < paneMinWidth && !strings.HasPrefix(rows[m.height-2], "▸ "+name) {
		t.Errorf("%dx%d %s: the strip does not name %s: %q", m.width, m.height, what, name, rows[m.height-2])
	}
	if m.paneShown() && !strings.Contains(rows[2], name) {
		t.Errorf("%dx%d %s: the pane does not open on %s: %q", m.width, m.height, what, name, rows[2])
	}
}

// A city with more corners than MAIN has rows for scrolls its grid by
// row to keep the corner under the cursor in view; the routes under
// the grid are listed whole whatever the height.
func TestMapGridScrollsRoutesNever(t *testing.T) {
	m := richModel(t, 80, 14)
	m.Update(key("5"))
	cs := m.shown().Corners
	routes := m.mapRoutes()
	if len(routes) == 0 {
		t.Fatal("no routes to list")
	}
	for i := range cs {
		m.mapCursor, m.onRoutes = i, false
		view := m.View()
		assertFits(t, view, 80, 14, "short map")
		plain := stripANSI(view)
		if !strings.Contains(plain, strings.ToUpper(cs[i].Name)) {
			t.Errorf("the grid scrolled %s out of view:\n%s", cs[i].Name, plain)
		}
		for _, r := range routes {
			if !strings.Contains(plain, r.Name) {
				t.Errorf("with the cursor on %s the route %s is clamped:\n%s", cs[i].Name, r.Name, plain)
			}
		}
	}
	// Down past the grid reaches the routes, and back up the grid.
	m.mapCursor, m.onRoutes = 0, false
	for !m.onRoutes {
		m.Update(key("j"))
	}
	if !strings.Contains(stripANSI(m.View()), "▸ "+routes[0].Name) {
		t.Errorf("the routes cursor is not drawn:\n%s", stripANSI(m.View()))
	}
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	assertFits(t, m.View(), 120, 40, "map after a resize")
}
