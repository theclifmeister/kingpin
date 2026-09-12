package ui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/ui/anim"
	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// The strike's scene (#158): the morning after a corner changes hands
// the map, first shown, burns its cell from the colour it was to the
// colour it is and slides its name into the inspector, once; any key
// ends it, consumed; the frame holds on every frame; with animation
// off the map is today's.

// strikeMorning ends the day on the rich fixture as n does and hands
// the morning a CornerTaken: the rival walks onto the corner Dre works
// overnight (the sim's own take, by hand, so the seed does not decide)
// and the tick's events say so. The report is closed, so the next key
// lands on the play screen. It returns the corner.
func strikeMorning(t *testing.T, m *Model) *game.Corner {
	t.Helper()
	c := &m.w.Home().Corners[1]
	if !c.Worked() || c.Owner != game.OwnerPlayer {
		t.Fatalf("the fixture's second corner is not worked by you: %+v", c)
	}
	evs := m.stepDay()
	c.Owner, c.Faction, c.Runner, c.Enforcer, c.Since = game.OwnerRival, m.w.Rival.Faction(), 0, 0, m.w.Day
	evs = append(evs, events.CornerTaken{Day: m.w.Day, Corner: c.ID, Name: c.Name, Rival: m.w.Rival.Leader, From: game.OwnerPlayer})
	m.morning(evs)
	skipScene(m) // a card's scene, if the seed dealt one (#154)
	closeMorning(t, m)
	if len(m.mapScene) != 1 || m.mapScene[0] != (mapFlip{c.ID, game.OwnerPlayer}) {
		t.Fatalf("the morning kept %+v for the map", m.mapScene)
	}
	return c
}

// TestStrikeSceneOnTheMap: the map's first render after the morning
// starts the scene and not the second; the cell is the burn's row and
// the inspector's title the name at the end; every frame holds the
// frame; a key ends it, consumed; the map after is play mode, no key
// returning a command.
func TestStrikeSceneOnTheMap(t *testing.T) {
	profile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(0) // termenv.TrueColor: the colours are the point
	defer lipgloss.SetColorProfile(profile)
	for _, sz := range [][2]int{{80, 24}, {120, 40}} {
		m := richModel(t, sz[0], sz[1])
		m.opts.Anim = true
		c := strikeMorning(t, m)
		gen := m.sceneGen
		// The dashboard draws and starts nothing.
		if _, cmd := m.Update(key("down")); cmd != nil || m.scene != nil {
			t.Fatalf("%dx%d: a key off the map started the scene", sz[0], sz[1])
		}
		_, cmd := m.Update(key("5"))
		if cmd == nil || m.scene == nil || m.scene.Idle || m.sceneGen != gen+1 || len(m.mapScene) != 0 {
			t.Fatalf("%dx%d: the map's first render: cmd %v scene %v pending %v", sz[0], sz[1], cmd, m.scene, m.mapScene)
		}
		if sel := m.mapSelected(); sel == nil || sel.ID != c.ID {
			t.Fatalf("%dx%d: the cursor is not on the corner: %v", sz[0], sz[1], sel)
		}
		now := time.Unix(1_700_000_000, 0)
		burnt, named := false, false
		for at := time.Duration(0); at < anim.StrikeLength; at += anim.Frame {
			if cmd := tickAt(m, now.Add(at)); cmd == nil || m.scene == nil {
				t.Fatalf("%dx%d at %v: the scene ended early", sz[0], sz[1], at)
			}
			what := fmt.Sprintf("%dx%d strike at %v", sz[0], sz[1], at)
			assertFrame(t, m, what)
			view := m.View()
			if strings.Contains(view, theme.RivalText.Render(fit("▴ "+strings.ToUpper(c.Name), m.mapCellW()-1))) {
				t.Errorf("%s: the cell is the map's own, not the scene's", what)
			}
			plain := stripANSI(view)
			if strings.ContainsAny(plain, "▙█▜▀▝") {
				burnt = true
			}
			// The name lands a word at a time (a blank is plain on the
			// canvas): its first word in the accent, not the heading's bold.
			if word := strings.Fields(strings.ToUpper(c.Name))[0]; strings.Contains(view, theme.Fg(theme.Rivals).Render(word)) {
				named = true
			}
		}
		if !burnt || !named {
			t.Errorf("%dx%d: the cell burned %v, the name slid in %v", sz[0], sz[1], burnt, named)
		}
		// Done: the map is its own again, the tick chain ends.
		if cmd := tickAt(m, now.Add(anim.StrikeLength)); cmd != nil || m.scene != nil {
			t.Fatalf("%dx%d: Done: cmd %v scene %v", sz[0], sz[1], cmd, m.scene)
		}
		assertFrame(t, m, fmt.Sprintf("%dx%d after the strike", sz[0], sz[1]))
		if !strings.Contains(m.View(), theme.Heading(theme.Rivals).Render(strings.ToUpper(c.Name))) {
			t.Errorf("%dx%d: the inspector is not the corner's after the scene", sz[0], sz[1])
		}
		// The second render starts nothing: the scene played once.
		for _, k := range []string{"down", "5", "up", "]", "]", "esc"} {
			if _, cmd := m.Update(key(k)); cmd != nil || m.scene != nil {
				t.Fatalf("%dx%d: key %q after the scene: cmd %v scene %v", sz[0], sz[1], k, cmd, m.scene)
			}
		}
	}
	// A key mid-scene ends it and is consumed: the cursor stays.
	m := richModel(t, 80, 24)
	m.opts.Anim = true
	c := strikeMorning(t, m)
	m.Update(key("5"))
	tickAt(m, time.Unix(1_700_000_000, 0))
	cur := m.mapCursor
	if _, cmd := m.Update(key("down")); cmd != nil || m.scene != nil || m.mapCursor != cur || m.onRoutes {
		t.Fatalf("down mid-scene: cmd %v scene %v cursor %d → %d", cmd, m.scene, cur, m.mapCursor)
	}
	assertFrame(t, m, "after the skip")
	if _, cmd := m.Update(key("down")); cmd != nil || m.mapCursor == cur && !m.onRoutes {
		t.Fatalf("down after the skip: cmd %v cursor %d → %d routes %v", cmd, cur, m.mapCursor, m.onRoutes)
	}
	if c.Owner != game.OwnerRival {
		t.Fatal("the scene wrote the world")
	}
}

// TestStrikeSceneIsTheMapOtherwise: with animation off the map after
// the morning is byte-for-byte the map with nothing waiting for it, no
// scene, no command; under 80x24 the same with animation on; and the
// map after the scene ended is today's map with the cursor on the
// corner.
func TestStrikeSceneIsTheMapOtherwise(t *testing.T) {
	off := richModelSeeded(t, 80, 24, 5)
	c := strikeMorning(t, off)
	plain := richModelSeeded(t, 80, 24, 5)
	strikeMorning(t, plain)
	plain.mapScene = nil
	if _, cmd := off.Update(key("5")); cmd != nil || off.scene != nil {
		t.Fatalf("animation off: cmd %v scene %v", cmd, off.scene)
	}
	plain.Update(key("5"))
	if off.View() != plain.View() {
		t.Errorf("animation off: the map is not today's:\n%s\n---\n%s", stripANSI(off.View()), stripANSI(plain.View()))
	}
	if len(off.mapScene) != 1 {
		t.Errorf("animation off: the map's render dropped what was waiting: %v", off.mapScene)
	}
	small := richModelSeeded(t, 80, 24, 5)
	small.opts.Anim = true
	strikeMorning(t, small)
	small.Update(tea.WindowSizeMsg{Width: 79, Height: 24})
	if _, cmd := small.Update(key("5")); cmd != nil || small.scene != nil || len(small.mapScene) != 1 {
		t.Fatalf("under 80x24: cmd %v scene %v pending %v", cmd, small.scene, small.mapScene)
	}
	// After the scene: the map with the cursor on the corner.
	on := richModelSeeded(t, 80, 24, 5)
	on.opts.Anim = true
	strikeMorning(t, on)
	on.Update(key("5"))
	now := time.Unix(1_700_000_000, 0)
	tickAt(on, now)
	tickAt(on, now.Add(anim.StrikeLength))
	for i := range plain.shown().Corners {
		if plain.shown().Corners[i].ID == c.ID {
			plain.mapCursor = i
		}
	}
	if on.scene != nil || on.View() != plain.View() {
		t.Errorf("after the scene the map is not today's:\n%s\n---\n%s", stripANSI(on.View()), stripANSI(plain.View()))
	}
}

// TestStrikeSceneInTheOtherCity: a corner that changed hands in the
// other city plays when that city's map is shown, not the home city's;
// a strike won burns purple to blue.
func TestStrikeSceneInTheOtherCity(t *testing.T) {
	profile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(0)
	defer lipgloss.SetColorProfile(profile)
	m := richModel(t, 120, 40)
	m.opts.Anim = true
	other := m.w.City(m.w.CityOrder[1])
	c := &other.Corners[0]
	c.Owner, c.Runner, c.Enforcer = game.OwnerRival, 0, 0
	evs := m.stepDay()
	// The strike went in overnight and took it.
	c.Owner, c.Faction, c.Since = game.OwnerPlayer, "", m.w.Day
	evs = append(evs, events.CornerStruck{Day: m.w.Day, Corner: c.ID, Name: c.Name, Taken: true})
	m.morning(evs)
	skipScene(m)
	closeMorning(t, m)
	if _, cmd := m.Update(key("5")); cmd != nil || m.scene != nil || len(m.mapScene) != 1 {
		t.Fatalf("the home map: cmd %v scene %v pending %v", cmd, m.scene, m.mapScene)
	}
	if _, cmd := m.Update(key("]")); cmd == nil || m.scene == nil || m.shown().ID != other.ID || len(m.mapScene) != 0 {
		t.Fatalf("the other city's map: cmd %v scene %v shown %s pending %v", cmd, m.scene, m.shown().ID, m.mapScene)
	}
	now := time.Unix(1_700_000_000, 0)
	tickAt(m, now)
	if open, _ := theme.Fg(theme.Rivals).Render("x"), ""; !strings.Contains(strings.Join(m.scene.Frame(40, 1), ""), open[:strings.Index(open, "x")]) {
		t.Errorf("the cell does not stand in the rival's colour at the start: %q", m.scene.Frame(40, 1))
	}
	assertFrame(t, m, "the other city's strike")
	tickAt(m, now.Add(anim.StrikeLength))
	if m.scene != nil {
		t.Fatal("the scene did not end")
	}
	// The map's own cell after: the cursor is on it, so it is Selected.
	if !strings.Contains(m.View(), theme.Selected.Render(fit("▪ "+strings.ToUpper(c.Name), m.mapCellW()-1))) || c.Owner != game.OwnerPlayer {
		t.Errorf("the corner is not yours and selected after the scene:\n%s", stripANSI(m.View()))
	}
}

// TestStrikeSceneOnFastForward: a fast-forward stops on a corner taken
// off you and the map, next shown, plays it; the morning replaces what
// waited, so a day ended with nothing to show clears it.
func TestStrikeSceneOnFastForward(t *testing.T) {
	m := richModelSeeded(t, 80, 24, fastStrikeSeed)
	m.opts.Anim = true
	m.w.Rival.Deals = nil // no split to keep it off your corners
	m.w.Rival.Muscle = 8
	for m.w.Day < 60 && !strings.Contains(m.fastStop, " took ") {
		fast(t, m, 30)
		skipScene(m)
		closeMorning(t, m)
	}
	if !strings.Contains(m.fastStop, " took ") {
		t.Fatalf("seed %d: the rival took nothing by day 60", fastStrikeSeed)
	}
	if len(m.mapScene) != 1 || m.mapScene[0].from != game.OwnerPlayer {
		t.Fatalf("the stop kept %+v for the map", m.mapScene)
	}
	if _, cmd := m.Update(key("5")); cmd == nil || m.scene == nil {
		t.Fatalf("the map after F: cmd %v scene %v", cmd, m.scene)
	}
	assertFrame(t, m, "the strike after F")
	m.Update(key("esc"))
	// The next morning, nothing waits: the flip is the journal's.
	m.Update(key("1"))
	m.w.Rival.Muscle = 0 // nothing pushes tonight
	endDay(t, m)
	closeMorning(t, m)
	if len(m.mapScene) != 0 {
		t.Errorf("the next morning kept %+v", m.mapScene)
	}
	if _, cmd := m.Update(key("5")); cmd != nil || m.scene != nil {
		t.Fatalf("the map the next day: cmd %v scene %v", cmd, m.scene)
	}
}

// fastStrikeSeed is a rich fixture on which the rival, with no deal and
// muscle to spare, takes a corner off you inside sixty days of
// fast-forwarding (day 16).
const fastStrikeSeed = 5
